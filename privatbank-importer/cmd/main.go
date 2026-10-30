package main

import (
	"crypto/sha256"
	"fmt"
	"math"
	"os"
	"privatbank-importer/internal/config"
	"strconv"
	"strings"
	"time"

	"github.com/JohannesJHN/iso4217"
	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

type BankTransactionPrivatbank struct {
	Date        time.Time
	Category    string
	Description string
	Amount      int64
	Currency    string
}

type CreateTransactionRequest struct {
	ID          uuid.UUID `json:"id"`
	AccountID   uuid.UUID `json:"account_id"`
	BankID      uint16    `json:"bank_id"`
	Amount      int64     `json:"amount"`
	Currency    uint16    `json:"currency"`
	MCC         uint16    `json:"mcc"`
	Description string    `json:"description"`
	OccurredAt  time.Time `json:"occurred_at"`
}

func currencyCode(code string) (uint16, error) {
	currency, ok := iso4217.LookupByAlpha3(code)
	if !ok {
		return 0, fmt.Errorf("unknown currency: %s", code)
	}

	return uint16(currency.Numeric), nil
}

func ReadXLSX(path string) ([]BankTransactionPrivatbank, error) {
	file, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("open xlsx: %w", err)
	}
	defer file.Close()

	sheet := file.GetSheetName(0)

	rows, err := file.GetRows(sheet)
	if err != nil {
		return nil, fmt.Errorf("read xlsx rows: %w", err)
	}

	var transactions []BankTransactionPrivatbank

	for i, row := range rows[2:] {
		if len(row) < 8 {
			continue
		}

		dateStr := strings.TrimSpace(row[0])
		date, err := time.Parse("02.01.2006 15:04:05", dateStr)
		if err != nil {
			date, err = time.Parse("02.01.2006 15:04", dateStr)
			if err != nil {
				return nil, fmt.Errorf("parse date at row %d (%q): %w", i+3, dateStr, err)
			}
		}

		amountStr := strings.TrimSpace(row[4])
		amount, err := strconv.ParseFloat(amountStr, 64)
		if err != nil {
			return nil, fmt.Errorf("parse amount at row %d (%q): %w", i+3, amountStr, err)
		}

		transactions = append(transactions, BankTransactionPrivatbank{
			Date:        date,
			Category:    strings.TrimSpace(row[1]),
			Description: strings.TrimSpace(row[3]),
			Amount:      int64(math.Round(amount * 100)),
			Currency:    strings.TrimSpace(row[7]),
		})
	}

	return transactions, nil
}

func ImportTransactions(accountID uuid.UUID, path string) error {
	cfg := config.Load()

	transactions, err := ReadXLSX(path)

	// for testing

	// data, err := json.MarshalIndent(transactions[0], "", "  ")
	// if err != nil {
	// 	return err
	// }

	// fmt.Println(string(data))
	// return nil

	if err != nil {
		return fmt.Errorf("read xlsx: %w", err)
	}

	client := resty.New().
		SetBaseURL(fmt.Sprintf("http://%s:%s", cfg.AppHost, cfg.AppPort)).
		SetTimeout(30 * time.Second)

	for _, bankTransaction := range transactions {
		// Generate deterministic transaction ID.
		hashInput := fmt.Sprintf(
			"privatbank:%s:%s:%d:%s",
			accountID.String(),
			bankTransaction.Date.Format(time.RFC3339),
			bankTransaction.Amount,
			bankTransaction.Description,
		)

		transactionID := uuid.NewHash(
			sha256.New(),
			uuid.NameSpaceURL,
			[]byte(hashInput),
			8,
		)

		currency, err := currencyCode(bankTransaction.Currency)
		if err != nil {
			return fmt.Errorf("convert currency %q: %w", bankTransaction.Currency, err)
		}

		request := CreateTransactionRequest{
			ID:          transactionID,
			AccountID:   accountID,
			BankID:      2,
			Amount:      bankTransaction.Amount,
			Currency:    currency,
			MCC:         7777,
			Description: bankTransaction.Description,
			OccurredAt:  bankTransaction.Date,
		}

		resp, err := client.R().
			SetHeader("Content-Type", "application/json").
			SetBody(request).
			Post("/api/v1/transactions")

		if err != nil {
			return fmt.Errorf("send transaction: %w", err)
		}

		if !resp.IsSuccess() && resp.StatusCode() != 409 {
			return fmt.Errorf(
				"send transaction returned HTTP %d: %s",
				resp.StatusCode(),
				resp.String(),
			)
		}
	}

	return nil
}

func main() {
	if len(os.Args) >= 3 {
		accountID, err := uuid.Parse(os.Args[1])
		if err != nil {
			fmt.Printf("Invalid account UUID: %v\n", err)
			os.Exit(1)
		}
		filePath := os.Args[2]
		if err := ImportTransactions(accountID, filePath); err != nil {
			fmt.Printf("Import failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Successfully imported %s into %s\n", filePath, accountID)
		return
	}

	// Default fallback files if present in directory
	defaults := []struct {
		accountID string
		filePath  string
	}{
		{"01a045c5-2cbd-79e6-919b-fc32a5baf8f9", "privat-universalna.xlsx"},
		{"01a045e1-31b5-7f1e-bf5a-38c548f735c9", "privat-dlyavyplat.xlsx"},
	}

	imported := 0
	for _, item := range defaults {
		if _, err := os.Stat(item.filePath); err == nil {
			id := uuid.MustParse(item.accountID)
			if err := ImportTransactions(id, item.filePath); err != nil {
				fmt.Printf("Failed to import %s: %v\n", item.filePath, err)
			} else {
				fmt.Printf("Imported %s\n", item.filePath)
				imported++
			}
		}
	}

	if imported == 0 && len(os.Args) < 3 {
		fmt.Println("Usage: privatbank-importer <account-uuid> <statement.xlsx>")
	}
}
