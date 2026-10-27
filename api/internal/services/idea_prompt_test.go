package services

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/dev-fritz/newsp/api/internal/models"
	"github.com/dev-fritz/newsp/api/internal/repositories"
)

func draftFixture() ideaDraft {
	return ideaDraft{
		Title: "Revisor de PR", Summary: "Resumo [1].", WhyNow: "Dor real [1][2] e ruído [9].", How: "GitHub App [E7].",
		Audience:   []models.AudienceSegment{{Kind: models.AudiencePrimary, Label: "Tech leads"}},
		MVPPlan:    []models.PlanStep{{Period: "S1", Title: "a"}, {Period: "S2", Title: "b"}},
		Difficulty: models.DifficultyIntermediate, EffortWeeks: 6, Evidence: []int{2, 3, 3, 0},
	}
}

func TestEvidenceMapperForNewIdea(t *testing.T) {
	t.Parallel()
	candidates := []repositories.Candidate{{ItemID: uuid.New()}, {ItemID: uuid.New()}, {ItemID: uuid.New()}}
	m := newEvidenceMapper(nil, candidates)
	title, summary, _, effort, content := m.toVersion(draftFixture())

	assert.Equal(t, "Revisor de PR", title)
	assert.Equal(t, 6, effort)
	// evidence [2,3] is registered first, then text citations in order of appearance.
	assert.Equal(t, "Resumo [E3].", summary)
	assert.Equal(t, "Dor real [E3][E1] e ruído .", content.WhyNow, "unknown numbers are dropped")
	assert.Equal(t, "GitHub App .", content.How, "E labels the idea does not have are dropped")
	assert.Equal(t, []models.IdeaEvidence{
		{ItemID: candidates[1].ItemID, Ref: "E1"}, {ItemID: candidates[2].ItemID, Ref: "E2"}, {ItemID: candidates[0].ItemID, Ref: "E3"},
	}, m.evidence)
	assert.NotNil(t, content.Competitors)
}

func TestEvidenceMapperKeepsPreviousEvidence(t *testing.T) {
	t.Parallel()
	shared := uuid.New()
	previous := []repositories.EvidenceItem{{Ref: "E1", ItemID: uuid.New()}, {Ref: "E2", ItemID: shared}}
	candidates := []repositories.Candidate{{ItemID: shared}, {ItemID: uuid.New()}}
	m := newEvidenceMapper(previous, candidates)

	text := m.rewrite("Antes [E1], o mesmo item [1], novo [2], inválido [E9].")
	assert.Equal(t, "Antes [E1], o mesmo item [E2], novo [E3], inválido .", text)
	assert.Len(t, m.evidence, 3, "an item already cited keeps its label")
}

func TestIdeaDraftValidate(t *testing.T) {
	t.Parallel()
	d := draftFixture()
	assert.NoError(t, d.validate(3, true))

	noEvidence := draftFixture()
	noEvidence.Evidence = []int{7}
	assert.ErrorContains(t, noEvidence.validate(3, true), "evidence")
	assert.NoError(t, noEvidence.validate(3, false), "revisions keep earlier evidence")

	twoPrimaries := draftFixture()
	twoPrimaries.Audience = append(twoPrimaries.Audience, models.AudienceSegment{Kind: models.AudiencePrimary})
	assert.ErrorContains(t, twoPrimaries.validate(3, true), "primary")

	shortPlan := draftFixture()
	shortPlan.MVPPlan = shortPlan.MVPPlan[:1]
	assert.ErrorContains(t, shortPlan.validate(3, true), "mvp_plan")

	effort := draftFixture()
	effort.EffortWeeks = 0
	assert.ErrorContains(t, effort.validate(3, true), "effort_weeks")
}

func TestGenerationPromptStartingSources(t *testing.T) {
	t.Parallel()
	candidates := []repositories.Candidate{{Title: "A"}, {Title: "B"}, {Title: "C"}}

	picked := buildGenerationPrompt(1, "pt-BR", nil, "", nil, candidates, startingSources{n: 2, picked: true})
	assert.Contains(t, picked, "Sources [1] and [2] are the ones the reader picked")
	assert.Contains(t, picked, "across the ideas, use all of them")
	assert.Less(t, strings.Index(picked, "[3] C"), strings.Index(picked, "are the ones the reader picked"),
		"the instruction comes after the sources, where small models still see it")

	report := buildGenerationPrompt(1, "pt-BR", nil, "", nil, candidates, startingSources{n: 3})
	assert.Contains(t, report, "Sources [1] to [3] are the ones the report the reader started from cites")

	plain := buildGenerationPrompt(1, "pt-BR", nil, "", nil, candidates, startingSources{})
	assert.NotContains(t, plain, "starting point")
}

func TestCitesStarting(t *testing.T) {
	t.Parallel()
	assert.True(t, citesStarting([]int{5, 2}, 2))
	assert.False(t, citesStarting([]int{3, 4}, 2), "only later sources")
	assert.False(t, citesStarting(nil, 2))
	assert.Equal(t, "[1]", sourceRefs(1))
	assert.Equal(t, "[1] to [4]", sourceRefs(4))
}
