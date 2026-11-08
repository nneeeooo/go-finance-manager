package middleware

import (
	"errors"
	"finance-api/internal/auth"
	"finance-api/internal/config"
	"finance-api/internal/models"
	"finance-api/internal/sso"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func AuthMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")

		if header == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Missing authorization header",
			})
			return
		}

		parts := strings.SplitN(header, " ", 2)

		if len(parts) != 2 || parts[0] != "Bearer" || parts[1] == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid authorization header",
			})
			return
		}

		claims, err := auth.ParseAccessToken(
			parts[1],
			cfg.JWTSecret,
		)

		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid or expired token",
			})
			return
		}

		userID, err := uuid.Parse(claims.Subject)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid token",
			})
			return
		}

		c.Set("user_id", userID)

		c.Next()
	}
}

// LoadUser: after AuthMiddleware. The user must still exist, so a removed user loses access at once instead of
// when the 15-minute token expires; the role is read fresh, so a role change applies to the next request.
func LoadUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		var user models.User
		err := c.MustGet("db").(*gorm.DB).Select("id", "email", "role").First(&user, "id = ?", c.MustGet("user_id")).Error
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "User not found",
			})
			return
		}
		c.Set("user_email", user.Email)
		c.Set("user_role", user.Role)
		c.Next()
	}
}

// RequireAdmin: after AuthMiddleware; only admins pass
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if role, _ := c.Get("user_role"); role != models.RoleAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Admins only",
			})
			return
		}
		c.Next()
	}
}

// Authenticate: the guard of every logged-in route. A fin_session cookie (SSO session) wins: unsafe methods need
// the CSRF check, and an expired auth access token is refreshed server-side. Without the cookie the request may
// still carry the app's own Bearer JWT (scripts and tests; password login mints those, and it is off when SSO is
// on). sso may be nil when SSO is off.
func Authenticate(cfg *config.Config, client *sso.Client) gin.HandlerFunc {
	bearer := AuthMiddleware(cfg)
	return func(c *gin.Context) {
		raw, err := c.Cookie(sso.SessionCookie)
		if client == nil || err != nil || raw == "" {
			bearer(c)
			return
		}
		if !sso.CSRFOK(cfg, c.Request) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Cross-site request rejected"})
			return
		}
		userID, err := client.Authenticate(c.Request.Context(), c.MustGet("db").(*gorm.DB), raw)
		switch {
		case errors.Is(err, sso.ErrNoSession):
			sso.ClearCookie(c.Writer, cfg, sso.SessionCookie, "/")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Session expired"})
			return
		case err != nil:
			log.Println("sso session:", err)
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "Auth service unavailable"})
			return
		}
		c.Set("user_id", userID)
		c.Next()
	}
}
