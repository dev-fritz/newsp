package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const devToBaseURL = "https://dev.to"

// devToTagPattern matches dev.to tag names.
var devToTagPattern = regexp.MustCompile(`^[a-z0-9]{1,30}$`)

// DevToConfig selects a dev.to article listing.
type DevToConfig struct {
	Tag   string `json:"tag"`   // optional
	Top   int    `json:"top"`   // 0 = newest; 1, 7 or 30 = most popular in the last N days
	Limit int    `json:"limit"` // 1..100
}

// DevTo reads articles from the public dev.to (Forem) API.
type DevTo struct {
	client  Doer
	baseURL string
}

// NewDevTo builds the connector. baseURL is empty in production.
func NewDevTo(client Doer, baseURL string) *DevTo {
	if baseURL == "" {
		baseURL = devToBaseURL
	}
	return &DevTo{client: client, baseURL: strings.TrimRight(baseURL, "/")}
}

// Kind implements Connector.
func (d *DevTo) Kind() string { return "devto" }

// Secret implements Connector: the source is public.
func (*DevTo) Secret(json.RawMessage) SecretMode { return SecretNone }

// Normalize implements Connector.
func (d *DevTo) Normalize(raw json.RawMessage) (json.RawMessage, error) {
	var c DevToConfig
	if err := decodeConfig(raw, &c); err != nil {
		return nil, err
	}
	c.Tag = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(c.Tag), "#"))
	if c.Tag != "" && !devToTagPattern.MatchString(c.Tag) {
		return nil, &ConfigError{Field: "tag", Reason: "must be 1 to 30 characters of a-z and 0-9"}
	}
	switch c.Top {
	case 0, 1, 7, 30:
	default:
		return nil, &ConfigError{Field: "top", Reason: "must be 0, 1, 7 or 30"}
	}
	if c.Limit == 0 {
		c.Limit = 30
	}
	if c.Limit < 1 || c.Limit > 100 {
		return nil, &ConfigError{Field: "limit", Reason: "must be between 1 and 100"}
	}
	return encodeConfig(c)
}

type devToArticle struct {
	ID                   int      `json:"id"`
	Title                string   `json:"title"`
	Description          string   `json:"description"`
	URL                  string   `json:"url"`
	PublishedAt          string   `json:"published_at"`
	PublicReactionsCount int      `json:"public_reactions_count"`
	CommentsCount        int      `json:"comments_count"`
	ReadingTimeMinutes   int      `json:"reading_time_minutes"`
	TagList              []string `json:"tag_list"`
	User                 struct {
		Username string `json:"username"`
	} `json:"user"`
}

// Fetch implements Connector.
func (d *DevTo) Fetch(ctx context.Context, raw json.RawMessage, _ string) ([]Item, error) {
	norm, err := d.Normalize(raw)
	if err != nil {
		return nil, err
	}
	var c DevToConfig
	_ = json.Unmarshal(norm, &c)

	q := url.Values{}
	q.Set("per_page", strconv.Itoa(c.Limit))
	if c.Tag != "" {
		q.Set("tag", c.Tag)
	}
	path := "/api/articles/latest"
	if c.Top > 0 {
		path = "/api/articles"
		q.Set("top", strconv.Itoa(c.Top))
	}
	var articles []devToArticle
	if err := getJSON(ctx, d.client, d.baseURL+path+"?"+q.Encode(), nil, &articles); err != nil {
		return nil, fmt.Errorf("devto articles: %w", err)
	}

	items := make([]Item, 0, len(articles))
	for _, a := range articles {
		if a.ID == 0 || a.Title == "" || a.URL == "" {
			continue
		}
		published, err := time.Parse(time.RFC3339, a.PublishedAt)
		if err != nil {
			continue
		}
		items = append(items, Item{
			ExternalID:  strconv.Itoa(a.ID),
			URL:         a.URL,
			Title:       a.Title,
			Summary:     summarize(a.Description),
			Author:      a.User.Username,
			PublishedAt: published.UTC(),
			Score:       intPtr(a.PublicReactionsCount),
			Comments:    intPtr(a.CommentsCount),
			Metrics:     map[string]any{"tags": a.TagList, "reading_time_minutes": a.ReadingTimeMinutes},
		})
	}
	return items, nil
}
