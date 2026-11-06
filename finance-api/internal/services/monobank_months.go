package services

import (
	"errors"
	"finance-api/internal/models"
	"fmt"
	"sync"
	"time"
	_ "time/tzdata" // Europe/Kyiv inside slim containers too

	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MaxMonobankMonths: Monobank allows one statement call a minute, so a year takes 12+ minutes; longer ranges go in parts
const MaxMonobankMonths = 24

// MonthRange: one calendar month as Monobank statement bounds (unix seconds); the current month ends now
type MonthRange struct {
	Label string `json:"label"` // "2026-06"
	From  int64  `json:"-"`
	To    int64  `json:"-"`
}

// Months are Kyiv months whatever the server's time zone
var kyiv = func() *time.Location {
	if loc, err := time.LoadLocation("Europe/Kyiv"); err == nil {
		return loc
	}
	return time.FixedZone("EET", 2*60*60)
}()

// MonthRanges: the months from..to ("2006-01", both included), newest first, none after now
func MonthRanges(from, to string, now time.Time) ([]MonthRange, error) {
	start, err := time.ParseInLocation("2006-01", from, kyiv)
	if err != nil {
		return nil, fmt.Errorf("from: expected YYYY-MM")
	}
	end, err := time.ParseInLocation("2006-01", to, kyiv)
	if err != nil {
		return nil, fmt.Errorf("to: expected YYYY-MM")
	}
	if end.Before(start) {
		return nil, fmt.Errorf("to is before from")
	}
	if start.After(now) {
		return nil, fmt.Errorf("from is in the future")
	}
	var out []MonthRange
	for m := end; !m.Before(start); m = m.AddDate(0, -1, 0) {
		if m.After(now) {
			continue
		}
		last := m.AddDate(0, 1, 0).Add(-time.Second)
		if last.After(now) {
			last = now
		}
		out = append(out, MonthRange{Label: m.Format("2006-01"), From: m.Unix(), To: last.Unix()})
	}
	if len(out) > MaxMonobankMonths {
		return nil, fmt.Errorf("at most %d months at once", MaxMonobankMonths)
	}
	return out, nil
}

// MonobankMonthsJob: progress of one fetch, read by the status endpoint while Run fills it
type MonobankMonthsJob struct {
	mu sync.Mutex

	AccountID  uuid.UUID `json:"account_id"`
	Status     string    `json:"status"` // running, done, error, cancelled
	Months     []string  `json:"months"`
	MonthsDone int       `json:"months_done"`
	Current    string    `json:"current,omitempty"`
	Fetched    int       `json:"fetched"`
	Created    int       `json:"created"`
	NextCallAt time.Time `json:"next_call_at,omitzero"` // when the next Monobank call goes out (rate limit)
	Error      string    `json:"error,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at,omitzero"`

	stop chan struct{} // closed by Cancel
}

func NewMonobankMonthsJob(accountID uuid.UUID, months []MonthRange) *MonobankMonthsJob {
	labels := make([]string, len(months))
	for i, m := range months {
		labels[i] = m.Label
	}
	return &MonobankMonthsJob{AccountID: accountID, Status: "running", Months: labels, StartedAt: time.Now(), stop: make(chan struct{})}
}

var errCancelled = errors.New("cancelled")

// Cancel stops a running fetch before its next Monobank call; months already saved stay. False when not running.
func (j *MonobankMonthsJob) Cancel() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.Status != "running" {
		return false
	}
	j.Status, j.Current, j.NextCallAt, j.FinishedAt = "cancelled", "", time.Time{}, time.Now()
	close(j.stop)
	return true
}

// Sleep waits d, or less when the fetch is cancelled meanwhile
func (j *MonobankMonthsJob) Sleep(d time.Duration) {
	select {
	case <-time.After(d):
	case <-j.stop:
	}
}

func (j *MonobankMonthsJob) cancelled() bool {
	select {
	case <-j.stop:
		return true
	default:
		return false
	}
}

