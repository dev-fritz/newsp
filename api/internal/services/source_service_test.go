package services_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/dev-fritz/newsp/api/internal/apperrors"
	"github.com/dev-fritz/newsp/api/internal/auth"
	"github.com/dev-fritz/newsp/api/internal/models"
	"github.com/dev-fritz/newsp/api/internal/repositories"
	"github.com/dev-fritz/newsp/api/internal/services"
	"github.com/dev-fritz/newsp/api/internal/workers"
	"github.com/dev-fritz/newsp/api/pkg/connectors"
	"github.com/dev-fritz/newsp/api/pkg/secretbox"
	"github.com/dev-fritz/newsp/api/tests/mocks"
)

type sourceDeps struct {
	sources   *mocks.MockSourceStore
	items     *mocks.MockItemStore
	registry  *mocks.MockConnectorRegistry
	connector *mocks.MockConnector
	embedder  *mocks.MockEmbedder
	queue     *mocks.MockEnqueuer
	box       *secretbox.Box
}

func newSourceService(t *testing.T) (*services.SourceService, sourceDeps) {
	t.Helper()
	ctrl := gomock.NewController(t)
	d := sourceDeps{
		sources: mocks.NewMockSourceStore(ctrl), items: mocks.NewMockItemStore(ctrl),
		registry: mocks.NewMockConnectorRegistry(ctrl), connector: mocks.NewMockConnector(ctrl),
		embedder: mocks.NewMockEmbedder(ctrl), queue: mocks.NewMockEnqueuer(ctrl),
	}
	box, err := secretbox.New(bytes.Repeat([]byte{7}, secretbox.KeySize))
	require.NoError(t, err)
	d.box = box
	svc := services.NewSourceService(d.sources, d.items, d.registry, d.embedder, d.queue, box, func() time.Time { return fixedNow })
	return svc, d
}

func collectJob(id uuid.UUID) models.Job {
	return models.Job{ID: uuid.New(), Kind: services.JobCollectSource, Payload: json.RawMessage(`{"source_id":"` + id.String() + `"}`)}
}

func testSource(failures int) models.Source {
	return models.Source{
		ID: uuid.New(), Kind: models.SourceRSS, Name: "Feed", Config: json.RawMessage(`{"url":"https://example.com/feed"}`),
		IntervalMinutes: 30, Enabled: true, ConsecutiveFailures: failures,
	}
}

func TestSourceAdministrationRequiresPermission(t *testing.T) {
	t.Parallel()
	member := auth.Identity{UserID: uuid.New(), Role: models.RoleMember}
	svc, _ := newSourceService(t)
	ctx := context.Background()
	id := uuid.New()

	_, err := svc.List(ctx, member)
	requireCode(t, err, "FORBIDDEN")
	_, err = svc.Create(ctx, member, services.SourceInput{Kind: "rss", Name: "x", Config: json.RawMessage(`{}`)})
	requireCode(t, err, "FORBIDDEN")
	_, err = svc.Update(ctx, member, id, services.SourceChange{})
	requireCode(t, err, "FORBIDDEN")
	requireCode(t, svc.Delete(ctx, member, id), "FORBIDDEN")
	_, err = svc.CollectNow(ctx, member, id)
	requireCode(t, err, "FORBIDDEN")
}

