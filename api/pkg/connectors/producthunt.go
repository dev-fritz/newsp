package connectors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const productHuntBaseURL = "https://api.producthunt.com"

var productHuntTopicPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,59}$`)

const productHuntQuery = `query Posts($first: Int!, $order: PostsOrder, $postedAfter: DateTime, $topic: String) { posts(first: $first, order: $order, postedAfter: $postedAfter, topic: $topic) { edges { node { id name tagline description url website votesCount commentsCount createdAt topics(first: 5) { edges { node { name } } } } } } }`

// ProductHuntConfig selects a Product Hunt launch listing.
type ProductHuntConfig struct {
	Order            string `json:"order"`              // ranking | votes | newest
	Topic            string `json:"topic"`              // optional topic slug
	PostedWithinDays int    `json:"posted_within_days"` // 1..7
	Limit            int    `json:"limit"`              // 1..50
}

// ProductHunt reads launches from the Product Hunt GraphQL API (needs a
// developer token).
type ProductHunt struct {
	client  Doer
	baseURL string
	now     func() time.Time
}

// NewProductHunt builds the connector. baseURL is empty in production.
func NewProductHunt(client Doer, baseURL string, now func() time.Time) *ProductHunt {
	if baseURL == "" {
		baseURL = productHuntBaseURL
	}
	return &ProductHunt{client: client, baseURL: baseURL, now: now}
}

// Kind implements Connector.
func (*ProductHunt) Kind() string { return "producthunt" }

// Secret implements Connector: the API requires a developer token.
func (*ProductHunt) Secret(json.RawMessage) SecretMode { return SecretRequired }

// Normalize implements Connector.
func (*ProductHunt) Normalize(raw json.RawMessage) (json.RawMessage, error) {
	var c ProductHuntConfig
	if err := decodeConfig(raw, &c); err != nil {
		return nil, err
	}
	if c.Order == "" {
		c.Order = "ranking"
	}
	if c.Order != "ranking" && c.Order != "votes" && c.Order != "newest" {
		return nil, &ConfigError{Field: "order", Reason: "must be ranking, votes or newest"}
	}
	c.Topic = strings.TrimSpace(c.Topic)
	if c.Topic != "" && !productHuntTopicPattern.MatchString(c.Topic) {
		return nil, &ConfigError{Field: "topic", Reason: "must be a lowercase topic slug such as artificial-intelligence"}
	}
	if c.PostedWithinDays == 0 {
		c.PostedWithinDays = 1
	}
	if c.PostedWithinDays < 1 || c.PostedWithinDays > 7 {
		return nil, &ConfigError{Field: "posted_within_days", Reason: "must be between 1 and 7"}
	}
	if c.Limit == 0 {
		c.Limit = 20
	}
	if c.Limit < 1 || c.Limit > 50 {
		return nil, &ConfigError{Field: "limit", Reason: "must be between 1 and 50"}
	}
	return encodeConfig(c)
}

type productHuntResponse struct {
	Data struct {
		Posts struct {
			Edges []struct {
				Node productHuntPost `json:"node"`
			} `json:"edges"`
		} `json:"posts"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

type productHuntPost struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Tagline       string    `json:"tagline"`
	Description   string    `json:"description"`
	URL           string    `json:"url"`
	Website       string    `json:"website"`
	VotesCount    int       `json:"votesCount"`
	CommentsCount int       `json:"commentsCount"`
	CreatedAt     time.Time `json:"createdAt"`
	Topics        struct {
		Edges []struct {
			Node struct {
				Name string `json:"name"`
			} `json:"node"`
		} `json:"edges"`
	} `json:"topics"`
}

// Fetch implements Connector.
func (p *ProductHunt) Fetch(ctx context.Context, raw json.RawMessage, secret string) ([]Item, error) {
	norm, err := p.Normalize(raw)
	if err != nil {
		return nil, err
	}
	if secret == "" {
		return nil, errors.New("producthunt needs a developer token")
	}
	var c ProductHuntConfig
	_ = json.Unmarshal(norm, &c)

	vars := map[string]any{
		"first":       c.Limit,
		"order":       strings.ToUpper(c.Order),
		"postedAfter": p.now().UTC().AddDate(0, 0, -c.PostedWithinDays).Format(time.RFC3339),
	}
	if c.Topic != "" {
		vars["topic"] = c.Topic
	}
	header := http.Header{
		"Authorization": {"Bearer " + secret},
		"Accept":        {"application/json"},
	}
	var res productHuntResponse
	payload := map[string]any{"query": productHuntQuery, "variables": vars}
	if err := postJSON(ctx, p.client, p.baseURL+"/v2/api/graphql", header, payload, &res); err != nil {
		return nil, fmt.Errorf("producthunt: %w", err)
	}
	if len(res.Errors) > 0 {
		msg := res.Errors[0].Message
		if utf8.RuneCountInString(msg) > 200 {
			msg = string([]rune(msg)[:200])
		}
		return nil, fmt.Errorf("producthunt: %s", msg)
	}

	items := make([]Item, 0, len(res.Data.Posts.Edges))
	for _, e := range res.Data.Posts.Edges {
		n := e.Node
		if n.ID == "" || n.Name == "" || n.URL == "" {
			continue
		}
		title := n.Name
		if t := strings.TrimSpace(n.Tagline); t != "" {
			title += " — " + t
		}
		summary := summarize(n.Description)
		if summary == "" {
			summary = summarize(n.Tagline)
		}
		topics := make([]string, 0, len(n.Topics.Edges))
		for _, t := range n.Topics.Edges {
			if t.Node.Name != "" {
				topics = append(topics, t.Node.Name)
			}
		}
		items = append(items, Item{
			ExternalID:  n.ID,
			URL:         n.URL,
			Title:       title,
			Summary:     summary,
			PublishedAt: n.CreatedAt.UTC(),
			Score:       intPtr(n.VotesCount),
			Comments:    intPtr(n.CommentsCount),
			Metrics:     map[string]any{"website": n.Website, "topics": topics},
		})
	}
	return items, nil
}
