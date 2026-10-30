package middleware_test

import (
	"finance-api/internal/auth"
	"finance-api/internal/config"
	"finance-api/internal/middleware"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/steinfletcher/apitest"
)

func setupTestEngine(cfg *config.Config) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.AuthMiddleware(cfg))
	r.GET("/protected", func(c *gin.Context) {
		uid, _ := c.Get("user_id")
		c.JSON(http.StatusOK, gin.H{"user_id": uid})
	})
	return r
}

func TestAuthMiddleware(t *testing.T) {
	cfg := &config.Config{
		JWTSecret: "test-super-secret-jwt-key-for-middleware-tests!",
	}
	r := setupTestEngine(cfg)

	validUserID := uuid.New()
	validToken, err := auth.GenerateAccessToken(validUserID, cfg.JWTSecret)
	if err != nil {
		t.Fatalf("failed to generate valid token: %v", err)
	}

	tests := []struct {
		name       string
		header     string
		statusCode int
	}{
		{
			name:       "missing header",
			header:     "",
			statusCode: http.StatusUnauthorized,
		},
		{
			name:       "wrong scheme (Basic)",
			header:     "Basic somecredentials",
			statusCode: http.StatusUnauthorized,
		},
		{
			name:       "Bearer without token",
			header:     "Bearer ",
			statusCode: http.StatusUnauthorized,
		},
		{
			name:       "invalid token format",
			header:     "Bearer not.a.valid.jwt",
			statusCode: http.StatusUnauthorized,
		},
		{
			name:       "wrong secret signature",
			header:     "Bearer " + func() string { tok, _ := auth.GenerateAccessToken(validUserID, "wrong-secret-key"); return tok }(),
			statusCode: http.StatusUnauthorized,
		},
		{
			name:       "valid token",
			header:     "Bearer " + validToken,
			statusCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := apitest.New().
				Handler(r).
				Get("/protected")

			if tt.header != "" {
				req.Header("Authorization", tt.header)
			}

			req.Expect(t).
				Status(tt.statusCode).
				End()
		})
	}

	// Test non-UUID subject in valid token
	t.Run("non-UUID subject in token", func(t *testing.T) {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
			Subject:   "not-a-uuid",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		})
		signed, err := token.SignedString([]byte(cfg.JWTSecret))
		if err != nil {
			t.Fatal(err)
		}

		apitest.New().
			Handler(r).
			Get("/protected").
			Header("Authorization", "Bearer "+signed).
			Expect(t).
			Status(http.StatusUnauthorized).
			End()
	})
}
