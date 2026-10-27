// Package extract pulls the readable text out of an article page.
package extract

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	readability "codeberg.org/readeck/go-readability/v2"

	"github.com/dev-fritz/newsp/api/pkg/safehttp"
)

// MaxTextRunes caps the extracted text so prompts stay bounded.
const MaxTextRunes = 12_000

// ErrNotArticle is returned for pages without readable content (videos,
// paywalls, single-page apps).
var ErrNotArticle = errors.New("no readable article content")

// Extractor downloads pages and extracts their main text.
type Extractor struct {
	client *http.Client
}

// New builds an Extractor. Use a safehttp client: article URLs come from feeds.
func New(client *http.Client) *Extractor {
	return &Extractor{client: client}
}

// Text returns the main text of the page at pageURL.
func (e *Extractor) Text(ctx context.Context, pageURL string) (string, error) {
	u, err := url.Parse(pageURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", fmt.Errorf("invalid article url: %q", pageURL)
	}
	header := http.Header{"Accept": {"text/html,application/xhtml+xml"}}
	body, err := safehttp.Get(ctx, e.client, pageURL, 5<<20, header)
	if err != nil {
		return "", fmt.Errorf("download article: %w", err)
	}
	return FromHTML(body, u)
}

// FromHTML extracts the main text from an HTML document.
func FromHTML(body []byte, pageURL *url.URL) (string, error) {
	article, err := readability.FromReader(bytes.NewReader(body), pageURL)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrNotArticle, err)
	}
	if article.Node == nil {
		return "", ErrNotArticle
	}
	var b strings.Builder
	if err := article.RenderText(&b); err != nil {
		return "", fmt.Errorf("render article: %w", err)
	}
	text := strings.Join(strings.Fields(b.String()), " ")
	if utf8.RuneCountInString(text) < 200 {
		return "", ErrNotArticle
	}
	if utf8.RuneCountInString(text) > MaxTextRunes {
		text = string([]rune(text)[:MaxTextRunes])
	}
	return text, nil
}