// Snapshot: a copy safe to encode while Run is writing
func (j *MonobankMonthsJob) Snapshot() MonobankMonthsJob {
	j.mu.Lock()
	defer j.mu.Unlock()
	return MonobankMonthsJob{
		AccountID: j.AccountID, Status: j.Status, Months: append([]string(nil), j.Months...), MonthsDone: j.MonthsDone,
		Current: j.Current, Fetched: j.Fetched, Created: j.Created, NextCallAt: j.NextCallAt, Error: j.Error,
		StartedAt: j.StartedAt, FinishedAt: j.FinishedAt,
	}
}

func (j *MonobankMonthsJob) update(f func(j *MonobankMonthsJob)) {
	j.mu.Lock()
	f(j)
	j.mu.Unlock()
}

// Running: still fetching
func (j *MonobankMonthsJob) Running() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.Status == "running"
}

// Run fetches every month (newest first, pages of 500 inside a month), saves the statements and, when the newest
// month reaches today, sets the account balance from the latest statement. wait is the pause between Monobank
// calls (its rate limit); sleep is j.Sleep outside tests. Cancel stops it before the next call.
func (j *MonobankMonthsJob) Run(db *gorm.DB, client *resty.Client, account models.Account, months []MonthRange, wait time.Duration, sleep func(time.Duration)) {
	calls := 0
	call := func(from, to int64) ([]models.MonobankStatement, error) {
		for attempt := 0; ; attempt++ {
			if calls > 0 {
				j.update(func(j *MonobankMonthsJob) { j.NextCallAt = time.Now().Add(wait) })
				sleep(wait)
			}
			if j.cancelled() {
				return nil, errCancelled
			}
			calls++
			items, err := FetchMonobankStatementWindow(client, account.ExternalID, from, to)
			var httpErr *MonobankHTTPError
			if errors.As(err, &httpErr) && httpErr.Status == 429 && attempt < 3 {
				continue // too many requests: wait another round
			}
			return items, err
		}
	}

	// fail and done leave a cancelled job as Cancel set it
	fail := func(err error) {
		j.update(func(j *MonobankMonthsJob) {
			if j.Status != "running" {
				return
			}
			j.Status, j.Error, j.Current, j.FinishedAt = "error", err.Error(), "", time.Now()
		})
	}

	for i, m := range months {
		if j.cancelled() {
			return
		}
		j.update(func(j *MonobankMonthsJob) { j.Current = m.Label })
		to := m.To
		for {
			items, err := call(m.From, to)
			var httpErr *MonobankHTTPError
			if errors.As(err, &httpErr) && (httpErr.Status == 401 || httpErr.Status == 403) {
				fail(fmt.Errorf("Monobank refused the token: check it on api.monobank.ua"))
				return
			}
			if err != nil {
				fail(fmt.Errorf("%s: %w", m.Label, err))
				return
			}
			txs := make([]models.Transaction, len(items))
			for k, it := range items {
				txs[k] = MonobankTransaction(account.ID, it)
			}
			created, err := UpsertTransactions(db, txs)
			if err != nil {
				fail(fmt.Errorf("%s: save: %w", m.Label, err))
				return
			}
			// Items come newest first: the very first one of the run is the account's current balance
			if i == 0 && to == m.To && len(items) > 0 && m.To >= time.Now().Add(-time.Hour).Unix() {
				db.Model(&models.Account{}).Where("id = ?", account.ID).Update("balance", items[0].Balance)
			}
			j.update(func(j *MonobankMonthsJob) { j.Fetched += len(items); j.Created += created })
			if len(items) < MonobankStatementPageSize {
				break
			}
			to = items[len(items)-1].Time - 1 // a full page: older items of this month remain
		}
		j.update(func(j *MonobankMonthsJob) { j.MonthsDone = i + 1 })
	}
	j.update(func(j *MonobankMonthsJob) {
		if j.Status != "running" {
			return
		}
		j.Status, j.Current, j.NextCallAt, j.FinishedAt = "done", "", time.Time{}, time.Now()
	})
}
