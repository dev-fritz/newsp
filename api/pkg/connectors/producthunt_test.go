package connectors

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProductHuntFetch(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("testdata/producthunt.json")
	require.NoError(t, err)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v2/api/graphql", r.URL.Path)
		assert.Empty(t, r.URL.RawQuery)
		assert.Equal(t, "Bearer ph-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		raw, _ := io.ReadAll(r.Body)
		var req struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		require.NoError(t, json.Unmarshal(raw, &req))
		assert.Equal(t, productHuntQuery, req.Query)
		assert.Equal(t, map[string]any{
			"first":       float64(15),
			"order":       "VOTES",
			"postedAfter": "2026-10-24T15:00:00Z",
			"topic":       "artificial-intelligence",
		}, req.Variables)
		assert.NotContains(t, string(raw), "ph-token")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	cfg := `{"order":"votes","topic":"artificial-intelligence","posted_within_days":2,"limit":15}`
	got, err := NewProductHunt(srv.Client(), srv.URL, fixedNow).Fetch(context.Background(), json.RawMessage(cfg), "ph-token")
	require.NoError(t, err)
	require.Len(t, got, 2, "nodes without a name are skipped")

	first := got[0]
	assert.Equal(t, "700001", first.ExternalID)
	assert.Equal(t, "https://www.producthunt.com/posts/skybound", first.URL)
	assert.Equal(t, "Skybound — Plan trips with an AI co-pilot", first.Title)
	assert.Equal(t, "Skybound builds itineraries from a single prompt.", first.Summary)
	assert.Empty(t, first.Author)
	assert.Equal(t, fixedNow().Add(-8*time.Hour).Add(time.Minute).UTC(), first.PublishedAt)
	assert.Equal(t, 321, *first.Score)
	assert.Equal(t, 24, *first.Comments)
	assert.Equal(t, "https://skybound.example/?ref=producthunt", first.Metrics["website"])
	assert.Equal(t, []string{"Travel", "Artificial Intelligence"}, first.Metrics["topics"])

	second := got[1]
	assert.Equal(t, "Tagless", second.Title)
	assert.Empty(t, second.Summary)
}

func TestProductHuntDefaultsOmitTopic(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var req struct {
			Variables map[string]any `json:"variables"`
		}
		require.NoError(t, json.Unmarshal(raw, &req))
		assert.Equal(t, map[string]any{"first": float64(20), "order": "RANKING", "postedAfter": "2026-10-25T15:00:00Z"}, req.Variables)
		_, _ = w.Write([]byte(`{"data":{"posts":{"edges":[]}}}`))
	}))
	defer srv.Close()
	got, err := NewProductHunt(srv.Client(), srv.URL, fixedNow).Fetch(context.Background(), json.RawMessage(`{}`), "tok")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestProductHuntGraphQLError(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("x", 300)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"errors":[{"message":"` + long + `"},{"message":"second"}]}`))
	}))
	defer srv.Close()
	_, err := NewProductHunt(srv.Client(), srv.URL, fixedNow).Fetch(context.Background(), json.RawMessage(`{}`), "tok")
	require.Error(t, err)
	assert.Equal(t, "producthunt: "+strings.Repeat("x", 200), err.Error())
}

func TestProductHuntMissingSecret(t *testing.T) {
	t.Parallel()
	_, err := NewProductHunt(http.DefaultClient, "http://127.0.0.1:0", fixedNow).Fetch(context.Background(), json.RawMessage(`{}`), "")
	require.EqualError(t, err, "producthunt needs a developer token")
}

func TestProductHuntUnauthorizedDoesNotLeakToken(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	_, err := NewProductHunt(srv.Client(), srv.URL, fixedNow).Fetch(context.Background(), json.RawMessage(`{}`), "ph-secret")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
	assert.NotContains(t, err.Error(), "ph-secret")
}

func TestProductHuntNormalize(t *testing.T) {
	t.Parallel()
	c := NewProductHunt(http.DefaultClient, "", fixedNow)
	assert.Equal(t, "producthunt", c.Kind())
	assert.Equal(t, SecretRequired, c.Secret(nil))

	got, err := c.Normalize(json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"order":"ranking","topic":"","posted_within_days":1,"limit":20}`, string(got))

	got, err = c.Normalize(json.RawMessage(`{"order":"newest","topic":"developer-tools","posted_within_days":7,"limit":50}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"order":"newest","topic":"developer-tools","posted_within_days":7,"limit":50}`, string(got))

	for _, raw := range []string{
		`{"order":"random"}`,
		`{"order":"VOTES"}`,
		`{"topic":"Bad Topic"}`,
		`{"topic":"` + strings.Repeat("a", 61) + `"}`,
		`{"posted_within_days":8}`,
		`{"posted_within_days":-1}`,
		`{"limit":51}`,
		`{"limit":-1}`,
		`[1]`,
	} {
		_, err := c.Normalize(json.RawMessage(raw))
		require.Error(t, err, raw)
		assert.True(t, IsConfigError(err), raw)
	}
}
