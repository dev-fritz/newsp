package connectors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/dev-fritz/newsp/api/pkg/safehttp"
)

const redditBaseURL = "https://www.reddit.com"

var subredditPattern = regexp.MustCompile(`^[A-Za-z0-9_]{2,21}$`)

// redditFooter is the "submitted by /u/x [link] [comments]" line Reddit
// appends to every feed entry.
var redditFooter = regexp.MustCompile(`(?s)\s*submitted by\s+/u/\S+.*$`)

// RedditConfig selects a subreddit listing.
type RedditConfig struct {
	Subreddit string `json:"subreddit"`
	Sort      string `json:"sort"`  // top | hot | new
	Time      string `json:"time"`  // hour | day | week (only used by top)
	Limit     int    `json:"limit"` // 1..100
}

// Reddit reads posts from the public Reddit JSON listings. Reddit refuses the
// JSON API to many anonymous clients (403); the subreddit's Atom feed is then
// used instead, which has no score or comment counts.
type Reddit struct {
	client  Doer
	baseURL string
	now     func() time.Time
}

// NewReddit builds the connector. baseURL is empty in production.
func NewReddit(client Doer, baseURL string, now func() time.Time) *Reddit {
	if baseURL == "" {
		baseURL = redditBaseURL
	}
	return &Reddit{client: client, baseURL: baseURL, now: now}
}

// Secret implements Connector: the source is public.
func (*Reddit) Secret(json.RawMessage) SecretMode { return SecretNone }

// Kind implements Connector.
func (r *Reddit) Kind() string { return "reddit" }

// Normalize implements Connector.
func (r *Reddit) Normalize(raw json.RawMessage) (json.RawMessage, error) {
	var c RedditConfig
	if err := decodeConfig(raw, &c); err != nil {
		return nil, err
	}
	sub := strings.TrimSpace(c.Subreddit)
	sub = strings.TrimPrefix(sub, "/")
	sub = strings.TrimPrefix(sub, "r/")
	if !subredditPattern.MatchString(sub) {
		return nil, &ConfigError{Field: "subreddit", Reason: "must be a subreddit name such as SaaS or r/SaaS (2-21 letters, digits or underscores)"}
	}
	c.Subreddit = sub
	if c.Sort == "" {
		c.Sort = "top"
	}
	if c.Sort != "top" && c.Sort != "hot" && c.Sort != "new" {
		return nil, &ConfigError{Field: "sort", Reason: "must be top, hot or new"}
	}
	if c.Time == "" {
		c.Time = "day"
	}
	if c.Time != "hour" && c.Time != "day" && c.Time != "week" {
		return nil, &ConfigError{Field: "time", Reason: "must be hour, day or week"}
	}
	if c.Limit == 0 {
		c.Limit = 25
	}
	if c.Limit < 1 || c.Limit > 100 {
		return nil, &ConfigError{Field: "limit", Reason: "must be between 1 and 100"}
	}
	return encodeConfig(c)
}

type redditListing struct {
	Data struct {
		Children []struct {
			Kind string     `json:"kind"`
			Data redditPost `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

type redditPost struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Selftext    string  `json:"selftext"`
	URL         string  `json:"url"`
	Permalink   string  `json:"permalink"`
	Author      string  `json:"author"`
	CreatedUTC  float64 `json:"created_utc"`
	Score       int     `json:"score"`
	NumComments int     `json:"num_comments"`
	Over18      bool    `json:"over_18"`
	Stickied    bool    `json:"stickied"`
	IsSelf      bool    `json:"is_self"`
}

// Fetch implements Connector.
func (r *Reddit) Fetch(ctx context.Context, raw json.RawMessage, _ string) ([]Item, error) {
	norm, err := r.Normalize(raw)
	if err != nil {
		return nil, err
	}
	var c RedditConfig
	_ = json.Unmarshal(norm, &c)

	endpoint := fmt.Sprintf("%s/r/%s/%s.json?limit=%d", r.baseURL, c.Subreddit, c.Sort, c.Limit)
	if c.Sort == "top" {
		endpoint += "&t=" + url.QueryEscape(c.Time)
	}
	// Reddit blocks default agents, so the newsp one is sent explicitly.
	header := http.Header{"User-Agent": {safehttp.UserAgent}}
	var listing redditListing
	if err := getJSON(ctx, r.client, endpoint, header, &listing); err != nil {
		var status *safehttp.StatusError
		if errors.As(err, &status) && status.Code == http.StatusForbidden {
			return r.fetchFeed(ctx, c)
		}
		return nil, fmt.Errorf("reddit r/%s %s: %w", c.Subreddit, c.Sort, err)
	}

	items := make([]Item, 0, len(listing.Data.Children))
	for _, child := range listing.Data.Children {
		p := child.Data
		if child.Kind != "t3" || p.Over18 || p.Stickied || p.Title == "" || p.ID == "" {
			continue
		}
		discussion := redditBaseURL + p.Permalink
		link := p.URL
		if p.IsSelf || link == "" {
			link = discussion
		}
		items = append(items, Item{
			ExternalID:  p.ID,
			URL:         link,
			Title:       p.Title,
			Summary:     summarize(p.Selftext),
			Author:      p.Author,
			PublishedAt: time.Unix(int64(p.CreatedUTC), 0).UTC(),
			Score:       intPtr(p.Score),
			Comments:    intPtr(p.NumComments),
			Metrics:     map[string]any{"subreddit": c.Subreddit, "discussion_url": discussion},
		})
	}
	return items, nil
}

// fetchFeed reads the subreddit's Atom feed, the fallback when the JSON API is
// refused. Entries link to the discussion; IDs match the JSON ones.
func (r *Reddit) fetchFeed(ctx context.Context, c RedditConfig) ([]Item, error) {
	endpoint := fmt.Sprintf("%s/r/%s/%s/.rss?limit=%d", r.baseURL, c.Subreddit, c.Sort, c.Limit)
	if c.Sort == "top" {
		endpoint += "&t=" + url.QueryEscape(c.Time)
	}
	items, err := fetchFeed(ctx, r.client, endpoint, c.Limit, r.now)
	if err != nil {
		return nil, fmt.Errorf("reddit r/%s %s feed: %w", c.Subreddit, c.Sort, err)
	}
	for i := range items {
		it := &items[i]
		it.ExternalID = strings.TrimPrefix(it.ExternalID, "t3_")
		it.Author = strings.TrimPrefix(it.Author, "/u/")
		it.Summary = strings.TrimSpace(redditFooter.ReplaceAllString(it.Summary, ""))
		it.Metrics = map[string]any{"subreddit": c.Subreddit, "discussion_url": it.URL}
	}
	return items, nil
}
