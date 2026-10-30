package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Bank uint16

const (
	BankMonobank   Bank = 1
	BankPrivatBank Bank = 2
	BankCash       Bank = 3
	BankOther      Bank = 4
)

type Account struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`

	UserID uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	User   User      `gorm:"foreignKey:UserID" json:"-"`

	Bank              Bank   `gorm:"not null" json:"bank"`
	ExternalID        string `gorm:"type:varchar(128);index;not null;default:''" json:"external_id"`
	MonobankAccountID string `gorm:"type:text;not null;default:''" json:"monobank_account_id,omitempty"`

	Name string `gorm:"type:varchar(100);not null" json:"name"`

	Currency uint16 `gorm:"not null" json:"currency"` // ISO 4217 (980 = UAH)
	Balance  int64  `gorm:"not null;default:0" json:"balance"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (a *Account) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.Must(uuid.NewV7())
	}
	if a.ExternalID == "" && a.MonobankAccountID != "" {
		a.ExternalID = a.MonobankAccountID
	}
	if a.MonobankAccountID == "" && a.ExternalID != "" {
		a.MonobankAccountID = a.ExternalID
	}
	return nil
}
