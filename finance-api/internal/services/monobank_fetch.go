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

// Monobank statement limits (api.monobank.ua/docs, v250818): one call per 60 s, and from..to spans at most
// 31 days + 1 hour. So every call asks for the widest window allowed: fewest calls for any range.
const (
	MonobankMaxWindow  = 2682000 // seconds, 31 days + 1 hour
	MaxMonobankWindows = 24      // ~2 years in one fetch: about 24 minutes at one call a minute
	// UpdateOverlap: an update starts this long before the newest saved operation, so holds settled since are refreshed
	UpdateOverlap = 3 * 24 * time.Hour
)

// Date inputs are Kyiv days whatever the server's time zone
var kyiv = func() *time.Location {
	if loc, err := time.LoadLocation("Europe/Kyiv"); err == nil {
		return loc
	}
	return time.FixedZone("EET", 2*60*60)
}()

// StatementWindow: one statement call's bounds, unix seconds, both included
type StatementWindow struct {
	From int64 `json:"from"`
	To   int64 `json:"to"`
}

// StatementWindows splits from..to into the fewest windows Monobank accepts, newest first
func StatementWindows(from, to int64) []StatementWindow {
	var out []StatementWindow
	for end := to; end >= from; {
		start := max(from, end-MonobankMaxWindow)
		out = append(out, StatementWindow{From: start, To: end})
		end = start - 1
	}
	return out
}

// DateSpan: from the start of day from to the end of day to ("2006-01-02", Kyiv time), never past now
func DateSpan(from, to string, now time.Time) (int64, int64, error) {
	start, err := time.ParseInLocation(time.DateOnly, from, kyiv)
	if err != nil {
		return 0, 0, fmt.Errorf("from: expected YYYY-MM-DD")
	}
	end, err := time.ParseInLocation(time.DateOnly, to, kyiv)
	if err != nil {
		return 0, 0, fmt.Errorf("to: expected YYYY-MM-DD")
	}
	if end.Before(start) {
		return 0, 0, fmt.Errorf("to is before from")
	}
	if start.After(now) {
		return 0, 0, fmt.Errorf("from is in the future")
	}
	last := end.AddDate(0, 0, 1).Add(-time.Second)
	if last.After(now) {
		last = now
	}
	return start.Unix(), last.Unix(), nil
}

// UpdateSpan: what an update fetches. From a little before the newest saved operation to now, and never less than
// one full window, since a call costs the same however short it is. Nothing saved yet: the last full window.
func UpdateSpan(newest *time.Time, now time.Time) (int64, int64) {
	from := now.Unix() - MonobankMaxWindow
	if newest != nil {
		from = min(from, newest.Add(-UpdateOverlap).Unix())
	}
	return from, now.Unix()
}

// MonobankFetchJob: progress of one fetch, read by the status endpoint while Run fills it
type MonobankFetchJob struct {
	mu sync.Mutex

	AccountID   uuid.UUID         `json:"account_id"`
	Status      string            `json:"status"` // running, done, error, cancelled
	Windows     []StatementWindow `json:"windows"`
	WindowsDone int               `json:"windows_done"`
	Calls       int               `json:"calls"` // Monobank calls so far: windows plus extra pages of busy windows
	Fetched     int               `json:"fetched"`
	Created     int               `json:"created"`
	NewestAt    *time.Time        `json:"newest_at,omitempty"`   // the newest operation fetched: where the next update starts
	NextCallAt  time.Time         `json:"next_call_at,omitzero"` // when the next Monobank call goes out (rate limit)
	Error       string            `json:"error,omitempty"`
	StartedAt   time.Time         `json:"started_at"`
	FinishedAt  time.Time         `json:"finished_at,omitzero"`

	stop chan struct{} // closed by Cancel
}

func NewMonobankFetchJob(accountID uuid.UUID, windows []StatementWindow) *MonobankFetchJob {
	return &MonobankFetchJob{AccountID: accountID, Status: "running", Windows: windows, StartedAt: time.Now(), stop: make(chan struct{})}
}

var errCancelled = errors.New("cancelled")

// Cancel stops a running fetch before its next Monobank call; windows already saved stay. False when not running.
func (j *MonobankFetchJob) Cancel() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.Status != "running" {
		return false
	}
	j.Status, j.NextCallAt, j.FinishedAt = "cancelled", time.Time{}, time.Now()
	close(j.stop)
	return true
}

// Sleep waits d, or less when the fetch is cancelled meanwhile
func (j *MonobankFetchJob) Sleep(d time.Duration) {
	select {
	case <-time.After(d):
	case <-j.stop:
	}
}