func TestCreateSourceValidatesKindAndConfig(t *testing.T) {
	t.Parallel()
	admin := auth.Identity{UserID: uuid.New(), Role: models.RoleAdmin}
	ctx := context.Background()

	t.Run("unknown kind", func(t *testing.T) {
		t.Parallel()
		svc, d := newSourceService(t)
		d.registry.EXPECT().Get("nope").Return(nil, false)
		_, err := svc.Create(ctx, admin, services.SourceInput{Kind: "nope", Name: "x", Config: json.RawMessage(`{}`)})
		requireCode(t, err, "VALIDATION_FAILED")
	})

	t.Run("config error becomes config.field", func(t *testing.T) {
		t.Parallel()
		svc, d := newSourceService(t)
		d.registry.EXPECT().Get("rss").Return(d.connector, true)
		d.connector.EXPECT().Normalize(gomock.Any()).Return(nil, &connectors.ConfigError{Field: "url", Reason: "must be an absolute http(s) URL"})
		_, err := svc.Create(ctx, admin, services.SourceInput{Kind: "rss", Name: "x", Config: json.RawMessage(`{"url":"x"}`)})
		requireCode(t, err, "VALIDATION_FAILED")
		appErr, _ := apperrors.As(err)
		assert.Equal(t, map[string]string{"config.url": "must be an absolute http(s) URL"}, appErr.Fields)
	})

	t.Run("config must be an object", func(t *testing.T) {
		t.Parallel()
		svc, d := newSourceService(t)
		d.registry.EXPECT().Get("rss").Return(d.connector, true)
		_, err := svc.Create(ctx, admin, services.SourceInput{Kind: "rss", Name: "x", Config: json.RawMessage(`[1]`)})
		requireCode(t, err, "VALIDATION_FAILED")
	})

	t.Run("defaults", func(t *testing.T) {
		t.Parallel()
		svc, d := newSourceService(t)
		d.registry.EXPECT().Get("rss").Return(d.connector, true)
		d.connector.EXPECT().Normalize(gomock.Any()).Return(json.RawMessage(`{"url":"https://e.com","limit":50}`), nil)
		d.connector.EXPECT().Secret(gomock.Any()).Return(connectors.SecretNone)
		d.sources.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, s models.Source) error {
			assert.Equal(t, 30, s.IntervalMinutes)
			assert.True(t, s.Enabled)
			assert.Equal(t, fixedNow, s.NextRunAt, "runs as soon as the worker sees it")
			assert.Equal(t, "Feed", s.Name, "names are trimmed")
			return nil
		})
		view, err := svc.Create(ctx, admin, services.SourceInput{Kind: "rss", Name: "  Feed ", Config: json.RawMessage(`{"url":"https://e.com"}`)})
		require.NoError(t, err)
		assert.JSONEq(t, `{"url":"https://e.com","limit":50}`, string(view.Source.Config), "the normalized config is stored")
	})
}

func TestUpdateSourceEnablingRunsSoon(t *testing.T) {
	t.Parallel()
	admin := auth.Identity{UserID: uuid.New(), Role: models.RoleAdmin}
	svc, d := newSourceService(t)
	src := testSource(3)
	src.Enabled = false
	src.NextRunAt = fixedNow.Add(6 * time.Hour)
	d.sources.EXPECT().GetByID(gomock.Any(), src.ID).Return(src, nil)
	d.sources.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, s models.Source) error {
		assert.True(t, s.Enabled)
		assert.Equal(t, fixedNow, s.NextRunAt)
		return nil
	})
	d.sources.EXPECT().GetWithStats(gomock.Any(), src.ID, fixedNow.Add(-24*time.Hour)).Return(repositories.SourceStats{Source: src, Items: 4}, nil)

	view, err := svc.Update(context.Background(), admin, src.ID, services.SourceChange{Enabled: ptr(true)})
	require.NoError(t, err)
	assert.Equal(t, 4, view.Items)
}

func TestDeleteSourceTranslatesErrors(t *testing.T) {
	t.Parallel()
	admin := auth.Identity{UserID: uuid.New(), Role: models.RoleAdmin}
	id := uuid.New()
	for name, tc := range map[string]struct {
		err  error
		code string
	}{
		"cited items":   {repositories.ErrReferenced, "SOURCE_HAS_CITED_ITEMS"},
		"missing":       {repositories.ErrNotFound, "SOURCE_NOT_FOUND"},
		"deleted fine":  {nil, ""},
		"database down": {errors.New("boom"), ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			svc, d := newSourceService(t)
			d.sources.EXPECT().Delete(gomock.Any(), id).Return(tc.err)
			err := svc.Delete(context.Background(), admin, id)
			switch {
			case tc.code != "":
				requireCode(t, err, tc.code)
			case tc.err != nil:
				require.ErrorIs(t, err, tc.err)
			default:
				require.NoError(t, err)
			}
		})
	}
}

