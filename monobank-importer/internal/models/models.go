package models

type ClientInfo struct {
	Accounts []Account `json:"accounts"`
}

type Account struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	CurrencyCode int    `json:"currencyCode"`
}

type Statement struct {
	ID              string `json:"id"`
	Time            int64  `json:"time"`
	Description     string `json:"description"`
	MCC             int    `json:"mcc"`
	OriginalMCC     int    `json:"originalMcc"`
	Amount          int64  `json:"amount"`
	OperationAmount int64  `json:"operationAmount"`
	CurrencyCode    int    `json:"currencyCode"`
	CommissionRate  int64  `json:"commissionRate"`
	CashbackAmount  int64  `json:"cashbackAmount"`
	Balance         int64  `json:"balance"`
	Hold            bool   `json:"hold"`
	ReceiptID       string `json:"receiptId"`

	Comment string `json:"comment,omitempty"`
}
