package services_test

import (
	"bytes"
	"finance-api/internal/services"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

// privatbankXLSX builds a statement like the Privat24 export: two header rows, then operations
func privatbankXLSX(t *testing.T, rows [][]any) *bytes.Buffer {
	t.Helper()
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	all := append([][]any{{"Виписка"}, {"Дата", "Категорія", "Картка", "Опис", "Сума в валюті картки", "", "", "Валюта картки"}}, rows...)
	for i, r := range all {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		if err := f.SetSheetRow(sheet, cell, &r); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	return &buf
}

func TestParsePrivatbankXLSX(t *testing.T) {
	buf := privatbankXLSX(t, [][]any{
		{"05.11.2026 19:41:00", "Супермаркети", "*1234", "АТБ", "-202.86", "", "", "UAH"},
		{"01.11.2026 09:00", "Перекази", "*1234", "Зарплата", "15000", "", "", "UAH"},
		{"short row"},
	})
	rows, err := services.ParsePrivatbankXLSX(buf)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows: %d", len(rows))
	}
	if rows[0].Amount != -20286 || rows[0].Currency != "UAH" || rows[0].Category != "Супермаркети" || rows[0].Description != "АТБ" {
		t.Errorf("row 0: %+v", rows[0])
	}
	if rows[1].Amount != 1500000 || rows[1].Date.Minute() != 0 || rows[1].Date.Day() != 1 {
		t.Errorf("row 1 (date without seconds): %+v", rows[1])
	}

	// Same line, same account: same id, so a re-import adds nothing; another account: another id
	acc := uuid.New()
	a, b := services.PrivatbankTransaction(acc, rows[0]), services.PrivatbankTransaction(acc, rows[0])
	if a.ID != b.ID || a.Currency != 980 || a.BankID != 2 {
		t.Errorf("transaction: %+v", a)
	}
	if services.PrivatbankTransaction(uuid.New(), rows[0]).ID == a.ID {
		t.Error("another account must give another id")
	}
}

func TestParsePrivatbankXLSXErrors(t *testing.T) {
	cases := map[string][][]any{
		"bad date":     {{"2026-11-05", "c", "", "d", "1", "", "", "UAH"}},
		"bad amount":   {{"05.11.2026 19:41", "c", "", "d", "abc", "", "", "UAH"}},
		"bad currency": {{"05.11.2026 19:41", "c", "", "d", "1", "", "", "XYZ"}},
		"empty":        {},
	}
	for name, rows := range cases {
		if _, err := services.ParsePrivatbankXLSX(privatbankXLSX(t, rows)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	if _, err := services.ParsePrivatbankXLSX(bytes.NewBufferString("not xlsx")); err == nil {
		t.Error("not an xlsx: expected an error")
	}
}

func TestMonthRanges(t *testing.T) {
	kyiv, _ := time.LoadLocation("Europe/Kyiv")
	now := time.Date(2026, 11, 6, 15, 0, 0, 0, kyiv)

	months, err := services.MonthRanges("2026-09", "2026-12", now)
	if err != nil {
		t.Fatal(err)
	}
	// December is in the future: skipped; newest first; November ends now
	if len(months) != 3 || months[0].Label != "2026-11" || months[2].Label != "2026-09" {
		t.Fatalf("months: %+v", months)
	}
	if months[0].To != now.Unix() {
		t.Error("the current month must end now")
	}
	oct := months[1]
	if oct.From != time.Date(2026, 10, 1, 0, 0, 0, 0, kyiv).Unix() || oct.To != time.Date(2026, 11, 1, 0, 0, 0, 0, kyiv).Unix()-1 {
		t.Errorf("October bounds: %+v", oct)
	}
	// Every month fits Monobank's 31 days + 1 hour window
	for _, m := range months {
		if m.To-m.From > 2682000 {
			t.Errorf("%s spans %d s", m.Label, m.To-m.From)
		}
	}

	for _, bad := range [][2]string{{"2026-10", "2026-09"}, {"2027-01", "2027-02"}, {"2024-01", "2026-11"}, {"x", "2026-11"}} {
		if _, err := services.MonthRanges(bad[0], bad[1], now); err == nil {
			t.Errorf("%v: expected an error", bad)
		}
	}
}
