package handlers

import (
	"crypto/sha256"
	"errors"
	"finance-api/internal/config"
	monobank_dto "finance-api/internal/dto/monobank"
	"finance-api/internal/models"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

// MonobankWebhookCheck handles verification pings from Monobank (GET).
func MonobankWebhookCheck(c *gin.Context) {
	cfg := c.MustGet("config").(*config.Config)
	if !verifyWebhookSecret(c, cfg) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized webhook token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// MonobankWebhookReceive handles live statement updates from Monobank (POST).
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

	// 2. Deterministic UUID calculation
	hashInput := fmt.Sprintf(
		"%s:%s:%d",
		account.ID.String(),
		item.ID,
		item.Time,
	)

	txID := uuid.NewHash(
		sha256.New(),
		uuid.NameSpaceURL,
		[]byte(hashInput),
		8,
	)

	externalID := item.ID
	occurredAt := time.Unix(item.Time, 0).UTC()

	transaction := models.Transaction{
		ID:                txID,
		AccountID:         account.ID,
		BankID:            models.BankMonobank,
		ExternalID:        &externalID,
		Amount:            item.Amount,
		Currency:          item.CurrencyCode,
		OperationAmount:   item.OperationAmount,
		OperationCurrency: item.CurrencyCode,
		Balance:           item.Balance,
		CashbackAmount:    item.CashbackAmount,
		CommissionRate:    item.CommissionRate,
		MCC:               item.MCC,
		Description:       item.Description,
		Comment:           item.Comment,
		Hold:              item.Hold,
		OccurredAt:        occurredAt,
	}

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
