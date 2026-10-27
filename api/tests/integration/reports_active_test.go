package integration

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dev-fritz/newsp/api/internal/database"
	"github.com/dev-fritz/newsp/api/internal/models"
	"github.com/dev-fritz/newsp/api/internal/repositories"
	"github.com/dev-fritz/newsp/api/internal/services"
	"github.com/dev-fritz/newsp/api/tests/apitest"
	"github.com/dev-fritz/newsp/api/tests/testdb"
)

func activeReports(t *testing.T, s *apitest.Server, userID string) int {
	t.Helper()
	var n int
	require.NoError(t, s.Container.DB.QueryRowContext(context.Background(),
		`SELECT count(*) FROM reports WHERE user_id = $1 AND status IN ('pending', 'running')`, userID).Scan(&n))
	return n
}

// Regression: the check-then-insert of "one active report per user" had no lock,
// so a double click created two reports and two AI jobs.
func TestConcurrentReportRequestsCreateOneReport(t *testing.T) {
	t.Parallel()
	s := apitest.New(t)
	admin, _ := setupAdmin(t, s)
	res := s.Do(http.MethodGet, "/api/v1/me", admin, nil)
	require.Equal(t, http.StatusOK, res.Status, res.Body)
	userID := res.Data()["id"].(string)

	const n = 8
	results := make([]apitest.Response, n)
	var ready, wg sync.WaitGroup
	start := make(chan struct{})
	ready.Add(n)
	wg.Add(n)
	for i := range n {
		go func() {
			defer wg.Done()
			ready.Done()
			<-start
			results[i] = s.Do(http.MethodPost, "/api/v1/reports", admin, map[string]any{"window_hours": 24})
		}()
	}
	ready.Wait()
	close(start)
	wg.Wait()

	accepted, conflicts := 0, 0
	for _, r := range results {
		switch r.Status {
		case http.StatusAccepted:
			accepted++
		case http.StatusConflict:
			conflicts++
			assert.Equal(t, "REPORT_IN_PROGRESS", r.ErrorCode())
		default:
			t.Errorf("unexpected response %d: %v", r.Status, r.Body)
		}
	}
	assert.Equal(t, 1, accepted, "exactly one request wins")
	assert.Equal(t, n-1, conflicts, "the others get 409 REPORT_IN_PROGRESS")
	assert.Equal(t, 1, activeReports(t, s, userID))

	var jobs int
	require.NoError(t, s.Container.DB.QueryRowContext(context.Background(),
		`SELECT count(*) FROM jobs WHERE kind = $1 AND user_id = $2`, services.JobGenerateReport, userID).Scan(&jobs))
	assert.Equal(t, 1, jobs, "one report, one job")
}

