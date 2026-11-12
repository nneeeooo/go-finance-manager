package sso

import (
	"context"
	"fmt"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
)

// discover runs OIDC discovery and returns the provider plus the end-session and revocation endpoints.
//
// With internal set (e.g. http://auth-app:8090 on the podman network) the back channel never leaves the host:
// discovery, token, JWKS, userinfo and revocation go to the internal URL. The issuer the tokens must carry and the
// browser endpoints (authorize, end_session) stay public.
func discover(ctx context.Context, issuer, internal string) (p *oidc.Provider, endSession, revocation string, err error) {
	if internal == "" {
		p, err = oidc.NewProvider(ctx, issuer)
	} else {
		// Fetched from the internal URL; go-oidc then skips its issuer check, so it is done below
		p, err = oidc.NewProvider(oidc.InsecureIssuerURLContext(ctx, issuer), internal)
	}
	if err != nil {
		return nil, "", "", err
	}
	var d struct {
		Issuer     string   `json:"issuer"`
		Auth       string   `json:"authorization_endpoint"`
		Token      string   `json:"token_endpoint"`
		JWKS       string   `json:"jwks_uri"`
		UserInfo   string   `json:"userinfo_endpoint"`
		EndSession string   `json:"end_session_endpoint"`
		Revocation string   `json:"revocation_endpoint"`
		Algs       []string `json:"id_token_signing_alg_values_supported"`
	}
	if err := p.Claims(&d); err != nil {
		return nil, "", "", err
	}
	if strings.TrimRight(d.Issuer, "/") != issuer {
		return nil, "", "", fmt.Errorf("discovery names issuer %q, want %q", d.Issuer, issuer)
	}
	if internal == "" {
		return p, d.EndSession, d.Revocation, nil
	}
	back := func(u string) string {
		if strings.HasPrefix(u, issuer+"/") {
			return internal + strings.TrimPrefix(u, issuer)
		}
		return u
	}
	pc := oidc.ProviderConfig{
		IssuerURL:   issuer,
		AuthURL:     d.Auth,
		TokenURL:    back(d.Token),
		JWKSURL:     back(d.JWKS),
		UserInfoURL: back(d.UserInfo),
		Algorithms:  d.Algs,
	}
	return pc.NewProvider(ctx), d.EndSession, back(d.Revocation), nil
}
