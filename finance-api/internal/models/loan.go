package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Loan: a loan or installment the user pays off. Banks give no API for it, so the user enters it from the contract.
// Payment k (1-based) is due on the last day of the month FirstDue + k - 1; the last one may differ (rounding).
type Loan struct {
	ID     uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID uuid.UUID `gorm:"type:uuid;not null;index" json:"-"`

	Name          string `gorm:"type:varchar(100);not null" json:"name"`
	Bank          Bank   `gorm:"not null" json:"bank"`
	Currency      uint16 `gorm:"not null" json:"currency"`
	Principal     int64  `gorm:"not null;default:0" json:"principal"`        // amount borrowed, minor units
	Payment       int64  `gorm:"not null" json:"payment"`                    // regular monthly payment, minor units
	LastPayment   int64  `gorm:"not null" json:"last_payment"`               // the final payment, minor units
	PaymentsTotal int    `gorm:"not null" json:"payments_total"`             // number of payments
	PaymentsPaid  int    `gorm:"not null;default:0" json:"payments_paid"`    // how many are paid already
	FirstDue      string `gorm:"type:varchar(10);not null" json:"first_due"` // YYYY-MM-DD, any day of the first payment's month

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (l *Loan) BeforeCreate(tx *gorm.DB) error {
	if l.ID == uuid.Nil {
		l.ID = uuid.Must(uuid.NewV7())
	}
	return nil
}
