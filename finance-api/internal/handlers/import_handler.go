package handlers

import (
	"errors"
	"finance-api/internal/config"
	"finance-api/internal/models"
	"finance-api/internal/services"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
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
// @Description Multipart form: account_id and file (Privat24 statement export). With dry_run=true nothing is saved and
// @Description the reply shows how many operations are new. Operation ids are deterministic, so an overlapping
// @Description statement adds only the new lines.
// @Tags imports
// @Accept multipart/form-data
// @Produce json
// @Param account_id formData string true "PrivatBank account UUID"
// @Param file formData file true "Statement .xlsx"
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
	header, err := c.FormFile("file")
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "The statement is over 10 MB"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "Attach the statement .xlsx"})
		return
	}
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".xlsx") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Expected an .xlsx statement"})
		return
	}
	f, err := header.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot read the file"})
		return
	}
	defer f.Close()
	rows, err := services.ParsePrivatbankXLSX(f)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	txs := make([]models.Transaction, len(rows))
	ids := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		txs[i] = services.PrivatbankTransaction(account.ID, r)
		ids[i] = txs[i].ID
	}
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
	monobankJobs   = map[uuid.UUID]*services.MonobankMonthsJob{}
)

type monobankImportRequest struct {
	AccountID string `json:"account_id" binding:"required"`
	Token     string `json:"token"`                   // personal Monobank token, used for this fetch only and never stored; empty: admins use MONOBANK_TOKEN
	From      string `json:"from" binding:"required"` // YYYY-MM
	To        string `json:"to" binding:"required"`   // YYYY-MM
}

// StartMonobankImport godoc
// @Summary Fetch Monobank statements by month
// @Description Starts a background fetch of the account's statements for the months from..to (YYYY-MM, at most 24).
// @Description Monobank allows one statement call a minute, so this takes about a minute per month. The token is
// @Description the user's personal Monobank API token; it is used for this fetch only and never stored. Admins may leave
// @Description it empty to use the server's MONOBANK_TOKEN.
// @Tags imports
// @Accept json
// @Produce json
// @Param request body monobankImportRequest true "Account, token and months"
// @Success 202 {object} services.MonobankMonthsJob
// @Failure 400 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse "A fetch is already running"
// @Security BearerAuth
// @Failure 401 {object} ErrorResponse "Not logged in"
// @Failure 404 {object} ErrorResponse "Account not found"
// @Router /imports/monobank [post]
func StartMonobankImport(c *gin.Context) {
	var req monobankImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Expected account_id, from and to"})
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
	months, err := services.MonthRanges(req.From, req.To, time.Now())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cfg := c.MustGet("config").(*config.Config)
	token := strings.TrimSpace(req.Token)
	if role, _ := c.Get("user_role"); token == "" && role == models.RoleAdmin {
		token = cfg.MonobankToken
	}
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Paste your Monobank API token (api.monobank.ua)"})
		return
	}

	userID := c.MustGet("user_id").(uuid.UUID)
	monobankJobsMu.Lock()
	if job := monobankJobs[userID]; job != nil && job.Running() {
		monobankJobsMu.Unlock()
		c.JSON(http.StatusConflict, gin.H{"error": "A Monobank fetch is already running"})
		return
	}
	job := services.NewMonobankMonthsJob(account.ID, months)
	monobankJobs[userID] = job
	monobankJobsMu.Unlock()

	apiURL := cfg.MonobankApiUrl
	if apiURL == "" {
		apiURL = "https://api.monobank.ua"
	}
	wait := time.Duration(cfg.MonobankRateLimitSeconds) * time.Second
	if wait <= 0 {
		wait = 60 * time.Second
	}
	db := c.MustGet("db").(*gorm.DB)
	client := services.MonobankApiClientBuilder(apiURL, token)
	go job.Run(db, client, account, months, wait, time.Sleep)

	c.JSON(http.StatusAccepted, job.Snapshot())
}

// MonobankImportStatus godoc
// @Summary Progress of the user's last Monobank fetch
// @Tags imports
// @Produce json
// @Success 200 {object} services.MonobankMonthsJob
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
