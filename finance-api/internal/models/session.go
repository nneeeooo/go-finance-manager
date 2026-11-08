package models

import (
	"time"

	"github.com/google/uuid"
)

// Session: a browser login through the central auth service (SSO). The browser holds only a random token (the
// fin_session cookie; its SHA-256 is stored here). The auth service's refresh token stays on the server.
type Session struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey"`

	TokenHash []byte    `gorm:"type:bytea;uniqueIndex;not null"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index"`
	User      User      `gorm:"foreignKey:UserID"`

	// Refresh token of the auth service (rotated on every refresh)
	RefreshToken string `gorm:"type:text;not null"`
	// When the auth service's access token expires; past it the next request refreshes it
	AccessTokenExpiresAt time.Time `gorm:"not null"`

	ExpiresAt time.Time `gorm:"not null;index"`
	CreatedAt time.Time
	UpdatedAt time.Time
}
