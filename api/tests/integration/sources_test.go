package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dev-fritz/newsp/api/internal/bootstrap"
	"github.com/dev-fritz/newsp/api/internal/models"
	"github.com/dev-fritz/newsp/api/internal/repositories"
	"github.com/dev-fritz/newsp/api/internal/services"
	"github.com/dev-fritz/newsp/api/internal/workers"
	"github.com/dev-fritz/newsp/api/pkg/safehttp"
	"github.com/dev-fritz/newsp/api/tests/apitest"
	"github.com/dev-fritz/newsp/api/tests/testdb"
)

// ---- helpers -------------------------------------------------------------

func intp(v int) *int { return &v }

// newSourceFixture opens a database and returns the repositories under test
// with a source already stored.
func newSourceFixture(t *testing.T) (*sql.DB, *repositories.SourceRepository, *repositories.ItemRepository, models.Source) {
	t.Helper()
	db := testdb.New(t)
	sources, items := repositories.NewSourceRepository(db), repositories.NewItemRepository(db)
	src := newSourceModel("Fixture")
	require.NoError(t, sources.Create(context.Background(), src))
	return db, sources, items, src
}

func newSourceModel(name string) models.Source {
	now := time.Now().UTC()
	return models.Source{
		ID: uuid.New(), Kind: models.SourceRSS, Name: name, Config: json.RawMessage(`{"url":"https://example.com/feed"}`),
		IntervalMinutes: 30, Enabled: true, NextRunAt: now, CreatedAt: now, UpdatedAt: now,
	}
}

func itemIDOf(t *testing.T, db *sql.DB, sourceID uuid.UUID, externalID string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, db.QueryRowContext(context.Background(),
		`SELECT id FROM items WHERE source_id = $1 AND external_id = $2`, sourceID, externalID).Scan(&id))
	return id
}

func countRows(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRowContext(context.Background(), query, args...).Scan(&n))
	return n
}

// rssXML renders a feed with one entry per title, newest first.
func rssXML(titles ...string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?><rss version="2.0"><channel><title>Example</title><link>https://news.example.com</link>`)
	base := time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC)
	for i, title := range titles {
		slug := strings.ReplaceAll(strings.ToLower(title), " ", "-")
		fmt.Fprintf(&b, `<item><title>%s</title><link>https://news.example.com/%s</link><guid>%s</guid>`+
			`<description>Summary of %s</description><pubDate>%s</pubDate></item>`,
			title, slug, slug, title, base.Add(-time.Duration(i)*time.Hour).Format(time.RFC1123))
	}
	b.WriteString(`</channel></rss>`)
	return b.String()
}

// serveFeed serves body with the status held in the returned counter.
func serveFeed(t *testing.T, body string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	status := &atomic.Int32{}
	status.Store(http.StatusOK)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if code := int(status.Load()); code != http.StatusOK {
			w.WriteHeader(code)
			return
		}
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, status
}

// newCollectingServer starts an API whose connectors may reach loopback feeds.
func newCollectingServer(t *testing.T) *apitest.Server {
	t.Helper()
	return apitest.NewWithOptions(t, bootstrap.Options{HTTPClient: safehttp.NewClient(safehttp.Options{AllowPrivate: true})})
}

// runJobs works the queue until no job is runnable, like the worker would.
func runJobs(t *testing.T, s *apitest.Server) {
	t.Helper()
	runner, _ := bootstrap.NewWorker(s.Container)
	for range 50 {
		worked, err := runner.RunOnce(context.Background(), "test", runner.Kinds())
		require.NoError(t, err)
		if !worked {
			return
		}
	}
	t.Fatal("the queue never drained")
}

func jobResult(t *testing.T, db *sql.DB, kind string) string {
	t.Helper()
	var result []byte
	require.NoError(t, db.QueryRowContext(context.Background(),
		`SELECT result FROM jobs WHERE kind = $1 ORDER BY created_at DESC LIMIT 1`, kind).Scan(&result))
	return string(result)
}

func inviteMember(t *testing.T, s *apitest.Server, admin string) string {
	t.Helper()
	res := s.Do(http.MethodPost, "/api/v1/admin/invites", admin, map[string]string{"role": "member"})
	require.Equal(t, http.StatusCreated, res.Status)
	res = s.Do(http.MethodPost, "/api/v1/invites/accept", "", map[string]string{
		"token": tokenFromURL(t, res.Data()["url"].(string)), "name": "Ana", "email": "ana@example.com", "password": strongPassword,
	})
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	member, _ := tokens(res)
	return member
}

func createRSSSource(t *testing.T, s *apitest.Server, admin, name, url string) string {
	t.Helper()
	res := s.Do(http.MethodPost, "/api/v1/admin/sources", admin, map[string]any{"kind": "rss", "name": name, "config": map[string]any{"url": url}})
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	return res.Data()["id"].(string)
}

// citeItem makes a report cite the item, the way report generation does.
func citeItem(t *testing.T, db *sql.DB, itemID uuid.UUID) {
	t.Helper()
	userID, reportID := uuid.New(), uuid.New()
	mustExec(t, db, `INSERT INTO users (id, email, name, password_hash, role) VALUES ($1, $2, 'Reader', 'x', 'member')`, userID, userID.String()+"@example.com")
	mustExec(t, db, `INSERT INTO reports (id, user_id, kind, window_start, window_end, output_lang) VALUES ($1, $2, 'manual', now() - interval '1 day', now(), 'en')`, reportID, userID)
	mustExec(t, db, `INSERT INTO report_items (report_id, item_id, ref) VALUES ($1, $2, 1)`, reportID, itemID)
}

// ---- repositories --------------------------------------------------------

