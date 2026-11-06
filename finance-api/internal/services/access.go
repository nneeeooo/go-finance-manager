package services

import (
	"finance-api/internal/models"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DefaultAllowedEmails: who may register on every install, and their role
var DefaultAllowedEmails = []struct {
	Email string
	Role  models.Role
}{
	{"lytvyn349@gmail.com", models.RoleAdmin},
	{"anastagoza@gmail.com", models.RoleRegular},
}

// SeedAccess adds the default allow-list entries and the comma-separated extra emails (regular) when missing;
// entries already there keep their role (the admin may have changed it). Then every registered user gets the
// role of their allow-list entry, which also promotes accounts that existed before roles.
func SeedAccess(db *gorm.DB, extraEmails string) error {
	entries := make([]models.AllowedEmail, 0, len(DefaultAllowedEmails))
	for _, d := range DefaultAllowedEmails {
		entries = append(entries, models.AllowedEmail{Email: d.Email, Role: d.Role})
	}
	for _, raw := range strings.Split(extraEmails, ",") {
		if email := strings.ToLower(strings.TrimSpace(raw)); email != "" {
			entries = append(entries, models.AllowedEmail{Email: email, Role: models.RoleRegular})
		}
	}
	for _, e := range entries {
		if err := db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "email"}}, DoNothing: true}).Create(&e).Error; err != nil {
			return err
		}
	}
	return db.Exec(`UPDATE users SET role = a.role FROM allowed_emails a
		WHERE LOWER(users.email) = LOWER(a.email) AND users.role <> a.role`).Error
}
