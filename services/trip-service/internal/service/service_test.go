package service

import (
	"context"
	"testing"
	"time"

	"drova/services/trip-service/internal/domain"
)

// fakeRepo, noopUsers, errNotFound, route() are defined in pricing_test.go

// ── CreateTrip ───────────────────────────────────────────────────────────────

func TestCreateTrip_SetsSearchingStatus(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, &noopUsers{}, nil)

	fare := &domain.RideFareModel{
		ID:                "fare-1",
		UserID:            "user-42",
		PackageSlug:       "economy",
		TotalPriceInCents: 2300,
		PickupAddress:     "Berlin Hbf",
		DropoffAddress:    "Alexanderplatz",
		DistanceMeters:    3500,
		DurationSeconds:   420,
	}

	trip, err := svc.CreateTrip(context.Background(), fare)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trip.Status != "searching" {
		t.Errorf("want status 'searching', got %q", trip.Status)
	}
	if trip.UserID != "user-42" {
		t.Errorf("want userID 'user-42', got %q", trip.UserID)
	}
	if trip.ID == "" {
		t.Error("trip ID must not be empty")
	}
}

func TestCreateTrip_MapsAllFareFields(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, &noopUsers{}, nil)

	fare := &domain.RideFareModel{
		UserID:            "u1",
		PackageSlug:       "business",
		TotalPriceInCents: 5000,
		PickupAddress:     "Pickup",
		DropoffAddress:    "Dropoff",
		DistanceMeters:    10000,
		DurationSeconds:   900,
	}

	trip, err := svc.CreateTrip(context.Background(), fare)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trip.PackageSlug != "business" {
		t.Errorf("want slug 'business', got %q", trip.PackageSlug)
	}
	if trip.AmountCents != 5000 {
		t.Errorf("want amount 5000, got %d", trip.AmountCents)
	}
	if trip.PickupAddress != "Pickup" {
		t.Errorf("want pickup 'Pickup', got %q", trip.PickupAddress)
	}
	if trip.DistanceMeters != 10000 {
		t.Errorf("want distance 10000, got %d", trip.DistanceMeters)
	}
	if trip.CreatedAt == 0 {
		t.Error("CreatedAt must be set")
	}
}

func TestCreateTrip_RepoError(t *testing.T) {
	repo := newFakeRepo()
	repo.createErr = errNotFound
	svc := NewService(repo, &noopUsers{}, nil)

	_, err := svc.CreateTrip(context.Background(), &domain.RideFareModel{UserID: "u1"})
	if err == nil {
		t.Fatal("expected error when repo fails")
	}
}

// ── CancelTrip ───────────────────────────────────────────────────────────────

func TestCancelTrip_SetsStatusCancelled(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, &noopUsers{}, nil)

	fare := &domain.RideFareModel{UserID: "u1"}
	trip, _ := svc.CreateTrip(context.Background(), fare)

	if err := svc.CancelTrip(context.Background(), trip.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := repo.GetTripByID(context.Background(), trip.ID)
	if got.Status != "cancelled" {
		t.Errorf("want status 'cancelled', got %q", got.Status)
	}
}

func TestCancelTrip_NotFound(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, &noopUsers{}, nil)

	err := svc.CancelTrip(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown trip")
	}
}

// ── RateTrip ─────────────────────────────────────────────────────────────────

func TestRateTrip_StoresRating(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, &noopUsers{}, nil)

	fare := &domain.RideFareModel{UserID: "u1"}
	trip, _ := svc.CreateTrip(context.Background(), fare)

	if err := svc.RateTrip(context.Background(), trip.ID, 5); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := repo.GetTripByID(context.Background(), trip.ID)
	if got.Rating != 5 {
		t.Errorf("want rating 5, got %d", got.Rating)
	}
}

func TestRateTrip_NotFound(t *testing.T) {
	svc := NewService(newFakeRepo(), &noopUsers{}, nil)
	if err := svc.RateTrip(context.Background(), "bad-id", 3); err == nil {
		t.Fatal("expected error for unknown trip")
	}
}

// ── ExpireSearch ─────────────────────────────────────────────────────────────

func TestExpireSearch_SearchingTripExpires(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, &noopUsers{}, nil)

	fare := &domain.RideFareModel{UserID: "u1"}
	trip, _ := svc.CreateTrip(context.Background(), fare)

	expired, err := svc.ExpireSearch(context.Background(), trip.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !expired {
		t.Error("want expired=true for a searching trip")
	}
}

func TestExpireSearch_NonSearchingTripNotExpired(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, &noopUsers{}, nil)

	fare := &domain.RideFareModel{UserID: "u1"}
	trip, _ := svc.CreateTrip(context.Background(), fare)
	_ = svc.CancelTrip(context.Background(), trip.ID)

	expired, err := svc.ExpireSearch(context.Background(), trip.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expired {
		t.Error("want expired=false for a cancelled trip")
	}
}

// ── GenerateTripFares ────────────────────────────────────────────────────────

func TestGenerateTripFares_SavesAllFares(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, &noopUsers{}, nil)

	r := route(10_000, 600)
	previews := svc.EstimatePackagesPriceWithRoute(r)

	fares, err := svc.GenerateTripFares(context.Background(), previews, "user-42", r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fares) != 4 {
		t.Errorf("want 4 fares, got %d", len(fares))
	}
	for _, f := range fares {
		if f.ID == "" {
			t.Error("generated fare must have an ID")
		}
		if f.UserID != "user-42" {
			t.Errorf("want userID 'user-42', got %q", f.UserID)
		}
		if f.ExpiresAt.Before(time.Now()) {
			t.Error("fare must expire in the future")
		}
		if _, err := repo.GetRideFareByID(context.Background(), f.ID); err != nil {
			t.Errorf("fare %q not found in repo after generate", f.ID)
		}
	}
}

func TestGenerateTripFares_SetsDistanceAndDuration(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, &noopUsers{}, nil)

	r := route(5000, 300)
	previews := svc.EstimatePackagesPriceWithRoute(r)

	fares, err := svc.GenerateTripFares(context.Background(), previews, "u1", r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, f := range fares {
		if f.DistanceMeters != 5000 {
			t.Errorf("want distance 5000, got %d", f.DistanceMeters)
		}
		if f.DurationSeconds != 300 {
			t.Errorf("want duration 300, got %d", f.DurationSeconds)
		}
	}
}
