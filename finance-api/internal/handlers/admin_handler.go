package handlers

import (
	"errors"
	"finance-api/internal/models"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type allowedEmailResponse struct {
	ID           uuid.UUID   `json:"id"`
	Email        string      `json:"email"`
	Role         models.Role `json:"role"`
	Registered   bool        `json:"registered"`
	RegisteredAt *time.Time  `json:"registered_at,omitempty"`
	CreatedAt    time.Time   `json:"created_at"`
}

func listAllowedEmails(db *gorm.DB) ([]allowedEmailResponse, error) {
	var entries []models.AllowedEmail
	if err := db.Order("created_at ASC").Find(&entries).Error; err != nil {
		return nil, err
	}
	// Registered = a live (not removed) user with this email
	var emails []string
	if err := db.Model(&models.User{}).Pluck("LOWER(email)", &emails).Error; err != nil {
		return nil, err
	}
	users := make(map[string]bool, len(emails))
	for _, e := range emails {
		users[e] = true
	}
	out := make([]allowedEmailResponse, len(entries))
	for i, e := range entries {
		out[i] = allowedEmailResponse{ID: e.ID, Email: e.Email, Role: e.Role, Registered: users[strings.ToLower(e.Email)],
			RegisteredAt: e.RegisteredAt, CreatedAt: e.CreatedAt}
	}
	return out, nil
}

func replyAllowedEmails(c *gin.Context, status int) {
	list, err := listAllowedEmails(c.MustGet("db").(*gorm.DB))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list allowed emails"})
		return
	}
	c.JSON(status, list)
}

// allowedEntry loads the entry of :id; refuses the admin's own entry when self is false
func allowedEntry(c *gin.Context, self bool) (models.AllowedEmail, bool) {
	var entry models.AllowedEmail
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return entry, false
	}
	if err := c.MustGet("db").(*gorm.DB).First(&entry, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Not on the allow-list"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get the entry"})
		}
		return entry, false
	}
	if !self && strings.EqualFold(entry.Email, c.GetString("user_email")) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "You cannot change or remove your own access"})
		return entry, false
	}
	return entry, true
}

// ListAllowedEmails godoc
// @Summary Who may register (admin)
// @Tags admin
// @Produce json
// @Success 200 {array} allowedEmailResponse
// @Failure 403 {object} ErrorResponse "Admins only"
// @Security BearerAuth
// @Failure 401 {object} ErrorResponse "Not logged in"
// @Failure 500 {object} ErrorResponse
// @Router /admin/allowed-emails [get]
func ListAllowedEmails(c *gin.Context) {
	replyAllowedEmails(c, http.StatusOK)
}

type addAllowedEmailRequest struct {
	Email string      `json:"email" binding:"required"`
	Role  models.Role `json:"role"`
}

// AddAllowedEmail godoc
// @Summary Allow an email to register (admin)
// @Description Role defaults to regular.
// @Tags admin
// @Accept json
// @Produce json
// @Param request body addAllowedEmailRequest true "Email and role"
// @Success 201 {array} allowedEmailResponse
// @Failure 400 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse "Already on the list"
// @Security BearerAuth
// @Failure 401 {object} ErrorResponse "Not logged in"
// @Failure 403 {object} ErrorResponse "Admins only"
// @Failure 500 {object} ErrorResponse
// @Router /admin/allowed-emails [post]
func AddAllowedEmail(c *gin.Context) {
	var req addAllowedEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Expected an email"})
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if a, err := mail.ParseAddress(email); err != nil || a.Address != email {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email"})
		return
	}
	if req.Role == "" {
		req.Role = models.RoleRegular
	}
	if !req.Role.Valid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Role must be regular or admin"})
		return
	}
	db := c.MustGet("db").(*gorm.DB)
	var count int64
	db.Model(&models.AllowedEmail{}).Where("LOWER(email) = ?", email).Count(&count)
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Already on the list"})
		return
	}
	if err := db.Create(&models.AllowedEmail{Email: email, Role: req.Role}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add the email"})
		return
	}
	replyAllowedEmails(c, http.StatusCreated)
}

type updateAllowedEmailRequest struct {
	Role models.Role `json:"role" binding:"required"`
}

// UpdateAllowedEmail godoc
// @Summary Change a role (admin)
// @Description Changes the allow-list entry and, when registered, the user. Not your own.
// @Tags admin
// @Accept json
// @Produce json
// @Param id path string true "Entry ID"
// @Param request body updateAllowedEmailRequest true "New role"
// @Success 200 {array} allowedEmailResponse
// @Failure 400 {object} ErrorResponse
// @Security BearerAuth
// @Failure 401 {object} ErrorResponse "Not logged in"
// @Failure 403 {object} ErrorResponse "Admins only"
// @Failure 404 {object} ErrorResponse "Not on the allow-list"
// @Failure 500 {object} ErrorResponse
// @Router /admin/allowed-emails/{id} [patch]
func UpdateAllowedEmail(c *gin.Context) {
	var req updateAllowedEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil || !req.Role.Valid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Role must be regular or admin"})
		return
	}
	entry, ok := allowedEntry(c, false)
	if !ok {
		return
	}
	db := c.MustGet("db").(*gorm.DB)
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&entry).Update("role", req.Role).Error; err != nil {
			return err
		}
		return tx.Model(&models.User{}).Where("LOWER(email) = ?", strings.ToLower(entry.Email)).Update("role", req.Role).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to change the role"})
		return
	}
	replyAllowedEmails(c, http.StatusOK)
}

// RemoveAllowedEmail godoc
// @Summary Take an email off the allow-list (admin)
// @Description The email can no longer register. A registered user is removed too: their sessions end and they
// @Description cannot log in. Their accounts and operations stay in the database (soft delete). Not your own.
// @Tags admin
// @Produce json
// @Param id path string true "Entry ID"
// @Success 200 {array} allowedEmailResponse
// @Failure 400 {object} ErrorResponse
// @Security BearerAuth
// @Failure 401 {object} ErrorResponse "Not logged in"
// @Failure 403 {object} ErrorResponse "Admins only"
// @Failure 404 {object} ErrorResponse "Not on the allow-list"
// @Failure 500 {object} ErrorResponse
// @Router /admin/allowed-emails/{id} [delete]
func RemoveAllowedEmail(c *gin.Context) {
	entry, ok := allowedEntry(c, false)
	if !ok {
		return
	}
	db := c.MustGet("db").(*gorm.DB)
	err := db.Transaction(func(tx *gorm.DB) error {
		var user models.User
		err := tx.Where("LOWER(email) = ?", strings.ToLower(entry.Email)).First(&user).Error
		if err == nil {
			if err := revokeUser(tx, user.ID); err != nil {
				return err
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return tx.Delete(&entry).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove the email"})
		return
	}
	replyAllowedEmails(c, http.StatusOK)
}

// revokeUser ends the user's sessions and soft-deletes the user (accounts and operations stay)
func revokeUser(tx *gorm.DB, userID uuid.UUID) error {
	now := time.Now()
	if err := tx.Model(&models.RefreshTokenFamily{}).Where("user_id = ? AND revoked_at IS NULL", userID).Update("revoked_at", &now).Error; err != nil {
		return err
	}
	if err := tx.Delete(&models.Session{}, "user_id = ?", userID).Error; err != nil {
		return err
	}
	return tx.Delete(&models.User{}, "id = ?", userID).Error
}
