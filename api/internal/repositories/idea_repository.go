package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/dev-fritz/newsp/api/internal/database"
	"github.com/dev-fritz/newsp/api/internal/models"
)

const ideaColumns = `i.id, i.user_id, i.status, i.title, i.summary, i.current_version, i.embedding_model, i.merged_candidates, i.created_at, i.updated_at`

const versionColumns = `v.id, v.idea_id, v.version, v.origin, v.title, v.summary, v.difficulty, v.effort_weeks, v.content,
	v.change_note, v.similarity, v.model_id, v.model_name, v.created_by, v.created_at`

// IdeaListItem is an idea with what the list needs from its current version.
type IdeaListItem struct {
	Idea          models.Idea
	Origin        models.VersionOrigin
	Difficulty    models.Difficulty
	EffortWeeks   *int
	EvidenceCount int
	Audience      string // primary audience label
}

// SimilarIdea is an existing idea close to a candidate.
type SimilarIdea struct {
	Idea       models.Idea
	Similarity float32
	Content    models.IdeaContent
}

// EvidenceItem is an item supporting an idea version.
type EvidenceItem struct {
	Ref         string
	ItemID      uuid.UUID
	URL         string
	Title       string
	SourceKind  string
	SourceName  string
	PublishedAt time.Time
	Score       *int
	Comments    *int
	// Excerpt is a short plain-text preview of the item ("" when it has none).
	Excerpt string
}

// IdeaFilter selects ideas of a user.
type IdeaFilter struct {
	UserID uuid.UUID
	Status string // "", "active" (not discarded), "new", "saved", "discarded"
	Query  string
	Limit  int
	Offset int
}

// IdeaRepository persists ideas and their immutable versions.
type IdeaRepository struct {
	db *sql.DB
}

// NewIdeaRepository builds an IdeaRepository.
func NewIdeaRepository(db *sql.DB) *IdeaRepository {
	return &IdeaRepository{db: db}
}

func scanIdea(row interface{ Scan(...any) error }, extra ...any) (models.Idea, error) {
	var i models.Idea
	dest := append([]any{&i.ID, &i.UserID, &i.Status, &i.Title, &i.Summary, &i.CurrentVersion, &i.EmbeddingModel,
		&i.MergedCandidates, &i.CreatedAt, &i.UpdatedAt}, extra...)
	return i, row.Scan(dest...)
}

func scanVersion(row interface{ Scan(...any) error }) (models.IdeaVersion, error) {
	var v models.IdeaVersion
	var content []byte
	err := row.Scan(&v.ID, &v.IdeaID, &v.Version, &v.Origin, &v.Title, &v.Summary, &v.Difficulty, &v.EffortWeeks, &content,
		&v.ChangeNote, &v.Similarity, &v.ModelID, &v.ModelName, &v.CreatedBy, &v.CreatedAt)
	v.Content = content
	return v, err
}

