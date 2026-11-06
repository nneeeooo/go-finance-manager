package handlers_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/steinfletcher/apitest"
	"github.com/xuri/excelize/v2"
)

func createPrivatbankAccount(t *testing.T, userID string) string {
	t.Helper()
	var resp struct {
		ID string `json:"id"`
	}
	apitest.New().
		Handler(router).
		Post("/api/v1/accounts").
		Header("Authorization", authHeaderForUser(t, userID)).
		JSON(`{"bank":2,"name":"Privat Універсальна","currency":980}`).
		Expect(t).
		Status(http.StatusCreated).
		Assert(func(res *http.Response, _ *http.Request) error { return json.NewDecoder(res.Body).Decode(&resp) }).
		End()
	return resp.ID
}

// statementUpload: a multipart request with a two-operation PrivatBank statement
func statementUpload(t *testing.T, userID, accountID, query string) *httptest.ResponseRecorder {
	t.Helper()
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	rows := [][]any{
		{"Виписка"},
		{"Дата", "Категорія", "Картка", "Опис", "Сума", "", "", "Валюта"},
		{"05.11.2026 19:41:00", "Супермаркети", "*1", "АТБ", "-202.86", "", "", "UAH"},
		{"01.11.2026 09:00:00", "Перекази", "*1", "Зарплата", "15000", "", "", "UAH"},
	}
	for i, r := range rows {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		_ = f.SetSheetRow(sheet, cell, &r)
	}
	var file bytes.Buffer
	_ = f.Write(&file)

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	_ = w.WriteField("account_id", accountID)
	part, _ := w.CreateFormFile("file", "statement.xlsx")
	_, _ = part.Write(file.Bytes())
	_ = w.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/imports/privatbank"+query, &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", authHeaderForUser(t, userID))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestImportPrivatbank(t *testing.T) {
	userID := createTestUser(t)
	accountID := createPrivatbankAccount(t, userID)

	decode := func(rec *httptest.ResponseRecorder) (res struct{ Total, New int }) {
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &res)
		return res
	}

	if r := decode(statementUpload(t, userID, accountID, "?dry_run=true")); r.Total != 2 || r.New != 2 {
		t.Fatalf("dry run: %+v", r)
	}
	if r := decode(statementUpload(t, userID, accountID, "")); r.New != 2 {
		t.Fatalf("import: %+v", r)
	}
	// The same statement again adds nothing
	if r := decode(statementUpload(t, userID, accountID, "")); r.New != 0 {
		t.Fatalf("re-import: %+v", r)
	}

	// Another user's account, or a Monobank account: refused
	if rec := statementUpload(t, createTestUser(t), accountID, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("other user's account: %d", rec.Code)
	}
	if rec := statementUpload(t, userID, createTestAccountForUser(t, userID), ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("monobank account: %d", rec.Code)
	}
}

func TestStartMonobankImportValidation(t *testing.T) {
	userID := createTestUser(t)
	monoID := createTestAccountForUser(t, userID) // no external_id
	privatID := createPrivatbankAccount(t, userID)

	cases := []struct {
		account, from, to string
		status            int
	}{
		{privatID, "2026-01", "2026-02", http.StatusBadRequest}, // not a Monobank account
		{monoID, "2026-01", "2026-02", http.StatusBadRequest},   // no Monobank account id
		{"not-a-uuid", "2026-01", "2026-02", http.StatusBadRequest},
	}
	for _, c := range cases {
		apitest.New().
			Handler(router).
			Post("/api/v1/imports/monobank").
			Header("Authorization", authHeaderForUser(t, userID)).
			JSON(fmt.Sprintf(`{"account_id":"%s","token":"t","from":"%s","to":"%s"}`, c.account, c.from, c.to)).
			Expect(t).
			Status(c.status).
			End()
	}

	apitest.New().
		Handler(router).
		Get("/api/v1/imports/monobank").
		Header("Authorization", authHeaderForUser(t, userID)).
		Expect(t).
		Status(http.StatusNoContent).
		End()
}

func TestDeleteOtherUsersAccount(t *testing.T) {
	owner := createTestUser(t)
	accountID := createTestAccountForUser(t, owner)

	apitest.New().
		Handler(router).
		Delete("/api/v1/accounts/"+accountID).
		Header("Authorization", authHeaderForUser(t, createTestUser(t))).
		Expect(t).
		Status(http.StatusNotFound).
		End()

	// Still there for the owner
	apitest.New().
		Handler(router).
		Get("/api/v1/accounts/"+accountID).
		Header("Authorization", authHeaderForUser(t, owner)).
		Expect(t).
		Status(http.StatusOK).
		End()
}
