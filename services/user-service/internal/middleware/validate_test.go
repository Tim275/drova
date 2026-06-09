package middleware

import "testing"

func TestIsStrongPassword(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"all classes", "Secret123", true},
		{"minimal valid", "Ab1", true},
		{"no uppercase", "lowercase1", false},
		{"no lowercase", "UPPERCASE1", false},
		{"no digit", "NoDigitsHere", false},
		{"empty", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isStrongPassword(c.in); got != c.want {
				t.Errorf("isStrongPassword(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

func TestPhoneRegex(t *testing.T) {
	valid := []string{"+491234567", "0123456789", "+1 (555) 123-4567"}
	invalid := []string{"abc", "12", ""}
	for _, p := range valid {
		if !phoneRx.MatchString(p) {
			t.Errorf("expected %q to be valid", p)
		}
	}
	for _, p := range invalid {
		if phoneRx.MatchString(p) {
			t.Errorf("expected %q to be invalid", p)
		}
	}
}
