package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const youTubeBaseURL = "https://www.youtube.com"

var youTubeChannelPattern = regexp.MustCompile(`^UC[A-Za-z0-9_-]{22}$`)

// YouTubeConfig selects a channel.
type YouTubeConfig struct {
	ChannelID string `json:"channel_id"`
	Limit     int    `json:"limit"` // 1..50
}

// YouTube reads a channel's public RSS feed.
type YouTube struct {
	client  Doer
	baseURL string
	now     func() time.Time
}

// NewYouTube builds the connector. baseURL is empty in production.
func NewYouTube(client Doer, baseURL string, now func() time.Time) *YouTube {
	if baseURL == "" {
		baseURL = youTubeBaseURL
	}
	return &YouTube{client: client, baseURL: baseURL, now: now}
}

// Secret implements Connector: the source is public.
func (*YouTube) Secret(json.RawMessage) SecretMode { return SecretNone }

// Kind implements Connector.
func (y *YouTube) Kind() string { return "youtube" }

// Normalize implements Connector.
func (y *YouTube) Normalize(raw json.RawMessage) (json.RawMessage, error) {
	var c YouTubeConfig
	if err := decodeConfig(raw, &c); err != nil {
		return nil, err
	}
	c.ChannelID = strings.TrimSpace(c.ChannelID)
	if !youTubeChannelPattern.MatchString(c.ChannelID) {
		return nil, &ConfigError{Field: "channel_id", Reason: "must be a channel ID starting with UC (24 characters)"}
	}
	if c.Limit == 0 {
		c.Limit = 15
	}
	if c.Limit < 1 || c.Limit > 50 {
		return nil, &ConfigError{Field: "limit", Reason: "must be between 1 and 50"}
	}
	return encodeConfig(c)
}

// Fetch implements Connector.
func (y *YouTube) Fetch(ctx context.Context, raw json.RawMessage, _ string) ([]Item, error) {
	norm, err := y.Normalize(raw)
	if err != nil {
		return nil, err
	}
	var c YouTubeConfig
	_ = json.Unmarshal(norm, &c)

	feedURL := fmt.Sprintf("%s/feeds/videos.xml?channel_id=%s", y.baseURL, url.QueryEscape(c.ChannelID))
	items, err := fetchFeed(ctx, y.client, feedURL, c.Limit, y.now)
	if err != nil {
		return nil, fmt.Errorf("youtube channel %s: %w", c.ChannelID, err)
	}
	for i := range items {
		items[i].Metrics["channel_id"] = c.ChannelID
	}
	return items, nil
}
