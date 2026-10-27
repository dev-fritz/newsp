package models

import (
	"time"

	"github.com/google/uuid"
)

// UserPreferences drives what a user's reports and ideas focus on.
type UserPreferences struct {
	UserID               uuid.UUID
	Topics               []string
	TopicsEmbedding      []float32
	TopicsEmbeddingModel *string
	OutputLang           string
	Timezone             string
	ReportEnabled        bool
	ReportTime           string // HH:MM in Timezone
	NextReportAt         *time.Time
	IdeasPerDay          int
	ReportModelID        *uuid.UUID
	IdeaModelID          *uuid.UUID
	Theme                string
	ExcludedSourceIDs    []uuid.UUID
	UpdatedAt            time.Time
}
