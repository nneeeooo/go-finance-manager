package handlers

import (
	"errors"
	"finance-api/internal/config"
	"finance-api/internal/models"
	"finance-api/internal/services"
	"fmt"
	"mime/multipart"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const maxStatementBytes = 10 << 20 // a year of PrivatBank operations is well under 1 MB

// ownAccount loads the account when it belongs to the logged-in user and is of the given bank; writes the error otherwise
func ownAccount(c *gin.Context, rawID string, bank models.Bank) (models.Account, bool) {
	var account models.Account
	id, err := uuid.Parse(rawID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid account ID"})
		return account, false
	}
	db := c.MustGet("db").(*gorm.DB)
	userID := c.MustGet("user_id").(uuid.UUID)
	if err := db.First(&account, "id = ? AND user_id = ?", id, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Account not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get account"})
		}
		return account, false
	}
	if account.Bank != bank {
		c.JSON(http.StatusBadRequest, gin.H{"error": "The account belongs to another bank"})
		return account, false
	}
	return account, true
}

type privatbankImportResponse struct {
	Total   int                      `json:"total"`
	New     int                      `json:"new"` // dry run: would be added; import: added
	DryRun  bool                     `json:"dry_run"`
	From    time.Time                `json:"from"`
	To      time.Time                `json:"to"`
	Preview []services.PrivatbankRow `json:"preview"` // newest operations first, up to 20
}

// ImportPrivatbank godoc
// @Summary Import a PrivatBank statement (.xlsx)
// @Description Multipart form: account_id and one or more file fields (Privat24 statement exports, 10 MB together).
// @Description With dry_run=true nothing is saved and
// @Description the reply shows how many operations are new. Operation ids are deterministic, so an overlapping
// @Description statement adds only the new lines.
// @Tags imports
// @Accept multipart/form-data
// @Produce json
// @Param account_id formData string true "PrivatBank account UUID"
// @Param file formData file true "Statement .xlsx (repeat the field for several)"
// @Param dry_run query bool false "Preview only"
// @Success 200 {object} privatbankImportResponse
// @Failure 400 {object} ErrorResponse
// @Security BearerAuth
// @Failure 401 {object} ErrorResponse "Not logged in"
// @Failure 404 {object} ErrorResponse "Account not found"
// @Failure 413 {object} ErrorResponse "Over 10 MB"
// @Failure 500 {object} ErrorResponse
// @Router /imports/privatbank [post]
func ImportPrivatbank(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxStatementBytes)
	account, ok := ownAccount(c, c.PostForm("account_id"), models.BankPrivatBank)
	if !ok {
		return
	}
	form, err := c.MultipartForm()
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "The statements are over 10 MB together"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "Attach the statement .xlsx"})
		return
	}
	files := form.File["file"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Attach the statement .xlsx"})
		return
	}

	// Several statements may overlap: the same operation has the same id, so it counts once
	var rows []services.PrivatbankRow
	var txs []models.Transaction
	var ids []uuid.UUID
	seen := map[uuid.UUID]bool{}
	for _, header := range files {
		if !strings.HasSuffix(strings.ToLower(header.Filename), ".xlsx") {
			c.JSON(http.StatusBadRequest, gin.H{"error": header.Filename + ": expected an .xlsx statement"})
			return
		}
		fileRows, err := parseStatement(header)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": header.Filename + ": " + err.Error()})
			return
		}
		for _, r := range fileRows {
			tx := services.PrivatbankTransaction(account.ID, r)
			if seen[tx.ID] {
				continue
			}
			seen[tx.ID] = true
			rows, txs, ids = append(rows, r), append(txs, tx), append(ids, tx.ID)
		}
	}
	sort.SliceStable(rows, func(i, k int) bool { return rows[i].Date.After(rows[k].Date) })
	resp := privatbankImportResponse{Total: len(rows), DryRun: c.Query("dry_run") == "true", From: rows[0].Date, To: rows[0].Date}
	for _, r := range rows {
		if r.Date.Before(resp.From) {
			resp.From = r.Date
		}
		if r.Date.After(resp.To) {
			resp.To = r.Date
		}
	}
	resp.Preview = rows[:min(20, len(rows))]

	db := c.MustGet("db").(*gorm.DB)
	if resp.DryRun {
		var existing int64
		if err := db.Unscoped().Model(&models.Transaction{}).Where("id IN ?", ids).Count(&existing).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check existing operations"})
			return
		}
		resp.New = len(rows) - int(existing)
	} else {
		created, err := services.UpsertTransactions(db, txs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save operations"})
			return
		}
		resp.New = created
	}
	c.JSON(http.StatusOK, resp)
}

