package dtos

import (
	"encoding/json"
	"time"

	"github.com/dev-fritz/newsp/api/internal/models"
)

// CreateSourceRequest registers a source. The config depends on the kind and
// is validated by its connector.
type CreateSourceRequest struct {
	Kind   string          `json:"kind" validate:"required" enums:"hackernews,tabnews,reddit,youtube,rss,lobsters,devto,mastodon,bluesky,github,producthunt" example:"rss"`
	Name   string          `json:"name" validate:"required,max=120" example:"BBC Technology"`
	Config json.RawMessage `json:"config" validate:"required" swaggertype:"object"`
	// Secret is the credential of kinds that take one (API token, app password).
	// It is encrypted at rest and never returned.
	Secret          *string `json:"secret" validate:"omitempty,max=4096"`
	IntervalMinutes *int    `json:"interval_minutes" validate:"omitempty,min=5,max=1440" example:"30"`
	Enabled         *bool   `json:"enabled"`
}

// UpdateSourceRequest changes a source; omitted fields stay as they are.
type UpdateSourceRequest struct {
	Name   *string         `json:"name" validate:"omitempty,min=1,max=120"`
	Config json.RawMessage `json:"config" swaggertype:"object"`
	// Secret replaces the credential; an empty string removes it.
	Secret          *string `json:"secret" validate:"omitempty,max=4096"`
	IntervalMinutes *int    `json:"interval_minutes" validate:"omitempty,min=5,max=1440"`
	Enabled         *bool   `json:"enabled"`
}

// SourceResponse is a source as admins see it.
type SourceResponse struct {
	ID                  string            `json:"id"`
	Kind                models.SourceKind `json:"kind" enums:"hackernews,tabnews,reddit,youtube,rss,lobsters,devto,mastodon,bluesky,github,producthunt"`
	Name                string            `json:"name"`
	Config              json.RawMessage   `json:"config" swaggertype:"object"`
	HasSecret           bool              `json:"has_secret"`
	IntervalMinutes     int               `json:"interval_minutes"`
	Enabled             bool              `json:"enabled"`
	NextRunAt           time.Time         `json:"next_run_at"`
	LastRunAt           *time.Time        `json:"last_run_at"`
	LastSuccessAt       *time.Time        `json:"last_success_at"`
	LastError           *string           `json:"last_error"`
	ConsecutiveFailures int               `json:"consecutive_failures"`
	Items24h            int               `json:"items_24h"`
	CreatedAt           time.Time         `json:"created_at"`
}

// NewSourceResponse maps a source and its items_24h counter.
func NewSourceResponse(s models.Source, items24h int) SourceResponse {
	return SourceResponse{
		ID: s.ID.String(), Kind: s.Kind, Name: s.Name, Config: s.Config, HasSecret: len(s.SecretEnc) > 0, IntervalMinutes: s.IntervalMinutes,
		Enabled: s.Enabled, NextRunAt: s.NextRunAt, LastRunAt: s.LastRunAt, LastSuccessAt: s.LastSuccessAt,
		LastError: s.LastError, ConsecutiveFailures: s.ConsecutiveFailures, Items24h: items24h, CreatedAt: s.CreatedAt,
	}
}

// SourceEnvelope wraps SourceResponse.
type SourceEnvelope struct {
	Data SourceResponse `json:"data"`
}

// SourceListEnvelope wraps the list of sources.
type SourceListEnvelope struct {
	Data []SourceResponse `json:"data"`
}

// SourceSummary is a source as any signed-in user sees it, for filters.
type SourceSummary struct {
	ID      string            `json:"id"`
	Kind    models.SourceKind `json:"kind" enums:"hackernews,tabnews,reddit,youtube,rss,lobsters,devto,mastodon,bluesky,github,producthunt"`
	Name    string            `json:"name"`
	Enabled bool              `json:"enabled"`
}

// NewSourceSummary maps a source.
func NewSourceSummary(s models.Source) SourceSummary {
	return SourceSummary{ID: s.ID.String(), Kind: s.Kind, Name: s.Name, Enabled: s.Enabled}
}

// SourceSummaryListEnvelope wraps the list of source summaries.
type SourceSummaryListEnvelope struct {
	Data []SourceSummary `json:"data"`
}
