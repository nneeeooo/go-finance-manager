package handlers_test

import (
	"encoding/json"
	"finance-api/internal/models"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/steinfletcher/apitest"
)

func call(t *testing.T, method, path, auth, body string) *http.Response {
	t.Helper()
	var res *http.Response
	req := apitest.New().Handler(router)
	var r *apitest.Request
	switch method {
	case http.MethodGet:
		r = req.Get(path)
	case http.MethodPost:
		r = req.Post(path)
	case http.MethodPatch:
		r = req.Patch(path)
	case http.MethodDelete:
		r = req.Delete(path)
	}
	if auth != "" {
		r = r.Header("Authorization", auth)
	}
	if body != "" {
		r = r.JSON(body)
	}
	r.Expect(t).Assert(func(r *http.Response, _ *http.Request) error { res = r; return nil }).End()
	return res
}

func expectStatus(t *testing.T, res *http.Response, want int, what string) {
	t.Helper()
	if res.StatusCode != want {
		t.Errorf("%s: status %d, want %d", what, res.StatusCode, want)
	}
}

type allowed struct {
	ID         string      `json:"id"`
	Email      string      `json:"email"`
	Role       models.Role `json:"role"`
	Registered bool        `json:"registered"`
}

func allowList(t *testing.T, res *http.Response) map[string]allowed {
	t.Helper()
	var list []allowed
	if err := json.NewDecoder(res.Body).Decode(&list); err != nil {
		t.Fatalf("allow-list reply: %v", err)
	}
	m := map[string]allowed{}
	for _, a := range list {
		m[a.Email] = a
	}
	return m
}

func TestDefaultAllowListAndAdmin(t *testing.T) {
	list := allowList(t, call(t, http.MethodGet, "/api/v1/admin/allowed-emails", adminHeader(t), ""))
	if list["lytvyn349@gmail.com"].Role != models.RoleAdmin || list["anastagoza@gmail.com"].Role != models.RoleRegular {
		t.Fatalf("seeded allow-list: %+v", list)
	}

	// Registering takes the role of the allow-list entry
	res := call(t, http.MethodPost, "/api/v1/auth/register", "", `{"email":"Lytvyn349@gmail.com","password":"password123"}`)
	expectStatus(t, res, http.StatusCreated, "admin registers")
	var tokens struct {
		AccessToken string `json:"access_token"`
	}
	_ = json.NewDecoder(res.Body).Decode(&tokens)
	var me struct {
		Email string      `json:"email"`
		Role  models.Role `json:"role"`
	}
	_ = json.NewDecoder(call(t, http.MethodGet, "/api/v1/users/me", "Bearer "+tokens.AccessToken, "").Body).Decode(&me)
	if me.Role != models.RoleAdmin || me.Email != "lytvyn349@gmail.com" {
		t.Fatalf("me: %+v", me)
	}
}

func TestRegularUserCannotUseAdminRoutes(t *testing.T) {
	user := authHeaderForUser(t, createTestUser(t))
	for _, c := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/admin/allowed-emails", ""},
		{http.MethodPost, "/api/v1/admin/allowed-emails", `{"email":"x@example.com"}`},
		{http.MethodGet, "/api/v1/users", ""},
		{http.MethodPost, "/api/v1/users", `{"email":"y@example.com","password":"password123"}`},
		{http.MethodDelete, "/api/v1/users/" + createTestUser(t), ""},
	} {
		expectStatus(t, call(t, c.method, c.path, user, c.body), http.StatusForbidden, c.method+" "+c.path)
	}
	// Without a login nothing under /users answers
	expectStatus(t, call(t, http.MethodPost, "/api/v1/users", "", `{"email":"z@example.com","password":"password123"}`), http.StatusUnauthorized, "anonymous POST /users")
}

