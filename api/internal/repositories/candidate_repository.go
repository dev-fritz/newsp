package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/dev-fritz/newsp/api/internal/database"
)

// Candidate is an item ranked for a report or an idea prompt.
type Candidate struct {
	ItemID      uuid.UUID
	URL         string
	Title       string
	Summary     string
	ContentText *string
	ContentDone bool // extraction already attempted
	Author      string
	SourceKind  string
	SourceName  string
	PublishedAt time.Time
	Score       *int
	Comments    *int
	Rank        float64
}

// CandidateQuery selects and ranks items of a time window for one user.
type CandidateQuery struct {
	UserID      uuid.UUID
	WindowStart time.Time
	WindowEnd   time.Time
	// Interest is the embedding to rank against (topics or a focus); nil ranks
	// by engagement and recency only.
	Interest  []float32
	Limit     int
	PerSource int // diversity cap
	// SkipCitedBefore, when set, leaves out the items (and other items with the
	// same URL) cited by the user's finished reports created before it, so a
	// report never retells what an earlier one covered.
	SkipCitedBefore time.Time
}

// CandidateRepository ranks collected items and caches their article text.
type CandidateRepository struct {
	db *sql.DB
}

// NewCandidateRepository builds a CandidateRepository.
func NewCandidateRepository(db *sql.DB) *CandidateRepository {
	return &CandidateRepository{db: db}
}

