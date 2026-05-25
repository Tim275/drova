package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"drova/services/trip-service/internal/domain"
	pbd "drova/shared/proto/driver"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgRepository struct{ db *pgxpool.Pool }

func NewPostgresRepository(db *pgxpool.Pool) domain.TripRepository {
	return &pgRepository{db: db}
}

func (r *pgRepository) CreateTrip(ctx context.Context, trip *domain.TripModel) (*domain.TripModel, error) {
	fareID := ""
	if trip.Fare != nil {
		fareID = trip.Fare.ID
	}
	_, err := r.db.Exec(ctx,
		`INSERT INTO trips (id, user_id, status, fare_id, created_at, rider_name, rider_avatar,
		                    pickup_address, dropoff_address, distance_meters, duration_seconds,
		                    package_slug, amount_cents)
		 VALUES ($1, $2, $3, $4, to_timestamp($5), $6, $7, $8, $9, $10, $11, $12, $13)`,
		trip.ID, trip.UserID, trip.Status, fareID, trip.CreatedAt,
		trip.RiderName, trip.RiderAvatar,
		trip.PickupAddress, trip.DropoffAddress,
		trip.DistanceMeters, trip.DurationSeconds,
		trip.PackageSlug, trip.AmountCents,
	)
	return trip, err
}