func TestAdminManagesAllowList(t *testing.T) {
	admin := adminHeader(t)
	email := fmt.Sprintf("invited_%d@example.com", time.Now().UnixNano())

	// Not invited: cannot register
	expectStatus(t, call(t, http.MethodPost, "/api/v1/auth/register", "", fmt.Sprintf(`{"email":"%s","password":"password123"}`, email)), http.StatusForbidden, "uninvited register")

	list := allowList(t, call(t, http.MethodPost, "/api/v1/admin/allowed-emails", admin, fmt.Sprintf(`{"email":"%s"}`, email)))
	entry := list[email]
	if entry.Role != models.RoleRegular || entry.Registered {
		t.Fatalf("added: %+v", entry)
	}
	expectStatus(t, call(t, http.MethodPost, "/api/v1/admin/allowed-emails", admin, fmt.Sprintf(`{"email":"%s"}`, email)), http.StatusConflict, "duplicate invite")
	expectStatus(t, call(t, http.MethodPost, "/api/v1/admin/allowed-emails", admin, `{"email":"bad","role":"admin"}`), http.StatusBadRequest, "bad email")
	expectStatus(t, call(t, http.MethodPost, "/api/v1/admin/allowed-emails", admin, `{"email":"r@example.com","role":"owner"}`), http.StatusBadRequest, "bad role")

	// Invited: registers, then the admin promotes them and the role applies at once
	res := call(t, http.MethodPost, "/api/v1/auth/register", "", fmt.Sprintf(`{"email":"%s","password":"password123"}`, email))
	expectStatus(t, res, http.StatusCreated, "invited register")
	var tokens struct {
		AccessToken string `json:"access_token"`
	}
	_ = json.NewDecoder(res.Body).Decode(&tokens)
	invited := "Bearer " + tokens.AccessToken
	expectStatus(t, call(t, http.MethodGet, "/api/v1/admin/allowed-emails", invited, ""), http.StatusForbidden, "regular before promotion")

	list = allowList(t, call(t, http.MethodPatch, "/api/v1/admin/allowed-emails/"+entry.ID, admin, `{"role":"admin"}`))
	if list[email].Role != models.RoleAdmin || !list[email].Registered {
		t.Fatalf("promoted: %+v", list[email])
	}
	expectStatus(t, call(t, http.MethodGet, "/api/v1/admin/allowed-emails", invited, ""), http.StatusOK, "admin after promotion")

	// An admin cannot change or remove their own access
	selfList := allowList(t, call(t, http.MethodGet, "/api/v1/admin/allowed-emails", invited, ""))
	expectStatus(t, call(t, http.MethodPatch, "/api/v1/admin/allowed-emails/"+selfList[email].ID, invited, `{"role":"regular"}`), http.StatusBadRequest, "self demote")
	expectStatus(t, call(t, http.MethodDelete, "/api/v1/admin/allowed-emails/"+selfList[email].ID, invited, ""), http.StatusBadRequest, "self remove")

	// Removed: the existing token stops working at once
	allowList(t, call(t, http.MethodDelete, "/api/v1/admin/allowed-emails/"+entry.ID, admin, ""))
	expectStatus(t, call(t, http.MethodGet, "/api/v1/accounts", invited, ""), http.StatusUnauthorized, "removed user's token")
	expectStatus(t, call(t, http.MethodPost, "/api/v1/auth/login", "", fmt.Sprintf(`{"email":"%s","password":"password123"}`, email)), http.StatusUnauthorized, "removed user's login")
}

