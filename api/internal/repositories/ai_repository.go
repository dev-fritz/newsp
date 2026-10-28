package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/dev-fritz/newsp/api/internal/database"
	"github.com/dev-fritz/newsp/api/internal/models"
)

const providerColumns = `id, user_id, name, kind, base_url, api_key_enc, enabled, last_check_at, last_error, created_at, updated_at`

const modelColumns = `m.id, m.provider_id, m.name, m.display_name, m.capability, m.enabled, m.created_at, m.updated_at`

// ModelWithProvider is a model joined with the provider that serves it.
type ModelWithProvider struct {
	Model    models.AIModel
	Provider models.AIProvider
}

// AIRepository persists providers, models and usage.
type AIRepository struct {
	db *sql.DB
}

// NewAIRepository builds an AIRepository.
func NewAIRepository(db *sql.DB) *AIRepository {
	return &AIRepository{db: db}
}

func scanProvider(row interface{ Scan(...any) error }) (models.AIProvider, error) {
	var p models.AIProvider
	err := row.Scan(&p.ID, &p.UserID, &p.Name, &p.Kind, &p.BaseURL, &p.APIKeyEnc, &p.Enabled, &p.LastCheckAt, &p.LastError, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func scanModel(row interface{ Scan(...any) error }, extra ...any) (models.AIModel, error) {
	var m models.AIModel
	dest := append([]any{&m.ID, &m.ProviderID, &m.Name, &m.DisplayName, &m.Capability, &m.Enabled, &m.CreatedAt, &m.UpdatedAt}, extra...)
	err := row.Scan(dest...)
	return m, err
}

// CreateProvider inserts a provider. A taken name returns ErrDuplicate.
func (r *AIRepository) CreateProvider(ctx context.Context, p models.AIProvider) error {
	const q = `INSERT INTO ai_providers (id, user_id, name, kind, base_url, api_key_enc, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)`
	_, err := database.Conn(ctx, r.db).ExecContext(ctx, q, p.ID, p.UserID, p.Name, p.Kind, p.BaseURL, p.APIKeyEnc, p.Enabled, p.CreatedAt)
	if err != nil {
		return fmt.Errorf("create provider: %w", translate(err))
	}
	return nil
}

// UpdateProvider stores the editable provider fields.
func (r *AIRepository) UpdateProvider(ctx context.Context, p models.AIProvider) error {
	const q = `UPDATE ai_providers SET name = $2, base_url = $3, api_key_enc = $4, enabled = $5, updated_at = $6 WHERE id = $1`
	res, err := database.Conn(ctx, r.db).ExecContext(ctx, q, p.ID, p.Name, p.BaseURL, p.APIKeyEnc, p.Enabled, p.UpdatedAt)
	return checkAffected(res, err, "update provider")
}

// SetProviderCheck records the outcome of a connection test.
func (r *AIRepository) SetProviderCheck(ctx context.Context, id uuid.UUID, at time.Time, checkErr *string) error {
	const q = `UPDATE ai_providers SET last_check_at = $2, last_error = $3 WHERE id = $1`
	res, err := database.Conn(ctx, r.db).ExecContext(ctx, q, id, at, checkErr)
	return checkAffected(res, err, "record provider check")
}

// GetProvider returns a provider or ErrNotFound.
func (r *AIRepository) GetProvider(ctx context.Context, id uuid.UUID) (models.AIProvider, error) {
	p, err := scanProvider(database.Conn(ctx, r.db).QueryRowContext(ctx, `SELECT `+providerColumns+` FROM ai_providers WHERE id = $1`, id))
	if err != nil {
		return models.AIProvider{}, fmt.Errorf("get provider: %w", translate(err))
	}
	return p, nil
}

// ListProviders returns the instance providers (admin-managed) by name (a handful at most).
func (r *AIRepository) ListProviders(ctx context.Context) ([]models.AIProvider, error) {
	return r.queryProviders(ctx, `SELECT `+providerColumns+` FROM ai_providers WHERE user_id IS NULL ORDER BY lower(name)`)
}

// ListUserProviders returns the providers owned by a user, ordered by kind.
func (r *AIRepository) ListUserProviders(ctx context.Context, userID uuid.UUID) ([]models.AIProvider, error) {
	return r.queryProviders(ctx, `SELECT `+providerColumns+` FROM ai_providers WHERE user_id = $1 ORDER BY kind`, userID)
}

func (r *AIRepository) queryProviders(ctx context.Context, q string, args ...any) ([]models.AIProvider, error) {
	rows, err := database.Conn(ctx, r.db).QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []models.AIProvider
	for rows.Next() {
		p, err := scanProvider(rows)
		if err != nil {
			return nil, fmt.Errorf("scan provider: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// DeleteProvider removes a provider and its models.
func (r *AIRepository) DeleteProvider(ctx context.Context, id uuid.UUID) error {
	res, err := database.Conn(ctx, r.db).ExecContext(ctx, `DELETE FROM ai_providers WHERE id = $1`, id)
	return checkAffected(res, err, "delete provider")
}

// CreateModel inserts a model. A duplicate (provider, name, capability) returns ErrDuplicate.
func (r *AIRepository) CreateModel(ctx context.Context, m models.AIModel) error {
	const q = `INSERT INTO ai_models (id, provider_id, name, display_name, capability, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7)`
	_, err := database.Conn(ctx, r.db).ExecContext(ctx, q, m.ID, m.ProviderID, m.Name, m.DisplayName, m.Capability, m.Enabled, m.CreatedAt)
	if err != nil {
		return fmt.Errorf("create model: %w", translate(err))
	}
	return nil
}

// UpdateModel stores the editable model fields.
func (r *AIRepository) UpdateModel(ctx context.Context, m models.AIModel) error {
	const q = `UPDATE ai_models SET display_name = $2, enabled = $3, updated_at = $4 WHERE id = $1`
	res, err := database.Conn(ctx, r.db).ExecContext(ctx, q, m.ID, m.DisplayName, m.Enabled, m.UpdatedAt)
	return checkAffected(res, err, "update model")
}

// DeleteModel removes a model; settings and preferences pointing at it fall back to null.
func (r *AIRepository) DeleteModel(ctx context.Context, id uuid.UUID) error {
	res, err := database.Conn(ctx, r.db).ExecContext(ctx, `DELETE FROM ai_models WHERE id = $1`, id)
	return checkAffected(res, err, "delete model")
}

// GetModel returns a model with its provider, or ErrNotFound.
func (r *AIRepository) GetModel(ctx context.Context, id uuid.UUID) (ModelWithProvider, error) {
	q := `SELECT ` + modelColumns + `, p.id, p.user_id, p.name, p.kind, p.base_url, p.api_key_enc, p.enabled, p.last_check_at, p.last_error, p.created_at, p.updated_at
		FROM ai_models m JOIN ai_providers p ON p.id = m.provider_id WHERE m.id = $1`
	var p models.AIProvider
	m, err := scanModel(database.Conn(ctx, r.db).QueryRowContext(ctx, q, id),
		&p.ID, &p.UserID, &p.Name, &p.Kind, &p.BaseURL, &p.APIKeyEnc, &p.Enabled, &p.LastCheckAt, &p.LastError, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return ModelWithProvider{}, fmt.Errorf("get model: %w", translate(err))
	}
	return ModelWithProvider{Model: m, Provider: p}, nil
}

// ListModels returns the models of instance providers ordered by provider and name.
func (r *AIRepository) ListModels(ctx context.Context) ([]models.AIModel, error) {
	return r.queryModels(ctx, `WHERE p.user_id IS NULL`)
}

// ListUserModels returns the models of the providers owned by a user, ordered by provider and name.
func (r *AIRepository) ListUserModels(ctx context.Context, userID uuid.UUID) ([]models.AIModel, error) {
	return r.queryModels(ctx, `WHERE p.user_id = $1`, userID)
}

func (r *AIRepository) queryModels(ctx context.Context, where string, args ...any) ([]models.AIModel, error) {
	q := `SELECT ` + modelColumns + ` FROM ai_models m JOIN ai_providers p ON p.id = m.provider_id ` + where + `
		ORDER BY lower(p.name), lower(m.display_name)`
	rows, err := database.Conn(ctx, r.db).QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []models.AIModel
	for rows.Next() {
		m, err := scanModel(rows)
		if err != nil {
			return nil, fmt.Errorf("scan model: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// RecordUsage stores the token usage of one AI call.
func (r *AIRepository) RecordUsage(ctx context.Context, u models.AIUsage) error {
	const q = `INSERT INTO ai_usage (id, user_id, model_id, model_name, purpose, input_tokens, output_tokens, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	_, err := database.Conn(ctx, r.db).ExecContext(ctx, q, u.ID, u.UserID, u.ModelID, u.ModelName, u.Purpose, u.InputTokens, u.OutputTokens, u.CreatedAt)
	if err != nil {
		return fmt.Errorf("record ai usage: %w", err)
	}
	return nil
}

// TokensSince sums input+output tokens per model since a point in time.
func (r *AIRepository) TokensSince(ctx context.Context, since time.Time) (map[uuid.UUID]int64, error) {
	const q = `SELECT model_id, sum(input_tokens + output_tokens) FROM ai_usage
		WHERE created_at >= $1 AND model_id IS NOT NULL GROUP BY model_id`
	rows, err := database.Conn(ctx, r.db).QueryContext(ctx, q, since)
	if err != nil {
		return nil, fmt.Errorf("sum ai usage: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[uuid.UUID]int64{}
	for rows.Next() {
		var id uuid.UUID
		var n int64
		if err := rows.Scan(&id, &n); err != nil {
			return nil, fmt.Errorf("scan ai usage: %w", err)
		}
		out[id] = n
	}
	return out, rows.Err()
}
