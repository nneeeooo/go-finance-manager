package handlers_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"finance-api/internal/models"
	"finance-api/internal/routes"
	"finance-api/internal/sso"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/steinfletcher/apitest"
)

// fakeAuth: a stand-in for the central auth service (discovery, JWKS with an Ed25519 key, token, revoke), so the
// SSO tests run without the real service.
type fakeAuth struct {
	*httptest.Server
	key ed25519.PrivateKey

	mu            sync.Mutex
	codes         map[string]fakeCode // code -> what the id_token will say
	refreshMode   string              // "" = rotate fine, "invalid_grant" = refuse
	refreshCalls  int
	revoked       []string
	issuedRefresh int
}

type fakeCode struct {
	challenge string
	nonce     string
	sub       string
	email     string
	verified  bool
}

const (
	fakeClientID     = "go-finance"
	fakeClientSecret = "dev-finance-secret"
	fakeRedirectURL  = "http://localhost:5173/auth/callback"
)

func newFakeAuth(t *testing.T) *fakeAuth {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeAuth{key: key, codes: map[string]fakeCode{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"issuer":                                f.URL,
			"authorization_endpoint":                f.URL + "/authorize",
			"token_endpoint":                        f.URL + "/token",
			"jwks_uri":                              f.URL + "/.well-known/jwks.json",
			"userinfo_endpoint":                     f.URL + "/userinfo",
			"revocation_endpoint":                   f.URL + "/revoke",
			"end_session_endpoint":                  f.URL + "/logout",
			"id_token_signing_alg_values_supported": []string{"EdDSA"},
			"code_challenge_methods_supported":      []string{"S256"},
			"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
			"response_types_supported":              []string{"code"},
			"subject_types_supported":               []string{"public"},
		})
	})
	mux.HandleFunc("/.well-known/jwks.json", func(w http.ResponseWriter, r *http.Request) {
		pub := key.Public().(ed25519.PublicKey)
		writeJSON(w, http.StatusOK, map[string]any{"keys": []map[string]string{{
			"kty": "OKP", "crv": "Ed25519", "alg": "EdDSA", "use": "sig", "kid": "k1",
			"x": base64.RawURLEncoding.EncodeToString(pub),
		}}})
	})
	mux.HandleFunc("/token", f.token)
	mux.HandleFunc("/revoke", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		f.mu.Lock()
		f.revoked = append(f.revoked, r.PostForm.Get("token"))
		f.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})
	f.Server = httptest.NewServer(mux)
	t.Cleanup(f.Close)
	return f
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (f *fakeAuth) token(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if id, secret, ok := r.BasicAuth(); !ok || id != fakeClientID || secret != fakeClientSecret {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_client"})
		return
	}
	_ = r.ParseForm()
	switch r.PostForm.Get("grant_type") {
	case "authorization_code":
		code, ok := f.codes[r.PostForm.Get("code")]
		delete(f.codes, r.PostForm.Get("code"))
		sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
		if !ok || r.PostForm.Get("redirect_uri") != fakeRedirectURL ||
			base64.RawURLEncoding.EncodeToString(sum[:]) != code.challenge {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_grant"})
			return
		}
		idToken := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims{
			"iss": f.URL, "sub": code.sub, "aud": fakeClientID,
			"exp": time.Now().Add(10 * time.Minute).Unix(), "iat": time.Now().Unix(),
			"email": code.email, "email_verified": code.verified, "name": "Test User", "nonce": code.nonce,
		})
		idToken.Header["kid"] = "k1"
		signed, err := idToken.SignedString(f.key)
		if err != nil {
			panic(err)
		}
		f.issuedRefresh++
		writeJSON(w, http.StatusOK, map[string]any{
			"access_token": "at-" + code.sub, "id_token": signed, "token_type": "Bearer", "expires_in": 600,
			"refresh_token": fmt.Sprintf("rt-%d", f.issuedRefresh),
		})
	case "refresh_token":
		f.refreshCalls++
		if f.refreshMode == "invalid_grant" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_grant", "error_description": "reused"})
			return
		}
		f.issuedRefresh++
		writeJSON(w, http.StatusOK, map[string]any{
			"access_token": "at-refreshed", "token_type": "Bearer", "expires_in": 600,
			"refresh_token": fmt.Sprintf("rt-%d", f.issuedRefresh),
		})
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported_grant_type"})
	}
}

// ssoEnv: a router with SSO on against the fake auth service (the shared test DB)
type ssoEnv struct {
	t      *testing.T
	auth   *fakeAuth
	router *gin.Engine
}

