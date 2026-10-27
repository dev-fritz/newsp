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

	"github.com/dev-fritz/newsp/api/internal/repositories"
	"github.com/dev-fritz/newsp/api/tests/apitest"
)

// dueFixture is an instance with an admin and an invited member whose daily
// report is already due, so DueReportUsers answers only for the gates under test.
type dueFixture struct {
	db     *sql.DB
	prefs  *repositories.PreferenceRepository
	users  *repositories.UserRepository
	member uuid.UUID
	now    time.Time
}

func newDueFixture(t *testing.T) dueFixture {
	t.Helper()
	s := apitest.New(t)
	admin, _ := setupAdmin(t, s)
	inviteMember(t, s, admin)

	db := s.Container.DB
	var member uuid.UUID
	require.NoError(t, db.QueryRowContext(context.Background(), `SELECT id FROM users WHERE email = 'ana@example.com'`).Scan(&member))
	return dueFixture{
		db: db, prefs: repositories.NewPreferenceRepository(db), users: repositories.NewUserRepository(db),
		member: member, now: time.Now().Add(25 * time.Hour),
	}
}

func (f dueFixture) exec(t *testing.T, query string, args ...any) {
	t.Helper()
	_, err := f.db.ExecContext(context.Background(), query, args...)
	require.NoError(t, err)
}

func (f dueFixture) memberIsDue(t *testing.T) bool {
	t.Helper()
	due, err := f.prefs.DueReportUsers(context.Background(), f.now, 10)
	require.NoError(t, err)
	return containsID(due, f.member)
}

func containsID(ids []uuid.UUID, id uuid.UUID) bool {
	for _, got := range ids {
		if got == id {
			return true
		}
	}
	return false
}

func TestDueReportsSkipUnverifiedEmail(t *testing.T) {
	t.Parallel()
	f := newDueFixture(t)
	require.True(t, f.memberIsDue(t))

	f.exec(t, `UPDATE users SET email_verified_at = NULL WHERE id = $1`, f.member)
	assert.False(t, f.memberIsDue(t), "an unverified account gets no scheduled report")

	require.NoError(t, f.users.MarkEmailVerified(context.Background(), f.member, time.Now()))
	assert.True(t, f.memberIsDue(t), "verifying brings the schedule back")
}

func TestDueReportsSkipInactiveAccounts(t *testing.T) {
	t.Parallel()
	f := newDueFixture(t)
	f.exec(t, `UPDATE instance_settings SET inactivity_pause_days = 30`)

	f.exec(t, `UPDATE users SET last_seen_at = now() - interval '40 days' WHERE id = $1`, f.member)
	assert.False(t, f.memberIsDue(t), "last seen 40 days ago, limit is 30")

	f.exec(t, `UPDATE users SET last_seen_at = now() - interval '20 days' WHERE id = $1`, f.member)
	assert.True(t, f.memberIsDue(t), "last seen 20 days ago, limit is 30")

	f.exec(t, `UPDATE users SET last_seen_at = now() - interval '40 days' WHERE id = $1`, f.member)
	f.exec(t, `UPDATE instance_settings SET inactivity_pause_days = 0`)
	assert.True(t, f.memberIsDue(t), "0 days means accounts never pause")

	f.exec(t, `UPDATE instance_settings SET inactivity_pause_days = 30`)
	require.False(t, f.memberIsDue(t))
	require.NoError(t, f.users.TouchLastSeen(context.Background(), f.member, time.Now()))
	assert.True(t, f.memberIsDue(t), "coming back resumes the reports")
}

func TestDueReportsFallBackToCreatedAt(t *testing.T) {
	t.Parallel()
	f := newDueFixture(t)
	f.exec(t, `UPDATE instance_settings SET inactivity_pause_days = 30`)

	f.exec(t, `UPDATE users SET last_seen_at = NULL, created_at = now() - interval '2 days' WHERE id = $1`, f.member)
	assert.True(t, f.memberIsDue(t), "never seen but created recently")

	f.exec(t, `UPDATE users SET created_at = now() - interval '60 days' WHERE id = $1`, f.member)
	assert.False(t, f.memberIsDue(t), "never seen and created long ago")
}

func TestTouchLastSeenWritesAtMostOncePerHour(t *testing.T) {
	t.Parallel()
	f := newDueFixture(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	lastSeen := func() time.Time {
		var at time.Time
		require.NoError(t, f.db.QueryRowContext(ctx, `SELECT last_seen_at FROM users WHERE id = $1`, f.member).Scan(&at))
		return at
	}

	recent := now.Add(-30 * time.Minute)
	f.exec(t, `UPDATE users SET last_seen_at = $2 WHERE id = $1`, f.member, recent)
	require.NoError(t, f.users.TouchLastSeen(ctx, f.member, now))
	assert.WithinDuration(t, recent, lastSeen(), time.Millisecond, "seen 30 minutes ago: no write")

	old := now.Add(-2 * time.Hour)
	f.exec(t, `UPDATE users SET last_seen_at = $2 WHERE id = $1`, f.member, old)
	require.NoError(t, f.users.TouchLastSeen(ctx, f.member, now))
	assert.WithinDuration(t, now, lastSeen(), time.Millisecond, "seen 2 hours ago: written")

	f.exec(t, `UPDATE users SET last_seen_at = NULL WHERE id = $1`, f.member)
	require.NoError(t, f.users.TouchLastSeen(ctx, f.member, now))
	assert.WithinDuration(t, now, lastSeen(), time.Millisecond, "never seen: written")
}

func TestManualGenerationNeedsVerifiedEmail(t *testing.T) {
	t.Parallel()
	s, _ := newMailServer(t)
	admin, _ := setupAdmin(t, s)
	enableSignup(t, s, admin)
	res := signup(s, "bia@example.com")
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	member, _ := tokens(res)

	requests := map[string]struct {
		path string
		body map[string]any
	}{
		"report": {"/api/v1/reports", map[string]any{"window_hours": 24}},
		"ideas":  {"/api/v1/ideas/generate", map[string]any{"count": 1}},
		"deepen": {"/api/v1/ideas/" + uuid.NewString() + "/deepen", map[string]any{"aspects": []string{"mvp_plan"}}},
	}
	for name, req := range requests {
		res = s.Do(http.MethodPost, req.path, member, req.body)
		assert.Equal(t, http.StatusForbidden, res.Status, "%s: %v", name, res.Body)
		assert.Equal(t, "EMAIL_NOT_VERIFIED", res.ErrorCode(), name)
	}

	_, err := s.Container.DB.ExecContext(context.Background(), `UPDATE users SET email_verified_at = now() WHERE email = 'bia@example.com'`)
	require.NoError(t, err)
	res = s.Do(http.MethodPost, "/api/v1/reports", member, map[string]any{"window_hours": 24})
	assert.Equal(t, http.StatusAccepted, res.Status, res.Body)
}
