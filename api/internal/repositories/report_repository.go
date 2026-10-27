package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/dev-fritz/newsp/api/internal/database"
	"github.com/dev-fritz/newsp/api/internal/models"
)

const reportColumns = `id, user_id, kind, status, window_start, window_end, focus, title, summary, content_md,
	output_lang, model_id, model_name, items_considered, error, created_at, completed_at`

// ReportRepository persists reports and their citations.
type ReportRepository struct {
	db *sql.DB
}

// NewReportRepository builds a ReportRepository.
func NewReportRepository(db *sql.DB) *ReportRepository {
	return &ReportRepository{db: db}
}

func scanReport(row interface{ Scan(...any) error }) (models.Report, error) {
	var r models.Report
	err := row.Scan(&r.ID, &r.UserID, &r.Kind, &r.Status, &r.WindowStart, &r.WindowEnd, &r.Focus, &r.Title, &r.Summary,
		&r.ContentMD, &r.OutputLang, &r.ModelID, &r.ModelName, &r.ItemsConsidered, &r.Error, &r.CreatedAt, &r.CompletedAt)
	return r, err
}

// Create inserts a pending report. It returns ErrDuplicate when the user
// already has a pending or running report (reports_one_active_per_user). The
// conflict is absorbed with ON CONFLICT DO NOTHING instead of raised, so it does
// not abort the caller's transaction.
func (r *ReportRepository) Create(ctx context.Context, rep models.Report) error {
	const q = `INSERT INTO reports (id, user_id, kind, status, window_start, window_end, focus, output_lang, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (user_id) WHERE status IN ('pending', 'running') DO NOTHING`
	res, err := database.Conn(ctx, r.db).ExecContext(ctx, q, rep.ID, rep.UserID, rep.Kind, rep.Status, rep.WindowStart,
		rep.WindowEnd, rep.Focus, rep.OutputLang, rep.CreatedAt)
	if err != nil {
		return fmt.Errorf("create report: %w", translate(err))
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("create report: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("create report: %w", ErrDuplicate)
	}
	return nil
}

// Get returns a report.
func (r *ReportRepository) Get(ctx context.Context, id uuid.UUID) (models.Report, error) {
	rep, err := scanReport(database.Conn(ctx, r.db).QueryRowContext(ctx, `SELECT `+reportColumns+` FROM reports WHERE id = $1`, id))
	if err != nil {
		return models.Report{}, fmt.Errorf("get report: %w", translate(err))
	}
	return rep, nil
}

// GetForUser returns a report owned by userID; other users' reports are ErrNotFound.
func (r *ReportRepository) GetForUser(ctx context.Context, id, userID uuid.UUID) (models.Report, error) {
	q := `SELECT ` + reportColumns + ` FROM reports WHERE id = $1 AND user_id = $2`
	rep, err := scanReport(database.Conn(ctx, r.db).QueryRowContext(ctx, q, id, userID))
	if err != nil {
		return models.Report{}, fmt.Errorf("get report: %w", translate(err))
	}
	return rep, nil
}

// DeleteForUser permanently removes a finished (done or failed) report owned by
// userID, cascading to its citations. It returns ErrNotFound when no such
// report exists, belongs to someone else or is still pending or running.
func (r *ReportRepository) DeleteForUser(ctx context.Context, id, userID uuid.UUID) error {
	const q = `DELETE FROM reports WHERE id = $1 AND user_id = $2 AND status IN ('done', 'failed')`
	res, err := database.Conn(ctx, r.db).ExecContext(ctx, q, id, userID)
	return checkAffected(res, err, "delete report")
}

// ListForUser returns a page of a user's reports, newest first. kind filters
// when not empty.
func (r *ReportRepository) ListForUser(ctx context.Context, userID uuid.UUID, kind string, limit, offset int) ([]models.Report, int, error) {
	conn := database.Conn(ctx, r.db)
	const where = `user_id = $1 AND ($2 = '' OR kind = $2)`
	var total int
	if err := conn.QueryRowContext(ctx, `SELECT count(*) FROM reports WHERE `+where, userID, kind).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count reports: %w", err)
	}
	q := `SELECT ` + reportColumns + ` FROM reports WHERE ` + where + ` ORDER BY created_at DESC, id LIMIT $3 OFFSET $4`
	rows, err := conn.QueryContext(ctx, q, userID, kind, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list reports: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]models.Report, 0)
	for rows.Next() {
		rep, err := scanReport(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan report: %w", err)
		}
		out = append(out, rep)
	}
	return out, total, rows.Err()
}

// HasActive reports whether the user already has a pending or running report.
func (r *ReportRepository) HasActive(ctx context.Context, userID uuid.UUID) (bool, error) {
	var exists bool
	const q = `SELECT EXISTS (SELECT 1 FROM reports WHERE user_id = $1 AND status IN ('pending', 'running'))`
	if err := database.Conn(ctx, r.db).QueryRowContext(ctx, q, userID).Scan(&exists); err != nil {
		return false, fmt.Errorf("active report: %w", err)
	}
	return exists, nil
}

