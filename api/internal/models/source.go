package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// SourceKind identifies a connector.
type SourceKind string

// Supported connectors.
const (
	SourceHackerNews  SourceKind = "hackernews"
	SourceTabNews     SourceKind = "tabnews"
	SourceReddit      SourceKind = "reddit"
	SourceYouTube     SourceKind = "youtube"
	SourceRSS         SourceKind = "rss"
	SourceLobsters    SourceKind = "lobsters"
	SourceDevTo       SourceKind = "devto"
	SourceMastodon    SourceKind = "mastodon"
	SourceBluesky     SourceKind = "bluesky"
	SourceGitHub      SourceKind = "github"
	SourceProductHunt SourceKind = "producthunt"
)

// Source is a configured feed collected on an interval.
type Source struct {
	ID                  uuid.UUID
	Kind                SourceKind
	Name                string
	Config              json.RawMessage // validated by the connector of Kind
	SecretEnc           []byte          // sealed credential; nil when the source has none
	IntervalMinutes     int
	Enabled             bool
	NextRunAt           time.Time
	LastRunAt           *time.Time
	LastSuccessAt       *time.Time
	LastError           *string
	ConsecutiveFailures int
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// Item is one collected post, story, video or article.
type Item struct {
	ID               uuid.UUID
	SourceID         uuid.UUID
	ExternalID       string
	URL              string
	Title            string
	Summary          string
	Author           string
	PublishedAt      time.Time
	CollectedAt      time.Time
	Score            *int
	Comments         *int
	Metrics          json.RawMessage
	ContentText      *string
	ContentFetchedAt *time.Time
	ContentError     *string
	Embedding        []float32
	EmbeddingModel   *string
	EmbeddedAt       *time.Time
}