func TestItemUpsertIsIdempotentAndRefreshesMetrics(t *testing.T) {
	t.Parallel()
	db, _, items, src := newSourceFixture(t)
	ctx := context.Background()
	first := time.Date(2026, 10, 20, 8, 0, 0, 0, time.UTC)
	published := first.Add(-time.Hour)

	batch := []models.Item{
		{ExternalID: "a", URL: "https://e.com/a", Title: "A", Summary: "sa", Author: "ann", PublishedAt: published, CollectedAt: first, Score: intp(1), Comments: intp(2), Metrics: json.RawMessage(`{"points":1}`)},
		{ExternalID: "b", URL: "https://e.com/b", Title: "B", PublishedAt: published, CollectedAt: first},
		{ExternalID: "b", URL: "https://e.com/b2", Title: "B again", PublishedAt: published, CollectedAt: first},
	}
	added, err := items.Upsert(ctx, src.ID, batch)
	require.NoError(t, err)
	assert.Equal(t, 2, added, "an item repeated inside the batch counts once")
	var title string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT title FROM items WHERE external_id = 'b'`).Scan(&title))
	assert.Equal(t, "B again", title, "the last occurrence wins")

	// A second collection: refreshed metrics, new item, different timestamps.
	later := first.Add(3 * time.Hour)
	batch = []models.Item{
		{ExternalID: "a", URL: "https://e.com/a-new", Title: "A edited", Summary: "sa2", Author: "ann", PublishedAt: later, CollectedAt: later, Score: intp(50), Comments: intp(9), Metrics: json.RawMessage(`{"points":50}`)},
		{ExternalID: "c", URL: "https://e.com/c", Title: "C", PublishedAt: later, CollectedAt: later},
	}
	originalID := itemIDOf(t, db, src.ID, "a")
	added, err = items.Upsert(ctx, src.ID, batch)
	require.NoError(t, err)
	assert.Equal(t, 1, added, "only c is new")

	var (
		url, gotTitle, summary string
		score, comments        int
		metrics                string
		pub, coll              time.Time
	)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT url, title, summary, score, comments, metrics::text, published_at, collected_at FROM items WHERE external_id = 'a'`).
		Scan(&url, &gotTitle, &summary, &score, &comments, &metrics, &pub, &coll))
	assert.Equal(t, "https://e.com/a-new", url)
	assert.Equal(t, "A edited", gotTitle)
	assert.Equal(t, "sa2", summary)
	assert.Equal(t, 50, score)
	assert.Equal(t, 9, comments)
	assert.JSONEq(t, `{"points":50}`, metrics)
	assert.True(t, pub.Equal(published), "published_at is kept")
	assert.True(t, coll.Equal(first), "collected_at is kept")
	assert.Equal(t, originalID, itemIDOf(t, db, src.ID, "a"), "the row is updated in place")

	added, err = items.Upsert(ctx, src.ID, batch)
	require.NoError(t, err)
	assert.Zero(t, added, "running the same batch again adds nothing")
	assert.Equal(t, 3, countRows(t, db, `SELECT count(*) FROM items`))

	added, err = items.Upsert(ctx, src.ID, nil)
	require.NoError(t, err)
	assert.Zero(t, added)
}

func TestSourceRepositoryStoresTheSealedSecret(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	_, sources, _, src := newSourceFixture(t)

	got, err := sources.GetByID(ctx, src.ID)
	require.NoError(t, err)
	assert.Nil(t, got.SecretEnc)

	got.SecretEnc = []byte{1, 2, 3}
	require.NoError(t, sources.Update(ctx, got))
	got, err = sources.GetByID(ctx, src.ID)
	require.NoError(t, err)
	assert.Equal(t, []byte{1, 2, 3}, got.SecretEnc)

	list, err := sources.ListWithStats(ctx, time.Now().Add(-time.Hour))
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, []byte{1, 2, 3}, list[0].Source.SecretEnc)

	got.SecretEnc = nil
	require.NoError(t, sources.Update(ctx, got))
	got, err = sources.GetByID(ctx, src.ID)
	require.NoError(t, err)
	assert.Nil(t, got.SecretEnc)
}

func TestSourceRepositoryDueAndRunBookkeeping(t *testing.T) {
	t.Parallel()
	db, sources, items, due := newSourceFixture(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	later := newSourceModel("Later")
	later.NextRunAt = now.Add(time.Hour)
	disabled := newSourceModel("Disabled")
	disabled.Enabled = false
	for _, s := range []models.Source{later, disabled} {
		require.NoError(t, sources.Create(ctx, s))
	}

	ids, err := sources.ListDue(ctx, now.Add(time.Minute), 100)
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{due.ID}, ids, "only enabled sources whose time came")
	ids, err = sources.ListDue(ctx, now.Add(2*time.Hour), 1)
	require.NoError(t, err)
	assert.Len(t, ids, 1, "the limit applies")

	next := now.Add(2 * time.Hour)
	require.NoError(t, sources.MarkFailure(ctx, due.ID, now, "boom", next))
	require.NoError(t, sources.MarkFailure(ctx, due.ID, now, "boom again", next))
	got, err := sources.GetByID(ctx, due.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, got.ConsecutiveFailures)
	require.NotNil(t, got.LastError)
	assert.Equal(t, "boom again", *got.LastError)
	assert.True(t, got.NextRunAt.Equal(next))
	assert.Nil(t, got.LastSuccessAt)

	// Editing never rewrites the run bookkeeping.
	got.Name = "Renamed"
	got.Config = json.RawMessage(`{"url":"https://example.com/other"}`)
	got.UpdatedAt = now
	got.ConsecutiveFailures = 0
	require.NoError(t, sources.Update(ctx, got))
	got, err = sources.GetByID(ctx, due.ID)
	require.NoError(t, err)
	assert.Equal(t, "Renamed", got.Name)
	assert.Equal(t, 2, got.ConsecutiveFailures)

	require.NoError(t, sources.MarkSuccess(ctx, due.ID, now, next))
	got, err = sources.GetByID(ctx, due.ID)
	require.NoError(t, err)
	assert.Zero(t, got.ConsecutiveFailures)
	assert.Nil(t, got.LastError)
	require.NotNil(t, got.LastSuccessAt)
	require.NotNil(t, got.LastRunAt)

	// items_24h comes with the list, computed in the same query.
	_, err = items.Upsert(ctx, due.ID, []models.Item{
		{ExternalID: "recent", URL: "https://e.com/1", Title: "r", PublishedAt: now, CollectedAt: now.Add(-time.Hour)},
		{ExternalID: "old", URL: "https://e.com/2", Title: "o", PublishedAt: now, CollectedAt: now.Add(-48 * time.Hour)},
	})
	require.NoError(t, err)
	stats, err := sources.ListWithStats(ctx, now.Add(-24*time.Hour))
	require.NoError(t, err)
	require.Len(t, stats, 3)
	byName := map[string]int{}
	for _, s := range stats {
		byName[s.Source.Name] = s.Items
	}
	assert.Equal(t, map[string]int{"Renamed": 1, "Later": 0, "Disabled": 0}, byName)
	assert.Equal(t, "Disabled", stats[0].Source.Name, "ordered by name")

	require.ErrorIs(t, sources.MarkSuccess(ctx, uuid.New(), now, next), repositories.ErrNotFound)
	require.ErrorIs(t, sources.Delete(ctx, uuid.New()), repositories.ErrNotFound)

	citeItem(t, db, itemIDOf(t, db, due.ID, "recent"))
	require.ErrorIs(t, sources.Delete(ctx, due.ID), repositories.ErrReferenced)
	require.NoError(t, sources.Delete(ctx, later.ID))
}

