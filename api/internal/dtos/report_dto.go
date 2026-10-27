package dtos

import (
	"encoding/json"
	"time"

	"github.com/dev-fritz/newsp/api/internal/models"
)

// CreateReportRequest asks for a report now.
type CreateReportRequest struct {
	WindowHours int    `json:"window_hours" validate:"required,min=1,max=720" example:"24"`
	Focus       string `json:"focus" validate:"max=200" example:"fintech no Brasil"`
}

// ReportSummaryResponse is a report in a list.
type ReportSummaryResponse struct {
	ID          string              `json:"id"`
	Kind        models.ReportKind   `json:"kind" enums:"scheduled,manual"`
	Status      models.ReportStatus `json:"status" enums:"pending,running,done,failed"`
	WindowStart time.Time           `json:"window_start"`
	WindowEnd   time.Time           `json:"window_end"`
	Focus       *string             `json:"focus"`
	Title       *string             `json:"title"`
	Summary     *string             `json:"summary"`
	// Error is a stable code (NO_ITEMS, REPORT_MODEL_NOT_SET, AI_REFUSED,
	// AI_INVALID_OUTPUT, AI_FAILURE) for failed reports.
	Error       *string    `json:"error"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

// NewReportSummaryResponse maps a report without its content.
func NewReportSummaryResponse(r models.Report) ReportSummaryResponse {
	return ReportSummaryResponse{
		ID: r.ID.String(), Kind: r.Kind, Status: r.Status, WindowStart: r.WindowStart, WindowEnd: r.WindowEnd,
		Focus: r.Focus, Title: r.Title, Summary: r.Summary, Error: r.Error, CreatedAt: r.CreatedAt, CompletedAt: r.CompletedAt,
	}
}

// CitationResponse is an item cited by a report as [ref].
type CitationResponse struct {
	Ref         int             `json:"ref"`
	ItemID      string          `json:"item_id"`
	URL         string          `json:"url"`
	Title       string          `json:"title"`
	SourceKind  string          `json:"source_kind"`
	SourceName  string          `json:"source_name"`
	Author      string          `json:"author"`
	PublishedAt time.Time       `json:"published_at"`
	Score       *int            `json:"score"`
	Comments    *int            `json:"comments"`
	Metrics     json.RawMessage `json:"metrics" swaggertype:"object"`
	// Excerpt is a short plain-text preview of the item ("" when it has none).
	Excerpt string `json:"excerpt"`
}

// ReportResponse is a full report.
type ReportResponse struct {
	ReportSummaryResponse
	ContentMD       *string            `json:"content_md"`
	OutputLang      string             `json:"output_lang"`
	ModelName       *string            `json:"model_name"`
	ItemsConsidered int                `json:"items_considered"`
	Citations       []CitationResponse `json:"citations"`
}

// ReportCreatedResponse is returned when a report is queued.
type ReportCreatedResponse struct {
	Report ReportSummaryResponse `json:"report"`
	JobID  string                `json:"job_id"`
}

// ReportEnvelope wraps ReportResponse.
type ReportEnvelope struct {
	Data ReportResponse `json:"data"`
}

// ReportListEnvelope is a page of reports.
type ReportListEnvelope struct {
	Data []ReportSummaryResponse `json:"data"`
	Meta PageMeta                `json:"meta"`
}

// ReportCreatedEnvelope wraps ReportCreatedResponse.
type ReportCreatedEnvelope struct {
	Data ReportCreatedResponse `json:"data"`
}
