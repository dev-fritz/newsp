// Package services holds the business rules. Services depend on small,
// consumer-defined interfaces so they can be unit tested with generated mocks.
package services

//go:generate go tool mockgen -source=ports.go -destination=../../tests/mocks/services_mocks.go -package=mocks
//go:generate go tool mockgen -destination=../../tests/mocks/connectors_mocks.go -package=mocks github.com/dev-fritz/newsp/api/pkg/connectors Connector

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/dev-fritz/newsp/api/internal/auth"
	"github.com/dev-fritz/newsp/api/internal/models"
	"github.com/dev-fritz/newsp/api/internal/repositories"
	"github.com/dev-fritz/newsp/api/internal/workers"
	"github.com/dev-fritz/newsp/api/pkg/connectors"
)

// TxRunner opens a transaction carried by the context.
type TxRunner interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// Limiter is a keyed rate limiter.
type Limiter interface {
	Allow(key string) (bool, time.Duration)
}

// UserStore persists users.
type UserStore interface {
	Create(ctx context.Context, u models.User) error
	GetByID(ctx context.Context, id uuid.UUID) (models.User, error)
	GetByIDForUpdate(ctx context.Context, id uuid.UUID) (models.User, error)
	GetByEmail(ctx context.Context, email string) (models.User, error)
	List(ctx context.Context, limit, offset int) ([]models.User, int, error)
	LockActiveAdmins(ctx context.Context) ([]uuid.UUID, error)
	UpdateAccess(ctx context.Context, id uuid.UUID, role models.Role, status models.UserStatus, now time.Time) error
	UpdatePassword(ctx context.Context, id uuid.UUID, hash string, now time.Time) error
	TouchLastSeen(ctx context.Context, id uuid.UUID, now time.Time) error
	MarkEmailVerified(ctx context.Context, id uuid.UUID, now time.Time) error
}

// UserTokenStore persists one-time links.
type UserTokenStore interface {
	Create(ctx context.Context, t models.UserToken) error
	GetUsable(ctx context.Context, kind models.UserTokenKind, hash []byte, now time.Time) (models.UserToken, error)
	GetUsableForUpdate(ctx context.Context, kind models.UserTokenKind, hash []byte, now time.Time) (models.UserToken, error)
	MarkUsed(ctx context.Context, id uuid.UUID, now time.Time) error
	RevokeInvite(ctx context.Context, id uuid.UUID, now time.Time) error
	RevokePendingResets(ctx context.Context, userID uuid.UUID, now time.Time) error
	RevokePendingSelfResets(ctx context.Context, userID uuid.UUID, now time.Time) error
	ListPendingInvites(ctx context.Context, now time.Time, limit, offset int) ([]models.UserToken, int, error)
}

// SessionStore persists refresh tokens.
type SessionStore interface {
	Create(ctx context.Context, t models.RefreshToken) error
	GetByHashForUpdate(ctx context.Context, hash []byte) (models.RefreshToken, error)
	MarkRotated(ctx context.Context, id uuid.UUID, now time.Time) error
	RevokeFamily(ctx context.Context, familyID uuid.UUID, now time.Time) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID, now time.Time) error
}

// SetupStore reads and completes the first-run setup state.
type SetupStore interface {
	Get(ctx context.Context) (models.InstanceSettings, error)
	GetForUpdate(ctx context.Context) (models.InstanceSettings, error)
	MarkSetupCompleted(ctx context.Context, now time.Time) error
}

// PreferenceSeeder stores the default preferences of a new account, so the
// scheduler delivers its daily report before the user ever opens the profile.
type PreferenceSeeder interface {
	Save(ctx context.Context, p models.UserPreferences) error
}

// EmailQueue queues transactional e-mails (see EmailService).
type EmailQueue interface {
	Enqueue(ctx context.Context, userID *uuid.UUID, e Email) error
	Ready() bool
}

// AccessIssuer signs access tokens.
type AccessIssuer interface {
	Issue(userID uuid.UUID, role models.Role) (string, time.Time, error)
}