// feedKey names a feed item by the number at the end of its URL; the feed does
// not expose external ids.
func feedKey(v repositories.ItemView) string {
	return strings.TrimPrefix(v.Item.URL, "https://e.com/")
}

func TestFeedFiltersSearchSortAndPagination(t *testing.T) {
	t.Parallel()
	db, sources, items, a := newSourceFixture(t)
	ctx := context.Background()
	b := newSourceModel("Other")
	require.NoError(t, sources.Create(ctx, b))

	day := func(d int) time.Time { return time.Date(2026, 10, d, 12, 0, 0, 0, time.UTC) }
	collected := day(30)
	_, err := items.Upsert(ctx, a.ID, []models.Item{
		{ExternalID: "1", URL: "https://e.com/1", Title: "Postgres vector search arrives", Summary: "pgvector speeds up similarity", PublishedAt: day(20), CollectedAt: collected, Score: intp(10), Comments: intp(5)},
		{ExternalID: "2", URL: "https://e.com/2", Title: "Rust compiler release", Summary: "faster builds", PublishedAt: day(22), CollectedAt: collected, Score: intp(100)},
		{ExternalID: "3", URL: "https://e.com/3", Title: "Go release notes", Summary: "postgres driver news", Author: "gopher", PublishedAt: day(24), CollectedAt: collected},
	})
	require.NoError(t, err)
	_, err = items.Upsert(ctx, b.ID, []models.Item{
		{ExternalID: "4", URL: "https://e.com/4", Title: "Search engines compared", PublishedAt: day(21), CollectedAt: collected, Comments: intp(80), Metrics: json.RawMessage(`{"k":"v"}`)},
	})
	require.NoError(t, err)

	titles := func(f repositories.ItemFilter, limit, offset int) ([]string, int) {
		t.Helper()
		got, total, err := items.List(ctx, f, limit, offset)
		require.NoError(t, err)
		out := make([]string, len(got))
		for i, v := range got {
			out[i] = feedKey(v)
		}
		return out, total
	}

	got, total := titles(repositories.ItemFilter{}, 10, 0)
	assert.Equal(t, []string{"3", "2", "4", "1"}, got, "newest first by default")
	assert.Equal(t, 4, total)

	got, _ = titles(repositories.ItemFilter{SourceID: &b.ID}, 10, 0)
	assert.Equal(t, []string{"4"}, got)

	since, until := day(21), day(24)
	got, total = titles(repositories.ItemFilter{Since: &since, Until: &until}, 10, 0)
	assert.Equal(t, []string{"2", "4"}, got, "since is inclusive, until exclusive")
	assert.Equal(t, 2, total)

	got, _ = titles(repositories.ItemFilter{Query: "postgres"}, 10, 0)
	assert.Equal(t, []string{"3", "1"}, got, "title and summary are searched")
	got, _ = titles(repositories.ItemFilter{Query: "postgres -driver"}, 10, 0)
	assert.Equal(t, []string{"1"}, got, "web search syntax: exclusion")
	got, _ = titles(repositories.ItemFilter{Query: `"vector search"`}, 10, 0)
	assert.Equal(t, []string{"1"}, got, "web search syntax: phrase")
	got, _ = titles(repositories.ItemFilter{Query: "gopher"}, 10, 0)
	assert.Equal(t, []string{"3"}, got, "authors are searched")
	got, total = titles(repositories.ItemFilter{Query: "nonexistentword"}, 10, 0)
	assert.Empty(t, got)
	assert.Zero(t, total)

	got, _ = titles(repositories.ItemFilter{Sort: repositories.ItemSortEngagement}, 10, 0)
	assert.Equal(t, []string{"2", "4", "1", "3"}, got, "score plus comments, missing counts as zero")
	got, _ = titles(repositories.ItemFilter{Sort: "published_at; DROP TABLE items"}, 10, 0)
	assert.Equal(t, []string{"3", "2", "4", "1"}, got, "unknown sorts fall back to recent, never reach SQL")

	got, total = titles(repositories.ItemFilter{}, 2, 1)
	assert.Equal(t, []string{"2", "4"}, got)
	assert.Equal(t, 4, total, "the total ignores the page")

	// cited: a report or an idea cites the item.
	citeItem(t, db, itemIDOf(t, db, a.ID, "1"))
	userID, ideaID, versionID := uuid.New(), uuid.New(), uuid.New()
	mustExec(t, db, `INSERT INTO users (id, email, name, password_hash, role) VALUES ($1, 'idea@example.com', 'Maker', 'x', 'member')`, userID)
	mustExec(t, db, `INSERT INTO ideas (id, user_id, title, summary) VALUES ($1, $2, 'Idea', 'Summary')`, ideaID, userID)
	mustExec(t, db, `INSERT INTO idea_versions (id, idea_id, version, origin, title, summary, difficulty, content) VALUES ($1, $2, 1, 'generated', 'Idea', 'Summary', 'beginner', '{}')`, versionID, ideaID)
	mustExec(t, db, `INSERT INTO idea_evidence (idea_version_id, item_id, ref) VALUES ($1, $2, 'E1')`, versionID, itemIDOf(t, db, a.ID, "2"))

	views, _, err := items.List(ctx, repositories.ItemFilter{}, 10, 0)
	require.NoError(t, err)
	cited := map[string]bool{}
	for _, v := range views {
		cited[feedKey(v)] = v.Cited
	}
	assert.Equal(t, map[string]bool{"1": true, "2": true, "3": false, "4": false}, cited)
	for _, v := range views {
		if feedKey(v) == "4" {
			assert.Equal(t, "Other", v.SourceName)
			assert.Equal(t, models.SourceRSS, v.SourceKind)
			assert.JSONEq(t, `{"k":"v"}`, string(v.Item.Metrics))
		}
	}
}

// unitLiteral is the pgvector text of the 768-dimension unit vector with a 1 at hot.
func unitLiteral(hot int) string {
	parts := make([]string, 768)
	for i := range parts {
		parts[i] = "0"
	}
	parts[hot] = "1"
	return "[" + strings.Join(parts, ",") + "]"
}

