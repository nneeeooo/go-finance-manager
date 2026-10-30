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
	Balance           int64       `json:"balance"`
}

type CreateAccountResponse struct {
	ID uuid.UUID `json:"id"`
}
