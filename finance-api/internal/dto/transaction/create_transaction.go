package transaction_dto

import (
	"time"

	"github.com/google/uuid"
)

type CreateTransactionRequest struct {
	ID                uuid.UUID `json:"id"`
	AccountID         uuid.UUID `json:"account_id" validate:"required"`
	BankID            uint16    `json:"bank_id" validate:"required"`
	ExternalID        *string   `json:"external_id,omitempty"`
	Amount            int64     `json:"amount" validate:"required"`
	Currency          uint16    `json:"currency" validate:"required"`
	OperationAmount   int64     `json:"operation_amount"`
	OperationCurrency uint16    `json:"operation_currency"`
	Balance           int64     `json:"balance"`
	CashbackAmount    int64     `json:"cashback_amount"`
	CommissionRate    int64     `json:"commission_rate"`
	MCC               uint16    `json:"mcc"`
	Category          string    `json:"category"`
	Description       string    `json:"description" validate:"required"`
	Comment           string    `json:"comment"`
	Hold              bool      `json:"hold"`
	OccurredAt        time.Time `json:"occurred_at" validate:"required"`
}

type CreateTransactionResponse struct {
	ID uuid.UUID `json:"id"`
}