func TestSetEmbeddingsUpdatesABatchInOneStatement(t *testing.T) {
	t.Parallel()
	db, _, items, src := newSourceFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	var batch []models.Item
	for i := range 3 {
		batch = append(batch, models.Item{
			ExternalID: fmt.Sprint(i), URL: fmt.Sprintf("https://e.com/%d", i), Title: fmt.Sprintf("T%d", i), Summary: "S",
			PublishedAt: now, CollectedAt: now.Add(time.Duration(i) * time.Second),
		})
	}
	_, err := items.Upsert(ctx, src.ID, batch)
	require.NoError(t, err)

	pending, err := items.Pending(ctx, 2)
	require.NoError(t, err)
	require.Len(t, pending, 2)
	assert.Equal(t, "T0", pending[0].Title, "oldest collected first")
	assert.Equal(t, "T1", pending[1].Title)

	ids := []uuid.UUID{pending[0].ID, pending[1].ID}
	unit := func(hot int) []float32 {
		v := make([]float32, 768)
		v[hot] = 1
		return v
	}
	require.NoError(t, items.SetEmbeddings(ctx, ids, [][]float32{unit(0), unit(1)}, "nomic-embed-text", now))

	assert.Equal(t, 2, countRows(t, db, `SELECT count(*) FROM items WHERE embedding IS NOT NULL AND embedding_model = 'nomic-embed-text' AND embedded_at IS NOT NULL`))
	for external, hot := range map[string]int{"0": 0, "1": 1} {
		var dist float64
		require.NoError(t, db.QueryRowContext(ctx, `SELECT embedding <=> $1::vector FROM items WHERE external_id = $2`, unitLiteral(hot), external).Scan(&dist))
		assert.InDelta(t, 0, dist, 1e-6, "item %s holds its own vector", external)
	}

	hasPending, err := items.HasPending(ctx)
	require.NoError(t, err)
	assert.True(t, hasPending)

	// One bad vector fails the whole statement: nothing is half-written.
	rest, err := items.Pending(ctx, 10)
	require.NoError(t, err)
	require.Len(t, rest, 1)
	err = items.SetEmbeddings(ctx, []uuid.UUID{rest[0].ID, ids[0]}, [][]float32{unit(2), {1, 2, 3}}, "m", now)
	require.Error(t, err)
	assert.Equal(t, 1, countRows(t, db, `SELECT count(*) FROM items WHERE embedding IS NULL`), "the valid vector was rolled back with the bad one")

	require.Error(t, items.SetEmbeddings(ctx, ids, [][]float32{unit(0)}, "m", now), "ids and vectors must match")
	require.NoError(t, items.SetEmbeddings(ctx, nil, nil, "m", now))
}

// ---- API -----------------------------------------------------------------

func TestSourceAdministrationHappyPath(t *testing.T) {
	t.Parallel()
	s := newCollectingServer(t)
	admin, _ := setupAdmin(t, s)

	res := s.Do(http.MethodPost, "/api/v1/admin/sources", admin, map[string]any{
		"kind": "rss", "name": " BBC Tech ", "config": map[string]any{"url": "https://feeds.bbci.co.uk/news/technology/rss.xml"},
	})
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	id := res.Data()["id"].(string)
	assert.Equal(t, "BBC Tech", res.Data()["name"])
	assert.InDelta(t, 30, res.Data()["interval_minutes"], 0, "default interval")
	assert.Equal(t, true, res.Data()["enabled"], "enabled by default")
	assert.Equal(t, map[string]any{"url": "https://feeds.bbci.co.uk/news/technology/rss.xml", "limit": float64(50)}, res.Data()["config"], "config comes back normalized")
	keys := make([]string, 0)
	for k := range res.Data() {
		keys = append(keys, k)
	}
	assert.ElementsMatch(t, []string{"id", "kind", "name", "config", "has_secret", "interval_minutes", "enabled", "next_run_at", "last_run_at",
		"last_success_at", "last_error", "consecutive_failures", "items_24h", "created_at"}, keys, "no internals leak")

	res = s.Do(http.MethodPost, "/api/v1/admin/sources", admin, map[string]any{
		"kind": "hackernews", "name": "Alpha HN", "config": map[string]any{}, "interval_minutes": 60, "enabled": false,
	})
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	hnID := res.Data()["id"].(string)
	assert.Equal(t, false, res.Data()["enabled"])
	assert.InDelta(t, 60, res.Data()["interval_minutes"], 0)

	res = s.Do(http.MethodGet, "/api/v1/admin/sources", admin, nil)
	require.Equal(t, http.StatusOK, res.Status)
	require.Len(t, res.List(), 2)
	assert.Equal(t, "Alpha HN", res.List()[0]["name"], "ordered by name")
	assert.InDelta(t, 0, res.List()[0]["items_24h"], 0)

	// Enabling a disabled source makes it due now; config is re-normalized.
	res = s.Do(http.MethodPatch, "/api/v1/admin/sources/"+hnID, admin, map[string]any{"enabled": true, "name": "HN front page", "interval_minutes": 15})
	require.Equal(t, http.StatusOK, res.Status, res.Body)
	assert.Equal(t, true, res.Data()["enabled"])
	assert.Equal(t, "HN front page", res.Data()["name"])
	assert.InDelta(t, 15, res.Data()["interval_minutes"], 0)
	assert.Equal(t, 1, countRows(t, s.Container.DB, `SELECT count(*) FROM sources WHERE id = $1 AND next_run_at <= now()`, hnID))

	res = s.Do(http.MethodPatch, "/api/v1/admin/sources/"+id, admin, map[string]any{"config": map[string]any{"url": "https://example.com/new.xml", "limit": 10}})
	require.Equal(t, http.StatusOK, res.Status, res.Body)
	assert.Equal(t, map[string]any{"url": "https://example.com/new.xml", "limit": float64(10)}, res.Data()["config"])
	assert.Equal(t, "BBC Tech", res.Data()["name"], "omitted fields stay")

	res = s.Do(http.MethodPatch, "/api/v1/admin/sources/"+uuid.NewString(), admin, map[string]any{"name": "x"})
	assert.Equal(t, http.StatusNotFound, res.Status)
	assert.Equal(t, "SOURCE_NOT_FOUND", res.ErrorCode())
	res = s.Do(http.MethodPatch, "/api/v1/admin/sources/not-a-uuid", admin, map[string]any{"name": "x"})
	assert.Equal(t, http.StatusBadRequest, res.Status)

	res = s.Do(http.MethodDelete, "/api/v1/admin/sources/"+id, admin, nil)
	assert.Equal(t, http.StatusNoContent, res.Status)
	res = s.Do(http.MethodDelete, "/api/v1/admin/sources/"+id, admin, nil)
	assert.Equal(t, http.StatusNotFound, res.Status)
}

