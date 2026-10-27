package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/dev-fritz/newsp/api/internal/apperrors"
	"github.com/dev-fritz/newsp/api/internal/auth"
	"github.com/dev-fritz/newsp/api/internal/models"
	"github.com/dev-fritz/newsp/api/internal/repositories"
	"github.com/dev-fritz/newsp/api/internal/workers"
	"github.com/dev-fritz/newsp/api/pkg/connectors"
)

// Job kinds of the collection pipeline.
const (
	// JobCollectSource fetches one source; payload {"source_id": "<uuid>"}.
	JobCollectSource = "source.collect"
	// JobEmbedItems embeds the items that have no vector yet.
	JobEmbedItems = "items.embed"
)

const (
	defaultIntervalMinutes = 30
	// maxFailureBackoff caps the wait before retrying a failing source.
	maxFailureBackoff = 6 * time.Hour
	// maxBackoffShift keeps interval << shift far from overflowing.
	maxBackoffShift = 10
	// maxRunErrorRunes caps the error stored on a source.
	maxRunErrorRunes = 500
	// dueBatch bounds how many due sources one scheduler tick enqueues.
	dueBatch = 100
	// embedMaxBatches bounds the work of one items.embed run.
	embedMaxBatches = 20
	// statsWindow is the period of the items_24h counter.
	statsWindow = 24 * time.Hour
)

var errSourceNotFound = apperrors.NotFound("SOURCE_NOT_FOUND", "source not found")

// errSourceSecret is recorded on a source whose credential cannot be opened
// (ENCRYPTION_KEY changed); it never carries the cause.
var errSourceSecret = errors.New("the source credential cannot be decrypted; set it again")

// SourceView is a source with its items_24h counter.
type SourceView = repositories.SourceStats

// SourceInput creates a source. Nil IntervalMinutes means 30 and nil Enabled
// means true. Secret is the credential of kinds that take one.
type SourceInput struct {
	Kind            string
	Name            string
	Config          json.RawMessage
	Secret          *string
	IntervalMinutes *int
	Enabled         *bool
}

// SourceChange updates a source; nil fields stay as they are. An empty Secret
// removes the credential.
type SourceChange struct {
	Name            *string
	Config          json.RawMessage
	Secret          *string
	IntervalMinutes *int
	Enabled         *bool
}

type collectPayload struct {
	SourceID uuid.UUID `json:"source_id"`
}

// SourceService administers sources and runs the collection and embedding jobs.
type SourceService struct {
	sources  SourceStore
	items    ItemStore
	registry ConnectorRegistry
	embedder Embedder
	queue    Enqueuer
	box      SecretBox
	now      func() time.Time
}

// NewSourceService builds a SourceService.
func NewSourceService(sources SourceStore, items ItemStore, registry ConnectorRegistry, embedder Embedder, queue Enqueuer,
	box SecretBox, now func() time.Time,
) *SourceService {
	return &SourceService{sources: sources, items: items, registry: registry, embedder: embedder, queue: queue, box: box, now: now}
}

// List returns every source with the items collected in the last 24 hours.
func (s *SourceService) List(ctx context.Context, actor auth.Identity) ([]SourceView, error) {
	if !auth.Can(actor, auth.PermSourcesManage) {
		return nil, errForbidden
	}
	views, err := s.sources.ListWithStats(ctx, s.now().Add(-statsWindow))
	if err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}
	return views, nil
}

// ListBasic returns every source, for the filters any signed-in user sees.
func (s *SourceService) ListBasic(ctx context.Context) ([]models.Source, error) {
	list, err := s.sources.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}
	return list, nil
}

// Create validates the config with the connector of kind and stores the source.
func (s *SourceService) Create(ctx context.Context, actor auth.Identity, in SourceInput) (SourceView, error) {
	if !auth.Can(actor, auth.PermSourcesManage) {
		return SourceView{}, errForbidden
	}
	connector, ok := s.registry.Get(in.Kind)
	if !ok {
		return SourceView{}, apperrors.Validation("invalid request", map[string]string{"kind": "unsupported"})
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return SourceView{}, apperrors.Validation("invalid request", map[string]string{"name": "required"})
	}
	config, err := normalizeConfig(connector, in.Config)
	if err != nil {
		return SourceView{}, err
	}
	secret, err := s.applySecret(connector, config, nil, in.Secret)
	if err != nil {
		return SourceView{}, err
	}

	now := s.now()
	src := models.Source{
		ID: uuid.New(), Kind: models.SourceKind(in.Kind), Name: name, Config: config, SecretEnc: secret,
		IntervalMinutes: defaultIntervalMinutes, Enabled: true, NextRunAt: now, CreatedAt: now, UpdatedAt: now,
	}
	if in.IntervalMinutes != nil {
		src.IntervalMinutes = *in.IntervalMinutes
	}
	if in.Enabled != nil {
		src.Enabled = *in.Enabled
	}
	if err := s.sources.Create(ctx, src); err != nil {
		return SourceView{}, fmt.Errorf("create source: %w", err)
	}
	return SourceView{Source: src}, nil
}

