package util

import (
	"strings"
	"testing"
)

func TestGetRandomAvatar(t *testing.T) {
	got := GetRandomAvatar(0)
	if !strings.HasPrefix(got, "https://api.dicebear.com/") {
		t.Errorf("unexpected URL: %s", got)
	}
	if !strings.Contains(got, "seed=alpha") {
		t.Errorf("seed 0 should map to alpha: %s", got)
	}
}

func TestGetRandomAvatar_WrapsModulo(t *testing.T) {
	if GetRandomAvatar(0) != GetRandomAvatar(len(driverSeeds)) {
		t.Error("seed should wrap via modulo")
	}
	if GetRandomAvatar(1) != GetRandomAvatar(len(driverSeeds)+1) {
		t.Error("seed should wrap via modulo (offset)")
	}
}
