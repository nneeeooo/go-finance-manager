package config

import (
	"log"
	"monobank-importer/internal/models"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppHost                  string
	AppPort                  string
	MonobankToken            string
	MonobankRateLimitSeconds int
	MonobankAccounts         []models.Account
	MonobankHistoryStart     time.Time
	MaxStatementWindow       int
}

func ParseAccounts(value string) []models.Account {
	var accounts []models.Account

	for _, entry := range strings.Split(value, ";") {
		if entry == "" {
			continue
		}

		parts := strings.SplitN(entry, ":", 2)
		if len(parts) != 2 {
			continue
		}

		accounts = append(accounts, models.Account{
			ID:   parts[0],
			Type: parts[1],
		})
	}

	return accounts
}

func Load() *Config {
	if err := godotenv.Load("../.env"); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	rateLimit, _ := strconv.Atoi(os.Getenv("MONOBANK_RATELIMIT"))
	if rateLimit == 0 {
		rateLimit = 60
	}

	historyStartStr := os.Getenv("MONOBANK_HISTORY_START")
	var historyStart time.Time
	if historyStartStr == "" {
		// Default to 30 days ago
		historyStart = time.Now().AddDate(0, -1, 0)
	} else {
		var err error
		historyStart, err = time.Parse("2006-01-02", historyStartStr)
		if err != nil {
			log.Printf("Invalid MONOBANK_HISTORY_START %q, falling back to 30 days ago: %v", historyStartStr, err)
			historyStart = time.Now().AddDate(0, -1, 0)
		}
	}

	maxStatementWindow := 2682000 // default 31 days in seconds
	if maxWindowStr := os.Getenv("MONOBANK_MAXSTATEMENTWINDOW"); maxWindowStr != "" {
		if val, err := strconv.Atoi(maxWindowStr); err == nil && val > 0 {
			maxStatementWindow = val
		}
	}

	return &Config{
		AppHost:                  os.Getenv("APP_HOST"),
		AppPort:                  os.Getenv("APP_PORT"),
		MonobankRateLimitSeconds: rateLimit,
		MonobankToken:            os.Getenv("MONOBANK_TOKEN"),
		MonobankAccounts:         ParseAccounts(os.Getenv("MONOBANK_ACCOUNTS")),
		MonobankHistoryStart:     historyStart,
		MaxStatementWindow:       maxStatementWindow,
	}
}
