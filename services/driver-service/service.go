package main

import (
	math "math/rand/v2"
	"context"

	"github.com/mmcloughlin/geohash"
	"github.com/redis/go-redis/v9"
	pb "drova/shared/proto/driver"
	"drova/shared/util"
)

type DriverStore interface {
	Upsert(ctx context.Context, d *pb.Driver) error
}

type Service struct {
	store DriverStore
	rdb   *redis.Client
}

func NewService(store DriverStore, rdb *redis.Client) *Service {
	return &Service{store: store, rdb: rdb}
}

const maxRadiusKm = 25.0

func geoKey(packageSlug string) string {
	return "drova:drivers:geo:" + packageSlug
}

func driverKey(driverID string) string {
	return "drova:driver:" + driverID
}

func (s *Service) RegisterDriver(driverID, packageSlug, name string) (*pb.Driver, error) {
	ctx := context.Background()

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
		"busy", "",
	)

	s.store.Upsert(ctx, driver)
	return driver, nil
}

func (s *Service) FindAvailableDrivers(packageSlug string, pickupLat, pickupLng float64, excludeIDs []string) []*pb.Driver {
	ctx := context.Background()

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

func (s *Service) SetBusy(driverID, tripID string) {
	s.rdb.HSet(context.Background(), driverKey(driverID), "busy", tripID)
}

func (s *Service) ClearBusy(driverID string) {
	s.rdb.HSet(context.Background(), driverKey(driverID), "busy", "")
}

func (s *Service) UpdateLocation(driverID string, lat, lng float64) {
	ctx := context.Background()
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

func (s *Service) UnregisterDriver(driverID string) {
	ctx := context.Background()
	packageSlug, err := s.rdb.HGet(ctx, driverKey(driverID), "packageSlug").Result()
	if err == nil && packageSlug != "" {
		s.rdb.ZRem(ctx, geoKey(packageSlug), driverID)
	}
	s.rdb.Del(ctx, driverKey(driverID))
}
