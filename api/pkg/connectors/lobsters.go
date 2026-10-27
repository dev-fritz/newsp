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

const lobstersBaseURL = "https://lobste.rs"

// lobstersTagPattern matches Lobsters tag names.
var lobstersTagPattern = regexp.MustCompile(`^[a-z0-9_+-]{1,25}$`)

// LobstersConfig selects a Lobsters listing.
type LobstersConfig struct {
	List  string `json:"list"`  // hottest | newest (ignored when tag is set)
	Tag   string `json:"tag"`   // optional tag page
	Limit int    `json:"limit"` // 1..25 (one page of the API)
}

// Lobsters reads stories from the public Lobsters JSON API.
type Lobsters struct {
	client  Doer
	baseURL string
}

// NewLobsters builds the connector. baseURL is empty in production.
func NewLobsters(client Doer, baseURL string) *Lobsters {
	if baseURL == "" {
		baseURL = lobstersBaseURL
	}
	return &Lobsters{client: client, baseURL: strings.TrimRight(baseURL, "/")}
}

// Kind implements Connector.
func (l *Lobsters) Kind() string { return "lobsters" }

// Secret implements Connector: the source is public.
func (*Lobsters) Secret(json.RawMessage) SecretMode { return SecretNone }

// Normalize implements Connector.
func (l *Lobsters) Normalize(raw json.RawMessage) (json.RawMessage, error) {
	var c LobstersConfig
	if err := decodeConfig(raw, &c); err != nil {
		return nil, err
	}
	if c.List == "" {
		c.List = "hottest"
	}
	if c.List != "hottest" && c.List != "newest" {
		return nil, &ConfigError{Field: "list", Reason: "must be hottest or newest"}
	}
	c.Tag = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(c.Tag), "#"))
	if c.Tag != "" && !lobstersTagPattern.MatchString(c.Tag) {
		return nil, &ConfigError{Field: "tag", Reason: "must be 1 to 25 characters of a-z, 0-9, _, + or -"}
	}
	if c.Limit == 0 {
		c.Limit = 25
	}
	if c.Limit < 1 || c.Limit > 25 {
		return nil, &ConfigError{Field: "limit", Reason: "must be between 1 and 25"}
	}
	return encodeConfig(c)
}

type lobstersStory struct {
	ShortID          string          `json:"short_id"`
	CreatedAt        string          `json:"created_at"`
	Title            string          `json:"title"`
	URL              string          `json:"url"`
	Score            int             `json:"score"`
	CommentCount     int             `json:"comment_count"`
	Description      string          `json:"description"`
	DescriptionPlain string          `json:"description_plain"`
	CommentsURL      string          `json:"comments_url"`
	Tags             []string        `json:"tags"`
	SubmitterUser    json.RawMessage `json:"submitter_user"`
}

// lobstersAuthor accepts the current form (a username string) and the older
// one (an object with a username field).
func lobstersAuthor(raw json.RawMessage) string {
	var name string
	if json.Unmarshal(raw, &name) == nil {
		return name
	}
	var obj struct {
		Username string `json:"username"`
	}
	if json.Unmarshal(raw, &obj) == nil {
		return obj.Username
	}
	return ""
}

// Fetch implements Connector.
func (l *Lobsters) Fetch(ctx context.Context, raw json.RawMessage, _ string) ([]Item, error) {
	norm, err := l.Normalize(raw)
	if err != nil {
		return nil, err
	}
	var c LobstersConfig
	_ = json.Unmarshal(norm, &c)

	endpoint := fmt.Sprintf("%s/%s.json", l.baseURL, c.List)
	label := c.List
	if c.Tag != "" {
		endpoint = fmt.Sprintf("%s/t/%s.json", l.baseURL, url.PathEscape(c.Tag))
		label = "tag " + c.Tag
	}
	var stories []lobstersStory
	if err := getJSON(ctx, l.client, endpoint, nil, &stories); err != nil {
		return nil, fmt.Errorf("lobsters %s: %w", label, err)
	}

	items := make([]Item, 0, len(stories))
	for _, s := range stories {
		if len(items) >= c.Limit {
			break
		}
		if s.ShortID == "" || s.Title == "" {
			continue
		}
		published, err := time.Parse(time.RFC3339, s.CreatedAt)
		if err != nil {
			continue
		}
		link := s.URL
		if link == "" {
			link = s.CommentsURL // text posts only have the discussion page
		}
		text := s.DescriptionPlain
		if text == "" {
			text = s.Description
		}
		items = append(items, Item{
			ExternalID:  s.ShortID,
			URL:         link,
			Title:       s.Title,
			Summary:     summarize(text),
			Author:      lobstersAuthor(s.SubmitterUser),
			PublishedAt: published.UTC(),
			Score:       intPtr(s.Score),
			Comments:    intPtr(s.CommentCount),
			Metrics:     map[string]any{"tags": s.Tags, "discussion_url": s.CommentsURL},
		})
	}
	return items, nil
}
