package services

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/dev-fritz/newsp/api/internal/models"
	"github.com/dev-fritz/newsp/api/internal/repositories"
	"github.com/dev-fritz/newsp/api/pkg/ai"
)

// ideaDraft is one idea as the model writes it.
type ideaDraft struct {
	Title             string                   `json:"title"`
	Summary           string                   `json:"summary"`
	WhyNow            string                   `json:"why_now"`
	Audience          []models.AudienceSegment `json:"audience"`
	How               string                   `json:"how"`
	Differentiator    string                   `json:"differentiator"`
	Stack             []string                 `json:"stack"`
	ArchitectureSteps []string                 `json:"architecture_steps"`
	MVPPlan           []models.PlanStep        `json:"mvp_plan"`
	Monetization      string                   `json:"monetization"`
	Competitors       []models.Competitor      `json:"competitors"`
	Difficulty        models.Difficulty        `json:"difficulty"`
	EffortWeeks       int                      `json:"effort_weeks"`
	Evidence          []int                    `json:"evidence"`
}

// ideaProperties is the JSON Schema of one idea, shared by every idea prompt.
const ideaProperties = `{
	"type": "object",
	"properties": {
		"title": {"type": "string", "description": "Specific name of the product, at most 80 characters"},
		"summary": {"type": "string", "description": "One or two sentences: what it is and the outcome it delivers"},
		"why_now": {"type": "string", "description": "The market need and why it is timely, grounded in the sources with inline citations"},
		"audience": {"type": "array", "description": "Who it is for: exactly one primary segment, optional secondary ones, and who it is not for", "items": {
			"type": "object",
			"properties": {
				"kind": {"type": "string", "enum": ["primary", "secondary", "out_of_scope"]},
				"label": {"type": "string"},
				"description": {"type": "string"}
			},
			"required": ["kind", "label", "description"], "additionalProperties": false}},
		"how": {"type": "string", "description": "How to build it: the approach and the key workflow, in Markdown"},
		"differentiator": {"type": "string", "description": "Why it wins against the alternatives"},
		"stack": {"type": "array", "items": {"type": "string"}, "description": "Suggested technologies"},
		"architecture_steps": {"type": "array", "items": {"type": "string"}, "description": "Main components in data-flow order"},
		"mvp_plan": {"type": "array", "description": "3 to 6 steps to a usable MVP", "items": {
			"type": "object",
			"properties": {
				"period": {"type": "string", "description": "e.g. Weeks 1-2"},
				"title": {"type": "string"},
				"description": {"type": "string"}
			},
			"required": ["period", "title", "description"], "additionalProperties": false}},
		"monetization": {"type": "string"},
		"competitors": {"type": "array", "items": {
			"type": "object",
			"properties": {"name": {"type": "string"}, "strengths": {"type": "string"}, "gap": {"type": "string"}},
			"required": ["name", "strengths", "gap"], "additionalProperties": false}},
		"difficulty": {"type": "string", "enum": ["beginner", "intermediate", "advanced"]},
		"effort_weeks": {"type": "integer", "description": "Weeks for one developer to reach the MVP"},
		"evidence": {"type": "array", "items": {"type": "integer"}, "description": "Numbers of the new sources that support the idea"}
	},
	"required": ["title", "summary", "why_now", "audience", "how", "differentiator", "stack", "architecture_steps", "mvp_plan", "monetization", "competitors", "difficulty", "effort_weeks", "evidence"],
	"additionalProperties": false
}`

var (
	generationSchema = &ai.Schema{Name: "project_ideas", Schema: json.RawMessage(`{
		"type": "object",
		"properties": {"ideas": {"type": "array", "items": ` + ideaProperties + `}},
		"required": ["ideas"], "additionalProperties": false}`)}
	revisionSchema = &ai.Schema{Name: "idea_revision", Schema: json.RawMessage(`{
		"type": "object",
		"properties": {
			"idea": ` + ideaProperties + `,
			"change_note": {"type": "string", "description": "One or two sentences on what changed and why"}
		},
		"required": ["idea", "change_note"], "additionalProperties": false}`)}
	judgeSchema = &ai.Schema{Name: "same_idea", Schema: json.RawMessage(`{
		"type": "object",
		"properties": {
			"match": {"type": "integer", "description": "Number of the existing idea that is the same idea, or 0"},
			"reason": {"type": "string"}
		},
		"required": ["match", "reason"], "additionalProperties": false}`)}
)

type generationOutput struct {
	Ideas []ideaDraft `json:"ideas"`
}

type revisionOutput struct {
	Idea       ideaDraft `json:"idea"`
	ChangeNote string    `json:"change_note"`
}

