package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const githubBaseURL = "https://api.github.com"

var (
	githubLanguagePattern = regexp.MustCompile(`^[A-Za-z0-9+#._ -]{1,40}$`)
	githubTopicPattern    = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,49}$`)
)

const (
	maxGitHubQueryRunes = 200
	maxGitHubTitleRunes = 200
)

// GitHubConfig selects newly created, popular repositories.
type GitHubConfig struct {
	Query             string `json:"query"`               // optional free text, up to 200 characters
	Language          string `json:"language"`            // optional, e.g. Go or "Jupyter Notebook"
	Topic             string `json:"topic"`               // optional topic slug, e.g. machine-learning
	MinStars          int    `json:"min_stars"`           // 1..1000000; 0 means the default (10)
	CreatedWithinDays int    `json:"created_within_days"` // 1..30; 0 means the default (7)
	Limit             int    `json:"limit"`               // 1..100; 0 means the default (30)
}

// GitHub lists trending new repositories through the repository search API.
type GitHub struct {
	client  Doer
	baseURL string
	now     func() time.Time
}

// NewGitHub builds the connector. baseURL is empty in production.
func NewGitHub(client Doer, baseURL string, now func() time.Time) *GitHub {
	if baseURL == "" {
		baseURL = githubBaseURL
	}
	return &GitHub{client: client, baseURL: baseURL, now: now}
}

// Kind implements Connector.
func (*GitHub) Kind() string { return "github" }

// Secret implements Connector: a token raises the search limit from 10 to 30
// requests per minute.
func (*GitHub) Secret(json.RawMessage) SecretMode { return SecretOptional }

// Normalize implements Connector.
func (*GitHub) Normalize(raw json.RawMessage) (json.RawMessage, error) {
	var c GitHubConfig
	if err := decodeConfig(raw, &c); err != nil {
		return nil, err
	}
	c.Query = strings.TrimSpace(c.Query)
	if utf8.RuneCountInString(c.Query) > maxGitHubQueryRunes {
		return nil, &ConfigError{Field: "query", Reason: "must have at most 200 characters"}
	}
	lower := strings.ToLower(c.Query)
	for _, q := range []string{"created:", "stars:", "sort:"} {
		if strings.Contains(lower, q) {
			return nil, &ConfigError{Field: "query", Reason: "must not contain " + q + " (set by the other fields)"}
		}
	}
	c.Language = strings.TrimSpace(c.Language)
	if c.Language != "" && !githubLanguagePattern.MatchString(c.Language) {
		return nil, &ConfigError{Field: "language", Reason: "must be a language name such as Go (letters, digits and + # . _ - space, up to 40)"}
	}
	c.Topic = strings.ToLower(strings.TrimSpace(c.Topic))
	if c.Topic != "" && !githubTopicPattern.MatchString(c.Topic) {
		return nil, &ConfigError{Field: "topic", Reason: "must be a lowercase topic slug such as machine-learning"}
	}
	if c.MinStars == 0 {
		c.MinStars = 10
	}
	if c.MinStars < 0 || c.MinStars > 1000000 {
		return nil, &ConfigError{Field: "min_stars", Reason: "must be between 1 and 1000000"}
	}
	if c.CreatedWithinDays == 0 {
		c.CreatedWithinDays = 7
	}
	if c.CreatedWithinDays < 1 || c.CreatedWithinDays > 30 {
		return nil, &ConfigError{Field: "created_within_days", Reason: "must be between 1 and 30"}
	}
	if c.Limit == 0 {
		c.Limit = 30
	}
	if c.Limit < 1 || c.Limit > 100 {
		return nil, &ConfigError{Field: "limit", Reason: "must be between 1 and 100"}
	}
	return encodeConfig(c)
}

type githubSearch struct {
	Items []githubRepo `json:"items"`
}

type githubRepo struct {
	ID          int64  `json:"id"`
	FullName    string `json:"full_name"`
	HTMLURL     string `json:"html_url"`
	Description string `json:"description"`
	Owner       struct {
		Login string `json:"login"`
	} `json:"owner"`
	CreatedAt       time.Time `json:"created_at"`
	PushedAt        string    `json:"pushed_at"`
	StargazersCount int       `json:"stargazers_count"`
	ForksCount      int       `json:"forks_count"`
	OpenIssues      int       `json:"open_issues_count"`
	Language        string    `json:"language"`
	Topics          []string  `json:"topics"`
	Fork            bool      `json:"fork"`
	Archived        bool      `json:"archived"`
}

// Fetch implements Connector.
func (g *GitHub) Fetch(ctx context.Context, raw json.RawMessage, secret string) ([]Item, error) {
	norm, err := g.Normalize(raw)
	if err != nil {
		return nil, err
	}
	var c GitHubConfig
	_ = json.Unmarshal(norm, &c)

	since := g.now().UTC().AddDate(0, 0, -c.CreatedWithinDays).Format("2006-01-02")
	var terms []string
	if c.Query != "" {
		terms = append(terms, c.Query)
	}
	terms = append(terms, "created:>="+since, "stars:>="+strconv.Itoa(c.MinStars))
	if c.Language != "" {
		lang := c.Language
		if strings.Contains(lang, " ") {
			lang = `"` + lang + `"`
		}
		terms = append(terms, "language:"+lang)
	}
	if c.Topic != "" {
		terms = append(terms, "topic:"+c.Topic)
	}
	v := url.Values{}
	v.Set("q", strings.Join(terms, " "))
	v.Set("sort", "stars")
	v.Set("order", "desc")
	v.Set("per_page", strconv.Itoa(c.Limit))

	header := http.Header{
		"Accept":               {"application/vnd.github+json"},
		"X-Github-Api-Version": {"2022-11-28"},
	}
	if secret != "" {
		header.Set("Authorization", "Bearer "+secret)
	}
	var res githubSearch
	if err := getJSON(ctx, g.client, g.baseURL+"/search/repositories?"+v.Encode(), header, &res); err != nil {
		return nil, fmt.Errorf("github search: %w", err)
	}

	items := make([]Item, 0, len(res.Items))
	for _, r := range res.Items {
		if r.Fork || r.Archived || r.ID == 0 || r.FullName == "" || r.HTMLURL == "" {
			continue
		}
		title := r.FullName
		if d := strings.TrimSpace(r.Description); d != "" {
			title += ": " + collapseSpaces(d)
		}
		if utf8.RuneCountInString(title) > maxGitHubTitleRunes {
			title = strings.TrimSpace(string([]rune(title)[:maxGitHubTitleRunes-1])) + "…"
		}
		summary := summarize(r.Description)
		if len(r.Topics) > 0 {
			summary = strings.TrimSpace(summary + " Topics: " + strings.Join(r.Topics, ", ") + ".")
		}
		topics := r.Topics
		if topics == nil {
			topics = []string{}
		}
		items = append(items, Item{
			ExternalID:  strconv.FormatInt(r.ID, 10),
			URL:         r.HTMLURL,
			Title:       title,
			Summary:     summary,
			Author:      r.Owner.Login,
			PublishedAt: r.CreatedAt.UTC(),
			Score:       intPtr(r.StargazersCount),
			Metrics: map[string]any{
				"stars":     r.StargazersCount,
				"forks":     r.ForksCount,
				"language":  r.Language,
				"topics":    topics,
				"pushed_at": r.PushedAt,
			},
		})
	}
	return items, nil
}
