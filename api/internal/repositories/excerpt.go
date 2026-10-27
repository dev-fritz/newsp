package repositories

import (
	"strings"
	"unicode/utf8"
)

// excerptRunes bounds the excerpt shown when a reader hovers a citation.
const excerptRunes = 240

// excerptSQL picks the text an item excerpt comes from: its summary, or the
// extracted article when the source gives no summary. Clipped in SQL first so
// whole articles never leave the database.
const excerptSQL = `left(coalesce(nullif(btrim(i.summary), ''), i.content_text, ''), 1000)`

// excerpt collapses whitespace and cuts the text at a word boundary.
func excerpt(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if utf8.RuneCountInString(text) <= excerptRunes {
		return text
	}
	cut := string([]rune(text)[:excerptRunes])
	if i := strings.LastIndexByte(cut, ' '); i > excerptRunes/2 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " ,;:.-–—") + "…"
}
