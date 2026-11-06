package handlers_test

import (
	"crypto/sha256"
	"encoding/json"
	dto "finance-api/internal/dto/transaction"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/steinfletcher/apitest"
)

func TestCreateTransaction(t *testing.T) {
	userID := createTestUser(t)
	accountID := createTestAccountForUser(t, userID)

	amount := 15000
	currency := 980
	mcc := 5411
	description := "Test transaction"
	occurredAt := time.Now().UTC().Format(time.RFC3339)

	data := fmt.Sprintf(
		"%s:%s:%d:%d:%d:%s",
		accountID,
		occurredAt,
		amount,
		currency,
		mcc,
		description,
	)

	transactionID := uuid.NewHash(
		sha256.New(),
		uuid.NameSpaceURL,
		[]byte(data),
		8,
	)

	apitest.New().
		Handler(router).
		Post("/api/v1/transactions").
		Header("Authorization", authHeaderForUser(t, userID)).
		JSON(fmt.Sprintf(`{
			"id":"%s",
			"account_id":"%s",
			"bank_id":1,
			"amount":%d,
			"currency":%d,
			"mcc":%d,
			"description":"%s",
			"occurred_at":"%s"
		}`,
			transactionID,
			accountID,
			amount,
			currency,
			mcc,
			description,
			occurredAt,
		)).
		Expect(t).
		Status(http.StatusCreated).
		End()
}

func TestCreateTransactionErrors(t *testing.T) {
	userID := createTestUser(t)
	accountID := createTestAccountForUser(t, userID)

	tests := []struct {
		name   string
		body   string
		status int
	}{
		{
			name:   "invalid json",
			body:   `{"account_id":`,
			status: http.StatusBadRequest,
		},
		{
			name: "missing id",
			body: fmt.Sprintf(`{
				"account_id":"%s",
				"bank_id":1,
				"amount":15000,
				"currency":980,
				"mcc":5411,
				"description":"Test transaction",
				"occurred_at":"%s"
			}`, accountID, time.Now().UTC().Format(time.RFC3339)),
			status: http.StatusBadRequest,
		},
		{
			name: "missing account id",
			body: fmt.Sprintf(`{
				"id":"%s",
				"bank_id":1,
				"amount":15000,
				"currency":980,
				"mcc":5411,
				"description":"Test transaction",
				"occurred_at":"%s"
			}`,
				uuid.New(),
				time.Now().UTC().Format(time.RFC3339),
			),
			status: http.StatusBadRequest,
		},
		{
			name: "missing amount",
			body: fmt.Sprintf(`{
				"id":"%s",
				"account_id":"%s",
				"bank_id":1,
				"currency":980,
				"mcc":5411,
				"description":"Test transaction",
				"occurred_at":"%s"
			}`,
				uuid.New(),
				accountID,
				time.Now().UTC().Format(time.RFC3339),
			),
			status: http.StatusBadRequest,
		},
		{
			name: "invalid account id",
			body: fmt.Sprintf(`{
				"id":"%s",
				"account_id":"not-a-uuid",
				"bank_id":1,
				"amount":15000,
				"currency":980,
				"mcc":5411,
				"description":"Test transaction",
				"occurred_at":"%s"
			}`,
				uuid.New(),
				time.Now().UTC().Format(time.RFC3339),
			),
			status: http.StatusBadRequest,
		},
		{
			name: "invalid id",
			body: fmt.Sprintf(`{
				"id":"not-a-uuid",
				"account_id":"%s",
				"bank_id":1,
				"amount":15000,
				"currency":980,
				"mcc":5411,
				"description":"Test transaction",
				"occurred_at":"%s"
			}`,
				accountID,
				time.Now().UTC().Format(time.RFC3339),
			),
			status: http.StatusBadRequest,
		},
		{
			name: "missing occurred at",
			body: fmt.Sprintf(`{
				"id":"%s",
				"account_id":"%s",
				"bank_id":1,
				"amount":15000,
				"currency":980,
				"mcc":5411,
				"description":"Test transaction"
			}`,
				uuid.New(),
				accountID,
			),
			status: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apitest.New().
				Handler(router).
				Post("/api/v1/transactions").
				Header("Authorization", authHeaderForUser(t, userID)).
				JSON(tt.body).
				Expect(t).
				Status(tt.status).
				End()
		})
	}
}

func TestGetUserTransactions(t *testing.T) {
	userID := createTestUser(t)
	accountID := createTestAccountForUser(t, userID)

	createTestTransaction(t, userID, accountID)

	apitest.New().
		Handler(router).
		Get("/api/v1/transactions").
		Header("Authorization", authHeaderForUser(t, userID)).
		Expect(t).
		Status(http.StatusOK).
		Assert(func(res *http.Response, _ *http.Request) error {
			var transactions []dto.TransactionResponse

			if err := json.NewDecoder(res.Body).Decode(&transactions); err != nil {
				return err
			}

			if len(transactions) != 1 {
				return fmt.Errorf(
					"expected 1 transaction, got %d",
					len(transactions),
				)
			}

			if transactions[0].AccountID.String() != accountID {
				return fmt.Errorf(
					"expected account ID %s, got %s",
					accountID,
					transactions[0].AccountID,
				)
			}

			return nil
		}).
		End()
}

