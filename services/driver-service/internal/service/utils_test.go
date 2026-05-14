package service

import (
	"regexp"
	"testing"
)

var platePattern = regexp.MustCompile(`^[A-Z]{2}-[A-Z]{2} [1-9]\d{3}$`)

func TestGenerateRandomPlate_Format(t *testing.T) {
	for range 200 {
		plate := GenerateRandomPlate()
		if !platePattern.MatchString(plate) {
			t.Errorf("plate %q does not match expected format 'AB-CD 1234'", plate)
		}
	}
}

func TestGenerateRandomPlate_NumberRange(t *testing.T) {
	for range 200 {
		plate := GenerateRandomPlate()
		// Extract numeric part (last 4 chars)
		numStr := plate[len(plate)-4:]
		num := 0
		for _, ch := range numStr {
			num = num*10 + int(ch-'0')
		}
		if num < 1000 || num > 9999 {
			t.Errorf("plate number %d out of range [1000,9999]: %q", num, plate)
		}
	}
}

func TestGenerateRandomPlate_Uniqueness(t *testing.T) {
	seen := make(map[string]struct{}, 100)
	for range 100 {
		plate := GenerateRandomPlate()
		seen[plate] = struct{}{}
	}
	// With a large alphabet space we expect many distinct values
	if len(seen) < 50 {
		t.Errorf("expected diverse plates, only got %d unique out of 100", len(seen))
	}
}
