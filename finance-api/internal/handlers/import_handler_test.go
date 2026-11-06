package handlers_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
	return statementsUpload(t, userID, accountID, query, 1)
}

// statementsUpload: the same statement attached n times, like overlapping exports
func statementsUpload(t *testing.T, userID, accountID, query string, n int) *httptest.ResponseRecorder {
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
	for i := range n {
		part, _ := w.CreateFormFile("file", fmt.Sprintf("statement-%d.xlsx", i))
		_, _ = part.Write(file.Bytes())
	}
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

func TestImportPrivatbankSeveralFiles(t *testing.T) {
	userID := createTestUser(t)
	accountID := createPrivatbankAccount(t, userID)
	// Two overlapping statements: each operation counts once
	rec := statementsUpload(t, userID, accountID, "", 2)
	var res struct{ Total, New int }
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if rec.Code != http.StatusOK || res.Total != 2 || res.New != 2 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
}

// fakeMonobank: client-info with one card and a one-item statement for any window
func fakeMonobank(t *testing.T) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json") // resty parses only JSON replies
		if r.Header.Get("X-Token") != "good" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		switch {
		case r.URL.Path == "/personal/client-info":
			fmt.Fprint(w, `{"name":"Test","accounts":[{"id":"mono-acc-1","type":"black","currencyCode":980,"balance":12345,"creditLimit":0,"maskedPan":["537541******1234"],"iban":"UA00"}]}`)
		case strings.HasPrefix(r.URL.Path, "/personal/statement/mono-acc-1/"):
			fmt.Fprint(w, `[{"id":"item-1","time":1767225600,"description":"АТБ","mcc":5411,"amount":-1000,"currencyCode":980,"balance":12345}]`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	oldURL, oldWait := testCfg.MonobankApiUrl, testCfg.MonobankRateLimitSeconds
	testCfg.MonobankApiUrl, testCfg.MonobankRateLimitSeconds = srv.URL, 3600 // the second call would wait an hour
	t.Cleanup(func() {
		srv.Close()
		testCfg.MonobankApiUrl, testCfg.MonobankRateLimitSeconds = oldURL, oldWait
	})
}

func TestListMonobankAccounts(t *testing.T) {
	fakeMonobank(t)
	userID := createTestUser(t)
	var linked struct {
		ID string `json:"id"`
	}
	apitest.New().Handler(router).Post("/api/v1/accounts").
		Header("Authorization", authHeaderForUser(t, userID)).
		JSON(`{"bank":1,"name":"Black","currency":980,"external_id":"mono-acc-1"}`).
		Expect(t).Status(http.StatusCreated).
		Assert(func(res *http.Response, _ *http.Request) error { return json.NewDecoder(res.Body).Decode(&linked) }).
		End()

	list := func(token string, status int) []map[string]any {
		var out []map[string]any
		apitest.New().Handler(router).Post("/api/v1/imports/monobank/accounts").
			Header("Authorization", authHeaderForUser(t, userID)).
			JSON(fmt.Sprintf(`{"token":%q}`, token)).
			Expect(t).Status(status).
			Assert(func(res *http.Response, _ *http.Request) error {
				if status == http.StatusOK {
					return json.NewDecoder(res.Body).Decode(&out)
				}
				return nil
			}).
			End()
		return out
	}

	got := list("good", http.StatusOK)
	if len(got) != 1 || got[0]["id"] != "mono-acc-1" || got[0]["linked_account_id"] != linked.ID {
		t.Fatalf("accounts: %+v", got)
	}
	list("bad", http.StatusBadRequest) // Monobank refused the token
	list("", http.StatusBadRequest)    // regular user without a token
}

func TestCancelMonobankImport(t *testing.T) {
	fakeMonobank(t)
	userID := createTestUser(t)
	auth := authHeaderForUser(t, userID)
	var acc struct {
		ID string `json:"id"`
	}
	apitest.New().Handler(router).Post("/api/v1/accounts").Header("Authorization", auth).
		JSON(`{"bank":1,"name":"Black","currency":980,"external_id":"mono-acc-1"}`).
		Expect(t).Status(http.StatusCreated).
		Assert(func(res *http.Response, _ *http.Request) error { return json.NewDecoder(res.Body).Decode(&acc) }).
		End()

	apitest.New().Handler(router).Delete("/api/v1/imports/monobank").Header("Authorization", auth).
		Expect(t).Status(http.StatusNotFound).End()

	apitest.New().Handler(router).Post("/api/v1/imports/monobank").Header("Authorization", auth).
		JSON(fmt.Sprintf(`{"account_id":%q,"token":"good","from":"2026-01","to":"2026-02"}`, acc.ID)).
		Expect(t).Status(http.StatusAccepted).End()

	// The first month is fetched at once; the second waits for the rate limit until cancelled
	status := func() (job struct {
		Status     string `json:"status"`
		MonthsDone int    `json:"months_done"`
		Fetched    int    `json:"fetched"`
	}) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/imports/monobank", nil)
		req.Header.Set("Authorization", auth)
		router.ServeHTTP(rec, req)
		_ = json.Unmarshal(rec.Body.Bytes(), &job)
		return job
	}
	deadline := time.Now().Add(5 * time.Second)
	for status().MonthsDone < 1 {
		if time.Now().After(deadline) {
			t.Fatalf("first month not fetched: %+v", status())
		}
		time.Sleep(20 * time.Millisecond)
	}

	apitest.New().Handler(router).Delete("/api/v1/imports/monobank").Header("Authorization", auth).
		Expect(t).Status(http.StatusOK).End()
	if job := status(); job.Status != "cancelled" || job.MonthsDone != 1 || job.Fetched != 1 {
		t.Fatalf("after cancel: %+v", job)
	}
	// A new fetch may start right away
	apitest.New().Handler(router).Delete("/api/v1/imports/monobank").Header("Authorization", auth).
		Expect(t).Status(http.StatusNotFound).End()
}