// MarkRunning flags a report as being generated.
func (r *ReportRepository) MarkRunning(ctx context.Context, id uuid.UUID) error {
	res, err := database.Conn(ctx, r.db).ExecContext(ctx, `UPDATE reports SET status = 'running', error = NULL WHERE id = $1 AND status <> 'done'`, id)
	return checkAffected(res, err, "mark report running")
}

// Complete stores the generated report and its citations. Call it in a transaction.
func (r *ReportRepository) Complete(ctx context.Context, rep models.Report, citations []models.ReportCitation) error {
	conn := database.Conn(ctx, r.db)
	const q = `UPDATE reports SET status = 'done', title = $2, summary = $3, content_md = $4, model_id = $5, model_name = $6,
		items_considered = $7, error = NULL, completed_at = $8 WHERE id = $1`
	res, err := conn.ExecContext(ctx, q, rep.ID, rep.Title, rep.Summary, rep.ContentMD, rep.ModelID, rep.ModelName, rep.ItemsConsidered, rep.CompletedAt)
	if err := checkAffected(res, err, "complete report"); err != nil {
		return err
	}
	if len(citations) == 0 {
		return nil
	}
	items := make([]string, len(citations))
	refs := make([]int32, len(citations))
	for i, c := range citations {
		items[i], refs[i] = c.ItemID.String(), int32(c.Ref) //nolint:gosec // refs are small citation numbers
	}
	const cite = `INSERT INTO report_items (report_id, item_id, ref) SELECT $1, unnest($2::uuid[]), unnest($3::int[])`
	if _, err := conn.ExecContext(ctx, cite, rep.ID, items, refs); err != nil {
		return fmt.Errorf("save citations: %w", translate(err))
	}
	return nil
}

// Fail marks a report as failed with a readable message.
func (r *ReportRepository) Fail(ctx context.Context, id uuid.UUID, message string, at time.Time) error {
	const q = `UPDATE reports SET status = 'failed', error = $2, completed_at = $3 WHERE id = $1 AND status <> 'done'`
	res, err := database.Conn(ctx, r.db).ExecContext(ctx, q, id, message, at)
	return checkAffected(res, err, "fail report")
}

// FailOrphaned marks as failed every pending or running report that has no
// pending or running job, and returns how many it changed. A report is created
// together with its job, so one without an active job can never finish. The
// lookup uses the partial unique index jobs_dedupe_active_key.
func (r *ReportRepository) FailOrphaned(ctx context.Context, message string, at time.Time) (int64, error) {
	const q = `UPDATE reports SET status = 'failed', error = $1, completed_at = $2
		WHERE status IN ('pending', 'running')
		AND NOT EXISTS (
			SELECT 1 FROM jobs j
			WHERE j.dedupe_key = 'report:' || reports.id::text AND j.status IN ('pending', 'running'))`
	res, err := database.Conn(ctx, r.db).ExecContext(ctx, q, message, at)
	if err != nil {
		return 0, fmt.Errorf("fail orphaned reports: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("fail orphaned reports: %w", err)
	}
	return n, nil
}

// PreviousSummary returns the summary and title of the user's latest finished
// report created before `before`, if any.
func (r *ReportRepository) PreviousSummary(ctx context.Context, userID uuid.UUID, before time.Time) (title, summary string, err error) {
	const q = `SELECT COALESCE(title, ''), COALESCE(summary, '') FROM reports
		WHERE user_id = $1 AND status = 'done' AND created_at < $2 ORDER BY created_at DESC LIMIT 1`
	err = database.Conn(ctx, r.db).QueryRowContext(ctx, q, userID, before).Scan(&title, &summary)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("previous report: %w", err)
	}
	return title, summary, nil
}

// CitedItem is a cited item with the fields the reader needs.
type CitedItem struct {
	Ref         int
	ItemID      uuid.UUID
	URL         string
	Title       string
	SourceKind  string
	SourceName  string
	Author      string
	PublishedAt time.Time
	Score       *int
	Comments    *int
	Metrics     json.RawMessage
	// Excerpt is a short plain-text preview of the item ("" when it has none).
	Excerpt string
}

// Citations returns the items cited by a report, ordered by reference number.
func (r *ReportRepository) Citations(ctx context.Context, reportID uuid.UUID) ([]CitedItem, error) {
	const q = `
		SELECT ri.ref, i.id, i.url, i.title, s.kind, s.name, i.author, i.published_at, i.score, i.comments, i.metrics, ` + excerptSQL + `
		FROM report_items ri JOIN items i ON i.id = ri.item_id JOIN sources s ON s.id = i.source_id
		WHERE ri.report_id = $1 ORDER BY ri.ref`
	rows, err := database.Conn(ctx, r.db).QueryContext(ctx, q, reportID)
	if err != nil {
		return nil, fmt.Errorf("report citations: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []CitedItem
	for rows.Next() {
		var c CitedItem
		var metrics []byte
		if err := rows.Scan(&c.Ref, &c.ItemID, &c.URL, &c.Title, &c.SourceKind, &c.SourceName, &c.Author, &c.PublishedAt, &c.Score, &c.Comments, &metrics, &c.Excerpt); err != nil {
			return nil, fmt.Errorf("scan citation: %w", err)
		}
		c.Metrics = metrics
		c.Excerpt = excerpt(c.Excerpt)
		out = append(out, c)
	}
	return out, rows.Err()
}
