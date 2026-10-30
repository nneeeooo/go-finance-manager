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
	"finance-api/internal/routes"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/steinfletcher/apitest"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	router  *gin.Engine
	testCfg *config.Config
	testDB  *gorm.DB
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
			"bank_id":1,
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

	// Connect to default 'postgres' database to clone and drop test database
	sysDSN := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=postgres sslmode=%s",
		testCfg.PostgresHost, testCfg.PostgresPort, testCfg.PostgresUser, testCfg.PostgresPassword, testCfg.PostgresSSLMode,
	)
	sysDB, err := gorm.Open(postgres.Open(sysDSN), &gorm.Config{
		Logger: logger.Discard,
	})
	if err != nil {
		panic("failed to connect to postgres system database: " + err.Error())
	}

	testDBName := fmt.Sprintf("finance_db_test_%d", time.Now().UnixNano())

	// Terminate active connections on prod DB and clone it as template
	sysDB.Exec(fmt.Sprintf("ALTER DATABASE %s WITH ALLOW_CONNECTIONS false;", testCfg.PostgresDB))
	sysDB.Exec(fmt.Sprintf("SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '%s' AND pid != pg_backend_pid();", testCfg.PostgresDB))

	cloneErr := sysDB.Exec(fmt.Sprintf("CREATE DATABASE %s TEMPLATE %s;", testDBName, testCfg.PostgresDB)).Error
	sysDB.Exec(fmt.Sprintf("ALTER DATABASE %s WITH ALLOW_CONNECTIONS true;", testCfg.PostgresDB))

	if cloneErr != nil {
		panic("failed to clone prod database: " + cloneErr.Error())
	}

	// Point test config to temporary cloned test database
	cloneCfg := *testCfg
	cloneCfg.PostgresDB = testDBName
	cloneCfg.GinMode = "test"

	gin.SetMode(gin.TestMode)

	db := database.Connect(&cloneCfg)
	testDB = db

	router = gin.New()
	router.Use(cors.Default())
	router.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Set("config", &cloneCfg)
		c.Next()
	})

	routes.Register(router, &cloneCfg)

	exitCode := m.Run()

	// Clean up: close test DB connection and drop test DB
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
	sysDB.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE);", testDBName))
	if sysSQL, err := sysDB.DB(); err == nil {
		_ = sysSQL.Close()
	}

	os.Exit(exitCode)
}