func newSSOEnv(t *testing.T) *ssoEnv {
	t.Helper()
	fa := newFakeAuth(t)
	cfg := *testCfg
	cfg.AuthIssuer = fa.URL
	cfg.AuthClientID = fakeClientID
	cfg.AuthClientSecret = fakeClientSecret
	cfg.AuthRedirectURL = fakeRedirectURL
	cfg.AuthPostLogoutURL = "http://localhost:5173/"
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("db", testDB)
		c.Set("config", &cfg)
		c.Next()
	})
	routes.Register(r, &cfg)
	return &ssoEnv{t: t, auth: fa, router: r}
}

func uniqueEmail() string { return fmt.Sprintf("sso_%d@example.com", time.Now().UnixNano()) }

type result struct {
	status   int
	location string
	cookies  map[string]*http.Cookie
	body     string
}

func (e *ssoEnv) do(method, target string, mod func(*http.Request)) result {
	e.t.Helper()
	req := httptest.NewRequest(method, target, nil)
	if mod != nil {
		mod(req)
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	res := rec.Result()
	out := result{status: res.StatusCode, location: res.Header.Get("Location"), cookies: map[string]*http.Cookie{}, body: rec.Body.String()}
	for _, c := range res.Cookies() {
		out.cookies[c.Name] = c
	}
	return out
}

// startLogin calls /auth/login and returns the fin_oauth cookie plus the state, nonce and challenge the browser
// would send to the auth service
func (e *ssoEnv) startLogin(returnTo string) (loginCookie *http.Cookie, state, nonce, challenge string) {
	e.t.Helper()
	res := e.do(http.MethodGet, "/auth/login?return_to="+url.QueryEscape(returnTo), nil)
	if res.status != http.StatusFound {
		e.t.Fatalf("/auth/login: status %d, body %s", res.status, res.body)
	}
	loc, err := url.Parse(res.location)
	if err != nil || !strings.HasPrefix(res.location, e.auth.URL+"/authorize") {
		e.t.Fatalf("login redirect goes elsewhere: %s", res.location)
	}
	q := loc.Query()
	if q.Get("client_id") != fakeClientID || q.Get("redirect_uri") != fakeRedirectURL || q.Get("response_type") != "code" ||
		q.Get("code_challenge_method") != "S256" || !strings.Contains(q.Get("scope"), "openid") {
		e.t.Fatalf("bad authorize params: %s", res.location)
	}
	c := res.cookies[sso.LoginCookie]
	if c == nil || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode {
		e.t.Fatalf("login state cookie missing or not HttpOnly/Lax: %+v", c)
	}
	return c, q.Get("state"), q.Get("nonce"), q.Get("code_challenge")
}

// login runs the whole flow with the identity and returns the callback result
func (e *ssoEnv) login(returnTo, sub, email string, verified bool) result {
	e.t.Helper()
	lc, state, nonce, challenge := e.startLogin(returnTo)
	code := fmt.Sprintf("code-%d", time.Now().UnixNano())
	e.auth.mu.Lock()
	e.auth.codes[code] = fakeCode{challenge: challenge, nonce: nonce, sub: sub, email: email, verified: verified}
	e.auth.mu.Unlock()
	return e.do(http.MethodGet, "/auth/callback?code="+code+"&state="+url.QueryEscape(state), func(r *http.Request) { r.AddCookie(lc) })
}

func (e *ssoEnv) loginOK(sub, email string) *http.Cookie {
	e.t.Helper()
	res := e.login("/", sub, email, true)
	c := res.cookies[sso.SessionCookie]
	if res.status != http.StatusFound || c == nil {
		e.t.Fatalf("login failed: status %d location %q body %s", res.status, res.location, res.body)
	}
	return c
}

func countSessions(t *testing.T, email string) int64 {
	t.Helper()
	var n int64
	if err := testDB.Model(&models.Session{}).Joins("JOIN users ON users.id = sessions.user_id").
		Where("users.email = ?", email).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func allowEmail(t *testing.T, email string, role models.Role) {
	t.Helper()
	if err := testDB.Create(&models.AllowedEmail{Email: email, Role: role}).Error; err != nil {
		t.Fatal(err)
	}
}

func TestSSOCallbackRejectsStateMismatch(t *testing.T) {
	e := newSSOEnv(t)
	email := uniqueEmail()
	allowEmail(t, email, models.RoleRegular)

	lc, _, nonce, challenge := e.startLogin("/")
	e.auth.codes["c1"] = fakeCode{challenge: challenge, nonce: nonce, sub: "sub-state", email: email, verified: true}

	// wrong state
	res := e.do(http.MethodGet, "/auth/callback?code=c1&state=attacker-state", func(r *http.Request) { r.AddCookie(lc) })
	if res.status != http.StatusBadRequest || res.cookies[sso.SessionCookie] != nil {
		t.Fatalf("state mismatch: status %d, session cookie %v", res.status, res.cookies[sso.SessionCookie])
	}
	// no login cookie at all
	res = e.do(http.MethodGet, "/auth/callback?code=c1&state=whatever", nil)
	if res.status != http.StatusBadRequest || res.cookies[sso.SessionCookie] != nil {
		t.Fatalf("no state cookie: status %d", res.status)
	}
	// forged cookie
	forged := &http.Cookie{Name: sso.LoginCookie, Value: lc.Value[:len(lc.Value)-2] + "xx"}
	res = e.do(http.MethodGet, "/auth/callback?code=c1&state=whatever", func(r *http.Request) { r.AddCookie(forged) })
	if res.status != http.StatusBadRequest {
		t.Fatalf("forged state cookie: status %d", res.status)
	}
	if n := countSessions(t, email); n != 0 {
		t.Fatalf("%d sessions after rejected callbacks", n)
	}
	// the code was never spent
	if _, ok := e.auth.codes["c1"]; !ok {
		t.Fatal("code was exchanged although state did not match")
	}
}

func TestSSOReturnToIsRelativeOnly(t *testing.T) {
	e := newSSOEnv(t)
	email := uniqueEmail()
	allowEmail(t, email, models.RoleRegular)

	cases := map[string]string{
		"/accounts?x=1":          "/accounts?x=1",
		"https://evil.example/":  "/",
		"//evil.example/path":    "/",
		"/\\evil.example":        "/",
		"http:evil.example":      "/",
		"javascript:alert(1)":    "/",
		"accounts":               "/",
		"/ok\r\nSet-Cookie: a=b": "/",
		"":                       "/",
		"/%2F%2Fevil.example/x":  "/", // decodes to //evil.example
		"///evil.example":        "/",
		"/\t/evil.example":       "/",
	}
	for in, want := range cases {
		res := e.login(in, "sub-return", email, true)
		if res.status != http.StatusFound || res.location != want {
			t.Errorf("return_to %q: want redirect to %q, got %d %q", in, want, res.status, res.location)
		}
	}
}

func TestSSOLinksExistingUserByVerifiedEmail(t *testing.T) {
	e := newSSOEnv(t)
	existing := insertUser(t, models.RoleAdmin) // password user with data
	accountID := createTestAccountForUser(t, existing)
	var before models.User
	if err := testDB.First(&before, "id = ?", existing).Error; err != nil {
		t.Fatal(err)
	}

	// an unverified email never links
	res := e.login("/", "sub-link", before.Email, false)
	if res.cookies[sso.SessionCookie] != nil || !strings.Contains(res.location, "auth_error=") {
		t.Fatalf("unverified email: %d %q", res.status, res.location)
	}

	cookie := e.loginOK("sub-link", strings.ToUpper(before.Email)) // case-insensitive
	var after models.User
	if err := testDB.First(&after, "id = ?", existing).Error; err != nil {
		t.Fatal(err)
	}
	if after.AuthSub == nil || *after.AuthSub != "sub-link" {
		t.Fatalf("auth_sub not linked: %v", after.AuthSub)
	}
	if after.PasswordHash != before.PasswordHash || after.Role != models.RoleAdmin || after.Email != before.Email {
		t.Fatal("linking changed the user's data")
	}
	var users int64
	testDB.Model(&models.User{}).Where("LOWER(email) = ?", before.Email).Count(&users)
	if users != 1 {
		t.Fatalf("%d users with the email, want 1", users)
	}
	var accounts int64
	testDB.Model(&models.Account{}).Where("id = ?", accountID).Count(&accounts)
	if accounts != 1 {
		t.Fatal("account of the linked user is gone")
	}

	// the session reaches the user's data and /auth/me
	me := e.do(http.MethodGet, "/auth/me", func(r *http.Request) { r.AddCookie(cookie) })
	if me.status != http.StatusOK || !strings.Contains(me.body, existing) || !strings.Contains(me.body, `"admin"`) {
		t.Fatalf("/auth/me: %d %s", me.status, me.body)
	}
	list := e.do(http.MethodGet, "/api/v1/accounts", func(r *http.Request) { r.AddCookie(cookie) })
	if list.status != http.StatusOK || !strings.Contains(list.body, accountID) {
		t.Fatalf("accounts via session: %d %s", list.status, list.body)
	}
	if c := cookie; !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/" {
		t.Fatalf("session cookie flags: %+v", c)
	}

	// next login resolves by the stored sub, even when the email changed at auth
	cookie2 := e.loginOK("sub-link", "renamed@example.com")
	me = e.do(http.MethodGet, "/auth/me", func(r *http.Request) { r.AddCookie(cookie2) })
	if me.status != http.StatusOK || !strings.Contains(me.body, existing) {
		t.Fatalf("login by sub: %d %s", me.status, me.body)
	}

	// another sub with the same email does not hijack the linked user
	res = e.login("/", "sub-other", before.Email, true)
	if res.cookies[sso.SessionCookie] != nil {
		t.Fatal("a second auth subject took over a linked user")
	}
}

func TestSSOCreatesUserOnlyFromAllowList(t *testing.T) {
	e := newSSOEnv(t)

	stranger := uniqueEmail()
	res := e.login("/", "sub-stranger", stranger, true)
	if res.cookies[sso.SessionCookie] != nil || res.location != "/?auth_error=not_invited" {
		t.Fatalf("stranger: %d %q", res.status, res.location)
	}
	var n int64
	testDB.Model(&models.User{}).Where("email = ?", stranger).Count(&n)
	if n != 0 {
		t.Fatal("user created without allow-list entry")
	}

	invited := uniqueEmail()
	allowEmail(t, invited, models.RoleRegular)
	e.loginOK("sub-invited", invited)
	var u models.User
	if err := testDB.First(&u, "email = ?", invited).Error; err != nil {
		t.Fatal(err)
	}
	if u.AuthSub == nil || *u.AuthSub != "sub-invited" || u.Role != models.RoleRegular || u.PasswordHash != "!" {
		t.Fatalf("created user: %+v", u)
	}
}

func TestSSORefreshInvalidGrantDropsSession(t *testing.T) {
	e := newSSOEnv(t)
	email := uniqueEmail()
	allowEmail(t, email, models.RoleRegular)
	cookie := e.loginOK("sub-refresh", email)
	get := func() result {
		return e.do(http.MethodGet, "/auth/me", func(r *http.Request) { r.AddCookie(cookie) })
	}
	expire := func() {
		if err := testDB.Model(&models.Session{}).Joins("JOIN users ON users.id = sessions.user_id").
			Where("users.email = ?", email).Update("access_token_expires_at", time.Now().Add(-time.Minute)).Error; err != nil {
			// gorm refuses UPDATE with JOIN on some drivers: fall back to a subquery
			testDB.Exec(`UPDATE sessions SET access_token_expires_at = ? WHERE user_id = (SELECT id FROM users WHERE email = ?)`,
				time.Now().Add(-time.Minute), email)
		}
	}

	// fresh token: no call to auth
	if res := get(); res.status != http.StatusOK || e.auth.refreshCalls != 0 {
		t.Fatalf("fresh session: %d, refresh calls %d", res.status, e.auth.refreshCalls)
	}

	// expired token: refreshed server-side, refresh token rotated in the DB
	expire()
	if res := get(); res.status != http.StatusOK || e.auth.refreshCalls != 1 {
		t.Fatalf("refresh: %d %s, calls %d", res.status, res.body, e.auth.refreshCalls)
	}
	var s models.Session
	testDB.Joins("JOIN users ON users.id = sessions.user_id").Where("users.email = ?", email).First(&s)
	if s.RefreshToken == "rt-1" || !s.AccessTokenExpiresAt.After(time.Now().Add(5*time.Minute)) {
		t.Fatalf("session not updated after refresh: %+v", s)
	}

	// auth answers invalid_grant: the session is dropped, the cookie cleared, the user must log in again
	expire()
	e.auth.refreshMode = "invalid_grant"
	res := get()
	if res.status != http.StatusUnauthorized {
		t.Fatalf("invalid_grant: status %d", res.status)
	}
	if c := res.cookies[sso.SessionCookie]; c == nil || c.MaxAge >= 0 {
		t.Fatalf("session cookie not cleared: %+v", c)
	}
	if n := countSessions(t, email); n != 0 {
		t.Fatalf("session row survived invalid_grant (%d)", n)
	}
	e.auth.refreshMode = ""
	if res := get(); res.status != http.StatusUnauthorized {
		t.Fatalf("dropped session still works: %d", res.status)
	}
}

func TestSSOCSRF(t *testing.T) {
	e := newSSOEnv(t)
	email := uniqueEmail()
	allowEmail(t, email, models.RoleRegular)
	cookie := e.loginOK("sub-csrf", email)
	var user models.User
	testDB.First(&user, "email = ?", email)
	body := fmt.Sprintf(`{"user_id":"%s","bank":1,"name":"CSRF","currency":980}`, user.ID)

	post := func(mod func(*http.Request)) result {
		return e.do(http.MethodPost, "/api/v1/accounts", func(r *http.Request) {
			r.AddCookie(cookie)
			r.Header.Set("Content-Type", "application/json")
			r.Body = httpBody(body)
			mod(r)
		})
	}
	// cookie only: rejected
	if res := post(func(r *http.Request) {}); res.status != http.StatusForbidden {
		t.Fatalf("no origin, no header: %d", res.status)
	}
	// another site's Origin: rejected, even with the header
	if res := post(func(r *http.Request) {
		r.Header.Set("Origin", "https://evil.example")
		r.Header.Set("X-Requested-With", "x")
	}); res.status != http.StatusForbidden {
		t.Fatalf("foreign origin: %d", res.status)
	}
	// custom header: allowed
	if res := post(func(r *http.Request) { r.Header.Set("X-Requested-With", "fetch") }); res.status != http.StatusCreated {
		t.Fatalf("with header: %d %s", res.status, res.body)
	}
	// the app's own origin: allowed
	if res := post(func(r *http.Request) { r.Header.Set("Origin", "http://localhost:5173") }); res.status != http.StatusCreated {
		t.Fatalf("own origin: %d %s", res.status, res.body)
	}
	// safe methods need nothing
	if res := e.do(http.MethodGet, "/api/v1/accounts", func(r *http.Request) { r.AddCookie(cookie) }); res.status != http.StatusOK {
		t.Fatalf("GET: %d", res.status)
	}

	// logout is checked the same way and leaves the session alone when rejected
	if res := e.do(http.MethodPost, "/auth/logout", func(r *http.Request) { r.AddCookie(cookie) }); res.status != http.StatusForbidden {
		t.Fatalf("logout without header: %d", res.status)
	}
	if countSessions(t, email) != 1 {
		t.Fatal("rejected logout deleted the session")
	}
	res := e.do(http.MethodPost, "/auth/logout", func(r *http.Request) {
		r.AddCookie(cookie)
		r.Header.Set("X-Requested-With", "fetch")
	})
	var out struct {
		RedirectURL string `json:"redirect_url"`
	}
	_ = json.Unmarshal([]byte(res.body), &out)
	if res.status != http.StatusOK || !strings.HasPrefix(out.RedirectURL, e.auth.URL+"/logout?") ||
		!strings.Contains(out.RedirectURL, "client_id=go-finance") || !strings.Contains(out.RedirectURL, "post_logout_redirect_uri=") {
		t.Fatalf("logout: %d %s", res.status, res.body)
	}
	if countSessions(t, email) != 0 || len(e.auth.revoked) != 1 || e.auth.revoked[0] != "rt-1" {
		t.Fatalf("logout: sessions %d, revoked %v", countSessions(t, email), e.auth.revoked)
	}
	if c := res.cookies[sso.SessionCookie]; c == nil || c.MaxAge >= 0 {
		t.Fatalf("cookie not cleared: %+v", c)
	}
}

func TestSSOPasswordLoginOffBearerKept(t *testing.T) {
	e := newSSOEnv(t)
	userID := createTestUser(t)

	for _, path := range []string{"/api/v1/auth/login", "/api/v1/auth/register", "/api/v1/auth/refresh"} {
		res := e.do(http.MethodPost, path, nil)
		if res.status != http.StatusForbidden {
			t.Errorf("%s with SSO on: %d", path, res.status)
		}
	}
	// the app's own Bearer JWT (scripts, tests) still passes the guard
	res := e.do(http.MethodGet, "/api/v1/users/me", func(r *http.Request) { r.Header.Set("Authorization", authHeaderForUser(t, userID)) })
	if res.status != http.StatusOK {
		t.Fatalf("bearer fallback: %d", res.status)
	}
	// no credentials at all
	if res := e.do(http.MethodGet, "/api/v1/users/me", nil); res.status != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", res.status)
	}
}

// apitest is the repo's HTTP test helper: one check of the SSO-off router (the shared one) that the SSO routes
// are absent there
func TestSSORoutesOffWithoutIssuer(t *testing.T) {
	apitest.New().Handler(router).Get("/auth/login").Expect(t).Status(http.StatusNotFound).End()
}

func httpBody(s string) io.ReadCloser { return io.NopCloser(strings.NewReader(s)) }
