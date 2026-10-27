package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// IdeaStatus is the user's triage state for an idea.
type IdeaStatus string

// Idea statuses. Discarded ideas still block duplicates.
const (
	IdeaNew       IdeaStatus = "new"
	IdeaSaved     IdeaStatus = "saved"
	IdeaDiscarded IdeaStatus = "discarded"
)

// VersionOrigin tells how an idea version was produced.
type VersionOrigin string

// Version origins.
const (
	OriginGenerated VersionOrigin = "generated"
	OriginMerged    VersionOrigin = "merged"
	OriginDeepened  VersionOrigin = "deepened"
	OriginManual    VersionOrigin = "manual"
)

// Difficulty is the estimated build difficulty of an idea.
type Difficulty string

// Difficulty levels.
const (
	DifficultyBeginner     Difficulty = "beginner"
	DifficultyIntermediate Difficulty = "intermediate"
	DifficultyAdvanced     Difficulty = "advanced"
)

// Idea is a unique project idea owned by a user; its content lives in versions.
type Idea struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	Status           IdeaStatus
	Title            string
	Summary          string
	CurrentVersion   int
	Embedding        []float32
	EmbeddingModel   *string
	MergedCandidates int
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// IdeaVersion is an immutable snapshot of an idea.
type IdeaVersion struct {
	ID          uuid.UUID
	IdeaID      uuid.UUID
	Version     int
	Origin      VersionOrigin
	Title       string
	Summary     string
	Difficulty  Difficulty
	EffortWeeks *int
	Content     json.RawMessage
	ChangeNote  *string
	Similarity  *float32
	ModelID     *uuid.UUID
	ModelName   *string
	CreatedBy   *uuid.UUID
	CreatedAt   time.Time
}

// IdeaEvidence links an idea version to an item that supports it.
type IdeaEvidence struct {
	IdeaVersionID uuid.UUID
	ItemID        uuid.UUID
	Ref           string
}

// AudienceKind classifies an audience segment.
type AudienceKind string

// Audience kinds.
const (
	AudiencePrimary    AudienceKind = "primary"
	AudienceSecondary  AudienceKind = "secondary"
	AudienceOutOfScope AudienceKind = "out_of_scope"
)

// AudienceSegment is one group of people the idea serves (or not).
type AudienceSegment struct {
	Kind        AudienceKind `json:"kind"`
	Label       string       `json:"label"`
	Description string       `json:"description"`
}

// PlanStep is one step of the MVP plan.
type PlanStep struct {
	Period      string `json:"period"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

// Competitor is an existing alternative and what it leaves open.
type Competitor struct {
	Name      string `json:"name"`
	Strengths string `json:"strengths"`
	Gap       string `json:"gap"`
}

// IdeaContent is the structured body of an idea version (idea_versions.content).
// Text fields are Markdown; why_now cites evidence as [E1], [E2]...
type IdeaContent struct {
	WhyNow            string            `json:"why_now"`
	Audience          []AudienceSegment `json:"audience"`
	How               string            `json:"how"`
	Differentiator    string            `json:"differentiator"`
	Stack             []string          `json:"stack"`
	ArchitectureSteps []string          `json:"architecture_steps"`
	MVPPlan           []PlanStep        `json:"mvp_plan"`
	Monetization      string            `json:"monetization"`
	Competitors       []Competitor      `json:"competitors"`
}
