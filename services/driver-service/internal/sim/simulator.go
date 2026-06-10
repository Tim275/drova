package sim

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"drova/shared/contracts"
	"drova/shared/messaging"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	tickInterval  = 1200 * time.Millisecond
	approachSteps = 6
	tripSteps     = 16
	geoKeyPrefix  = "drova:drivers:geo:"
)

type TripPlan struct {
	TripID      string
	RiderID     string
	DriverID    string
	PackageSlug string
	StartLat    float64
	StartLng    float64
	Pickup      messaging.Coordinate
	Route       []messaging.Coordinate
}

type Simulator struct {
	kafka  *messaging.Kafka
	rdb    *redis.Client
	log    *zap.SugaredLogger
	active sync.Map
}

func New(kafka *messaging.Kafka, rdb *redis.Client, log *zap.SugaredLogger) *Simulator {
	return &Simulator{kafka: kafka, rdb: rdb, log: log}
}

func (s *Simulator) Start(plan TripPlan) {
	if len(plan.Route) == 0 {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	if _, busy := s.active.LoadOrStore(plan.TripID, cancel); busy {
		cancel()
		return
	}
	go s.run(ctx, plan)
}

func (s *Simulator) Stop(tripID string) {
	if v, ok := s.active.LoadAndDelete(tripID); ok {
		v.(context.CancelFunc)()
	}
}

func (s *Simulator) run(ctx context.Context, p TripPlan) {
	defer s.active.Delete(p.TripID)
	s.log.Infow("sim trip started", "trip", p.TripID, "driver", p.DriverID, "rider", p.RiderID)

	start := p.Route[0]
	if p.StartLat == 0 || p.StartLng == 0 {
		p.StartLat, p.StartLng = start.Lat, start.Lng
	}

	if !s.driveLeg(ctx, p, p.StartLat, p.StartLng, p.Pickup.Lat, p.Pickup.Lng, approachSteps) {
		return
	}
	s.publishStatus(ctx, messaging.TopicTripDriverArrived, p, 0, 0)

	select {
	case <-ctx.Done():
		return
	case <-time.After(2 * time.Second):
	}
	s.publishStatus(ctx, messaging.TopicTripInProgress, p, 0, 0)

	pts := sampleRoute(p.Route, tripSteps)
	for i := 1; i < len(pts); i++ {
		if !s.driveLeg(ctx, p, pts[i-1].Lat, pts[i-1].Lng, pts[i].Lat, pts[i].Lng, 1) {
			return
		}
	}

	dest := p.Route[len(p.Route)-1]
	s.publishStatus(ctx, messaging.TopicTripCompleted, p, dest.Lat, dest.Lng)
	s.log.Infow("sim trip completed", "trip", p.TripID, "driver", p.DriverID)
}

func (s *Simulator) driveLeg(ctx context.Context, p TripPlan, fromLat, fromLng, toLat, toLng float64, steps int) bool {
	if steps < 1 {
		steps = 1
	}
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()
	for i := 1; i <= steps; i++ {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
		}
		f := float64(i) / float64(steps)
		s.publishLocation(ctx, p, fromLat+(toLat-fromLat)*f, fromLng+(toLng-fromLng)*f)
	}
	return true
}

func (s *Simulator) publishLocation(ctx context.Context, p TripPlan, lat, lng float64) {
	if s.rdb != nil && p.PackageSlug != "" {
		s.rdb.GeoAdd(ctx, geoKeyPrefix+p.PackageSlug, &redis.GeoLocation{Name: p.DriverID, Longitude: lng, Latitude: lat})
	}
	data, err := json.Marshal(messaging.DriverLocationData{Lat: lat, Lng: lng, RiderID: p.RiderID})
	if err != nil {
		return
	}
	s.publish(ctx, messaging.TopicDriverLocation, p.RiderID, data)
	s.publishToDriver(ctx, p.DriverID, messaging.TopicDriverLocation, data)
}

func (s *Simulator) publishStatus(ctx context.Context, topic string, p TripPlan, actualLat, actualLng float64) {
	ev := messaging.TripStatusEvent{TripID: p.TripID, RiderID: p.RiderID, DriverID: p.DriverID}
	if actualLat != 0 {
		ev.ActualLat = actualLat
		ev.ActualLng = actualLng
	}
	data, err := json.Marshal(ev)
	if err != nil {
		return
	}
	s.publish(ctx, topic, p.RiderID, data)
	s.publishToDriver(ctx, p.DriverID, topic, data)
}

func (s *Simulator) publishToDriver(ctx context.Context, driverID, msgType string, data json.RawMessage) {
	if s.rdb == nil {
		return
	}
	wsMsg, err := json.Marshal(contracts.WSMessage{Type: msgType, Data: data})
	if err != nil {
		return
	}
	if err := s.rdb.Publish(ctx, "ws:driver:"+driverID, string(wsMsg)).Err(); err != nil {
		s.log.Warnw("sim publish to driver failed", "driver", driverID, zap.Error(err))
	}
}

func (s *Simulator) publish(ctx context.Context, topic, ownerID string, data json.RawMessage) {
	payload, err := json.Marshal(messaging.KafkaMessage{Type: topic, OwnerID: ownerID, Data: data})
	if err != nil {
		return
	}
	if err := s.kafka.PublishMessage(ctx, topic, payload); err != nil {
		s.log.Warnw("sim publish failed", "topic", topic, zap.Error(err))
	}
}

func sampleRoute(route []messaging.Coordinate, n int) []messaging.Coordinate {
	if len(route) <= n {
		return route
	}
	out := make([]messaging.Coordinate, 0, n)
	step := float64(len(route)-1) / float64(n-1)
	for i := 0; i < n; i++ {
		out = append(out, route[int(float64(i)*step)])
	}
	out[len(out)-1] = route[len(route)-1]
	return out
}
