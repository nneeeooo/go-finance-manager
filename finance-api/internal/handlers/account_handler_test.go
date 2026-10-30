package handlers_test

import (
	"encoding/json"
	account_dto "finance-api/internal/dto/account"
	transaction_dto "finance-api/internal/dto/transaction"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/steinfletcher/apitest"
)

func TestCreateAccount(t *testing.T) {
	userID := createTestUser(t)

	apitest.New().
		Handler(router).
		Post("/api/v1/accounts").
		Header("Authorization", authHeaderForUser(t, userID)).
		JSON(fmt.Sprintf(`{
			"user_id":"%s",
			"bank":1,
			"name":"Monobank",
			"currency":980
		}`, userID)).
		Expect(t).
		Status(http.StatusCreated).
		Assert(func(res *http.Response, _ *http.Request) error {
			body, _ := io.ReadAll(res.Body)
			t.Logf("response: %s", body)
			return nil
		}).
		End()
}

func TestCreateAccountErrors(t *testing.T) {
	userID := createTestUser(t)

	tests := []struct {
		name       string
		body       string
		statusCode int
	}{
		{
			name:       "invalid json",
			body:       `{"user_id":`,
			statusCode: http.StatusBadRequest,
		},
		{
			name: "missing bank",
			body: fmt.Sprintf(`{
				"name":"Mono",
				"currency":980
			}`),
			statusCode: http.StatusBadRequest,
		},
		{
			name: "missing name",
			body: fmt.Sprintf(`{
				"bank":1,
				"currency":980
			}`),
			statusCode: http.StatusBadRequest,
		},
		{
			name: "missing currency",
			body: fmt.Sprintf(`{
				"bank":1,
				"name":"Mono"
			}`),
			statusCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apitest.New().
				Handler(router).
				Post("/api/v1/accounts").
				Header("Authorization", authHeaderForUser(t, userID)).
				JSON(tt.body).
				Expect(t).
				Status(tt.statusCode).
				End()
		})
	}
}

func TestGetAccountTransactions(t *testing.T) {
	userID := createTestUser(t)

	accountID := createTestAccountForUser(t, userID)
	otherAccountID := createTestAccountForUser(t, userID)

	createTestTransaction(t, userID, accountID)
	createTestTransaction(t, userID, accountID)
	createTestTransaction(t, userID, otherAccountID)

	apitest.New().
		Handler(router).
		Get("/api/v1/accounts/" + accountID + "/transactions").
		Header("Authorization", authHeaderForUser(t, userID)).
		Expect(t).
		Status(http.StatusOK).
		Assert(func(res *http.Response, _ *http.Request) error {
			var response []transaction_dto.TransactionResponse

			if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
				return err
			}

			if len(response) != 2 {
				return fmt.Errorf("expected 2 transactions, got %d", len(response))
			}

			for _, transaction := range response {
				if transaction.AccountID.String() != accountID {
					return fmt.Errorf(
						"expected transaction to belong to account %s, got %s",
						accountID,
						transaction.AccountID,
					)
				}
			}

			return nil
		}).
		End()
}

func TestGetAccountTransactionsErrors(t *testing.T) {
	userID := createTestUser(t)

	t.Run("invalid account ID", func(t *testing.T) {
		apitest.New().
			Handler(router).
			Get("/api/v1/accounts/not-a-uuid/transactions").
			Header("Authorization", authHeaderForUser(t, userID)).
			Expect(t).
			Status(http.StatusBadRequest).
			End()
	})

	t.Run("account not found", func(t *testing.T) {
		apitest.New().
			Handler(router).
			Get("/api/v1/accounts/" + uuid.New().String() + "/transactions").
			Header("Authorization", authHeaderForUser(t, userID)).
			Expect(t).
			Status(http.StatusNotFound).
			End()
	})
}