type judgeOutput struct {
	Match  int    `json:"match"`
	Reason string `json:"reason"`
}

// validate checks what the schema cannot. requireEvidence is false for
// revisions, which keep their earlier evidence.
func (d *ideaDraft) validate(sources int, requireEvidence bool) error {
	switch {
	case strings.TrimSpace(d.Title) == "" || len([]rune(d.Title)) > 120:
		return fmt.Errorf("title must have 1 to 120 characters")
	case strings.TrimSpace(d.Summary) == "", strings.TrimSpace(d.WhyNow) == "", strings.TrimSpace(d.How) == "":
		return fmt.Errorf("summary, why_now and how must not be empty")
	case len(d.MVPPlan) < 2:
		return fmt.Errorf("mvp_plan needs at least 2 steps")
	case d.EffortWeeks < 1 || d.EffortWeeks > 104:
		return fmt.Errorf("effort_weeks must be between 1 and 104")
	}
	primary := 0
	for _, a := range d.Audience {
		if a.Kind == models.AudiencePrimary {
			primary++
		}
	}
	if primary != 1 {
		return fmt.Errorf("audience must have exactly one primary segment")
	}
	if requireEvidence && len(validRefs(d.Evidence, sources)) == 0 {
		return fmt.Errorf("evidence must list at least one source number between 1 and %d", sources)
	}
	return nil
}

func validRefs(refs []int, n int) []int {
	var out []int
	seen := map[int]bool{}
	for _, r := range refs {
		if r >= 1 && r <= n && !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	return out
}

func ideaSystemPrompt(lang string) string {
	return `You are the product strategist of newsp. From recent market signals you propose software projects that a solo developer or a small team can build.

Write everything in ` + languageName(lang) + `.

Every idea must answer concretely:
- Why now: the market need, grounded in the numbered sources. Cite them inline as [n] right after each claim; never cite numbers that are not listed.
- For whom: who uses it and who pays, named precisely (role, company size, situation), and who it is not for.
- How: the approach, the key workflow, a realistic stack and architecture, and an MVP plan reachable in under 12 weeks when possible.

Rules:
- Do not invent facts or numbers. Mention competitors only when you are confident they exist; otherwise describe the category. Always say what gap they leave.
- Ideas must be different from each other and from the reader's existing ideas. A new name for the same product is not a new idea.
- Plain, specific language. No hype, no emojis.`
}

// startingSources are the first numbered sources of a generation prompt that the
// ideas must start from: the items the reader picked, or the ones a report cites.
type startingSources struct {
	n      int
	picked bool
}

func buildGenerationPrompt(count int, lang string, topics []string, focus string, existing []models.Idea, candidates []repositories.Candidate,
	start startingSources,
) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Propose exactly %d project ideas.\n", count)
	if len(topics) > 0 {
		fmt.Fprintf(&b, "Reader's topics of interest: %s.\n", strings.Join(topics, "; "))
	}
	if focus != "" {
		fmt.Fprintf(&b, "The reader asked for ideas about: %s.\n", focus)
	}
	if len(existing) > 0 {
		b.WriteString("\nThe reader already has these ideas. Do not propose them again, nor close variants:\n")
		for _, idea := range existing {
			fmt.Fprintf(&b, "- %s — %s\n", idea.Title, clip(idea.Summary, 200))
		}
	}
	writeSources(&b, candidates)
	writeStarting(&b, start)
	// Repeated after the sources: smaller models lose an instruction that only
	// appears before thousands of tokens of context.
	fmt.Fprintf(&b, "\nReturn exactly %d distinct ideas in the \"ideas\" array. Write every field, titles included, in %s, even when the sources are in another language.\n", count, languageName(lang))
	return b.String()
}

func writeSources(b *strings.Builder, candidates []repositories.Candidate) {
	if len(candidates) == 0 {
		return
	}
	b.WriteString("\nSources:\n")
	for i, c := range candidates {
		fmt.Fprintf(b, "\n[%d] %s\n", i+1, c.Title)
		fmt.Fprintf(b, "source: %s (%s) · %s%s\n", c.SourceName, c.SourceKind, c.PublishedAt.UTC().Format("2006-01-02"), engagement(c))
		if excerpt := sourceExcerpt(c); excerpt != "" {
			b.WriteString(excerpt)
			b.WriteByte('\n')
		}
	}
}

func writeStarting(b *strings.Builder, start startingSources) {
	if start.n == 0 {
		return
	}
	origin := "the report the reader started from cites"
	if start.picked {
		origin = "the reader picked"
	}
	fmt.Fprintf(b, "\nSources %s are the ones %s: they are the starting point. Build every idea on at least one of them and list it in the idea's evidence; across the ideas, use all of them. The other sources only add context.\n",
		sourceRefs(start.n), origin)
}

