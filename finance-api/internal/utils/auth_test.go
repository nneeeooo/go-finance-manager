package utils_test

import (
	"finance-api/internal/utils"
	"testing"
)

func TestHashAndVerifyPassword(t *testing.T) {
	password := "CorrectHorseBatteryStaple#2026"

	hash, err := utils.HashPassword(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	if hash == "" {
		t.Fatal("empty password hash")
	}

	// 1. Correct password verifies
	if !utils.VerifyPassword(password, hash) {
		t.Fatal("expected correct password to verify successfully")
	}

	// 2. Incorrect password fails
	if utils.VerifyPassword("WrongPassword123!", hash) {
		t.Fatal("expected wrong password to fail verification")
	}

	// 3. Corrupted hash formats return false gracefully
	corrupted := []string{
		"",
		"nosaltseparator",
		"invalid!base64$hash",
		"salt$invalid!base64",
		"salt$hash$extrapart",
	}

	for _, c := range corrupted {
		if utils.VerifyPassword(password, c) {
			t.Fatalf("corrupted hash %q should fail verification", c)
		}
	}
}
