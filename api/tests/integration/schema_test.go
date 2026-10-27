// Package integration exercises database constraints that the services rely on
// as the last line of defense.
package integration

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dev-fritz/newsp/api/tests/testdb"
)

func pgCode(t *testing.T, err error) string {
	t.Helper()
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	return pgErr.Code
}

func TestUserEmailIsCaseInsensitiveUnique(t *testing.T) {
	t.Parallel()
	db := testdb.New(t)
	ctx := context.Background()

	const q = `INSERT INTO users (id, email, name, password_hash, role) VALUES ($1, $2, 'Ana', 'x', 'member')`
	_, err := db.ExecContext(ctx, q, uuid.New(), "ana@example.com")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, q, uuid.New(), "ANA@example.com")
	assert.Equal(t, "23505", pgCode(t, err)) // unique_violation
}

func TestInstanceSettingsIsSingleton(t *testing.T) {
	t.Parallel()
	db := testdb.New(t)

	_, err := db.ExecContext(context.Background(), `INSERT INTO instance_settings (id) VALUES ($1)`, uuid.New())
	assert.Equal(t, "23505", pgCode(t, err))
}

func TestJobDedupeKeyOnlyBlocksActiveJobs(t *testing.T) {
	t.Parallel()
	db := testdb.New(t)
	ctx := context.Background()

	const q = `INSERT INTO jobs (id, kind, dedupe_key, status) VALUES ($1, 'collect', 'source:1', $2)`
	_, err := db.ExecContext(ctx, q, uuid.New(), "pending")
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, q, uuid.New(), "pending")
	assert.Equal(t, "23505", pgCode(t, err), "a second active job with the same key is rejected")

	_, err = db.ExecContext(ctx, `UPDATE jobs SET status = 'done' WHERE dedupe_key = 'source:1'`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, q, uuid.New(), "pending")
	assert.NoError(t, err, "finished jobs do not block new ones")
}

func TestCitedItemsCannotBeDeleted(t *testing.T) {
	t.Parallel()
	db := testdb.New(t)
	ctx := context.Background()

	userID, sourceID, itemID, reportID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	mustExec(t, db, `INSERT INTO users (id, email, name, password_hash, role) VALUES ($1, 'a@b.c', 'A', 'x', 'admin')`, userID)
	mustExec(t, db, `INSERT INTO sources (id, kind, name) VALUES ($1, 'hackernews', 'HN')`, sourceID)
	mustExec(t, db, `INSERT INTO items (id, source_id, external_id, url, title, published_at) VALUES ($1, $2, '1', 'https://x', 'T', now())`, itemID, sourceID)
	mustExec(t, db, `INSERT INTO reports (id, user_id, kind, window_start, window_end, output_lang) VALUES ($1, $2, 'manual', now() - interval '1 day', now(), 'pt-BR')`, reportID, userID)
	mustExec(t, db, `INSERT INTO report_items (report_id, item_id, ref) VALUES ($1, $2, 1)`, reportID, itemID)

	_, err := db.ExecContext(ctx, `DELETE FROM items WHERE id = $1`, itemID)
	assert.Equal(t, "23503", pgCode(t, err), "foreign_key_violation keeps cited items")

	_, err = db.ExecContext(ctx, `DELETE FROM sources WHERE id = $1`, sourceID)
	assert.Equal(t, "23503", pgCode(t, err), "deleting a source cannot cascade into cited items")
}

func TestEmbeddingDimensionIsEnforced(t *testing.T) {
	t.Parallel()
	db := testdb.New(t)
	ctx := context.Background()

	sourceID := uuid.New()
	mustExec(t, db, `INSERT INTO sources (id, kind, name) VALUES ($1, 'rss', 'BBC')`, sourceID)
	_, err := db.ExecContext(ctx,
		`INSERT INTO items (id, source_id, external_id, url, title, published_at, embedding) VALUES ($1, $2, '1', 'https://x', 'T', now(), '[1,2,3]')`,
		uuid.New(), sourceID)
	assert.Equal(t, "22000", pgCode(t, err), "vectors must have 768 dimensions")
}

func mustExec(t *testing.T, db *sql.DB, q string, args ...any) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), q, args...)
	require.NoError(t, err)
}
