package handlers_test

import (
	"encoding/json"
	account_dto "finance-api/internal/dto/account"
	user_dto "finance-api/internal/dto/user"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/steinfletcher/apitest"
)

func TestCreateUser(t *testing.T) {
	email := fmt.Sprintf(
		"test_%d@example.com",
		time.Now().UnixNano(),
	)

	apitest.New().
		Handler(router).
		Post("/api/v1/users").
		JSON(fmt.Sprintf(`{
			"email":"%s",
			"password":"password123"
		}`, email)).
		Expect(t).
		Status(http.StatusCreated).
		End()
}

func TestCreateUserErrors(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		statusCode int
	}{
		{
			name:       "invalid json",
			body:       `{"email":`,
			statusCode: http.StatusBadRequest,
		},
		{
			name:       "missing email",
			body:       `{"password":"password123"}`,
			statusCode: http.StatusBadRequest,
		},
		{
			name:       "invalid email",
			body:       `{"email":"abc","password":"password123"}`,
			statusCode: http.StatusBadRequest,
		},
		{
			name:       "missing password",
			body:       `{"email":"test@example.com"}`,
			statusCode: http.StatusBadRequest,
		},
		{
			name:       "short password",
			body:       `{"email":"test@example.com","password":"123"}`,
			statusCode: http.StatusBadRequest,
		},
		{
			name:       "duplicate email",
			body:       `{"email":"existing@example.com","password":"password123"}`,
			statusCode: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apitest.New().
				Handler(router).
				Post("/api/v1/users").
				JSON(tt.body).
				Expect(t).
				Status(tt.statusCode).
				End()
		})
	}
}

func TestGetUser(t *testing.T) {
	userID := createTestUser(t)

	apitest.New().
		Handler(router).
		Get("/api/v1/users/" + userID).
		Expect(t).
		Status(http.StatusOK).
		Assert(func(res *http.Response, _ *http.Request) error {
			var response user_dto.CreateUserResponse

			if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
				return err
			}

			if response.ID.String() != userID {
				return fmt.Errorf("expected user ID %s, got %s", userID, response.ID)
			}

			if response.Email == "" {
				return fmt.Errorf("expected email to be present")
			}

			return nil
		}).
		End()
}

func TestGetUserErrors(t *testing.T) {
	t.Run("invalid user ID", func(t *testing.T) {
		apitest.New().
			Handler(router).
			Get("/api/v1/users/not-a-uuid").
			Expect(t).
			Status(http.StatusBadRequest).
			End()
	})

	t.Run("user not found", func(t *testing.T) {
		userID := uuid.New()

		apitest.New().
			Handler(router).
			Get("/api/v1/users/" + userID.String()).
			Expect(t).
			Status(http.StatusNotFound).
			End()
	})
}

func TestGetUserAccounts(t *testing.T) {
	userID := createTestUser(t)

	createTestAccountForUser(t, userID)
	createTestAccountForUser(t, userID)

	apitest.New().
		Handler(router).
		Get("/api/v1/accounts").
		Header("Authorization", authHeaderForUser(t, userID)).
		Expect(t).
		Status(http.StatusOK).
		Assert(func(res *http.Response, _ *http.Request) error {
			var response []account_dto.GetAccountResponse

			if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
				return err
			}

			if len(response) != 2 {
				return fmt.Errorf("expected 2 accounts, got %d", len(response))
			}

			for _, account := range response {
				if account.UserID.String() != userID {
					return fmt.Errorf(
						"expected account to belong to user %s, got %s",
						userID,
						account.UserID,
					)
				}
			}

			return nil
		}).
		End()
}

func TestGetUserAccountsErrors(t *testing.T) {
	t.Run("invalid user ID", func(t *testing.T) {
		apitest.New().
			Handler(router).
			Get("/api/v1/users/not-a-uuid/accounts").
			Expect(t).
			Status(http.StatusBadRequest).
			End()
	})
}

func TestDeleteUser(t *testing.T) {
	userID := createTestUser(t)

	apitest.New().
		Handler(router).
		Delete("/api/v1/users/" + userID).
		Expect(t).
		Status(http.StatusNoContent).
		End()
}

func TestDeleteUserErrors(t *testing.T) {
	tests := []struct {
		name   string
		userID string
		status int
	}{
		{
			name:   "invalid uuid",
			userID: "abc",
			status: http.StatusBadRequest,
		},
		{
			name:   "user not found",
			userID: "01988888-8888-7888-8888-888888888888",
			status: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apitest.New().
				Handler(router).
				Delete("/api/v1/users/" + tt.userID).
				Expect(t).
				Status(tt.status).
				End()
		})
	}
}
