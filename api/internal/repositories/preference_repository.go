package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dev-fritz/newsp/api/internal/database"
	"github.com/dev-fritz/newsp/api/internal/models"
)

// PreferenceRepository persists user preferences.
type PreferenceRepository struct {
	db *sql.DB
}

// NewPreferenceRepository builds a PreferenceRepository.
func NewPreferenceRepository(db *sql.DB) *PreferenceRepository {
	return &PreferenceRepository{db: db}
}

// Get returns a user's preferences or ErrNotFound when they were never saved.
// Arrays are read as JSON to stay within database/sql scanning.
func (r *PreferenceRepository) Get(ctx context.Context, userID uuid.UUID) (models.UserPreferences, error) {
	const q = `
		SELECT p.user_id, array_to_json(p.topics), p.topics_embedding_model, p.output_lang, p.timezone,
			p.report_enabled, to_char(p.report_time, 'HH24:MI'), p.next_report_at, p.ideas_per_day,
			p.report_model_id, p.idea_model_id, p.theme, p.updated_at,
			COALESCE((SELECT json_agg(e.source_id) FROM user_source_exclusions e WHERE e.user_id = p.user_id), '[]'::json)
		FROM user_preferences p WHERE p.user_id = $1`
	var (
		p                models.UserPreferences
		topics, excluded []byte
	)
	err := database.Conn(ctx, r.db).QueryRowContext(ctx, q, userID).Scan(
		&p.UserID, &topics, &p.TopicsEmbeddingModel, &p.OutputLang, &p.Timezone,
		&p.ReportEnabled, &p.ReportTime, &p.NextReportAt, &p.IdeasPerDay,
		&p.ReportModelID, &p.IdeaModelID, &p.Theme, &p.UpdatedAt, &excluded)
	if err != nil {
		return models.UserPreferences{}, fmt.Errorf("get preferences: %w", translate(err))
	}
	if err := json.Unmarshal(topics, &p.Topics); err != nil {
		return models.UserPreferences{}, fmt.Errorf("decode topics: %w", err)
	}
	if err := json.Unmarshal(excluded, &p.ExcludedSourceIDs); err != nil {
		return models.UserPreferences{}, fmt.Errorf("decode exclusions: %w", err)
	}
	return p, nil
}

// Save upserts the preferences and replaces the source exclusions. Call it in
// a transaction. The topics embedding is cleared when the topics change.
func (r *PreferenceRepository) Save(ctx context.Context, p models.UserPreferences) error {
	conn := database.Conn(ctx, r.db)
	const upsert = `
		INSERT INTO user_preferences (user_id, topics, output_lang, timezone, report_enabled, report_time, next_report_at,
			ideas_per_day, report_model_id, idea_model_id, theme, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6::time, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (user_id) DO UPDATE SET
			topics_embedding = CASE WHEN user_preferences.topics = EXCLUDED.topics THEN user_preferences.topics_embedding END,
			topics_embedding_model = CASE WHEN user_preferences.topics = EXCLUDED.topics THEN user_preferences.topics_embedding_model END,
			topics = EXCLUDED.topics, output_lang = EXCLUDED.output_lang, timezone = EXCLUDED.timezone,
			report_enabled = EXCLUDED.report_enabled, report_time = EXCLUDED.report_time, next_report_at = EXCLUDED.next_report_at,
			ideas_per_day = EXCLUDED.ideas_per_day, report_model_id = EXCLUDED.report_model_id,
			idea_model_id = EXCLUDED.idea_model_id, theme = EXCLUDED.theme, updated_at = EXCLUDED.updated_at`
	topics := p.Topics
	if topics == nil {
		topics = []string{}
	}
	if _, err := conn.ExecContext(ctx, upsert, p.UserID, topics, p.OutputLang, p.Timezone, p.ReportEnabled, p.ReportTime,
		p.NextReportAt, p.IdeasPerDay, p.ReportModelID, p.IdeaModelID, p.Theme, p.UpdatedAt); err != nil {
		return fmt.Errorf("save preferences: %w", translate(err))
	}

	if _, err := conn.ExecContext(ctx, `DELETE FROM user_source_exclusions WHERE user_id = $1`, p.UserID); err != nil {
		return fmt.Errorf("clear source exclusions: %w", err)
	}
	if len(p.ExcludedSourceIDs) > 0 {
		ids := make([]string, len(p.ExcludedSourceIDs))
		for i, id := range p.ExcludedSourceIDs {
			ids[i] = id.String()
		}
		const q = `INSERT INTO user_source_exclusions (user_id, source_id) SELECT $1, unnest($2::uuid[])`
		if _, err := conn.ExecContext(ctx, q, p.UserID, ids); err != nil {
			return fmt.Errorf("save source exclusions: %w", translate(err))
		}
	}
	return nil
}

