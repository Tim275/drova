package main

import (
	math "math/rand/v2"
	"sync"

	"github.com/mmcloughlin/geohash"
	pb "drova/shared/proto/driver"
	"drova/shared/util"
)

type driverInMap struct {
	Driver *pb.Driver
}

type Service struct {
	drivers []*driverInMap
	mu      sync.Mutex
}

func NewService() *Service {
	return &Service{
		drivers: make([]*driverInMap, 0),
	}
}

func (s *Service) RegisterDriver(driverID string, packageSlug string) (*pb.Driver, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	randomIndex := math.IntN(len(PredefinedRoutes))
	randomRoute := PredefinedRoutes[randomIndex]

	randomPlate := GenerateRandomPlate()
	randomAvatar := util.GetRandomAvatar(randomIndex)

	gh := geohash.Encode(randomRoute[0][0], randomRoute[0][1])

	driver := &pb.Driver{
		Id:             driverID,
		Geohash:        gh,
		Location:       &pb.Location{Latitude: randomRoute[0][0], Longitude: randomRoute[0][1]},
		Name:           "Lando Norris",
		PackageSlug:    packageSlug,
		ProfilePicture: randomAvatar,
		CarPlate:       randomPlate,
	}

	s.drivers = append(s.drivers, &driverInMap{Driver: driver})

	return driver, nil
}

func (s *Service) UnregisterDriver(driverID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, d := range s.drivers {
		if d.Driver.Id == driverID {
			s.drivers = append(s.drivers[:i], s.drivers[i+1:]...)
			return
		}
	}
}
