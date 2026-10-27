package repositories

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

func TestExcerpt(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", excerpt("  \n "))
	assert.Equal(t, "Uma linha só.", excerpt("Uma   linha\n\nsó."), "whitespace collapses")

	long := strings.Repeat("palavra ", 60)
	got := excerpt(long)
	assert.True(t, strings.HasSuffix(got, "palavra…"), "cut at a word boundary: %q", got)
	assert.LessOrEqual(t, utf8.RuneCountInString(got), excerptRunes+1)

	accents := strings.Repeat("ação ", 80)
	assert.True(t, utf8.ValidString(excerpt(accents)), "never splits a rune")
}
