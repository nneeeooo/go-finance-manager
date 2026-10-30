package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Transaction struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey"`

	AccountID uuid.UUID `gorm:"type:uuid;not null;index"`
	Account   Account   `gorm:"foreignKey:AccountID"`

	BankID Bank `gorm:"not null"`

	Amount int64 `gorm:"not null"`

	Currency uint16 `gorm:"not null"`

	MCC uint16 `gorm:"not null"`

	Description string `gorm:"type:text;not null;default:''"`

	OccurredAt time.Time `gorm:"not null;index"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}
