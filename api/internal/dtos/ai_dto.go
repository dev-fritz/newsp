package dtos

import (
	"time"

	"github.com/dev-fritz/newsp/api/internal/models"
)

// CreateProviderRequest registers an AI provider.
type CreateProviderRequest struct {
	Name    string              `json:"name" validate:"required,max=60"`
	Kind    models.ProviderKind `json:"kind" validate:"required,oneof=openai_compatible openai anthropic gemini" enums:"openai_compatible,openai,anthropic,gemini"`
	BaseURL string              `json:"base_url" validate:"omitempty,max=500"`
	APIKey  string              `json:"api_key" validate:"omitempty,max=500"`
}

// UpdateProviderRequest changes a provider. api_key "" removes the key; omit it to keep the current one.
type UpdateProviderRequest struct {
	Name    *string `json:"name" validate:"omitempty,min=1,max=60"`
	BaseURL *string `json:"base_url" validate:"omitempty,max=500"`
	APIKey  *string `json:"api_key" validate:"omitempty,max=500"`
	Enabled *bool   `json:"enabled"`
}

// CreateModelRequest registers a model of a provider.
type CreateModelRequest struct {
	Name        string                 `json:"name" validate:"required,max=200"`
	DisplayName string                 `json:"display_name" validate:"omitempty,max=100"`
	Capability  models.ModelCapability `json:"capability" validate:"required,oneof=chat embedding" enums:"chat,embedding"`
}

// UpdateModelRequest changes a model.
type UpdateModelRequest struct {
	DisplayName *string `json:"display_name" validate:"omitempty,min=1,max=100"`
	Enabled     *bool   `json:"enabled"`
}

// ModelResponse is a registered model.
type ModelResponse struct {
	ID           string                 `json:"id"`
	ProviderID   string                 `json:"provider_id"`
	Name         string                 `json:"name" example:"qwen3:14b"`
	DisplayName  string                 `json:"display_name"`
	Capability   models.ModelCapability `json:"capability" enums:"chat,embedding"`
	Enabled      bool                   `json:"enabled"`
	TokensLast7d int64                  `json:"tokens_last_7d"`
}

// NewModelResponse maps a model.
func NewModelResponse(m models.AIModel, tokens int64) ModelResponse {
	return ModelResponse{
		ID: m.ID.String(), ProviderID: m.ProviderID.String(), Name: m.Name, DisplayName: m.DisplayName,
		Capability: m.Capability, Enabled: m.Enabled, TokensLast7d: tokens,
	}
}

// ProviderResponse is a provider as admins see it. The key is never returned.
type ProviderResponse struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Kind        models.ProviderKind `json:"kind" enums:"openai_compatible,openai,anthropic,gemini"`
	BaseURL     string              `json:"base_url"`
	HasAPIKey   bool                `json:"has_api_key"`
	KeyHint     string              `json:"key_hint,omitempty" example:"••••a9Fk"`
	Enabled     bool                `json:"enabled"`
	LastCheckAt *time.Time          `json:"last_check_at"`
	LastError   *string             `json:"last_error"`
	Models      []ModelResponse     `json:"models"`
}

// NewProviderResponse maps a provider without its models.
func NewProviderResponse(p models.AIProvider, keyHint string) ProviderResponse {
	return ProviderResponse{
		ID: p.ID.String(), Name: p.Name, Kind: p.Kind, BaseURL: p.BaseURL, HasAPIKey: len(p.APIKeyEnc) > 0,
		KeyHint: keyHint, Enabled: p.Enabled, LastCheckAt: p.LastCheckAt, LastError: p.LastError, Models: []ModelResponse{},
	}
}

// ConnectionTestResponse is the result of a connection test.
type ConnectionTestResponse struct {
	OK     bool     `json:"ok"`
	Models []string `json:"models"`
	Error  string   `json:"error,omitempty"`
}

// ChatModelResponse is a chat model users may pick.
type ChatModelResponse struct {
	ID           string `json:"id"`
	DisplayName  string `json:"display_name"`
	Name         string `json:"name"`
	ProviderName string `json:"provider_name"`
	Local        bool   `json:"local"`
}

// SettingsResponse is the instance configuration.
type SettingsResponse struct {
	EmbeddingModelID        *string `json:"embedding_model_id"`
	DefaultReportModelID    *string `json:"default_report_model_id"`
	DefaultIdeaModelID      *string `json:"default_idea_model_id"`
	RetentionDays           int     `json:"retention_days"`
	IdeaSimilarityThreshold float32 `json:"idea_similarity_threshold"`
	SignupEnabled           bool    `json:"signup_enabled"`
	// InactivityPauseDays pauses reports and ideas of accounts not seen for
	// this many days; 0 never pauses.
	InactivityPauseDays int `json:"inactivity_pause_days"`
	// MailConfigured tells whether e-mail delivery works; signup needs it.
	MailConfigured bool      `json:"mail_configured"`
	UpdatedAt      time.Time `json:"updated_at"`
	ReindexJobID   *string   `json:"reindex_job_id,omitempty"`
}

// UpdateSettingsRequest changes instance settings. Send "" for a model id to clear it.
type UpdateSettingsRequest struct {
	EmbeddingModelID        *string  `json:"embedding_model_id" validate:"omitempty,uuid"`
	DefaultReportModelID    *string  `json:"default_report_model_id" validate:"omitempty,uuid"`
	DefaultIdeaModelID      *string  `json:"default_idea_model_id" validate:"omitempty,uuid"`
	RetentionDays           *int     `json:"retention_days" validate:"omitempty,min=7,max=3650"`
	IdeaSimilarityThreshold *float32 `json:"idea_similarity_threshold" validate:"omitempty,min=0.5,max=0.99"`
	SignupEnabled           *bool    `json:"signup_enabled"`
	InactivityPauseDays     *int     `json:"inactivity_pause_days" validate:"omitempty,min=0,max=3650"`
}

// ProviderEnvelope wraps ProviderResponse.
type ProviderEnvelope struct {
	Data ProviderResponse `json:"data"`
}

// ProviderListEnvelope wraps a list of providers.
type ProviderListEnvelope struct {
	Data []ProviderResponse `json:"data"`
}

// ModelEnvelope wraps ModelResponse.
type ModelEnvelope struct {
	Data ModelResponse `json:"data"`
}

// ChatModelListEnvelope wraps the chat models users may pick.
type ChatModelListEnvelope struct {
	Data []ChatModelResponse `json:"data"`
}

// ConnectionTestEnvelope wraps ConnectionTestResponse.
type ConnectionTestEnvelope struct {
	Data ConnectionTestResponse `json:"data"`
}

// SettingsEnvelope wraps SettingsResponse.
type SettingsEnvelope struct {
	Data SettingsResponse `json:"data"`
}