func (r *pgRepository) CreateTripWithOutbox(ctx context.Context, trip *domain.TripModel, topic string, payload []byte) (*domain.TripModel, error) {
	fareID := ""
	if trip.Fare != nil {
		fareID = trip.Fare.ID
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err = tx.Exec(ctx,
		`INSERT INTO trips (id, user_id, status, fare_id, created_at, rider_name, rider_avatar,
		                    pickup_address, dropoff_address, distance_meters, duration_seconds,
		                    package_slug, amount_cents)
		 VALUES ($1, $2, $3, $4, to_timestamp($5), $6, $7, $8, $9, $10, $11, $12, $13)`,
		trip.ID, trip.UserID, trip.Status, fareID, trip.CreatedAt,
		trip.RiderName, trip.RiderAvatar,
		trip.PickupAddress, trip.DropoffAddress,
		trip.DistanceMeters, trip.DurationSeconds,
		trip.PackageSlug, trip.AmountCents,
	); err != nil {
		return nil, err
	}

	if _, err = tx.Exec(ctx,
		`INSERT INTO outbox (topic, payload) VALUES ($1, $2)`, topic, payload,
	); err != nil {
		return nil, err
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return trip, nil
}

func (r *pgRepository) SaveRideFare(ctx context.Context, f *domain.RideFareModel) error {
	routeJSON, _ := json.Marshal(f.Route)
	_, err := r.db.Exec(ctx,
		`INSERT INTO ride_fares (id, user_id, package_slug, total_price_cents, route, expires_at,
		                         rider_name, rider_avatar,
		                         pickup_address, dropoff_address,
		                         pickup_lat, pickup_lng, dropoff_lat, dropoff_lng,
		                         distance_meters, duration_seconds)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`,
		f.ID, f.UserID, f.PackageSlug, f.TotalPriceInCents, string(routeJSON), f.ExpiresAt,
		f.RiderName, f.RiderAvatar,
		f.PickupAddress, f.DropoffAddress,
		f.PickupLat, f.PickupLng, f.DropoffLat, f.DropoffLng,
		f.DistanceMeters, f.DurationSeconds,
	)
	return err
}

func (r *pgRepository) GetRideFareByID(ctx context.Context, id string) (*domain.RideFareModel, error) {
	var f domain.RideFareModel
	var routeJSON []byte
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, package_slug, total_price_cents, route, expires_at,
		        COALESCE(rider_name,''), COALESCE(rider_avatar,''),
		        COALESCE(pickup_address,''), COALESCE(dropoff_address,''),
		        COALESCE(pickup_lat,0), COALESCE(pickup_lng,0),
		        COALESCE(dropoff_lat,0), COALESCE(dropoff_lng,0),
		        COALESCE(distance_meters,0), COALESCE(duration_seconds,0)
		 FROM ride_fares WHERE id = $1`, id,
	).Scan(&f.ID, &f.UserID, &f.PackageSlug, &f.TotalPriceInCents, &routeJSON, &f.ExpiresAt,
		&f.RiderName, &f.RiderAvatar,
		&f.PickupAddress, &f.DropoffAddress,
		&f.PickupLat, &f.PickupLng, &f.DropoffLat, &f.DropoffLng,
		&f.DistanceMeters, &f.DurationSeconds)
	if err != nil {
		return nil, fmt.Errorf("fare %s not found", id)
	}
	if routeJSON != nil {
		if err := json.Unmarshal(routeJSON, &f.Route); err != nil {
			return nil, fmt.Errorf("unmarshal route for fare %s: %w", id, err)
		}
	}
	return &f, nil
}

func (r *pgRepository) GetTripByID(ctx context.Context, id string) (*domain.TripModel, error) {
	var t domain.TripModel
	var fareID, fareUserID, farePackage pgtype.Text
	var farePriceCents pgtype.Int8
	var fareRouteJSON []byte
	err := r.db.QueryRow(ctx, `
		SELECT t.id, t.user_id, t.status,
		       COALESCE(t.driver_id,''), COALESCE(t.driver_name,''),
		       COALESCE(t.driver_plate,''), COALESCE(t.driver_avatar,''),
		       COALESCE(t.rating,0),
		       EXTRACT(EPOCH FROM t.created_at)::BIGINT,
		       f.id, f.user_id, f.package_slug, f.total_price_cents::BIGINT, f.route
		FROM trips t
		LEFT JOIN ride_fares f ON f.id = t.fare_id
		WHERE t.id = $1`, id,
	).Scan(&t.ID, &t.UserID, &t.Status,
		&t.DriverID, &t.DriverName, &t.DriverPlate, &t.DriverAvatar,
		&t.Rating, &t.CreatedAt,
		&fareID, &fareUserID, &farePackage, &farePriceCents, &fareRouteJSON)
	if err != nil {
		return nil, domain.ErrNotFound
	}
	if fareID.Valid {
		f := &domain.RideFareModel{
			ID:                fareID.String,
			UserID:            fareUserID.String,
			PackageSlug:       farePackage.String,
			TotalPriceInCents: farePriceCents.Int64,
		}
		if fareRouteJSON != nil {
			if err := json.Unmarshal(fareRouteJSON, &f.Route); err != nil {
				return nil, fmt.Errorf("unmarshal route for trip %s: %w", id, err)
			}
		}
		t.Fare = f
	}
	return &t, nil
}

var allowedFrom = map[string][]string{
	"accepted":       {"searching"},
	"driver_arrived": {"accepted"},
	"in_progress":    {"driver_arrived"},
	"completed":      {"in_progress"},
	"paid":           {"completed"},
	"cancelled":      {"searching", "accepted", "driver_arrived"},
}

func (r *pgRepository) UpdateTrip(ctx context.Context, tripID string, status string, driver *pbd.Driver) error {
	from, ok := allowedFrom[status]
	if !ok {
		return fmt.Errorf("unknown trip status: %s", status)
	}

	var tag interface{ RowsAffected() int64 }
	var err error

	switch status {
	case "completed":
		now := time.Now().UTC()
		tag, err = r.db.Exec(ctx,
			`UPDATE trips SET status=$1, completed_at=$2 WHERE id=$3 AND status = ANY($4)`,
			status, now, tripID, from,
		)
	case "cancelled":
		now := time.Now().UTC()
		tag, err = r.db.Exec(ctx,
			`UPDATE trips SET status=$1, cancelled_at=$2 WHERE id=$3 AND status = ANY($4)`,
			status, now, tripID, from,
		)
	default:
		if driver != nil {
			tag, err = r.db.Exec(ctx,
				`UPDATE trips SET status=$1, driver_id=$2, driver_name=$3, driver_plate=$4, driver_avatar=$5
				 WHERE id=$6 AND status = ANY($7)`,
				status, driver.Id, driver.Name, driver.CarPlate, driver.ProfilePicture, tripID, from,
			)
		} else {
			tag, err = r.db.Exec(ctx,
				`UPDATE trips SET status=$1 WHERE id=$2 AND status = ANY($3)`,
				status, tripID, from,
			)
		}
	}
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("invalid transition to %q: trip not found or wrong current status", status)
	}
	return nil
}

var cancellableStates = []string{"searching", "accepted", "driver_arrived"}

func (r *pgRepository) CancelTrip(ctx context.Context, tripID string) error {
	now := time.Now().UTC()
	tag, err := r.db.Exec(ctx,
		`UPDATE trips SET status='cancelled', cancelled_at=$1, cancelled_by='rider'
		 WHERE id=$2 AND status = ANY($3)`,
		now, tripID, cancellableStates,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("cannot cancel trip %q: not found or in non-cancellable state", tripID)
	}
	return nil
}

func (r *pgRepository) ExpireSearch(ctx context.Context, tripID string) (bool, error) {
	tag, err := r.db.Exec(ctx, `UPDATE trips SET status='cancelled' WHERE id=$1 AND status='searching'`, tripID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (r *pgRepository) ExpireStaleSearching(ctx context.Context, olderThan time.Duration) (int64, error) {
	tag, err := r.db.Exec(ctx,
		`UPDATE trips SET status='cancelled' WHERE status='searching' AND created_at < NOW() - $1::interval`,
		fmt.Sprintf("%.0f seconds", olderThan.Seconds()),
	)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (r *pgRepository) GetTripsByUser(ctx context.Context, userID string) ([]*domain.TripModel, error) {
	rows, err := r.db.Query(ctx, `
		SELECT t.id, t.user_id, t.status,
		       COALESCE(t.driver_id,''), COALESCE(t.driver_name,''),
		       COALESCE(t.driver_plate,''), COALESCE(t.driver_avatar,''),
		       COALESCE(t.rating,0),
		       EXTRACT(EPOCH FROM t.created_at)::BIGINT,
		       COALESCE(t.pickup_address,''), COALESCE(t.dropoff_address,''),
		       COALESCE(t.distance_meters,0), COALESCE(t.duration_seconds,0),
		       COALESCE(t.package_slug,''), COALESCE(t.amount_cents,0),
		       COALESCE(f.id,''), COALESCE(f.total_price_cents,0)::BIGINT
		FROM trips t
		LEFT JOIN ride_fares f ON f.id = t.fare_id
		WHERE t.user_id=$1 ORDER BY t.created_at DESC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var trips []*domain.TripModel
	for rows.Next() {
		var t domain.TripModel
		var fareID string
		var farePriceCents int64
		if err := rows.Scan(&t.ID, &t.UserID, &t.Status,
			&t.DriverID, &t.DriverName, &t.DriverPlate, &t.DriverAvatar,
			&t.Rating, &t.CreatedAt,
			&t.PickupAddress, &t.DropoffAddress,
			&t.DistanceMeters, &t.DurationSeconds,
			&t.PackageSlug, &t.AmountCents,
			&fareID, &farePriceCents); err != nil {
			return nil, fmt.Errorf("scan user trip row: %w", err)
		}
		if fareID != "" {
			t.Fare = &domain.RideFareModel{
				ID:                fareID,
				PackageSlug:       t.PackageSlug,
				TotalPriceInCents: farePriceCents,
			}
		}
		trips = append(trips, &t)
	}
	return trips, nil
}

func (r *pgRepository) GetTripsByDriver(ctx context.Context, driverID string) ([]*domain.TripModel, error) {
	rows, err := r.db.Query(ctx, `
		SELECT t.id, t.user_id, t.status,
		       COALESCE(t.rider_name,''), COALESCE(t.rider_avatar,''),
		       EXTRACT(EPOCH FROM t.created_at)::BIGINT,
		       COALESCE(t.pickup_address,''), COALESCE(t.dropoff_address,''),
		       COALESCE(t.distance_meters,0), COALESCE(t.duration_seconds,0),
		       COALESCE(t.package_slug,''), COALESCE(t.amount_cents,0)
		FROM trips t
		WHERE t.driver_id=$1 ORDER BY t.created_at DESC`, driverID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var trips []*domain.TripModel
	for rows.Next() {
		var t domain.TripModel
		if err := rows.Scan(&t.ID, &t.UserID, &t.Status,
			&t.RiderName, &t.RiderAvatar,
			&t.CreatedAt,
			&t.PickupAddress, &t.DropoffAddress,
			&t.DistanceMeters, &t.DurationSeconds,
			&t.PackageSlug, &t.AmountCents); err != nil {
			return nil, fmt.Errorf("scan driver trip row: %w", err)
		}
		t.DriverID = driverID
		if t.PackageSlug != "" {
			t.Fare = &domain.RideFareModel{
				PackageSlug:       t.PackageSlug,
				TotalPriceInCents: t.AmountCents,
			}
		}
		trips = append(trips, &t)
	}
	return trips, nil
}

func (r *pgRepository) RateTrip(ctx context.Context, tripID string, rating int) error {
	if rating < 1 || rating > 5 {
		return fmt.Errorf("invalid rating %d: must be 1–5", rating)
	}
	_, err := r.db.Exec(ctx, `UPDATE trips SET rating=$1 WHERE id=$2`, rating, tripID)
	return err
}
