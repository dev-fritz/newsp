package dtos

import "time"

// PreferencesRequest replaces the caller's preferences.
type PreferencesRequest struct {
	Topics            []string `json:"topics" validate:"max=20,dive,max=60"`
	OutputLang        string   `json:"output_lang" validate:"required,oneof=pt-BR en" enums:"pt-BR,en"`
	Timezone          string   `json:"timezone" validate:"required,max=64" example:"America/Sao_Paulo"`
	ReportEnabled     bool     `json:"report_enabled"`
	ReportTime        string   `json:"report_time" validate:"required,len=5" example:"08:00"`
	IdeasPerDay       int      `json:"ideas_per_day" validate:"min=0,max=10"`
	ReportModelID     *string  `json:"report_model_id" validate:"omitempty,uuid"`
	IdeaModelID       *string  `json:"idea_model_id" validate:"omitempty,uuid"`
	Theme             string   `json:"theme" validate:"required,oneof=light dark system" enums:"light,dark,system"`
	ExcludedSourceIDs []string `json:"excluded_source_ids" validate:"max=500,dive,uuid"`
	UILocale          string   `json:"ui_locale" validate:"required,oneof=pt-BR en" enums:"pt-BR,en"`
}

// PreferencesResponse is the caller's preferences.
type PreferencesResponse struct {
	Topics            []string   `json:"topics"`
	OutputLang        string     `json:"output_lang" enums:"pt-BR,en"`
	Timezone          string     `json:"timezone"`
	ReportEnabled     bool       `json:"report_enabled"`
	ReportTime        string     `json:"report_time"`
	NextReportAt      *time.Time `json:"next_report_at"`
	IdeasPerDay       int        `json:"ideas_per_day"`
	ReportModelID     *string    `json:"report_model_id"`
	IdeaModelID       *string    `json:"idea_model_id"`
	Theme             string     `json:"theme" enums:"light,dark,system"`
	ExcludedSourceIDs []string   `json:"excluded_source_ids"`
	UILocale          string     `json:"ui_locale" enums:"pt-BR,en"`
	// TopicsRanked is false when no embedding model is configured: reports then
	// rank items by recency and engagement only.
	TopicsRanked bool `json:"topics_ranked"`
}

// PreferencesEnvelope wraps PreferencesResponse.
type PreferencesEnvelope struct {
	Data PreferencesResponse `json:"data"`
}
