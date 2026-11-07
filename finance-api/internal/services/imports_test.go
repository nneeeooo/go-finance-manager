package services_test

import (
	"bytes"
	"crypto/sha256"
	"finance-api/internal/services"
	"fmt"
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

func TestStatementWindows(t *testing.T) {
	const w = services.MonobankMaxWindow
	// 70 days: three widest windows, newest first, contiguous, none over Monobank's limit
	got := services.StatementWindows(1_000_000, 1_000_000+70*86400)
	if len(got) != 3 || got[0].To != 1_000_000+70*86400 || got[2].From != 1_000_000 {
		t.Fatalf("windows: %+v", got)
	}
	for i, x := range got {
		if x.To-x.From > w {
			t.Errorf("window %d spans %d s", i, x.To-x.From)
		}
		if i > 0 && got[i-1].From != x.To+1 {
			t.Errorf("gap or overlap before window %d", i)
		}
	}
	if got[0].To-got[0].From != w || got[1].To-got[1].From != w {
		t.Error("all but the oldest window must be full")
	}
	// Exactly one window fits in one call
	if n := len(services.StatementWindows(0, w)); n != 1 {
		t.Errorf("one full window: %d calls", n)
	}
	// A year: 12 calls, as many as months, though windows run past month ends
	if n := len(services.StatementWindows(0, 365*86400)); n != 12 {
		t.Errorf("a year: %d calls", n)
	}
}

func TestDateSpan(t *testing.T) {
	kyiv, _ := time.LoadLocation("Europe/Kyiv")
	now := time.Date(2026, 11, 6, 15, 0, 0, 0, kyiv)

	// Kyiv days, both included; never past now
	from, to, err := services.DateSpan("2026-09-15", "2026-12-31", now)
	if err != nil {
		t.Fatal(err)
	}
	if from != time.Date(2026, 9, 15, 0, 0, 0, 0, kyiv).Unix() || to != now.Unix() {
		t.Fatalf("span %d..%d", from, to)
	}
	_, to, _ = services.DateSpan("2026-10-01", "2026-10-31", now)
	if to != time.Date(2026, 11, 1, 0, 0, 0, 0, kyiv).Unix()-1 {
		t.Errorf("31 October ends %d", to)
	}
	for _, bad := range [][2]string{{"2026-10-02", "2026-10-01"}, {"2027-01-01", "2027-02-01"}, {"2026-10", "2026-11-01"}} {
		if _, _, err := services.DateSpan(bad[0], bad[1], now); err == nil {
			t.Errorf("%v: expected an error", bad)
		}
	}
}

func TestUpdateSpan(t *testing.T) {
	now := time.Date(2026, 11, 6, 15, 0, 0, 0, time.UTC)
	full := now.Unix() - services.MonobankMaxWindow

	// Nothing saved, or saved yesterday: one full window, the same single call
	if from, to := services.UpdateSpan(nil, now); from != full || to != now.Unix() {
		t.Errorf("nothing saved: %d..%d", from, to)
	}
	yesterday := now.Add(-24 * time.Hour)
	if from, _ := services.UpdateSpan(&yesterday, now); from != full {
		t.Errorf("saved yesterday: from %d", from)
	}
	// Saved 50 days ago: from 3 days before it, two calls
	old := now.Add(-50 * 24 * time.Hour)
	from, to := services.UpdateSpan(&old, now)
	if from != old.Add(-services.UpdateOverlap).Unix() || len(services.StatementWindows(from, to)) != 2 {
		t.Errorf("saved 50 days ago: from %d", from)
	}
}

// PrivatBank statement times are Kyiv time; the operation id must stay what it was when they were read as UTC
func TestPrivatbankKyivTimeKeepsID(t *testing.T) {
	kyiv, _ := time.LoadLocation("Europe/Kyiv")
	account := uuid.MustParse("01a1b2cd-78e5-71d7-81d4-8ea0d6e4c587")
	row := services.PrivatbankRow{Date: time.Date(2026, 11, 5, 19, 41, 0, 0, kyiv), Amount: -20286, Description: "АТБ", Currency: "UAH"}
	oldInput := fmt.Sprintf("privatbank:%s:%s:%d:%s", account, "2026-11-05T19:41:00Z", row.Amount, row.Description)
	want := uuid.NewHash(sha256.New(), uuid.NameSpaceURL, []byte(oldInput), 8)

	tx := services.PrivatbankTransaction(account, row)
	if tx.ID != want {
		t.Fatalf("id changed: %s, want %s", tx.ID, want)
	}
	if !tx.OccurredAt.Equal(time.Date(2026, 11, 5, 17, 41, 0, 0, time.UTC)) {
		t.Fatalf("occurred_at %s, want 17:41 UTC", tx.OccurredAt.UTC())
	}
}
