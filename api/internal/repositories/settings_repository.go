package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/dev-fritz/newsp/api/internal/database"
	"github.com/dev-fritz/newsp/api/internal/models"
)

const settingsColumns = `id, embedding_model_id, default_report_model_id, default_idea_model_id,
	retention_days, idea_similarity_threshold, signup_enabled, inactivity_pause_days, byok_enabled, setup_completed_at, updated_at`

// SettingsRepository reads and writes the singleton instance settings row.
type SettingsRepository struct {
	db *sql.DB
}

// NewSettingsRepository builds a SettingsRepository.
func NewSettingsRepository(db *sql.DB) *SettingsRepository {
	return &SettingsRepository{db: db}
}

func (r *SettingsRepository) get(ctx context.Context, lock bool) (models.InstanceSettings, error) {
	q := `SELECT ` + settingsColumns + ` FROM instance_settings`
	if lock {
		q += ` FOR UPDATE`
	}
	var s models.InstanceSettings
	err := database.Conn(ctx, r.db).QueryRowContext(ctx, q).Scan(
		&s.ID, &s.EmbeddingModelID, &s.DefaultReportModelID, &s.DefaultIdeaModelID,
		&s.RetentionDays, &s.IdeaSimilarityThreshold, &s.SignupEnabled, &s.InactivityPauseDays, &s.BYOKEnabled, &s.SetupCompletedAt, &s.UpdatedAt)
	if err != nil {
		return models.InstanceSettings{}, fmt.Errorf("get instance settings: %w", translate(err))
	}
	return s, nil
}

// Get returns the instance settings.
func (r *SettingsRepository) Get(ctx context.Context) (models.InstanceSettings, error) {
	return r.get(ctx, false)
}

// GetForUpdate returns the settings and locks the row; it serializes the
// first-run setup.
func (r *SettingsRepository) GetForUpdate(ctx context.Context) (models.InstanceSettings, error) {
	return r.get(ctx, true)
}

// MarkSetupCompleted records that the first admin was created.
func (r *SettingsRepository) MarkSetupCompleted(ctx context.Context, now time.Time) error {
	const q = `UPDATE instance_settings SET setup_completed_at = $1, updated_at = $1 WHERE setup_completed_at IS NULL`
	res, err := database.Conn(ctx, r.db).ExecContext(ctx, q, now)
	return checkAffected(res, err, "mark setup completed")
}

// UpdateAI stores the instance-wide AI settings.
func (r *SettingsRepository) UpdateAI(ctx context.Context, s models.InstanceSettings) error {
	const q = `UPDATE instance_settings SET embedding_model_id = $1, default_report_model_id = $2, default_idea_model_id = $3,
		retention_days = $4, idea_similarity_threshold = $5, byok_enabled = $6, updated_at = $7`
	res, err := database.Conn(ctx, r.db).ExecContext(ctx, q, s.EmbeddingModelID, s.DefaultReportModelID, s.DefaultIdeaModelID,
		s.RetentionDays, s.IdeaSimilarityThreshold, s.BYOKEnabled, s.UpdatedAt)
	return checkAffected(res, err, "update instance settings")
}

// UpdateAccounts stores the instance-wide account settings.
func (r *SettingsRepository) UpdateAccounts(ctx context.Context, s models.InstanceSettings) error {
	const q = `UPDATE instance_settings SET signup_enabled = $1, inactivity_pause_days = $2, updated_at = $3`
	res, err := database.Conn(ctx, r.db).ExecContext(ctx, q, s.SignupEnabled, s.InactivityPauseDays, s.UpdatedAt)
	return checkAffected(res, err, "update account settings")
}
