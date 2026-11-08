package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey"`

	Email string `gorm:"type:varchar(320);uniqueIndex;not null"`

	PasswordHash string `gorm:"type:text;not null"`

	// Subject of the central auth service (SSO); set when the user first logs in through it
	AuthSub *string `gorm:"type:varchar(64);uniqueIndex"`

	Role Role `gorm:"type:varchar(16);not null;default:'regular'"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.Must(uuid.NewV7())
	}
	return nil
}
