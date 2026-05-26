package env

import "testing"

func TestGetString(t *testing.T) {
	if got := GetString("DROVA_TEST_MISSING", "fallback"); got != "fallback" {
		t.Errorf("missing key: got %q, want fallback", got)
	}

	t.Setenv("DROVA_TEST_SET", "value")
	if got := GetString("DROVA_TEST_SET", "fallback"); got != "value" {
		t.Errorf("set key: got %q, want value", got)
	}

	t.Setenv("DROVA_TEST_EMPTY", "")
	if got := GetString("DROVA_TEST_EMPTY", "fallback"); got != "fallback" {
		t.Errorf("empty value should fall back: got %q", got)
	}
}