func TestCollectNowEnqueuesDeduplicatedJob(t *testing.T) {
	t.Parallel()
	admin := auth.Identity{UserID: uuid.New(), Role: models.RoleAdmin}
	svc, d := newSourceService(t)
	src := testSource(0)
	jobID := uuid.New()
	d.sources.EXPECT().GetByID(gomock.Any(), src.ID).Return(src, nil)
	d.queue.EXPECT().Enqueue(gomock.Any(), services.JobCollectSource, gomock.Any(), workers.EnqueueOptions{DedupeKey: "source.collect:" + src.ID.String()}).
		DoAndReturn(func(_ context.Context, _ string, payload any, _ workers.EnqueueOptions) (uuid.UUID, error) {
			raw, err := json.Marshal(payload)
			require.NoError(t, err)
			assert.JSONEq(t, `{"source_id":"`+src.ID.String()+`"}`, string(raw))
			return jobID, nil
		})

	got, err := svc.CollectNow(context.Background(), admin, src.ID)
	require.NoError(t, err)
	assert.Equal(t, jobID, got)
}

func TestCollectSourceSkipsDisabledSources(t *testing.T) {
	t.Parallel()
	svc, d := newSourceService(t)
	src := testSource(0)
	src.Enabled = false
	d.sources.EXPECT().GetByID(gomock.Any(), src.ID).Return(src, nil)

	res, err := svc.CollectSource(context.Background(), collectJob(src.ID))
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"skipped": "disabled"}, res)
}

func TestCollectSourceRejectsBadJobs(t *testing.T) {
	t.Parallel()
	svc, d := newSourceService(t)

	_, err := svc.CollectSource(context.Background(), models.Job{Payload: json.RawMessage(`{"source_id":"nope"}`)})
	require.Error(t, err)

	id := uuid.New()
	d.sources.EXPECT().GetByID(gomock.Any(), id).Return(models.Source{}, repositories.ErrNotFound)
	_, err = svc.CollectSource(context.Background(), collectJob(id))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestCollectSourceFailureBackoff(t *testing.T) {
	t.Parallel()
	interval := 30 * time.Minute
	for name, tc := range map[string]struct {
		failuresBefore int
		wait           time.Duration
	}{
		"first failure doubles the interval": {0, 2 * interval},
		"second failure":                     {1, 4 * interval},
		"third failure":                      {2, 8 * interval},
		"capped at six hours":                {3, 6 * time.Hour},
		"stays capped":                       {50, 6 * time.Hour},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			svc, d := newSourceService(t)
			src := testSource(tc.failuresBefore)
			d.sources.EXPECT().GetByID(gomock.Any(), src.ID).Return(src, nil)
			d.registry.EXPECT().Get("rss").Return(d.connector, true)
			d.connector.EXPECT().Fetch(gomock.Any(), src.Config, "").Return(nil, errors.New("fetch feed: unexpected HTTP status 500"))
			d.sources.EXPECT().MarkFailure(gomock.Any(), src.ID, fixedNow, "fetch feed: unexpected HTTP status 500", fixedNow.Add(tc.wait)).Return(nil)

			res, err := svc.CollectSource(context.Background(), collectJob(src.ID))
			require.NoError(t, err, "a connector failure does not fail the job")
			assert.Equal(t, map[string]any{"fetched": 0, "new": 0, "error": "fetch feed: unexpected HTTP status 500"}, res)
		})
	}
}

