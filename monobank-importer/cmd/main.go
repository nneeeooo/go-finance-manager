package main

import (
	"encoding/json"
	"log"
	"monobank-importer/internal/config"
	"monobank-importer/internal/models"
	"monobank-importer/internal/services"
	"os"
	"time"

	"github.com/google/uuid"
)

func fetchAccounts(token string) {
	info, err := services.GetClientAccountInfo(token)
	if err != nil {
		panic(err)
	}

	accounts := services.ExtractAccounts(info)

	if err := services.WriteAccountsJson(accounts); err != nil {
		panic(err)
	}
}

func WriteTempStatements(statements []models.Statement) error {
	file, err := os.Create("temp.json")
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	return encoder.Encode(statements)
}

func main() {
	cfg := config.Load()

	// fetchAccounts(cfg.MonobankToken)

	// // err := services.SyncAllAccounts(cfg.Token, cfg.Accounts, cfg.RateLimitSeconds)
	// statements, err := services.FetchDayStatements(cfg.Token, cfg.Accounts[0].ID, time.Now().AddDate(0, 0, -1))

	// if err != nil {
	// 	panic(err)
	// }

	// if err := WriteTempStatements(statements); err != nil {
	// 	panic(err)
	// }

	// accountID, err := services.CreateUserAndAccount()
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// Read JSON file

	// jsonData, err := os.ReadFile("accounts_black.json")
	// if err != nil {
	// 	log.Fatal(err)
	// }

	if cfg.MonobankToken == "" || len(cfg.MonobankAccounts) == 0 {
		log.Println("MONOBANK_TOKEN or MONOBANK_ACCOUNTS is not configured in .env. Skipping statement fetch.")
		return
	}

	accountID, err := uuid.Parse("019fe757-e2cc-7c88-aa9b-f00ddf587798")
	if err != nil {
		log.Fatal(err)
	}

	statements, err := services.FetchMonthStatements(
		cfg.MonobankToken,
		cfg.MonobankAccounts[0].ID,
		time.Now(),
	)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Fetched %d transactions", len(statements))

	jsonData, err := json.Marshal(statements)
	if err != nil {
		log.Fatal(err)
	}

	if err := services.ImportTransactions(
		accountID,
		jsonData,
	); err != nil {
		log.Fatal(err)
	}
}
