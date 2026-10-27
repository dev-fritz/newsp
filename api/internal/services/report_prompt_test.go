package services

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dev-fritz/newsp/api/internal/repositories"
	"github.com/dev-fritz/newsp/api/pkg/ai"
)

func TestSanitizeCitations(t *testing.T) {
	t.Parallel()
	text, refs := sanitizeCitations("A [2] and B [1][2], bogus [9] and [0].", 3)
	assert.Equal(t, "A [2] and B [1][2], bogus  and .", text)
	assert.Equal(t, []int{2, 1}, refs, "first-use order, no duplicates")
}

func sampleOutput() reportOutput {
	var out reportOutput
	out.Title, out.Summary = "Agents move into code review", "Teams use agents as first reviewers."
	out.WhatChanged = []string{"New: observability launches"}
	out.Sections = append(out.Sections,
		struct {
			Heading string `json:"heading"`
			Body    string `json:"body"`
		}{"Review becomes process", "Teams report daily use [1][3]."},
		struct {
			Heading string `json:"heading"`
			Body    string `json:"body"`
		}{"Signals for builders", "People want cost per feature [2]."},
	)
	return out
}

func TestRenderReport(t *testing.T) {
	t.Parallel()
	md, refs := renderReport(sampleOutput(), "pt-BR", 3)
	assert.True(t, strings.HasPrefix(md, "### O que mudou desde o relatório anterior"))
	assert.Contains(t, md, "## 1. Review becomes process")
	assert.Contains(t, md, "## 2. Signals for builders")
	assert.Equal(t, []int{1, 3, 2}, refs)

	en, _ := renderReport(sampleOutput(), "en", 3)
	assert.Contains(t, en, "### What changed since the previous brief")
}

func TestReportOutputValidate(t *testing.T) {
	t.Parallel()
	out := sampleOutput()
	require.NoError(t, out.validate(3))

	noCitations := sampleOutput()
	noCitations.Sections[0].Body, noCitations.Sections[1].Body = "No refs.", "Still none [7]."
	assert.ErrorContains(t, noCitations.validate(3), "cite the sources")

	oneSection := sampleOutput()
	oneSection.Sections = oneSection.Sections[:1]
	require.NoError(t, oneSection.validate(3), "a short brief with a single theme is fine")

	noSections := sampleOutput()
	noSections.Sections = nil
	assert.ErrorContains(t, noSections.validate(3), "at least 1 section")

	empty := sampleOutput()
	empty.Title = " "
	assert.Error(t, empty.validate(3))
}

func TestBuildReportPrompt(t *testing.T) {
	t.Parallel()
	text := "Full article text"
	score := 412
	candidates := []repositories.Candidate{
		{ItemID: uuid.New(), Title: "Ask HN: AI review?", SourceName: "Hacker News", SourceKind: "hackernews", PublishedAt: time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC), Score: &score, ContentText: &text},
		{ItemID: uuid.New(), Title: "Feed post", SourceName: "BBC", SourceKind: "rss", PublishedAt: time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC), Summary: "Short summary"},
	}
	prompt := buildReportPrompt([]string{"AI agents"}, "fintech", time.Date(2026, 10, 25, 8, 0, 0, 0, time.UTC), time.Date(2026, 10, 26, 8, 0, 0, 0, time.UTC), "Old summary", candidates)
	assert.Contains(t, prompt, "[1] Ask HN: AI review?")
	assert.Contains(t, prompt, "412 points")
	assert.Contains(t, prompt, "Full article text", "extracted text wins over the summary")
	assert.Contains(t, prompt, "[2] Feed post")
	assert.Contains(t, prompt, "Short summary")
	assert.Contains(t, prompt, "focus on: fintech")
	assert.Contains(t, prompt, "The previous brief, which the reader has already read, said: Old summary")
	assert.Contains(t, prompt, "Do not repeat its stories")

	noPrev := buildReportPrompt(nil, "", time.Now(), time.Now(), "", candidates)
	assert.Contains(t, noPrev, "leave what_changed empty")
	assert.Contains(t, reportSystemPrompt("en"), "Write everything in English")
	assert.Contains(t, reportSystemPrompt("pt-BR"), "Brazilian Portuguese")
	assert.Contains(t, reportSystemPrompt("pt-BR"), "never a date", "the title names the content")
	assert.Contains(t, reportSystemPrompt("pt-BR"), "headline for the whole brief, not for one section", "the title covers several themes")
	assert.Contains(t, reportSystemPrompt("pt-BR"), "one on the other themes", "the summary covers several themes")
	assert.Contains(t, reportSystemPrompt("pt-BR"), "never retell its stories", "a brief does not repeat the previous one")
	assert.Contains(t, reportSystemPrompt("pt-BR"), "write a short brief", "little new material makes a short brief")
	assert.Contains(t, string(reportSchema.Schema), "1 to 5 themes")
	assert.Contains(t, string(reportSchema.Schema), "then one or two other themes")
}