// Update changes a source. Changing the config re-validates it, and enabling a
// disabled source makes it run soon.
func (s *SourceService) Update(ctx context.Context, actor auth.Identity, id uuid.UUID, change SourceChange) (SourceView, error) {
	if !auth.Can(actor, auth.PermSourcesManage) {
		return SourceView{}, errForbidden
	}
	src, err := s.sources.GetByID(ctx, id)
	if errors.Is(err, repositories.ErrNotFound) {
		return SourceView{}, errSourceNotFound
	}
	if err != nil {
		return SourceView{}, fmt.Errorf("update source: %w", err)
	}

	now := s.now()
	if change.Name != nil {
		src.Name = strings.TrimSpace(*change.Name)
		if src.Name == "" {
			return SourceView{}, apperrors.Validation("invalid request", map[string]string{"name": "required"})
		}
	}
	if change.Config != nil || change.Secret != nil {
		connector, ok := s.registry.Get(string(src.Kind))
		if !ok {
			return SourceView{}, fmt.Errorf("update source: no connector for kind %q", src.Kind)
		}
		if change.Config != nil {
			if src.Config, err = normalizeConfig(connector, change.Config); err != nil {
				return SourceView{}, err
			}
		}
		if src.SecretEnc, err = s.applySecret(connector, src.Config, src.SecretEnc, change.Secret); err != nil {
			return SourceView{}, err
		}
	}
	if change.IntervalMinutes != nil {
		src.IntervalMinutes = *change.IntervalMinutes
	}
	if change.Enabled != nil {
		if *change.Enabled && !src.Enabled {
			src.NextRunAt = now
		}
		src.Enabled = *change.Enabled
	}
	src.UpdatedAt = now
	if err := s.sources.Update(ctx, src); err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return SourceView{}, errSourceNotFound
		}
		return SourceView{}, fmt.Errorf("update source: %w", err)
	}
	view, err := s.sources.GetWithStats(ctx, id, now.Add(-statsWindow))
	if err != nil {
		return SourceView{}, fmt.Errorf("update source: %w", err)
	}
	return view, nil
}

// Delete removes a source and its items, unless reports or ideas cite them.
func (s *SourceService) Delete(ctx context.Context, actor auth.Identity, id uuid.UUID) error {
	if !auth.Can(actor, auth.PermSourcesManage) {
		return errForbidden
	}
	err := s.sources.Delete(ctx, id)
	switch {
	case errors.Is(err, repositories.ErrNotFound):
		return errSourceNotFound
	case errors.Is(err, repositories.ErrReferenced):
		return apperrors.Conflict("SOURCE_HAS_CITED_ITEMS", "reports or ideas cite items of this source; disable it instead")
	case err != nil:
		return fmt.Errorf("delete source: %w", err)
	}
	return nil
}

// CollectNow queues a collection of the source ahead of its schedule and
// returns the job to follow. A collection already queued is reused.
func (s *SourceService) CollectNow(ctx context.Context, actor auth.Identity, id uuid.UUID) (uuid.UUID, error) {
	if !auth.Can(actor, auth.PermSourcesManage) {
		return uuid.Nil, errForbidden
	}
	if _, err := s.sources.GetByID(ctx, id); err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return uuid.Nil, errSourceNotFound
		}
		return uuid.Nil, fmt.Errorf("collect source: %w", err)
	}
	jobID, err := s.enqueueCollect(ctx, id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("collect source: %w", err)
	}
	return jobID, nil
}

func (s *SourceService) enqueueCollect(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	return s.queue.Enqueue(ctx, JobCollectSource, collectPayload{SourceID: id}, workers.EnqueueOptions{DedupeKey: JobCollectSource + ":" + id.String()})
}

// EnqueueDue queues a collection for every enabled source that is due. Dedupe
// keys make it safe to run on several worker replicas at once.
func (s *SourceService) EnqueueDue(ctx context.Context, now time.Time) error {
	ids, err := s.sources.ListDue(ctx, now, dueBatch)
	if err != nil {
		return fmt.Errorf("enqueue due sources: %w", err)
	}
	var errs []error
	for _, id := range ids {
		if _, err := s.enqueueCollect(ctx, id); err != nil {
			errs = append(errs, fmt.Errorf("source %s: %w", id, err))
		}
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("enqueue due sources: %w", err)
	}
	return nil
}

