package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dev-fritz/newsp/api/internal/bootstrap"
	"github.com/dev-fritz/newsp/api/internal/models"
	"github.com/dev-fritz/newsp/api/pkg/safehttp"
	"github.com/dev-fritz/newsp/api/tests/apitest"
)

func TestRetentionKeepsRecentAndCitedItems(t *testing.T) {
	t.Parallel()
	s := apitest.New(t)
	setupAdmin(t, s)
	ctx := context.Background()
	db := s.Container.DB

	var userID, sourceID uuid.UUID
	require.NoError(t, db.QueryRowContext(ctx, `SELECT id FROM users LIMIT 1`).Scan(&userID))
	sourceID = uuid.New()
	_, err := db.ExecContext(ctx, `INSERT INTO sources (id, kind, name) VALUES ($1, 'rss', 'Feed')`, sourceID)
	require.NoError(t, err)

	insertItem := func(name string, age time.Duration) uuid.UUID {
		id := uuid.New()
		_, err := db.ExecContext(ctx, `INSERT INTO items (id, source_id, external_id, url, title, published_at, collected_at)
			VALUES ($1, $2, $3, 'https://example.com/' || $3, $3, now(), now() - $4::interval)`, id, sourceID, name, age.String())
		require.NoError(t, err)
		return id
	}
	const old = 200 * 24 * time.Hour // retention defaults to 90 days
	recent := insertItem("recent", time.Hour)
	oldUncited := insertItem("old-uncited", old)
	oldReported := insertItem("old-reported", old)
	oldInIdea := insertItem("old-in-idea", old)

	reportID, ideaID, versionID := uuid.New(), uuid.New(), uuid.New()
	_, err = db.ExecContext(ctx, `INSERT INTO reports (id, user_id, kind, window_start, window_end, output_lang)
		VALUES ($1, $2, 'manual', now() - interval '2 days', now(), 'en')`, reportID, userID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO report_items (report_id, item_id, ref) VALUES ($1, $2, 1)`, reportID, oldReported)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO ideas (id, user_id, title, summary) VALUES ($1, $2, 'T', 'S')`, ideaID, userID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO idea_versions (id, idea_id, version, origin, title, summary, difficulty, content)
		VALUES ($1, $2, 1, 'generated', 'T', 'S', 'beginner', '{}')`, versionID, ideaID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO idea_evidence (idea_version_id, item_id, ref) VALUES ($1, $2, 'E1')`, versionID, oldInIdea)
	require.NoError(t, err)

	res, err := s.Container.Maintenance.Retention(ctx, models.Job{})
	require.NoError(t, err)
	assert.Equal(t, map[string]int{"deleted": 1}, res)

	exists := func(id uuid.UUID) bool {
		var ok bool
		require.NoError(t, db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM items WHERE id = $1)`, id).Scan(&ok))
		return ok
	}
	assert.True(t, exists(recent))
	assert.True(t, exists(oldReported))
	assert.True(t, exists(oldInIdea))
	assert.False(t, exists(oldUncited))
}

func TestReindexWithoutEmbeddingModelIsSkipped(t *testing.T) {
	t.Parallel()
	s := apitest.New(t)
	setupAdmin(t, s)

	res, err := s.Container.Maintenance.Reindex(context.Background(), models.Job{})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"skipped": "no embedding model"}, res)
}

func TestReindexMovesEveryVectorToTheNewModel(t *testing.T) {
	t.Parallel()
	fake := &fakeAI{}
	s := apitest.NewWithOptions(t, bootstrap.Options{
		AIFactory:  fake.factory,
		HTTPClient: safehttp.NewClient(safehttp.Options{AllowPrivate: true}),
	})
	admin, _ := setupAdmin(t, s)
	feed := rssFeed(t)
	ctx := context.Background()

	res := s.Do(http.MethodPost, "/api/v1/admin/ai/providers", admin, map[string]string{"name": "Local", "kind": "openai_compatible", "base_url": "http://ollama:11434/v1"})
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	providerID := res.Data()["id"].(string)
	addModel := func(name, capability string) string {
		r := s.Do(http.MethodPost, "/api/v1/admin/ai/providers/"+providerID+"/models", admin, map[string]string{"name": name, "capability": capability})
		require.Equal(t, http.StatusCreated, r.Status, r.Body)
		return r.Data()["id"].(string)
	}
	chatID := addModel("fake-chat", "chat")
	embedA := addModel("embed-a", "embedding")
	embedB := addModel("embed-b", "embedding")
	res = s.Do(http.MethodPatch, "/api/v1/admin/settings", admin, map[string]any{
		"embedding_model_id": embedA, "default_report_model_id": chatID, "default_idea_model_id": chatID,
	})
	require.Equal(t, http.StatusOK, res.Status, res.Body)

	res = s.Do(http.MethodPost, "/api/v1/admin/sources", admin, map[string]any{"kind": "rss", "name": "Tech", "config": map[string]string{"url": feed.URL}})
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	res = s.Do(http.MethodPost, "/api/v1/admin/sources/"+res.Data()["id"].(string)+"/collect", admin, nil)
	require.Equal(t, http.StatusAccepted, res.Status)
	res = s.Do(http.MethodPut, "/api/v1/me/preferences", admin, map[string]any{
		"topics": []string{"revisão de código", "custo de LLM"}, "output_lang": "pt-BR", "timezone": "America/Sao_Paulo",
		"report_enabled": false, "report_time": "08:00", "ideas_per_day": 2, "theme": "dark", "excluded_source_ids": []string{}, "ui_locale": "pt-BR",
	})
	require.Equal(t, http.StatusOK, res.Status, res.Body)
	drain(t, s)

	fake.propose(reviewIdea)
	res = s.Do(http.MethodPost, "/api/v1/ideas/generate", admin, map[string]any{"count": 1})
	require.Equal(t, http.StatusAccepted, res.Status, res.Body)
	drain(t, s)

	count := func(q string, args ...any) int {
		var n int
		require.NoError(t, s.Container.DB.QueryRowContext(ctx, q, args...).Scan(&n))
		return n
	}
	require.Equal(t, 3, count(`SELECT count(*) FROM items WHERE embedding_model = 'embed-a'`))
	require.Equal(t, 1, count(`SELECT count(*) FROM ideas WHERE embedding_model = 'embed-a'`))
	require.Equal(t, 1, count(`SELECT count(*) FROM user_preferences WHERE topics_embedding_model = 'embed-a'`))

	// Switching the model enqueues the reindex job.
	res = s.Do(http.MethodPatch, "/api/v1/admin/settings", admin, map[string]any{"embedding_model_id": embedB})
	require.Equal(t, http.StatusOK, res.Status, res.Body)
	drain(t, s)

	assert.Equal(t, 3, count(`SELECT count(*) FROM items WHERE embedding_model = 'embed-b' AND embedding IS NOT NULL AND embedded_at IS NOT NULL`))
	assert.Equal(t, 0, count(`SELECT count(*) FROM items WHERE embedding_model IS DISTINCT FROM 'embed-b'`))
	assert.Equal(t, 1, count(`SELECT count(*) FROM ideas WHERE embedding_model = 'embed-b' AND embedding IS NOT NULL`))
	assert.Equal(t, 1, count(`SELECT count(*) FROM user_preferences WHERE topics_embedding_model = 'embed-b' AND topics_embedding IS NOT NULL`))
	assert.Equal(t, 0, count(`SELECT count(*) FROM jobs WHERE kind = 'embeddings.reindex' AND status <> 'done'`))
}
