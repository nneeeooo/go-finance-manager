package handlers_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"finance-api/internal/auth"
	"finance-api/internal/config"
	"finance-api/internal/database"
	"finance-api/internal/models"
	"finance-api/internal/routes"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/steinfletcher/apitest"
)

var (
	router  *gin.Engine
	testCfg *config.Config
)

func authHeaderForUser(t *testing.T, userID string) string {
	t.Helper()
	parsed, err := uuid.Parse(userID)
	if err != nil {
		t.Fatalf("invalid user id: %v", err)
	}
	token, err := auth.GenerateAccessToken(parsed, testCfg.JWTSecret)
	if err != nil {
		t.Fatalf("failed to generate access token: %v", err)
	}
	return "Bearer " + token
}

func createTestUser(t *testing.T) string {
	t.Helper()

	email := fmt.Sprintf(
		"test_%d@example.com",
		time.Now().UnixNano(),
	)

	var resp struct {
		ID string `json:"id"`
	}

	apitest.New().
		Handler(router).
		Post("/api/v1/users").
		JSON(fmt.Sprintf(`{
			"email":"%s",
			"password":"password123"
		}`, email)).
		Expect(t).
		Status(http.StatusCreated).
		Assert(func(res *http.Response, _ *http.Request) error {
			return json.NewDecoder(res.Body).Decode(&resp)
		}).
		End()

	if resp.ID == "" {
		t.Fatal("failed to create test user")
	}

	return resp.ID
}

func createTestAccountForUser(t *testing.T, userID string) string {
	t.Helper()

	var resp struct {
		ID string `json:"id"`
	}

	apitest.New().
		Handler(router).
		Post("/api/v1/accounts").
		Header("Authorization", authHeaderForUser(t, userID)).
		JSON(fmt.Sprintf(`{
			"user_id":"%s",
			"bank":1,
			"name":"Monobank",
			"currency":980
		}`, userID)).
		Expect(t).
		Status(http.StatusCreated).
		Assert(func(res *http.Response, _ *http.Request) error {
			return json.NewDecoder(res.Body).Decode(&resp)
		}).
		End()

	if resp.ID == "" {
		t.Fatal("failed to create test account")
	}

	return resp.ID
}

func createTestTransaction(t *testing.T, userID string, accountID string) string {
	t.Helper()

	amount := 15000
	currency := 980
	mcc := 5411
	description := "Test transaction"
	occurredAt := time.Now().UTC().
		Add(time.Duration(time.Now().UnixNano()%3600) * time.Second).
		Truncate(time.Second)

	data := fmt.Sprintf(
		"%s:%d:%d:%d:%s:%s",
		accountID,
		amount,
		currency,
		mcc,
		description,
		occurredAt.Format(time.RFC3339),
	)

	transactionID := uuid.NewHash(
		sha256.New(),
		uuid.NameSpaceURL,
		[]byte(data),
		8,
	)

	var resp struct {
		ID string `json:"id"`
	}

	apitest.New().
		Handler(router).
		Post("/api/v1/transactions").
		Header("Authorization", authHeaderForUser(t, userID)).
		JSON(fmt.Sprintf(`{
			"id":"%s",
			"account_id":"%s",
			"amount":%d,
			"currency":%d,
			"mcc":%d,
			"description":"%s",
			"occurred_at":"%s"
		}`,
			transactionID,
			accountID,
			amount,
			currency,
			mcc,
			description,
			occurredAt.Format(time.RFC3339),
		)).
		Expect(t).
		Status(http.StatusCreated).
		Assert(func(res *http.Response, _ *http.Request) error {
			return json.NewDecoder(res.Body).Decode(&resp)
		}).
		End()

	if resp.ID == "" {
		t.Fatal("failed to create test transaction")
	}

	return resp.ID
}

func TestMain(m *testing.M) {
	testCfg = config.Load("../../../.env")

	gin.SetMode(gin.TestMode)

	db := database.Connect(testCfg)

	if err := db.AutoMigrate(
		&models.User{},
		&models.Account{},
		&models.Transaction{}); err != nil {
		panic(err)
	}

	router = gin.New()

	router.Use(cors.Default())

	router.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	})

	routes.Register(router, testCfg)

	os.Exit(m.Run())
}