func TestUsersSelfOrAdmin(t *testing.T) {
	alice, bob := createTestUser(t), createTestUser(t)
	aliceAuth := authHeaderForUser(t, alice)

	expectStatus(t, call(t, http.MethodGet, "/api/v1/users/"+alice, aliceAuth, ""), http.StatusOK, "own user")
	expectStatus(t, call(t, http.MethodGet, "/api/v1/users/"+bob, aliceAuth, ""), http.StatusNotFound, "other user")
	expectStatus(t, call(t, http.MethodGet, "/api/v1/users/"+bob, adminHeader(t), ""), http.StatusOK, "admin reads anyone")
	expectStatus(t, call(t, http.MethodGet, "/api/v1/users", adminHeader(t), ""), http.StatusOK, "admin lists users")

	// Own password: needs the current one
	expectStatus(t, call(t, http.MethodPatch, "/api/v1/users/"+alice, aliceAuth, `{"password":"new-password-1"}`), http.StatusForbidden, "no current password")
	expectStatus(t, call(t, http.MethodPatch, "/api/v1/users/"+alice, aliceAuth, `{"password":"new-password-1","current_password":"password123"}`), http.StatusOK, "own password")
	expectStatus(t, call(t, http.MethodPatch, "/api/v1/users/"+alice, aliceAuth, `{"password":"short","current_password":"new-password-1"}`), http.StatusBadRequest, "short password")
	expectStatus(t, call(t, http.MethodPatch, "/api/v1/users/"+bob, aliceAuth, `{"password":"new-password-1"}`), http.StatusNotFound, "other's password")
	expectStatus(t, call(t, http.MethodPatch, "/api/v1/users/"+bob, adminHeader(t), `{"password":"reset-password-1"}`), http.StatusOK, "admin reset")

	expectStatus(t, call(t, http.MethodDelete, "/api/v1/users/"+adminUserID, adminHeader(t), ""), http.StatusBadRequest, "admin deletes self")
}

func TestAccountAndTransactionCRUDIsolation(t *testing.T) {
	owner, other := createTestUser(t), createTestUser(t)
	ownerAuth, otherAuth := authHeaderForUser(t, owner), authHeaderForUser(t, other)
	accountID := createTestAccountForUser(t, owner)
	txID := createTestTransaction(t, owner, accountID)

	// PATCH account
	res := call(t, http.MethodPatch, "/api/v1/accounts/"+accountID, ownerAuth, `{"name":"Renamed","external_id":"mono-ext-1","balance":12345}`)
	expectStatus(t, res, http.StatusOK, "patch own account")
	var acc struct {
		Name       string `json:"name"`
		ExternalID string `json:"external_id"`
		Balance    int64  `json:"balance"`
	}
	_ = json.NewDecoder(call(t, http.MethodGet, "/api/v1/accounts/"+accountID, ownerAuth, "").Body).Decode(&acc)
	if acc.Name != "Renamed" || acc.ExternalID != "mono-ext-1" || acc.Balance != 12345 {
		t.Fatalf("account after patch: %+v", acc)
	}
	expectStatus(t, call(t, http.MethodPatch, "/api/v1/accounts/"+accountID, otherAuth, `{"name":"Hacked"}`), http.StatusNotFound, "patch other's account")
	expectStatus(t, call(t, http.MethodPatch, "/api/v1/accounts/"+accountID, ownerAuth, `{"name":""}`), http.StatusBadRequest, "empty name")

	// GET / PATCH / DELETE transaction
	expectStatus(t, call(t, http.MethodGet, "/api/v1/transactions/"+txID, ownerAuth, ""), http.StatusOK, "get own transaction")
	expectStatus(t, call(t, http.MethodGet, "/api/v1/transactions/"+txID, otherAuth, ""), http.StatusNotFound, "get other's transaction")
	res = call(t, http.MethodPatch, "/api/v1/transactions/"+txID, ownerAuth, `{"category":"Groceries","comment":"weekly"}`)
	expectStatus(t, res, http.StatusOK, "patch own transaction")
	var tx struct {
		Category string `json:"category"`
		Comment  string `json:"comment"`
		Amount   int64  `json:"amount"`
	}
	_ = json.NewDecoder(call(t, http.MethodGet, "/api/v1/transactions/"+txID, ownerAuth, "").Body).Decode(&tx)
	if tx.Category != "Groceries" || tx.Comment != "weekly" || tx.Amount != 15000 {
		t.Fatalf("transaction after patch: %+v", tx)
	}
	expectStatus(t, call(t, http.MethodPatch, "/api/v1/transactions/"+txID, otherAuth, `{"comment":"x"}`), http.StatusNotFound, "patch other's transaction")
	expectStatus(t, call(t, http.MethodDelete, "/api/v1/transactions/"+txID, otherAuth, ""), http.StatusNotFound, "delete other's transaction")
	expectStatus(t, call(t, http.MethodDelete, "/api/v1/transactions/"+txID, ownerAuth, ""), http.StatusNoContent, "delete own transaction")
}
