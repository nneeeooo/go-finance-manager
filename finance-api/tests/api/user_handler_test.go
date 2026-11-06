package api_test

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
)

// Against a running server (TEST_API_BASEURL): /users needs a login
func TestUsersNeedLogin(t *testing.T) {
	client := resty.New()
	for _, c := range []struct{ method, path string }{
		{http.MethodPost, "/users"},
		{http.MethodGet, "/users"},
		{http.MethodGet, "/users/me"},
	} {
		resp, err := client.R().SetBody(map[string]string{"email": "x@example.com", "password": "password123"}).
			Execute(c.method, config.BaseURL+c.path)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode() != http.StatusUnauthorized {
			t.Errorf("%s %s without login: expected 401, got %d: %s", c.method, c.path, resp.StatusCode(), resp.Body())
		}
	}
}

// With TEST_ADMIN_TOKEN (an admin's access token): an admin creates and deletes a user
func TestAdminCreatesAndDeletesUser(t *testing.T) {
	token := os.Getenv("TEST_ADMIN_TOKEN")
	if token == "" {
		t.Skip("TEST_ADMIN_TOKEN is not set")
	}
	client := resty.New().SetAuthToken(token)

	var user struct {
		ID string `json:"id"`
	}
	createResp, err := client.R().
		SetBody(map[string]string{"email": fmt.Sprintf("test_%d@example.com", time.Now().UnixNano()), "password": "password123"}).
		SetResult(&user).
		Post(config.BaseURL + "/users")
	if err != nil {
		t.Fatal(err)
	}
	if createResp.StatusCode() != http.StatusCreated || user.ID == "" {
		t.Fatalf("create user: expected 201, got %d: %s", createResp.StatusCode(), createResp.Body())
	}

	deleteResp, err := client.R().Delete(config.BaseURL + "/users/" + user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if deleteResp.StatusCode() != http.StatusNoContent {
		t.Fatalf("delete user: expected 204, got %d: %s", deleteResp.StatusCode(), deleteResp.Body())
	}
}
