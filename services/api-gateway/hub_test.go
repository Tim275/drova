package main

import (
	"testing"
)

func newTestHub() *Hub {
	return &Hub{
		drivers: make(map[string]*Client),
		riders:  make(map[string]*Client),
		pairs:   make(map[string]string),
	}
}

func TestHub_AddDriver(t *testing.T) {
	h := newTestHub()
	c := &Client{ID: "d1", PackageSlug: "economy"}
	h.AddDriver(c)

	h.mu.Lock()
	got := h.drivers["d1"]
	h.mu.Unlock()

	if got != c {
		t.Error("driver not stored in hub")
	}
}

func TestHub_RemoveDriver_ClearsPairs(t *testing.T) {
	h := newTestHub()
	driver := &Client{ID: "d1", PackageSlug: "economy"}
	rider := &Client{ID: "r1", PackageSlug: "economy"}

	h.AddDriver(driver)
	h.AddRider(rider)

	h.mu.Lock()
	_, driverPaired := h.pairs["d1"]
	h.mu.Unlock()

	if !driverPaired {
		t.Fatal("expected pair to exist before removal")
	}

	h.RemoveDriver("d1")

	h.mu.Lock()
	_, driverExists := h.drivers["d1"]
	_, pairD := h.pairs["d1"]
	_, pairR := h.pairs["r1"]
	h.mu.Unlock()

	if driverExists {
		t.Error("driver should be deleted")
	}
	if pairD || pairR {
		t.Error("pairs should be cleared after driver removal")
	}
}

func TestHub_RemoveDriver_NoPanic_WhenNotPaired(t *testing.T) {
	h := newTestHub()
	h.AddDriver(&Client{ID: "d1", PackageSlug: "economy"})
	h.RemoveDriver("d1") // should not panic even without a pair
}

func TestHub_AddRider_PairsWithAvailableDriver(t *testing.T) {
	h := newTestHub()
	driver := &Client{ID: "d1", PackageSlug: "economy"}
	rider := &Client{ID: "r1", PackageSlug: "economy"}

	h.AddDriver(driver)
	paired := h.AddRider(rider)

	if paired == nil {
		t.Fatal("expected a paired driver, got nil")
	}
	if paired.ID != "d1" {
		t.Errorf("want paired ID 'd1', got %q", paired.ID)
	}

	h.mu.Lock()
	pairD := h.pairs["d1"]
	pairR := h.pairs["r1"]
	h.mu.Unlock()

	if pairD != "r1" || pairR != "d1" {
		t.Errorf("pair mapping wrong: d1→%q, r1→%q", pairD, pairR)
	}
}

func TestHub_AddRider_NilWhenNoDriverAvailable(t *testing.T) {
	h := newTestHub()
	rider := &Client{ID: "r1", PackageSlug: "economy"}
	paired := h.AddRider(rider)

	if paired != nil {
		t.Errorf("expected nil, got %v", paired)
	}
}

func TestHub_AddRider_NilWhenPackageSlugMismatch(t *testing.T) {
	h := newTestHub()
	h.AddDriver(&Client{ID: "d1", PackageSlug: "business"})
	paired := h.AddRider(&Client{ID: "r1", PackageSlug: "economy"})

	if paired != nil {
		t.Errorf("expected nil for slug mismatch, got %v", paired)
	}
}

func TestHub_AddRider_NilWhenDriverAlreadyPaired(t *testing.T) {
	h := newTestHub()
	driver := &Client{ID: "d1", PackageSlug: "economy"}
	rider1 := &Client{ID: "r1", PackageSlug: "economy"}
	rider2 := &Client{ID: "r2", PackageSlug: "economy"}

	h.AddDriver(driver)
	h.AddRider(rider1)
	paired := h.AddRider(rider2)

	if paired != nil {
		t.Errorf("expected nil when driver already paired, got %v", paired)
	}
}

func TestHub_RemoveRider_ClearsPairs(t *testing.T) {
	h := newTestHub()
	driver := &Client{ID: "d1", PackageSlug: "economy"}
	rider := &Client{ID: "r1", PackageSlug: "economy"}

	h.AddDriver(driver)
	h.AddRider(rider)
	h.RemoveRider("r1")

	h.mu.Lock()
	_, riderExists := h.riders["r1"]
	_, pairD := h.pairs["d1"]
	_, pairR := h.pairs["r1"]
	h.mu.Unlock()

	if riderExists {
		t.Error("rider should be deleted from hub")
	}
	if pairD || pairR {
		t.Error("pairs should be cleared after rider removal")
	}
}

func TestHub_GetPaired_ReturnsDriver(t *testing.T) {
	h := newTestHub()
	driver := &Client{ID: "d1", PackageSlug: "economy"}
	rider := &Client{ID: "r1", PackageSlug: "economy"}

	h.AddDriver(driver)
	h.AddRider(rider)

	got := h.GetPaired("r1")
	if got == nil || got.ID != "d1" {
		t.Errorf("want driver d1, got %v", got)
	}

	got = h.GetPaired("d1")
	if got == nil || got.ID != "r1" {
		t.Errorf("want rider r1, got %v", got)
	}
}

func TestHub_GetPaired_NilForUnknown(t *testing.T) {
	h := newTestHub()
	if got := h.GetPaired("nonexistent"); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}
