package services_test

import (
	"bytes"
	"testing"

	"finance-api/internal/services"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

// vstbankXLSX lays a statement out like the bank's print export: labelled values on top, a header row, the operations
func vstbankXLSX(t *testing.T, rows [][]string) *bytes.Buffer {
	t.Helper()
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	set := func(col, row int, v string) {
		cell, _ := excelize.CoordinatesToCellName(col, row)
		if err := f.SetCellValue(sheet, cell, v); err != nil {
			t.Fatal(err)
		}
	}
	set(4, 12, "Кредитний ліміт:")
	set(6, 12, "28000 UAH")
	set(4, 30, "Залишок коштів на кінець періоду:")
	set(8, 30, "12345,67 UAH")
	// time, desc, amount, fee and mcc sit in far apart columns, like in the print layout
	for col, h := range map[int]string{4: "Дата\nта час\nтранзакції", 7: "Деталі\nоперації", 42: "Фактична сума операції(UAH)", 53: "Комісія\n(UAH)", 55: "MCC"} {
		set(col, 34, h)
	}
	for i, r := range rows {
		set(4, 35+i, r[0])
		set(7, 35+i, r[1])
		set(42, 35+i, r[2])
		set(53, 35+i, r[3])
		set(55, 35+i, r[4])
	}
	set(4, 40, "Загальна сума видаткових операцій за весь період:")
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	return &buf
}

func TestParseVstbankXLSX(t *testing.T) {
	buf := vstbankXLSX(t, [][]string{
		{"22.01.24 00:00", "Кредитний ліміт", "5000", "-", "-"},
		{"22.01.24 10:03", "Переказ – MoneySend/VisaDirect Код авторизації AB12CD", "10", "-", "-"},
		{"27.01.24 18:19", "Купівля: SHOP QR TESTMARKET\\main-st Код авторизації EF34GH", "-432.85", "-", "5411"},
		{"03.08.24 14:45", "Списання коштів з ПР за операцію MS/VD: MONODirect\\\n(Комісія: 1.02) Код авторизації JK56LM", "-100", "-1.02", "-"},
	})
	st, err := services.ParseVstbankXLSX(buf)
	if err != nil {
		t.Fatal(err)
	}
	if st.CreditLimit == nil || *st.CreditLimit != 2800000 {
		t.Fatalf("limit: %v", st.CreditLimit)
	}
	if len(st.Rows) != 3 {
		t.Fatalf("rows: %d (the limit line must be skipped)", len(st.Rows))
	}
	transfer, purchase, fee := st.Rows[0], st.Rows[1], st.Rows[2]
	if transfer.Amount != 1000 || transfer.Category != "Переказ" || transfer.Ref != "AB12CD" || transfer.Description != "Переказ – MoneySend/VisaDirect" {
		t.Errorf("transfer: %+v", transfer)
	}
	if purchase.Amount != -43285 || purchase.MCC != 5411 || purchase.Category != "" || purchase.Description != `Купівля: SHOP QR TESTMARKET\main-st` {
		t.Errorf("purchase: %+v", purchase)
	}
	if fee.Amount != -10102 { // -100.00 and the 1.02 commission on top
		t.Errorf("amount with commission: %d", fee.Amount)
	}
	// Closing balance 12345.67 is spendable money; own money is that minus the 28000 limit, on the newest line only
	if !fee.HasBalance || fee.Balance != 1234567-2800000 || transfer.HasBalance || purchase.HasBalance {
		t.Errorf("balance: %+v", st.Rows)
	}

	account := uuid.New()
	a, b := services.VstbankTransaction(account, transfer), services.VstbankTransaction(account, transfer)
	if a.ID != b.ID || a.BankID != 5 || a.Currency != 980 || services.VstbankTransaction(uuid.New(), transfer).ID == a.ID {
		t.Errorf("transaction: %+v", a)
	}
}

func TestParseVstbankXLSXRejectsOtherFiles(t *testing.T) {
	buf := privatbankXLSX(t, [][]any{{"05.11.2026 19:41", "c", "", "d", "1", "", "", "UAH"}})
	if _, err := services.ParseVstbankXLSX(buf); err == nil {
		t.Fatal("a PrivatBank statement must not parse as VST Bank")
	}
}
