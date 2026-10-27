package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/dev-fritz/newsp/api/internal/models"
	"github.com/dev-fritz/newsp/api/internal/repositories"
	"github.com/dev-fritz/newsp/api/internal/services"
	"github.com/dev-fritz/newsp/api/internal/workers"
	"github.com/dev-fritz/newsp/api/tests/mocks"
)

type maintenanceDeps struct {
	store    *mocks.MockMaintenanceStore
	settings *mocks.MockMaintenanceSettings
	embedder *mocks.MockReindexEmbedder
	queue    *mocks.MockEnqueuer
}

func newMaintenanceService(t *testing.T) (*services.MaintenanceService, maintenanceDeps) {
	t.Helper()
	ctrl := gomock.NewController(t)
	d := maintenanceDeps{
		store: mocks.NewMockMaintenanceStore(ctrl), settings: mocks.NewMockMaintenanceSettings(ctrl),
		embedder: mocks.NewMockReindexEmbedder(ctrl), queue: mocks.NewMockEnqueuer(ctrl),
	}
	return services.NewMaintenanceService(d.store, d.settings, d.embedder, d.queue, func() time.Time { return fixedNow }), d
}

func TestReindexWithoutModelIsSkipped(t *testing.T) {
	t.Parallel()
	svc, d := newMaintenanceService(t)
	d.embedder.EXPECT().EmbeddingModel(gomock.Any()).Return(services.ResolvedModel{}, services.ErrEmbeddingModelNotSet)

	res, err := svc.Reindex(context.Background(), models.Job{})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"skipped": "no embedding model"}, res)
}

func TestReindexCompletesWithoutContinuation(t *testing.T) {
	t.Parallel()
	svc, d := newMaintenanceService(t)
	d.embedder.EXPECT().EmbeddingModel(gomock.Any()).Return(services.ResolvedModel{Name: "m2"}, nil)
	d.store.EXPECT().ResetStaleItemEmbeddings(gomock.Any(), "m2").Return(5, nil)
	d.queue.EXPECT().Enqueue(gomock.Any(), services.JobEmbedItems, gomock.Any(), workers.EnqueueOptions{DedupeKey: "items.embed"}).Return(uuid.New(), nil)
	d.store.EXPECT().StaleIdeas(gomock.Any(), "m2", 32).Return([]repositories.IdeaToEmbed{{ID: uuid.New(), Version: 2, Title: "T", Summary: "S"}}, nil)
	d.embedder.EXPECT().Embed(gomock.Any(), models.PurposeEmbedding, gomock.Len(1)).Return([][]float32{{1}}, "m2", nil)
	d.store.EXPECT().SetIdeaEmbeddings(gomock.Any(), gomock.Len(1), []int{2}, [][]float32{{1}}, "m2").Return(nil)
	d.store.EXPECT().StaleIdeas(gomock.Any(), "m2", 32).Return(nil, nil)
	userID := uuid.New()
	d.store.EXPECT().StaleTopics(gomock.Any(), "m2", 64).Return([]repositories.TopicsToEmbed{{UserID: userID, Topics: []string{"go"}}}, nil)
	d.embedder.EXPECT().Embed(gomock.Any(), models.PurposeEmbedding, []string{services.TopicsText([]string{"go"})}).Return([][]float32{{2}}, "m2", nil)
	d.store.EXPECT().SetTopicsEmbeddings(gomock.Any(), []uuid.UUID{userID}, [][]string{{"go"}}, [][]float32{{2}}, "m2").Return(nil)
	d.store.EXPECT().StaleTopics(gomock.Any(), "m2", 64).Return(nil, nil)

	res, err := svc.Reindex(context.Background(), models.Job{})
	require.NoError(t, err)
	assert.Equal(t, map[string]int{"items_reset": 5, "ideas": 1, "topics": 1}, res)
}

