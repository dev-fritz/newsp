package connectors

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dev-fritz/newsp/api/pkg/safehttp"
)

func TestRedditFetch(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("testdata/reddit.json")
	require.NoError(t, err)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/r/SaaS/top.json", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "10", q.Get("limit"))
		assert.Equal(t, "week", q.Get("t"))
		assert.Equal(t, safehttp.UserAgent, r.Header.Get("User-Agent"))
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	got, err := NewReddit(srv.Client(), srv.URL, time.Now).Fetch(context.Background(), json.RawMessage(`{"subreddit":"r/SaaS","time":"week","limit":10}`), "")
	require.NoError(t, err)
	require.Len(t, got, 2, "NSFW, stickied and non-t3 children are skipped")

	self := got[0]
	assert.Equal(t, "1abc23", self.ExternalID)
	assert.Equal(t, "https://www.reddit.com/r/SaaS/comments/1abc23/i_built_a_niche_invoicing_saas/", self.URL)
	assert.Equal(t, "Here is **what worked**: cold outreach and a small landing page.", self.Summary)
	assert.Equal(t, "indie_builder", self.Author)
	assert.Equal(t, time.Unix(1793010000, 0).UTC(), self.PublishedAt)
	assert.Equal(t, 321, *self.Score)
	assert.Equal(t, 58, *self.Comments)
	assert.Equal(t, "SaaS", self.Metrics["subreddit"])
	assert.Equal(t, self.URL, self.Metrics["discussion_url"])

	link := got[1]
	assert.Equal(t, "https://example.com/pricing-teardown", link.URL, "link posts point at the target")
	assert.Equal(t, "https://www.reddit.com/r/SaaS/comments/1def45/pricing_teardown_of_50_b2b_tools/", link.Metrics["discussion_url"])
}

func TestRedditNonTopOmitsTime(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/r/golang/new.json", r.URL.Path)
		assert.Equal(t, "25", r.URL.Query().Get("limit"))
		assert.False(t, r.URL.Query().Has("t"))
		_, _ = w.Write([]byte(`{"data":{"children":[]}}`))
	}))
	defer srv.Close()
	got, err := NewReddit(srv.Client(), srv.URL, time.Now).Fetch(context.Background(), json.RawMessage(`{"subreddit":"golang","sort":"new"}`), "")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestRedditNormalize(t *testing.T) {
	t.Parallel()
	c := NewReddit(http.DefaultClient, "", time.Now)
	assert.Equal(t, "reddit", c.Kind())
	for _, raw := range []string{`{"subreddit":"r/SaaS"}`, `{"subreddit":"SaaS"}`, `{"subreddit":" /r/SaaS "}`} {
		got, err := c.Normalize(json.RawMessage(raw))
		require.NoError(t, err, raw)
		assert.JSONEq(t, `{"subreddit":"SaaS","sort":"top","time":"day","limit":25}`, string(got), raw)
	}

	for _, raw := range []string{
		`{}`,
		`{"subreddit":"a"}`,
		`{"subreddit":"this_name_is_way_too_long"}`,
		`{"subreddit":"bad name"}`,
		`{"subreddit":"r/"}`,
		`{"subreddit":"SaaS","sort":"rising"}`,
		`{"subreddit":"SaaS","time":"year"}`,
		`{"subreddit":"SaaS","limit":101}`,
		`{"subreddit":"SaaS","limit":-5}`,
		`[1]`,
	} {
		_, err := c.Normalize(json.RawMessage(raw))
		require.Error(t, err, raw)
		assert.True(t, IsConfigError(err), raw)
	}
}

func TestRedditUpstreamError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	_, err := NewReddit(srv.Client(), srv.URL, time.Now).Fetch(context.Background(), json.RawMessage(`{"subreddit":"SaaS"}`), "")
	require.Error(t, err)
	assert.False(t, IsConfigError(err))
	assert.Contains(t, err.Error(), "429")
}

func TestRedditFallsBackToFeedWhenJSONIsForbidden(t *testing.T) {
	t.Parallel()
	feed, err := os.ReadFile("testdata/reddit.atom")
	require.NoError(t, err)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/r/SaaS/top.json":
			w.WriteHeader(http.StatusForbidden)
		case "/r/SaaS/top/.rss":
			assert.Equal(t, "week", r.URL.Query().Get("t"))
			assert.Equal(t, "10", r.URL.Query().Get("limit"))
			_, _ = w.Write(feed)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	got, err := NewReddit(srv.Client(), srv.URL, time.Now).Fetch(context.Background(), json.RawMessage(`{"subreddit":"SaaS","time":"week","limit":10}`), "")
	require.NoError(t, err)
	require.Len(t, got, 1)
	item := got[0]
	assert.Equal(t, "1abc23", item.ExternalID, "same ID as the JSON listing")
	assert.Equal(t, "https://www.reddit.com/r/SaaS/comments/1abc23/invoicing/", item.URL)
	assert.Equal(t, "I built a niche invoicing SaaS", item.Title)
	assert.Equal(t, "Here is what worked: cold outreach.", item.Summary, "the submitted-by footer is dropped")
	assert.Equal(t, "indie_builder", item.Author)
	assert.Equal(t, time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC), item.PublishedAt)
	assert.Nil(t, item.Score, "the feed has no score")
	assert.Equal(t, item.URL, item.Metrics["discussion_url"])
}
