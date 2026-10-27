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
)

func TestDevToFetch(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("testdata/devto.json")
	require.NoError(t, err)

	cases := []struct {
		name, config, path string
		query              map[string]string
	}{
		{"latest default", ``, "/api/articles/latest", map[string]string{"per_page": "30", "tag": "", "top": ""}},
		{"latest tag", `{"tag":"#Rust","limit":5}`, "/api/articles/latest", map[string]string{"per_page": "5", "tag": "rust", "top": ""}},
		{"top", `{"top":7,"limit":10}`, "/api/articles", map[string]string{"per_page": "10", "tag": "", "top": "7"}},
		{"top with tag", `{"top":30,"tag":"go"}`, "/api/articles", map[string]string{"per_page": "30", "tag": "go", "top": "30"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tc.path, r.URL.Path)
				q := r.URL.Query()
				for k, v := range tc.query {
					assert.Equal(t, v, q.Get(k), k)
				}
				_, _ = w.Write(body)
			}))
			defer srv.Close()
			got, err := NewDevTo(srv.Client(), srv.URL).Fetch(context.Background(), json.RawMessage(tc.config), "")
			require.NoError(t, err)
			require.Len(t, got, 2, "entries with id 0 are skipped")

			first := got[0]
			assert.Equal(t, "1001", first.ExternalID)
			assert.Equal(t, "https://dev.example/carol/building-a-cli-in-rust-1abc", first.URL)
			assert.Equal(t, "Building a CLI in Rust", first.Title)
			assert.Equal(t, "A short walkthrough of a command line tool.", first.Summary)
			assert.Equal(t, "carol", first.Author)
			assert.Equal(t, time.Date(2026, 10, 25, 14, 30, 0, 0, time.UTC), first.PublishedAt)
			assert.Equal(t, 55, *first.Score)
			assert.Equal(t, 4, *first.Comments)
			assert.Equal(t, map[string]any{"tags": []string{"rust", "cli"}, "reading_time_minutes": 6}, first.Metrics)

			assert.Equal(t, time.Date(2026, 10, 26, 15, 0, 0, 0, time.UTC), got[1].PublishedAt, "converted to UTC")
		})
	}
}

func TestDevToNormalize(t *testing.T) {
	t.Parallel()
	c := NewDevTo(http.DefaultClient, "")
	assert.Equal(t, "devto", c.Kind())
	assert.Equal(t, SecretNone, c.Secret(nil))

	got, err := c.Normalize(nil)
	require.NoError(t, err)
	assert.JSONEq(t, `{"tag":"","top":0,"limit":30}`, string(got))

	got, err = c.Normalize(json.RawMessage(`{"tag":" #WebDev ","top":1}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"tag":"webdev","top":1,"limit":30}`, string(got))

	for _, raw := range []string{
		`{"tag":"web-dev"}`, `{"top":2}`, `{"top":-1}`, `{"limit":101}`, `{"limit":-1}`, `[1]`,
	} {
		_, err := c.Normalize(json.RawMessage(raw))
		require.Error(t, err, raw)
		assert.True(t, IsConfigError(err), raw)
	}
}

func TestDevToUpstreamError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	_, err := NewDevTo(srv.Client(), srv.URL).Fetch(context.Background(), nil, "")
	require.Error(t, err)
	assert.False(t, IsConfigError(err))
	assert.Contains(t, err.Error(), "500")
}