func TestCollectSourceFailureMessageIsShortAndOneLine(t *testing.T) {
	t.Parallel()
	svc, d := newSourceService(t)
	src := testSource(0)
	d.sources.EXPECT().GetByID(gomock.Any(), src.ID).Return(src, nil)
	d.registry.EXPECT().Get("rss").Return(d.connector, true)
	d.connector.EXPECT().Fetch(gomock.Any(), src.Config, "").Return(nil, errors.New("line one\nline two\t"+strings.Repeat("é", 800)))
	d.sources.EXPECT().MarkFailure(gomock.Any(), src.ID, fixedNow, gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ uuid.UUID, _ time.Time, msg string, _ time.Time) error {
			assert.NotContains(t, msg, "\n")
			assert.Len(t, []rune(msg), 500)
			assert.True(t, strings.HasPrefix(msg, "line one line two é"))
			return nil
		})

	_, err := svc.CollectSource(context.Background(), collectJob(src.ID))
	require.NoError(t, err)
}

func TestCollectSourceReturnsCanceledContextForRetry(t *testing.T) {
	t.Parallel()
	svc, d := newSourceService(t)
	src := testSource(0)
	ctx, cancel := context.WithCancel(context.Background())
	d.sources.EXPECT().GetByID(gomock.Any(), src.ID).Return(src, nil)
	d.registry.EXPECT().Get("rss").Return(d.connector, true)
	d.connector.EXPECT().Fetch(gomock.Any(), src.Config, "").DoAndReturn(func(context.Context, json.RawMessage, string) ([]connectors.Item, error) {
		cancel()
		return nil, context.Canceled
	})

	_, err := svc.CollectSource(ctx, collectJob(src.ID))
	require.ErrorIs(t, err, context.Canceled, "the queue retries; the source is not marked as failed")
}

func TestCollectSourceStoresItemsAndQueuesEmbedding(t *testing.T) {
	t.Parallel()
	score := 7
	fetched := []connectors.Item{
		{ExternalID: "a", URL: "https://e.com/a", Title: "A", PublishedAt: fixedNow.Add(-time.Hour), Score: &score, Metrics: map[string]any{"points": 7}},
		{ExternalID: "b", URL: "https://e.com/b", Title: "B"},
		{ExternalID: "", URL: "https://e.com/skipped", Title: "no id"},
	}

	for name, tc := range map[string]struct {
		added       int
		wantEnqueue bool
	}{"new items queue the embedding job": {2, true}, "nothing new": {0, false}} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			svc, d := newSourceService(t)
			src := testSource(2)
			d.sources.EXPECT().GetByID(gomock.Any(), src.ID).Return(src, nil)
			d.registry.EXPECT().Get("rss").Return(d.connector, true)
			d.connector.EXPECT().Fetch(gomock.Any(), src.Config, "").Return(fetched, nil)
			d.items.EXPECT().Upsert(gomock.Any(), src.ID, gomock.Any()).DoAndReturn(func(_ context.Context, _ uuid.UUID, items []models.Item) (int, error) {
				require.Len(t, items, 2, "items without an external id are dropped")
				assert.Equal(t, fixedNow, items[0].CollectedAt)
				assert.JSONEq(t, `{"points":7}`, string(items[0].Metrics))
				assert.Equal(t, fixedNow, items[1].PublishedAt, "missing dates fall back to the collection time")
				return tc.added, nil
			})
			d.sources.EXPECT().MarkSuccess(gomock.Any(), src.ID, fixedNow, fixedNow.Add(30*time.Minute)).Return(nil)
			if tc.wantEnqueue {
				d.queue.EXPECT().Enqueue(gomock.Any(), services.JobEmbedItems, gomock.Any(), workers.EnqueueOptions{DedupeKey: "items.embed"}).Return(uuid.New(), nil)
			}

			res, err := svc.CollectSource(context.Background(), collectJob(src.ID))
			require.NoError(t, err)
			assert.Equal(t, map[string]int{"fetched": 3, "new": tc.added}, res)
		})
	}
}

