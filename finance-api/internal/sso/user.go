package sso

import (
	"errors"
	"strings"
	"time"

	"finance-api/internal/models"

	"gorm.io/gorm"
)

// ErrNotInvited: the email is verified but neither linked to a user nor on the allow-list
var ErrNotInvited = errors.New("email is not on the allow-list")

// ResolveUser finds the app user for a verified auth identity:
//  1. by the stored auth subject;
//  2. else by the verified email: an existing user is linked (auth_sub set, all data kept);
//  3. else the email's allow-list entry creates the user (a removed user with that email comes back);
//  4. else ErrNotInvited.
func ResolveUser(db *gorm.DB, id *Identity) (*models.User, error) {
	if id.Subject == "" || id.Email == "" || !id.EmailVerified {
		return nil, errors.New("identity without subject or verified email")
	}
	var user models.User
	err := db.Transaction(func(tx *gorm.DB) error {
		// 1. known subject
		err := tx.First(&user, "auth_sub = ?", id.Subject).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		// 2. existing user with this email
		err = tx.First(&user, "LOWER(email) = ?", id.Email).Error
		if err == nil {
			if user.AuthSub != nil && *user.AuthSub != id.Subject {
				return errors.New("user is linked to another auth subject")
			}
			sub := id.Subject
			if err := tx.Model(&user).Update("auth_sub", sub).Error; err != nil {
				return err
			}
			user.AuthSub = &sub
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		// 3. allow-list
		var allowed models.AllowedEmail
		if err := tx.First(&allowed, "LOWER(email) = ?", id.Email).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotInvited
			}
			return err
		}
		sub := id.Subject
		now := time.Now()
		var removed models.User
		if err := tx.Unscoped().Where("LOWER(email) = ? AND deleted_at IS NOT NULL", id.Email).First(&removed).Error; err == nil {
			if err := tx.Unscoped().Model(&removed).Updates(map[string]any{
				"auth_sub": sub, "role": allowed.Role, "deleted_at": nil,
			}).Error; err != nil {
				return err
			}
			user = removed
			user.AuthSub, user.Role = &sub, allowed.Role
		} else {
			// no password: "!" is not a valid hash, so password login can never match
			user = models.User{Email: strings.ToLower(id.Email), PasswordHash: "!", Role: allowed.Role, AuthSub: &sub}
			if err := tx.Create(&user).Error; err != nil {
				return err
			}
		}
		return tx.Model(&allowed).Update("registered_at", &now).Error
	})
	if err != nil {
		return nil, err
	}
	return &user, nil
}
