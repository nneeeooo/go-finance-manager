package handlers_test

import (
	"crypto/sha256"
	"encoding/json"
	account_dto "finance-api/internal/dto/account"
	dto "finance-api/internal/dto/transaction"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/steinfletcher/apitest"
)

func TestMultiTenancyAndIDORIsolation(t *testing.T) {
	// Create User A and User B
	userA := createTestUser(t)
	userB := createTestUser(t)

	// User A creates Account A
	accountA := createTestAccountForUser(t, userA)

	// User B creates Account B
	accountB := createTestAccountForUser(t, userB)

	// 1. User A lists accounts: must contain ONLY account A, NEVER account B
	apitest.New().
		Handler(router).
		Get("/api/v1/accounts").
		Header("Authorization", authHeaderForUser(t, userA)).
		Expect(t).
		Status(http.StatusOK).
		Assert(func(res *http.Response, _ *http.Request) error {
			var accounts []account_dto.GetAccountResponse
			if err := json.NewDecoder(res.Body).Decode(&accounts); err != nil {
				return err
			}
			for _, acc := range accounts {
				if acc.ID.String() == accountB {
					return fmt.Errorf("LEAK: User A received User B's account %s", accountB)
				}
				if acc.UserID.String() != userA {
					return fmt.Errorf("LEAK: User A received account of user %s", acc.UserID)
				}
			}
			return nil
		}).
		End()

	// 2. User B lists accounts: must contain ONLY account B, NEVER account A
	apitest.New().
		Handler(router).
		Get("/api/v1/accounts").
		Header("Authorization", authHeaderForUser(t, userB)).
		Expect(t).
		Status(http.StatusOK).
		Assert(func(res *http.Response, _ *http.Request) error {
			var accounts []account_dto.GetAccountResponse
			if err := json.NewDecoder(res.Body).Decode(&accounts); err != nil {
				return err
			}
			for _, acc := range accounts {
				if acc.ID.String() == accountA {
					return fmt.Errorf("LEAK: User B received User A's account %s", accountA)
				}
				if acc.UserID.String() != userB {
					return fmt.Errorf("LEAK: User B received account of user %s", acc.UserID)
				}
			}
			return nil
		}).
		End()

	// 3. Transactions creation
	txA := createTestTransaction(t, userA, accountA)
	txB := createTestTransaction(t, userB, accountB)

	// 4. User A lists transactions: must see txA, NEVER txB
	apitest.New().
		Handler(router).
		Get("/api/v1/transactions").
		Header("Authorization", authHeaderForUser(t, userA)).
		Expect(t).
		Status(http.StatusOK).
		Assert(func(res *http.Response, _ *http.Request) error {
			var txs []dto.TransactionResponse
			if err := json.NewDecoder(res.Body).Decode(&txs); err != nil {
				return err
			}
			foundA := false
			for _, tx := range txs {
				if tx.ID.String() == txB {
					return fmt.Errorf("LEAK: User A can see User B's transaction %s", txB)
				}
				if tx.ID.String() == txA {
					foundA = true
				}
			}
			if !foundA {
				return fmt.Errorf("User A did not see own transaction %s", txA)
			}
			return nil
		}).
		End()

	// 5. IDOR Attack: User A tries to insert transaction into User B's account -> 403 Forbidden
	occurredAt := time.Now().UTC().Format(time.RFC3339)
	idorTxID := uuid.NewHash(sha256.New(), uuid.NameSpaceURL, []byte(fmt.Sprintf("%s:%s", accountB, occurredAt)), 8)

	apitest.New().
		Handler(router).
		Post("/api/v1/transactions").
		Header("Authorization", authHeaderForUser(t, userA)). // Authenticated as User A
		JSON(fmt.Sprintf(`{
			"id":"%s",
			"account_id":"%s",
			"bank_id":1,
			"amount":10000,
			"currency":980,
			"mcc":5411,
			"description":"IDOR Attack",
			"occurred_at":"%s"
		}`, idorTxID, accountB, occurredAt)). // Targeting Account B
		Expect(t).
		Status(http.StatusForbidden).
		End()

	// 6. IDOR Attack: User A tries to view transactions of User B's account -> 404 Not Found (doesn't leak account existence)
	apitest.New().
		Handler(router).
		Get("/api/v1/accounts/" + accountB + "/transactions").
		Header("Authorization", authHeaderForUser(t, userA)). // Authenticated as User A
		Expect(t).
		Status(http.StatusNotFound).
		End()
}
