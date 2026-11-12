package sso

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDiscoverInternalBackChannel(t *testing.T) {
	const pub = "https://auth.example"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                pub,
			"authorization_endpoint":                pub + "/authorize",
			"token_endpoint":                        pub + "/token",
			"jwks_uri":                              pub + "/.well-known/jwks.json",
			"userinfo_endpoint":                     pub + "/userinfo",
			"revocation_endpoint":                   pub + "/revoke",
			"end_session_endpoint":                  pub + "/logout",
			"id_token_signing_alg_values_supported": []string{"EdDSA"},
		})
	}))
	defer srv.Close()

	p, endSession, revocation, err := discover(context.Background(), pub, srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	ep := p.Endpoint()
	for name, c := range map[string][2]string{
		"authorize (browser, public)":   {ep.AuthURL, pub + "/authorize"},
		"end_session (browser, public)": {endSession, pub + "/logout"},
		"token (internal)":              {ep.TokenURL, srv.URL + "/token"},
		"revocation (internal)":         {revocation, srv.URL + "/revoke"},
		"userinfo (internal)":           {p.UserInfoEndpoint(), srv.URL + "/userinfo"},
	} {
		if c[0] != c[1] {
			t.Errorf("%s: got %q, want %q", name, c[0], c[1])
		}
	}

	// A document naming another issuer is refused
	if _, _, _, err := discover(context.Background(), "https://other.example", srv.URL); err == nil {
		t.Error("issuer mismatch accepted")
	}
}
