package auth

import "testing"

func TestHashPasswordVerifies(t *testing.T) {
	hash, err := HashPassword("secret123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	if hash == "secret123" {
		t.Fatal("password hash must not store plaintext")
	}
	if !VerifyPassword("secret123", hash) {
		t.Fatal("expected password to verify")
	}
	if VerifyPassword("wrong", hash) {
		t.Fatal("expected wrong password to be rejected")
	}
}

func TestHashPasswordUsesRandomSalt(t *testing.T) {
	first, err := HashPassword("secret123")
	if err != nil {
		t.Fatalf("hash first password: %v", err)
	}
	second, err := HashPassword("secret123")
	if err != nil {
		t.Fatalf("hash second password: %v", err)
	}

	if first == second {
		t.Fatal("expected different hashes for the same password")
	}
}
