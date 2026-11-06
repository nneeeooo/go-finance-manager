package handlers

import (
	"errors"
	"net/http"

	transaction_dto "finance-api/internal/dto/transaction"
	"finance-api/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// CreateTransaction godoc
// @Summary Create a transaction
// @Description Creates a new transaction for an account.
// @Tags transactions
// @Accept json
// @Produce json
// @Param request body transaction_dto.CreateTransactionRequest true "Transaction data"
// @Success 201 {object} transaction_dto.CreateTransactionResponse
// @Failure 400 {object} ErrorResponse "Invalid request body or input"
// @Failure 500 {object} ErrorResponse "Failed to create transaction"
// @Security BearerAuth
// @Failure 401 {object} ErrorResponse "Not logged in"
// @Failure 403 {object} ErrorResponse "Not your account"
// @Failure 409 {object} ErrorResponse "Transaction id already exists"
// @Router /transactions [post]
func CreateTransaction(c *gin.Context) {
	var validate = validator.New()

	var req transaction_dto.CreateTransactionRequest

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
	db := c.MustGet("db").(*gorm.DB)

	// Verify AccountID belongs to authenticated user
	var account models.Account
	if err := db.Select("id").First(&account, "id = ? AND user_id = ?", req.AccountID, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Account not found or access denied",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to verify account ownership",
		})
		return
	}

	transaction := models.Transaction{
		ID:                req.ID,
		AccountID:         req.AccountID,
		BankID:            models.Bank(req.BankID),
		ExternalID:        req.ExternalID,
		Amount:            req.Amount,
		Currency:          req.Currency,
		OperationAmount:   req.OperationAmount,
		OperationCurrency: req.OperationCurrency,
		Balance:           req.Balance,
		CashbackAmount:    req.CashbackAmount,
		CommissionRate:    req.CommissionRate,
		MCC:               req.MCC,
		Category:          req.Category,
		Description:       req.Description,
		Comment:           req.Comment,
		Hold:              req.Hold,
		OccurredAt:        req.OccurredAt,
	}

	if err := db.Create(&transaction).Error; err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			c.JSON(http.StatusConflict, gin.H{
				"error": "Transaction already exists",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create transaction",
		})
		return
	}

	c.JSON(http.StatusCreated, transaction_dto.CreateTransactionResponse{
		ID: transaction.ID,
	})
}

// TODO: bulk create user (avoid hundred million api calls)

// GetUserTransactions godoc
// @Summary Get user transactions
// @Description Returns all transactions belonging to accounts owned by the specified user, ordered by occurrence time descending.
// @Tags transactions
// @Produce json
// @Success 200 {array} transaction_dto.TransactionResponse
// @Failure 500 {object} ErrorResponse "Failed to fetch transactions"
// @Security BearerAuth
// @Failure 401 {object} ErrorResponse "Not logged in"
// @Router /transactions [get]
func GetUserTransactions(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)

	// Get DB
	db := c.MustGet("db").(*gorm.DB)

	// Get transactions
	var transactions []models.Transaction

	if err := db.
		Joins("JOIN accounts ON accounts.id = transactions.account_id").
		Where("accounts.user_id = ?", userID).
		Order("occurred_at DESC").
		Find(&transactions).Error; err != nil {

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch transactions",
		})
		return
	}

	// respond
	responses := make([]transaction_dto.TransactionResponse, len(transactions))

	for i, t := range transactions {
		responses[i] = transaction_dto.TransactionResponse{
			ID:                t.ID,
			AccountID:         t.AccountID,
			BankID:            uint16(t.BankID),
			ExternalID:        t.ExternalID,
			Amount:            t.Amount,
			Currency:          t.Currency,
			OperationAmount:   t.OperationAmount,
			OperationCurrency: t.OperationCurrency,
			Balance:           t.Balance,
			CashbackAmount:    t.CashbackAmount,
			CommissionRate:    t.CommissionRate,
			MCC:               t.MCC,
			Category:          t.Category,
			Description:       t.Description,
			Comment:           t.Comment,
			Hold:              t.Hold,
			OccurredAt:        t.OccurredAt,
		}
	}

	c.JSON(http.StatusOK, responses)
}

