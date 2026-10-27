package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"

	"github.com/dev-fritz/newsp/api/internal/database"
	"github.com/dev-fritz/newsp/api/internal/models"
)

// IdeaToEmbed is an idea with the current version's text, ready to be embedded.
type IdeaToEmbed struct {
	ID      uuid.UUID
	Version int
	Title   string
	Summary string
	Content models.IdeaContent
}

// TopicsToEmbed is a user's topics, ready to be embedded.
type TopicsToEmbed struct {
	UserID uuid.UUID
	Topics []string
}

// MaintenanceRepository runs the retention and reindexing statements.
type MaintenanceRepository struct {
	db *sql.DB
}

// NewMaintenanceRepository builds a MaintenanceRepository.
func NewMaintenanceRepository(db *sql.DB) *MaintenanceRepository {
	return &MaintenanceRepository{db: db}
}

// DeleteExpiredItems deletes up to limit items collected before cutoff that no
// report and no idea cites, and returns how many it deleted.
func (r *MaintenanceRepository) DeleteExpiredItems(ctx context.Context, cutoff time.Time, limit int) (int, error) {
	const q = `
		DELETE FROM items WHERE id IN (
			SELECT i.id FROM items i
			WHERE i.collected_at < $1
			  AND NOT EXISTS (SELECT 1 FROM report_items ri WHERE ri.item_id = i.id)
			  AND NOT EXISTS (SELECT 1 FROM idea_evidence e WHERE e.item_id = i.id)
			LIMIT $2)`
	res, err := database.Conn(ctx, r.db).ExecContext(ctx, q, cutoff, limit)
	if err != nil {
		return 0, fmt.Errorf("delete expired items: %w", translate(err))
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("delete expired items: %w", err)
	}
	return int(n), nil
}

// ResetStaleItemEmbeddings clears the vectors made by a model other than
// model, so the embedding job redoes them. It returns how many items changed.
func (r *MaintenanceRepository) ResetStaleItemEmbeddings(ctx context.Context, model string) (int, error) {
	const q = `
		UPDATE items SET embedding = NULL, embedding_model = NULL, embedded_at = NULL
		WHERE embedding IS NOT NULL AND embedding_model IS DISTINCT FROM $1`
	res, err := database.Conn(ctx, r.db).ExecContext(ctx, q, model)
	if err != nil {
		return 0, fmt.Errorf("reset item embeddings: %w", translate(err))
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("reset item embeddings: %w", err)
	}
	return int(n), nil
}

// StaleIdeas returns up to limit ideas, with their current version, whose
// vector was not made by model.
func (r *MaintenanceRepository) StaleIdeas(ctx context.Context, model string, limit int) ([]IdeaToEmbed, error) {
	const q = `
		SELECT i.id, v.version, v.title, v.summary, v.content
		FROM ideas i
		JOIN idea_versions v ON v.idea_id = i.id AND v.version = i.current_version
		WHERE i.embedding_model IS DISTINCT FROM $1
		ORDER BY i.id
		LIMIT $2`
	rows, err := database.Conn(ctx, r.db).QueryContext(ctx, q, model, limit)
	if err != nil {
		return nil, fmt.Errorf("list stale ideas: %w", translate(err))
	}
	defer func() { _ = rows.Close() }()
	var out []IdeaToEmbed
	for rows.Next() {
		var (
			idea IdeaToEmbed
			raw  []byte
		)
		if err := rows.Scan(&idea.ID, &idea.Version, &idea.Title, &idea.Summary, &raw); err != nil {
			return nil, fmt.Errorf("scan stale idea: %w", err)
		}
		if err := json.Unmarshal(raw, &idea.Content); err != nil {
			return nil, fmt.Errorf("decode content of idea %s: %w", idea.ID, err)
		}
		out = append(out, idea)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list stale ideas: %w", err)
	}
	return out, nil
}

// HasStaleIdeas reports whether any idea has a vector not made by model.
func (r *MaintenanceRepository) HasStaleIdeas(ctx context.Context, model string) (bool, error) {
	const q = `SELECT EXISTS (SELECT 1 FROM ideas WHERE embedding_model IS DISTINCT FROM $1)`
	var ok bool
	if err := database.Conn(ctx, r.db).QueryRowContext(ctx, q, model).Scan(&ok); err != nil {
		return false, fmt.Errorf("check stale ideas: %w", translate(err))
	}
	return ok, nil
}

