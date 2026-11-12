package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	JWTSecret string

	AppHost string
	AppPort string

	PostgresHost     string
	PostgresPort     string
	PostgresDB       string
	PostgresUser     string
	PostgresPassword string
	PostgresSSLMode  string

	GinMode string

	// SSO through the central auth service (OIDC); SSO is on when AuthIssuer is set
	AuthIssuer        string
	AuthClientID      string
	AuthClientSecret  string
	AuthRedirectURL   string
	AuthPostLogoutURL string
	// Optional back-channel base (e.g. http://auth-app:8090 on the podman network): discovery, token, JWKS,
	// userinfo and revocation go there instead of the public issuer; the browser still uses the public URLs
	AuthInternalURL string

	MonobankApiUrl             string
	MonobankToken              string
	MonobankRateLimitSeconds   int
	MonobankHistoryStart       time.Time
	MonobankMaxStatementWindow int
	MonobankWebhookSecret      string
}

func Load(configPath string) *Config {
	if err := godotenv.Load(configPath); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	rateLimit, _ := strconv.Atoi(os.Getenv("MONOBANK_RATELIMIT"))
	if rateLimit == 0 {
		rateLimit = 60
	}

	historyStart, err := time.Parse("2006-01-02", os.Getenv("MONOBANK_HISTORY_START"))
	if err != nil {
		log.Fatal(err)
	}

	monobankMaxStatementWindow, err := strconv.Atoi(os.Getenv("MONOBANK_MAXSTATEMENTWINDOW"))
	if err != nil {
		log.Fatal(err)
	}
	if monobankMaxStatementWindow == 0 {
		monobankMaxStatementWindow = 2682000
	}

	return &Config{
		JWTSecret:                  os.Getenv("APP_JWT_SECRET"),
		AppHost:                    os.Getenv("APP_HOST"),
		AppPort:                    os.Getenv("APP_PORT"),
		PostgresHost:               os.Getenv("POSTGRES_HOST"),
		PostgresPort:               os.Getenv("POSTGRES_PORT"),
		PostgresDB:                 os.Getenv("POSTGRES_DB"),
		PostgresUser:               os.Getenv("POSTGRES_USER"),
		PostgresPassword:           os.Getenv("POSTGRES_PASSWORD"),
		PostgresSSLMode:            os.Getenv("POSTGRES_SSLMODE"),
		GinMode:                    os.Getenv("GIN_MODE"),
		AuthIssuer:                 strings.TrimRight(os.Getenv("AUTH_ISSUER"), "/"),
		AuthClientID:               os.Getenv("AUTH_CLIENT_ID"),
		AuthClientSecret:           os.Getenv("AUTH_CLIENT_SECRET"),
		AuthRedirectURL:            os.Getenv("AUTH_REDIRECT_URL"),
		AuthPostLogoutURL:          os.Getenv("AUTH_POST_LOGOUT_URL"),
		AuthInternalURL:            strings.TrimRight(os.Getenv("AUTH_INTERNAL_URL"), "/"),
		MonobankApiUrl:             os.Getenv("MONOBANK_API_URL"),
		MonobankRateLimitSeconds:   rateLimit,
		MonobankToken:              os.Getenv("MONOBANK_TOKEN"),
		MonobankHistoryStart:       historyStart,
		MonobankMaxStatementWindow: monobankMaxStatementWindow,
		MonobankWebhookSecret:      os.Getenv("MONOBANK_WEBHOOK_SECRET"),
	}
}

// SSOEnabled: login goes through the central auth service and password login is off
func (c *Config) SSOEnabled() bool { return c.AuthIssuer != "" }
