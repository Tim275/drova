package domain

import "testing"

func TestHashPasswordAndCompare(t *testing.T) {
	hash, err := hashPassword("Secret123")
	if err != nil {
		t.Fatalf("hashPassword: %v", err)
	}
	if len(hash) == 0 {
		t.Fatal("expected non-empty hash")
	}

	ok, err := comparePassword(hash, "Secret123")
	if err != nil {
		t.Fatalf("comparePassword (match): %v", err)
	}
	if !ok {
		t.Fatal("expected password to match")
	}

	ok, err = comparePassword(hash, "wrong-password")
	if err != nil {
		t.Fatalf("comparePassword (mismatch) returned error: %v", err)
	}
	if ok {
		t.Fatal("expected mismatch for wrong password")
	}
}

func TestComparePassword_InvalidHash(t *testing.T) {
	ok, err := comparePassword([]byte("not-a-bcrypt-hash"), "whatever")
	if ok {
		t.Fatal("expected ok=false for invalid hash")
	}
	if err == nil {
		t.Fatal("expected error for invalid hash")
	}
}
