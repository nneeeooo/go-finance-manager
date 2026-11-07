package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	account_dto "finance-api/internal/dto/account"
	user_dto "finance-api/internal/dto/user"
	"finance-api/internal/models"
	"finance-api/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// CreateUser godoc
// @Summary Create user
// @Description Admin only: creates a user directly (no allow-list needed). Role defaults to regular.
// @Tags users
// @Accept json
// @Produce json
// @Param request body user_dto.CreateUserRequest true "User data"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Failure 401 {object} ErrorResponse "Not logged in"
// @Failure 403 {object} ErrorResponse "Admins only"
// @Router /users [post]
func CreateUser(c *gin.Context) {
	var validate = validator.New()
	var req user_dto.CreateUserRequest

	// Bind JSON
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	// Validate
	if err := validate.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid input",
		})
		return
	}

	// Hash password
	passwordHash, err := utils.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to process password",
		})
		return
	}

	// Create user
	role := models.Role(req.Role)
	if role == "" {
		role = models.RoleRegular
	}
	user := models.User{
		Email:        strings.ToLower(strings.TrimSpace(req.Email)),
		PasswordHash: passwordHash,
		Role:         role,
	}

	db := c.MustGet("db").(*gorm.DB)

	if err := db.Create(&user).Error; err != nil {
		// 409 only for a taken email (also by a removed user); anything else is the server's fault
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			c.JSON(http.StatusConflict, gin.H{
				"error": "Email already exists",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create user",
		})
		return
	}

	c.JSON(http.StatusCreated, user_dto.CreateUserResponse{
		ID:    user.ID,
		Email: user.Email,
		Role:  user.Role,
	})
}

// GetMe godoc
// @Summary The logged-in user
// @Tags users
// @Produce json
// @Success 200 {object} user_dto.CreateUserResponse
// @Security BearerAuth
// @Failure 401 {object} ErrorResponse "Not logged in"
// @Router /users/me [get]
func GetMe(c *gin.Context) {
	role, _ := c.Get("user_role")
	c.JSON(http.StatusOK, user_dto.CreateUserResponse{
		ID:    c.MustGet("user_id").(uuid.UUID),
		Email: c.GetString("user_email"),
		Role:  role.(models.Role),
	})
}

// isSelfOrAdmin: regular users reach only their own user record
func isSelfOrAdmin(c *gin.Context, id uuid.UUID) bool {
	if role, _ := c.Get("user_role"); role == models.RoleAdmin || c.MustGet("user_id").(uuid.UUID) == id {
		return true
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "User not found"}) // the same answer as a missing user
	return false
}

// GetUser godoc
// @Summary Get user
// @Description Your own user, or anyone's as an admin (another user answers 404 for regular users).
// @Tags users
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} user_dto.CreateUserResponse
// @Failure 400 {object} ErrorResponse "Invalid user ID"
// @Failure 404 {object} ErrorResponse "User not found"
// @Failure 500 {object} ErrorResponse "Failed to fetch user"
// @Security BearerAuth
// @Failure 401 {object} ErrorResponse "Not logged in"
// @Router /users/{id} [get]
func GetUser(c *gin.Context) {
	// Parse ID
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid user ID",
		})
		return
	}

	if !isSelfOrAdmin(c, id) {
		return
	}

	// Get DB
	db := c.MustGet("db").(*gorm.DB)

	// Find user
	var user models.User

	if err := db.First(&user, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "User not found",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch user",
		})
		return
	}

	// Respond
	c.JSON(http.StatusOK, user_dto.CreateUserResponse{
		ID:    user.ID,
		Email: user.Email,
		Role:  user.Role,
	})
}

