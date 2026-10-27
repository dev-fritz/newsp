package services

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dev-fritz/newsp/api/internal/models"
)

func changeFixture() (IdeaVersionDetail, ideaText) {
	effort := 6
	content := models.IdeaContent{
		WhyNow: "Dor real [E1].", How: "GitHub App  que   indexa revisões.",
		Audience:    []models.AudienceSegment{{Kind: models.AudiencePrimary, Label: "Tech leads"}},
		MVPPlan:     []models.PlanStep{{Period: "S1", Title: "a"}, {Period: "S2", Title: "b"}, {Period: "S3", Title: "c"}},
		Competitors: []models.Competitor{},
	}
	current := IdeaVersionDetail{
		Version: models.IdeaVersion{Title: "Revisor", Summary: "Resumo.", Difficulty: models.DifficultyIntermediate, EffortWeeks: &effort},
		Content: content,
	}
	revised := ideaText{Title: "Revisor", Summary: "Resumo.", Difficulty: models.DifficultyIntermediate, EffortWeeks: 6, Content: content}
	return current, revised
}

func TestUnchangedIgnoresWhitespaceAndCitations(t *testing.T) {
	t.Parallel()
	current, revised := changeFixture()
	// What the real model did: collapsed spaces and cited one more source.
	revised.Content.How = "GitHub App que indexa revisões."
	revised.Content.WhyNow = "Dor real [E1][E2]."
	revised.Content.Competitors = nil
	assert.True(t, unchanged(current, revised))
}

func TestUnchangedSeesRealChanges(t *testing.T) {
	t.Parallel()
	cases := map[string]func(*ideaText){
		"new plan step": func(r *ideaText) {
			r.Content.MVPPlan = append(r.Content.MVPPlan, models.PlanStep{Period: "S4", Title: "d"})
		},
		"new competitor": func(r *ideaText) { r.Content.Competitors = []models.Competitor{{Name: "X"}} },
		"edited text":    func(r *ideaText) { r.Content.How += " Com cache." },
		"effort":         func(r *ideaText) { r.EffortWeeks = 8 },
		"title":          func(r *ideaText) { r.Title = "Revisor de PR" },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			current, revised := changeFixture()
			change(&revised)
			assert.False(t, unchanged(current, revised))
		})
	}
}
