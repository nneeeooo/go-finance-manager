package services

import (
	"finance-api/internal/models"
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"
)

// TODO: remove this junk; useless if we proceed with webhook aka better way of doing things
func MonobankApiClientBuilder(apiUrl string, token string) *resty.Client {
	client := resty.New().
		SetBaseURL(apiUrl).
		SetHeader("X-Token", token).
		SetTimeout(30 * time.Second)

	return client
}

func FetchMonobankStatementWindow(
	client *resty.Client,
	monobankAccountID string,
	from int64,
	to int64,
) ([]models.MonobankStatement, error) {
	var statements []models.MonobankStatement

	resp, err := client.R().
		SetResult(&statements).
		Get(fmt.Sprintf(
			"/personal/statement/%s/%d/%d",
			monobankAccountID,
			from,
			to,
		))

	if err != nil {
		return nil, err
	}

	if !resp.IsSuccess() {
		return nil, fmt.Errorf("monobank returned %s", resp.Status())
	}

	return statements, nil
}
