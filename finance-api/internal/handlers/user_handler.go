package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	account_dto "finance-api/internal/dto/account"
	user_dto "finance-api/internal/dto/user"
	"finance-api/internal/models"
	"finance-api/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

// CreateUser godoc
// @Summary Create user
// @Description Creates a new user.
// @Tags users
// @Accept json
// @Produce json
// @Param request body user_dto.CreateUserRequest true "User data"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 500 {object} map[string]string
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
	user := models.User{
		Email:        strings.ToLower(strings.TrimSpace(req.Email)),
		PasswordHash: passwordHash,
	}

	db := c.MustGet("db").(*gorm.DB)

	if err := db.Create(&user).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{
			"error": "Email already exists",
		})
		return
	}

	c.JSON(http.StatusCreated, user_dto.CreateUserResponse{
		ID:    user.ID,
		Email: user.Email,
	})
}

// GetUser godoc
// @Summary Get user
// @Description Returns a user by their ID.
// @Tags users
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} user_dto.CreateUserResponse
// @Failure 400 {object} map[string]string "Invalid user ID"
// @Failure 404 {object} map[string]string "User not found"
// @Failure 500 {object} map[string]string "Failed to fetch user"
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
	})
}

// GetUserAccounts godoc
// @Summary Get user accounts
// @Description Returns all accounts belonging to a user.
// @Tags accounts
// @Produce json
// @Success 200 {array} account_dto.GetAccountResponse
// @Failure 400 {object} map[string]string "Invalid user ID"
// @Failure 500 {object} map[string]string "Failed to fetch accounts"
// @Router /users/accounts [get]
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
		responses[i] = account_dto.GetAccountResponse{
			ID:       account.ID,
			UserID:   account.UserID,
			Bank:     account.Bank,
			Name:     account.Name,
			Currency: account.Currency,
		}
	}

	c.JSON(http.StatusOK, responses)
}

// DeleteUser godoc
// @Summary Delete user
// @Description Deletes a user by their ID.
// @Tags users
// @Produce json
// @Param id path string true "User ID"
// @Success 204
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
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

	// Delete user
	db := c.MustGet("db").(*gorm.DB)

	result := db.Delete(&models.User{}, "id = ?", userID)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete user",
		})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "User not found",
		})
		return
	}

	c.Status(http.StatusNoContent)
}