// sourceRefs names the first n sources: "[1]", "[1] and [2]", "[1] to [5]".
func sourceRefs(n int) string {
	switch n {
	case 1:
		return "[1]"
	case 2:
		return "[1] and [2]"
	default:
		return fmt.Sprintf("[1] to [%d]", n)
	}
}

// citesStarting reports whether the evidence cites one of the first n sources.
func citesStarting(evidence []int, n int) bool {
	for _, ref := range evidence {
		if ref >= 1 && ref <= n {
			return true
		}
	}
	return false
}

func judgeSystemPrompt() string {
	return `You decide whether a newly proposed project idea is the same idea as one the reader already has.

Two ideas are the same when they solve the same core problem for the same audience with essentially the same product. Different wording, names, stacks or minor features do not make them different. A different audience or a fundamentally different product does.

Answer with the number of the existing idea that is the same idea, or 0 when none is.`
}

func buildJudgePrompt(candidate ideaDraft, similar []repositories.SimilarIdea) string {
	var b strings.Builder
	fmt.Fprintf(&b, "New idea:\nTitle: %s\nSummary: %s\nFor: %s\nHow: %s\n\nExisting ideas:\n",
		candidate.Title, candidate.Summary, primaryAudience(candidate.Audience), clip(candidate.How, 600))
	for i, s := range similar {
		fmt.Fprintf(&b, "\n[%d] %s\nSummary: %s\nFor: %s\nHow: %s\n", i+1, s.Idea.Title, s.Idea.Summary,
			primaryAudience(s.Content.Audience), clip(s.Content.How, 600))
	}
	return b.String()
}

func primaryAudience(segments []models.AudienceSegment) string {
	for _, a := range segments {
		if a.Kind == models.AudiencePrimary {
			return a.Label + " — " + a.Description
		}
	}
	return ""
}

func revisionSystemPrompt(lang string) string {
	return `You maintain a project idea for the reader of newsp. You receive the current version of the idea and new information, and you write the next version.

Write everything in ` + languageName(lang) + `.

Rules:
- Keep what is still valid; do not rewrite for style. Change what the new information changes, and add depth where asked.
- The current version cites its evidence as [E1], [E2]... Keep those citations where they still apply. Cite new sources as [n] and list their numbers in evidence.
- Do not invent facts or numbers.
- change_note: one or two sentences saying what changed and why, for someone comparing the versions. Name only the sections you actually changed; never claim a change you did not make.`
}

// deepenRetryNote follows a deepen whose answer repeated the current version.
const deepenRetryNote = "\nYour previous answer repeated the current version without changes. That is not a valid answer: " +
	"add new, concrete content to the requested aspects (more steps, competitors, risks or details from the sources).\n"

func buildMergePrompt(current models.IdeaVersion, content models.IdeaContent, candidate ideaDraft, candidates []repositories.Candidate) string {
	var b strings.Builder
	b.WriteString("A new proposal turned out to be the same idea as the current one. Integrate what the proposal adds (new evidence, a sharper audience, a better plan) into the next version.\n\n")
	writeCurrentIdea(&b, current, content)
	proposal, _ := json.MarshalIndent(candidate, "", "  ")
	fmt.Fprintf(&b, "\nNew proposal (its [n] citations refer to the sources below):\n%s\n", proposal)
	writeSources(&b, candidates)
	return b.String()
}

func buildDeepenPrompt(current models.IdeaVersion, content models.IdeaContent, aspects []string, instruction string, candidates []repositories.Candidate) string {
	var b strings.Builder
	b.WriteString("The reader asked to deepen this idea")
	if len(aspects) > 0 {
		fmt.Fprintf(&b, ", especially: %s", strings.Join(aspects, ", "))
	}
	b.WriteString(".\n")
	b.WriteString("Deepening means adding new, concrete content (more MVP steps, competitors, architecture details, numbers from the sources), " +
		"not restating the current text. Returning the current version unchanged is not acceptable.\n")
	if instruction != "" {
		fmt.Fprintf(&b, "Reader's instruction: %s\n", instruction)
	}
	b.WriteString("Use the recent sources below when they are relevant.\n\n")
	writeCurrentIdea(&b, current, content)
	writeSources(&b, candidates)
	return b.String()
}

