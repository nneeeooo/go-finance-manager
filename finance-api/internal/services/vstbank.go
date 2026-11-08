package services

import (
	"crypto/sha256"
	"errors"
	"finance-api/internal/models"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

// VST Bank («Банк Власний Рахунок») statement «Виписка по рахунку», .xlsx. The file is a print layout: about 60
// narrow columns, merged cells, a block of labelled values on top («IBAN:», «Кредитний ліміт:», «Залишок коштів на
// кінець періоду:») and then the operations under a header row. So columns are found by their header text.
//
// «Залишок коштів» is what the card can spend: own money plus the credit limit. Own money (Account.Balance, negative
// when owed) is that minus the limit. Rows named «Кредитний ліміт» only move the limit, not money, and are skipped.
// A commission is a separate column on top of the amount.

var (
	vstDateTime = "02.01.06 15:04"
	vstAuthCode = regexp.MustCompile(`\s*Код авторизації\s+(\S+)`)
)

func vstNumber(s string) (int64, error) {
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "UAH"))
	if s == "" || s == "-" {
		return 0, nil
	}
	f, err := strconv.ParseFloat(strings.ReplaceAll(strings.ReplaceAll(s, " ", ""), ",", "."), 64)
	if err != nil {
		return 0, fmt.Errorf("number %q", s)
	}
	return int64(math.Round(f * 100)), nil
}

func flat(s string) string { return strings.Join(strings.Fields(s), " ") }

// ParseVstbankXLSX reads the statement: the limit and the closing balance from the top block, then every operation
func ParseVstbankXLSX(r io.Reader) (Statement, error) {
	var st Statement
	file, err := excelize.OpenReader(r)
	if err != nil {
		return st, fmt.Errorf("not an xlsx file: %w", err)
	}
	defer file.Close()
	rows, err := file.GetRows(file.GetSheetName(0))
	if err != nil {
		return st, fmt.Errorf("read rows: %w", err)
	}

	// The labelled values on top: the value is the next filled cell after its label
	var closing *int64
	valueAfter := func(row []string, i int) string {
		for _, c := range row[i+1:] {
			if strings.TrimSpace(c) != "" {
				return c
			}
		}
		return ""
	}
	cols := map[string]int{}
	header := -1
	for n, row := range rows {
		for i, cell := range row {
			text := flat(cell)
			switch {
			case strings.HasPrefix(text, "Кредитний ліміт:"):
				v, err := vstNumber(valueAfter(row, i))
				if err != nil {
					return st, fmt.Errorf("row %d: credit limit: %w", n+1, err)
				}
				st.CreditLimit = &v
			case strings.HasPrefix(text, "Залишок коштів на кінець періоду:"):
				v, err := vstNumber(valueAfter(row, i))
				if err != nil {
					return st, fmt.Errorf("row %d: closing balance: %w", n+1, err)
				}
				closing = &v
			case strings.HasPrefix(text, "Деталі"):
				header = n
			}
		}
		if header == n {
			for i, cell := range row {
				t := flat(cell)
				switch {
				case strings.HasPrefix(t, "Дата та час"):
					cols["time"] = i
				case strings.HasPrefix(t, "Деталі"):
					cols["desc"] = i
				case strings.HasPrefix(t, "Фактична сума"):
					cols["amount"] = i
				case strings.HasPrefix(t, "Комісія"):
					cols["fee"] = i
				case t == "MCC":
					cols["mcc"] = i
				}
			}
			break
		}
	}
	if header < 0 || len(cols) < 5 {
		return st, errors.New("this is not a VST Bank statement (no operations header)")
	}

	get := func(row []string, key string) string {
		if i := cols[key]; i < len(row) {
			return strings.TrimSpace(row[i])
		}
		return ""
	}
	for n, row := range rows[header+1:] {
		line := header + n + 2
		when := flat(get(row, "time"))
		date, err := time.ParseInLocation(vstDateTime, when, kyiv)
		if err != nil {
			continue // totals, signatures and notes below the table
		}
		desc := get(row, "desc")
		if strings.HasPrefix(desc, "Кредитний ліміт") {
			continue
		}
		amount, err := vstNumber(get(row, "amount"))
		if err != nil {
			return st, fmt.Errorf("row %d: amount: %w", line, err)
		}
		fee, err := vstNumber(get(row, "fee"))
		if err != nil {
			return st, fmt.Errorf("row %d: commission: %w", line, err)
		}
		ref := ""
		if m := vstAuthCode.FindStringSubmatch(desc); m != nil {
			ref = m[1]
			desc = vstAuthCode.ReplaceAllString(desc, "")
		}
		desc = strings.TrimSuffix(flat(desc), `\`)
		var mcc uint16
		if v, err := strconv.ParseUint(get(row, "mcc"), 10, 16); err == nil {
			mcc = uint16(v)
		}
		st.Rows = append(st.Rows, StatementRow{
			Date: date, Category: vstCategory(desc, mcc), Description: strings.TrimSpace(desc),
			Amount: amount + fee, Currency: "UAH", MCC: mcc, Ref: ref,
		})
	}
	if len(st.Rows) == 0 {
		return st, errors.New("the statement has no operations")
	}

	// The closing balance belongs to the newest operation; it is the card's spendable money, so subtract the limit
	if closing != nil {
		newest := 0
		for i, r := range st.Rows {
			if !r.Date.Before(st.Rows[newest].Date) {
				newest = i
			}
		}
		limit := int64(0)
		if st.CreditLimit != nil {
			limit = *st.CreditLimit
		}
		st.Rows[newest].Balance, st.Rows[newest].HasBalance = *closing-limit, true
	}
	return st, nil
}

// vstCategory names the lines that have no merchant category; purchases are told apart by their MCC
func vstCategory(desc string, mcc uint16) string {
	switch {
	case mcc != 0:
		return ""
	case strings.HasPrefix(desc, "Купівля"):
		return ""
	case strings.Contains(desc, "Поповнення") || strings.Contains(desc, "Пополнение") || strings.Contains(desc, "Зарахування") || strings.HasPrefix(desc, "Credit"):
		return "Поповнення"
	case strings.Contains(desc, "Переказ") || strings.Contains(desc, "Списання коштів"):
		return "Переказ"
	}
	return ""
}

// VstbankTransaction: the row for one statement line. The id is deterministic, so an overlapping statement adds only new lines.
func VstbankTransaction(accountID uuid.UUID, row StatementRow) models.Transaction {
	d := row.Date
	wall := time.Date(d.Year(), d.Month(), d.Day(), d.Hour(), d.Minute(), d.Second(), 0, time.UTC)
	hashInput := fmt.Sprintf("vstbank:%s:%s:%d:%s:%s", accountID, wall.Format(time.RFC3339), row.Amount, row.Ref, row.Description)
	return models.Transaction{
		ID:          uuid.NewHash(sha256.New(), uuid.NameSpaceURL, []byte(hashInput), 8),
		AccountID:   accountID,
		BankID:      models.BankVSTBank,
		Amount:      row.Amount,
		Balance:     row.Balance,
		Currency:    privatbankCurrencies[row.Currency],
		MCC:         row.MCC,
		Category:    row.Category,
		Description: row.Description,
		OccurredAt:  row.Date,
	}
}
