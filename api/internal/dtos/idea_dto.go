package dtos

import (
	"time"

	"github.com/dev-fritz/newsp/api/internal/models"
)

// GenerateIdeasRequest asks for new ideas.
type GenerateIdeasRequest struct {
	Count      int    `json:"count" validate:"required,min=1,max=5" example:"3"`
	Focus      string `json:"focus" validate:"max=200" example:"ferramentas para devs"`
	WindowDays int    `json:"window_days" validate:"omitempty,min=1,max=30" example:"7"`
	// ReportID starts from one of the caller's finished reports: its cited items
	// first, then its window and focus (focus and window_days are ignored).
	ReportID *string `json:"report_id,omitempty" validate:"omitempty,uuid" example:"5f0c1f8e-3c8a-4c1e-9a43-2b7e0f1d6c11"`
	// ItemIDs starts from news items the caller picked: they are cited first, then
	// the window and focus fill the rest. Not allowed with report_id.
	ItemIDs []string `json:"item_ids,omitempty" validate:"omitempty,max=5,dive,uuid" example:"5f0c1f8e-3c8a-4c1e-9a43-2b7e0f1d6c11"`
}

// DeepenIdeaRequest asks for a deeper version of an idea.
type DeepenIdeaRequest struct {
	Aspects     []string `json:"aspects" validate:"max=6,dive,oneof=architecture mvp_plan market monetization audience competitors" enums:"architecture,mvp_plan,market,monetization,audience,competitors"`
	Instruction string   `json:"instruction" validate:"max=500"`
}

// IdeaStatusRequest triages an idea.
type IdeaStatusRequest struct {
	Status models.IdeaStatus `json:"status" validate:"required,oneof=new saved discarded" enums:"new,saved,discarded"`
}

// IdeaContentDTO is the structured body of an idea version.
type IdeaContentDTO struct {
	WhyNow            string                   `json:"why_now" validate:"required,max=8000"`
	Audience          []models.AudienceSegment `json:"audience" validate:"required,min=1,max=8,dive"`
	How               string                   `json:"how" validate:"required,max=8000"`
	Differentiator    string                   `json:"differentiator" validate:"max=4000"`
	Stack             []string                 `json:"stack" validate:"max=20,dive,max=80"`
	ArchitectureSteps []string                 `json:"architecture_steps" validate:"max=20,dive,max=300"`
	MVPPlan           []models.PlanStep        `json:"mvp_plan" validate:"required,min=2,max=12"`
	Monetization      string                   `json:"monetization" validate:"max=4000"`
	Competitors       []models.Competitor      `json:"competitors" validate:"max=12"`
}

// EditIdeaRequest stores a hand-written version of an idea.
type EditIdeaRequest struct {
	Title       string            `json:"title" validate:"required,max=120"`
	Summary     string            `json:"summary" validate:"required,max=1000"`
	Difficulty  models.Difficulty `json:"difficulty" validate:"required,oneof=beginner intermediate advanced" enums:"beginner,intermediate,advanced"`
	EffortWeeks int               `json:"effort_weeks" validate:"required,min=1,max=104"`
	Content     IdeaContentDTO    `json:"content" validate:"required"`
}

// IdeaListItemResponse is an idea in a list.
type IdeaListItemResponse struct {
	ID               string               `json:"id"`
	Status           models.IdeaStatus    `json:"status" enums:"new,saved,discarded"`
	Title            string               `json:"title"`
	Summary          string               `json:"summary"`
	CurrentVersion   int                  `json:"current_version"`
	LastOrigin       models.VersionOrigin `json:"last_origin" enums:"generated,merged,deepened,manual"`
	Difficulty       models.Difficulty    `json:"difficulty" enums:"beginner,intermediate,advanced"`
	EffortWeeks      *int                 `json:"effort_weeks"`
	EvidenceCount    int                  `json:"evidence_count"`
	PrimaryAudience  string               `json:"primary_audience"`
	MergedCandidates int                  `json:"merged_candidates"`
	CreatedAt        time.Time            `json:"created_at"`
	UpdatedAt        time.Time            `json:"updated_at"`
}

// EvidenceResponse is an item supporting an idea version, cited as [ref].
type EvidenceResponse struct {
	Ref         string    `json:"ref" example:"E1"`
	ItemID      string    `json:"item_id"`
	URL         string    `json:"url"`
	Title       string    `json:"title"`
	SourceKind  string    `json:"source_kind"`
	SourceName  string    `json:"source_name"`
	PublishedAt time.Time `json:"published_at"`
	Score       *int      `json:"score"`
	Comments    *int      `json:"comments"`
	// Excerpt is a short plain-text preview of the item ("" when it has none).
	Excerpt string `json:"excerpt"`
}

// IdeaVersionResponse is a full version of an idea.
type IdeaVersionResponse struct {
	Version     int                  `json:"version"`
	Origin      models.VersionOrigin `json:"origin" enums:"generated,merged,deepened,manual"`
	Title       string               `json:"title"`
	Summary     string               `json:"summary"`
	Difficulty  models.Difficulty    `json:"difficulty" enums:"beginner,intermediate,advanced"`
	EffortWeeks *int                 `json:"effort_weeks"`
	Content     models.IdeaContent   `json:"content"`
	ChangeNote  *string              `json:"change_note"`
	Similarity  *float32             `json:"similarity"`
	ModelName   *string              `json:"model_name"`
	CreatedAt   time.Time            `json:"created_at"`
	Evidence    []EvidenceResponse   `json:"evidence"`
}

// IdeaVersionSummaryResponse is a version in the history list.
type IdeaVersionSummaryResponse struct {
	Version    int                  `json:"version"`
	Origin     models.VersionOrigin `json:"origin" enums:"generated,merged,deepened,manual"`
	ChangeNote *string              `json:"change_note"`
	Similarity *float32             `json:"similarity"`
	ModelName  *string              `json:"model_name"`
	CreatedAt  time.Time            `json:"created_at"`
}

// IdeaResponse is an idea with its current version and history.
type IdeaResponse struct {
	ID               string                       `json:"id"`
	Status           models.IdeaStatus            `json:"status" enums:"new,saved,discarded"`
	CurrentVersion   int                          `json:"current_version"`
	MergedCandidates int                          `json:"merged_candidates"`
	CreatedAt        time.Time                    `json:"created_at"`
	UpdatedAt        time.Time                    `json:"updated_at"`
	Current          IdeaVersionResponse          `json:"current"`
	Versions         []IdeaVersionSummaryResponse `json:"versions"`
}

// IdeaEnvelope wraps IdeaResponse.
type IdeaEnvelope struct {
	Data IdeaResponse `json:"data"`
}

// IdeaVersionEnvelope wraps IdeaVersionResponse.
type IdeaVersionEnvelope struct {
	Data IdeaVersionResponse `json:"data"`
}

// IdeaListEnvelope is a page of ideas.
type IdeaListEnvelope struct {
	Data []IdeaListItemResponse `json:"data"`
	Meta PageMeta               `json:"meta"`
}
