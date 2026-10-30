package handlers_test

import (
	"encoding/json"
	"finance-api/internal/models"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/steinfletcher/apitest"
)

func TestMonobankWebhookCheck(t *testing.T) {
	apitest.New().
		Handler(router).
		Get("/api/v1/webhooks/monobank").
		Expect(t).
		Status(http.StatusOK).
		Assert(func(res *http.Response, _ *http.Request) error {
			var resp map[string]any
			if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
				return err
			}
			if resp["status"] != "ok" {
				return fmt.Errorf("expected status ok, got %v", resp["status"])
			}
			return nil
		}).
		End()
}

func TestMonobankWebhookReceivePing(t *testing.T) {
	// Monobank non-statement ping event on initial registration
	apitest.New().
		Handler(router).
		Post("/api/v1/webhooks/monobank").
		JSON(`{
			"type": "Ping",
			"data": {}
		}`).
		Expect(t).
		Status(http.StatusOK).
		End()
}

func TestMonobankWebhookReceiveTransactionAndBalanceUpdate(t *testing.T) {
	userID := createTestUser(t)

	// Create Monobank account with known ExternalID
	cardExternalID := fmt.Sprintf("mono_card_%d", time.Now().UnixNano())
	account := models.Account{
		ID:                uuid.Must(uuid.NewV7()),
		UserID:            uuid.Must(uuid.Parse(userID)),
		Bank:              models.BankMonobank,
		ExternalID:        cardExternalID,
		MonobankAccountID: cardExternalID,
		Name:              "Monobank Black",
		Currency:          980,
		Balance:           500000, // 5,000.00 UAH
	}
	testDB.Create(&account)

	monoTxID := fmt.Sprintf("tx_mono_%d", time.Now().UnixNano())
	nowUnix := time.Now().Unix()

	// 1. Send live webhook statement item (Initial Purchase with HOLD)
	webhookPayload := fmt.Sprintf(`{
		"type": "StatementItem",
		"data": {
			"account": "%s",
			"statementItem": {
				"id": "%s",
				"time": %d,
				"description": "Silpo Supermarket",
				"mcc": 5411,
				"originalMcc": 5411,
				"hold": true,
				"amount": -25050,
				"operationAmount": -25050,
				"currencyCode": 980,
				"commissionRate": 0,
				"cashbackAmount": 250,
				"balance": 474950,
				"comment": "Apple Pay"
			}
		}
	}`, cardExternalID, monoTxID, nowUnix)

	var resp struct {
		Status        string    `json:"status"`
		TransactionID uuid.UUID `json:"transaction_id"`
		AccountID     uuid.UUID `json:"account_id"`
	}

	apitest.New().
		Handler(router).
		Post("/api/v1/webhooks/monobank").
		JSON(webhookPayload).
		Expect(t).
		Status(http.StatusOK).
		Assert(func(res *http.Response, _ *http.Request) error {
			return json.NewDecoder(res.Body).Decode(&resp)
		}).
		End()

	if resp.Status != "ok" || resp.TransactionID == uuid.Nil {
		t.Fatalf("unexpected webhook response: %+v", resp)
	}

	// Verify transaction saved in database
	var tx models.Transaction
	if err := testDB.First(&tx, "id = ?", resp.TransactionID).Error; err != nil {
		t.Fatalf("transaction not found in DB: %v", err)
	}

	if tx.Amount != -25050 || *tx.ExternalID != monoTxID || !tx.Hold {
		t.Fatalf("transaction data mismatch: %+v", tx)
	}

	// Verify account balance updated in DB
	var updatedAccount models.Account
	testDB.First(&updatedAccount, "id = ?", account.ID)
	if updatedAccount.Balance != 474950 {
		t.Fatalf("expected account balance to update to 474950, got %d", updatedAccount.Balance)
	}

	// 2. Test Idempotency / Settlement (HOLD cleared, updated balance)
	settlePayload := fmt.Sprintf(`{
		"type": "StatementItem",
		"data": {
			"account": "%s",
			"statementItem": {
				"id": "%s",
				"time": %d,
				"description": "Silpo Supermarket",
				"mcc": 5411,
				"originalMcc": 5411,
				"hold": false,
				"amount": -25050,
				"operationAmount": -25050,
				"currencyCode": 980,
				"commissionRate": 0,
				"cashbackAmount": 250,
				"balance": 474950
			}
		}
	}`, cardExternalID, monoTxID, nowUnix)

	apitest.New().
		Handler(router).
		Post("/api/v1/webhooks/monobank").
		JSON(settlePayload).
		Expect(t).
		Status(http.StatusOK).
		End()

	// Verify hold is now false and no duplicate was created
	var count int64
	testDB.Model(&models.Transaction{}).Where("external_id = ?", monoTxID).Count(&count)
	if count != 1 {
		t.Fatalf("expected exactly 1 transaction, got %d (duplicate created!)", count)
	}

	testDB.First(&tx, "id = ?", resp.TransactionID)
	if tx.Hold != false {
		t.Fatalf("expected hold to be updated to false, got %v", tx.Hold)
	}
}

func TestMonobankWebhookAccountFallback(t *testing.T) {
	userID := createTestUser(t)

	// Create Monobank account with currency 980
	account := models.Account{
		ID:         uuid.Must(uuid.NewV7()),
		UserID:     uuid.Must(uuid.Parse(userID)),
		Bank:       models.BankMonobank,
		ExternalID: fmt.Sprintf("fallback_card_%d", time.Now().UnixNano()),
		Name:       "Monobank Default",
		Currency:   980,
		Balance:    100000,
	}
	testDB.Create(&account)

	// Webhook sent with account "0" (Monobank default account indicator)
	monoTxID := fmt.Sprintf("tx_fallback_%d", time.Now().UnixNano())
	nowUnix := time.Now().Unix()

	fallbackPayload := fmt.Sprintf(`{
		"type": "StatementItem",
		"data": {
			"account": "0",
			"statementItem": {
				"id": "%s",
				"time": %d,
				"description": "Taxi",
				"mcc": 4121,
				"amount": -15000,
				"currencyCode": 980,
				"balance": 85000
			}
		}
	}`, monoTxID, nowUnix)

	apitest.New().
		Handler(router).
		Post("/api/v1/webhooks/monobank").
		JSON(fallbackPayload).
		Expect(t).
		Status(http.StatusOK).
		End()
}

func TestMonobankWebhookAuthSecret(t *testing.T) {
	// Temporarily set secret
	testCfg.MonobankWebhookSecret = "secret-token-xyz-123"
	defer func() {
		testCfg.MonobankWebhookSecret = ""
	}()

	// 1. Request without secret -> 401
	apitest.New().
		Handler(router).
		Get("/api/v1/webhooks/monobank").
		Expect(t).
		Status(http.StatusUnauthorized).
		End()

	// 2. Request with query param ?token=... -> 200
	apitest.New().
		Handler(router).
		Get("/api/v1/webhooks/monobank").
		QueryParams(map[string]string{
			"token": "secret-token-xyz-123",
		}).
		Expect(t).
		Status(http.StatusOK).
		End()

	// 3. Request with header X-Webhook-Secret -> 200
	apitest.New().
		Handler(router).
		Get("/api/v1/webhooks/monobank").
		Header("X-Webhook-Secret", "secret-token-xyz-123").
		Expect(t).
		Status(http.StatusOK).
		End()
}
