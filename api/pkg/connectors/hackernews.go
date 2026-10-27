package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/dev-fritz/newsp/api/pkg/safehttp"
)

const hnBaseURL = "https://hacker-news.firebaseio.com/v0"

// HackerNewsConfig selects a Hacker News list.
type HackerNewsConfig struct {
	List  string `json:"list"`  // top | best | new
	Limit int    `json:"limit"` // 1..100
}

// HackerNews reads stories from the official Firebase API.
type HackerNews struct {
	client  Doer
	baseURL string
}

// NewHackerNews builds the connector. baseURL is empty in production.
func NewHackerNews(client Doer, baseURL string) *HackerNews {
	if baseURL == "" {
		baseURL = hnBaseURL
	}
	return &HackerNews{client: client, baseURL: baseURL}
}

// Secret implements Connector: the source is public.
func (*HackerNews) Secret(json.RawMessage) SecretMode { return SecretNone }

// Kind implements Connector.
func (h *HackerNews) Kind() string { return "hackernews" }

// Normalize implements Connector.
func (h *HackerNews) Normalize(raw json.RawMessage) (json.RawMessage, error) {
	var c HackerNewsConfig
	if err := decodeConfig(raw, &c); err != nil {
		return nil, err
	}
	if c.List == "" {
		c.List = "top"
	}
	if c.List != "top" && c.List != "best" && c.List != "new" {
		return nil, &ConfigError{Field: "list", Reason: "must be top, best or new"}
	}
	if c.Limit == 0 {
		c.Limit = 30
	}
	if c.Limit < 1 || c.Limit > 100 {
		return nil, &ConfigError{Field: "limit", Reason: "must be between 1 and 100"}
	}
	return encodeConfig(c)
}

type hnItem struct {
	ID          int    `json:"id"`
	Type        string `json:"type"`
	By          string `json:"by"`
	Time        int64  `json:"time"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Text        string `json:"text"`
	Score       int    `json:"score"`
	Descendants int    `json:"descendants"`
	Deleted     bool   `json:"deleted"`
	Dead        bool   `json:"dead"`
}

// Fetch implements Connector.
func (h *HackerNews) Fetch(ctx context.Context, raw json.RawMessage, _ string) ([]Item, error) {
	norm, err := h.Normalize(raw)
	if err != nil {
		return nil, err
	}
	var c HackerNewsConfig
	_ = json.Unmarshal(norm, &c)

	var ids []int
	if err := getJSON(ctx, h.client, fmt.Sprintf("%s/%sstories.json", h.baseURL, c.List), nil, &ids); err != nil {
		return nil, fmt.Errorf("hacker news %s list: %w", c.List, err)
	}
	if len(ids) > c.Limit {
		ids = ids[:c.Limit]
	}

	// Items are fetched one request each; a small pool keeps it polite.
	results := make([]*hnItem, len(ids))
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
	)
	sem := make(chan struct{}, 8)
	for i, id := range ids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			var it hnItem
			if err := getJSON(ctx, h.client, fmt.Sprintf("%s/item/%d.json", h.baseURL, id), nil, &it); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}
			results[i] = &it
		}()
	}
	wg.Wait()

	items := make([]Item, 0, len(results))
	for _, it := range results {
		if it == nil || it.Deleted || it.Dead || it.Type != "story" || it.Title == "" {
			continue
		}
		discussion := "https://news.ycombinator.com/item?id=" + strconv.Itoa(it.ID)
		url := it.URL
		if url == "" {
			url = discussion // Ask HN and text posts
		}
		items = append(items, Item{
			ExternalID:  strconv.Itoa(it.ID),
			URL:         url,
			Title:       it.Title,
			Summary:     summarize(it.Text),
			Author:      it.By,
			PublishedAt: time.Unix(it.Time, 0).UTC(),
			Score:       intPtr(it.Score),
			Comments:    intPtr(it.Descendants),
			Metrics:     map[string]any{"discussion_url": discussion},
		})
	}
	if len(items) == 0 && firstErr != nil {
		return nil, fmt.Errorf("hacker news items: %w", firstErr)
	}
	return items, nil
}

// getJSON GETs url and decodes a JSON body of at most 5 MB.
func getJSON(ctx context.Context, client Doer, url string, header http.Header, dst any) error {
	body, err := safehttp.Get(ctx, client, url, 5<<20, header)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, dst); err != nil {
		return fmt.Errorf("decode %s: %w", url, err)
	}
	return nil
}

// postJSON sends payload as JSON to url and decodes the response into dst.
func postJSON(ctx context.Context, client Doer, url string, header http.Header, payload, dst any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}
	res, err := safehttp.Post(ctx, client, url, body, 5<<20, header)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(res, dst); err != nil {
		return fmt.Errorf("decode %s: %w", url, err)
	}
	return nil
}
