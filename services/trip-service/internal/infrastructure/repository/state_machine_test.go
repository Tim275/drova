package repository

import (
	"slices"
	"testing"
)

// TestAllowedFromTransitions verifies the state machine transition map.
// This guards against accidental edits to allowedFrom that would corrupt
// the trip lifecycle enforced by the DB WHERE clause.
func TestAllowedFromTransitions(t *testing.T) {
	tests := []struct {
		from    string
		to      string
		allowed bool
	}{
		// valid forward path
		{"searching", "accepted", true},
		{"accepted", "driver_arrived", true},
		{"driver_arrived", "in_progress", true},
		{"in_progress", "completed", true},
		{"completed", "paid", true},
		// valid cancellations
		{"searching", "cancelled", true},
		{"accepted", "cancelled", true},
		{"driver_arrived", "cancelled", true},
		// invalid: skip steps
		{"searching", "completed", false},
		{"searching", "in_progress", false},
		{"accepted", "in_progress", false},
		// invalid: backwards
		{"completed", "accepted", false},
		{"in_progress", "accepted", false},
		// invalid: cancel after completion
		{"completed", "cancelled", false},
		{"paid", "cancelled", false},
	}

	for _, tt := range tests {
		t.Run(tt.from+"→"+tt.to, func(t *testing.T) {
			from, ok := allowedFrom[tt.to]
			if !ok {
				if tt.allowed {
					t.Errorf("transition to %q has no allowedFrom entry but should be reachable", tt.to)
				}
				return
			}
			found := slices.Contains(from, tt.from)
			if tt.allowed && !found {
				t.Errorf("transition %q→%q should be allowed but %q is not in allowedFrom[%q]=%v", tt.from, tt.to, tt.from, tt.to, from)
			}
			if !tt.allowed && found {
				t.Errorf("transition %q→%q should NOT be allowed but %q is in allowedFrom[%q]=%v", tt.from, tt.to, tt.from, tt.to, from)
			}
		})
	}
}

// TestCancellableStates verifies only pre-accept states can be rider-cancelled.
// Completing or paying a trip must not be cancellable.
func TestCancellableStates(t *testing.T) {
	mustContain := []string{"searching", "accepted", "driver_arrived"}
	mustNotContain := []string{"in_progress", "completed", "paid"}

	for _, s := range mustContain {
		if !slices.Contains(cancellableStates, s) {
			t.Errorf("expected %q in cancellableStates", s)
		}
	}
	for _, s := range mustNotContain {
		if slices.Contains(cancellableStates, s) {
			t.Errorf("%q must NOT be in cancellableStates", s)
		}
	}
}
