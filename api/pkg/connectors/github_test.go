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

func fixedNow() time.Time { return time.Date(2026, 10, 26, 15, 0, 0, 0, time.UTC) }

func TestGitHubFetch(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("testdata/github.json")
	require.NoError(t, err)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/search/repositories", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, `notes created:>=2026-10-19 stars:>=50 language:"Jupyter Notebook" topic:ai-tools`, q.Get("q"))
		assert.Equal(t, "stars", q.Get("sort"))
		assert.Equal(t, "desc", q.Get("order"))
		assert.Equal(t, "40", q.Get("per_page"))
		assert.Equal(t, "application/vnd.github+json", r.Header.Get("Accept"))
		assert.Equal(t, "2022-11-28", r.Header.Get("X-GitHub-Api-Version"))
		assert.Equal(t, "Bearer tok-123", r.Header.Get("Authorization"))
		assert.NotContains(t, r.URL.String(), "tok-123")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	cfg := `{"query":" notes ","language":"Jupyter Notebook","topic":"ai-tools","min_stars":50,"limit":40}`
	got, err := NewGitHub(srv.Client(), srv.URL, fixedNow).Fetch(context.Background(), json.RawMessage(cfg), "tok-123")
	require.NoError(t, err)
	require.Len(t, got, 2, "fork and archived repos are skipped")

	first := got[0]
	assert.Equal(t, "900001", first.ExternalID)
	assert.Equal(t, "https://github.com/acme-labs/quillbase", first.URL)
	assert.Equal(t, "acme-labs/quillbase: A tiny embeddable notes database", first.Title)
	assert.Equal(t, "A tiny embeddable notes database Topics: database, notes.", first.Summary)
	assert.Equal(t, "acme-labs", first.Author)
	assert.Equal(t, time.Date(2026, 10, 22, 8, 30, 0, 0, time.UTC), first.PublishedAt)
	assert.Equal(t, 412, *first.Score)
	assert.Nil(t, first.Comments)
	assert.Equal(t, 17, first.Metrics["forks"])
	assert.Equal(t, 412, first.Metrics["stars"])
	assert.Equal(t, "Go", first.Metrics["language"])
	assert.Equal(t, []string{"database", "notes"}, first.Metrics["topics"])
	assert.Equal(t, "2026-10-25T12:00:00Z", first.Metrics["pushed_at"])

	bare := got[1]
	assert.Equal(t, "dev-example/bare-repo", bare.Title)
	assert.Empty(t, bare.Summary)
}

func TestGitHubDefaultsAndNoToken(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		assert.Equal(t, "created:>=2026-10-19 stars:>=10", q.Get("q"))
		assert.Equal(t, "30", q.Get("per_page"))
		assert.Empty(t, r.Header.Values("Authorization"))
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()
	got, err := NewGitHub(srv.Client(), srv.URL, fixedNow).Fetch(context.Background(), json.RawMessage(`{}`), "")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestGitHubErrorDoesNotLeakToken(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	_, err := NewGitHub(srv.Client(), srv.URL, fixedNow).Fetch(context.Background(), json.RawMessage(`{}`), "tok-secret")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "403")
	assert.NotContains(t, err.Error(), "tok-secret")
	assert.False(t, IsConfigError(err))
}

func TestGitHubLongTitleIsCapped(t *testing.T) {
	t.Parallel()
	desc := strings.Repeat("palavra ", 60)
	body := `{"items":[{"id":1,"full_name":"a/b","html_url":"https://github.com/a/b","description":"` + desc + `","owner":{"login":"a"},"created_at":"2026-10-25T00:00:00Z"}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }))
	defer srv.Close()
	got, err := NewGitHub(srv.Client(), srv.URL, fixedNow).Fetch(context.Background(), json.RawMessage(`{}`), "")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.LessOrEqual(t, len([]rune(got[0].Title)), 200)
}

func TestGitHubNormalize(t *testing.T) {
	t.Parallel()
	c := NewGitHub(http.DefaultClient, "", fixedNow)
	assert.Equal(t, "github", c.Kind())
	assert.Equal(t, SecretOptional, c.Secret(nil))

	got, err := c.Normalize(json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"query":"","language":"","topic":"","min_stars":10,"created_within_days":7,"limit":30}`, string(got))

	got, err = c.Normalize(json.RawMessage(`{"query":" ai ","language":"C++","topic":"llm","min_stars":1000000,"created_within_days":30,"limit":100}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"query":"ai","language":"C++","topic":"llm","min_stars":1000000,"created_within_days":30,"limit":100}`, string(got))

	for _, raw := range []string{
		`{"query":"` + strings.Repeat("a", 201) + `"}`,
		`{"query":"foo created:>2020-01-01"}`,
		`{"query":"STARS:>5"}`,
		`{"query":"x sort:forks"}`,
		`{"language":"bad/lang"}`,
		`{"language":"` + strings.Repeat("a", 41) + `"}`,
		`{"topic":"under_score"}`,
		`{"topic":"-lead"}`,
		`{"min_stars":-1}`,
		`{"min_stars":1000001}`,
		`{"created_within_days":31}`,
		`{"created_within_days":-1}`,
		`{"limit":101}`,
		`{"limit":-1}`,
		`[1]`,
	} {
		_, err := c.Normalize(json.RawMessage(raw))
		require.Error(t, err, raw)
		assert.True(t, IsConfigError(err), raw)
	}
}
