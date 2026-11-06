package handlers

import (
	"errors"
	"finance-api/internal/config"
	monobank_dto "finance-api/internal/dto/monobank"
	"finance-api/internal/models"
	"finance-api/internal/services"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func verifyWebhookSecret(c *gin.Context, cfg *config.Config) bool {
	if cfg.MonobankWebhookSecret == "" {
		return true // No secret configured, allow (open or development mode)
	}

	provided := c.Query("token")
	if provided == "" {
		provided = c.Query("secret")
	}
	if provided == "" {
		provided = c.GetHeader("X-Webhook-Secret")
	}
	if provided == "" {
		provided = c.GetHeader("X-Token")
	}

	return provided == cfg.MonobankWebhookSecret
}

// MonobankWebhookCheck godoc
// @Summary Monobank webhook check
// @Description Monobank calls it with GET when the webhook is set. With MONOBANK_WEBHOOK_SECRET set, the secret must
// @Description come as ?token=, ?secret=, X-Webhook-Secret or X-Token.
// @Tags webhooks
// @Produce json
// @Success 200 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse "Wrong webhook secret"
// @Router /webhooks/monobank [get]
func MonobankWebhookCheck(c *gin.Context) {
	cfg := c.MustGet("config").(*config.Config)
	if !verifyWebhookSecret(c, cfg) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized webhook token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// MonobankWebhookReceive godoc
// @Summary Monobank statement item
// @Description Saves one live purchase (idempotent, same id as a month fetch) and the account's balance.
// @Tags webhooks
// @Accept json
// @Produce json
// @Param request body monobank_dto.WebhookRequest true "Monobank StatementItem event"
// @Success 200 {object} map[string]any
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse "Wrong webhook secret"
// @Failure 404 {object} ErrorResponse "No matching Monobank account"
// @Router /webhooks/monobank [post]
func MonobankWebhookReceive(c *gin.Context) {
	cfg := c.MustGet("config").(*config.Config)
	if !verifyWebhookSecret(c, cfg) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized webhook token"})
		return
	}

	var req monobank_dto.WebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook payload"})
		return
	}

	// Monobank may send an empty ping payload on initial webhook registration
	if req.Type != "StatementItem" || req.Data.StatementItem.ID == "" {
		c.JSON(http.StatusOK, gin.H{"status": "ignored", "reason": "non-statement event"})
		return
	}

	db := c.MustGet("db").(*gorm.DB)
	item := req.Data.StatementItem

	// 1. Resolve Account
	var account models.Account
	accountFound := false

	if req.Data.Account != "" && req.Data.Account != "0" {
		err := db.Where("bank = ? AND (external_id = ? OR monobank_account_id = ?)",
			models.BankMonobank, req.Data.Account, req.Data.Account).First(&account).Error
		if err == nil {
			accountFound = true
		}
	}

	// Fallback: match by currency if account is default "0" or not directly matched
	if !accountFound {
		err := db.Where("bank = ? AND currency = ?", models.BankMonobank, item.CurrencyCode).
			Order("created_at ASC").First(&account).Error
		if err == nil {
			accountFound = true
		}
	}

	if !accountFound {
		c.JSON(http.StatusNotFound, gin.H{"error": "Matching Monobank account not found in database"})
		return
	}

	// 2. Same row (and deterministic id) as a month fetch of this statement item
	transaction := services.MonobankTransaction(account.ID, models.MonobankStatement{
		ID: item.ID, Time: item.Time, Description: item.Description, MCC: item.MCC, Amount: item.Amount,
		OperationAmount: item.OperationAmount, CurrencyCode: item.CurrencyCode, CommissionRate: item.CommissionRate,
		CashbackAmount: item.CashbackAmount, Balance: item.Balance, Hold: item.Hold, ReceiptID: item.ReceiptID,
		Comment: item.Comment,
	})
	txID := transaction.ID

	// 3. Upsert transaction (idempotent: updates hold, balance, etc. if already exists)
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"hold", "balance", "amount", "operation_amount", "cashback_amount", "commission_rate", "updated_at",
			}),
		}).Create(&transaction).Error; err != nil {
			return err
		}

		// Update latest account balance
		if err := tx.Model(&account).Update("balance", item.Balance).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save webhook transaction"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":         "ok",
		"transaction_id": txID,
		"account_id":     account.ID,
	})
}
