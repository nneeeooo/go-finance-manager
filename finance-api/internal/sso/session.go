package sso

import (
	"context"
	"errors"
	"time"

	"finance-api/internal/models"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	// SessionTTL: absolute lifetime of an app session (the auth service's refresh family lives 60 days)
	SessionTTL = 60 * 24 * time.Hour
	// refreshLeeway: refresh the access token a little before it expires
	refreshLeeway = 30 * time.Second
)

// ErrNoSession: no such session, expired, or the auth service ended it (invalid_grant)
var ErrNoSession = errors.New("no session")

// CreateSession stores a new session for the user and returns the raw cookie token
func CreateSession(db *gorm.DB, userID uuid.UUID, tok *oauth2.Token) (string, error) {
	raw, err := RandomToken()
	if err != nil {
		return "", err
	}
	s := models.Session{
		ID:                   uuid.Must(uuid.NewV7()),
		TokenHash:            hashToken(raw),
		UserID:               userID,
		RefreshToken:         tok.RefreshToken,
		AccessTokenExpiresAt: tok.Expiry,
		ExpiresAt:            time.Now().Add(SessionTTL),
	}
	if err := db.Create(&s).Error; err != nil {
		return "", err
	}
	// housekeeping: expired sessions
	db.Delete(&models.Session{}, "expires_at < ?", time.Now())
	return raw, nil
}

// DeleteSession removes the session of the cookie token and returns it (nil when there was none)
func DeleteSession(db *gorm.DB, raw string) *models.Session {
	var s models.Session
	if err := db.First(&s, "token_hash = ?", hashToken(raw)).Error; err != nil {
		return nil
	}
	db.Delete(&s)
	return &s
}

// Authenticate finds the live session of the cookie token and, when the access token has expired, refreshes it
// server-side. The refresh runs under a row lock, so parallel requests refresh once (the auth service rotates the
// refresh token, and reusing an old one would kill the whole family). invalid_grant drops the session.
func (c *Client) Authenticate(ctx context.Context, db *gorm.DB, raw string) (uuid.UUID, error) {
	var s models.Session
	if err := db.First(&s, "token_hash = ? AND expires_at > ?", hashToken(raw), time.Now()).Error; err != nil {
		return uuid.Nil, ErrNoSession
	}
	if s.AccessTokenExpiresAt.After(time.Now().Add(refreshLeeway)) {
		return s.UserID, nil
	}

	dropped := false
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&s, "id = ?", s.ID).Error; err != nil {
			return ErrNoSession
		}
		if s.AccessTokenExpiresAt.After(time.Now().Add(refreshLeeway)) {
			return nil // a parallel request refreshed it already
		}
		tok, err := c.Refresh(ctx, s.RefreshToken)
		if errors.Is(err, ErrInvalidGrant) {
			dropped = true
			return tx.Delete(&s).Error
		}
		if err != nil {
			return err
		}
		upd := map[string]any{"access_token_expires_at": tok.Expiry}
		if tok.RefreshToken != "" {
			upd["refresh_token"] = tok.RefreshToken
		}
		return tx.Model(&s).Updates(upd).Error
	})
	switch {
	case dropped, errors.Is(err, ErrNoSession):
		return uuid.Nil, ErrNoSession
	case err != nil:
		return uuid.Nil, err
	}
	return s.UserID, nil
}