// SetTopicsEmbedding stores the embedding of a user's topics.
func (r *PreferenceRepository) SetTopicsEmbedding(ctx context.Context, userID uuid.UUID, vector []float32, model string) error {
	const q = `UPDATE user_preferences SET topics_embedding = $2::vector, topics_embedding_model = $3 WHERE user_id = $1`
	res, err := database.Conn(ctx, r.db).ExecContext(ctx, q, userID, VectorLiteral(vector), model)
	return checkAffected(res, err, "set topics embedding")
}

// SetNextReportAt reschedules a user's daily report.
func (r *PreferenceRepository) SetNextReportAt(ctx context.Context, userID uuid.UUID, at *time.Time) error {
	res, err := database.Conn(ctx, r.db).ExecContext(ctx, `UPDATE user_preferences SET next_report_at = $2 WHERE user_id = $1`, userID, at)
	return checkAffected(res, err, "set next report time")
}

// generationEligible keeps AI spend on accounts that can be reached and still
// use the app: a verified e-mail and a visit within the instance's inactivity
// window (0 days disables the pause). It expects the aliases u (users) and s
// (instance_settings, a single row).
const generationEligible = `
		u.email_verified_at IS NOT NULL
		AND (s.inactivity_pause_days = 0
			OR coalesce(u.last_seen_at, u.created_at) >= $1 - make_interval(days => s.inactivity_pause_days))`

// DueReportUsers returns the users whose daily report is due, for the scheduler.
// Unverified and inactive accounts are skipped and keep their next_report_at, so
// they get a report on the first scheduler tick after they come back.
func (r *PreferenceRepository) DueReportUsers(ctx context.Context, now time.Time, limit int) ([]uuid.UUID, error) {
	const q = `
		SELECT p.user_id FROM user_preferences p JOIN users u ON u.id = p.user_id CROSS JOIN instance_settings s
		WHERE p.report_enabled AND p.next_report_at <= $1 AND u.status = 'active' AND` + generationEligible + `
		ORDER BY p.next_report_at LIMIT $2`
	rows, err := database.Conn(ctx, r.db).QueryContext(ctx, q, now, limit)
	if err != nil {
		return nil, fmt.Errorf("due reports: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan due report: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// EmailVerified reports whether the user confirmed their e-mail address.
func (r *PreferenceRepository) EmailVerified(ctx context.Context, userID uuid.UUID) (bool, error) {
	var verified bool
	err := database.Conn(ctx, r.db).QueryRowContext(ctx,
		`SELECT email_verified_at IS NOT NULL FROM users WHERE id = $1`, userID).Scan(&verified)
	if err != nil {
		return false, fmt.Errorf("email verified: %w", translate(err))
	}
	return verified, nil
}

// TopicsEmbedding returns the stored embedding of a user's topics and the model
// that produced it; a nil vector means there is none.
func (r *PreferenceRepository) TopicsEmbedding(ctx context.Context, userID uuid.UUID) ([]float32, string, error) {
	const q = `SELECT topics_embedding::text, COALESCE(topics_embedding_model, '') FROM user_preferences WHERE user_id = $1`
	var (
		raw   sql.NullString
		model string
	)
	err := database.Conn(ctx, r.db).QueryRowContext(ctx, q, userID).Scan(&raw, &model)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !raw.Valid) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("topics embedding: %w", err)
	}
	v, err := ParseVector(raw.String)
	if err != nil {
		return nil, "", fmt.Errorf("topics embedding: %w", err)
	}
	return v, model, nil
}

// ParseVector parses pgvector's text format ("[1,2,3]").
func ParseVector(s string) ([]float32, error) {
	s = strings.TrimSpace(s)
	if len(s) < 2 || s[0] != '[' || s[len(s)-1] != ']' {
		return nil, fmt.Errorf("invalid vector %q", s)
	}
	parts := strings.Split(s[1:len(s)-1], ",")
	out := make([]float32, 0, len(parts))
	for _, p := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 32)
		if err != nil {
			return nil, fmt.Errorf("invalid vector component %q", p)
		}
		out = append(out, float32(f))
	}
	return out, nil
}

// VectorLiteral renders a vector in pgvector's text format ("[1,2,3]").
func VectorLiteral(v []float32) string {
	var b strings.Builder
	b.Grow(len(v) * 10)
	b.WriteByte('[')
	for i, f := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(f), 'g', -1, 32))
	}
	b.WriteByte(']')
	return b.String()
}
