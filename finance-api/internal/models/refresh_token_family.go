package models

import (
	"time"

	"github.com/google/uuid"
)

type RefreshTokenFamily struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index"`
	User      User      `gorm:"foreignKey:UserID"`
	CreatedAt time.Time
	RevokedAt *time.Time
}
