package connectors

import (
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
)

// maxSummaryRunes caps summaries; the full text is fetched later only for items
// that end up in a report or idea.
const maxSummaryRunes = 1000

// StripHTML returns the visible text of an HTML fragment with collapsed spaces.
func StripHTML(s string) string {
	if !strings.ContainsAny(s, "<&") {
		return collapseSpaces(s)
	}
	z := html.NewTokenizer(strings.NewReader(s))
	var b strings.Builder
	skip := 0 // inside <script> or <style>
	for {
		switch z.Next() {
		case html.ErrorToken: // end of input (or malformed tail): keep what we have
			return collapseSpaces(b.String())
		case html.StartTagToken:
			name, _ := z.TagName()
			switch string(name) {
			case "script", "style":
				skip++
			case "br", "p", "div", "li":
				b.WriteByte(' ')
			}
		case html.EndTagToken:
			name, _ := z.TagName()
			if n := string(name); (n == "script" || n == "style") && skip > 0 {
				skip--
			}
		case html.TextToken:
			if skip == 0 {
				b.Write(z.Text())
			}
		}
	}
}

func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// truncate cuts s to maxSummaryRunes on a rune boundary, adding an ellipsis.
func truncate(s string) string {
	if utf8.RuneCountInString(s) <= maxSummaryRunes {
		return s
	}
	r := []rune(s)
	return strings.TrimSpace(string(r[:maxSummaryRunes])) + "…"
}

// summarize converts an HTML or plain fragment into a bounded summary.
func summarize(s string) string {
	return truncate(StripHTML(s))
}

// maxTitleRunes caps titles derived from post text.
const maxTitleRunes = 120

// titleFromText builds a title for posts that have none (Mastodon, Bluesky):
// the plain text cut at maxTitleRunes on a word boundary.
func titleFromText(s string) string {
	s = collapseSpaces(s)
	if utf8.RuneCountInString(s) <= maxTitleRunes {
		return s
	}
	cut := string([]rune(s)[:maxTitleRunes])
	if i := strings.LastIndexByte(cut, ' '); i > len(cut)/2 {
		cut = cut[:i]
	}
	return strings.TrimSpace(cut) + "…"
}