func TestRetryableReportError(t *testing.T) {
	t.Parallel()
	assert.True(t, retryableReportError("", errors.New("db down")))
	assert.True(t, retryableReportError(ReportErrAIFailure, &ai.APIError{Status: 529}))
	assert.False(t, retryableReportError(ReportErrAIFailure, &ai.APIError{Status: 400}))
	assert.False(t, retryableReportError(ReportErrNoItems, errors.New("no items")))
	assert.False(t, retryableReportError(ReportErrRefused, ai.ErrRefused))
}

func TestGenericTitle(t *testing.T) {
	t.Parallel()
	for _, title := range []string{
		"Briefing de Mercado - 26 de Outubro de 2026",
		"Briefing Diário - 26 de Outubro de 2026",
		"Resumo do dia",
		"Market Brief - October 26, 2026",
		"Relatório de mercado 26/10",
		"Panorama de mercado 2026-10-26",
	} {
		assert.True(t, genericTitle(title), title)
	}
	for _, title := range []string{
		"Agentes de código entram no fluxo de revisão dos times",
		"Anthropic prepara IPO e 3 de cada 4 devs já usam IA",
		"Code agents move into teams' review workflow",
	} {
		assert.False(t, genericTitle(title), title)
	}
}

func TestReportTitleFallsBackToTheMainThemes(t *testing.T) {
	t.Parallel()
	section := func(heading string) struct {
		Heading string `json:"heading"`
		Body    string `json:"body"`
	} {
		return struct {
			Heading string `json:"heading"`
			Body    string `json:"body"`
		}{heading, "Body [1]."}
	}
	out := sampleOutput()
	out.Title = "Briefing de Mercado - 26 de Outubro de 2026"
	out.Sections = append(out.Sections[:0],
		section("Microsoft oferece saída voluntária a 60 mil"),
		section("GPT-6.1 Terra e novas ferramentas de agentes"),
		section("Apple bloqueia certificados da eMudhra"),
		section("Oportunidades para desenvolvedores"),
	)
	assert.Equal(t, "Microsoft oferece saída voluntária a 60 mil; GPT-6.1 Terra e novas ferramentas de agentes", reportTitle(out),
		"a brief with several themes is not titled after the first one only")

	out.Sections[0].Heading = "Briefing"
	assert.Equal(t, "GPT-6.1 Terra e novas ferramentas de agentes; Apple bloqueia certificados da eMudhra", reportTitle(out),
		"generic headings are skipped")

	out.Sections = out.Sections[2:]
	assert.Equal(t, "Apple bloqueia certificados da eMudhra; Oportunidades para desenvolvedores", reportTitle(out),
		"with two sections both are themes")

	out.Sections[0].Heading, out.Sections[1].Heading = strings.Repeat("a", 70), strings.Repeat("b", 70)
	assert.Equal(t, strings.Repeat("a", 70), reportTitle(out), "too long to join: keep the first theme")

	out.Title = "  IPO da Anthropic domina as discussões; GPT-6.1 Terra chega "
	assert.Equal(t, "IPO da Anthropic domina as discussões; GPT-6.1 Terra chega", reportTitle(out), "a real headline is kept")

	out.Title, out.Sections = "Resumo do dia", out.Sections[:1]
	out.Sections[0].Heading = "Briefing"
	assert.Equal(t, "Resumo do dia", reportTitle(out), "no better candidate: keep what the model wrote")
}
