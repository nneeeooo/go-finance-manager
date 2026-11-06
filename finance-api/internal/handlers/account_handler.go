package handlers

import (
	"errors"
	account_dto "finance-api/internal/dto/account"
	transaction_dto "finance-api/internal/dto/transaction"
	"finance-api/internal/models"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CreateAccount godoc
// @Summary Create an account
// @Description Creates a new bank account for a user.
// @Tags accounts
// @Accept json
// @Produce json
// @Param request body account_dto.CreateAccountRequest true "Account data"
// @Success 201 {object} account_dto.CreateAccountResponse
// @Failure 400 {object} ErrorResponse "Invalid request body or input"
// @Failure 500 {object} ErrorResponse "Failed to create account"
// @Security BearerAuth
// @Failure 401 {object} ErrorResponse "Not logged in"
// @Router /accounts [post]
func CreateAccount(c *gin.Context) {
	var req account_dto.CreateAccountRequest
	var validate = validator.New()

	// Bind
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

	userID := c.MustGet("user_id").(uuid.UUID)

	externalID := req.ExternalID
	if externalID == "" {
		externalID = req.MonobankAccountID
	}

	account := models.Account{
		UserID:            userID,
		Bank:              req.Bank,
		ExternalID:        externalID,
		MonobankAccountID: externalID,
		Name:              req.Name,
		Currency:          req.Currency,
		Balance:           req.Balance,
	}

	db := c.MustGet("db").(*gorm.DB)

	if err := db.Create(&account).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create account",
		})
		return
	}

	c.JSON(http.StatusCreated, account_dto.CreateAccountResponse{
		ID: account.ID,
	})
}

// GetAccount godoc
// @Summary Get account
// @Description Returns an account by ID.
// @Tags accounts
// @Produce json
// @Param id path string true "Account ID"
// @Success 200 {object} account_dto.GetAccountResponse
// @Failure 400 {object} ErrorResponse "Invalid account ID"
// @Failure 404 {object} ErrorResponse "Account not found"
// @Failure 500 {object} ErrorResponse "Failed to get account"
// @Security BearerAuth
// @Failure 401 {object} ErrorResponse "Not logged in"
// @Router /accounts/{id} [get]
func GetAccount(c *gin.Context) {
	idStr := c.Param("account_id")
	if idStr == "" {
		idStr = c.Param("id")
	}
	accountID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid account ID",
		})
		return
	}

	userID := c.MustGet("user_id").(uuid.UUID)
	db := c.MustGet("db").(*gorm.DB)

	var account models.Account

	if err := db.First(&account, "id = ? AND user_id = ?", accountID, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Account not found",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get account",
		})
		return
	}

	response := account_dto.GetAccountResponse{
		ID:                account.ID,
		UserID:            account.UserID,
		Bank:              account.Bank,
		ExternalID:        account.ExternalID,
		MonobankAccountID: account.ExternalID,
		Name:              account.Name,
		Currency:          account.Currency,
		Balance:           account.Balance,
	}

	c.JSON(http.StatusOK, response)
}

