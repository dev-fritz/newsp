package services

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dev-fritz/newsp/api/internal/repositories"
	"github.com/dev-fritz/newsp/api/pkg/ai"
)

// Excerpt sizes keep the prompt bounded even with 40 sources.
const (
	excerptWithContent = 1200
	excerptSummaryOnly = 400
)

// maxFallbackTitle bounds a title built from section headings, matching the
// length the schema asks of the model.
const maxFallbackTitle = 120

// reportOutput is the JSON the model returns for a report.
type reportOutput struct {
	Title       string   `json:"title"`
	Summary     string   `json:"summary"`
	WhatChanged []string `json:"what_changed"`
	Sections    []struct {
		Heading string `json:"heading"`
		Body    string `json:"body"`
	} `json:"sections"`
}

var reportSchema = &ai.Schema{Name: "market_report", Schema: json.RawMessage(`{
	"type": "object",
	"properties": {
		"title": {"type": "string", "description": "Headline covering the brief's main themes: the lead story first, then one or two other themes when the brief has them, each with its concrete subject; at most 120 characters; no date and no generic label like 'Market brief'"},
		"summary": {"type": "string", "description": "One or two sentences: the first on the lead story, the second on the other themes of the brief when there are any"},
		"what_changed": {"type": "array", "items": {"type": "string"}, "description": "Up to 4 short bullets on new developments of stories the previous brief covered; empty when there are none"},
		"sections": {
			"type": "array",
			"description": "1 to 5 themes, as many as the new material supports; with 3 or more, the last one is about opportunities for people who build software",
			"items": {
				"type": "object",
				"properties": {
					"heading": {"type": "string"},
					"body": {"type": "string", "description": "1 to 3 Markdown paragraphs with [n] citations, no headings"}
				},
				"required": ["heading", "body"],
				"additionalProperties": false
			}
		}
	},
	"required": ["title", "summary", "what_changed", "sections"],
	"additionalProperties": false
}`)}

// languageName is how prompts name the output language.
func languageName(lang string) string {
	if lang == "en" {
		return "English"
	}
	return "Brazilian Portuguese"
}

func reportSystemPrompt(lang string) string {
	return `You are the market analyst of newsp. You read recent news and community discussions about technology and business and write a short daily market brief for one reader.

Write everything in ` + languageName(lang) + `.

Rules:
- Use only the numbered sources you are given. Cite them inline as [n] right after the claim they support. Never cite a number that is not in the list.
- Do not invent facts, figures, companies or quotes that the sources do not contain. When sources disagree, say so.
- Group related sources into themes; explain what is changing and why it matters, rather than summarizing posts one by one.
- Weigh what the reader cares about (their topics or focus) but report important shifts outside it too.
- Each brief covers only what is new. The reader has already read the previous brief: never retell its stories. When a source brings a new development to one of them, say only what is new, in a sentence that refers to the previous brief.
- When the sources bring little that matters, write a short brief (one or two themes) instead of stretching thin material.
- With three or more themes, the last section is about opportunities for people who build software: unmet needs, recurring complaints, workarounds people describe.
- The title is a headline for the whole brief, not for one section: the lead story first, then one or two other themes, each named by its concrete subject (company, product, trend, event), as a newspaper front page would. Leave out the opportunities section. Never a generic label such as "Market brief", "Daily briefing" or "Resumo do dia", and never a date: the app already shows when the brief was written. Good: "Microsoft offers buyouts to 60,000; GPT-6.1 Terra ships and Apple distrusts eMudhra certificates". Bad: "Microsoft offers buyouts to 60,000 employees" (leaves out the other themes) and "Market Brief - October 26, 2026".
- The summary also covers the whole brief: one sentence on the lead story, one on the other themes when there are any.
- Plain, concrete language. No hype, no filler, no emojis.`
}

