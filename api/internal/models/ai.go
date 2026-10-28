package models

import (
	"time"

	"github.com/google/uuid"
)

// ProviderKind is the wire protocol an AI provider speaks.
type ProviderKind string

// Provider kinds. OpenAICompatible covers Ollama, llama.cpp and LM Studio.
const (
	ProviderOpenAICompatible ProviderKind = "openai_compatible"
	ProviderOpenAI           ProviderKind = "openai"
	ProviderAnthropic        ProviderKind = "anthropic"
	ProviderGemini           ProviderKind = "gemini"
)

// AIProvider is an AI endpoint, configured by an admin or owned by a user (BYOK).
type AIProvider struct {
	ID          uuid.UUID
	UserID      *uuid.UUID // nil = instance provider (admin-managed)
	Name        string
	Kind        ProviderKind
	BaseURL     string
	APIKeyEnc   []byte // AES-256-GCM; never leaves the backend
	Enabled     bool
	LastCheckAt *time.Time
	LastError   *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ModelCapability says what a model is used for.
type ModelCapability string

// Model capabilities.
const (
	CapabilityChat      ModelCapability = "chat"
	CapabilityEmbedding ModelCapability = "embedding"
)

// AIModel is a model exposed by a provider.
type AIModel struct {
	ID          uuid.UUID
	ProviderID  uuid.UUID
	Name        string // model id at the provider
	DisplayName string
	Capability  ModelCapability
	Enabled     bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// InstanceSettings is the singleton configuration row.
type InstanceSettings struct {
	ID                      uuid.UUID
	EmbeddingModelID        *uuid.UUID
	DefaultReportModelID    *uuid.UUID
	DefaultIdeaModelID      *uuid.UUID
	RetentionDays           int
	IdeaSimilarityThreshold float32
	SignupEnabled           bool
	InactivityPauseDays     int
	BYOKEnabled             bool
	SetupCompletedAt        *time.Time
	UpdatedAt               time.Time
}

// UsagePurpose tags an AI call for usage accounting.
type UsagePurpose string

// AI usage purposes.
const (
	PurposeReport         UsagePurpose = "report"
	PurposeIdea           UsagePurpose = "idea"
	PurposeIdeaJudge      UsagePurpose = "idea_judge"
	PurposeIdeaMerge      UsagePurpose = "idea_merge"
	PurposeIdeaDeepen     UsagePurpose = "idea_deepen"
	PurposeEmbedding      UsagePurpose = "embedding"
	PurposeConnectionTest UsagePurpose = "connection_test"
)

// AIUsage records the tokens of one AI call.
type AIUsage struct {
	ID           uuid.UUID
	UserID       *uuid.UUID
	ModelID      *uuid.UUID
	ModelName    string
	Purpose      UsagePurpose
	InputTokens  int
	OutputTokens int
	CreatedAt    time.Time
}