// GetAccountTransactions godoc
// @Summary Get account transactions
// @Description Returns all transactions belonging to an account.
// @Tags transactions
// @Produce json
// @Param id path string true "Account ID"
// @Success 200 {array} transaction_dto.TransactionResponse
// @Failure 400 {object} ErrorResponse "Invalid account ID"
// @Failure 500 {object} ErrorResponse "Failed to fetch transactions"
// @Security BearerAuth
// @Failure 401 {object} ErrorResponse "Not logged in"
// @Failure 404 {object} ErrorResponse "Account not found"
// @Router /accounts/{id}/transactions [get]
func GetAccountTransactions(c *gin.Context) {
	// Parse account ID
	accountID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid account ID",
		})
		return
	}

	userID := c.MustGet("user_id").(uuid.UUID)

	// Get DB
	db := c.MustGet("db").(*gorm.DB)

	// Verify account belongs to user
	var account models.Account
	if err := db.Select("id").First(&account, "id = ? AND user_id = ?", accountID, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Account not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to verify account ownership",
		})
		return
	}

	// Get transactions
	var transactions []models.Transaction

	if err := db.
		Where("account_id = ?", accountID).
		Order("occurred_at DESC").
		Find(&transactions).Error; err != nil {

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch transactions",
		})
		return
	}

	// Respond
	responses := make([]transaction_dto.TransactionResponse, len(transactions))

	for i, transaction := range transactions {
		responses[i] = transaction_dto.TransactionResponse{
			ID:                transaction.ID,
			AccountID:         transaction.AccountID,
			BankID:            uint16(transaction.BankID),
			ExternalID:        transaction.ExternalID,
			Amount:            transaction.Amount,
			OperationAmount:   transaction.OperationAmount,
			OperationCurrency: transaction.OperationCurrency,
			Balance:           transaction.Balance,
			CashbackAmount:    transaction.CashbackAmount,
			CommissionRate:    transaction.CommissionRate,
			MCC:               transaction.MCC,
			Category:          transaction.Category,
			Description:       transaction.Description,
			Comment:           transaction.Comment,
			Hold:              transaction.Hold,
			OccurredAt:        transaction.OccurredAt,
		}
	}

	c.JSON(http.StatusOK, responses)
}

// DeleteAccount godoc
// @Summary Delete account
// @Description Deletes an account by its ID.
// @Tags accounts
// @Param id path string true "Account ID"
// @Success 204
// @Failure 400 {object} ErrorResponse "Invalid account ID"
// @Failure 404 {object} ErrorResponse "Account not found"
// @Failure 500 {object} ErrorResponse "Failed to delete account"
// @Security BearerAuth
// @Failure 401 {object} ErrorResponse "Not logged in"
// @Router /accounts/{id} [delete]
func DeleteAccount(c *gin.Context) {
	// Parse ID
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid account ID",
		})
		return
	}

	// Get DB
	db := c.MustGet("db").(*gorm.DB)
	userID := c.MustGet("user_id").(uuid.UUID)

	// Delete account: only the owner's (another user's id answers 404, like a missing one)
	result := db.Delete(&models.Account{}, "id = ? AND user_id = ?", id, userID)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete account",
		})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Account not found",
		})
		return
	}

	c.Status(http.StatusNoContent)
}

// UpdateAccount godoc
// @Summary Edit an account
// @Description Name, Monobank account id, currency or balance; only the owner's account. The bank stays.
// @Tags accounts
// @Accept json
// @Produce json
// @Param id path string true "Account ID"
// @Param request body account_dto.UpdateAccountRequest true "Fields to change"
// @Success 200 {object} account_dto.GetAccountResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Failure 401 {object} ErrorResponse "Not logged in"
// @Failure 500 {object} ErrorResponse
// @Router /accounts/{id} [patch]
func UpdateAccount(c *gin.Context) {
	var req account_dto.UpdateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	if err := validator.New().Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid account ID"})
		return
	}
	db := c.MustGet("db").(*gorm.DB)
	var account models.Account
	if err := db.First(&account, "id = ? AND user_id = ?", id, c.MustGet("user_id")).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Account not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get account"})
		return
	}
	updates := map[string]any{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.ExternalID != nil {
		updates["external_id"] = *req.ExternalID
		updates["monobank_account_id"] = *req.ExternalID // kept in step, as on create
	}
	if req.Currency != nil {
		updates["currency"] = *req.Currency
	}
	if req.Balance != nil {
		updates["balance"] = *req.Balance
	}
	if len(updates) > 0 {
		if err := db.Model(&account).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update account"})
			return
		}
	}
	c.JSON(http.StatusOK, account_dto.GetAccountResponse{
		ID: account.ID, UserID: account.UserID, Bank: account.Bank, ExternalID: account.ExternalID,
		MonobankAccountID: account.ExternalID, Name: account.Name, Currency: account.Currency, Balance: account.Balance,
	})
}
