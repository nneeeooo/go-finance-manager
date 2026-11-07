package account_dto

import (
	"finance-api/internal/models"

	"github.com/google/uuid"
)

type GetAccountResponse struct {
	ID                uuid.UUID   `json:"id"`
	UserID            uuid.UUID   `json:"userId"`
	Bank              models.Bank `json:"bank"`
	ExternalID        string      `json:"external_id"`
	MonobankAccountID string      `json:"monobank_account_id,omitempty"`
	Name              string      `json:"name"`
	Currency          uint16      `json:"currency"`
	Balance           int64       `json:"balance"`      // own money, negative = owed
	CreditLimit       int64       `json:"credit_limit"` // 0 = debit card
}

// FromModel: the API view of an account
func FromModel(a models.Account) GetAccountResponse {
	return GetAccountResponse{
		ID: a.ID, UserID: a.UserID, Bank: a.Bank, ExternalID: a.ExternalID, MonobankAccountID: a.ExternalID,
		Name: a.Name, Currency: a.Currency, Balance: a.Balance, CreditLimit: a.CreditLimit,
	}
}