func TestGetUserTransactionsErrors(t *testing.T) {
	t.Run("unauthorized without token", func(t *testing.T) {
		apitest.New().
			Handler(router).
			Get("/api/v1/transactions").
			Expect(t).
			Status(http.StatusUnauthorized).
			End()
	})
}

func TestDeleteTransaction(t *testing.T) {
	userID := createTestUser(t)
	accountID := createTestAccountForUser(t, userID)
	transactionID := createTestTransaction(t, userID, accountID)

	apitest.New().
		Handler(router).
		Delete("/api/v1/transactions/"+transactionID).
		Header("Authorization", authHeaderForUser(t, userID)).
		Expect(t).
		Status(http.StatusNoContent).
		End()
}

func TestDeleteTransactionErrors(t *testing.T) {
	userID := createTestUser(t)

	t.Run("invalid transaction ID", func(t *testing.T) {
		apitest.New().
			Handler(router).
			Delete("/api/v1/transactions/not-a-uuid").
			Header("Authorization", authHeaderForUser(t, userID)).
			Expect(t).
			Status(http.StatusBadRequest).
			End()
	})

	t.Run("transaction not found", func(t *testing.T) {
		transactionID := uuid.New()

		apitest.New().
			Handler(router).
			Delete("/api/v1/transactions/"+transactionID.String()).
			Header("Authorization", authHeaderForUser(t, userID)).
			Expect(t).
			Status(http.StatusNotFound).
			End()
	})
}

func TestDuplicateTransactionConflict(t *testing.T) {
	userID := createTestUser(t)
	accountID := createTestAccountForUser(t, userID)

	txID := uuid.New()
	occurredAt := time.Now().UTC().Format(time.RFC3339)

	body := fmt.Sprintf(`{
		"id":"%s",
		"account_id":"%s",
		"bank_id":1,
		"amount":-4500,
		"currency":980,
		"mcc":5411,
		"description":"Silpo supermarket",
		"occurred_at":"%s"
	}`, txID, accountID, occurredAt)

	// First insert -> 201 Created
	apitest.New().
		Handler(router).
		Post("/api/v1/transactions").
		Header("Authorization", authHeaderForUser(t, userID)).
		JSON(body).
		Expect(t).
		Status(http.StatusCreated).
		End()

	// Second insert with identical ID -> 409 Conflict
	apitest.New().
		Handler(router).
		Post("/api/v1/transactions").
		Header("Authorization", authHeaderForUser(t, userID)).
		JSON(body).
		Expect(t).
		Status(http.StatusConflict).
		End()
}

func TestTransactionSortingAndFieldsPreservation(t *testing.T) {
	userID := createTestUser(t)
	accountID := createTestAccountForUser(t, userID)

	now := time.Now().UTC().Truncate(time.Second)

	// Create 3 transactions at different timestamps
	timestamps := []time.Time{
		now.Add(-2 * time.Hour), // oldest
		now.Add(-1 * time.Hour), // middle
		now,                     // newest
	}

	for i, ts := range timestamps {
		apitest.New().
			Handler(router).
			Post("/api/v1/transactions").
			Header("Authorization", authHeaderForUser(t, userID)).
			JSON(fmt.Sprintf(`{
				"id":"%s",
				"account_id":"%s",
				"bank_id":2,
				"amount":%d,
				"currency":980,
				"operation_amount":%d,
				"operation_currency":980,
				"balance":%d,
				"cashback_amount":50,
				"commission_rate":10,
				"mcc":5411,
				"category":"Groceries",
				"description":"Tx %d",
				"comment":"Test comment",
				"hold":false,
				"occurred_at":"%s"
			}`, uuid.New(), accountID, (i+1)*1000, (i+1)*1000, 100000+(i*1000), i, ts.Format(time.RFC3339))).
			Expect(t).
			Status(http.StatusCreated).
			End()
	}

	// Fetch all transactions -> must be ordered by occurred_at DESC
	apitest.New().
		Handler(router).
		Get("/api/v1/transactions").
		Header("Authorization", authHeaderForUser(t, userID)).
		Expect(t).
		Status(http.StatusOK).
		Assert(func(res *http.Response, _ *http.Request) error {
			var txs []dto.TransactionResponse
			if err := json.NewDecoder(res.Body).Decode(&txs); err != nil {
				return err
			}
			if len(txs) != 3 {
				return fmt.Errorf("expected 3 transactions, got %d", len(txs))
			}

			// Verify DESC order: [0] newest, [1] middle, [2] oldest
			if !txs[0].OccurredAt.After(txs[1].OccurredAt) || !txs[1].OccurredAt.After(txs[2].OccurredAt) {
				return fmt.Errorf("transactions not ordered by occurred_at DESC: %v, %v, %v",
					txs[0].OccurredAt, txs[1].OccurredAt, txs[2].OccurredAt)
			}

			// Verify fields preservation
			newest := txs[0]
			if newest.BankID != 2 {
				return fmt.Errorf("expected BankID 2, got %d", newest.BankID)
			}
			if newest.Category != "Groceries" {
				return fmt.Errorf("expected Category Groceries, got %s", newest.Category)
			}
			if newest.CashbackAmount != 50 {
				return fmt.Errorf("expected CashbackAmount 50, got %d", newest.CashbackAmount)
			}
			if newest.CommissionRate != 10 {
				return fmt.Errorf("expected CommissionRate 10, got %d", newest.CommissionRate)
			}
			return nil
		}).
		End()
}