func TestCollectSourceDatabaseErrorsFailTheJob(t *testing.T) {
	t.Parallel()
	svc, d := newSourceService(t)
	src := testSource(0)
	boom := errors.New("connection reset")
	d.sources.EXPECT().GetByID(gomock.Any(), src.ID).Return(src, nil)
	d.registry.EXPECT().Get("rss").Return(d.connector, true)
	d.connector.EXPECT().Fetch(gomock.Any(), src.Config, "").Return([]connectors.Item{{ExternalID: "a", URL: "https://e.com/a"}}, nil)
	d.items.EXPECT().Upsert(gomock.Any(), src.ID, gomock.Any()).Return(0, boom)

	_, err := svc.CollectSource(context.Background(), collectJob(src.ID))
	require.ErrorIs(t, err, boom)
}

func TestEnqueueDueQueuesEachSourceOnce(t *testing.T) {
	t.Parallel()
	svc, d := newSourceService(t)
	a, b := uuid.New(), uuid.New()
	d.sources.EXPECT().ListDue(gomock.Any(), fixedNow, 100).Return([]uuid.UUID{a, b}, nil)
	d.queue.EXPECT().Enqueue(gomock.Any(), services.JobCollectSource, gomock.Any(), workers.EnqueueOptions{DedupeKey: "source.collect:" + a.String()}).Return(uuid.New(), nil)
	d.queue.EXPECT().Enqueue(gomock.Any(), services.JobCollectSource, gomock.Any(), workers.EnqueueOptions{DedupeKey: "source.collect:" + b.String()}).Return(uuid.New(), nil)

	require.NoError(t, svc.EnqueueDue(context.Background(), fixedNow))
}

func TestEmbedPendingWithoutModelIsSkipped(t *testing.T) {
	t.Parallel()
	svc, d := newSourceService(t)
	d.items.EXPECT().Pending(gomock.Any(), 64).Return([]repositories.PendingItem{{ID: uuid.New(), Title: "T", Summary: "S"}}, nil)
	d.embedder.EXPECT().Embed(gomock.Any(), models.PurposeEmbedding, []string{"T\n\nS"}).Return(nil, "", services.ErrEmbeddingModelNotSet)

	res, err := svc.EmbedPending(context.Background(), models.Job{})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"skipped": "no embedding model"}, res)
}

func TestEmbedPendingStopsWhenNothingIsPending(t *testing.T) {
	t.Parallel()
	svc, d := newSourceService(t)
	d.items.EXPECT().Pending(gomock.Any(), 64).Return(nil, nil)

	res, err := svc.EmbedPending(context.Background(), models.Job{})
	require.NoError(t, err)
	assert.Equal(t, map[string]int{"embedded": 0}, res)
}

func TestEmbedPendingContinuesInAnotherJobAfterTheCap(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		dedupeKey *string
		wantKey   string
	}{
		"plain job continues under the alternate key": {ptr("items.embed"), "items.embed:next"},
		"alternate job goes back to the plain key":    {ptr("items.embed:next"), "items.embed"},
		"job without a key uses the plain key":        {nil, "items.embed"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			svc, d := newSourceService(t)
			pending := make([]repositories.PendingItem, 64)
			vectors := make([][]float32, 64)
			for i := range pending {
				pending[i] = repositories.PendingItem{ID: uuid.New(), Title: "T", Summary: "S"}
				vectors[i] = []float32{1}
			}
			d.items.EXPECT().Pending(gomock.Any(), 64).Return(pending, nil).Times(20)
			d.embedder.EXPECT().Embed(gomock.Any(), models.PurposeEmbedding, gomock.Any()).Return(vectors, "nomic", nil).Times(20)
			d.items.EXPECT().SetEmbeddings(gomock.Any(), gomock.Any(), vectors, "nomic", fixedNow).Return(nil).Times(20)
			d.items.EXPECT().HasPending(gomock.Any()).Return(true, nil)
			d.queue.EXPECT().Enqueue(gomock.Any(), services.JobEmbedItems, gomock.Any(), workers.EnqueueOptions{DedupeKey: tc.wantKey}).Return(uuid.New(), nil)

			res, err := svc.EmbedPending(context.Background(), models.Job{DedupeKey: tc.dedupeKey})
			require.NoError(t, err)
			assert.Equal(t, map[string]int{"embedded": 20 * 64}, res)
		})
	}
}
