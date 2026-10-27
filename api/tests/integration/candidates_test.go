package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dev-fritz/newsp/api/internal/repositories"
	"github.com/dev-fritz/newsp/api/tests/testdb"
)

func TestCandidatesByIDsKeepTheGivenOrder(t *testing.T) {
	t.Parallel()
	db := testdb.New(t)
	ctx := context.Background()
	repo := repositories.NewCandidateRepository(db)

	sourceID := uuid.New()
	_, err := db.ExecContext(ctx, `INSERT INTO sources (id, kind, name) VALUES ($1, 'rss', 'Feed')`, sourceID)
	require.NoError(t, err)
	insertItem := func(name string) uuid.UUID {
		id := uuid.New()
		_, err := db.ExecContext(ctx, `INSERT INTO items (id, source_id, external_id, url, title, published_at)
			VALUES ($1, $2, $3, 'https://example.com/' || $3, $3, now())`, id, sourceID, name)
		require.NoError(t, err)
		return id
	}
	a, b, c := insertItem("a"), insertItem("b"), insertItem("c")

	got, err := repo.ByIDs(ctx, []uuid.UUID{c, a, b})
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, []string{"c", "a", "b"}, []string{got[0].Title, got[1].Title, got[2].Title}, "the order of the ids, not of the table")
	assert.Equal(t, "Feed", got[0].SourceName)
	assert.Equal(t, "rss", got[0].SourceKind)

	got, err = repo.ByIDs(ctx, []uuid.UUID{b, uuid.New(), a, b})
	require.NoError(t, err)
	require.Len(t, got, 2, "unknown ids are left out and a repeated id yields the item once")
	assert.Equal(t, []uuid.UUID{b, a}, []uuid.UUID{got[0].ItemID, got[1].ItemID})

	got, err = repo.ByIDs(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, got)
}

// Regression: a report must not retell what an earlier one covered. Items cited
// by the user's earlier finished reports, and items sharing their URL, leave
// the ranking.
func TestRankSkipsItemsCitedByEarlierReports(t *testing.T) {
	t.Parallel()
	db := testdb.New(t)
	ctx := context.Background()
	repo := repositories.NewCandidateRepository(db)

	userID, otherID, sourceID, mirrorID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{userID, otherID} {
		mustExec(t, db, `INSERT INTO users (id, email, name, password_hash, role) VALUES ($1, $2, 'Reader', 'x', 'member')`, id, id.String()+"@example.com")
	}
	mustExec(t, db, `INSERT INTO sources (id, kind, name) VALUES ($1, 'rss', 'Feed'), ($2, 'rss', 'Mirror')`, sourceID, mirrorID)
	now := time.Now().UTC().Truncate(time.Second)
	insertItem := func(source uuid.UUID, name, url string) uuid.UUID {
		id := uuid.New()
		mustExec(t, db, `INSERT INTO items (id, source_id, external_id, url, title, published_at) VALUES ($1, $2, $3, $4, $3, $5)`,
			id, source, name, url, now.Add(-time.Hour))
		return id
	}
	covered := insertItem(sourceID, "covered", "https://example.com/covered")
	insertItem(mirrorID, "covered-mirror", "https://example.com/covered") // same story, another source
	fresh := insertItem(sourceID, "fresh", "https://example.com/fresh")
	failedOnly := insertItem(sourceID, "failed-only", "https://example.com/failed")
	later := insertItem(sourceID, "later", "https://example.com/later")
	otherUser := insertItem(sourceID, "other-user", "https://example.com/other")

	cite := func(user uuid.UUID, status string, created time.Time, item uuid.UUID) {
		reportID := uuid.New()
		mustExec(t, db, `INSERT INTO reports (id, user_id, kind, status, window_start, window_end, output_lang, created_at)
			VALUES ($1, $2, 'manual', $3, $4, $5, 'pt-BR', $5)`, reportID, user, status, created.Add(-24*time.Hour), created)
		mustExec(t, db, `INSERT INTO report_items (report_id, item_id, ref) VALUES ($1, $2, 1)`, reportID, item)
	}
	cite(userID, "done", now.Add(-12*time.Hour), covered)
	cite(userID, "failed", now.Add(-12*time.Hour), failedOnly)
	cite(userID, "done", now.Add(time.Hour), later)
	cite(otherID, "done", now.Add(-12*time.Hour), otherUser)

	q := repositories.CandidateQuery{UserID: userID, WindowStart: now.Add(-24 * time.Hour), WindowEnd: now, Limit: 40, PerSource: 12}
	all, total, err := repo.Rank(ctx, q)
	require.NoError(t, err)
	assert.Len(t, all, 6, "without SkipCitedBefore nothing is left out")
	assert.Equal(t, 6, total)

	q.SkipCitedBefore = now
	got, total, err := repo.Rank(ctx, q)
	require.NoError(t, err)
	ids := make([]uuid.UUID, len(got))
	for i, c := range got {
		ids[i] = c.ItemID
	}
	assert.ElementsMatch(t, []uuid.UUID{fresh, failedOnly, later, otherUser}, ids,
		"the covered item and its mirror leave; failed, later and other users' reports do not count")
	assert.Equal(t, 4, total)
}
