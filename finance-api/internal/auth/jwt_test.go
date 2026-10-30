package auth_test

import (
	"finance-api/internal/auth"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestJWTGenerationAndParsing(t *testing.T) {
	userID := uuid.New()
	secret := "test-secret-key-12345678901234567890"

	// 1. Generate valid token
	tokenStr, err := auth.GenerateAccessToken(userID, secret)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	// 2. Parse valid token
	claims, err := auth.ParseAccessToken(tokenStr, secret)
	if err != nil {
		t.Fatalf("failed to parse valid token: %v", err)
	}

	if claims.Subject != userID.String() {
		t.Fatalf("expected subject %s, got %s", userID.String(), claims.Subject)
	}

	// 3. Parse with wrong secret -> error
	_, err = auth.ParseAccessToken(tokenStr, "wrong-secret-key")
	if err == nil {
		t.Fatal("expected error with wrong secret, got nil")
	}

	// 4. Parse invalid string -> error
	_, err = auth.ParseAccessToken("not.a.jwt", secret)
	if err == nil {
		t.Fatal("expected error with invalid token string, got nil")
	}

	// 5. Parse token with wrong signing method (None)
	noneToken := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
		"sub": userID.String(),
		"iss": "finance-api",
	})
	noneString, _ := noneToken.SignedString(jwt.UnsafeAllowNoneSignatureType)
	_, err = auth.ParseAccessToken(noneString, secret)
	if err == nil {
		t.Fatal("expected error for UnsafeAllowNoneSignatureType, got nil")
	}
}

func TestRefreshToken(t *testing.T) {
	rt1, err := auth.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("failed to generate refresh token: %v", err)
	}
	if len(rt1) == 0 {
		t.Fatal("refresh token is empty")
	}

	rt2, err := auth.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("failed to generate second refresh token: %v", err)
	}

	if rt1 == rt2 {
		t.Fatal("subsequent refresh tokens must be cryptographically unique")
	}

	hash1 := auth.HashRefreshToken(rt1)
	hash2 := auth.HashRefreshToken(rt1)
	if string(hash1) != string(hash2) {
		t.Fatal("identical refresh tokens must produce identical hash")
	}

	hashOther := auth.HashRefreshToken(rt2)
	if string(hash1) == string(hashOther) {
		t.Fatal("different refresh tokens must produce different hash")
	}
}
