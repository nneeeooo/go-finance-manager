package models

import (
	"time"

	"github.com/google/uuid"
)

type RefreshToken struct {
	ID                   uuid.UUID          `gorm:"type:uuid;primaryKey"`
	RefreshTokenFamilyID uuid.UUID          `gorm:"type:uuid;not null;index"`
	RefreshTokenFamily   RefreshTokenFamily `gorm:"foreignKey:RefreshTokenFamilyID"`
	TokenHash            []byte             `gorm:"type:bytea;uniqueIndex;not null"`
	ExpiresAt            time.Time          `gorm:"not null"`
	RevokedAt            *time.Time
	ReplacedByID         *uuid.UUID
	CreatedAt            time.Time
}
