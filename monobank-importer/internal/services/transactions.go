package services

import (
	"encoding/json"
	"fmt"
	"log"
	"monobank-importer/internal/config"
	"monobank-importer/internal/models"
	"os"
	"time"

	"github.com/go-resty/resty/v2"
)

func ClientBuilder(token string) *resty.Client {
	client := resty.New().
		SetBaseURL("https://api.monobank.ua").
		SetHeader("X-Token", token).
		SetTimeout(30 * time.Second)

	return client
}

func SyncAllAccounts(
	token string,
	accounts []models.Account,
	rateLimitSeconds int,
) error {
	start := time.Now()

	client := ClientBuilder(token)

	for _, account := range accounts {
		log.Printf("========================================")
		log.Printf("Starting account: %s (%s)", account.Type, account.ID)

		if err := SyncAccount(client, account, rateLimitSeconds); err != nil {
			return err
		}

		log.Printf("Finished account: %s", account.Type)
	}

	log.Printf(
		"Finished in %s",
		time.Since(start).Round(time.Second),
	)

	return nil
}

func SyncAccount(
	client *resty.Client,
	account models.Account,
	rateLimitSeconds int,
) error {

	cfg := config.Load()
	stop := cfg.MonobankHistoryStart.Unix()
	to := time.Now().Unix()

	var allStatements []models.Statement

	for to > stop {
		from := to - int64(cfg.MaxStatementWindow)
		if from < stop {
			from = stop
		}

		statements, err := FetchStatementWindow(
			client,
			account.ID,
			from,
			to,
		)
		if err != nil {
			return err
		}

		allStatements = append(allStatements, statements...)

		log.Printf(
			"[%s] Window: %d | Total: %d",
			account.Type,
			len(statements),
			len(allStatements),
		)

		to = from - 1

		if to > stop {
			Wait(rateLimitSeconds)
		}
	}

	// Called exactly once for this account.
	return WriteStatements(account, allStatements)
}

func Wait(seconds int) {
	for i := seconds; i > 0; i-- {
		fmt.Printf("\rNext request in %2ds...", i)
		time.Sleep(time.Second)
	}
	fmt.Println()
}

func WriteStatements(
	account models.Account,
	statements []models.Statement,
) error {

	filename := fmt.Sprintf("accounts_%s.json", account.Type)

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	log.Printf(
		"Saved %d transactions to %s",
		len(statements),
		filename,
	)

	return encoder.Encode(statements)
}

func FetchStatementWindow(
	client *resty.Client,
	accountID string,
	from int64,
	to int64,
) ([]models.Statement, error) {

	log.Printf(
		"Fetching %s | %s -> %s",
		accountID,
		time.Unix(from, 0).Format(time.DateOnly),
		time.Unix(to, 0).Format(time.DateOnly),
	)

	var statements []models.Statement

	resp, err := client.R().
		SetResult(&statements).
		Get(fmt.Sprintf(
			"/personal/statement/%s/%d/%d",
			accountID,
			from,
			to,
		))

	if err != nil {
		return nil, err
	}

	if !resp.IsSuccess() {
		return nil, fmt.Errorf("monobank returned %s", resp.Status())
	}

	log.Printf("Fetched %d transactions", len(statements))

	return statements, nil
}

func FetchDayStatements(
	token string,
	accountID string,
	day time.Time,
) ([]models.Statement, error) {

	from := time.Date(
		day.Year(),
		day.Month(),
		day.Day(),
		0, 0, 0, 0,
		day.Location(),
	)

	to := from.Add(24*time.Hour - time.Second)

	Wait(60)

	return FetchStatementWindow(
		ClientBuilder(token),
		accountID,
		from.Unix(),
		to.Unix(),
	)
}

func FetchMonthStatements(
	token string,
	accountID string,
	month time.Time,
) ([]models.Statement, error) {
	from := time.Date(
		month.Year(),
		month.Month(),
		1,
		0, 0, 0, 0,
		month.Location(),
	)

	to := from.AddDate(0, 1, 0).Add(-time.Second)

	Wait(60)

	return FetchStatementWindow(
		ClientBuilder(token),
		accountID,
		from.Unix(),
		to.Unix(),
	)
}
