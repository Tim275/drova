package main

import (
	"context"
	"errors"
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

var searchRadii = []float64{5, 25, 100, 500, 1500}

func geoKey(packageSlug string) string {
	return "drova:drivers:geo:" + packageSlug
}

func driverKey(driverID string) string {
	return "drova:driver:" + driverID
}

func (s *Service) RegisterDriver(ctx context.Context, driverID, packageSlug, name string, lat, lng float64) (*pb.Driver, error) {
	randomIndex := math.IntN(len(PredefinedRoutes))
	randomPlate := GenerateRandomPlate()
	randomAvatar := util.GetRandomAvatar(randomIndex)

	if name == "" {
		name = "Driver"
	}

	// Use real GPS from browser if provided, otherwise fall back to a demo location.
	if lat == 0 || lng == 0 {
		randomRoute := PredefinedRoutes[randomIndex]
		lat = randomRoute[0][0]
		lng = randomRoute[0][1]
	}

	gh := geohash.Encode(lat, lng)

	driver := &pb.Driver{
		Id:             driverID,
		Geohash:        gh,
		Location:       &pb.Location{Latitude: lat, Longitude: lng},
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
		Longitude: lng,
		Latitude:  lat,
	})

	_ = s.store.Upsert(ctx, driver)
	return driver, nil
}

func (s *Service) FindAvailableDrivers(ctx context.Context, packageSlug string, pickupLat, pickupLng float64, excludeIDs []string) []*pb.Driver {
	excluded := make(map[string]struct{}, len(excludeIDs))
	for _, id := range excludeIDs {
		excluded[id] = struct{}{}
	}

	s.log.Infow("driver search start", "package", packageSlug, "lat", pickupLat, "lng", pickupLng, "excluded", excludeIDs)

	for _, radius := range searchRadii {
		results, err := s.rdb.GeoSearchLocation(ctx, geoKey(packageSlug), &redis.GeoSearchLocationQuery{
			GeoSearchQuery: redis.GeoSearchQuery{
				Longitude:  pickupLng,
				Latitude:   pickupLat,
				Radius:     radius,
				RadiusUnit: "km",
				Sort:       "ASC",
				Count:      100,
			},
			WithCoord: true,
			WithDist:  true,
		}).Result()
		if err != nil {
			s.log.Warnw("geo search error", "package", packageSlug, "radius", radius, "err", err)
			continue
		}

		s.log.Infow("geo search", "package", packageSlug, "radius", radius, "candidates", len(results))

		var drivers []*pb.Driver
		for _, r := range results {
			if _, skip := excluded[r.Name]; skip {
				s.log.Debugw("driver excluded", "driver", r.Name)
				continue
			}
			hash, err := s.rdb.HGetAll(ctx, driverKey(r.Name)).Result()
			if err != nil {
				s.log.Warnw("driver hash error", "driver", r.Name, "err", err)
				continue
			}
			if hash["name"] == "" {
				s.log.Warnw("driver hash missing — ghost GeoSet entry", "driver", r.Name)
				continue
			}
			if hash["busy"] != "" {
				s.log.Infow("driver busy, skipping", "driver", r.Name, "trip", hash["busy"])
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
		if len(drivers) > 0 {
			return drivers
		}
	}

	s.log.Warnw("no drivers found after all radii", "package", packageSlug)
	return nil
}

func (s *Service) SetBusy(ctx context.Context, driverID, tripID string) {
	if err := s.rdb.HSet(ctx, driverKey(driverID), "busy", tripID).Err(); err != nil {
		s.log.Warnw("SetBusy failed", "driver", driverID, "trip", tripID, zap.Error(err))
	}
}

func (s *Service) ClearBusy(ctx context.Context, driverID string) {
	if err := s.rdb.HSet(ctx, driverKey(driverID), "busy", "").Err(); err != nil {
		s.log.Warnw("ClearBusy failed", "driver", driverID, zap.Error(err))
	}
}


func (s *Service) UpdateLocation(ctx context.Context, driverID string, lat, lng float64) {
	if lat == 0 || lng == 0 {
		return
	}
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

func (s *Service) GetBusyTripID(ctx context.Context, driverID string) string {
	val, err := s.rdb.HGet(ctx, driverKey(driverID), "busy").Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		s.log.Warnw("GetBusyTripID failed", "driver", driverID, zap.Error(err))
	}
	return val
}

func (s *Service) UnregisterDriver(_ context.Context, driverID string) {
	// Always use background context: the caller context (WS/gRPC stream) is already
	// cancelled at disconnect time, which makes HGet return zero-value strings and
	// causes ZRem to be skipped while Del runs — leaving a ghost GeoSet entry with
	// no hash. Background context ensures both cleanup steps always complete.
	bgCtx := context.Background()
	busy, _ := s.rdb.HGet(bgCtx, driverKey(driverID), "busy").Result()
	packageSlug, err := s.rdb.HGet(bgCtx, driverKey(driverID), "packageSlug").Result()
	if err == nil && packageSlug != "" {
		s.rdb.ZRem(bgCtx, geoKey(packageSlug), driverID)
	}
	// Only delete the driver key when not in an active trip.
	// If busy, preserve the key so reconnect can resume the trip.
	if busy == "" {
		s.rdb.Del(bgCtx, driverKey(driverID))
	}
}