func TestReindexContinuesInAnotherJobAfterTheCap(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		dedupeKey *string
		wantKey   string
	}{
		"plain job continues under the alternate key": {ptr("embeddings.reindex"), "embeddings.reindex:next"},
		"alternate job goes back to the plain key":    {ptr("embeddings.reindex:next"), "embeddings.reindex"},
		"job without a key uses the plain key":        {nil, "embeddings.reindex"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			svc, d := newMaintenanceService(t)
			batch := make([]repositories.IdeaToEmbed, 32)
			vectors := make([][]float32, 32)
			for i := range batch {
				batch[i] = repositories.IdeaToEmbed{ID: uuid.New(), Version: 1, Title: "T", Summary: "S"}
				vectors[i] = []float32{1}
			}
			d.embedder.EXPECT().EmbeddingModel(gomock.Any()).Return(services.ResolvedModel{Name: "m2"}, nil)
			d.store.EXPECT().ResetStaleItemEmbeddings(gomock.Any(), "m2").Return(0, nil)
			d.queue.EXPECT().Enqueue(gomock.Any(), services.JobEmbedItems, gomock.Any(), gomock.Any()).Return(uuid.New(), nil)
			d.store.EXPECT().StaleIdeas(gomock.Any(), "m2", 32).Return(batch, nil).Times(50)
			d.embedder.EXPECT().Embed(gomock.Any(), models.PurposeEmbedding, gomock.Len(32)).Return(vectors, "m2", nil).Times(50)
			d.store.EXPECT().SetIdeaEmbeddings(gomock.Any(), gomock.Len(32), gomock.Any(), vectors, "m2").Return(nil).Times(50)
			d.store.EXPECT().StaleTopics(gomock.Any(), "m2", 64).Return(nil, nil)
			d.store.EXPECT().HasStaleIdeas(gomock.Any(), "m2").Return(true, nil)
			d.queue.EXPECT().Enqueue(gomock.Any(), services.JobReindexEmbeddings, gomock.Any(), workers.EnqueueOptions{DedupeKey: tc.wantKey}).Return(uuid.New(), nil)

			res, err := svc.Reindex(context.Background(), models.Job{DedupeKey: tc.dedupeKey})
			require.NoError(t, err)
			assert.Equal(t, map[string]int{"items_reset": 0, "ideas": 50 * 32, "topics": 0}, res)
		})
	}
}

func TestReindexCapWithNothingLeftDoesNotContinue(t *testing.T) {
	t.Parallel()
	svc, d := newMaintenanceService(t)
	batch := make([]repositories.IdeaToEmbed, 32)
	vectors := make([][]float32, 32)
	for i := range batch {
		batch[i] = repositories.IdeaToEmbed{ID: uuid.New(), Version: 1}
		vectors[i] = []float32{1}
	}
	d.embedder.EXPECT().EmbeddingModel(gomock.Any()).Return(services.ResolvedModel{Name: "m2"}, nil)
	d.store.EXPECT().ResetStaleItemEmbeddings(gomock.Any(), "m2").Return(0, nil)
	d.queue.EXPECT().Enqueue(gomock.Any(), services.JobEmbedItems, gomock.Any(), gomock.Any()).Return(uuid.New(), nil)
	d.store.EXPECT().StaleIdeas(gomock.Any(), "m2", 32).Return(batch, nil).Times(50)
	d.embedder.EXPECT().Embed(gomock.Any(), models.PurposeEmbedding, gomock.Any()).Return(vectors, "m2", nil).Times(50)
	d.store.EXPECT().SetIdeaEmbeddings(gomock.Any(), gomock.Any(), gomock.Any(), vectors, "m2").Return(nil).Times(50)
	d.store.EXPECT().StaleTopics(gomock.Any(), "m2", 64).Return(nil, nil)
	d.store.EXPECT().HasStaleIdeas(gomock.Any(), "m2").Return(false, nil)
	d.store.EXPECT().HasStaleTopics(gomock.Any(), "m2").Return(false, nil)

	_, err := svc.Reindex(context.Background(), models.Job{})
	require.NoError(t, err)
}

func TestRetentionDeletesInBatchesUntilAShortOne(t *testing.T) {
	t.Parallel()
	svc, d := newMaintenanceService(t)
	d.settings.EXPECT().Get(gomock.Any()).Return(models.InstanceSettings{RetentionDays: 30}, nil)
	cutoff := fixedNow.AddDate(0, 0, -30)
	gomock.InOrder(
		d.store.EXPECT().DeleteExpiredItems(gomock.Any(), cutoff, 1000).Return(1000, nil),
		d.store.EXPECT().DeleteExpiredItems(gomock.Any(), cutoff, 1000).Return(250, nil),
	)

	res, err := svc.Retention(context.Background(), models.Job{})
	require.NoError(t, err)
	assert.Equal(t, map[string]int{"deleted": 1250}, res)
}

func TestRetentionStopsAtTheBatchCap(t *testing.T) {
	t.Parallel()
	svc, d := newMaintenanceService(t)
	d.settings.EXPECT().Get(gomock.Any()).Return(models.InstanceSettings{RetentionDays: 90}, nil)
	d.store.EXPECT().DeleteExpiredItems(gomock.Any(), fixedNow.AddDate(0, 0, -90), 1000).Return(1000, nil).Times(100)

	res, err := svc.Retention(context.Background(), models.Job{})
	require.NoError(t, err)
	assert.Equal(t, map[string]int{"deleted": 100000}, res)
}

func TestEnqueueRetentionDedupes(t *testing.T) {
	t.Parallel()
	svc, d := newMaintenanceService(t)
	d.queue.EXPECT().Enqueue(gomock.Any(), services.JobRetention, gomock.Any(), workers.EnqueueOptions{DedupeKey: "items.retention"}).Return(uuid.New(), nil)

	require.NoError(t, svc.EnqueueRetention(context.Background(), fixedNow))
}