func writeCurrentIdea(b *strings.Builder, v models.IdeaVersion, content models.IdeaContent) {
	effort := 0
	if v.EffortWeeks != nil {
		effort = *v.EffortWeeks
	}
	current := map[string]any{
		"title": v.Title, "summary": v.Summary, "why_now": content.WhyNow, "audience": content.Audience, "how": content.How,
		"differentiator": content.Differentiator, "stack": content.Stack, "architecture_steps": content.ArchitectureSteps,
		"mvp_plan": content.MVPPlan, "monetization": content.Monetization, "competitors": content.Competitors,
		"difficulty": v.Difficulty, "effort_weeks": effort,
	}
	raw, _ := json.MarshalIndent(current, "", "  ")
	fmt.Fprintf(b, "Current version (v%d):\n%s\n", v.Version, raw)
}

var (
	newRefPattern      = regexp.MustCompile(`\[(\d{1,3})\]`)
	evidenceRefPattern = regexp.MustCompile(`\[E(\d{1,3})\]`)
)

// evidenceMapper turns the model's [n] citations into stable [Ek] labels and
// collects the evidence of a version.
type evidenceMapper struct {
	candidates []repositories.Candidate
	labels     map[int]string // source number -> E label
	keep       map[string]bool
	evidence   []models.IdeaEvidence
	next       int
}

// newEvidenceMapper starts from the evidence an idea already has (nil for a new idea).
func newEvidenceMapper(previous []repositories.EvidenceItem, candidates []repositories.Candidate) *evidenceMapper {
	m := &evidenceMapper{candidates: candidates, labels: map[int]string{}, keep: map[string]bool{}, next: 1}
	for _, e := range previous {
		m.evidence = append(m.evidence, models.IdeaEvidence{ItemID: e.ItemID, Ref: e.Ref})
		m.keep[e.Ref] = true
		if n, err := strconv.Atoi(strings.TrimPrefix(e.Ref, "E")); err == nil && n >= m.next {
			m.next = n + 1
		}
	}
	return m
}

// cite registers source n (1-based) and returns its label.
func (m *evidenceMapper) cite(n int) (string, bool) {
	if n < 1 || n > len(m.candidates) {
		return "", false
	}
	if label, ok := m.labels[n]; ok {
		return label, true
	}
	itemID := m.candidates[n-1].ItemID
	for _, e := range m.evidence { // the same item may already be evidence
		if e.ItemID == itemID {
			m.labels[n] = e.Ref
			return e.Ref, true
		}
	}
	label := "E" + strconv.Itoa(m.next)
	m.next++
	m.labels[n] = label
	m.keep[label] = true
	m.evidence = append(m.evidence, models.IdeaEvidence{ItemID: itemID, Ref: label})
	return label, true
}

// rewrite replaces [n] with [Ek] and drops citations that point nowhere.
func (m *evidenceMapper) rewrite(text string) string {
	text = newRefPattern.ReplaceAllStringFunc(text, func(tok string) string {
		n, _ := strconv.Atoi(tok[1 : len(tok)-1])
		if label, ok := m.cite(n); ok {
			return "[" + label + "]"
		}
		return ""
	})
	return evidenceRefPattern.ReplaceAllStringFunc(text, func(tok string) string {
		if m.keep[tok[1:len(tok)-1]] {
			return tok
		}
		return ""
	})
}

// toVersion converts a draft into version fields and content, mapping citations.
func (m *evidenceMapper) toVersion(d ideaDraft) (title, summary string, difficulty models.Difficulty, effort int, content models.IdeaContent) {
	for _, n := range validRefs(d.Evidence, len(m.candidates)) {
		m.cite(n)
	}
	content = models.IdeaContent{
		WhyNow: m.rewrite(strings.TrimSpace(d.WhyNow)), Audience: d.Audience, How: m.rewrite(strings.TrimSpace(d.How)),
		Differentiator: m.rewrite(strings.TrimSpace(d.Differentiator)), Stack: nonNil(d.Stack), ArchitectureSteps: nonNil(d.ArchitectureSteps),
		MVPPlan: d.MVPPlan, Monetization: m.rewrite(strings.TrimSpace(d.Monetization)), Competitors: d.Competitors,
	}
	if content.Competitors == nil {
		content.Competitors = []models.Competitor{}
	}
	return strings.TrimSpace(d.Title), m.rewrite(strings.TrimSpace(d.Summary)), d.Difficulty, d.EffortWeeks, content
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// embeddingText is what represents an idea in vector space.
func embeddingText(title, summary string, content models.IdeaContent) string {
	plain := evidenceRefPattern.ReplaceAllString(content.WhyNow, "")
	return clip(title+"\n"+summary+"\nFor: "+primaryAudience(content.Audience)+"\n"+plain, 2000)
}
