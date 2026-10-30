package account_dto

import (
	"finance-api/internal/models"

	"github.com/google/uuid"
)

type GetAccountResponse struct {
	ID                uuid.UUID   `json:"id"`
	UserID            uuid.UUID   `json:"userId"`
	Bank              models.Bank `json:"bank"`
	MonobankAccountID string      `json:"monobank_account_id"`
	Name              string      `json:"name"`
	Currency          uint16      `json:"currency"`
}
