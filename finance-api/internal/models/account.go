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
)

type Account struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey"`

	UserID uuid.UUID `gorm:"type:uuid;not null;index"`
	User   User      `gorm:"foreignKey:UserID"`

	Bank              Bank   `gorm:"not null"`
	MonobankAccountID string `gorm:"type:text;not null;default:''"`

	Name string `gorm:"type:text;not null"`

	Currency uint16 `gorm:"not null"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (a *Account) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.Must(uuid.NewV7())
	}
	return nil
}
