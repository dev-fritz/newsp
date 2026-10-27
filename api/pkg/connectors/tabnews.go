package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

const tabNewsBaseURL = "https://www.tabnews.com.br/api/v1"

// tabNewsSiteURL is where discussion pages live.
const tabNewsSiteURL = "https://www.tabnews.com.br"

// TabNewsConfig selects a TabNews listing.
type TabNewsConfig struct {
	Strategy string `json:"strategy"` // relevant | new | old
	Limit    int    `json:"limit"`    // 1..100
}

// TabNews reads contents from the public TabNews API.
type TabNews struct {
	client  Doer
	baseURL string
}

// NewTabNews builds the connector. baseURL is empty in production.
func NewTabNews(client Doer, baseURL string) *TabNews {
	if baseURL == "" {
		baseURL = tabNewsBaseURL
	}
	return &TabNews{client: client, baseURL: baseURL}
}

// Secret implements Connector: the source is public.
func (*TabNews) Secret(json.RawMessage) SecretMode { return SecretNone }

// Kind implements Connector.
func (t *TabNews) Kind() string { return "tabnews" }

// Normalize implements Connector.
func (t *TabNews) Normalize(raw json.RawMessage) (json.RawMessage, error) {
	var c TabNewsConfig
	if err := decodeConfig(raw, &c); err != nil {
		return nil, err
	}
	if c.Strategy == "" {
		c.Strategy = "relevant"
	}
	if c.Strategy != "relevant" && c.Strategy != "new" && c.Strategy != "old" {
		return nil, &ConfigError{Field: "strategy", Reason: "must be relevant, new or old"}
	}
	if c.Limit == 0 {
		c.Limit = 30
	}
	if c.Limit < 1 || c.Limit > 100 {
		return nil, &ConfigError{Field: "limit", Reason: "must be between 1 and 100"}
	}
	return encodeConfig(c)
}

type tabNewsContent struct {
	ID                string `json:"id"`
	OwnerUsername     string `json:"owner_username"`
	Slug              string `json:"slug"`
	Title             string `json:"title"`
	Body              string `json:"body"`
	SourceURL         string `json:"source_url"`
	PublishedAt       string `json:"published_at"`
	TabCoins          int    `json:"tabcoins"`
	ChildrenDeepCount int    `json:"children_deep_count"`
}

// Fetch implements Connector.
func (t *TabNews) Fetch(ctx context.Context, raw json.RawMessage, _ string) ([]Item, error) {
	norm, err := t.Normalize(raw)
	if err != nil {
		return nil, err
	}
	var c TabNewsConfig
	_ = json.Unmarshal(norm, &c)

	endpoint := fmt.Sprintf("%s/contents?page=1&per_page=%d&strategy=%s", t.baseURL, c.Limit, url.QueryEscape(c.Strategy))
	var contents []tabNewsContent
	if err := getJSON(ctx, t.client, endpoint, nil, &contents); err != nil {
		return nil, fmt.Errorf("tabnews %s contents: %w", c.Strategy, err)
	}

	items := make([]Item, 0, len(contents))
	for _, ct := range contents {
		if ct.Title == "" || ct.Slug == "" {
			continue
		}
		published, err := time.Parse(time.RFC3339, ct.PublishedAt)
		if err != nil {
			continue // drafts and malformed entries have no usable date
		}
		item := Item{
			ExternalID:  ct.ID,
			URL:         fmt.Sprintf("%s/%s/%s", tabNewsSiteURL, ct.OwnerUsername, ct.Slug),
			Title:       ct.Title,
			Summary:     summarize(ct.Body),
			Author:      ct.OwnerUsername,
			PublishedAt: published.UTC(),
			Score:       intPtr(ct.TabCoins),
			Comments:    intPtr(ct.ChildrenDeepCount),
		}
		if ct.SourceURL != "" {
			item.Metrics = map[string]any{"source_url": ct.SourceURL}
		}
		items = append(items, item)
	}
	return items, nil
}
