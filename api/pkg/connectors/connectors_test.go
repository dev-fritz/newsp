package connectors

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testNow = time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)

func clockNow() time.Time { return testNow }

func serveFile(t *testing.T, path, contentType string) *httptest.Server {
	t.Helper()
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestRSSFetch(t *testing.T) {
	t.Parallel()
	srv := serveFile(t, "testdata/rss2.xml", "application/rss+xml")
	c := NewRSS(srv.Client(), clockNow)

	items, err := c.Fetch(context.Background(), json.RawMessage(`{"url":"`+srv.URL+`"}`), "")
	require.NoError(t, err)
	require.Len(t, items, 2, "entries without a title are skipped")

	first := items[0]
	assert.Equal(t, "https://news.example.com/ai-finance", first.ExternalID)
	assert.Equal(t, "Regulators open consultation on AI in finance", first.Title)
	assert.Equal(t, "The consultation runs until December.", first.Summary, "HTML and scripts are stripped")
	assert.Equal(t, time.Date(2026, 10, 26, 10, 30, 0, 0, time.UTC), first.PublishedAt)
	assert.Equal(t, "Example Tech News", first.Metrics["feed_title"])

	assert.Equal(t, testNow, items[1].PublishedAt, "missing dates fall back to the fetch time")
	assert.Equal(t, "https://news.example.com/no-date", items[1].ExternalID, "missing GUIDs fall back to the link")
}

func TestRSSFetchAtomAndLimit(t *testing.T) {
	t.Parallel()
	srv := serveFile(t, "testdata/atom.xml", "application/atom+xml")
	items, err := NewRSS(srv.Client(), clockNow).Fetch(context.Background(), json.RawMessage(`{"url":"`+srv.URL+`","limit":1}`), "")
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "tag:blog.example.com,2026:inference", items[0].ExternalID)
	assert.Equal(t, "Ana Costa", items[0].Author)
}

func TestRSSNormalize(t *testing.T) {
	t.Parallel()
	c := NewRSS(http.DefaultClient, clockNow)
	tests := []struct {
		raw     string
		want    string
		wantErr string
	}{
		{raw: `{"url":" https://feeds.bbci.co.uk/news/rss.xml "}`, want: `{"url":"https://feeds.bbci.co.uk/news/rss.xml","limit":50}`},
		{raw: `{"url":"ftp://x/feed"}`, wantErr: "url"},
		{raw: `{"url":"javascript:alert(1)"}`, wantErr: "url"},
		{raw: `{}`, wantErr: "url"},
		{raw: `{"url":"https://x.com/f","limit":500}`, wantErr: "limit"},
		{raw: `[1]`, wantErr: "config"},
	}
	for _, tt := range tests {
		got, err := c.Normalize(json.RawMessage(tt.raw))
		if tt.wantErr != "" {
			require.Error(t, err, tt.raw)
			assert.True(t, IsConfigError(err))
			assert.Contains(t, err.Error(), tt.wantErr)
			continue
		}
		require.NoError(t, err)
		assert.JSONEq(t, tt.want, string(got))
	}
}

func TestHackerNewsFetch(t *testing.T) {
	t.Parallel()
	items := map[string]string{
		"/topstories.json": `[1, 2, 3, 4]`,
		"/item/1.json":     `{"id":1,"type":"story","by":"pg","time":1793010000,"title":"Show HN: Tracing for LLM calls","url":"https://example.com/tracing","score":238,"descendants":91}`,
		"/item/2.json":     `{"id":2,"type":"story","by":"dang","time":1793010100,"title":"Ask HN: Who uses AI code review?","text":"<p>Curious about <i>real</i> usage.</p>","score":412,"descendants":286}`,
		"/item/3.json":     `{"id":3,"type":"job","title":"We are hiring","time":1793010200}`,
		"/item/4.json":     `{"id":4,"type":"story","dead":true,"title":"Spam","time":1793010300}`,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := items[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	got, err := NewHackerNews(srv.Client(), srv.URL).Fetch(context.Background(), json.RawMessage(`{"list":"top","limit":10}`), "")
	require.NoError(t, err)
	require.Len(t, got, 2, "jobs and dead stories are skipped")

	assert.Equal(t, "1", got[0].ExternalID)
	assert.Equal(t, "https://example.com/tracing", got[0].URL)
	assert.Equal(t, 238, *got[0].Score)
	assert.Equal(t, 91, *got[0].Comments)
	assert.Equal(t, "https://news.ycombinator.com/item?id=1", got[0].Metrics["discussion_url"])

	assert.Equal(t, "https://news.ycombinator.com/item?id=2", got[1].URL, "text posts link to the discussion")
	assert.Equal(t, "Curious about real usage.", got[1].Summary)
	assert.Equal(t, time.Unix(1793010100, 0).UTC(), got[1].PublishedAt)
}

func TestHackerNewsNormalize(t *testing.T) {
	t.Parallel()
	c := NewHackerNews(http.DefaultClient, "")
	got, err := c.Normalize(nil)
	require.NoError(t, err)
	assert.JSONEq(t, `{"list":"top","limit":30}`, string(got))

	_, err = c.Normalize(json.RawMessage(`{"list":"ask"}`))
	assert.True(t, IsConfigError(err))
	_, err = c.Normalize(json.RawMessage(`{"limit":101}`))
	assert.True(t, IsConfigError(err))
}

func TestHackerNewsListError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	_, err := NewHackerNews(srv.Client(), srv.URL).Fetch(context.Background(), nil, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "503")
}

func TestStripHTMLAndTruncate(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "a b c", StripHTML("a<br>b<style>.x{}</style> c"))
	assert.Equal(t, "Tom & Jerry", StripHTML("Tom &amp; Jerry"))
	long := strings.Repeat("é", maxSummaryRunes+10)
	assert.Equal(t, maxSummaryRunes+1, len([]rune(summarize(long))))
}

func TestTitleFromText(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "short post", titleFromText("  short\n post "))
	long := strings.Repeat("palavra ", 30)
	got := titleFromText(long)
	assert.True(t, strings.HasSuffix(got, "palavra…"), "cut on a word boundary: %q", got)
	assert.LessOrEqual(t, len([]rune(got)), maxTitleRunes+1)
	noSpaces := strings.Repeat("é", 200)
	assert.Equal(t, strings.Repeat("é", maxTitleRunes)+"…", titleFromText(noSpaces))
}

func TestRegistry(t *testing.T) {
	t.Parallel()
	r := NewRegistry(NewHackerNews(http.DefaultClient, ""), NewRSS(http.DefaultClient, clockNow))
	_, ok := r.Get("rss")
	assert.True(t, ok)
	_, ok = r.Get("myspace")
	assert.False(t, ok)
	assert.Equal(t, []string{"hackernews", "rss"}, r.Kinds())
}
