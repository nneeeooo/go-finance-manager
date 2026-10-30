package account_dto

import (
	"finance-api/internal/models"

	"github.com/google/uuid"
)

type CreateAccountRequest struct {
	UserID            uuid.UUID   `json:"user_id,omitempty"`
	Bank              models.Bank `json:"bank" validate:"oneof=1 2"`
	MonobankAccountID string      `json:"monobank_account_id"`
	Name              string      `json:"name" validate:"required"`
	Currency          uint16      `json:"currency" validate:"required"`
}

type CreateAccountResponse struct {
	ID uuid.UUID `json:"id"`
}
