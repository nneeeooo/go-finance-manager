package handlers

import (
	"errors"
	"finance-api/internal/auth"
	"finance-api/internal/config"
	auth_dto "finance-api/internal/dto"
	"finance-api/internal/models"
	"finance-api/internal/utils"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func Login(c *gin.Context) {
	var req auth_dto.LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request",
		})
		return
	}

	db := c.MustGet("db").(*gorm.DB)
	cfg := c.MustGet("config").(*config.Config)

	email := strings.ToLower(strings.TrimSpace(req.Email))

	// Password
	var user models.User

	if err := db.
		Where("email = ?", email).
		First(&user).Error; err != nil {

		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid email or password",
		})
		return
	}

	if !utils.VerifyPassword(req.Password, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid email or password",
		})
		return
	}

	// AT
	accessToken, err := auth.GenerateAccessToken(
		user.ID,
		cfg.JWTSecret,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to generate access token",
		})
		return
	}

	// RT child
	refreshToken, err := auth.GenerateRefreshToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to generate refresh token",
		})
		return
	}

	refreshTokenHash := auth.HashRefreshToken(refreshToken)

	// RT family
	familyID, err := uuid.NewV7()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create session",
		})
		return
	}

	// RT state write
	now := time.Now()

	familyItem := models.RefreshTokenFamily{
		ID:     familyID,
		UserID: user.ID,
	}

	refreshTokenItem := models.RefreshToken{
		ID:                   uuid.Must(uuid.NewV7()),
		TokenHash:            refreshTokenHash,
		RefreshTokenFamilyID: familyID,
		ExpiresAt:            now.Add(auth.RefreshTokenTTL),
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&familyItem).Error; err != nil {
			return err
		}

		if err := tx.Create(&refreshTokenItem).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create session",
		})
		return
	}

	// set RT cookie, return AT response
	secure := cfg.GinMode == "production"
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     "/api/v1/auth",
		MaxAge:   int(auth.RefreshTokenTTL.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})

	c.JSON(http.StatusOK, auth_dto.AccessTokenResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
	})
}

func Refresh(c *gin.Context) {
	var (
		ErrRefreshTokenExpired = errors.New("refresh token expired")
		ErrRefreshTokenReuse   = errors.New("refresh token reuse detected")
		ErrRefreshTokenRace    = errors.New("refresh token race")
		ErrSessionRevoked      = errors.New("session revoked")
	)

	db := c.MustGet("db").(*gorm.DB)
	cfg := c.MustGet("config").(*config.Config)

	refreshToken, err := c.Cookie("refresh_token")
	if err != nil || refreshToken == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Missing refresh token",
		})
		return
	}

	tokenHash := auth.HashRefreshToken(refreshToken)

	var newRefreshToken string
	var newAccessToken string

	err = db.Transaction(func(tx *gorm.DB) error {
		// Find the refresh token.
		var storedToken models.RefreshToken

		if err := tx.
			Where("token_hash = ?", tokenHash).
			First(&storedToken).
			Error; err != nil {
			return auth.ErrInvalidToken
		}

		// Token has already been used.
		if storedToken.RevokedAt != nil {
			now := time.Now()

			// Revoke the entire family.
			if err := tx.Model(&models.RefreshTokenFamily{}).
				Where(
					"id = ? AND revoked_at IS NULL",
					storedToken.RefreshTokenFamilyID,
				).
				Update("revoked_at", now).
				Error; err != nil {
				return err
			}

			return ErrRefreshTokenReuse
		}

		// Token expired.
		if time.Now().After(storedToken.ExpiresAt) {
			return ErrRefreshTokenExpired
		}

		// Get the token family.
		var family models.RefreshTokenFamily

		if err := tx.
			Where("id = ?", storedToken.RefreshTokenFamilyID).
			First(&family).
			Error; err != nil {
			return auth.ErrInvalidToken
		}

		// Family has been revoked.
		if family.RevokedAt != nil {
			return ErrSessionRevoked
		}

		// Generate replacement refresh token.
		newRefreshToken, err = auth.GenerateRefreshToken()
		if err != nil {
			return err
		}

		newRefreshTokenHash := auth.HashRefreshToken(newRefreshToken)

		newRefreshTokenID, err := uuid.NewV7()
		if err != nil {
			return err
		}

		// Generate new access token.
		newAccessToken, err = auth.GenerateAccessToken(
			family.UserID,
			cfg.JWTSecret,
		)
		if err != nil {
			return err
		}

		newToken := models.RefreshToken{
			ID:                   newRefreshTokenID,
			RefreshTokenFamilyID: storedToken.RefreshTokenFamilyID,
			TokenHash:            newRefreshTokenHash,
			ExpiresAt:            time.Now().Add(auth.RefreshTokenTTL),
		}

		now := time.Now()

		// Atomically consume the current refresh token.
		result := tx.Model(&models.RefreshToken{}).
			Where(
				"id = ? AND revoked_at IS NULL",
				storedToken.ID,
			).
			Updates(map[string]any{
				"revoked_at":     now,
				"replaced_by_id": newToken.ID,
			})

		if result.Error != nil {
			return result.Error
		}

		// Another request consumed this token first.
		if result.RowsAffected != 1 {
			return ErrRefreshTokenRace
		}

		// Store replacement.
		if err := tx.Create(&newToken).Error; err != nil {
			return err
		}

		return nil
	})

	switch {
	case errors.Is(err, auth.ErrInvalidToken):
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid refresh token",
		})
		return

	case errors.Is(err, ErrRefreshTokenExpired):
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Refresh token expired",
		})
		return

	case errors.Is(err, ErrSessionRevoked):
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Session revoked",
		})
		return

	case errors.Is(err, ErrRefreshTokenReuse):
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Refresh token reuse detected",
		})
		return

	case errors.Is(err, ErrRefreshTokenRace):
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Refresh token already used",
		})
		return

	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to rotate refresh token",
		})
		return
	}

	secure := cfg.GinMode == "production"

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "refresh_token",
		Value:    newRefreshToken,
		Path:     "/api/v1/auth",
		MaxAge:   int(auth.RefreshTokenTTL.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})

	c.JSON(http.StatusOK, auth_dto.AccessTokenResponse{
		AccessToken: newAccessToken,
		TokenType:   "Bearer",
	})
}

func Logout(c *gin.Context) {
	db := c.MustGet("db").(*gorm.DB)

	refreshToken, err := c.Cookie("refresh_token")
	if err != nil || refreshToken == "" {
		// Already effectively logged out.
		utils.ClearRefreshTokenCookie(c)

		c.Status(http.StatusNoContent)
		return
	}

	tokenHash := auth.HashRefreshToken(refreshToken)

	var token models.RefreshToken

	if err := db.
		Where("token_hash = ?", tokenHash).
		First(&token).
		Error; err != nil {
		// Don't reveal whether the token exists.
		utils.ClearRefreshTokenCookie(c)

		c.Status(http.StatusNoContent)
		return
	}

	now := time.Now()

	// Revoke the entire refresh-token family.
	if err := db.Model(&models.RefreshTokenFamily{}).
		Where(
			"id = ? AND revoked_at IS NULL",
			token.RefreshTokenFamilyID,
		).
		Update("revoked_at", now).
		Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to logout",
		})
		return
	}

	// Remove the browser's refresh-token cookie.
	utils.ClearRefreshTokenCookie(c)

	c.Status(http.StatusNoContent)
}
