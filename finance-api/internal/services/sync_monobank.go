package services

import (
	"crypto/sha256"
	"finance-api/internal/models"
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MonobankStatementPageSize: Monobank returns at most this many items per call; a full page means older ones remain
const MonobankStatementPageSize = 500

func MonobankApiClientBuilder(apiUrl string, token string) *resty.Client {
	client := resty.New().
		SetBaseURL(apiUrl).
		SetHeader("X-Token", token).
		SetTimeout(30 * time.Second)

	return client
}

// MonobankHTTPError: Monobank answered with a non-2xx status (429 means wait and retry)
type MonobankHTTPError struct {
	Status int
	Body   string
}

func (e *MonobankHTTPError) Error() string {
	return fmt.Sprintf("monobank returned %d: %s", e.Status, e.Body)
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
		return nil, &MonobankHTTPError{Status: resp.StatusCode(), Body: resp.String()}
	}

	return statements, nil
}

// MonobankTransactionID: deterministic id of a statement item, the same the webhook and the old importer use,
// so the same purchase arriving by webhook and by a month fetch is one row
func MonobankTransactionID(accountID uuid.UUID, itemID string, unixTime int64) uuid.UUID {
	return uuid.NewHash(sha256.New(), uuid.NameSpaceURL, []byte(fmt.Sprintf("%s:%s:%d", accountID, itemID, unixTime)), 8)
}

// MonobankTransaction: the transaction row for one statement item
func MonobankTransaction(accountID uuid.UUID, item models.MonobankStatement) models.Transaction {
	externalID := item.ID
	return models.Transaction{
		ID:                MonobankTransactionID(accountID, item.ID, item.Time),
		AccountID:         accountID,
		BankID:            models.BankMonobank,
		ExternalID:        &externalID,
		Amount:            item.Amount,
		Currency:          item.CurrencyCode,
		OperationAmount:   item.OperationAmount,
		OperationCurrency: item.CurrencyCode,
		Balance:           item.Balance,
		CashbackAmount:    item.CashbackAmount,
		CommissionRate:    item.CommissionRate,
		MCC:               item.MCC,
		Description:       item.Description,
		Comment:           item.Comment,
		Hold:              item.Hold,
		OccurredAt:        time.Unix(item.Time, 0).UTC(),
	}
}

// UpsertTransactions saves transactions by id: new ones are inserted, known ones get their changing fields
// (hold, balance, amounts) updated. Returns how many were new.
func UpsertTransactions(db *gorm.DB, transactions []models.Transaction) (created int, err error) {
	if len(transactions) == 0 {
		return 0, nil
	}
	ids := make([]uuid.UUID, len(transactions))
	for i, t := range transactions {
		ids[i] = t.ID
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		var existing int64
		if err := tx.Unscoped().Model(&models.Transaction{}).Where("id IN ?", ids).Count(&existing).Error; err != nil {
			return err
		}
		created = len(transactions) - int(existing)
		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"hold", "balance", "amount", "operation_amount", "cashback_amount", "commission_rate", "occurred_at", "updated_at",
			}),
		}).CreateInBatches(&transactions, 200).Error
	})
	return created, err
}

// MonobankClientAccount: one card account from /personal/client-info
type MonobankClientAccount struct {
	ID           string   `json:"id"`
	Type         string   `json:"type"` // black, white, platinum, iron, fop, yellow, eAid, madeInUkraine
	CurrencyCode uint16   `json:"currencyCode"`
	Balance      int64    `json:"balance"`
	CreditLimit  int64    `json:"creditLimit"`
	MaskedPan    []string `json:"maskedPan"`
	IBAN         string   `json:"iban"`
}

// MonobankClientInfo: the token owner's name and card accounts (jars are left out: they have no card statements)
type MonobankClientInfo struct {
	Name     string                  `json:"name"`
	Accounts []MonobankClientAccount `json:"accounts"`
}

// FetchMonobankClientInfo: the accounts the token can read; Monobank allows one call a minute
func FetchMonobankClientInfo(client *resty.Client) (MonobankClientInfo, error) {
	var info MonobankClientInfo
	resp, err := client.R().SetResult(&info).Get("/personal/client-info")
	if err != nil {
		return info, err
	}
	if !resp.IsSuccess() {
		return info, &MonobankHTTPError{Status: resp.StatusCode(), Body: resp.String()}
	}
	return info, nil
}
