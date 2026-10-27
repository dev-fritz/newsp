package integration

import (
	"context"
	"database/sql"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dev-fritz/newsp/api/internal/services"
	"github.com/dev-fritz/newsp/api/tests/apitest"
)

func TestReconcileOrphanedReports(t *testing.T) {
	t.Parallel()
	s := apitest.New(t)
	db := s.Container.DB
	admin, _ := setupAdmin(t, s)

	res := s.Do(http.MethodGet, "/api/v1/me", admin, nil)
	require.Equal(t, http.StatusOK, res.Status, res.Body)
	userID := res.Data()["id"].(string)

	// newReport inserts a report; jobStatus "" means no job at all.
	newReport := func(status, jobStatus string) uuid.UUID {
		id := uuid.New()
		mustExec(t, db, `INSERT INTO reports (id, user_id, kind, status, window_start, window_end, output_lang)
			VALUES ($1, $2, 'manual', $3, now() - interval '1 day', now(), 'pt-BR')`, id, userID, status)
		if jobStatus != "" {
			mustExec(t, db, `INSERT INTO jobs (id, kind, status, user_id, dedupe_key, payload)
				VALUES ($1, $2, $3, $4, $5, jsonb_build_object('report_id', $6::text))`,
				uuid.New(), services.JobGenerateReport, jobStatus, userID, "report:"+id.String(), id.String())
		}
		return id
	}

	failedJob := newReport("running", "failed")
	canceledJob := newReport("pending", "canceled")
	noJob := newReport("pending", "")
	noJobRunning := newReport("running", "")
	pendingJob := newReport("running", "pending")
	runningJob := newReport("running", "running")
	done := newReport("done", "")
	failed := newReport("failed", "")

	state := func(id uuid.UUID) (status string, errMsg sql.NullString, completed bool) {
		var at sql.NullTime
		require.NoError(t, db.QueryRowContext(context.Background(),
			`SELECT status, error, completed_at FROM reports WHERE id = $1`, id).Scan(&status, &errMsg, &at))
		return status, errMsg, at.Valid
	}

	// Before reconciling, the orphans block the user: the regression being fixed.
	res = s.Do(http.MethodPost, "/api/v1/reports", admin, map[string]any{"window_hours": 24})
	require.Equal(t, http.StatusConflict, res.Status, res.Body)
	assert.Equal(t, "REPORT_IN_PROGRESS", res.ErrorCode())

	require.NoError(t, s.Container.Reports.ReconcileOrphans(context.Background(), time.Now()))

	for name, id := range map[string]uuid.UUID{
		"failed job": failedJob, "canceled job": canceledJob, "no job (pending)": noJob, "no job (running)": noJobRunning,
	} {
		status, errMsg, completed := state(id)
		assert.Equal(t, "failed", status, name)
		assert.Equal(t, services.ReportErrAIFailure, errMsg.String, name)
		assert.True(t, completed, "%s: completed_at is set", name)
	}

	status, _, _ := state(pendingJob)
	assert.Equal(t, "running", status, "pending job keeps its report")
	status, _, _ = state(runningJob)
	assert.Equal(t, "running", status, "running job keeps its report")
	status, errMsg, _ := state(done)
	assert.Equal(t, "done", status)
	assert.False(t, errMsg.Valid)
	status, errMsg, _ = state(failed)
	assert.Equal(t, "failed", status)
	assert.False(t, errMsg.Valid, "already failed reports are untouched")

	t.Run("reconciling again changes nothing", func(t *testing.T) {
		require.NoError(t, s.Container.Reports.ReconcileOrphans(context.Background(), time.Now()))
		status, _, _ := state(pendingJob)
		assert.Equal(t, "running", status)
	})

	t.Run("a user with only orphans can request a report again", func(t *testing.T) {
		// The two reports that still have an active job keep blocking, so finish them first.
		mustExec(t, db, `UPDATE jobs SET status = 'failed' WHERE dedupe_key IN ($1, $2)`, "report:"+pendingJob.String(), "report:"+runningJob.String())
		require.NoError(t, s.Container.Reports.ReconcileOrphans(context.Background(), time.Now()))

		res := s.Do(http.MethodPost, "/api/v1/reports", admin, map[string]any{"window_hours": 24})
		assert.Equal(t, http.StatusAccepted, res.Status, res.Body)
	})
}
