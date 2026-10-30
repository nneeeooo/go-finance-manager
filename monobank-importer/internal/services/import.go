package services

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"monobank-importer/internal/config"
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
)

type BankTransaction struct {
	ID              string `json:"id"`
	Time            int64  `json:"time"`
	Description     string `json:"description"`
	MCC             uint16 `json:"mcc"`
	OriginalMCC     uint16 `json:"originalMcc"`
	Amount          int64  `json:"amount"`
	OperationAmount int64  `json:"operationAmount"`
	CurrencyCode    uint16 `json:"currencyCode"`
	CommissionRate  int64  `json:"commissionRate"`
	CashbackAmount  int64  `json:"cashbackAmount"`
	Balance         int64  `json:"balance"`
	Hold            bool   `json:"hold"`
	ReceiptID       string `json:"receiptId"`
}

type CreateTransactionRequest struct {
	ID          uuid.UUID `json:"id"`
	AccountID   uuid.UUID `json:"account_id"`
	BankID      uint16    `json:"bank_id"`
	Amount      int64     `json:"amount"`
	Currency    uint16    `json:"currency"`
	MCC         uint16    `json:"mcc"`
	Description string    `json:"description"`
	OccurredAt  time.Time `json:"occurred_at"`
}

func ImportTransactions(accountID uuid.UUID, jsonData []byte) error {
	cfg := config.Load()

	// Parse transactions
	var transactions []BankTransaction

	if err := json.Unmarshal(jsonData, &transactions); err != nil {
		return fmt.Errorf("parse JSON: %w", err)
	}

	client := resty.New().
		SetBaseURL(fmt.Sprintf("http://%s:%s", cfg.AppHost, cfg.AppPort)).SetTimeout(30 * time.Second)

	for _, bankTransaction := range transactions {
		occurredAt := time.Unix(bankTransaction.Time, 0).UTC()

		// Generate deterministic transaction ID.
		hashInput := fmt.Sprintf(
			"%s:%s:%d",
			accountID.String(),
			bankTransaction.ID,
			bankTransaction.Time,
		)

		transactionID := uuid.NewHash(
			sha256.New(),
			uuid.NameSpaceURL,
			[]byte(hashInput),
			8,
		)

		request := CreateTransactionRequest{
			ID:          transactionID,
			AccountID:   accountID,
			BankID:      1, // 1 - monobank; 2 - privatbank
			Amount:      bankTransaction.Amount,
			Currency:    bankTransaction.CurrencyCode,
			MCC:         bankTransaction.MCC,
			Description: bankTransaction.Description,
			OccurredAt:  occurredAt,
		}

		_, err := client.R().
			SetHeader("Content-Type", "application/json").
			SetBody(request).
			Post("/api/v1/transactions")

		if err != nil {
			return fmt.Errorf(
				"send transaction %s: %w",
				bankTransaction.ID,
				err,
			)
		}
	}

	return nil
}

func CreateUserAndAccount() (uuid.UUID, error) {
	cfg := config.Load()

	client := resty.New().
		SetBaseURL(fmt.Sprintf("http://%s:%s", cfg.AppHost, cfg.AppPort))

	// Create user
	userResponse := struct {
		ID uuid.UUID `json:"id"`
	}{}

	resp, err := client.R().
		SetBody(map[string]string{
			"email":    "import@example.com",
			"password": "password123",
		}).
		SetResult(&userResponse).
		Post("/api/v1/users")

	if err != nil {
		return uuid.Nil, fmt.Errorf("create user: %w", err)
	}

	if resp.StatusCode() != http.StatusCreated {
		return uuid.Nil, fmt.Errorf(
			"create user: API returned %d",
			resp.StatusCode(),
		)
	}

	// Create account
	accountResponse := struct {
		ID uuid.UUID `json:"id"`
	}{}

	resp, err = client.R().
		SetBody(map[string]any{
			"user_id":  userResponse.ID,
			"bank":     1,
			"name":     "Monobank",
			"currency": 980,
		}).
		SetResult(&accountResponse).
		Post("/api/v1/accounts")

	if err != nil {
		return uuid.Nil, fmt.Errorf("create account: %w", err)
	}

	if resp.StatusCode() != http.StatusCreated {
		return uuid.Nil, fmt.Errorf(
			"create account: API returned %d",
			resp.StatusCode(),
		)
	}

	return accountResponse.ID, nil
}