func TestSourceValidation(t *testing.T) {
	t.Parallel()
	s := newCollectingServer(t)
	admin, _ := setupAdmin(t, s)
	create := func(body map[string]any) apitest.Response {
		return s.Do(http.MethodPost, "/api/v1/admin/sources", admin, body)
	}
	field := func(r apitest.Response, name string) any {
		e, _ := r.Body["error"].(map[string]any)
		f, _ := e["fields"].(map[string]any)
		return f[name]
	}

	res := create(map[string]any{"kind": "myspace", "name": "x", "config": map[string]any{}})
	assert.Equal(t, http.StatusBadRequest, res.Status)
	assert.Equal(t, "unsupported", field(res, "kind"))

	res = create(map[string]any{"kind": "rss", "name": "x", "config": map[string]any{"url": "ftp://example.com/feed"}})
	assert.Equal(t, http.StatusBadRequest, res.Status)
	assert.NotNil(t, field(res, "config.url"), res.Body)

	res = create(map[string]any{"kind": "rss", "name": "x", "config": map[string]any{"url": "https://example.com/feed", "limit": 1000}})
	assert.Equal(t, http.StatusBadRequest, res.Status)
	assert.NotNil(t, field(res, "config.limit"), res.Body)

	res = create(map[string]any{"kind": "rss", "name": "x", "config": map[string]any{"url": "https://example.com/feed"}, "secret": "token"})
	assert.Equal(t, http.StatusBadRequest, res.Status)
	assert.Equal(t, "unsupported", field(res, "secret"), "public kinds take no credential")

	res = create(map[string]any{"kind": "rss", "name": "x", "config": []int{1}})
	assert.Equal(t, http.StatusBadRequest, res.Status)
	assert.Equal(t, "object", field(res, "config"))

	res = create(map[string]any{"kind": "rss", "name": "x"})
	assert.Equal(t, http.StatusBadRequest, res.Status, "config is required")
	res = create(map[string]any{"kind": "rss", "name": "   ", "config": map[string]any{"url": "https://example.com/f"}})
	assert.Equal(t, http.StatusBadRequest, res.Status)
	assert.Equal(t, "required", field(res, "name"))
	for _, minutes := range []int{4, 1441} {
		res = create(map[string]any{"kind": "rss", "name": "x", "config": map[string]any{"url": "https://example.com/f"}, "interval_minutes": minutes})
		assert.Equal(t, http.StatusBadRequest, res.Status, "interval %d", minutes)
	}
	res = create(map[string]any{"kind": "rss", "name": "x", "config": map[string]any{"url": "https://example.com/f"}, "id": uuid.NewString()})
	assert.Equal(t, http.StatusBadRequest, res.Status, "unknown fields are rejected")

	id := createRSSSource(t, s, admin, "ok", "https://example.com/feed")
	res = s.Do(http.MethodPatch, "/api/v1/admin/sources/"+id, admin, map[string]any{"config": map[string]any{"url": "javascript:alert(1)"}})
	assert.Equal(t, http.StatusBadRequest, res.Status)
	assert.NotNil(t, field(res, "config.url"), res.Body)
	res = s.Do(http.MethodPatch, "/api/v1/admin/sources/"+id, admin, map[string]any{"interval_minutes": 2})
	assert.Equal(t, http.StatusBadRequest, res.Status)
	res = s.Do(http.MethodPatch, "/api/v1/admin/sources/"+id, admin, map[string]any{"name": ""})
	assert.Equal(t, http.StatusBadRequest, res.Status)
}

func TestSourceSecretIsWriteOnlyAndEncrypted(t *testing.T) {
	t.Parallel()
	s := newCollectingServer(t)
	admin, _ := setupAdmin(t, s)
	const token = "ghp_example_token_value"

	res := s.Do(http.MethodPost, "/api/v1/admin/sources", admin, map[string]any{
		"kind": "github", "name": "New Go repos", "config": map[string]any{"language": "Go"}, "secret": token,
	})
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	id := res.Data()["id"].(string)
	assert.Equal(t, true, res.Data()["has_secret"])
	assert.NotContains(t, fmt.Sprint(res.Body), token, "the secret is never returned")

	var stored []byte
	require.NoError(t, s.Container.DB.QueryRowContext(context.Background(), `SELECT secret_enc FROM sources WHERE id = $1`, id).Scan(&stored))
	assert.NotEmpty(t, stored)
	assert.NotContains(t, string(stored), token, "encrypted at rest")

	res = s.Do(http.MethodGet, "/api/v1/admin/sources", admin, nil)
	require.Equal(t, http.StatusOK, res.Status)
	assert.NotContains(t, fmt.Sprint(res.Body), token)
	assert.Equal(t, true, res.List()[0]["has_secret"])

	res = s.Do(http.MethodPatch, "/api/v1/admin/sources/"+id, admin, map[string]any{"name": "Go repos"})
	require.Equal(t, http.StatusOK, res.Status, res.Body)
	assert.Equal(t, true, res.Data()["has_secret"], "an omitted secret is kept")

	res = s.Do(http.MethodPatch, "/api/v1/admin/sources/"+id, admin, map[string]any{"secret": ""})
	require.Equal(t, http.StatusOK, res.Status, res.Body)
	assert.Equal(t, false, res.Data()["has_secret"], "an empty secret removes it")
	assert.Equal(t, 1, countRows(t, s.Container.DB, `SELECT count(*) FROM sources WHERE id = $1 AND secret_enc IS NULL`, id))

	res = s.Do(http.MethodPost, "/api/v1/admin/sources", admin, map[string]any{
		"kind": "producthunt", "name": "PH", "config": map[string]any{},
	})
	assert.Equal(t, http.StatusBadRequest, res.Status)
	assert.Equal(t, "VALIDATION_FAILED", res.ErrorCode())
	assert.Contains(t, fmt.Sprint(res.Body), "secret", "Product Hunt needs a developer token")
}

