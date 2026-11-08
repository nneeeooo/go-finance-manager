package sso

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"finance-api/internal/config"
)

const (
	SessionCookie = "fin_session" // the app session (random token; the DB holds its hash)
	LoginCookie   = "fin_oauth"   // state, nonce, PKCE verifier and return_to of a login in flight (signed)
	loginTTL      = 10 * time.Minute
)

// RandomToken: 32 random bytes, base64url
func RandomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

// LoginState: kept in the signed fin_oauth cookie between /auth/login and /auth/callback
type LoginState struct {
	State    string `json:"s"`
	Nonce    string `json:"n"`
	Verifier string `json:"v"`
	ReturnTo string `json:"r"`
	Expires  int64  `json:"e"`
}

func signingKey(cfg *config.Config) []byte {
	m := hmac.New(sha256.New, []byte(cfg.AuthClientSecret))
	m.Write([]byte("finance-api login state v1"))
	return m.Sum(nil)
}

// Seal: payload.signature (HMAC-SHA256 keyed from the client secret), so the cookie cannot be forged or swapped
func (s LoginState) Seal(cfg *config.Config) (string, error) {
	s.Expires = time.Now().Add(loginTTL).Unix()
	raw, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(raw)
	m := hmac.New(sha256.New, signingKey(cfg))
	m.Write([]byte(payload))
	return payload + "." + base64.RawURLEncoding.EncodeToString(m.Sum(nil)), nil
}

func OpenLoginState(cfg *config.Config, value string) (*LoginState, error) {
	payload, sig, ok := strings.Cut(value, ".")
	if !ok {
		return nil, errors.New("malformed login state")
	}
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		return nil, errors.New("malformed login state")
	}
	m := hmac.New(sha256.New, signingKey(cfg))
	m.Write([]byte(payload))
	if !hmac.Equal(got, m.Sum(nil)) {
		return nil, errors.New("bad login state signature")
	}
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return nil, errors.New("malformed login state")
	}
	var s LoginState
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, errors.New("malformed login state")
	}
	if time.Now().Unix() > s.Expires {
		return nil, errors.New("login state expired")
	}
	return &s, nil
}

// StateEqual: constant-time compare of the state in the URL with the one in the cookie
func StateEqual(a, b string) bool {
	return a != "" && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// SafeReturnTo: only a path on this site is allowed ("/x?y#z"); anything else (absolute URL, "//host",
// "/\host", control characters) becomes "/".
func SafeReturnTo(raw string) string {
	if raw == "" || raw[0] != '/' || strings.HasPrefix(raw, "//") || strings.Contains(raw, "\\") {
		return "/"
	}
	for _, r := range raw {
		if r < 0x20 || r == 0x7f {
			return "/"
		}
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "" || u.Host != "" || u.User != nil || !strings.HasPrefix(u.Path, "/") || strings.HasPrefix(u.Path, "//") {
		return "/"
	}
	return raw
}

// Secure: cookies get Secure when the app is served over https (AUTH_REDIRECT_URL says so)
func Secure(cfg *config.Config) bool {
	u, err := url.Parse(cfg.AuthRedirectURL)
	return err == nil && u.Scheme == "https"
}

// AppOrigin: scheme://host of the app as the browser sees it (from AUTH_REDIRECT_URL)
func AppOrigin(cfg *config.Config) string {
	u, err := url.Parse(cfg.AuthRedirectURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

func SetCookie(w http.ResponseWriter, cfg *config.Config, name, value, path string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: value, Path: path, MaxAge: maxAge,
		HttpOnly: true, Secure: Secure(cfg), SameSite: http.SameSiteLaxMode,
	})
}

func ClearCookie(w http.ResponseWriter, cfg *config.Config, name, path string) {
	SetCookie(w, cfg, name, "", path, -1)
}

// CSRFOK: for unsafe methods on cookie-authenticated requests. SameSite=Lax already keeps the cookie off
// cross-site POSTs; on top of that an Origin header, when present, must be this app's origin, and without Origin
// the request must carry the custom X-Requested-With header (a cross-site page cannot add it without CORS).
func CSRFOK(cfg *config.Config, r *http.Request) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		want := AppOrigin(cfg)
		return want != "" && origin == want
	}
	return r.Header.Get("X-Requested-With") != ""
}