// SetIdeaEmbeddings stores one vector per idea in a single statement. An idea
// that gained a version meanwhile (versions[i] is no longer current) is left
// untouched; the next run picks it up.
func (r *MaintenanceRepository) SetIdeaEmbeddings(ctx context.Context, ids []uuid.UUID, versions []int, vectors [][]float32, model string) error {
	if len(ids) != len(vectors) || len(ids) != len(versions) {
		return fmt.Errorf("set idea embeddings: %d ids, %d versions, %d vectors", len(ids), len(versions), len(vectors))
	}
	if len(ids) == 0 {
		return nil
	}
	texts := make([]string, len(vectors))
	for i, v := range vectors {
		texts[i] = pgvector.NewVector(v).String()
	}
	const q = `
		UPDATE ideas SET embedding = v.embedding::vector, embedding_model = $4
		FROM unnest($1::uuid[], $2::int[], $3::text[]) AS v (id, version, embedding)
		WHERE ideas.id = v.id AND ideas.current_version = v.version`
	if _, err := database.Conn(ctx, r.db).ExecContext(ctx, q, ids, versions, texts, model); err != nil {
		return fmt.Errorf("set idea embeddings: %w", translate(err))
	}
	return nil
}

// StaleTopics returns up to limit users with topics whose vector was not made
// by model.
func (r *MaintenanceRepository) StaleTopics(ctx context.Context, model string, limit int) ([]TopicsToEmbed, error) {
	const q = `
		SELECT user_id, array_to_json(topics)
		FROM user_preferences
		WHERE cardinality(topics) > 0 AND topics_embedding_model IS DISTINCT FROM $1
		ORDER BY user_id
		LIMIT $2`
	rows, err := database.Conn(ctx, r.db).QueryContext(ctx, q, model, limit)
	if err != nil {
		return nil, fmt.Errorf("list stale topics: %w", translate(err))
	}
	defer func() { _ = rows.Close() }()
	var out []TopicsToEmbed
	for rows.Next() {
		var (
			t   TopicsToEmbed
			raw []byte
		)
		if err := rows.Scan(&t.UserID, &raw); err != nil {
			return nil, fmt.Errorf("scan stale topics: %w", err)
		}
		if err := json.Unmarshal(raw, &t.Topics); err != nil {
			return nil, fmt.Errorf("decode topics of user %s: %w", t.UserID, err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list stale topics: %w", err)
	}
	return out, nil
}

// HasStaleTopics reports whether any user has topics with a vector not made by model.
func (r *MaintenanceRepository) HasStaleTopics(ctx context.Context, model string) (bool, error) {
	const q = `SELECT EXISTS (SELECT 1 FROM user_preferences WHERE cardinality(topics) > 0 AND topics_embedding_model IS DISTINCT FROM $1)`
	var ok bool
	if err := database.Conn(ctx, r.db).QueryRowContext(ctx, q, model).Scan(&ok); err != nil {
		return false, fmt.Errorf("check stale topics: %w", translate(err))
	}
	return ok, nil
}

// SetTopicsEmbeddings stores one vector per user in a single statement. A user
// whose topics changed meanwhile (topics[i] no longer matches) is left
// untouched: saving topics already schedules their own embedding.
func (r *MaintenanceRepository) SetTopicsEmbeddings(ctx context.Context, userIDs []uuid.UUID, topics [][]string, vectors [][]float32, model string) error {
	if len(userIDs) != len(vectors) || len(userIDs) != len(topics) {
		return fmt.Errorf("set topics embeddings: %d users, %d topic lists, %d vectors", len(userIDs), len(topics), len(vectors))
	}
	if len(userIDs) == 0 {
		return nil
	}
	texts := make([]string, len(vectors))
	topicsJSON := make([]string, len(topics))
	for i, v := range vectors {
		texts[i] = pgvector.NewVector(v).String()
		raw, err := json.Marshal(topics[i])
		if err != nil {
			return fmt.Errorf("set topics embeddings: %w", err)
		}
		topicsJSON[i] = string(raw)
	}
	const q = `
		UPDATE user_preferences SET topics_embedding = v.embedding::vector, topics_embedding_model = $4
		FROM unnest($1::uuid[], $2::text[], $3::text[]) AS v (user_id, topics, embedding)
		WHERE user_preferences.user_id = v.user_id AND array_to_json(user_preferences.topics)::jsonb = v.topics::jsonb`
	if _, err := database.Conn(ctx, r.db).ExecContext(ctx, q, userIDs, topicsJSON, texts, model); err != nil {
		return fmt.Errorf("set topics embeddings: %w", translate(err))
	}
	return nil
}
