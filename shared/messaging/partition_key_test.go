package messaging

import "testing"

func TestPartitionKey(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    string
	}{
		{"trip_id wins over owner_id", `{"trip_id":"t1","owner_id":"o1"}`, "t1"},
		{"owner_id when no trip_id", `{"owner_id":"o1"}`, "o1"},
		{"user_id fallback", `{"user_id":"u1"}`, "u1"},
		{"driver_id fallback", `{"driver_id":"d1"}`, "d1"},
		{"rider_id fallback", `{"rider_id":"r1"}`, "r1"},
		{"no known id yields nil", `{"foo":"bar"}`, ""},
		{"invalid json yields nil", `not json`, ""},
		{"empty ids yield nil", `{"trip_id":"","owner_id":""}`, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := string(partitionKey([]byte(tc.payload)))
			if got != tc.want {
				t.Errorf("partitionKey(%s) = %q, want %q", tc.payload, got, tc.want)
			}
		})
	}
}