// One Monobank fetch per user at a time; the newest one per user is kept for the status endpoint
var (
	monobankJobsMu sync.Mutex
	monobankJobs   = map[uuid.UUID]*services.MonobankFetchJob{}
)

type monobankImportRequest struct {
	AccountID string `json:"account_id" binding:"required"`
	Token     string `json:"token"` // personal Monobank token, used for this fetch only and never stored; empty: admins use MONOBANK_TOKEN
	From      string `json:"from"`  // YYYY-MM; from and to both empty: update since the newest saved operation
	To        string `json:"to"`    // YYYY-MM
}

// StartMonobankImport godoc
// @Summary Fetch Monobank statements
// @Description Starts a background fetch of the account's statements. With from and to (YYYY-MM) it fetches those
// @Description months; with both empty it updates: from 3 days before the newest saved operation (at least the last
// @Description 31 days) to now, usually one call. The range is cut into Monobank's widest windows (31 days + 1 hour,
// @Description at most 24 windows) and Monobank allows one call a minute, so each window takes about a minute.
// @Description The token is the user's personal Monobank API token, used for this fetch only and never stored.
// @Description Admins may leave it empty to use the server's MONOBANK_TOKEN.
// @Tags imports
// @Accept json
// @Produce json
// @Param request body monobankImportRequest true "Account, token and optional months"
// @Success 202 {object} services.MonobankFetchJob
// @Failure 400 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse "A fetch is already running"
// @Security BearerAuth
// @Failure 401 {object} ErrorResponse "Not logged in"
// @Failure 404 {object} ErrorResponse "Account not found"
// @Failure 500 {object} ErrorResponse
// @Router /imports/monobank [post]
func StartMonobankImport(c *gin.Context) {
	var req monobankImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Expected account_id"})
		return
	}
	account, ok := ownAccount(c, req.AccountID, models.BankMonobank)
	if !ok {
		return
	}
	if account.ExternalID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "The account has no Monobank account id (external_id)"})
		return
	}

	db := c.MustGet("db").(*gorm.DB)
	now := time.Now()
	var from, to int64
	if req.From == "" && req.To == "" {
		var newest struct{ At *time.Time }
		if err := db.Model(&models.Transaction{}).Select("max(occurred_at) AS at").Where("account_id = ?", account.ID).Scan(&newest).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read the newest operation"})
			return
		}
		from, to = services.UpdateSpan(newest.At, now)
	} else {
		var err error
		if from, to, err = services.MonthSpan(req.From, req.To, now); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	windows := services.StatementWindows(from, to)
	if len(windows) > services.MaxMonobankWindows {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("At most %d windows of 31 days at once (about 2 years): split the range", services.MaxMonobankWindows)})
		return
	}

	client, ok := monobankClient(c, req.Token)
	if !ok {
		return
	}

	userID := c.MustGet("user_id").(uuid.UUID)
	monobankJobsMu.Lock()
	if job := monobankJobs[userID]; job != nil && job.Running() {
		monobankJobsMu.Unlock()
		c.JSON(http.StatusConflict, gin.H{"error": "A Monobank fetch is already running"})
		return
	}
	job := services.NewMonobankFetchJob(account.ID, windows)
	monobankJobs[userID] = job
	monobankJobsMu.Unlock()

	wait := time.Duration(c.MustGet("config").(*config.Config).MonobankRateLimitSeconds) * time.Second
	if wait <= 0 {
		wait = 60 * time.Second
	}
	go job.Run(db, client, account, windows, wait, job.Sleep)

	c.JSON(http.StatusAccepted, job.Snapshot())
}

// monobankClient: a Monobank API client for the pasted token; empty token: admins use MONOBANK_TOKEN. Writes 400 otherwise.
func monobankClient(c *gin.Context, rawToken string) (*resty.Client, bool) {
	cfg := c.MustGet("config").(*config.Config)
	token := strings.TrimSpace(rawToken)
	if role, _ := c.Get("user_role"); token == "" && role == models.RoleAdmin {
		token = cfg.MonobankToken
	}
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Paste your Monobank API token (api.monobank.ua)"})
		return nil, false
	}
	apiURL := cfg.MonobankApiUrl
	if apiURL == "" {
		apiURL = "https://api.monobank.ua"
	}
	return services.MonobankApiClientBuilder(apiURL, token), true
}

type monobankTokenRequest struct {
	Token string `json:"token"` // personal Monobank token, never stored; empty: admins use MONOBANK_TOKEN
}

