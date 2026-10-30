package transaction_dto

import (
	"time"

	"github.com/google/uuid"
)

type TransactionResponse struct {
	ID          uuid.UUID `json:"id" validate:"required"`
	AccountID   uuid.UUID `json:"account_id" validate:"required"`
	BankID      uint16    `json:"bank_id" validate:"required"`
	Amount      int64     `json:"amount" validate:"required"`
	Currency    uint16    `json:"currency" validate:"required"`
	MCC         uint16    `json:"mcc" validate:"required"`
	Description string    `json:"description" validate:"required"`
	OccurredAt  time.Time `json:"occurred_at" validate:"required"`
}
