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

const testChannelID = "UCabcdefghijklmnopqrstuv"

func TestYouTubeFetch(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("testdata/youtube.xml")
	require.NoError(t, err)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/feeds/videos.xml", r.URL.Path)
		assert.Equal(t, testChannelID, r.URL.Query().Get("channel_id"))
		w.Header().Set("Content-Type", "application/atom+xml")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	got, err := NewYouTube(srv.Client(), srv.URL, clockNow).Fetch(context.Background(), json.RawMessage(`{"channel_id":"`+testChannelID+`"}`), "")
	require.NoError(t, err)
	require.Len(t, got, 2)

	first := got[0]
	assert.Equal(t, "Step by step walkthrough.", first.Summary, "media:description fills the summary")
	assert.Equal(t, "yt:video:AAAAAAAAAAA", first.ExternalID)
	assert.Equal(t, "https://www.youtube.com/watch?v=AAAAAAAAAAA", first.URL)
	assert.Equal(t, "Building a RAG pipeline from scratch", first.Title)
	assert.Equal(t, "Example Dev Channel", first.Author)
	assert.Equal(t, time.Date(2026, 10, 25, 15, 0, 0, 0, time.UTC), first.PublishedAt)
	for _, it := range got {
		assert.Equal(t, testChannelID, it.Metrics["channel_id"])
		assert.Equal(t, "Example Dev Channel", it.Metrics["feed_title"])
	}
}

func TestYouTubeLimit(t *testing.T) {
	t.Parallel()
	srv := serveFile(t, "testdata/youtube.xml", "application/atom+xml")
	got, err := NewYouTube(srv.Client(), srv.URL, clockNow).Fetch(context.Background(), json.RawMessage(`{"channel_id":"`+testChannelID+`","limit":1}`), "")
	require.NoError(t, err)
	assert.Len(t, got, 1)
}

func TestYouTubeNormalize(t *testing.T) {
	t.Parallel()
	c := NewYouTube(http.DefaultClient, "", clockNow)
	assert.Equal(t, "youtube", c.Kind())
	got, err := c.Normalize(json.RawMessage(`{"channel_id":" ` + testChannelID + ` "}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"channel_id":"`+testChannelID+`","limit":15}`, string(got))

	for _, raw := range []string{
		`{}`,
		`{"channel_id":"abcdefghijklmnopqrstuvwx"}`,
		`{"channel_id":"UCshort"}`,
		`{"channel_id":"UCabcdefghijklmnopqrstuvw"}`,
		`{"channel_id":"UCabcdefghijklmnopqrstu!"}`,
		`{"channel_id":"` + testChannelID + `","limit":51}`,
		`{"channel_id":"` + testChannelID + `","limit":-1}`,
		`[1]`,
	} {
		_, err := c.Normalize(json.RawMessage(raw))
		require.Error(t, err, raw)
		assert.True(t, IsConfigError(err), raw)
	}
}

func TestYouTubeUpstreamError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.NotFound(w, nil)
	}))
	defer srv.Close()
	_, err := NewYouTube(srv.Client(), srv.URL, clockNow).Fetch(context.Background(), json.RawMessage(`{"channel_id":"`+testChannelID+`"}`), "")
	require.Error(t, err)
	assert.False(t, IsConfigError(err))
}
