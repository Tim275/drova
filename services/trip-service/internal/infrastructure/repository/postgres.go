package repository

import (
	"context"
	"encoding/json"
	"fmt"

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
		`INSERT INTO trips (id, user_id, status, fare_id, created_at) VALUES ($1, $2, $3, $4, to_timestamp($5))`,
		trip.ID, trip.UserID, trip.Status, fareID, trip.CreatedAt,
	)
	return trip, err
}

func (r *pgRepository) SaveRideFare(ctx context.Context, f *domain.RideFareModel) error {
	routeJSON, _ := json.Marshal(f.Route)
	_, err := r.db.Exec(ctx,
		`INSERT INTO ride_fares (id, user_id, package_slug, total_price_cents, route, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		f.ID, f.UserID, f.PackageSlug, f.TotalPriceInCents, routeJSON, f.ExpiresAt,
	)
	return err
}

func (r *pgRepository) GetRideFareByID(ctx context.Context, id string) (*domain.RideFareModel, error) {
	var f domain.RideFareModel
	var routeJSON []byte
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, package_slug, total_price_cents, route, expires_at
		 FROM ride_fares WHERE id = $1`, id,
	).Scan(&f.ID, &f.UserID, &f.PackageSlug, &f.TotalPriceInCents, &routeJSON, &f.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("fare %s not found", id)
	}
	if routeJSON != nil {
		json.Unmarshal(routeJSON, &f.Route)
	}
	return &f, nil
}

func (r *pgRepository) GetTripByID(ctx context.Context, id string) (*domain.TripModel, error) {
	var t domain.TripModel
	var fareID, fareUserID, farePackage pgtype.Text
	var farePriceCents pgtype.Float8
	var fareRouteJSON []byte
	err := r.db.QueryRow(ctx, `
		SELECT t.id, t.user_id, t.status,
		       COALESCE(t.driver_id,''), COALESCE(t.driver_name,''),
		       COALESCE(t.driver_plate,''), COALESCE(t.driver_avatar,''),
		       COALESCE(t.rating,0),
		       EXTRACT(EPOCH FROM t.created_at)::BIGINT,
		       f.id, f.user_id, f.package_slug, f.total_price_cents, f.route
		FROM trips t
		LEFT JOIN ride_fares f ON f.id = t.fare_id
		WHERE t.id = $1`, id,
	).Scan(&t.ID, &t.UserID, &t.Status,
		&t.DriverID, &t.DriverName, &t.DriverPlate, &t.DriverAvatar,
		&t.Rating, &t.CreatedAt,
		&fareID, &fareUserID, &farePackage, &farePriceCents, &fareRouteJSON)
	if err != nil {
		return nil, nil
	}
	if fareID.Valid {
		f := &domain.RideFareModel{
			ID:                fareID.String,
			UserID:            fareUserID.String,
			PackageSlug:       farePackage.String,
			TotalPriceInCents: farePriceCents.Float64,
		}
		if fareRouteJSON != nil {
			json.Unmarshal(fareRouteJSON, &f.Route)
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
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("invalid transition to %q: trip not found or wrong current status", status)
	}
	return nil
}

func (r *pgRepository) CancelTrip(ctx context.Context, tripID string) error {
	_, err := r.db.Exec(ctx, `UPDATE trips SET status='cancelled' WHERE id=$1`, tripID)
	return err
}

func (r *pgRepository) ExpireSearch(ctx context.Context, tripID string) (bool, error) {
	tag, err := r.db.Exec(ctx, `UPDATE trips SET status='cancelled' WHERE id=$1 AND status='searching'`, tripID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (r *pgRepository) GetTripsByUser(ctx context.Context, userID string) ([]*domain.TripModel, error) {
	rows, err := r.db.Query(ctx, `
		SELECT t.id, t.user_id, t.status,
		       COALESCE(t.driver_id,''), COALESCE(t.driver_name,''),
		       COALESCE(t.driver_plate,''), COALESCE(t.driver_avatar,''),
		       COALESCE(t.rating,0),
		       EXTRACT(EPOCH FROM t.created_at)::BIGINT,
		       COALESCE(f.id,''), COALESCE(f.package_slug,''), COALESCE(f.total_price_cents,0)
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
		var fareID, farePackage string
		var farePriceCents float64
		rows.Scan(&t.ID, &t.UserID, &t.Status,
			&t.DriverID, &t.DriverName, &t.DriverPlate, &t.DriverAvatar,
			&t.Rating, &t.CreatedAt,
			&fareID, &farePackage, &farePriceCents)
		if fareID != "" {
			t.Fare = &domain.RideFareModel{
				ID:                fareID,
				PackageSlug:       farePackage,
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
		       COALESCE(f.id,''), COALESCE(f.package_slug,''), COALESCE(f.total_price_cents,0)
		FROM trips t
		LEFT JOIN ride_fares f ON f.id = t.fare_id
		WHERE t.driver_id=$1 ORDER BY t.created_at DESC`, driverID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var trips []*domain.TripModel
	for rows.Next() {
		var t domain.TripModel
		var fareID, farePackage string
		var farePriceCents float64
		rows.Scan(&t.ID, &t.UserID, &t.Status,
			&t.RiderName, &t.RiderAvatar,
			&t.CreatedAt,
			&fareID, &farePackage, &farePriceCents)
		t.DriverID = driverID
		if fareID != "" {
			t.Fare = &domain.RideFareModel{
				ID:                fareID,
				PackageSlug:       farePackage,
				TotalPriceInCents: farePriceCents,
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
