package services

//go:generate go tool mockgen -source=maintenance_service.go -destination=../../tests/mocks/maintenance_mocks.go -package=mocks

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/dev-fritz/newsp/api/internal/models"
	"github.com/dev-fritz/newsp/api/internal/repositories"
	"github.com/dev-fritz/newsp/api/internal/workers"
)

// JobRetention deletes items older than the retention period.
const JobRetention = "items.retention"

const (
	retentionBatch      = 1000
	retentionMaxBatches = 100
	// ideaEmbedBatch is how many ideas one reindex request embeds.
	ideaEmbedBatch = 32
	// topicsEmbedBatch is how many users' topics one reindex request embeds.
	topicsEmbedBatch = 64
	// reindexMaxBatches bounds the ideas, and separately the topics, one run handles.
	reindexMaxBatches = 50
)

// MaintenanceStore runs the retention and reindexing statements.
type MaintenanceStore interface {
	DeleteExpiredItems(ctx context.Context, cutoff time.Time, limit int) (int, error)
	ResetStaleItemEmbeddings(ctx context.Context, model string) (int, error)
	StaleIdeas(ctx context.Context, model string, limit int) ([]repositories.IdeaToEmbed, error)
	HasStaleIdeas(ctx context.Context, model string) (bool, error)
	SetIdeaEmbeddings(ctx context.Context, ids []uuid.UUID, versions []int, vectors [][]float32, model string) error
	StaleTopics(ctx context.Context, model string, limit int) ([]repositories.TopicsToEmbed, error)
	HasStaleTopics(ctx context.Context, model string) (bool, error)
	SetTopicsEmbeddings(ctx context.Context, userIDs []uuid.UUID, topics [][]string, vectors [][]float32, model string) error
}

// MaintenanceSettings reads the instance settings.
type MaintenanceSettings interface {
	Get(ctx context.Context) (models.InstanceSettings, error)
}

// ReindexEmbedder resolves the instance embedding model and embeds with it.
type ReindexEmbedder interface {
	EmbeddingModel(ctx context.Context) (ResolvedModel, error)
	Embed(ctx context.Context, purpose models.UsagePurpose, texts []string) ([][]float32, string, error)
}

// MaintenanceService runs data retention and embedding reindexing.
type MaintenanceService struct {
	store    MaintenanceStore
	settings MaintenanceSettings
	embedder ReindexEmbedder
	queue    Enqueuer
	now      func() time.Time
}

// NewMaintenanceService builds a MaintenanceService.
func NewMaintenanceService(store MaintenanceStore, settings MaintenanceSettings, embedder ReindexEmbedder, queue Enqueuer, now func() time.Time) *MaintenanceService {
	return &MaintenanceService{store: store, settings: settings, embedder: embedder, queue: queue, now: now}
}

// EnqueueRetention schedules the retention job; at most one is active.
func (s *MaintenanceService) EnqueueRetention(ctx context.Context, _ time.Time) error {
	if _, err := s.queue.Enqueue(ctx, JobRetention, struct{}{}, workers.EnqueueOptions{DedupeKey: JobRetention}); err != nil {
		return fmt.Errorf("enqueue %s: %w", JobRetention, err)
	}
	return nil
}

// Retention is the handler of JobRetention: it deletes items collected before
// the retention period that no report and no idea cites, 1000 per statement
// and at most 100 statements per run (the next run continues).
func (s *MaintenanceService) Retention(ctx context.Context, _ models.Job) (any, error) {
	settings, err := s.settings.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("retention: %w", err)
	}
	cutoff := s.now().AddDate(0, 0, -settings.RetentionDays)
	deleted := 0
	for range retentionMaxBatches {
		n, err := s.store.DeleteExpiredItems(ctx, cutoff, retentionBatch)
		if err != nil {
			return nil, fmt.Errorf("retention: %w", err)
		}
		deleted += n
		if n < retentionBatch {
			break
		}
	}
	return map[string]int{"deleted": deleted}, nil
}

