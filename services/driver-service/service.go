package main

import (
	"context"
	math "math/rand/v2"

	"github.com/mmcloughlin/geohash"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	pb "drova/shared/proto/driver"
	"drova/shared/util"
)

type DriverStore interface {
	Upsert(ctx context.Context, d *pb.Driver) error
}

type Service struct {
	store DriverStore
	rdb   *redis.Client
	log   *zap.SugaredLogger
}

func NewService(store DriverStore, rdb *redis.Client, log *zap.SugaredLogger) *Service {
	return &Service{store: store, rdb: rdb, log: log}
}

const maxRadiusKm = 25.0

func geoKey(packageSlug string) string {
	return "drova:drivers:geo:" + packageSlug
}

func driverKey(driverID string) string {
	return "drova:driver:" + driverID
}

func (s *Service) RegisterDriver(ctx context.Context, driverID, packageSlug, name string) (*pb.Driver, error) {
	randomIndex := math.IntN(len(PredefinedRoutes))
	randomRoute := PredefinedRoutes[randomIndex]
	randomPlate := GenerateRandomPlate()
	randomAvatar := util.GetRandomAvatar(randomIndex)

	if name == "" {
		name = "Driver"
	}

	gh := geohash.Encode(randomRoute[0][0], randomRoute[0][1])

	driver := &pb.Driver{
		Id:             driverID,
		Geohash:        gh,
		Location:       &pb.Location{Latitude: randomRoute[0][0], Longitude: randomRoute[0][1]},
		Name:           name,
		PackageSlug:    packageSlug,
		ProfilePicture: randomAvatar,
		CarPlate:       randomPlate,
	}

	s.rdb.HSet(ctx, driverKey(driverID),
		"name", name,
		"plate", randomPlate,
		"avatar", randomAvatar,
		"packageSlug", packageSlug,
	)
	s.rdb.HSetNX(ctx, driverKey(driverID), "busy", "")
	s.rdb.GeoAdd(ctx, geoKey(packageSlug), &redis.GeoLocation{
		Name:      driverID,
		Longitude: randomRoute[0][1],
		Latitude:  randomRoute[0][0],
	})

	_ = s.store.Upsert(ctx, driver)
	return driver, nil
}

func (s *Service) FindAvailableDrivers(ctx context.Context, packageSlug string, pickupLat, pickupLng float64, excludeIDs []string) []*pb.Driver {
	excluded := make(map[string]struct{}, len(excludeIDs))
	for _, id := range excludeIDs {
		excluded[id] = struct{}{}
	}

	results, err := s.rdb.GeoSearchLocation(ctx, geoKey(packageSlug), &redis.GeoSearchLocationQuery{
		GeoSearchQuery: redis.GeoSearchQuery{
			Longitude:  pickupLng,
			Latitude:   pickupLat,
			Radius:     maxRadiusKm,
			RadiusUnit: "km",
			Sort:       "ASC",
			Count:      100,
		},
		WithCoord: true,
		WithDist:  true,
	}).Result()
	if err != nil {
		return nil
	}

	var drivers []*pb.Driver
	for _, r := range results {
		if _, skip := excluded[r.Name]; skip {
			continue
		}
		hash, err := s.rdb.HGetAll(ctx, driverKey(r.Name)).Result()
		if err != nil || hash["busy"] != "" {
			continue
		}
		drivers = append(drivers, &pb.Driver{
			Id:             r.Name,
			Name:           hash["name"],
			PackageSlug:    hash["packageSlug"],
			ProfilePicture: hash["avatar"],
			CarPlate:       hash["plate"],
			Geohash:        geohash.Encode(r.Latitude, r.Longitude),
			Location:       &pb.Location{Latitude: r.Latitude, Longitude: r.Longitude},
		})
	}
	return drivers
}

func (s *Service) SetBusy(ctx context.Context, driverID, tripID string) {
	s.rdb.HSet(ctx, driverKey(driverID), "busy", tripID)
}

func (s *Service) ClearBusy(ctx context.Context, driverID string) {
	s.rdb.HSet(ctx, driverKey(driverID), "busy", "")
}

func (s *Service) UpdateLocation(ctx context.Context, driverID string, lat, lng float64) {
	packageSlug, err := s.rdb.HGet(ctx, driverKey(driverID), "packageSlug").Result()
	if err != nil {
		return
	}
	s.rdb.GeoAdd(ctx, geoKey(packageSlug), &redis.GeoLocation{
		Name:      driverID,
		Longitude: lng,
		Latitude:  lat,
	})
}

func (s *Service) UnregisterDriver(ctx context.Context, driverID string) {
	packageSlug, err := s.rdb.HGet(ctx, driverKey(driverID), "packageSlug").Result()
	if err == nil && packageSlug != "" {
		s.rdb.ZRem(ctx, geoKey(packageSlug), driverID)
	}
	s.rdb.Del(ctx, driverKey(driverID))
}
