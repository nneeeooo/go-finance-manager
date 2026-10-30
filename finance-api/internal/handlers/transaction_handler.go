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
// @Failure 400 {object} map[string]string "Invalid request body or input"
// @Failure 500 {object} map[string]string "Failed to create transaction"
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
// @Failure 400 {object} map[string]string "Invalid user ID"
// @Failure 500 {object} map[string]string "Failed to fetch transactions"
// @Router /users/transactions [get]
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
// @Failure 400 {object} map[string]string "Invalid transaction ID"
// @Failure 404 {object} map[string]string "Transaction not found"
// @Failure 500 {object} map[string]string "Failed to delete transaction"
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

	// Delete transaction
	result := db.Delete(&models.Transaction{}, "id = ?", transactionID)

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
