package services

import (
	"encoding/json"
	"fmt"
	"monobank-importer/internal/models"
	"net/http"
	"os"
)

func GetClientAccountInfo(token string) (*models.ClientInfo, error) {
	req, err := http.NewRequest(
		http.MethodGet,
		"https://api.monobank.ua/personal/client-info",
		nil,
	)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Token", token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("monobank returned %s", resp.Status)
	}

	var info models.ClientInfo

	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}

	return &info, nil
}

func ExtractAccounts(info *models.ClientInfo) []models.Account {
	var accounts []models.Account

	for _, account := range info.Accounts {
		if account.Type == "fop" {
			continue
		}

		accounts = append(accounts, account)
	}

	return accounts
}

func WriteAccountsJson(accounts []models.Account) error {
	file, err := os.Create("monobank_accounts.json")
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	return encoder.Encode(accounts)
}