func TestSourcesAreAdminOnlyButReadableByMembers(t *testing.T) {
	t.Parallel()
	s := newCollectingServer(t)
	admin, _ := setupAdmin(t, s)
	member := inviteMember(t, s, admin)
	id := createRSSSource(t, s, admin, "Feed", "https://example.com/feed")
	body := map[string]any{"kind": "rss", "name": "x", "config": map[string]any{"url": "https://example.com/f"}}

	for _, tc := range []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, "/api/v1/admin/sources", nil},
		{http.MethodPost, "/api/v1/admin/sources", body},
		{http.MethodPatch, "/api/v1/admin/sources/" + id, map[string]any{"name": "y"}},
		{http.MethodDelete, "/api/v1/admin/sources/" + id, nil},
		{http.MethodPost, "/api/v1/admin/sources/" + id + "/collect", nil},
	} {
		res := s.Do(tc.method, tc.path, member, tc.body)
		assert.Equal(t, http.StatusForbidden, res.Status, "%s %s", tc.method, tc.path)
		res = s.Do(tc.method, tc.path, "", tc.body)
		assert.Equal(t, http.StatusUnauthorized, res.Status, "%s %s without a token", tc.method, tc.path)
	}

	res := s.Do(http.MethodGet, "/api/v1/sources", member, nil)
	require.Equal(t, http.StatusOK, res.Status)
	require.Len(t, res.List(), 1)
	assert.Equal(t, map[string]any{"id": id, "kind": "rss", "name": "Feed", "enabled": true}, res.List()[0], "members only see the minimal fields")

	res = s.Do(http.MethodGet, "/api/v1/items", member, nil)
	assert.Equal(t, http.StatusOK, res.Status)
	res = s.Do(http.MethodGet, "/api/v1/items", "", nil)
	assert.Equal(t, http.StatusUnauthorized, res.Status)
	res = s.Do(http.MethodGet, "/api/v1/sources", "", nil)
	assert.Equal(t, http.StatusUnauthorized, res.Status)
}

func TestCollectNowReturnsAJobAdminsCanFollow(t *testing.T) {
	t.Parallel()
	s := newCollectingServer(t)
	admin, _ := setupAdmin(t, s)
	id := createRSSSource(t, s, admin, "Feed", "https://example.com/feed")

	res := s.Do(http.MethodPost, "/api/v1/admin/sources/"+id+"/collect", admin, nil)
	require.Equal(t, http.StatusAccepted, res.Status, res.Body)
	jobID := res.Data()["job_id"].(string)
	again := s.Do(http.MethodPost, "/api/v1/admin/sources/"+id+"/collect", admin, nil)
	assert.Equal(t, jobID, again.Data()["job_id"], "a queued collection is reused")

	res = s.Do(http.MethodGet, "/api/v1/jobs/"+jobID, admin, nil)
	require.Equal(t, http.StatusOK, res.Status)
	assert.Equal(t, "source.collect", res.Data()["kind"])
	assert.Equal(t, "pending", res.Data()["status"])

	var payload, key string
	require.NoError(t, s.Container.DB.QueryRowContext(context.Background(),
		`SELECT payload::text, dedupe_key FROM jobs WHERE id = $1`, jobID).Scan(&payload, &key))
	assert.JSONEq(t, `{"source_id":"`+id+`"}`, payload)
	assert.Equal(t, "source.collect:"+id, key)

	res = s.Do(http.MethodPost, "/api/v1/admin/sources/"+uuid.NewString()+"/collect", admin, nil)
	assert.Equal(t, http.StatusNotFound, res.Status)
}

func TestCollectionJobFillsTheFeed(t *testing.T) {
	t.Parallel()
	s := newCollectingServer(t)
	admin, _ := setupAdmin(t, s)
	member := inviteMember(t, s, admin)
	feed, _ := serveFeed(t, rssXML("Alpha story", "Beta story"))
	id := createRSSSource(t, s, admin, "Example feed", feed.URL)

	res := s.Do(http.MethodPost, "/api/v1/admin/sources/"+id+"/collect", admin, nil)
	require.Equal(t, http.StatusAccepted, res.Status)
	jobID := res.Data()["job_id"].(string)
	runJobs(t, s)

	res = s.Do(http.MethodGet, "/api/v1/jobs/"+jobID, admin, nil)
	assert.Equal(t, "done", res.Data()["status"])
	assert.Equal(t, map[string]any{"fetched": float64(2), "new": float64(2)}, res.Data()["result"])
	assert.JSONEq(t, `{"skipped":"no embedding model"}`, jobResult(t, s.Container.DB, "items.embed"),
		"new items queue the embedding job; it skips without a model")

	res = s.Do(http.MethodGet, "/api/v1/items", member, nil)
	require.Equal(t, http.StatusOK, res.Status, res.Body)
	require.Len(t, res.List(), 2)
	first := res.List()[0]
	assert.Equal(t, "Alpha story", first["title"], "newest first")
	assert.Equal(t, false, first["cited"])
	assert.Equal(t, map[string]any{"id": id, "kind": "rss", "name": "Example feed"}, first["source"])
	assert.Equal(t, "Summary of Alpha story", first["summary"])
	assert.NotEmpty(t, first["published_at"])
	assert.NotEmpty(t, first["collected_at"])
	assert.Contains(t, first, "metrics")
	assert.Equal(t, float64(2), res.Body["meta"].(map[string]any)["total"])

	res = s.Do(http.MethodGet, "/api/v1/admin/sources", admin, nil)
	src := res.List()[0]
	assert.InDelta(t, 2, src["items_24h"], 0)
	assert.NotNil(t, src["last_success_at"])
	assert.NotNil(t, src["last_run_at"])
	assert.Nil(t, src["last_error"])
	assert.InDelta(t, 0, src["consecutive_failures"], 0)

	// The next run is pushed one interval ahead, and collecting again is idempotent.
	var inMinutes float64
	require.NoError(t, s.Container.DB.QueryRowContext(context.Background(),
		`SELECT extract(epoch FROM next_run_at - now()) / 60 FROM sources WHERE id = $1`, id).Scan(&inMinutes))
	assert.InDelta(t, 30, inMinutes, 1)

	res = s.Do(http.MethodPost, "/api/v1/admin/sources/"+id+"/collect", admin, nil)
	jobID = res.Data()["job_id"].(string)
	runJobs(t, s)
	res = s.Do(http.MethodGet, "/api/v1/jobs/"+jobID, admin, nil)
	assert.Equal(t, map[string]any{"fetched": float64(2), "new": float64(0)}, res.Data()["result"])
	assert.Equal(t, 2, countRows(t, s.Container.DB, `SELECT count(*) FROM items`))
}

