package sim

import (
	"testing"

	"drova/shared/messaging"
)

func TestSampleRoute(t *testing.T) {
	route := make([]messaging.Coordinate, 100)
	for i := range route {
		route[i] = messaging.Coordinate{Lat: float64(i), Lng: float64(i)}
	}

	got := sampleRoute(route, 10)
	if len(got) != 10 {
		t.Fatalf("len = %d, want 10", len(got))
	}
	if got[0] != route[0] {
		t.Errorf("first point = %+v, want %+v", got[0], route[0])
	}
	if got[len(got)-1] != route[len(route)-1] {
		t.Errorf("last point must equal route end: got %+v, want %+v", got[len(got)-1], route[len(route)-1])
	}
}

func TestSampleRoute_ShorterThanN(t *testing.T) {
	route := []messaging.Coordinate{{Lat: 1}, {Lat: 2}, {Lat: 3}}
	got := sampleRoute(route, 10)
	if len(got) != 3 {
		t.Fatalf("short route should be returned unchanged, got len %d", len(got))
	}
}