// Replays the losing interleaving deterministically: both transactions see no
// active report, the first inserts and commits, then the second inserts.
func TestCreateReportAfterConcurrentCheckIsDuplicate(t *testing.T) {
	t.Parallel()
	db := testdb.New(t)
	ctx := context.Background()
	repo := repositories.NewReportRepository(db)
	txm := database.NewTxManager(db)

	userID := uuid.New()
	mustExec(t, db, `INSERT INTO users (id, email, name, password_hash, role) VALUES ($1, 'a@b.c', 'A', 'x', 'member')`, userID)
	newReport := func() models.Report {
		now := time.Now().UTC()
		return models.Report{
			ID: uuid.New(), UserID: userID, Kind: models.ReportManual, Status: models.ReportPending,
			WindowStart: now.Add(-time.Hour), WindowEnd: now, OutputLang: "en", CreatedAt: now,
		}
	}

	checked, firstDone := make(chan struct{}), make(chan struct{})
	second := make(chan error, 1)
	go func() {
		second <- txm.WithinTx(ctx, func(ctx context.Context) error {
			active, err := repo.HasActive(ctx, userID)
			if err != nil || active {
				return err
			}
			close(checked)
			<-firstDone
			return repo.Create(ctx, newReport())
		})
	}()

	<-checked
	require.NoError(t, txm.WithinTx(ctx, func(ctx context.Context) error { return repo.Create(ctx, newReport()) }))
	close(firstDone)

	require.ErrorIs(t, <-second, repositories.ErrDuplicate)
	var n int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM reports WHERE user_id = $1`, userID).Scan(&n))
	assert.Equal(t, 1, n)
}

func TestSchedulerRacingManualRequestsKeepsOneReport(t *testing.T) {
	t.Parallel()
	s := apitest.New(t)
	db := s.Container.DB
	admin, _ := setupAdmin(t, s)
	res := s.Do(http.MethodGet, "/api/v1/me", admin, nil)
	require.Equal(t, http.StatusOK, res.Status, res.Body)
	userID := res.Data()["id"].(string)

	mustExec(t, db, `INSERT INTO user_preferences (user_id, report_enabled, next_report_at) VALUES ($1, true, now() - interval '1 minute')
		ON CONFLICT (user_id) DO UPDATE SET report_enabled = true, next_report_at = now() - interval '1 minute'`, userID)

	const n = 6
	var ready, wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, n)
	ready.Add(n)
	wg.Add(n)
	for i := range n {
		go func() {
			defer wg.Done()
			ready.Done()
			<-start
			if i%2 == 0 {
				errs[i] = s.Container.Reports.EnqueueDue(context.Background(), time.Now())
				return
			}
			s.Do(http.MethodPost, "/api/v1/reports", admin, map[string]any{"window_hours": 24})
		}()
	}
	ready.Wait()
	close(start)
	wg.Wait()

	for i, err := range errs {
		assert.NoError(t, err, "scheduler tick %d: losing the race is not an error", i)
	}
	assert.Equal(t, 1, activeReports(t, s, userID))

	var advanced bool
	require.NoError(t, db.QueryRowContext(context.Background(),
		`SELECT next_report_at > now() FROM user_preferences WHERE user_id = $1`, userID).Scan(&advanced))
	assert.True(t, advanced, "the slot is consumed even when another report covers it")
}

func TestReportsOneActiveMigrationFailsOlderDuplicates(t *testing.T) {
	t.Parallel()
	db, url := testdb.NewEmpty(t)
	ctx := context.Background()

	m, err := database.NewMigrator(url)
	require.NoError(t, err)
	require.NoError(t, m.Migrate(10))
	_, _ = m.Close()

	userID, otherID := uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{userID, otherID} {
		mustExec(t, db, `INSERT INTO users (id, email, name, password_hash, role) VALUES ($1, $2, 'U', 'x', 'member')`, id, id.String()+"@example.com")
	}
	insert := func(owner uuid.UUID, status string, age string) uuid.UUID {
		id := uuid.New()
		mustExec(t, db, `INSERT INTO reports (id, user_id, kind, status, window_start, window_end, output_lang, created_at)
			VALUES ($1, $2, 'manual', $3, now() - interval '2 days', now(), 'pt-BR', now() - $4::interval)`, id, owner, status, age)
		mustExec(t, db, `INSERT INTO jobs (id, kind, status, user_id, dedupe_key) VALUES ($1, $2, 'pending', $3, $4)`,
			uuid.New(), services.JobGenerateReport, owner, "report:"+id.String())
		return id
	}
	oldest := insert(userID, "pending", "3 hours")
	middle := insert(userID, "running", "2 hours")
	newest := insert(userID, "pending", "1 hour")
	other := insert(otherID, "pending", "1 hour")
	done := insert(userID, "done", "5 hours")

	require.NoError(t, database.MigrateUp(url))

	state := func(id uuid.UUID) (status, errCode, jobStatus string) {
		require.NoError(t, db.QueryRowContext(ctx, `SELECT r.status, coalesce(r.error, ''), j.status
			FROM reports r JOIN jobs j ON j.dedupe_key = 'report:' || r.id::text WHERE r.id = $1`, id).Scan(&status, &errCode, &jobStatus))
		return status, errCode, jobStatus
	}
	for name, id := range map[string]uuid.UUID{"oldest": oldest, "middle": middle} {
		status, code, job := state(id)
		assert.Equal(t, "failed", status, name)
		assert.Equal(t, services.ReportErrAIFailure, code, name)
		assert.Equal(t, "canceled", job, name+": its job cannot revive it")
	}
	status, _, job := state(newest)
	assert.Equal(t, "pending", status, "the newest active report is kept")
	assert.Equal(t, "pending", job)
	status, _, _ = state(other)
	assert.Equal(t, "pending", status, "other users are untouched")
	status, _, _ = state(done)
	assert.Equal(t, "done", status)

	_, err = db.ExecContext(ctx, `INSERT INTO reports (id, user_id, kind, window_start, window_end, output_lang)
		VALUES ($1, $2, 'manual', now() - interval '1 day', now(), 'pt-BR')`, uuid.New(), userID)
	assert.Equal(t, "23505", pgCode(t, err), "a second active report is rejected")
}
