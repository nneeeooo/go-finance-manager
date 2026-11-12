// Package sso: login through the central auth service (OIDC authorization code flow + PKCE, BFF pattern). The
// backend does the code exchange and keeps the tokens in a server-side session; the browser gets only the app's
// own session cookie.
package sso

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"finance-api/internal/config"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

var (
	// ErrInvalidGrant: the auth service refuses the refresh token (reused, expired or revoked); the session is dead
	ErrInvalidGrant = errors.New("invalid_grant")
	// ErrUpstream: the auth service is unreachable or failed; the session may still be fine
	ErrUpstream = errors.New("auth service unavailable")
)

// clockSkew tolerated when checking token times (contract: 60 s)
const clockSkew = 60 * time.Second

// Client talks to the auth service. Discovery is lazy and retried, so the API starts even when auth is down.
type Client struct {
	cfg *config.Config

	mu         sync.Mutex
	provider   *oidc.Provider
	oauth      oauth2.Config
	endSession string
	revocation string
}

func NewClient(cfg *config.Config) *Client { return &Client{cfg: cfg} }

func (c *Client) Config() *config.Config { return c.cfg }

func (c *Client) ensure(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.provider != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	p, endSession, revocation, err := discover(ctx, c.cfg.AuthIssuer, c.cfg.AuthInternalURL)
	if err != nil {
		return fmt.Errorf("%w: discovery: %v", ErrUpstream, err)
	}
	ep := p.Endpoint()
	ep.AuthStyle = oauth2.AuthStyleInHeader // client_secret_basic
	c.provider = p
	c.endSession = endSession
	c.revocation = revocation
	c.oauth = oauth2.Config{
		ClientID:     c.cfg.AuthClientID,
		ClientSecret: c.cfg.AuthClientSecret,
		RedirectURL:  c.cfg.AuthRedirectURL,
		Endpoint:     ep,
		Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
	}
	return nil
}

// AuthCodeURL: where to send the browser to log in. provider picks the upstream at auth ("google", "telegram");
// anything else is left out and auth uses Google
func (c *Client) AuthCodeURL(ctx context.Context, state, nonce, verifier, provider string) (string, error) {
	if err := c.ensure(ctx); err != nil {
		return "", err
	}
	opts := []oauth2.AuthCodeOption{oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier)}
	if provider == "google" || provider == "telegram" {
		opts = append(opts, oauth2.SetAuthURLParam("provider", provider))
	}
	return c.oauth.AuthCodeURL(state, opts...), nil
}

// Identity: what the verified id_token says about the user
type Identity struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	Nonce         string
}

// Exchange trades the code for tokens (server-side, with the PKCE verifier) and verifies the id_token: signature
// against the issuer's JWKS, issuer, audience, expiry.
func (c *Client) Exchange(ctx context.Context, code, verifier string) (*oauth2.Token, *Identity, error) {
	if err := c.ensure(ctx); err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	tok, err := c.oauth.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		var re *oauth2.RetrieveError
		if errors.As(err, &re) && re.Response != nil && re.Response.StatusCode < 500 {
			return nil, nil, fmt.Errorf("code exchange rejected: %w", err)
		}
		return nil, nil, fmt.Errorf("%w: code exchange: %v", ErrUpstream, err)
	}
	raw, _ := tok.Extra("id_token").(string)
	if raw == "" || tok.RefreshToken == "" {
		return nil, nil, errors.New("token response without id_token or refresh_token")
	}
	verifier2 := c.provider.Verifier(&oidc.Config{
		ClientID:             c.cfg.AuthClientID,
		SupportedSigningAlgs: []string{"EdDSA"},
		Now:                  func() time.Time { return time.Now().Add(-clockSkew) }, // accept tokens expired by up to 60 s
	})
	idt, err := verifier2.Verify(ctx, raw)
	if err != nil {
		return nil, nil, fmt.Errorf("id_token: %w", err)
	}
	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Nonce         string `json:"nonce"`
	}
	if err := idt.Claims(&claims); err != nil {
		return nil, nil, fmt.Errorf("id_token claims: %w", err)
	}
	return tok, &Identity{
		Subject: idt.Subject, Email: strings.ToLower(strings.TrimSpace(claims.Email)),
		EmailVerified: claims.EmailVerified, Name: claims.Name, Nonce: claims.Nonce,
	}, nil
}

// Refresh trades a refresh token for new tokens. ErrInvalidGrant when the auth service says invalid_grant.
func (c *Client) Refresh(ctx context.Context, refreshToken string) (*oauth2.Token, error) {
	if err := c.ensure(ctx); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	// No access token: the token source must refresh
	tok, err := c.oauth.TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken}).Token()
	if err != nil {
		var re *oauth2.RetrieveError
		if errors.As(err, &re) && re.ErrorCode == "invalid_grant" {
			return nil, ErrInvalidGrant
		}
		return nil, fmt.Errorf("%w: refresh: %v", ErrUpstream, err)
	}
	return tok, nil
}

// Revoke tells the auth service to kill the refresh token's family (RFC 7009; best effort)
func (c *Client) Revoke(ctx context.Context, refreshToken string) error {
	if err := c.ensure(ctx); err != nil {
		return err
	}
	if c.revocation == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	form := url.Values{"token": {refreshToken}, "token_type_hint": {"refresh_token"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.revocation, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(url.QueryEscape(c.cfg.AuthClientID), url.QueryEscape(c.cfg.AuthClientSecret))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// EndSessionURL: the auth service's logout page (ends its SSO session), which sends the browser back to
// AUTH_POST_LOGOUT_URL. Empty when the auth service is unreachable.
func (c *Client) EndSessionURL(ctx context.Context) string {
	if err := c.ensure(ctx); err != nil || c.endSession == "" {
		return ""
	}
	q := url.Values{"client_id": {c.cfg.AuthClientID}}
	if c.cfg.AuthPostLogoutURL != "" {
		q.Set("post_logout_redirect_uri", c.cfg.AuthPostLogoutURL)
	}
	sep := "?"
	if strings.Contains(c.endSession, "?") {
		sep = "&"
	}
	return c.endSession + sep + q.Encode()
}