// CollectSource is the handler of JobCollectSource. A connector failure is
// recorded on the source and its own schedule retries it, so the job succeeds;
// database errors and a canceled or expired job context fail the job so the
// queue retries it.
func (s *SourceService) CollectSource(ctx context.Context, job models.Job) (any, error) {
	var payload collectPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil || payload.SourceID == uuid.Nil {
		return nil, workers.Permanent(fmt.Errorf("invalid %s payload", JobCollectSource))
	}
	src, err := s.sources.GetByID(ctx, payload.SourceID)
	if errors.Is(err, repositories.ErrNotFound) {
		return nil, workers.Permanent(fmt.Errorf("source %s not found", payload.SourceID))
	}
	if err != nil {
		return nil, fmt.Errorf("collect source: %w", err)
	}
	if !src.Enabled {
		return map[string]string{"skipped": "disabled"}, nil
	}
	connector, ok := s.registry.Get(string(src.Kind))
	if !ok {
		return nil, workers.Permanent(fmt.Errorf("no connector for kind %q", src.Kind))
	}

	secret, fetchErr := s.openSecret(src.SecretEnc)
	var fetched []connectors.Item
	if fetchErr == nil {
		fetched, fetchErr = connector.Fetch(ctx, src.Config, secret)
	}
	if fetchErr != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("collect source %s: %w", src.ID, fetchErr)
		}
		msg := runErrorMessage(fetchErr)
		now := s.now()
		next := nextRunAfterFailure(now, time.Duration(src.IntervalMinutes)*time.Minute, src.ConsecutiveFailures+1)
		if err := s.sources.MarkFailure(ctx, src.ID, now, msg, next); err != nil {
			return nil, sourceWriteError(src.ID, err)
		}
		return map[string]any{"fetched": 0, "new": 0, "error": msg}, nil
	}

	now := s.now()
	added, err := s.items.Upsert(ctx, src.ID, toModelItems(fetched, now))
	if err != nil {
		return nil, sourceWriteError(src.ID, err)
	}
	next := now.Add(time.Duration(src.IntervalMinutes) * time.Minute)
	if err := s.sources.MarkSuccess(ctx, src.ID, now, next); err != nil {
		return nil, sourceWriteError(src.ID, err)
	}
	if added > 0 {
		if _, err := s.queue.Enqueue(ctx, JobEmbedItems, struct{}{}, workers.EnqueueOptions{DedupeKey: JobEmbedItems}); err != nil {
			return nil, fmt.Errorf("enqueue %s: %w", JobEmbedItems, err)
		}
	}
	return map[string]int{"fetched": len(fetched), "new": added}, nil
}

// sourceWriteError classifies a failed write after a collection: a source
// deleted mid-run is final, anything else is worth a retry.
func sourceWriteError(id uuid.UUID, err error) error {
	if errors.Is(err, repositories.ErrNotFound) || errors.Is(err, repositories.ErrReferenced) {
		return workers.Permanent(fmt.Errorf("source %s was deleted during collection", id))
	}
	return fmt.Errorf("save collection of source %s: %w", id, err)
}

// EmbedPending is the handler of JobEmbedItems: it embeds items without a
// vector, 64 per request and at most 20 requests per run, and queues another
// run when items remain. Without an embedding model it does nothing.
func (s *SourceService) EmbedPending(ctx context.Context, job models.Job) (any, error) {
	embedded := 0
	for batch := range embedMaxBatches {
		pending, err := s.items.Pending(ctx, embedBatchSize)
		if err != nil {
			return nil, fmt.Errorf("embed items: %w", err)
		}
		if len(pending) == 0 {
			return map[string]int{"embedded": embedded}, nil
		}
		ids := make([]uuid.UUID, len(pending))
		texts := make([]string, len(pending))
		for i, p := range pending {
			ids[i], texts[i] = p.ID, p.Title+"\n\n"+p.Summary
		}
		vectors, model, err := s.embedder.Embed(ctx, models.PurposeEmbedding, texts)
		if errors.Is(err, ErrEmbeddingModelNotSet) {
			return map[string]string{"skipped": "no embedding model"}, nil
		}
		if err != nil {
			return nil, fmt.Errorf("embed items: %w", err)
		}
		if len(vectors) != len(ids) {
			return nil, fmt.Errorf("embed items: got %d vectors for %d texts", len(vectors), len(ids))
		}
		if err := s.items.SetEmbeddings(ctx, ids, vectors, model, s.now()); err != nil {
			return nil, fmt.Errorf("embed items: %w", err)
		}
		embedded += len(ids)
		if batch < embedMaxBatches-1 {
			continue
		}
		// The cap was reached: continue in a fresh job when items remain.
		more, err := s.items.HasPending(ctx)
		if err != nil {
			return nil, fmt.Errorf("embed items: %w", err)
		}
		if more {
			if _, err := s.queue.Enqueue(ctx, JobEmbedItems, struct{}{}, workers.EnqueueOptions{DedupeKey: embedContinuationKey(job)}); err != nil {
				return nil, fmt.Errorf("enqueue %s: %w", JobEmbedItems, err)
			}
		}
	}
	return map[string]int{"embedded": embedded}, nil
}

