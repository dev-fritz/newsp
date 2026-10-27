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

func TestLobstersFetch(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("testdata/lobsters.json")
	require.NoError(t, err)

	cases := []struct {
		name, config, path string
	}{
		{"newest", `{"list":"newest"}`, "/newest.json"},
		{"default", ``, "/hottest.json"},
		{"tag ignores list", `{"list":"newest","tag":"#Go"}`, "/t/go.json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tc.path, r.URL.Path)
				_, _ = w.Write(body)
			}))
			defer srv.Close()
			got, err := NewLobsters(srv.Client(), srv.URL).Fetch(context.Background(), json.RawMessage(tc.config), "")
			require.NoError(t, err)
			require.Len(t, got, 2, "entries without short_id are skipped")
		})
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) }))
	defer srv.Close()
	c := NewLobsters(srv.Client(), srv.URL)
	got, err := c.Fetch(context.Background(), nil, "")
	require.NoError(t, err)

	first := got[0]
	assert.Equal(t, "abc123", first.ExternalID)
	assert.Equal(t, "https://example.com/tiny-db", first.URL)
	assert.Equal(t, "alice", first.Author)
	assert.Equal(t, time.Date(2026, 10, 25, 15, 30, 0, 0, time.UTC), first.PublishedAt)
	assert.Equal(t, 31, *first.Score)
	assert.Equal(t, 9, *first.Comments)
	assert.Equal(t, map[string]any{
		"tags":           []string{"databases", "programming"},
		"discussion_url": "https://lobste.rs/s/abc123/a_tiny_database",
	}, first.Metrics)

	second := got[1]
	assert.Equal(t, "https://lobste.rs/s/def456/ask_how_do_you_review", second.URL, "text post links to the discussion")
	assert.Equal(t, "bob", second.Author, "object-form submitter_user")
	assert.Equal(t, "Looking for practical review habits.", second.Summary)

	limited, err := c.Fetch(context.Background(), json.RawMessage(`{"limit":1}`), "")
	require.NoError(t, err)
	assert.Len(t, limited, 1)
}

func TestLobstersNormalize(t *testing.T) {
	t.Parallel()
	c := NewLobsters(http.DefaultClient, "")
	assert.Equal(t, "lobsters", c.Kind())
	assert.Equal(t, SecretNone, c.Secret(nil))

	got, err := c.Normalize(nil)
	require.NoError(t, err)
	assert.JSONEq(t, `{"list":"hottest","tag":"","limit":25}`, string(got))

	got, err = c.Normalize(json.RawMessage(`{"tag":"  #C++ "}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"list":"hottest","tag":"c++","limit":25}`, string(got))

	for _, raw := range []string{
		`{"list":"top"}`, `{"tag":"has space"}`, `{"tag":"abcdefghijklmnopqrstuvwxyz"}`,
		`{"limit":26}`, `{"limit":-1}`, `[1]`,
	} {
		_, err := c.Normalize(json.RawMessage(raw))
		require.Error(t, err, raw)
		assert.True(t, IsConfigError(err), raw)
	}
}

func TestLobstersUpstreamError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	_, err := NewLobsters(srv.Client(), srv.URL).Fetch(context.Background(), nil, "")
	require.Error(t, err)
	assert.False(t, IsConfigError(err))
	assert.Contains(t, err.Error(), "500")
}