func (j *MonobankFetchJob) cancelled() bool {
	select {
	case <-j.stop:
		return true
	default:
		return false
	}
}

// Snapshot: a copy safe to encode while Run is writing
func (j *MonobankFetchJob) Snapshot() MonobankFetchJob {
	j.mu.Lock()
	defer j.mu.Unlock()
	return MonobankFetchJob{
		AccountID: j.AccountID, Status: j.Status, Windows: append([]StatementWindow(nil), j.Windows...),
		WindowsDone: j.WindowsDone, Calls: j.Calls, Fetched: j.Fetched, Created: j.Created, NewestAt: j.NewestAt, NextCallAt: j.NextCallAt,
		Error: j.Error, StartedAt: j.StartedAt, FinishedAt: j.FinishedAt,
	}
}

func (j *MonobankFetchJob) update(f func(j *MonobankFetchJob)) {
	j.mu.Lock()
	f(j)
	j.mu.Unlock()
}

// Running: still fetching
func (j *MonobankFetchJob) Running() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.Status == "running"
}

// Run fetches every window (newest first), saves the statements and, when the newest window reaches now, sets the
// account balance from the latest item. A window with a full page (500 items) goes on from its oldest item's time.
// wait is the pause between Monobank calls (its rate limit); sleep is j.Sleep outside tests.
func (j *MonobankFetchJob) Run(db *gorm.DB, client *resty.Client, account models.Account, windows []StatementWindow, wait time.Duration, sleep func(time.Duration)) {
	calls := 0
	call := func(from, to int64) ([]models.MonobankStatement, error) {
		for attempt := 0; ; attempt++ {
			if calls > 0 {
				j.update(func(j *MonobankFetchJob) { j.NextCallAt = time.Now().Add(wait) })
				sleep(wait)
			}
			if j.cancelled() {
				return nil, errCancelled
			}
			calls++
			j.update(func(j *MonobankFetchJob) { j.Calls = calls })
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
		j.update(func(j *MonobankFetchJob) {
			if j.Status != "running" {
				return
			}
			j.Status, j.Error, j.NextCallAt, j.FinishedAt = "error", err.Error(), time.Time{}, time.Now()
		})
	}
	day := func(unix int64) string { return time.Unix(unix, 0).In(kyiv).Format("02.01.2006") }

	for i, w := range windows {
		to := w.To
		for {
			items, err := call(w.From, to)
			if errors.Is(err, errCancelled) {
				return
			}
			var httpErr *MonobankHTTPError
			if errors.As(err, &httpErr) && (httpErr.Status == 401 || httpErr.Status == 403) {
				fail(fmt.Errorf("Monobank refused the token: check it on api.monobank.ua"))
				return
			}
			if err != nil {
				fail(fmt.Errorf("%s–%s: %w", day(w.From), day(w.To), err))
				return
			}
			txs := make([]models.Transaction, len(items))
			for k, it := range items {
				txs[k] = MonobankTransaction(account, it)
			}
			created, err := UpsertTransactions(db, txs)
			if err != nil {
				fail(fmt.Errorf("%s–%s: save: %w", day(w.From), day(w.To), err))
				return
			}
			// Items come newest first: the very first one of the run is the account's current balance (own money)
			if i == 0 && to == w.To && len(items) > 0 && w.To >= time.Now().Add(-time.Hour).Unix() {
				db.Model(&models.Account{}).Where("id = ?", account.ID).Update("balance", txs[0].Balance)
			}
			j.update(func(j *MonobankFetchJob) {
				j.Fetched += len(items)
				j.Created += created
				if len(items) > 0 && (j.NewestAt == nil || items[0].Time > j.NewestAt.Unix()) {
					at := time.Unix(items[0].Time, 0).UTC() // items come newest first
					j.NewestAt = &at
				}
			})
			if len(items) < MonobankStatementPageSize {
				break
			}
			// A full page: older items of this window remain. Go on from the oldest item's second (included, so
			// another item at that same second is not lost; the repeated one is upserted, not duplicated).
			next := items[len(items)-1].Time
			if next >= to {
				next = to - 1 // 500 items in one second: never loop forever
			}
			to = next
		}
		j.update(func(j *MonobankFetchJob) { j.WindowsDone = i + 1 })
	}
	j.update(func(j *MonobankFetchJob) {
		if j.Status == "running" {
			j.Status, j.NextCallAt, j.FinishedAt = "done", time.Time{}, time.Now()
		}
	})
}
