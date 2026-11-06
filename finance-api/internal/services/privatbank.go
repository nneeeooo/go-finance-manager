package services

import (
	"crypto/sha256"
	"finance-api/internal/models"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

// PrivatbankRow: one operation from a PrivatBank statement export (.xlsx, Privat24 «Виписка»)
type PrivatbankRow struct {
	Date        time.Time `json:"date"`
	Category    string    `json:"category"`
	Description string    `json:"description"`
	Amount      int64     `json:"amount"` // minor units, negative = spent
	Currency    string    `json:"currency"`
}

// privatbankMCC: the statement has no MCC; the importer always used this one
const privatbankMCC = 7777

// Currencies a PrivatBank card statement uses
var privatbankCurrencies = map[string]uint16{"UAH": 980, "USD": 840, "EUR": 978, "PLN": 985, "GBP": 826, "CHF": 756, "CZK": 203}

// ParsePrivatbankXLSX reads the statement: two header rows, then date (col A), category (B), description (D),
// card amount (E) and card currency (H). Rows with fewer columns are skipped; a bad date or amount stops with the row number.
func ParsePrivatbankXLSX(r io.Reader) ([]PrivatbankRow, error) {
	file, err := excelize.OpenReader(r)
	if err != nil {
		return nil, fmt.Errorf("not an xlsx file: %w", err)
	}
	defer file.Close()

	rows, err := file.GetRows(file.GetSheetName(0))
	if err != nil {
		return nil, fmt.Errorf("read rows: %w", err)
	}
	if len(rows) < 3 {
		return nil, fmt.Errorf("the statement has no operations")
	}

	var out []PrivatbankRow
	for i, row := range rows[2:] {
		line := i + 3
		if len(row) < 8 {
			continue
		}
		dateStr := strings.TrimSpace(row[0])
		date, err := time.Parse("02.01.2006 15:04:05", dateStr)
		if err != nil {
			if date, err = time.Parse("02.01.2006 15:04", dateStr); err != nil {
				return nil, fmt.Errorf("row %d: date %q", line, dateStr)
			}
		}
		amountStr := strings.ReplaceAll(strings.TrimSpace(row[4]), ",", ".")
		amount, err := strconv.ParseFloat(amountStr, 64)
		if err != nil {
			return nil, fmt.Errorf("row %d: amount %q", line, amountStr)
		}
		currency := strings.ToUpper(strings.TrimSpace(row[7]))
		if _, ok := privatbankCurrencies[currency]; !ok {
			return nil, fmt.Errorf("row %d: unknown currency %q", line, currency)
		}
		out = append(out, PrivatbankRow{
			Date:        date,
			Category:    strings.TrimSpace(row[1]),
			Description: strings.TrimSpace(row[3]),
			Amount:      int64(math.Round(amount * 100)),
			Currency:    currency,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("the statement has no operations")
	}
	return out, nil
}

// PrivatbankTransaction: the transaction row for one statement line. The id is deterministic (same formula as the
// CLI importer), so importing an overlapping statement again adds only the new lines.
func PrivatbankTransaction(accountID uuid.UUID, row PrivatbankRow) models.Transaction {
	hashInput := fmt.Sprintf("privatbank:%s:%s:%d:%s", accountID, row.Date.Format(time.RFC3339), row.Amount, row.Description)
	return models.Transaction{
		ID:          uuid.NewHash(sha256.New(), uuid.NameSpaceURL, []byte(hashInput), 8),
		AccountID:   accountID,
		BankID:      models.BankPrivatBank,
		Amount:      row.Amount,
		Currency:    privatbankCurrencies[row.Currency],
		MCC:         privatbankMCC,
		Category:    row.Category,
		Description: row.Description,
		OccurredAt:  row.Date,
	}
}