// compile-time check that the real issuer fits.
var _ AccessIssuer = (*auth.TokenIssuer)(nil)

// AIStore persists providers, models and usage.
type AIStore interface {
	CreateProvider(ctx context.Context, p models.AIProvider) error
	UpdateProvider(ctx context.Context, p models.AIProvider) error
	SetProviderCheck(ctx context.Context, id uuid.UUID, at time.Time, checkErr *string) error
	GetProvider(ctx context.Context, id uuid.UUID) (models.AIProvider, error)
	ListProviders(ctx context.Context) ([]models.AIProvider, error)
	ListUserProviders(ctx context.Context, userID uuid.UUID) ([]models.AIProvider, error)
	DeleteProvider(ctx context.Context, id uuid.UUID) error
	CreateModel(ctx context.Context, m models.AIModel) error
	UpdateModel(ctx context.Context, m models.AIModel) error
	DeleteModel(ctx context.Context, id uuid.UUID) error
	GetModel(ctx context.Context, id uuid.UUID) (repositories.ModelWithProvider, error)
	ListModels(ctx context.Context) ([]models.AIModel, error)
	ListUserModels(ctx context.Context, userID uuid.UUID) ([]models.AIModel, error)
	RecordUsage(ctx context.Context, u models.AIUsage) error
	TokensSince(ctx context.Context, since time.Time) (map[uuid.UUID]int64, error)
}

// SettingsStore reads and updates the instance settings.
type SettingsStore interface {
	Get(ctx context.Context) (models.InstanceSettings, error)
	GetForUpdate(ctx context.Context) (models.InstanceSettings, error)
	UpdateAI(ctx context.Context, s models.InstanceSettings) error
	UpdateAccounts(ctx context.Context, s models.InstanceSettings) error
}

// SecretBox encrypts secrets at rest.
type SecretBox interface {
	Seal(plaintext []byte) ([]byte, error)
	Open(sealed []byte) ([]byte, error)
}

// Enqueuer schedules background jobs.
type Enqueuer interface {
	Enqueue(ctx context.Context, kind string, payload any, opts workers.EnqueueOptions) (uuid.UUID, error)
}

// SourceStore persists sources.
type SourceStore interface {
	Create(ctx context.Context, s models.Source) error
	GetByID(ctx context.Context, id uuid.UUID) (models.Source, error)
	GetWithStats(ctx context.Context, id uuid.UUID, since time.Time) (repositories.SourceStats, error)
	ListWithStats(ctx context.Context, since time.Time) ([]repositories.SourceStats, error)
	List(ctx context.Context) ([]models.Source, error)
	Update(ctx context.Context, s models.Source) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListDue(ctx context.Context, now time.Time, limit int) ([]uuid.UUID, error)
	MarkSuccess(ctx context.Context, id uuid.UUID, now, nextRunAt time.Time) error
	MarkFailure(ctx context.Context, id uuid.UUID, now time.Time, errMsg string, nextRunAt time.Time) error
}

// ItemStore persists and reads collected items.
type ItemStore interface {
	Upsert(ctx context.Context, sourceID uuid.UUID, items []models.Item) (int, error)
	Pending(ctx context.Context, limit int) ([]repositories.PendingItem, error)
	HasPending(ctx context.Context) (bool, error)
	SetEmbeddings(ctx context.Context, ids []uuid.UUID, vectors [][]float32, model string, now time.Time) error
	List(ctx context.Context, f repositories.ItemFilter, limit, offset int) ([]repositories.ItemView, int, error)
}

// ConnectorRegistry looks source connectors up by kind.
type ConnectorRegistry interface {
	Get(kind string) (connectors.Connector, bool)
}

// Embedder turns texts into vectors with the instance embedding model.
type Embedder interface {
	Embed(ctx context.Context, purpose models.UsagePurpose, texts []string) ([][]float32, string, error)
}

// compile-time checks that the real implementations fit.
var (
	_ ConnectorRegistry = (*connectors.Registry)(nil)
	_ Embedder          = (*AIGateway)(nil)
)
