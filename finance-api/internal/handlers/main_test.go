package handlers_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"finance-api/internal/auth"
	"finance-api/internal/config"
	"finance-api/internal/database"
	"finance-api/internal/models"
	"finance-api/internal/routes"
	"finance-api/internal/services"
	"finance-api/internal/utils"

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

// testPassword: the password of every user the tests create
const testPassword = "password123"

var (
	adminOnce   sync.Once
	adminUserID string
)

// insertUser puts a user straight into the test DB (POST /users is admin-only)
func insertUser(t *testing.T, role models.Role) string {
	t.Helper()
	hash, err := utils.HashPassword(testPassword)
	if err != nil {
		t.Fatal(err)
	}
	user := models.User{Email: fmt.Sprintf("test_%d@example.com", time.Now().UnixNano()), PasswordHash: hash, Role: role}
	if err := testDB.Create(&user).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	return user.ID.String()
}

// createTestUser: a regular user
func createTestUser(t *testing.T) string {
	t.Helper()
	return insertUser(t, models.RoleRegular)
}

// adminHeader: Authorization header of one admin shared by the tests
func adminHeader(t *testing.T) string {
	t.Helper()
	adminOnce.Do(func() { adminUserID = insertUser(t, models.RoleAdmin) })
	return authHeaderForUser(t, adminUserID)
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

// TestMain runs the tests on a new empty database on the configured Postgres server, migrated like the server's,
// and drops it afterwards. The production database is not cloned or touched.
func TestMain(m *testing.M) {
	testCfg = config.Load("../../../.env")

	sysDSN := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=postgres sslmode=%s",
		testCfg.PostgresHost, testCfg.PostgresPort, testCfg.PostgresUser, testCfg.PostgresPassword, testCfg.PostgresSSLMode,
	)
	sysDB, err := gorm.Open(postgres.Open(sysDSN), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		panic("failed to connect to postgres system database: " + err.Error())
	}

	testDBName := fmt.Sprintf("finance_db_test_%d", time.Now().UnixNano())
	if err := sysDB.Exec(fmt.Sprintf("CREATE DATABASE %s;", testDBName)).Error; err != nil {
		panic("failed to create test database: " + err.Error())
	}

	cloneCfg := *testCfg
	cloneCfg.PostgresDB = testDBName
	cloneCfg.GinMode = "test"
	testCfg = &cloneCfg

	gin.SetMode(gin.TestMode)

	db := database.Connect(testCfg)
	testDB = db
	if err := database.Migrate(db); err != nil {
		panic("failed to migrate test database: " + err.Error())
	}
	if err := services.SeedAccess(db, ""); err != nil {
		panic("failed to seed access: " + err.Error())
	}

	router = gin.New()
	router.Use(cors.Default())
	router.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Set("config", testCfg)
		c.Next()
	})

	routes.Register(router, testCfg)

	exitCode := m.Run()

	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
	sysDB.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE);", testDBName))
	if sysSQL, err := sysDB.DB(); err == nil {
		_ = sysSQL.Close()
	}

	os.Exit(exitCode)
}