func TestGetAccount(t *testing.T) {
	userID := createTestUser(t)
	accountID := createTestAccountForUser(t, userID)

	apitest.New().
		Handler(router).
		Get("/api/v1/accounts/" + accountID).
		Header("Authorization", authHeaderForUser(t, userID)).
		Expect(t).
		Status(http.StatusOK).
		Assert(func(res *http.Response, _ *http.Request) error {
			var resp account_dto.GetAccountResponse
			if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
				return err
			}
			if resp.ID.String() != accountID {
				return fmt.Errorf("expected account ID %s, got %s", accountID, resp.ID)
			}
			if resp.Name != "Monobank" {
				return fmt.Errorf("expected Name Monobank, got %s", resp.Name)
			}
			return nil
		}).
		End()
}

func TestGetAccountErrors(t *testing.T) {
	userID := createTestUser(t)

	t.Run("invalid account ID", func(t *testing.T) {
		apitest.New().
			Handler(router).
			Get("/api/v1/accounts/not-a-uuid").
			Header("Authorization", authHeaderForUser(t, userID)).
			Expect(t).
			Status(http.StatusBadRequest).
			End()
	})

	t.Run("account not found", func(t *testing.T) {
		apitest.New().
			Handler(router).
			Get("/api/v1/accounts/" + uuid.New().String()).
			Header("Authorization", authHeaderForUser(t, userID)).
			Expect(t).
			Status(http.StatusNotFound).
			End()
	})
}

func TestDeleteAccount(t *testing.T) {
	userID := createTestUser(t)
	accountID := createTestAccountForUser(t, userID)

	apitest.New().
		Handler(router).
		Delete("/api/v1/accounts/" + accountID).
		Header("Authorization", authHeaderForUser(t, userID)).
		Expect(t).
		Status(http.StatusNoContent).
		End()

	// Verify account is removed and not returned in list
	apitest.New().
		Handler(router).
		Get("/api/v1/accounts").
		Header("Authorization", authHeaderForUser(t, userID)).
		Expect(t).
		Status(http.StatusOK).
		Assert(func(res *http.Response, _ *http.Request) error {
			var accounts []account_dto.GetAccountResponse
			if err := json.NewDecoder(res.Body).Decode(&accounts); err != nil {
				return err
			}
			for _, acc := range accounts {
				if acc.ID.String() == accountID {
					return fmt.Errorf("deleted account %s still present in GET /accounts", accountID)
				}
			}
			return nil
		}).
		End()
}

func TestCreateAccountsAllSupportedBanks(t *testing.T) {
	userID := createTestUser(t)

	banks := []struct {
		id   int
		name string
	}{
		{id: 1, name: "Monobank Black"},
		{id: 2, name: "PrivatBank Universal"},
		{id: 3, name: "PUMB Debit"},
		{id: 4, name: "Sense Bank Caméléon"},
	}

	for _, b := range banks {
		t.Run(b.name, func(t *testing.T) {
			apitest.New().
				Handler(router).
				Post("/api/v1/accounts").
				Header("Authorization", authHeaderForUser(t, userID)).
				JSON(fmt.Sprintf(`{
					"bank":%d,
					"name":"%s",
					"currency":980,
					"balance":100000
				}`, b.id, b.name)).
				Expect(t).
				Status(http.StatusCreated).
				End()
		})
	}
}

func TestDeleteAccountErrors(t *testing.T) {
	userID := createTestUser(t)

	t.Run("invalid account ID", func(t *testing.T) {
		apitest.New().
			Handler(router).
			Delete("/api/v1/accounts/not-a-uuid").
			Header("Authorization", authHeaderForUser(t, userID)).
			Expect(t).
			Status(http.StatusBadRequest).
			End()
	})

	t.Run("account not found", func(t *testing.T) {
		accountID := uuid.New()

		apitest.New().
			Handler(router).
			Delete("/api/v1/accounts/" + accountID.String()).
			Header("Authorization", authHeaderForUser(t, userID)).
			Expect(t).
			Status(http.StatusNotFound).
			End()
	})
}
