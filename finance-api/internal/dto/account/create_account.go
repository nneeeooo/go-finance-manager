package account_dto

import (
	"finance-api/internal/models"

	"github.com/google/uuid"
)

type CreateAccountRequest struct {
	UserID            uuid.UUID   `json:"user_id,omitempty"`
	Bank              models.Bank `json:"bank" validate:"required"`
	ExternalID        string      `json:"external_id"`
	MonobankAccountID string      `json:"monobank_account_id,omitempty"`
	Name              string      `json:"name" validate:"required"`
	Currency          uint16      `json:"currency" validate:"required"`
	Balance           int64       `json:"balance"`                       // own money, negative = owed
	CreditLimit       int64       `json:"credit_limit" validate:"min=0"` // 0 = debit card
}

type CreateAccountResponse struct {
	ID uuid.UUID `json:"id"`
}

// UpdateAccountRequest: nil = unchanged. The bank of an account does not change.
type UpdateAccountRequest struct {
	Name        *string `json:"name" validate:"omitempty,min=1,max=100"`
	ExternalID  *string `json:"external_id" validate:"omitempty,max=128"`
	Currency    *uint16 `json:"currency"`
	Balance     *int64  `json:"balance"`
	CreditLimit *int64  `json:"credit_limit" validate:"omitempty,min=0"`
}