// GetUserAccounts godoc
// @Summary Get user accounts
// @Description Returns all accounts belonging to a user.
// @Tags accounts
// @Produce json
// @Success 200 {array} account_dto.GetAccountResponse
// @Failure 500 {object} ErrorResponse "Failed to fetch accounts"
// @Security BearerAuth
// @Failure 401 {object} ErrorResponse "Not logged in"
// @Router /accounts [get]
func GetUserAccounts(c *gin.Context) {
	// Parse user ID
	// userID, err := uuid.Parse(c.Param("id"))
	// if err != nil {
	// 	c.JSON(http.StatusBadRequest, gin.H{
	// 		"error": "Invalid user ID",
	// 	})
	// 	return
	// }

	userID := c.MustGet("user_id").(uuid.UUID)

	// Get DB
	db := c.MustGet("db").(*gorm.DB)

	// Get accounts
	var accounts []models.Account

	if err := db.
		Where("user_id = ?", userID).
		Order("created_at ASC").
		Find(&accounts).Error; err != nil {

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch accounts",
		})
		return
	}

	// Respond
	responses := make([]account_dto.GetAccountResponse, len(accounts))

	for i, account := range accounts {
		responses[i] = account_dto.FromModel(account)
	}

	c.JSON(http.StatusOK, responses)
}

// DeleteUser godoc
// @Summary Delete user
// @Description Admin only. The user's sessions end and they cannot log in; their accounts and operations stay
// @Description (soft delete). Not yourself.
// @Tags users
// @Produce json
// @Param id path string true "User ID"
// @Success 204
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Failure 401 {object} ErrorResponse "Not logged in"
// @Failure 403 {object} ErrorResponse "Admins only"
// @Failure 500 {object} ErrorResponse
// @Router /users/{id} [delete]
func DeleteUser(c *gin.Context) {
	// Parse id
	id := c.Param("id")

	userID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid user ID",
		})
		return
	}

	if userID == c.MustGet("user_id").(uuid.UUID) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "You cannot delete yourself",
		})
		return
	}

	// Delete user: sessions end, accounts and operations stay (soft delete)
	db := c.MustGet("db").(*gorm.DB)

	var user models.User
	if err := db.Select("id").First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "User not found",
		})
		return
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return revokeUser(tx, userID) }); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete user",
		})
		return
	}

	c.Status(http.StatusNoContent)
}

// ListUsers godoc
// @Summary All users (admin)
// @Tags users
// @Produce json
// @Success 200 {array} user_dto.CreateUserResponse
// @Failure 403 {object} ErrorResponse "Admins only"
// @Security BearerAuth
// @Failure 401 {object} ErrorResponse "Not logged in"
// @Failure 500 {object} ErrorResponse
// @Router /users [get]
func ListUsers(c *gin.Context) {
	var users []models.User
	if err := c.MustGet("db").(*gorm.DB).Order("created_at ASC").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list users"})
		return
	}
	out := make([]user_dto.CreateUserResponse, len(users))
	for i, u := range users {
		out[i] = user_dto.CreateUserResponse{ID: u.ID, Email: u.Email, Role: u.Role}
	}
	c.JSON(http.StatusOK, out)
}

// UpdateUser godoc
// @Summary Change a password
// @Description Your own password (current_password required), or as an admin anyone's. Roles change on the
// @Description allow-list (PATCH /admin/allowed-emails/{id}).
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param request body user_dto.UpdateUserRequest true "New password"
// @Success 200 {object} user_dto.CreateUserResponse
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse "Wrong current password"
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Failure 401 {object} ErrorResponse "Not logged in"
// @Failure 500 {object} ErrorResponse
// @Router /users/{id} [patch]
func UpdateUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}
	if !isSelfOrAdmin(c, id) {
		return
	}
	var req user_dto.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil || validator.New().Struct(req) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password must be 8-128 characters"})
		return
	}
	db := c.MustGet("db").(*gorm.DB)
	var user models.User
	if err := db.First(&user, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	self := id == c.MustGet("user_id").(uuid.UUID)
	if self && !utils.VerifyPassword(req.CurrentPassword, user.PasswordHash) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Current password is wrong"})
		return
	}
	hash, err := utils.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process password"})
		return
	}
	if err := db.Model(&user).Update("password_hash", hash).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}
	// Another user's password reset by an admin: their other sessions end
	if !self {
		now := time.Now()
		db.Model(&models.RefreshTokenFamily{}).Where("user_id = ? AND revoked_at IS NULL", id).Update("revoked_at", &now)
	}
	c.JSON(http.StatusOK, user_dto.CreateUserResponse{ID: user.ID, Email: user.Email, Role: user.Role})
}