// embedContinuationKey is the dedupe key of the follow-up run. The running job
// still holds its own key, so reusing it would drop the follow-up; the two
// keys alternate instead.
func embedContinuationKey(job models.Job) string {
	if job.DedupeKey != nil && *job.DedupeKey == JobEmbedItems {
		return JobEmbedItems + ":next"
	}
	return JobEmbedItems
}

// nextRunAfterFailure schedules the retry of a source that has now failed
// failures times in a row: interval doubled per failure, capped at 6 hours.
func nextRunAfterFailure(now time.Time, interval time.Duration, failures int) time.Time {
	wait := interval << min(max(failures, 0), maxBackoffShift)
	return now.Add(min(wait, maxFailureBackoff))
}

// runErrorMessage turns a collection failure into a short one-line message.
func runErrorMessage(err error) string {
	msg := strings.Join(strings.Fields(err.Error()), " ")
	if utf8.RuneCountInString(msg) <= maxRunErrorRunes {
		return msg
	}
	return string([]rune(msg)[:maxRunErrorRunes])
}

// applySecret returns the sealed credential a source keeps after a change:
// nil change keeps current, an empty one removes it and any other value
// replaces it. The result must suit what the connector expects for config.
func (s *SourceService) applySecret(c connectors.Connector, config json.RawMessage, current []byte, change *string) ([]byte, error) {
	mode := c.Secret(config)
	sealed := current
	if change != nil {
		sealed = nil
		if plain := strings.TrimSpace(*change); plain != "" {
			if mode == connectors.SecretNone {
				return nil, apperrors.Validation("invalid request", map[string]string{"secret": "unsupported"})
			}
			var err error
			if sealed, err = s.box.Seal([]byte(plain)); err != nil {
				return nil, fmt.Errorf("encrypt source secret: %w", err)
			}
		}
	}
	switch {
	case mode == connectors.SecretRequired && len(sealed) == 0:
		return nil, apperrors.Validation("invalid request", map[string]string{"secret": "required"})
	case mode == connectors.SecretNone:
		return nil, nil // a config change may make a kept credential useless
	}
	return sealed, nil
}

// openSecret decrypts a source credential; no credential is an empty string.
func (s *SourceService) openSecret(sealed []byte) (string, error) {
	if len(sealed) == 0 {
		return "", nil
	}
	plain, err := s.box.Open(sealed)
	if err != nil {
		return "", errSourceSecret
	}
	return string(plain), nil
}

// normalizeConfig checks that raw is a JSON object and lets the connector
// validate it; a bad config becomes a validation error on config.<field>.
func normalizeConfig(c connectors.Connector, raw json.RawMessage) (json.RawMessage, error) {
	if !bytes.HasPrefix(bytes.TrimSpace(raw), []byte("{")) {
		return nil, apperrors.Validation("invalid source config", map[string]string{"config": "object"})
	}
	config, err := c.Normalize(raw)
	if err == nil {
		return config, nil
	}
	var ce *connectors.ConfigError
	if errors.As(err, &ce) {
		field := "config"
		if ce.Field != "config" {
			field += "." + ce.Field
		}
		return nil, apperrors.Validation("invalid source config", map[string]string{field: ce.Reason})
	}
	return nil, fmt.Errorf("normalize config: %w", err)
}

// toModelItems maps fetched items, dropping those the database could not
// identify, and stamps the collection time.
func toModelItems(in []connectors.Item, now time.Time) []models.Item {
	out := make([]models.Item, 0, len(in))
	for _, it := range in {
		if it.ExternalID == "" || it.URL == "" {
			continue
		}
		published := it.PublishedAt
		if published.IsZero() {
			published = now
		}
		var metrics json.RawMessage
		if len(it.Metrics) > 0 {
			metrics, _ = json.Marshal(it.Metrics) // a map of plain values always encodes; failure falls back to {}
		}
		out = append(out, models.Item{
			ExternalID: stripNUL(it.ExternalID), URL: stripNUL(it.URL), Title: stripNUL(it.Title),
			Summary: stripNUL(it.Summary), Author: stripNUL(it.Author), PublishedAt: published, CollectedAt: now,
			Score: it.Score, Comments: it.Comments, Metrics: metrics,
		})
	}
	return out
}

// stripNUL removes the byte Postgres refuses in text.
func stripNUL(s string) string { return strings.ReplaceAll(s, "\x00", "") }
