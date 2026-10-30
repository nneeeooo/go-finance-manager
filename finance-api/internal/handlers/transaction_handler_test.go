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

// TODO: write test to check duplicate transactions import

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
	t.Run("invalid user id", func(t *testing.T) {
		apitest.New().
			Handler(router).
			Get("/api/v1/users/not-a-uuid/transactions").
			Expect(t).
			Status(http.StatusBadRequest).
			End()
	})
}

func TestDeleteTransaction(t *testing.T) {
	userID := createTestUser(t)
	accountID := createTestAccountForUser(t, userID)
	transactionID := createTestTransaction(t, userID, accountID)

	apitest.New().
		Handler(router).
		Delete("/api/v1/transactions/" + transactionID).
		Expect(t).
		Status(http.StatusNoContent).
		End()
}

func TestDeleteTransactionErrors(t *testing.T) {
	t.Run("invalid transaction ID", func(t *testing.T) {
		apitest.New().
			Handler(router).
			Delete("/api/v1/transactions/not-a-uuid").
			Expect(t).
			Status(http.StatusBadRequest).
			End()
	})

	t.Run("transaction not found", func(t *testing.T) {
		transactionID := uuid.New()

		apitest.New().
			Handler(router).
			Delete("/api/v1/transactions/" + transactionID.String()).
			Expect(t).
			Status(http.StatusNotFound).
			End()
	})
}
