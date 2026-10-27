package extract

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dev-fritz/newsp/api/pkg/safehttp"
)

func TestFromHTML(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("testdata/article.html")
	require.NoError(t, err)
	u, _ := url.Parse("https://blog.example.com/ai-review")

	text, err := FromHTML(body, u)
	require.NoError(t, err)
	assert.Contains(t, text, "Teams are adopting AI reviewers")
	assert.Contains(t, text, "approved by tech leads")
	assert.NotContains(t, text, "tracking()")
	assert.NotContains(t, text, "Copyright footer")
}

func TestFromHTMLRejectsEmptyPages(t *testing.T) {
	t.Parallel()
	u, _ := url.Parse("https://example.com")
	_, err := FromHTML([]byte(`<html><body><div id="root"></div></body></html>`), u)
	assert.ErrorIs(t, err, ErrNotArticle)
}

func TestTextDownloadsWithSafeClient(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("testdata/article.html")
	require.NoError(t, err)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) }))
	defer srv.Close()

	text, err := New(safehttp.NewClient(safehttp.Options{AllowPrivate: true})).Text(context.Background(), srv.URL)
	require.NoError(t, err)
	assert.Contains(t, text, "AI reviewers")

	_, err = New(safehttp.NewClient(safehttp.Options{})).Text(context.Background(), srv.URL)
	assert.ErrorIs(t, err, safehttp.ErrBlockedAddress, "the production client refuses local addresses")

	_, err = New(safehttp.NewClient(safehttp.Options{})).Text(context.Background(), "file:///etc/passwd")
	assert.Error(t, err)
}