// Rank returns the best items of the window for the user (excluded sources and,
// with SkipCitedBefore, already cited items skipped) and how many items the
// window had in total.
//
// Score: with an interest vector, 0.55·similarity + 0.30·engagement + 0.15·recency;
// without, 0.70·engagement + 0.30·recency. Engagement is the percentile within
// the item's own source so one large community does not drown the others.
func (r *CandidateRepository) Rank(ctx context.Context, q CandidateQuery) ([]Candidate, int, error) {
	var interest, citedBefore any
	if q.Interest != nil {
		interest = VectorLiteral(q.Interest)
	}
	if !q.SkipCitedBefore.IsZero() {
		citedBefore = q.SkipCitedBefore
	}
	const query = `
		WITH windowed AS (
			SELECT i.*, s.kind AS source_kind, s.name AS source_name
			FROM items i JOIN sources s ON s.id = i.source_id
			WHERE i.published_at >= $2 AND i.published_at < $3
			  AND NOT EXISTS (SELECT 1 FROM user_source_exclusions e WHERE e.user_id = $1 AND e.source_id = i.source_id)
			  AND ($7::timestamptz IS NULL OR NOT EXISTS (
				SELECT 1 FROM report_items ri
				JOIN reports r ON r.id = ri.report_id
				JOIN items c ON c.id = ri.item_id
				WHERE r.user_id = $1 AND r.status = 'done' AND r.created_at < $7::timestamptz
				  AND (c.id = i.id OR c.url = i.url)))
		), scored AS (
			SELECT w.*,
				percent_rank() OVER (PARTITION BY w.source_id ORDER BY COALESCE(w.score, 0) + COALESCE(w.comments, 0)) AS engagement,
				GREATEST(0, 1 - EXTRACT(EPOCH FROM ($3 - w.published_at)) / GREATEST(EXTRACT(EPOCH FROM ($3 - $2)), 1)) AS recency,
				CASE WHEN $4::vector IS NOT NULL AND w.embedding IS NOT NULL THEN 1 - (w.embedding <=> $4::vector) END AS similarity
			FROM windowed w
		), ranked AS (
			SELECT sc.*,
				CASE WHEN $4::vector IS NOT NULL
					THEN 0.55 * COALESCE(sc.similarity, 0) + 0.30 * sc.engagement + 0.15 * sc.recency
					ELSE 0.70 * sc.engagement + 0.30 * sc.recency END AS rank_score,
				count(*) OVER () AS total
			FROM scored sc
		), diverse AS (
			SELECT rk.*, row_number() OVER (PARTITION BY rk.source_id ORDER BY rk.rank_score DESC) AS per_source
			FROM ranked rk
		)
		SELECT id, url, title, summary, content_text, content_fetched_at IS NOT NULL, author, source_kind, source_name,
			published_at, score, comments, rank_score, total
		FROM diverse WHERE per_source <= $5
		ORDER BY rank_score DESC, published_at DESC
		LIMIT $6`
	rows, err := database.Conn(ctx, r.db).QueryContext(ctx, query, q.UserID, q.WindowStart, q.WindowEnd, interest, q.PerSource, q.Limit, citedBefore)
	if err != nil {
		return nil, 0, fmt.Errorf("rank candidates: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var (
		out   []Candidate
		total int
	)
	for rows.Next() {
		var c Candidate
		if err := rows.Scan(&c.ItemID, &c.URL, &c.Title, &c.Summary, &c.ContentText, &c.ContentDone, &c.Author, &c.SourceKind,
			&c.SourceName, &c.PublishedAt, &c.Score, &c.Comments, &c.Rank, &total); err != nil {
			return nil, 0, fmt.Errorf("scan candidate: %w", err)
		}
		out = append(out, c)
	}
	return out, total, rows.Err()
}

// Cited returns the items a report cites, in reference order, as candidates.
func (r *CandidateRepository) Cited(ctx context.Context, reportID uuid.UUID) ([]Candidate, error) {
	const query = `
		SELECT i.id, i.url, i.title, i.summary, i.content_text, i.content_fetched_at IS NOT NULL, i.author, s.kind, s.name,
			i.published_at, i.score, i.comments
		FROM report_items ri JOIN items i ON i.id = ri.item_id JOIN sources s ON s.id = i.source_id
		WHERE ri.report_id = $1
		ORDER BY ri.ref`
	rows, err := database.Conn(ctx, r.db).QueryContext(ctx, query, reportID)
	if err != nil {
		return nil, fmt.Errorf("cited candidates: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []Candidate
	for rows.Next() {
		var c Candidate
		if err := rows.Scan(&c.ItemID, &c.URL, &c.Title, &c.Summary, &c.ContentText, &c.ContentDone, &c.Author, &c.SourceKind,
			&c.SourceName, &c.PublishedAt, &c.Score, &c.Comments); err != nil {
			return nil, fmt.Errorf("scan cited candidate: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ByIDs returns the given items as candidates, in the order of ids. Ids that
// do not exist are left out. Items belong to the instance, not to a user.
func (r *CandidateRepository) ByIDs(ctx context.Context, ids []uuid.UUID) ([]Candidate, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	texts := make([]string, len(ids))
	for i, id := range ids {
		texts[i] = id.String()
	}
	const query = `
		SELECT i.id, i.url, i.title, i.summary, i.content_text, i.content_fetched_at IS NOT NULL, i.author, s.kind, s.name,
			i.published_at, i.score, i.comments
		FROM items i JOIN sources s ON s.id = i.source_id
		WHERE i.id = ANY($1::uuid[])`
	rows, err := database.Conn(ctx, r.db).QueryContext(ctx, query, texts)
	if err != nil {
		return nil, fmt.Errorf("candidates by ids: %w", err)
	}
	defer func() { _ = rows.Close() }()
	byID := make(map[uuid.UUID]Candidate, len(ids))
	for rows.Next() {
		var c Candidate
		if err := rows.Scan(&c.ItemID, &c.URL, &c.Title, &c.Summary, &c.ContentText, &c.ContentDone, &c.Author, &c.SourceKind,
			&c.SourceName, &c.PublishedAt, &c.Score, &c.Comments); err != nil {
			return nil, fmt.Errorf("scan candidate: %w", err)
		}
		byID[c.ItemID] = c
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]Candidate, 0, len(byID))
	for _, id := range ids {
		if c, ok := byID[id]; ok {
			out = append(out, c)
			delete(byID, id) // a repeated id yields the item once
		}
	}
	return out, nil
}

// SetContent caches the extracted article text (or the extraction error).
func (r *CandidateRepository) SetContent(ctx context.Context, itemID uuid.UUID, text, extractErr *string, at time.Time) error {
	const q = `UPDATE items SET content_text = $2, content_error = $3, content_fetched_at = $4 WHERE id = $1`
	res, err := database.Conn(ctx, r.db).ExecContext(ctx, q, itemID, text, extractErr, at)
	return checkAffected(res, err, "set item content")
}