// LockUser serializes idea writes of one user for the rest of the transaction,
// so two generations can never both decide that a candidate is new.
func (r *IdeaRepository) LockUser(ctx context.Context, userID uuid.UUID) error {
	_, err := database.Conn(ctx, r.db).ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 42))`, "ideas:"+userID.String())
	if err != nil {
		return fmt.Errorf("lock user ideas: %w", err)
	}
	return nil
}

// Create inserts an idea with its first version and evidence. Call it in a transaction.
func (r *IdeaRepository) Create(ctx context.Context, idea models.Idea, v models.IdeaVersion, embedding []float32, evidence []models.IdeaEvidence) error {
	const q = `INSERT INTO ideas (id, user_id, status, title, summary, current_version, embedding, embedding_model, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 1, $6::vector, $7, $8, $8)`
	if _, err := database.Conn(ctx, r.db).ExecContext(ctx, q, idea.ID, idea.UserID, idea.Status, idea.Title, idea.Summary,
		vectorOrNil(embedding), idea.EmbeddingModel, idea.CreatedAt); err != nil {
		return fmt.Errorf("create idea: %w", translate(err))
	}
	return r.insertVersion(ctx, v, evidence)
}

// AddVersion stores a new version and makes it current. Call it in a
// transaction after GetForUpdate. A nil embedding keeps the current one.
func (r *IdeaRepository) AddVersion(ctx context.Context, idea models.Idea, v models.IdeaVersion, embedding []float32, evidence []models.IdeaEvidence) error {
	if err := r.insertVersion(ctx, v, evidence); err != nil {
		return err
	}
	const q = `UPDATE ideas SET title = $2, summary = $3, current_version = $4, merged_candidates = $5, updated_at = $6,
		embedding = COALESCE($7::vector, embedding), embedding_model = COALESCE($8, embedding_model) WHERE id = $1`
	res, err := database.Conn(ctx, r.db).ExecContext(ctx, q, idea.ID, v.Title, v.Summary, v.Version, idea.MergedCandidates,
		v.CreatedAt, vectorOrNil(embedding), idea.EmbeddingModel)
	return checkAffected(res, err, "update idea")
}

func (r *IdeaRepository) insertVersion(ctx context.Context, v models.IdeaVersion, evidence []models.IdeaEvidence) error {
	conn := database.Conn(ctx, r.db)
	const q = `INSERT INTO idea_versions (id, idea_id, version, origin, title, summary, difficulty, effort_weeks, content,
		change_note, similarity, model_id, model_name, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`
	if _, err := conn.ExecContext(ctx, q, v.ID, v.IdeaID, v.Version, v.Origin, v.Title, v.Summary, v.Difficulty, v.EffortWeeks,
		[]byte(v.Content), v.ChangeNote, v.Similarity, v.ModelID, v.ModelName, v.CreatedBy, v.CreatedAt); err != nil {
		return fmt.Errorf("insert idea version: %w", translate(err))
	}
	if len(evidence) == 0 {
		return nil
	}
	items := make([]string, len(evidence))
	refs := make([]string, len(evidence))
	for i, e := range evidence {
		items[i], refs[i] = e.ItemID.String(), e.Ref
	}
	const ev = `INSERT INTO idea_evidence (idea_version_id, item_id, ref) SELECT $1, unnest($2::uuid[]), unnest($3::text[])`
	if _, err := conn.ExecContext(ctx, ev, v.ID, items, refs); err != nil {
		return fmt.Errorf("insert idea evidence: %w", translate(err))
	}
	return nil
}

// GetForUser returns an idea owned by userID.
func (r *IdeaRepository) GetForUser(ctx context.Context, id, userID uuid.UUID) (models.Idea, error) {
	q := `SELECT ` + ideaColumns + ` FROM ideas i WHERE i.id = $1 AND i.user_id = $2`
	idea, err := scanIdea(database.Conn(ctx, r.db).QueryRowContext(ctx, q, id, userID))
	if err != nil {
		return models.Idea{}, fmt.Errorf("get idea: %w", translate(err))
	}
	return idea, nil
}

// GetForUpdate locks an idea row; versions are added under this lock.
func (r *IdeaRepository) GetForUpdate(ctx context.Context, id uuid.UUID) (models.Idea, error) {
	q := `SELECT ` + ideaColumns + ` FROM ideas i WHERE i.id = $1 FOR UPDATE`
	idea, err := scanIdea(database.Conn(ctx, r.db).QueryRowContext(ctx, q, id))
	if err != nil {
		return models.Idea{}, fmt.Errorf("lock idea: %w", translate(err))
	}
	return idea, nil
}

// SetStatus changes the triage status of an idea.
func (r *IdeaRepository) SetStatus(ctx context.Context, id uuid.UUID, status models.IdeaStatus, now time.Time) error {
	res, err := database.Conn(ctx, r.db).ExecContext(ctx, `UPDATE ideas SET status = $2, updated_at = $3 WHERE id = $1`, id, status, now)
	return checkAffected(res, err, "set idea status")
}

// List returns a page of a user's ideas with their current version details.
func (r *IdeaRepository) List(ctx context.Context, f IdeaFilter) ([]IdeaListItem, int, error) {
	conn := database.Conn(ctx, r.db)
	const where = `i.user_id = $1
		AND ($2 = '' OR ($2 = 'active' AND i.status <> 'discarded') OR i.status = $2)
		AND ($3 = '' OR i.title ILIKE '%' || $3 || '%' OR i.summary ILIKE '%' || $3 || '%')`
	var total int
	if err := conn.QueryRowContext(ctx, `SELECT count(*) FROM ideas i WHERE `+where, f.UserID, f.Status, f.Query).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count ideas: %w", err)
	}
	q := `SELECT ` + ideaColumns + `, v.origin, v.difficulty, v.effort_weeks,
			(SELECT count(*) FROM idea_evidence e WHERE e.idea_version_id = v.id),
			COALESCE((SELECT a->>'label' FROM jsonb_array_elements(v.content->'audience') a WHERE a->>'kind' = 'primary' LIMIT 1), '')
		FROM ideas i JOIN idea_versions v ON v.idea_id = i.id AND v.version = i.current_version
		WHERE ` + where + ` ORDER BY i.updated_at DESC, i.id LIMIT $4 OFFSET $5`
	rows, err := conn.QueryContext(ctx, q, f.UserID, f.Status, f.Query, f.Limit, f.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list ideas: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]IdeaListItem, 0, f.Limit)
	for rows.Next() {
		var it IdeaListItem
		idea, err := scanIdea(rows, &it.Origin, &it.Difficulty, &it.EffortWeeks, &it.EvidenceCount, &it.Audience)
		if err != nil {
			return nil, 0, fmt.Errorf("scan idea: %w", err)
		}
		it.Idea = idea
		out = append(out, it)
	}
	return out, total, rows.Err()
}

// Version returns one version of an idea.
func (r *IdeaRepository) Version(ctx context.Context, ideaID uuid.UUID, version int) (models.IdeaVersion, error) {
	q := `SELECT ` + versionColumns + ` FROM idea_versions v WHERE v.idea_id = $1 AND v.version = $2`
	v, err := scanVersion(database.Conn(ctx, r.db).QueryRowContext(ctx, q, ideaID, version))
	if err != nil {
		return models.IdeaVersion{}, fmt.Errorf("get idea version: %w", translate(err))
	}
	return v, nil
}

// Versions lists every version of an idea, newest first (content included; ideas have few versions).
func (r *IdeaRepository) Versions(ctx context.Context, ideaID uuid.UUID) ([]models.IdeaVersion, error) {
	q := `SELECT ` + versionColumns + ` FROM idea_versions v WHERE v.idea_id = $1 ORDER BY v.version DESC`
	rows, err := database.Conn(ctx, r.db).QueryContext(ctx, q, ideaID)
	if err != nil {
		return nil, fmt.Errorf("list idea versions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []models.IdeaVersion
	for rows.Next() {
		v, err := scanVersion(rows)
		if err != nil {
			return nil, fmt.Errorf("scan idea version: %w", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// Evidence returns the items supporting a version, ordered by reference.
func (r *IdeaRepository) Evidence(ctx context.Context, versionID uuid.UUID) ([]EvidenceItem, error) {
	const q = `
		SELECT e.ref, i.id, i.url, i.title, s.kind, s.name, i.published_at, i.score, i.comments, ` + excerptSQL + `
		FROM idea_evidence e JOIN items i ON i.id = e.item_id JOIN sources s ON s.id = i.source_id
		WHERE e.idea_version_id = $1 ORDER BY length(e.ref), e.ref`
	rows, err := database.Conn(ctx, r.db).QueryContext(ctx, q, versionID)
	if err != nil {
		return nil, fmt.Errorf("idea evidence: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []EvidenceItem
	for rows.Next() {
		var e EvidenceItem
		if err := rows.Scan(&e.Ref, &e.ItemID, &e.URL, &e.Title, &e.SourceKind, &e.SourceName, &e.PublishedAt, &e.Score, &e.Comments, &e.Excerpt); err != nil {
			return nil, fmt.Errorf("scan evidence: %w", err)
		}
		e.Excerpt = excerpt(e.Excerpt)
		out = append(out, e)
	}
	return out, rows.Err()
}

// Similar finds the user's ideas (any status) whose embedding is at least
// minSimilarity close to vector, best first.
func (r *IdeaRepository) Similar(ctx context.Context, userID uuid.UUID, vector []float32, minSimilarity float32, limit int) ([]SimilarIdea, error) {
	q := `SELECT ` + ideaColumns + `, (1 - (i.embedding <=> $2::vector))::real, v.content
		FROM ideas i JOIN idea_versions v ON v.idea_id = i.id AND v.version = i.current_version
		WHERE i.user_id = $1 AND i.embedding IS NOT NULL AND 1 - (i.embedding <=> $2::vector) >= $3
		ORDER BY i.embedding <=> $2::vector LIMIT $4`
	rows, err := database.Conn(ctx, r.db).QueryContext(ctx, q, userID, VectorLiteral(vector), minSimilarity, limit)
	if err != nil {
		return nil, fmt.Errorf("similar ideas: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []SimilarIdea
	for rows.Next() {
		var s SimilarIdea
		var content []byte
		idea, err := scanIdea(rows, &s.Similarity, &content)
		if err != nil {
			return nil, fmt.Errorf("scan similar idea: %w", err)
		}
		s.Idea = idea
		if err := json.Unmarshal(content, &s.Content); err != nil {
			return nil, fmt.Errorf("decode idea content: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// RecentTitles lists the titles and summaries of a user's latest ideas (any
// status) so prompts can steer away from them.
func (r *IdeaRepository) RecentTitles(ctx context.Context, userID uuid.UUID, limit int) ([]models.Idea, error) {
	q := `SELECT ` + ideaColumns + ` FROM ideas i WHERE i.user_id = $1 ORDER BY i.updated_at DESC LIMIT $2`
	rows, err := database.Conn(ctx, r.db).QueryContext(ctx, q, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("recent ideas: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []models.Idea
	for rows.Next() {
		idea, err := scanIdea(rows)
		if err != nil {
			return nil, fmt.Errorf("scan idea: %w", err)
		}
		out = append(out, idea)
	}
	return out, rows.Err()
}

// FindByTitle finds a user's idea with the same normalized title; it is the
// dedup fallback when no embedding model is configured.
func (r *IdeaRepository) FindByTitle(ctx context.Context, userID uuid.UUID, title string) (models.Idea, error) {
	q := `SELECT ` + ideaColumns + ` FROM ideas i
		WHERE i.user_id = $1 AND regexp_replace(lower(i.title), '[^[:alnum:]]+', '', 'g') = regexp_replace(lower($2), '[^[:alnum:]]+', '', 'g')
		LIMIT 1`
	idea, err := scanIdea(database.Conn(ctx, r.db).QueryRowContext(ctx, q, userID, title))
	if err != nil {
		return models.Idea{}, fmt.Errorf("find idea by title: %w", translate(err))
	}
	return idea, nil
}

func vectorOrNil(v []float32) any {
	if v == nil {
		return nil
	}
	return VectorLiteral(v)
}
