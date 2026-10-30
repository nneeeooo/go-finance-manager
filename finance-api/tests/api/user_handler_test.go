package api_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
)

func TestCreateAndDeleteUser(t *testing.T) {
	client := resty.New()

	email := fmt.Sprintf(
		"test_%d@example.com",
		time.Now().UnixNano(),
	)
	password := "password123"

	var user struct {
		ID string `json:"id"`
	}

	// Create user
	createResp, err := client.R().
		SetBody(map[string]string{
			"email":    email,
			"password": password,
		}).
		SetResult(&user).
		Post(config.BaseURL + "/users")

	if err != nil {
		t.Fatal(err)
	}

	if createResp.StatusCode() != http.StatusCreated {
		t.Fatalf(
			"create user: expected %d, got %d: %s",
			http.StatusCreated,
			createResp.StatusCode(),
			createResp.Body(),
		)
	}

	if user.ID == "" {
		t.Fatal("create user: missing id")
	}

	t.Logf("created user %s", user.ID)

	// Delete user
	deleteResp, err := client.R().
		Delete(config.BaseURL + "/users/" + user.ID)

	if err != nil {
		t.Fatal(err)
	}

	if deleteResp.StatusCode() != http.StatusNoContent {
		t.Fatalf(
			"delete user: expected %d, got %d: %s",
			http.StatusNoContent,
			deleteResp.StatusCode(),
			deleteResp.Body(),
		)
	}

	t.Log("user deleted")
}
