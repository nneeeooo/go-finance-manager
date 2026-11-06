package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AllowedEmail struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`

	Email string `gorm:"type:varchar(320);uniqueIndex;not null" json:"email"`

	// Role the user gets on registration; kept in step with users.role
	Role Role `gorm:"type:varchar(16);not null;default:'regular'" json:"role"`

	RegisteredAt *time.Time `json:"registered_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (a *AllowedEmail) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.Must(uuid.NewV7())
	}
	return nil
}
