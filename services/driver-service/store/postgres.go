package store

import (
	"context"

	pb "drova/shared/proto/driver"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PgStore struct{ db *pgxpool.Pool }

func NewPostgresStore(db *pgxpool.Pool) *PgStore {
	return &PgStore{db: db}
}

func (s *PgStore) Upsert(ctx context.Context, d *pb.Driver) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO drivers (id, name, package_slug, car_plate, profile_picture, last_seen)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (id) DO UPDATE SET
			name      = EXCLUDED.name,
			last_seen = NOW()`,
		d.Id, d.Name, d.PackageSlug, d.CarPlate, d.ProfilePicture,
	)
	return err
}

func (s *PgStore) GetAll(ctx context.Context) ([]*pb.Driver, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, name, package_slug, car_plate, profile_picture FROM drivers`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var drivers []*pb.Driver
	for rows.Next() {
		d := &pb.Driver{}
		_ = rows.Scan(&d.Id, &d.Name, &d.PackageSlug, &d.CarPlate, &d.ProfilePicture)
		drivers = append(drivers, d)
	}
	return drivers, nil
}
