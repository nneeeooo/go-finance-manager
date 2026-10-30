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
	Balance           int64       `json:"balance"`
}
