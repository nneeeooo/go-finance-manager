package handlers_test

import (
	"encoding/json"
	auth_dto "finance-api/internal/dto"
	"finance-api/internal/models"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/steinfletcher/apitest"
)

func TestAuthRegisterWhitelist(t *testing.T) {
	// 1. Whitelisted email
	whitelistedEmail := fmt.Sprintf("whitelisted_%d@example.com", time.Now().UnixNano())
	testDB.Create(&models.AllowedEmail{
		Email: strings.ToLower(whitelistedEmail),
	})

	var resp auth_dto.AccessTokenResponse
	var setCookieHeader string

	apitest.New().
		Handler(router).
		Post("/api/v1/auth/register").
		JSON(fmt.Sprintf(`{
			"email":"%s",
			"password":"securepassword123"
		}`, whitelistedEmail)).
		Expect(t).
		Status(http.StatusCreated).
		Assert(func(res *http.Response, _ *http.Request) error {
			setCookieHeader = res.Header.Get("Set-Cookie")
			if !strings.Contains(setCookieHeader, "refresh_token=") {
				return fmt.Errorf("expected refresh_token cookie, got: %s", setCookieHeader)
			}
			return json.NewDecoder(res.Body).Decode(&resp)
		}).
		End()

	if resp.AccessToken == "" {
		t.Fatal("expected access token to be returned")
	}

	// 2. Non-whitelisted email -> 403 Forbidden
	notAllowedEmail := fmt.Sprintf("stranger_%d@example.com", time.Now().UnixNano())
	apitest.New().
		Handler(router).
		Post("/api/v1/auth/register").
		JSON(fmt.Sprintf(`{
			"email":"%s",
			"password":"securepassword123"
		}`, notAllowedEmail)).
		Expect(t).
		Status(http.StatusForbidden).
		End()

	// 3. Duplicate email -> 409 Conflict
	apitest.New().
		Handler(router).
		Post("/api/v1/auth/register").
		JSON(fmt.Sprintf(`{
			"email":"%s",
			"password":"securepassword123"
		}`, whitelistedEmail)).
		Expect(t).
		Status(http.StatusConflict).
		End()

	// 4. Short password -> 400 Bad Request
	shortPassEmail := fmt.Sprintf("short_%d@example.com", time.Now().UnixNano())
	testDB.Create(&models.AllowedEmail{
		Email: strings.ToLower(shortPassEmail),
	})
	apitest.New().
		Handler(router).
		Post("/api/v1/auth/register").
		JSON(fmt.Sprintf(`{
			"email":"%s",
			"password":"123"
		}`, shortPassEmail)).
		Expect(t).
		Status(http.StatusBadRequest).
		End()
}

func TestAuthLogin(t *testing.T) {
	email := fmt.Sprintf("login_test_%d@example.com", time.Now().UnixNano())
	password := "correct_password_123"

	testDB.Create(&models.AllowedEmail{
		Email: strings.ToLower(email),
	})

	// Pre-register user
	apitest.New().
		Handler(router).
		Post("/api/v1/auth/register").
		JSON(fmt.Sprintf(`{"email":"%s","password":"%s"}`, email, password)).
		Expect(t).
		Status(http.StatusCreated).
		End()

	// 1. Success login
	var loginResp auth_dto.AccessTokenResponse
	var setCookieHeader string

	apitest.New().
		Handler(router).
		Post("/api/v1/auth/login").
		JSON(fmt.Sprintf(`{"email":"%s","password":"%s"}`, email, password)).
		Expect(t).
		Status(http.StatusOK).
		Assert(func(res *http.Response, _ *http.Request) error {
			setCookieHeader = res.Header.Get("Set-Cookie")
			if !strings.Contains(setCookieHeader, "refresh_token=") {
				return fmt.Errorf("expected refresh_token cookie, got: %s", setCookieHeader)
			}
			return json.NewDecoder(res.Body).Decode(&loginResp)
		}).
		End()

	if loginResp.AccessToken == "" {
		t.Fatal("expected access token from login")
	}

	// 2. Wrong password -> 401
	apitest.New().
		Handler(router).
		Post("/api/v1/auth/login").
		JSON(fmt.Sprintf(`{"email":"%s","password":"wrong_password"}`, email)).
		Expect(t).
		Status(http.StatusUnauthorized).
		End()

	// 3. Non-existent email -> 401
	apitest.New().
		Handler(router).
		Post("/api/v1/auth/login").
		JSON(`{"email":"ghost@example.com","password":"password123"}`).
		Expect(t).
		Status(http.StatusUnauthorized).
		End()
}

func TestAuthRefreshAndLogout(t *testing.T) {
	email := fmt.Sprintf("refresh_test_%d@example.com", time.Now().UnixNano())
	password := "refresh_password_123"

	testDB.Create(&models.AllowedEmail{
		Email: strings.ToLower(email),
	})

	var refreshCookie string

	// Register to get initial refresh cookie
	apitest.New().
		Handler(router).
		Post("/api/v1/auth/register").
		JSON(fmt.Sprintf(`{"email":"%s","password":"%s"}`, email, password)).
		Expect(t).
		Status(http.StatusCreated).
		Assert(func(res *http.Response, _ *http.Request) error {
			for _, c := range res.Cookies() {
				if c.Name == "refresh_token" {
					refreshCookie = c.Value
				}
			}
			return nil
		}).
		End()

	if refreshCookie == "" {
		t.Fatal("missing initial refresh cookie")
	}

	// 1. Missing cookie -> 401
	apitest.New().
		Handler(router).
		Post("/api/v1/auth/refresh").
		Expect(t).
		Status(http.StatusUnauthorized).
		End()

	// 2. Invalid cookie -> 401
	apitest.New().
		Handler(router).
		Post("/api/v1/auth/refresh").
		Cookie("refresh_token", "invalid-token-value").
		Expect(t).
		Status(http.StatusUnauthorized).
		End()

	// 3. Successful refresh -> 200 + new tokens
	var newRefreshToken string
	var refreshResp auth_dto.AccessTokenResponse

	apitest.New().
		Handler(router).
		Post("/api/v1/auth/refresh").
		Cookie("refresh_token", refreshCookie).
		Expect(t).
		Status(http.StatusOK).
		Assert(func(res *http.Response, _ *http.Request) error {
			for _, c := range res.Cookies() {
				if c.Name == "refresh_token" {
					newRefreshToken = c.Value
				}
			}
			return json.NewDecoder(res.Body).Decode(&refreshResp)
		}).
		End()

	if refreshResp.AccessToken == "" || newRefreshToken == "" {
		t.Fatal("expected new access token and new refresh cookie")
	}

	// 4. Token Reuse Detection (using old refresh cookie again) -> 401
	apitest.New().
		Handler(router).
		Post("/api/v1/auth/refresh").
		Cookie("refresh_token", refreshCookie).
		Expect(t).
		Status(http.StatusUnauthorized).
		End()

	// 5. Logout -> 204 No Content + cleared cookie
	apitest.New().
		Handler(router).
		Post("/api/v1/auth/logout").
		Cookie("refresh_token", newRefreshToken).
		Expect(t).
		Status(http.StatusNoContent).
		Assert(func(res *http.Response, _ *http.Request) error {
			for _, c := range res.Cookies() {
				if c.Name == "refresh_token" && c.MaxAge < 0 {
					return nil // Cookie cleared
				}
			}
			return fmt.Errorf("expected refresh_token cookie to be expired (MaxAge < 0)")
		}).
		End()
}