// DeleteTransaction godoc
// @Summary Delete transaction
// @Description Deletes a transaction by its ID.
// @Tags transactions
// @Param id path string true "Transaction ID"
// @Success 204
// @Failure 400 {object} ErrorResponse "Invalid transaction ID"
// @Failure 404 {object} ErrorResponse "Transaction not found"
// @Failure 500 {object} ErrorResponse "Failed to delete transaction"
// @Security BearerAuth
// @Failure 401 {object} ErrorResponse "Not logged in"
// @Router /transactions/{id} [delete]
func DeleteTransaction(c *gin.Context) {
	// Parse ID
	transactionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid transaction ID",
		})
		return
	}

	// Get DB
	db := c.MustGet("db").(*gorm.DB)

	// Delete transaction: only from the user's own accounts
	result := db.Where("account_id IN (?)", db.Model(&models.Account{}).Select("id").Where("user_id = ?", c.MustGet("user_id"))).
		Delete(&models.Transaction{}, "id = ?", transactionID)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete transaction",
		})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Transaction not found",
		})
		return
	}

	c.Status(http.StatusNoContent)
}

func transactionResponse(t models.Transaction) transaction_dto.TransactionResponse {
	return transaction_dto.TransactionResponse{
		ID: t.ID, AccountID: t.AccountID, BankID: uint16(t.BankID), ExternalID: t.ExternalID, Amount: t.Amount,
		Currency: t.Currency, OperationAmount: t.OperationAmount, OperationCurrency: t.OperationCurrency,
		Balance: t.Balance, CashbackAmount: t.CashbackAmount, CommissionRate: t.CommissionRate, MCC: t.MCC,
		Category: t.Category, Description: t.Description, Comment: t.Comment, Hold: t.Hold, OccurredAt: t.OccurredAt,
	}
}

// ownTransaction loads :id when it belongs to an account of the logged-in user; writes 400/404 otherwise
// (another user's transaction answers 404, like a missing one)
func ownTransaction(c *gin.Context) (models.Transaction, bool) {
	var t models.Transaction
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid transaction ID"})
		return t, false
	}
	db := c.MustGet("db").(*gorm.DB)
	err = db.Joins("JOIN accounts ON accounts.id = transactions.account_id AND accounts.deleted_at IS NULL").
		Where("transactions.id = ? AND accounts.user_id = ?", id, c.MustGet("user_id")).
		First(&t).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Transaction not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get transaction"})
		}
		return t, false
	}
	return t, true
}

// GetTransaction godoc
// @Summary Get a transaction
// @Tags transactions
// @Produce json
// @Param id path string true "Transaction ID"
// @Success 200 {object} transaction_dto.TransactionResponse
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Failure 401 {object} ErrorResponse "Not logged in"
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /transactions/{id} [get]
func GetTransaction(c *gin.Context) {
	if t, ok := ownTransaction(c); ok {
		c.JSON(http.StatusOK, transactionResponse(t))
	}
}

// UpdateTransaction godoc
// @Summary Edit a transaction
// @Description Changes what the user may correct: category, description, comment, MCC. Amounts and dates come from
// @Description the bank and stay as they are.
// @Tags transactions
// @Accept json
// @Produce json
// @Param id path string true "Transaction ID"
// @Param request body transaction_dto.UpdateTransactionRequest true "Fields to change"
// @Success 200 {object} transaction_dto.TransactionResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Failure 401 {object} ErrorResponse "Not logged in"
// @Failure 500 {object} ErrorResponse
// @Router /transactions/{id} [patch]
func UpdateTransaction(c *gin.Context) {
	var req transaction_dto.UpdateTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	if err := validator.New().Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}
	t, ok := ownTransaction(c)
	if !ok {
		return
	}
	updates := map[string]any{}
	if req.Category != nil {
		updates["category"] = *req.Category
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Comment != nil {
		updates["comment"] = *req.Comment
	}
	if req.MCC != nil {
		updates["mcc"] = *req.MCC
	}
	if len(updates) > 0 {
		if err := c.MustGet("db").(*gorm.DB).Model(&t).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update transaction"})
			return
		}
	}
	c.JSON(http.StatusOK, transactionResponse(t))
}
