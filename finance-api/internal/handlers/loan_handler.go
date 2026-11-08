package handlers

import (
	"errors"
	"finance-api/internal/models"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type loanRequest struct {
	Name          string      `json:"name" binding:"required"`
	Bank          models.Bank `json:"bank" binding:"required"`
	Currency      uint16      `json:"currency" binding:"required"`
	Principal     int64       `json:"principal" binding:"min=0"`
	Payment       int64       `json:"payment" binding:"required,gt=0"`
	LastPayment   int64       `json:"last_payment" binding:"gte=0"` // 0: the same as payment
	PaymentsTotal int         `json:"payments_total" binding:"required,gt=0"`
	PaymentsPaid  int         `json:"payments_paid" binding:"gte=0"`
	FirstDue      string      `json:"first_due" binding:"required"` // YYYY-MM-DD
}

type loanPatch struct {
	Name         *string `json:"name"`
	Payment      *int64  `json:"payment"`
	LastPayment  *int64  `json:"last_payment"`
	PaymentsPaid *int    `json:"payments_paid"`
}

// validLoan checks the numbers a request or an edited loan must satisfy
func validLoan(l models.Loan) string {
	if _, err := time.Parse(time.DateOnly, l.FirstDue); err != nil {
		return "first_due: expected YYYY-MM-DD"
	}
	if l.PaymentsPaid > l.PaymentsTotal {
		return "payments_paid is more than payments_total"
	}
	return ""
}

// ownLoan loads the loan when it belongs to the logged-in user; writes the error otherwise
func ownLoan(c *gin.Context) (models.Loan, bool) {
	var loan models.Loan
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid loan ID"})
		return loan, false
	}
	db := c.MustGet("db").(*gorm.DB)
	if err := db.First(&loan, "id = ? AND user_id = ?", id, c.MustGet("user_id")).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Loan not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get loan"})
		}
		return loan, false
	}
	return loan, true
}

// CreateLoan godoc
// @Summary Add a loan
// @Description A loan or installment the user pays off, entered from the contract (banks have no API for it).
// @Description Payment k is due on the last day of the month first_due + k - 1.
// @Tags loans
// @Accept json
// @Produce json
// @Param request body loanRequest true "Loan"
// @Success 201 {object} models.Loan
// @Failure 400 {object} ErrorResponse
// @Security BearerAuth
// @Failure 401 {object} ErrorResponse "Not logged in"
// @Router /loans [post]
func CreateLoan(c *gin.Context) {
	var req loanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	loan := models.Loan{
		UserID: c.MustGet("user_id").(uuid.UUID), Name: req.Name, Bank: req.Bank, Currency: req.Currency,
		Principal: req.Principal, Payment: req.Payment, LastPayment: req.LastPayment, PaymentsTotal: req.PaymentsTotal,
		PaymentsPaid: req.PaymentsPaid, FirstDue: req.FirstDue,
	}
	if loan.LastPayment == 0 {
		loan.LastPayment = loan.Payment
	}
	if msg := validLoan(loan); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	if err := c.MustGet("db").(*gorm.DB).Create(&loan).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create loan"})
		return
	}
	c.JSON(http.StatusCreated, loan)
}

// ListLoans godoc
// @Summary The user's loans
// @Tags loans
// @Produce json
// @Success 200 {array} models.Loan
// @Security BearerAuth
// @Failure 401 {object} ErrorResponse "Not logged in"
// @Failure 500 {object} ErrorResponse
// @Router /loans [get]
func ListLoans(c *gin.Context) {
	loans := []models.Loan{}
	if err := c.MustGet("db").(*gorm.DB).Where("user_id = ?", c.MustGet("user_id")).Order("created_at").Find(&loans).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get loans"})
		return
	}
	c.JSON(http.StatusOK, loans)
}

// UpdateLoan godoc
// @Summary Edit a loan
// @Description Name, payment amounts and how many payments are paid (mark a payment: payments_paid + 1).
// @Tags loans
// @Accept json
// @Produce json
// @Param id path string true "Loan ID"
// @Param request body loanPatch true "Fields to change"
// @Success 200 {object} models.Loan
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Failure 401 {object} ErrorResponse "Not logged in"
// @Router /loans/{id} [patch]
func UpdateLoan(c *gin.Context) {
	var req loanPatch
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	loan, ok := ownLoan(c)
	if !ok {
		return
	}
	if req.Name != nil && *req.Name != "" {
		loan.Name = *req.Name
	}
	if req.Payment != nil && *req.Payment > 0 {
		loan.Payment = *req.Payment
	}
	if req.LastPayment != nil && *req.LastPayment > 0 {
		loan.LastPayment = *req.LastPayment
	}
	if req.PaymentsPaid != nil && *req.PaymentsPaid >= 0 {
		loan.PaymentsPaid = *req.PaymentsPaid
	}
	if msg := validLoan(loan); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	if err := c.MustGet("db").(*gorm.DB).Save(&loan).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update loan"})
		return
	}
	c.JSON(http.StatusOK, loan)
}

// DeleteLoan godoc
// @Summary Delete a loan
// @Tags loans
// @Param id path string true "Loan ID"
// @Success 204
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Failure 401 {object} ErrorResponse "Not logged in"
// @Router /loans/{id} [delete]
func DeleteLoan(c *gin.Context) {
	loan, ok := ownLoan(c)
	if !ok {
		return
	}
	if err := c.MustGet("db").(*gorm.DB).Delete(&loan).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete loan"})
		return
	}
	c.Status(http.StatusNoContent)
}