// Reindex is the handler of JobReindexEmbeddings: it brings every stored
// vector to the current embedding model. Items are only reset, since the
// embedding job redoes them; ideas and user topics are re-embedded here. A run
// that hits its caps with work left queues a follow-up.
func (s *MaintenanceService) Reindex(ctx context.Context, job models.Job) (any, error) {
	resolved, err := s.embedder.EmbeddingModel(ctx)
	if errors.Is(err, ErrEmbeddingModelNotSet) {
		return map[string]string{"skipped": "no embedding model"}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reindex: %w", err)
	}
	model := resolved.Name

	itemsReset, err := s.store.ResetStaleItemEmbeddings(ctx, model)
	if err != nil {
		return nil, fmt.Errorf("reindex items: %w", err)
	}
	if _, err := s.queue.Enqueue(ctx, JobEmbedItems, struct{}{}, workers.EnqueueOptions{DedupeKey: JobEmbedItems}); err != nil {
		return nil, fmt.Errorf("enqueue %s: %w", JobEmbedItems, err)
	}

	ideas, ideasCapped, err := s.reindexIdeas(ctx, model)
	if err != nil {
		return nil, err
	}
	topics, topicsCapped, err := s.reindexTopics(ctx, model)
	if err != nil {
		return nil, err
	}

	result := map[string]int{"items_reset": itemsReset, "ideas": ideas, "topics": topics}
	if !ideasCapped && !topicsCapped {
		return result, nil
	}
	more, err := s.hasStale(ctx, model)
	if err != nil {
		return nil, err
	}
	if more {
		if _, err := s.queue.Enqueue(ctx, JobReindexEmbeddings, struct{}{}, workers.EnqueueOptions{DedupeKey: reindexContinuationKey(job)}); err != nil {
			return nil, fmt.Errorf("enqueue %s: %w", JobReindexEmbeddings, err)
		}
	}
	return result, nil
}

// reindexIdeas re-embeds stale ideas; capped reports that the batch limit was
// reached, so more may remain.
func (s *MaintenanceService) reindexIdeas(ctx context.Context, model string) (done int, capped bool, err error) {
	for range reindexMaxBatches {
		batch, err := s.store.StaleIdeas(ctx, model, ideaEmbedBatch)
		if err != nil {
			return done, false, fmt.Errorf("reindex ideas: %w", err)
		}
		if len(batch) == 0 {
			return done, false, nil
		}
		ids := make([]uuid.UUID, len(batch))
		versions := make([]int, len(batch))
		texts := make([]string, len(batch))
		for i, idea := range batch {
			ids[i], versions[i] = idea.ID, idea.Version
			texts[i] = embeddingText(idea.Title, idea.Summary, idea.Content)
		}
		vectors, usedModel, err := s.embedder.Embed(ctx, models.PurposeEmbedding, texts)
		if err != nil {
			return done, false, fmt.Errorf("reindex ideas: %w", err)
		}
		if len(vectors) != len(ids) {
			return done, false, fmt.Errorf("reindex ideas: got %d vectors for %d texts", len(vectors), len(ids))
		}
		if err := s.store.SetIdeaEmbeddings(ctx, ids, versions, vectors, usedModel); err != nil {
			return done, false, fmt.Errorf("reindex ideas: %w", err)
		}
		done += len(ids)
	}
	return done, true, nil
}

// reindexTopics re-embeds the topics of users whose vector is stale.
func (s *MaintenanceService) reindexTopics(ctx context.Context, model string) (done int, capped bool, err error) {
	for range reindexMaxBatches {
		batch, err := s.store.StaleTopics(ctx, model, topicsEmbedBatch)
		if err != nil {
			return done, false, fmt.Errorf("reindex topics: %w", err)
		}
		if len(batch) == 0 {
			return done, false, nil
		}
		ids := make([]uuid.UUID, len(batch))
		topics := make([][]string, len(batch))
		texts := make([]string, len(batch))
		for i, t := range batch {
			ids[i], topics[i], texts[i] = t.UserID, t.Topics, TopicsText(t.Topics)
		}
		vectors, usedModel, err := s.embedder.Embed(ctx, models.PurposeEmbedding, texts)
		if err != nil {
			return done, false, fmt.Errorf("reindex topics: %w", err)
		}
		if len(vectors) != len(ids) {
			return done, false, fmt.Errorf("reindex topics: got %d vectors for %d texts", len(vectors), len(ids))
		}
		if err := s.store.SetTopicsEmbeddings(ctx, ids, topics, vectors, usedModel); err != nil {
			return done, false, fmt.Errorf("reindex topics: %w", err)
		}
		done += len(ids)
	}
	return done, true, nil
}

func (s *MaintenanceService) hasStale(ctx context.Context, model string) (bool, error) {
	ideas, err := s.store.HasStaleIdeas(ctx, model)
	if err != nil {
		return false, fmt.Errorf("reindex: %w", err)
	}
	if ideas {
		return true, nil
	}
	topics, err := s.store.HasStaleTopics(ctx, model)
	if err != nil {
		return false, fmt.Errorf("reindex: %w", err)
	}
	return topics, nil
}

// reindexContinuationKey is the dedupe key of the follow-up run; the running
// job still holds its own key, so the two keys alternate (see
// embedContinuationKey).
func reindexContinuationKey(job models.Job) string {
	if job.DedupeKey != nil && *job.DedupeKey == JobReindexEmbeddings {
		return JobReindexEmbeddings + ":next"
	}
	return JobReindexEmbeddings
}
