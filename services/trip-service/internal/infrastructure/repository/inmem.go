package repository

import (
	"context"
	"fmt"
	"sync"

	"drova/services/trip-service/internal/domain"
	pbd "drova/shared/proto/driver"
)

var validTransitions = map[string][]string{
	"searching":      {"accepted", "cancelled"},
	"accepted":       {"driver_arrived", "cancelled"},
	"driver_arrived": {"in_progress", "cancelled"},
	"in_progress":    {"completed"},
	"completed":      {"paid"},
	"paid":           {},
	"cancelled":      {},
}

func isValidTransition(from, to string) bool {
	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

type inmemRepository struct {
	trips     map[string]*domain.TripModel
	rideFares map[string]*domain.RideFareModel
	mu        sync.RWMutex
}

func NewInmemRepository() domain.TripRepository {
	return &inmemRepository{
		trips:     make(map[string]*domain.TripModel),
		rideFares: make(map[string]*domain.RideFareModel),
	}
}

func (r *inmemRepository) CreateTrip(ctx context.Context, trip *domain.TripModel) (*domain.TripModel, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.trips[trip.ID] = trip
	return trip, nil
}

func (r *inmemRepository) SaveRideFare(ctx context.Context, f *domain.RideFareModel) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rideFares[f.ID] = f
	return nil
}

func (r *inmemRepository) GetRideFareByID(ctx context.Context, id string) (*domain.RideFareModel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	fare, ok := r.rideFares[id]
	if !ok {
		return nil, fmt.Errorf("fare %s not found", id)
	}
	return fare, nil
}

func (r *inmemRepository) GetTripByID(ctx context.Context, id string) (*domain.TripModel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	trip, ok := r.trips[id]
	if !ok {
		return nil, nil
	}
	return trip, nil
}

func (r *inmemRepository) UpdateTrip(ctx context.Context, tripID string, status string, driver *pbd.Driver) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	trip, ok := r.trips[tripID]
	if !ok {
		return fmt.Errorf("trip not found: %s", tripID)
	}
	if !isValidTransition(trip.Status, status) {
		return fmt.Errorf("invalid trip transition: %s → %s", trip.Status, status)
	}
	trip.Status = status
	if driver != nil {
		trip.DriverID = driver.Id
		trip.DriverName = driver.Name
		trip.DriverAvatar = driver.ProfilePicture
		trip.DriverPlate = driver.CarPlate
	}
	return nil
}

func (r *inmemRepository) CancelTrip(ctx context.Context, tripID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	trip, ok := r.trips[tripID]
	if !ok {
		return fmt.Errorf("trip not found: %s", tripID)
	}
	trip.Status = "cancelled"
	return nil
}

func (r *inmemRepository) ExpireSearch(ctx context.Context, tripID string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	trip, ok := r.trips[tripID]
	if !ok || trip.Status != "searching" {
		return false, nil
	}
	trip.Status = "cancelled"
	return true, nil
}

func (r *inmemRepository) GetTripsByUser(ctx context.Context, userID string) ([]*domain.TripModel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var trips []*domain.TripModel
	for _, t := range r.trips {
		if t.UserID == userID {
			trips = append(trips, t)
		}
	}
	return trips, nil
}

func (r *inmemRepository) GetTripsByDriver(ctx context.Context, driverID string) ([]*domain.TripModel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var trips []*domain.TripModel
	for _, t := range r.trips {
		if t.DriverID == driverID {
			trips = append(trips, t)
		}
	}
	return trips, nil
}

func (r *inmemRepository) RateTrip(ctx context.Context, tripID string, rating int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rating < 1 || rating > 5 {
		return fmt.Errorf("invalid rating %d: must be 1–5", rating)
	}
	trip, ok := r.trips[tripID]
	if !ok {
		return fmt.Errorf("trip not found: %s", tripID)
	}
	trip.Rating = rating
	return nil
}
