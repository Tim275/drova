package main

import (
	stdmath "math"
	math "math/rand/v2"
	"sync"

	"github.com/mmcloughlin/geohash"
	"context"
	pb "drova/shared/proto/driver"
	"drova/shared/util"
)

type DriverStore interface {
	Upsert(ctx context.Context, d *pb.Driver) error
	GetAll(ctx context.Context) ([]*pb.Driver, error)
}

type driverInMap struct {
	Driver         *pb.Driver
	hasLocation    bool
	busyWithTripID string
}

type Service struct {
	store   DriverStore
	drivers map[string]*driverInMap
	mu      sync.Mutex
}

func NewService(store DriverStore) *Service {
	svc := &Service{
		store:   store,
		drivers: make(map[string]*driverInMap),
	}
	existing, _ := store.GetAll(context.Background())
	for _, d := range existing {
		svc.drivers[d.Id] = &driverInMap{Driver: d}
	}
	return svc
}

func (s *Service) RegisterDriver(driverID, packageSlug, name string) (*pb.Driver, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	randomIndex := math.IntN(len(PredefinedRoutes))
	randomRoute := PredefinedRoutes[randomIndex]

	randomPlate := GenerateRandomPlate()
	randomAvatar := util.GetRandomAvatar(randomIndex)

	gh := geohash.Encode(randomRoute[0][0], randomRoute[0][1])

	if name == "" {
		name = "Driver"
	}

	driver := &pb.Driver{
		Id:             driverID,
		Geohash:        gh,
		Location:       &pb.Location{Latitude: randomRoute[0][0], Longitude: randomRoute[0][1]},
		Name:           name,
		PackageSlug:    packageSlug,
		ProfilePicture: randomAvatar,
		CarPlate:       randomPlate,
	}

	s.drivers[driver.Id] = &driverInMap{Driver: driver}
	s.store.Upsert(context.Background(), driver)

	return driver, nil
}

const maxRadiusKm = 25.0

func haversineKm(lat1, lng1, lat2, lng2 float64) float64 {
	const R = 6371.0
	dLat := (lat2 - lat1) * stdmath.Pi / 180
	dLng := (lng2 - lng1) * stdmath.Pi / 180
	a := stdmath.Sin(dLat/2)*stdmath.Sin(dLat/2) +
		stdmath.Cos(lat1*stdmath.Pi/180)*stdmath.Cos(lat2*stdmath.Pi/180)*
			stdmath.Sin(dLng/2)*stdmath.Sin(dLng/2)
	return R * 2 * stdmath.Atan2(stdmath.Sqrt(a), stdmath.Sqrt(1-a))
}

type driverWithDist struct {
	driver *pb.Driver
	distKm float64
}

func (s *Service) FindAvailableDrivers(packageSlug string, pickupLat, pickupLng float64, excludeIDs []string) []*pb.Driver {
	s.mu.Lock()
	defer s.mu.Unlock()

	excluded := make(map[string]struct{}, len(excludeIDs))
	for _, id := range excludeIDs {
		excluded[id] = struct{}{}
	}

	var candidates []driverWithDist
	for _, d := range s.drivers {
		if d.Driver.PackageSlug != packageSlug {
			continue
		}
		if d.busyWithTripID != "" {
			continue
		}
		if _, skip := excluded[d.Driver.Id]; skip {
			continue
		}
		if !d.hasLocation {
			continue // skip drivers whose real GPS hasn't arrived yet
		}
		loc := d.Driver.Location
		if loc == nil {
			continue
		}
		dist := haversineKm(loc.Latitude, loc.Longitude, pickupLat, pickupLng)
		if dist > maxRadiusKm {
			continue
		}
		candidates = append(candidates, driverWithDist{driver: d.Driver, distKm: dist})
	}

	// Sort nearest-first (Uber/Bolt model — minimise ETA)
	for i := 1; i < len(candidates); i++ {
		for j := i; j > 0 && candidates[j].distKm < candidates[j-1].distKm; j-- {
			candidates[j], candidates[j-1] = candidates[j-1], candidates[j]
		}
	}

	result := make([]*pb.Driver, len(candidates))
	for i, c := range candidates {
		result[i] = c.driver
	}
	return result
}

func (s *Service) SetBusy(driverID, tripID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d, ok := s.drivers[driverID]; ok {
		d.busyWithTripID = tripID
	}
}

func (s *Service) ClearBusy(driverID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d, ok := s.drivers[driverID]; ok {
		d.busyWithTripID = ""
	}
}

func (s *Service) UpdateLocation(driverID string, lat, lng float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d, ok := s.drivers[driverID]; ok {
		d.Driver.Location = &pb.Location{Latitude: lat, Longitude: lng}
		d.Driver.Geohash = geohash.Encode(lat, lng)
		d.hasLocation = true
	}
}

func (s *Service) UnregisterDriver(driverID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.drivers, driverID)
}
