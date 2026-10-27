package dtos

import (
	"encoding/json"
	"time"

	"github.com/dev-fritz/newsp/api/internal/models"
)

// ItemSourceResponse identifies where an item came from.
type ItemSourceResponse struct {
	ID   string            `json:"id"`
	Kind models.SourceKind `json:"kind" enums:"hackernews,tabnews,reddit,youtube,rss,lobsters,devto,mastodon,bluesky,github,producthunt"`
	Name string            `json:"name"`
}

// ItemResponse is a collected post, story, video or article.
type ItemResponse struct {
	ID          string             `json:"id"`
	Source      ItemSourceResponse `json:"source"`
	URL         string             `json:"url"`
	Title       string             `json:"title"`
	Summary     string             `json:"summary"`
	Author      string             `json:"author"`
	PublishedAt time.Time          `json:"published_at"`
	CollectedAt time.Time          `json:"collected_at"`
	Score       *int               `json:"score"`
	Comments    *int               `json:"comments"`
	Metrics     json.RawMessage    `json:"metrics" swaggertype:"object"`
	// Cited is true when a report or an idea cites the item.
	Cited bool `json:"cited"`
}

// ItemListEnvelope is a page of the item feed.
type ItemListEnvelope struct {
	Data []ItemResponse `json:"data"`
	Meta PageMeta       `json:"meta"`
}
