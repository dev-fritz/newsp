package services

//go:generate go tool mockgen -source=preference_ports.go -destination=../../tests/mocks/preference_mocks.go -package=mocks

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/dev-fritz/newsp/api/internal/models"
	"github.com/dev-fritz/newsp/api/internal/repositories"
)

// PreferenceStore persists user preferences.
type PreferenceStore interface {
	Get(ctx context.Context, userID uuid.UUID) (models.UserPreferences, error)
	Save(ctx context.Context, p models.UserPreferences) error
	SetTopicsEmbedding(ctx context.Context, userID uuid.UUID, vector []float32, model string) error
	SetNextReportAt(ctx context.Context, userID uuid.UUID, at *time.Time) error
	DueReportUsers(ctx context.Context, now time.Time, limit int) ([]uuid.UUID, error)
	// EmailVerified reports whether the user confirmed their e-mail address.
	EmailVerified(ctx context.Context, userID uuid.UUID) (bool, error)
}

// AccountStore reads accounts and updates their interface language.
type AccountStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (models.User, error)
	UpdateLocale(ctx context.Context, id uuid.UUID, locale string, now time.Time) error
}

// TextEmbedder embeds texts with the instance embedding model.
type TextEmbedder interface {
	Embed(ctx context.Context, purpose models.UsagePurpose, texts []string) ([][]float32, string, error)
}

// ModelLookup finds a model with its provider.
type ModelLookup interface {
	GetModel(ctx context.Context, id uuid.UUID) (repositories.ModelWithProvider, error)
}
