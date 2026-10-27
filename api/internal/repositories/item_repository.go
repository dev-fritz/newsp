package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"

	"github.com/dev-fritz/newsp/api/internal/database"
	"github.com/dev-fritz/newsp/api/internal/models"
)

// upsertChunk bounds the rows of one INSERT so the statement stays far below
// the 65535 bind parameter limit (12 parameters per row).
const upsertChunk = 500

// ItemSort is how the feed is ordered.
type ItemSort string

// Feed orderings.
const (
	ItemSortRecent     ItemSort = "recent"
	ItemSortEngagement ItemSort = "engagement"
)

// itemOrder maps a sort to its ORDER BY clause. Only these literals reach the
// query; the sort never comes from input directly.
var itemOrder = map[ItemSort]string{
	ItemSortRecent:     `i.published_at DESC, i.id DESC`,
	ItemSortEngagement: `COALESCE(i.score, 0) + COALESCE(i.comments, 0) DESC, i.published_at DESC, i.id DESC`,
}

// ItemFilter narrows the feed. Zero fields mean "no filter".
type ItemFilter struct {
	SourceID *uuid.UUID
	Since    *time.Time // published at or after
	Until    *time.Time // published before
	Query    string     // full-text search, web search syntax
	Sort     ItemSort   // defaults to ItemSortRecent
}

// ItemView is an item in the feed, with its source and whether a report or an
// idea cites it.
type ItemView struct {
	Item       models.Item
	SourceKind models.SourceKind
	SourceName string
	Cited      bool
}

// PendingItem is an item waiting for its embedding.
type PendingItem struct {
	ID      uuid.UUID
	Title   string
	Summary string
}

// ItemRepository persists collected items.
type ItemRepository struct {
	db *sql.DB
}

// NewItemRepository builds an ItemRepository.
func NewItemRepository(db *sql.DB) *ItemRepository {
	return &ItemRepository{db: db}
}

// Upsert stores items of one source idempotently, one statement per chunk of
// 500 rows, and returns how many were new. Existing items (same source and
// external id) get url, title, summary, author, score, comments and metrics
// refreshed, keeping their id, published_at, collected_at and embedding.
// Items repeated inside the call keep the last occurrence.
func (r *ItemRepository) Upsert(ctx context.Context, sourceID uuid.UUID, items []models.Item) (int, error) {
	items = lastByExternalID(items)
	inserted := 0
	for start := 0; start < len(items); start += upsertChunk {
		n, err := r.upsertChunk(ctx, sourceID, items[start:min(start+upsertChunk, len(items))])
		if err != nil {
			return inserted, err
		}
		inserted += n
	}
	return inserted, nil
}

func lastByExternalID(items []models.Item) []models.Item {
	index := make(map[string]int, len(items))
	out := make([]models.Item, 0, len(items))
	for _, it := range items {
		if i, ok := index[it.ExternalID]; ok {
			out[i] = it
			continue
		}
		index[it.ExternalID] = len(out)
		out = append(out, it)
	}
	return out
}

func (r *ItemRepository) upsertChunk(ctx context.Context, sourceID uuid.UUID, items []models.Item) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}
	var b strings.Builder
	b.WriteString(`INSERT INTO items (id, source_id, external_id, url, title, summary, author, published_at, collected_at, score, comments, metrics) VALUES `)
	args := make([]any, 0, len(items)*12)
	for i, it := range items {
		metrics := it.Metrics
		if len(metrics) == 0 {
			metrics = json.RawMessage(`{}`)
		}
		if i > 0 {
			b.WriteByte(',')
		}
		base := i * 12
		b.WriteByte('(')
		for j := 1; j <= 12; j++ {
			if j > 1 {
				b.WriteByte(',')
			}
			b.WriteString("$" + strconv.Itoa(base+j))
		}
		b.WriteByte(')')
		args = append(args, uuid.New(), sourceID, it.ExternalID, it.URL, it.Title, it.Summary, it.Author,
			it.PublishedAt, it.CollectedAt, it.Score, it.Comments, []byte(metrics))
	}
	b.WriteString(` ON CONFLICT (source_id, external_id) DO UPDATE SET
		url = EXCLUDED.url, title = EXCLUDED.title, summary = EXCLUDED.summary, author = EXCLUDED.author,
		score = EXCLUDED.score, comments = EXCLUDED.comments, metrics = EXCLUDED.metrics
		WHERE (items.url, items.title, items.summary, items.author, items.score, items.comments, items.metrics)
			IS DISTINCT FROM (EXCLUDED.url, EXCLUDED.title, EXCLUDED.summary, EXCLUDED.author, EXCLUDED.score, EXCLUDED.comments, EXCLUDED.metrics)
		RETURNING (xmax = 0)`)

	rows, err := database.Conn(ctx, r.db).QueryContext(ctx, b.String(), args...)
	if err != nil {
		return 0, fmt.Errorf("upsert items: %w", translate(err))
	}
	defer func() { _ = rows.Close() }()
	inserted := 0
	for rows.Next() {
		var isNew bool
		if err := rows.Scan(&isNew); err != nil {
			return 0, fmt.Errorf("scan upserted item: %w", err)
		}
		if isNew {
			inserted++
		}
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("upsert items: %w", err)
	}
	return inserted, nil
}

