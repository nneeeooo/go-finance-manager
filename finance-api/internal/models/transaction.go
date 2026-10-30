package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Transaction struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`

	AccountID uuid.UUID `gorm:"type:uuid;not null;index:idx_account_occurred,priority:1;index:idx_account_external,priority:1" json:"account_id"`
	Account   Account   `gorm:"foreignKey:AccountID" json:"-"`

	BankID Bank `gorm:"not null;index" json:"bank_id"`

	ExternalID *string `gorm:"type:varchar(128);index:idx_account_external,priority:2" json:"external_id,omitempty"`

	// Amounts in minor units (e.g. cents / kopecks)
	Amount            int64  `gorm:"not null" json:"amount"`
	Currency          uint16 `gorm:"not null" json:"currency"`
	OperationAmount   int64  `gorm:"not null;default:0" json:"operation_amount"`
	OperationCurrency uint16 `gorm:"not null;default:0" json:"operation_currency"`

	Balance        int64 `gorm:"not null;default:0" json:"balance"`
	CashbackAmount int64 `gorm:"not null;default:0" json:"cashback_amount"`
	CommissionRate int64 `gorm:"not null;default:0" json:"commission_rate"`

	MCC         uint16 `gorm:"not null;index" json:"mcc"`
	Category    string `gorm:"type:varchar(128);not null;default:''" json:"category"`
	Description string `gorm:"type:text;not null;default:''" json:"description"`
	Comment     string `gorm:"type:text;not null;default:''" json:"comment"`
	Hold        bool   `gorm:"not null;default:false" json:"hold"`

	OccurredAt time.Time      `gorm:"not null;index:idx_account_occurred,priority:2" json:"occurred_at"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

func (t *Transaction) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.Must(uuid.NewV7())
	}
	return nil
}
