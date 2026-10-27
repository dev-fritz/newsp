package handlers

import (
	"net/http"

	"github.com/dev-fritz/newsp/api/internal/apperrors"
	"github.com/dev-fritz/newsp/api/internal/auth"
	"github.com/dev-fritz/newsp/api/internal/dtos"
	"github.com/dev-fritz/newsp/api/internal/services"
)

// ReportHandler serves the caller's market reports.
type ReportHandler struct {
	service *services.ReportService
}

// NewReportHandler builds a ReportHandler.
func NewReportHandler(service *services.ReportService) *ReportHandler {
	return &ReportHandler{service: service}
}

// Create godoc
// @Summary      Generate a report now
// @Description  Queues a report over the last window_hours. Poll the job, or the report, until it is done. One report at a time per user.
// @Tags         reports
// @Accept       json
// @Produce      json
// @Param        request body dtos.CreateReportRequest true "Window and optional focus"
// @Success      202 {object} dtos.ReportCreatedEnvelope
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      409 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/reports [post]
func (h *ReportHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dtos.CreateReportRequest
	if err := decodeJSON(w, r, &req); err != nil {
		respondError(w, r, err)
		return
	}
	actor, _ := auth.FromContext(r.Context())
	rep, jobID, err := h.service.Request(r.Context(), actor, services.ReportRequest{WindowHours: req.WindowHours, Focus: req.Focus})
	if err != nil {
		respondError(w, r, err)
		return
	}
	respondJSON(w, http.StatusAccepted, dtos.ReportCreatedResponse{Report: dtos.NewReportSummaryResponse(rep), JobID: jobID.String()})
}

// List godoc
// @Summary      My reports
// @Tags         reports
// @Produce      json
// @Param        kind   query string false "scheduled or manual"
// @Param        limit  query int    false "Page size (max 100)"
// @Param        offset query int    false "Offset"
// @Success      200 {object} dtos.ReportListEnvelope
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/reports [get]
func (h *ReportHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := parsePage(r)
	if err != nil {
		respondError(w, r, err)
		return
	}
	kind := r.URL.Query().Get("kind")
	if kind != "" && kind != "scheduled" && kind != "manual" {
		respondError(w, r, apperrors.Validation("invalid query", map[string]string{"kind": "oneof"}))
		return
	}
	actor, _ := auth.FromContext(r.Context())
	reports, total, err := h.service.List(r.Context(), actor, kind, limit, offset)
	if err != nil {
		respondError(w, r, err)
		return
	}
	out := make([]dtos.ReportSummaryResponse, len(reports))
	for i, rep := range reports {
		out[i] = dtos.NewReportSummaryResponse(rep)
	}
	respondPage(w, out, dtos.PageMeta{Total: total, Limit: limit, Offset: offset})
}

// Get godoc
// @Summary      One of my reports, with its citations
// @Tags         reports
// @Produce      json
// @Param        id path string true "Report ID"
// @Success      200 {object} dtos.ReportEnvelope
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      404 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/reports/{id} [get]
func (h *ReportHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuidParam(r, "id")
	if err != nil {
		respondError(w, r, err)
		return
	}
	actor, _ := auth.FromContext(r.Context())
	detail, err := h.service.Get(r.Context(), actor, id)
	if err != nil {
		respondError(w, r, err)
		return
	}
	rep := detail.Report
	citations := make([]dtos.CitationResponse, len(detail.Citations))
	for i, c := range detail.Citations {
		citations[i] = dtos.CitationResponse{
			Ref: c.Ref, ItemID: c.ItemID.String(), URL: c.URL, Title: c.Title, SourceKind: c.SourceKind, SourceName: c.SourceName,
			Author: c.Author, PublishedAt: c.PublishedAt, Score: c.Score, Comments: c.Comments, Metrics: c.Metrics,
			Excerpt: c.Excerpt,
		}
	}
	respondJSON(w, http.StatusOK, dtos.ReportResponse{
		ReportSummaryResponse: dtos.NewReportSummaryResponse(rep), ContentMD: rep.ContentMD, OutputLang: rep.OutputLang,
		ModelName: rep.ModelName, ItemsConsidered: rep.ItemsConsidered, Citations: citations,
	})
}

// Delete godoc
// @Summary      Delete one of my reports
// @Description  Permanently removes a finished (done or failed) report and its citations; collected items are kept. Reports still being generated answer 409 REPORT_IN_PROGRESS.
// @Tags         reports
// @Produce      json
// @Param        id path string true "Report ID"
// @Success      204
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      404 {object} dtos.ErrorResponse
// @Failure      409 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/reports/{id} [delete]
func (h *ReportHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuidParam(r, "id")
	if err != nil {
		respondError(w, r, err)
		return
	}
	actor, _ := auth.FromContext(r.Context())
	if err := h.service.Delete(r.Context(), actor, id); err != nil {
		respondError(w, r, err)
		return
	}
	respondNoContent(w)
}
