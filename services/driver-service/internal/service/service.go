package service

import (
	"context"
	"errors"
	math "math/rand/v2"
	"time"

	"drova/services/driver-service/internal/domain"
	pb "drova/shared/proto/driver"
	"drova/shared/util"

	"github.com/mmcloughlin/geohash"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type Service struct {
	store domain.DriverStore
	rdb   *redis.Client
	log   *zap.SugaredLogger

	// OnDriverOnline is invoked after a driver registers/comes online. It lets the
	// dispatch layer re-check the waiting queue so a rider who got "no drivers" is
	// matched the moment a driver appears — without the service importing events
	// (wired in main.go to TripConsumer.TryMatchWaiting). Optional; nil in tests.
	OnDriverOnline func(ctx context.Context, packageSlug string, lat, lng float64)
}

func NewService(store domain.DriverStore, rdb *redis.Client, log *zap.SugaredLogger) *Service {
	return &Service{store: store, rdb: rdb, log: log}
}

var searchRadii = []float64{5, 25, 100, 500, 1500}

// driverTTL is the presence grace window. A driver counts as online while its
// heartbeat key is alive; it's refreshed on every location ping. A brief WS/gRPC
// drop (network blip, pod rollout) no longer removes the driver instantly — the
// heartbeat simply lapses after this duration if pings stop. Keep it comfortably
// larger than the location-ping interval.
const driverTTL = 30 * time.Second

func geoKey(packageSlug string) string { return "drova:drivers:geo:" + packageSlug }
func driverKey(driverID string) string { return "drova:driver:" + driverID }
func hbKey(driverID string) string     { return "drova:driver:hb:" + driverID }

func (s *Service) RegisterDriver(ctx context.Context, driverID, packageSlug, name string, lat, lng float64) (*pb.Driver, error) {
	randomIndex := math.IntN(len(PredefinedRoutes)) // #nosec G404 -- route index, not security-sensitive
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

	if err := s.rdb.HSet(ctx, driverKey(driverID),
		"name", name,
		"plate", randomPlate,
		"avatar", randomAvatar,
		"packageSlug", packageSlug,
	).Err(); err != nil {
		s.log.Warnw("RegisterDriver HSet failed", "driver", driverID, zap.Error(err))
	}
	if err := s.rdb.HSetNX(ctx, driverKey(driverID), "busy", "").Err(); err != nil {
		s.log.Warnw("RegisterDriver HSetNX busy failed", "driver", driverID, zap.Error(err))
	}
	if err := s.rdb.GeoAdd(ctx, geoKey(packageSlug), &redis.GeoLocation{
		Name:      driverID,
		Longitude: lng,
		Latitude:  lat,
	}).Err(); err != nil {
		s.log.Warnw("RegisterDriver GeoAdd failed", "driver", driverID, zap.Error(err))
	}
	// Mark the driver present; refreshed on every location ping (see UpdateLocation).
	if err := s.rdb.Set(ctx, hbKey(driverID), "1", driverTTL).Err(); err != nil {
		s.log.Warnw("RegisterDriver heartbeat set failed", "driver", driverID, zap.Error(err))
	}

	if err := s.store.Upsert(ctx, driver); err != nil {
		s.log.Warnw("RegisterDriver store upsert failed", "driver", driverID, zap.Error(err))
	}

	// New supply just appeared — let dispatch re-check riders waiting for this package.
	if s.OnDriverOnline != nil {
		go s.OnDriverOnline(context.Background(), packageSlug, lat, lng) //nolint:gosec // detached: registration must not block on matching
	}
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
			// Presence gate: skip drivers whose heartbeat has lapsed (disconnected
			// longer than driverTTL). Lazily reap the stale geo entry so it stops
			// showing up. A transient redis error (err != nil) keeps the driver to
			// avoid wrongly dropping someone on a blip.
			if alive, err := s.rdb.Exists(ctx, hbKey(r.Name)).Result(); err == nil && alive == 0 {
				s.log.Infow("driver heartbeat expired, reaping", "driver", r.Name)
				s.rdb.ZRem(ctx, geoKey(packageSlug), r.Name)
				if hash["busy"] == "" {
					s.rdb.Del(ctx, driverKey(r.Name))
				}
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
	// Refresh presence on every ping — this is what keeps the driver "online".
	s.rdb.Set(ctx, hbKey(driverID), "1", driverTTL)
}

func (s *Service) GetBusyTripID(ctx context.Context, driverID string) string {
	val, err := s.rdb.HGet(ctx, driverKey(driverID), "busy").Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		s.log.Warnw("GetBusyTripID failed", "driver", driverID, zap.Error(err))
	}
	return val
}

func (s *Service) UnregisterDriver(_ context.Context, driverID string) {
	// Soft disconnect. We deliberately do NOT remove the driver from the geo set
	// here. A WS/gRPC stream end fires on every network blip and on every pod
	// rollout — removing instantly made drivers vanish ("no drivers available"
	// flicker). Instead we let the heartbeat key lapse: it's refreshed on each
	// location ping (driverTTL), so a driver that reconnects within the grace
	// window is never seen as offline. Stale entries (no heartbeat) are reaped
	// lazily in FindAvailableDrivers. Busy/trip state stays intact for resume.
	s.log.Infow("driver stream ended; presence lapses via heartbeat", "driver", driverID)
}
