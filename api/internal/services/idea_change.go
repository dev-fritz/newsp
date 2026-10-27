package services

import (
	"reflect"
	"regexp"
	"strings"

	"github.com/dev-fritz/newsp/api/internal/models"
)

// ideaText is the reader-visible text of a version, used to tell whether a
// revision changed anything.
type ideaText struct {
	Title, Summary string
	Difficulty     models.Difficulty
	EffortWeeks    int
	Content        models.IdeaContent
}

// unchanged reports whether a revision leaves the idea as it was: the same
// text once whitespace and citation labels are ignored. New evidence alone
// is no change.
func unchanged(current IdeaVersionDetail, revised ideaText) bool {
	effort := 0
	if current.Version.EffortWeeks != nil {
		effort = *current.Version.EffortWeeks
	}
	before := ideaText{
		Title: current.Version.Title, Summary: current.Version.Summary, Difficulty: current.Version.Difficulty,
		EffortWeeks: effort, Content: current.Content,
	}
	return reflect.DeepEqual(before.normalized(), revised.normalized())
}

func (t ideaText) normalized() ideaText {
	c := t.Content
	out := models.IdeaContent{
		WhyNow: plainText(c.WhyNow), How: plainText(c.How), Differentiator: plainText(c.Differentiator),
		Monetization: plainText(c.Monetization),
		Stack:        mapSlice(c.Stack, plainText), ArchitectureSteps: mapSlice(c.ArchitectureSteps, plainText),
		Audience: mapSlice(c.Audience, func(a models.AudienceSegment) models.AudienceSegment {
			return models.AudienceSegment{Kind: a.Kind, Label: plainText(a.Label), Description: plainText(a.Description)}
		}),
		MVPPlan: mapSlice(c.MVPPlan, func(p models.PlanStep) models.PlanStep {
			return models.PlanStep{Period: plainText(p.Period), Title: plainText(p.Title), Description: plainText(p.Description)}
		}),
		Competitors: mapSlice(c.Competitors, func(p models.Competitor) models.Competitor {
			return models.Competitor{Name: plainText(p.Name), Strengths: plainText(p.Strengths), Gap: plainText(p.Gap)}
		}),
	}
	return ideaText{
		Title: plainText(t.Title), Summary: plainText(t.Summary), Difficulty: t.Difficulty, EffortWeeks: t.EffortWeeks, Content: out,
	}
}

// citationLabel is an [En] label with the space before it.
var citationLabel = regexp.MustCompile(`\s*\[E\d{1,3}\]`)

// plainText drops citation labels and collapses whitespace.
func plainText(s string) string {
	return strings.Join(strings.Fields(citationLabel.ReplaceAllString(s, "")), " ")
}

// mapSlice maps a slice; nil and empty both become empty so they compare equal.
func mapSlice[T any](in []T, f func(T) T) []T {
	out := make([]T, 0, len(in))
	for _, v := range in {
		out = append(out, f(v))
	}
	return out
}