// buildReportPrompt renders the reader context and the numbered sources.
func buildReportPrompt(topics []string, focus string, start, end time.Time, prevSummary string, candidates []repositories.Candidate) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Time window: %s to %s (UTC).\n", start.UTC().Format("2006-01-02 15:04"), end.UTC().Format("2006-01-02 15:04"))
	if len(topics) > 0 {
		fmt.Fprintf(&b, "Reader's topics of interest: %s.\n", strings.Join(topics, "; "))
	}
	if focus != "" {
		fmt.Fprintf(&b, "The reader asked this brief to focus on: %s.\n", focus)
	}
	if prevSummary != "" {
		// Only the summary: earlier titles, once generic, were copied as a template.
		fmt.Fprintf(&b, "\nThe previous brief, which the reader has already read, said: %s\n"+
			"Do not repeat its stories; the sources it cited are not in the list below. Mention one of them only for a new development, "+
			"referring to the previous brief. what_changed lists only those new developments.\n", prevSummary)
	} else {
		b.WriteString("\nThere is no previous brief; leave what_changed empty.\n")
	}
	b.WriteString("\nSources:\n")
	for i, c := range candidates {
		fmt.Fprintf(&b, "\n[%d] %s\n", i+1, c.Title)
		fmt.Fprintf(&b, "source: %s (%s) · %s%s\n", c.SourceName, c.SourceKind, c.PublishedAt.UTC().Format("2006-01-02"), engagement(c))
		if excerpt := sourceExcerpt(c); excerpt != "" {
			b.WriteString(excerpt)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func engagement(c repositories.Candidate) string {
	var parts []string
	if c.Score != nil && *c.Score > 0 {
		parts = append(parts, strconv.Itoa(*c.Score)+" points")
	}
	if c.Comments != nil && *c.Comments > 0 {
		parts = append(parts, strconv.Itoa(*c.Comments)+" comments")
	}
	if len(parts) == 0 {
		return ""
	}
	return " · " + strings.Join(parts, ", ")
}

func sourceExcerpt(c repositories.Candidate) string {
	if c.ContentText != nil && *c.ContentText != "" {
		return clip(*c.ContentText, excerptWithContent)
	}
	return clip(c.Summary, excerptSummaryOnly)
}

func clip(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "…"
}

var citationPattern = regexp.MustCompile(`\[(\d{1,3})\]`)

// sanitizeCitations drops references outside 1..n and returns the text and the
// references that remain, in first-use order.
func sanitizeCitations(text string, n int) (string, []int) {
	var used []int
	seen := map[int]bool{}
	clean := citationPattern.ReplaceAllStringFunc(text, func(m string) string {
		ref, _ := strconv.Atoi(m[1 : len(m)-1])
		if ref < 1 || ref > n {
			return ""
		}
		if !seen[ref] {
			seen[ref] = true
			used = append(used, ref)
		}
		return m
	})
	return clean, used
}

// renderReport turns the model output into Markdown and collects citations.
func renderReport(out reportOutput, lang string, sources int) (markdown string, refs []int) {
	var b strings.Builder
	changedHeading := "O que mudou desde o relatório anterior"
	if lang == "en" {
		changedHeading = "What changed since the previous brief"
	}
	if len(out.WhatChanged) > 0 {
		fmt.Fprintf(&b, "### %s\n\n", changedHeading)
		for _, c := range out.WhatChanged[:min(len(out.WhatChanged), 4)] {
			fmt.Fprintf(&b, "- %s\n", strings.TrimSpace(c))
		}
		b.WriteByte('\n')
	}
	for i, s := range out.Sections {
		fmt.Fprintf(&b, "## %d. %s\n\n%s\n\n", i+1, strings.TrimSpace(s.Heading), strings.TrimSpace(s.Body))
	}
	return sanitizeCitations(strings.TrimSpace(b.String()), sources)
}

// validate checks the parts the schema cannot express.
var (
	titleDatePattern = regexp.MustCompile(`(?i)\b\d{1,2}\s+de\s+(janeiro|fevereiro|março|abril|maio|junho|julho|agosto|setembro|outubro|novembro|dezembro)|` +
		`\b(january|february|march|april|may|june|july|august|september|october|november|december)\s+\d{1,2}\b|` +
		`\b\d{1,2}\s+(january|february|march|april|may|june|july|august|september|october|november|december)\b|` +
		`\b\d{4}-\d{2}-\d{2}\b|\b\d{1,2}/\d{1,2}(/\d{2,4})?\b`)
	titleLabelPattern = regexp.MustCompile(`(?i)\b(briefing|boletim|newsletter|digest|roundup|market brief|daily brief|market report|` +
		`resumo (do dia|diário|de mercado|semanal)|relatório (do dia|diário|de mercado|semanal)|panorama (do dia|diário|de mercado))`)
)

// genericTitle reports whether a title is a newsletter label or a date rather
// than a headline about the content ("Briefing de Mercado - 26 de Outubro").
func genericTitle(title string) bool {
	return titleDatePattern.MatchString(title) || titleLabelPattern.MatchString(title)
}

// reportTitle is the model's title, or, when it is a generic label or a date,
// the headings of the first two themes, which name what the brief covers. The
// last section (opportunities) is left out. Not a validation error: a second
// answer is not guaranteed and must not fail the report.
func reportTitle(o reportOutput) string {
	title := strings.TrimSpace(o.Title)
	if !genericTitle(title) || len(o.Sections) == 0 {
		return title
	}
	themes := o.Sections
	if len(themes) > 2 {
		themes = themes[:len(themes)-1]
	}
	var headings []string
	for _, s := range themes {
		if heading := strings.TrimSpace(s.Heading); heading != "" && !genericTitle(heading) {
			headings = append(headings, heading)
		}
		if len(headings) == 2 {
			break
		}
	}
	if len(headings) == 0 {
		return title
	}
	if joined := strings.Join(headings, "; "); utf8.RuneCountInString(joined) <= maxFallbackTitle {
		return joined
	}
	return headings[0]
}

func (o *reportOutput) validate(sources int) error {
	switch {
	case strings.TrimSpace(o.Title) == "":
		return fmt.Errorf("title must not be empty")
	case strings.TrimSpace(o.Summary) == "":
		return fmt.Errorf("summary must not be empty")
	case len(o.Sections) == 0:
		return fmt.Errorf("write at least 1 section")
	}
	for _, s := range o.Sections {
		if strings.TrimSpace(s.Heading) == "" || strings.TrimSpace(s.Body) == "" {
			return fmt.Errorf("every section needs a heading and a body")
		}
	}
	_, refs := renderReport(*o, "", sources)
	if len(refs) == 0 {
		return fmt.Errorf("cite the sources inline as [n], using only numbers from 1 to %d", sources)
	}
	return nil
}