func TestFailingFeedIsRecordedOnTheSource(t *testing.T) {
	t.Parallel()
	s := newCollectingServer(t)
	admin, _ := setupAdmin(t, s)
	feed, status := serveFeed(t, rssXML("Alpha story"))
	status.Store(http.StatusInternalServerError)
	id := createRSSSource(t, s, admin, "Flaky", feed.URL)

	res := s.Do(http.MethodPost, "/api/v1/admin/sources/"+id+"/collect", admin, nil)
	jobID := res.Data()["job_id"].(string)
	runJobs(t, s)

	res = s.Do(http.MethodGet, "/api/v1/jobs/"+jobID, admin, nil)
	assert.Equal(t, "done", res.Data()["status"], "the source's own schedule retries, not the queue")
	assert.InDelta(t, 1, res.Data()["attempts"], 0)

	res = s.Do(http.MethodGet, "/api/v1/admin/sources", admin, nil)
	src := res.List()[0]
	assert.Contains(t, src["last_error"], "500")
	assert.InDelta(t, 1, src["consecutive_failures"], 0)
	assert.NotNil(t, src["last_run_at"])
	assert.Nil(t, src["last_success_at"])
	var inMinutes float64
	require.NoError(t, s.Container.DB.QueryRowContext(context.Background(),
		`SELECT extract(epoch FROM next_run_at - now()) / 60 FROM sources WHERE id = $1`, id).Scan(&inMinutes))
	assert.InDelta(t, 60, inMinutes, 1, "interval doubled after the first failure")
	assert.Zero(t, countRows(t, s.Container.DB, `SELECT count(*) FROM jobs WHERE kind = 'items.embed'`), "nothing new, nothing to embed")

	// A later success clears the failure state.
	status.Store(http.StatusOK)
	s.Do(http.MethodPost, "/api/v1/admin/sources/"+id+"/collect", admin, nil)
	runJobs(t, s)
	res = s.Do(http.MethodGet, "/api/v1/admin/sources", admin, nil)
	src = res.List()[0]
	assert.Nil(t, src["last_error"])
	assert.InDelta(t, 0, src["consecutive_failures"], 0)
	assert.InDelta(t, 1, src["items_24h"], 0)
}

func TestDisabledSourceIsSkippedAndMissingSourceFails(t *testing.T) {
	t.Parallel()
	s := newCollectingServer(t)
	admin, _ := setupAdmin(t, s)
	feed, _ := serveFeed(t, rssXML("Alpha story"))
	id := createRSSSource(t, s, admin, "Feed", feed.URL)
	s.Do(http.MethodPatch, "/api/v1/admin/sources/"+id, admin, map[string]any{"enabled": false})

	res := s.Do(http.MethodPost, "/api/v1/admin/sources/"+id+"/collect", admin, nil)
	jobID := res.Data()["job_id"].(string)
	runJobs(t, s)
	res = s.Do(http.MethodGet, "/api/v1/jobs/"+jobID, admin, nil)
	assert.Equal(t, "done", res.Data()["status"])
	assert.Equal(t, map[string]any{"skipped": "disabled"}, res.Data()["result"])
	assert.Zero(t, countRows(t, s.Container.DB, `SELECT count(*) FROM items`))

	gone, err := s.Container.Queue.Enqueue(context.Background(), services.JobCollectSource, map[string]string{"source_id": uuid.NewString()}, workers.EnqueueOptions{})
	require.NoError(t, err)
	runJobs(t, s)
	res = s.Do(http.MethodGet, "/api/v1/jobs/"+gone.String(), admin, nil)
	assert.Equal(t, "failed", res.Data()["status"], "a missing source is not retried")
	assert.InDelta(t, 1, res.Data()["attempts"], 0)
}

func TestSchedulerEnqueuesDueSourcesOnce(t *testing.T) {
	t.Parallel()
	s := newCollectingServer(t)
	admin, _ := setupAdmin(t, s)
	dueID := createRSSSource(t, s, admin, "Due", "https://example.com/due")
	createRSSSource(t, s, admin, "Also due", "https://example.com/also")
	offID := createRSSSource(t, s, admin, "Disabled", "https://example.com/off")
	s.Do(http.MethodPatch, "/api/v1/admin/sources/"+offID, admin, map[string]any{"enabled": false})
	laterID := createRSSSource(t, s, admin, "Later", "https://example.com/later")
	mustExec(t, s.Container.DB, `UPDATE sources SET next_run_at = now() + interval '1 hour' WHERE id = $1`, laterID)

	ctx := context.Background()
	require.NoError(t, s.Container.Sources.EnqueueDue(ctx, time.Now().UTC()))
	require.NoError(t, s.Container.Sources.EnqueueDue(ctx, time.Now().UTC()), "ticks of several replicas are harmless")

	assert.Equal(t, 2, countRows(t, s.Container.DB, `SELECT count(*) FROM jobs WHERE kind = 'source.collect'`))
	assert.Equal(t, 1, countRows(t, s.Container.DB, `SELECT count(*) FROM jobs WHERE dedupe_key = $1`, "source.collect:"+dueID))
	assert.Zero(t, countRows(t, s.Container.DB, `SELECT count(*) FROM jobs WHERE dedupe_key = $1 OR dedupe_key = $2`, "source.collect:"+offID, "source.collect:"+laterID))
}

func TestDeletingASourceWithCitedItemsIsRefused(t *testing.T) {
	t.Parallel()
	s := newCollectingServer(t)
	admin, _ := setupAdmin(t, s)
	citedID := createRSSSource(t, s, admin, "Cited", "https://example.com/cited")
	plainID := createRSSSource(t, s, admin, "Plain", "https://example.com/plain")

	items := repositories.NewItemRepository(s.Container.DB)
	now := time.Now().UTC()
	for _, id := range []string{citedID, plainID} {
		_, err := items.Upsert(context.Background(), uuid.MustParse(id), []models.Item{
			{ExternalID: "1", URL: "https://e.com/1", Title: "T", PublishedAt: now, CollectedAt: now},
		})
		require.NoError(t, err)
	}
	citeItem(t, s.Container.DB, itemIDOf(t, s.Container.DB, uuid.MustParse(citedID), "1"))

	res := s.Do(http.MethodDelete, "/api/v1/admin/sources/"+citedID, admin, nil)
	assert.Equal(t, http.StatusConflict, res.Status)
	assert.Equal(t, "SOURCE_HAS_CITED_ITEMS", res.ErrorCode())
	assert.Equal(t, 1, countRows(t, s.Container.DB, `SELECT count(*) FROM sources WHERE id = $1`, citedID))

	// Disabling is the way out.
	res = s.Do(http.MethodPatch, "/api/v1/admin/sources/"+citedID, admin, map[string]any{"enabled": false})
	assert.Equal(t, http.StatusOK, res.Status)

	res = s.Do(http.MethodDelete, "/api/v1/admin/sources/"+plainID, admin, nil)
	assert.Equal(t, http.StatusNoContent, res.Status)
	assert.Equal(t, 1, countRows(t, s.Container.DB, `SELECT count(*) FROM items`), "uncited items cascade with their source")
}

