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

func TestTabNewsFetch(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("testdata/tabnews.json")
	require.NoError(t, err)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/contents", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "1", q.Get("page"))
		assert.Equal(t, "10", q.Get("per_page"))
		assert.Equal(t, "new", q.Get("strategy"))
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	got, err := NewTabNews(srv.Client(), srv.URL).Fetch(context.Background(), json.RawMessage(`{"strategy":"new","limit":10}`), "")
	require.NoError(t, err)
	require.Len(t, got, 2, "entries without title or slug are skipped")

	first := got[0]
	assert.Equal(t, "6f1c1f0e-1a2b-4c3d-8e9f-0a1b2c3d4e5f", first.ExternalID)
	assert.Equal(t, "https://www.tabnews.com.br/mariasilva/como-reduzimos-o-custo-de-inferencia", first.URL)
	assert.Equal(t, "Como reduzimos o custo de inferência em 70%", first.Title)
	assert.Equal(t, "mariasilva", first.Author)
	assert.Equal(t, time.Date(2026, 10, 25, 14, 30, 0, 0, time.UTC), first.PublishedAt)
	assert.Equal(t, 42, *first.Score)
	assert.Equal(t, 7, *first.Comments)
	assert.Equal(t, map[string]any{"source_url": "https://example.com/inferencia"}, first.Metrics)
	assert.NotEmpty(t, first.Summary)

	second := got[1]
	assert.Equal(t, time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC), second.PublishedAt, "converted to UTC")
	assert.Empty(t, second.Summary, "list responses may omit the body")
	assert.Nil(t, second.Metrics, "no source_url, no metrics")
}

func TestTabNewsNormalize(t *testing.T) {
	t.Parallel()
	c := NewTabNews(http.DefaultClient, "")
	assert.Equal(t, "tabnews", c.Kind())
	got, err := c.Normalize(nil)
	require.NoError(t, err)
	assert.JSONEq(t, `{"strategy":"relevant","limit":30}`, string(got))

	for _, raw := range []string{`{"strategy":"top"}`, `{"limit":101}`, `{"limit":-1}`, `[1]`} {
		_, err := c.Normalize(json.RawMessage(raw))
		require.Error(t, err, raw)
		assert.True(t, IsConfigError(err), raw)
	}
}

func TestTabNewsUpstreamError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	_, err := NewTabNews(srv.Client(), srv.URL).Fetch(context.Background(), nil, "")
	require.Error(t, err)
	assert.False(t, IsConfigError(err))
	assert.Contains(t, err.Error(), "503")
}
