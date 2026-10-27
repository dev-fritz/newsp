package connectors

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"

	"github.com/dev-fritz/newsp/api/pkg/safehttp"
)

// RSSConfig points at an RSS or Atom feed.
type RSSConfig struct {
	URL   string `json:"url"`
	Limit int    `json:"limit"` // 1..200
}

// RSS reads any RSS/Atom/JSON feed (CNN, BBC, blogs). YouTube channels have
// their own connector built on the same feed parsing.
type RSS struct {
	client Doer
	now    func() time.Time
	kind   string
}

// NewRSS builds the generic feed connector.
func NewRSS(client Doer, now func() time.Time) *RSS {
	return &RSS{client: client, now: now, kind: "rss"}
}

// Secret implements Connector: the source is public.
func (*RSS) Secret(json.RawMessage) SecretMode { return SecretNone }

// Kind implements Connector.
func (r *RSS) Kind() string { return r.kind }

// Normalize implements Connector.
func (r *RSS) Normalize(raw json.RawMessage) (json.RawMessage, error) {
	var c RSSConfig
	if err := decodeConfig(raw, &c); err != nil {
		return nil, err
	}
	c.URL = strings.TrimSpace(c.URL)
	u, err := url.Parse(c.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, &ConfigError{Field: "url", Reason: "must be an absolute http(s) URL"}
	}
	if c.Limit == 0 {
		c.Limit = 50
	}
	if c.Limit < 1 || c.Limit > 200 {
		return nil, &ConfigError{Field: "limit", Reason: "must be between 1 and 200"}
	}
	return encodeConfig(c)
}

// Fetch implements Connector.
func (r *RSS) Fetch(ctx context.Context, raw json.RawMessage, _ string) ([]Item, error) {
	norm, err := r.Normalize(raw)
	if err != nil {
		return nil, err
	}
	var c RSSConfig
	_ = json.Unmarshal(norm, &c)
	return fetchFeed(ctx, r.client, c.URL, c.Limit, r.now)
}

// fetchFeed downloads and parses a feed, normalizing up to limit entries.
func fetchFeed(ctx context.Context, client Doer, feedURL string, limit int, now func() time.Time) ([]Item, error) {
	body, err := safehttp.Get(ctx, client, feedURL, 10<<20, nil)
	if err != nil {
		return nil, fmt.Errorf("fetch feed: %w", err)
	}
	feed, err := gofeed.NewParser().Parse(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse feed: %w", err)
	}

	items := make([]Item, 0, min(len(feed.Items), limit))
	for _, e := range feed.Items {
		if len(items) == limit {
			break
		}
		title := StripHTML(e.Title)
		link := strings.TrimSpace(e.Link)
		if title == "" || link == "" {
			continue
		}
		published := now().UTC()
		switch {
		case e.PublishedParsed != nil:
			published = e.PublishedParsed.UTC()
		case e.UpdatedParsed != nil:
			published = e.UpdatedParsed.UTC()
		}
		summary := e.Description
		if summary == "" {
			summary = e.Content
		}
		if summary == "" {
			summary = mediaDescription(e) // YouTube and podcast feeds
		}
		author := ""
		if len(e.Authors) > 0 && e.Authors[0] != nil {
			author = e.Authors[0].Name
		}
		items = append(items, Item{
			ExternalID:  feedItemID(e, link),
			URL:         link,
			Title:       title,
			Summary:     summarize(summary),
			Author:      author,
			PublishedAt: published,
			Metrics:     map[string]any{"feed_title": feed.Title},
		})
	}
	return items, nil
}

// feedItemID prefers the GUID; feeds without one fall back to the link. Long
// values are hashed to keep the key compact.
func feedItemID(e *gofeed.Item, link string) string {
	id := strings.TrimSpace(e.GUID)
	if id == "" {
		id = link
	}
	if len(id) > 200 {
		sum := sha256.Sum256([]byte(id))
		return hex.EncodeToString(sum[:])
	}
	return id
}

// mediaDescription reads <media:group><media:description>, used by feeds that
// carry the text only in the Media RSS extension.
func mediaDescription(e *gofeed.Item) string {
	for _, group := range e.Extensions["media"]["group"] {
		for _, d := range group.Children["description"] {
			if d.Value != "" {
				return d.Value
			}
		}
	}
	return ""
}