type monobankAccountResponse struct {
	ID              string     `json:"id"` // Monobank account id, the local account's external_id
	Type            string     `json:"type"`
	Currency        uint16     `json:"currency"`
	Balance         int64      `json:"balance"`
	CreditLimit     int64      `json:"credit_limit"`
	MaskedPan       []string   `json:"masked_pan"`
	IBAN            string     `json:"iban"`
	LinkedAccountID *uuid.UUID `json:"linked_account_id,omitempty"` // the user's account with this external_id
}

// ListMonobankAccounts godoc
// @Summary List the Monobank accounts a token can read
// @Description Calls Monobank /personal/client-info with the pasted token (used once, never stored; admins may leave
// @Description it empty to use MONOBANK_TOKEN) and marks which ones are already the user's accounts. Monobank allows
// @Description one such call a minute.
// @Tags imports
// @Accept json
// @Produce json
// @Param request body monobankTokenRequest true "Token"
// @Success 200 {array} monobankAccountResponse
// @Failure 400 {object} ErrorResponse "No token, or Monobank refused it"
// @Failure 429 {object} ErrorResponse "Monobank rate limit: try again in a minute"
// @Failure 502 {object} ErrorResponse "Monobank unreachable or failed"
// @Security BearerAuth
// @Failure 401 {object} ErrorResponse "Not logged in"
// @Failure 500 {object} ErrorResponse
// @Router /imports/monobank/accounts [post]
func ListMonobankAccounts(c *gin.Context) {
	var req monobankTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	client, ok := monobankClient(c, req.Token)
	if !ok {
		return
	}
	info, err := services.FetchMonobankClientInfo(client)
	var httpErr *services.MonobankHTTPError
	switch {
	case errors.As(err, &httpErr) && (httpErr.Status == 401 || httpErr.Status == 403):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Monobank refused the token: check it on api.monobank.ua"})
		return
	case errors.As(err, &httpErr) && httpErr.Status == 429:
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Monobank allows one accounts request a minute: try again shortly"})
		return
	case err != nil:
		c.JSON(http.StatusBadGateway, gin.H{"error": "Monobank did not answer: try again later"})
		return
	}

	db := c.MustGet("db").(*gorm.DB)
	userID := c.MustGet("user_id").(uuid.UUID)
	var mine []models.Account
	if err := db.Where("user_id = ? AND bank = ? AND external_id <> ''", userID, models.BankMonobank).Find(&mine).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get accounts"})
		return
	}
	linked := make(map[string]uuid.UUID, len(mine))
	for _, a := range mine {
		linked[a.ExternalID] = a.ID
	}

	out := make([]monobankAccountResponse, len(info.Accounts))
	for i, a := range info.Accounts {
		out[i] = monobankAccountResponse{
			ID: a.ID, Type: a.Type, Currency: a.CurrencyCode, Balance: a.Balance, CreditLimit: a.CreditLimit,
			MaskedPan: a.MaskedPan, IBAN: a.IBAN,
		}
		if id, ok := linked[a.ID]; ok {
			out[i].LinkedAccountID = &id
		}
	}
	c.JSON(http.StatusOK, out)
}

// CancelMonobankImport godoc
// @Summary Stop the user's running Monobank fetch
// @Description Stops before the next Monobank call; months already fetched stay saved.
// @Tags imports
// @Produce json
// @Success 200 {object} services.MonobankFetchJob
// @Failure 404 {object} ErrorResponse "No fetch is running"
// @Security BearerAuth
// @Failure 401 {object} ErrorResponse "Not logged in"
// @Router /imports/monobank [delete]
func CancelMonobankImport(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)
	monobankJobsMu.Lock()
	job := monobankJobs[userID]
	monobankJobsMu.Unlock()
	if job == nil || !job.Cancel() {
		c.JSON(http.StatusNotFound, gin.H{"error": "No Monobank fetch is running"})
		return
	}
	c.JSON(http.StatusOK, job.Snapshot())
}

// MonobankImportStatus godoc
// @Summary Progress of the user's last Monobank fetch
// @Tags imports
// @Produce json
// @Success 200 {object} services.MonobankFetchJob
// @Success 204 "No fetch since the server started"
// @Security BearerAuth
// @Failure 401 {object} ErrorResponse "Not logged in"
// @Router /imports/monobank [get]
func MonobankImportStatus(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)
	monobankJobsMu.Lock()
	job := monobankJobs[userID]
	monobankJobsMu.Unlock()
	if job == nil {
		c.Status(http.StatusNoContent)
		return
	}
	c.JSON(http.StatusOK, job.Snapshot())
}

func parseStatement(header *multipart.FileHeader) ([]services.PrivatbankRow, error) {
	f, err := header.Open()
	if err != nil {
		return nil, errors.New("cannot read the file")
	}
	defer f.Close()
	return services.ParsePrivatbankXLSX(f)
}