func TestItemFeedQueryParameters(t *testing.T) {
	t.Parallel()
	s := newCollectingServer(t)
	admin, _ := setupAdmin(t, s)
	feed, _ := serveFeed(t, rssXML("Postgres news", "Rust news", "Go news"))
	id := createRSSSource(t, s, admin, "Feed", feed.URL)
	s.Do(http.MethodPost, "/api/v1/admin/sources/"+id+"/collect", admin, nil)
	runJobs(t, s)

	res := s.Do(http.MethodGet, "/api/v1/items?q=postgres", admin, nil)
	require.Equal(t, http.StatusOK, res.Status)
	require.Len(t, res.List(), 1)
	assert.Equal(t, "Postgres news", res.List()[0]["title"])

	res = s.Do(http.MethodGet, "/api/v1/items?limit=2&offset=1&source_id="+id, admin, nil)
	require.Equal(t, http.StatusOK, res.Status)
	assert.Len(t, res.List(), 2)
	assert.Equal(t, map[string]any{"total": float64(3), "limit": float64(2), "offset": float64(1)}, res.Body["meta"])

	res = s.Do(http.MethodGet, "/api/v1/items?limit=1000", admin, nil)
	assert.Equal(t, float64(100), res.Body["meta"].(map[string]any)["limit"], "the server caps the page size")

	res = s.Do(http.MethodGet, "/api/v1/items?sort=engagement&since=2026-10-26T09:30:00Z&until=2026-10-27T00:00:00Z", admin, nil)
	require.Equal(t, http.StatusOK, res.Status, res.Body)
	assert.Len(t, res.List(), 1, "only the entry published at 10:00 is inside the window")

	for _, query := range []string{
		"source_id=nope", "since=yesterday", "until=2026-10-26", "sort=random", "limit=0", "offset=-1",
		"since=2026-10-27T00:00:00Z&until=2026-10-26T00:00:00Z", "q=" + strings.Repeat("a", 201),
	} {
		res = s.Do(http.MethodGet, "/api/v1/items?"+query, admin, nil)
		assert.Equal(t, http.StatusBadRequest, res.Status, query)
	}
}

// embeddingServer starts an API whose AI client reaches a fake OpenAI-compatible
// server, with an embedding model configured through the admin API.
func embeddingServer(t *testing.T, configureModel bool) (*apitest.Server, string) {
	t.Helper()
	s := newCollectingServer(t)
	admin, _ := setupAdmin(t, s)
	if !configureModel {
		return s, admin
	}
	fake := fakeOpenAI(t)
	res := s.Do(http.MethodPost, "/api/v1/admin/ai/providers", admin, map[string]string{"name": "Local", "kind": "openai_compatible", "base_url": fake.URL + "/v1"})
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	providerID := res.Data()["id"].(string)
	res = s.Do(http.MethodPost, "/api/v1/admin/ai/providers/"+providerID+"/models", admin, map[string]string{"name": "nomic-embed-text", "capability": "embedding"})
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	res = s.Do(http.MethodPatch, "/api/v1/admin/settings", admin, map[string]string{"embedding_model_id": res.Data()["id"].(string)})
	require.Equal(t, http.StatusOK, res.Status, res.Body)
	return s, admin
}

func seedItems(t *testing.T, s *apitest.Server, admin string, n int) {
	t.Helper()
	id := createRSSSource(t, s, admin, "Seed", "https://example.com/seed")
	now := time.Now().UTC()
	batch := make([]models.Item, n)
	for i := range batch {
		batch[i] = models.Item{
			ExternalID: fmt.Sprint(i), URL: fmt.Sprintf("https://e.com/%d", i), Title: fmt.Sprintf("Title %d", i), Summary: "S",
			PublishedAt: now, CollectedAt: now.Add(time.Duration(i) * time.Millisecond),
		}
	}
	_, err := repositories.NewItemRepository(s.Container.DB).Upsert(context.Background(), uuid.MustParse(id), batch)
	require.NoError(t, err)
}

func TestEmbeddingJobStoresVectors(t *testing.T) {
	t.Parallel()
	s, admin := embeddingServer(t, true)
	seedItems(t, s, admin, 70) // more than one batch of 64

	jobID, err := s.Container.Queue.Enqueue(context.Background(), services.JobEmbedItems, struct{}{}, workers.EnqueueOptions{})
	require.NoError(t, err)
	runJobs(t, s)

	db := s.Container.DB
	assert.Equal(t, 70, countRows(t, db, `SELECT count(*) FROM items WHERE embedding IS NOT NULL AND embedding_model = 'nomic-embed-text' AND embedded_at IS NOT NULL`))
	res := s.Do(http.MethodGet, "/api/v1/jobs/"+jobID.String(), admin, nil)
	assert.Equal(t, "done", res.Data()["status"])
	assert.Equal(t, map[string]any{"embedded": float64(70)}, res.Data()["result"])
	assert.Equal(t, 2, countRows(t, db, `SELECT count(*) FROM ai_usage WHERE purpose = 'embedding'`), "one request of 64 and one of 6, each accounted")
}

func TestEmbeddingJobWithoutModelIsSkipped(t *testing.T) {
	t.Parallel()
	s, admin := embeddingServer(t, false)
	seedItems(t, s, admin, 3)

	jobID, err := s.Container.Queue.Enqueue(context.Background(), services.JobEmbedItems, struct{}{}, workers.EnqueueOptions{})
	require.NoError(t, err)
	runJobs(t, s)

	res := s.Do(http.MethodGet, "/api/v1/jobs/"+jobID.String(), admin, nil)
	assert.Equal(t, "done", res.Data()["status"], "no model is not an error")
	assert.Equal(t, map[string]any{"skipped": "no embedding model"}, res.Data()["result"])
	assert.Equal(t, 3, countRows(t, s.Container.DB, `SELECT count(*) FROM items WHERE embedding IS NULL`))
}