// Pending returns up to limit items without an embedding, oldest first.
func (r *ItemRepository) Pending(ctx context.Context, limit int) ([]PendingItem, error) {
	const q = `SELECT id, title, summary FROM items WHERE embedding IS NULL ORDER BY collected_at, id LIMIT $1`
	rows, err := database.Conn(ctx, r.db).QueryContext(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("list pending items: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []PendingItem
	for rows.Next() {
		var p PendingItem
		if err := rows.Scan(&p.ID, &p.Title, &p.Summary); err != nil {
			return nil, fmt.Errorf("scan pending item: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list pending items: %w", err)
	}
	return out, nil
}

// HasPending reports whether any item still lacks an embedding.
func (r *ItemRepository) HasPending(ctx context.Context) (bool, error) {
	var ok bool
	err := database.Conn(ctx, r.db).QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM items WHERE embedding IS NULL)`).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("check pending items: %w", err)
	}
	return ok, nil
}

// SetEmbeddings stores one vector per item, with the model that made them, in
// a single statement. ids and vectors must have the same length.
func (r *ItemRepository) SetEmbeddings(ctx context.Context, ids []uuid.UUID, vectors [][]float32, model string, now time.Time) error {
	if len(ids) != len(vectors) {
		return fmt.Errorf("set embeddings: %d ids but %d vectors", len(ids), len(vectors))
	}
	if len(ids) == 0 {
		return nil
	}
	texts := make([]string, len(vectors))
	for i, v := range vectors {
		texts[i] = pgvector.NewVector(v).String()
	}
	const q = `
		UPDATE items SET embedding = v.embedding::vector, embedding_model = $3, embedded_at = $4
		FROM unnest($1::uuid[], $2::text[]) AS v (id, embedding)
		WHERE items.id = v.id`
	if _, err := database.Conn(ctx, r.db).ExecContext(ctx, q, ids, texts, model, now); err != nil {
		return fmt.Errorf("set embeddings: %w", translate(err))
	}
	return nil
}

// List returns a page of the feed and the total number of matching items.
func (r *ItemRepository) List(ctx context.Context, f ItemFilter, limit, offset int) ([]ItemView, int, error) {
	order, ok := itemOrder[f.Sort]
	if !ok {
		order = itemOrder[ItemSortRecent]
	}

	var where []string
	var args []any
	arg := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}
	if f.SourceID != nil {
		where = append(where, "i.source_id = "+arg(*f.SourceID))
	}
	if f.Since != nil {
		where = append(where, "i.published_at >= "+arg(*f.Since))
	}
	if f.Until != nil {
		where = append(where, "i.published_at < "+arg(*f.Until))
	}
	if f.Query != "" {
		where = append(where, "i.search @@ websearch_to_tsquery('simple', "+arg(f.Query)+")")
	}
	clause := ""
	if len(where) > 0 {
		clause = " WHERE " + strings.Join(where, " AND ")
	}

	conn := database.Conn(ctx, r.db)
	var total int
	if err := conn.QueryRowContext(ctx, `SELECT count(*) FROM items i`+clause, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count items: %w", err)
	}

	q := `
		SELECT i.id, i.source_id, s.kind, s.name, i.url, i.title, i.summary, i.author, i.published_at, i.collected_at,
			i.score, i.comments, i.metrics,
			EXISTS (SELECT 1 FROM report_items ri WHERE ri.item_id = i.id)
				OR EXISTS (SELECT 1 FROM idea_evidence ie WHERE ie.item_id = i.id) AS cited
		FROM items i
		JOIN sources s ON s.id = i.source_id` + clause + `
		ORDER BY ` + order + `
		LIMIT ` + arg(limit) + ` OFFSET ` + arg(offset)
	rows, err := conn.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list items: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]ItemView, 0)
	for rows.Next() {
		var v ItemView
		var metrics []byte
		it := &v.Item
		if err := rows.Scan(&it.ID, &it.SourceID, &v.SourceKind, &v.SourceName, &it.URL, &it.Title, &it.Summary, &it.Author,
			&it.PublishedAt, &it.CollectedAt, &it.Score, &it.Comments, &metrics, &v.Cited); err != nil {
			return nil, 0, fmt.Errorf("scan item: %w", err)
		}
		it.Metrics = metrics
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("list items: %w", err)
	}
	return out, total, nil
}
