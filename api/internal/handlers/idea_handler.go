package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/dev-fritz/newsp/api/internal/apperrors"
	"github.com/dev-fritz/newsp/api/internal/auth"
	"github.com/dev-fritz/newsp/api/internal/dtos"
	"github.com/dev-fritz/newsp/api/internal/models"
	"github.com/dev-fritz/newsp/api/internal/services"
)

// IdeaHandler serves the caller's project ideas.
type IdeaHandler struct {
	service *services.IdeaService
}

// NewIdeaHandler builds an IdeaHandler.
func NewIdeaHandler(service *services.IdeaService) *IdeaHandler {
	return &IdeaHandler{service: service}
}

func versionResponse(d services.IdeaVersionDetail) dtos.IdeaVersionResponse {
	v := d.Version
	evidence := make([]dtos.EvidenceResponse, len(d.Evidence))
	for i, e := range d.Evidence {
		evidence[i] = dtos.EvidenceResponse{
			Ref: e.Ref, ItemID: e.ItemID.String(), URL: e.URL, Title: e.Title, SourceKind: e.SourceKind,
			SourceName: e.SourceName, PublishedAt: e.PublishedAt, Score: e.Score, Comments: e.Comments,
			Excerpt: e.Excerpt,
		}
	}
	return dtos.IdeaVersionResponse{
		Version: v.Version, Origin: v.Origin, Title: v.Title, Summary: v.Summary, Difficulty: v.Difficulty,
		EffortWeeks: v.EffortWeeks, Content: d.Content, ChangeNote: v.ChangeNote, Similarity: v.Similarity,
		ModelName: v.ModelName, CreatedAt: v.CreatedAt, Evidence: evidence,
	}
}

func ideaResponse(d services.IdeaDetail) dtos.IdeaResponse {
	versions := make([]dtos.IdeaVersionSummaryResponse, len(d.Versions))
	for i, v := range d.Versions {
		versions[i] = dtos.IdeaVersionSummaryResponse{
			Version: v.Version, Origin: v.Origin, ChangeNote: v.ChangeNote, Similarity: v.Similarity, ModelName: v.ModelName, CreatedAt: v.CreatedAt,
		}
	}
	return dtos.IdeaResponse{
		ID: d.Idea.ID.String(), Status: d.Idea.Status, CurrentVersion: d.Idea.CurrentVersion, MergedCandidates: d.Idea.MergedCandidates,
		CreatedAt: d.Idea.CreatedAt, UpdatedAt: d.Idea.UpdatedAt, Current: versionResponse(d.Current), Versions: versions,
	}
}

// List godoc
// @Summary      My ideas
// @Tags         ideas
// @Produce      json
// @Param        status query string false "active (default: everything but discarded), new, saved, discarded or all"
// @Param        q      query string false "Search in title and summary"
// @Param        limit  query int    false "Page size (max 100)"
// @Param        offset query int    false "Offset"
// @Success      200 {object} dtos.IdeaListEnvelope
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/ideas [get]
func (h *IdeaHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := parsePage(r)
	if err != nil {
		respondError(w, r, err)
		return
	}
	q := r.URL.Query()
	status := q.Get("status")
	switch status {
	case "":
		status = "active"
	case "all":
		status = ""
	case "active", "new", "saved", "discarded":
	default:
		respondError(w, r, apperrors.Validation("invalid query", map[string]string{"status": "oneof"}))
		return
	}
	if len(q.Get("q")) > 200 {
		respondError(w, r, apperrors.Validation("invalid query", map[string]string{"q": "max"}))
		return
	}
	actor, _ := auth.FromContext(r.Context())
	items, total, err := h.service.List(r.Context(), actor, status, q.Get("q"), limit, offset)
	if err != nil {
		respondError(w, r, err)
		return
	}
	out := make([]dtos.IdeaListItemResponse, len(items))
	for i, it := range items {
		out[i] = dtos.IdeaListItemResponse{
			ID: it.Idea.ID.String(), Status: it.Idea.Status, Title: it.Idea.Title, Summary: it.Idea.Summary,
			CurrentVersion: it.Idea.CurrentVersion, LastOrigin: it.Origin, Difficulty: it.Difficulty, EffortWeeks: it.EffortWeeks,
			EvidenceCount: it.EvidenceCount, PrimaryAudience: it.Audience, MergedCandidates: it.Idea.MergedCandidates,
			CreatedAt: it.Idea.CreatedAt, UpdatedAt: it.Idea.UpdatedAt,
		}
	}
	respondPage(w, out, dtos.PageMeta{Total: total, Limit: limit, Offset: offset})
}

// Generate godoc
// @Summary      Generate ideas now
// @Description  Queues a generation, optionally from one of the caller's finished reports (report_id) or from news items the caller picked (item_ids, max 5, not allowed with report_id; they are cited first and the window and focus fill the rest). Ideas equivalent to existing ones become new versions instead of duplicates. While one runs, the same job is returned.
// @Tags         ideas
// @Accept       json
// @Produce      json
// @Param        request body dtos.GenerateIdeasRequest true "How many and about what"
// @Success      202 {object} dtos.JobAcceptedEnvelope
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      404 {object} dtos.ErrorResponse "report_id is not one of the caller's reports"
// @Security     BearerAuth
// @Router       /api/v1/ideas/generate [post]
func (h *IdeaHandler) Generate(w http.ResponseWriter, r *http.Request) {
	var req dtos.GenerateIdeasRequest
	if err := decodeJSON(w, r, &req); err != nil {
		respondError(w, r, err)
		return
	}
	if req.WindowDays == 0 {
		req.WindowDays = 7
	}
	in := services.IdeaGenerationRequest{Count: req.Count, Focus: req.Focus, WindowDays: req.WindowDays}
	if req.ReportID != nil {
		reportID, err := uuid.Parse(*req.ReportID)
		if err != nil {
			respondError(w, r, apperrors.Validation("invalid request", map[string]string{"report_id": "uuid"}))
			return
		}
		in.ReportID = &reportID
	}
	itemIDs, err := parseItemIDs(req.ItemIDs)
	if err != nil {
		respondError(w, r, err)
		return
	}
	in.ItemIDs = itemIDs
	actor, _ := auth.FromContext(r.Context())
	jobID, err := h.service.RequestGeneration(r.Context(), actor, in)
	if err != nil {
		respondError(w, r, err)
		return
	}
	respondJSON(w, http.StatusAccepted, dtos.JobAccepted{JobID: jobID.String()})
}

// parseItemIDs parses the picked item ids, dropping repeats and keeping the order.
func parseItemIDs(raw []string) ([]uuid.UUID, error) {
	var out []uuid.UUID
	seen := make(map[uuid.UUID]bool, len(raw))
	for _, s := range raw {
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, apperrors.Validation("invalid request", map[string]string{"item_ids": "uuid"})
		}
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out, nil
}

// Get godoc
// @Summary      One of my ideas
// @Tags         ideas
// @Produce      json
// @Param        id path string true "Idea ID"
// @Success      200 {object} dtos.IdeaEnvelope
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      404 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/ideas/{id} [get]
func (h *IdeaHandler) Get(w http.ResponseWriter, r *http.Request) {
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
	respondJSON(w, http.StatusOK, ideaResponse(detail))
}

// GetVersion godoc
// @Summary      One version of my idea (for comparing versions)
// @Tags         ideas
// @Produce      json
// @Param        id      path string true "Idea ID"
// @Param        version path int    true "Version number"
// @Success      200 {object} dtos.IdeaVersionEnvelope
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      404 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/ideas/{id}/versions/{version} [get]
func (h *IdeaHandler) GetVersion(w http.ResponseWriter, r *http.Request) {
	id, err := uuidParam(r, "id")
	if err != nil {
		respondError(w, r, err)
		return
	}
	number, err := strconv.Atoi(chi.URLParam(r, "version"))
	if err != nil || number < 1 {
		respondError(w, r, apperrors.Validation("invalid path parameter", map[string]string{"version": "min"}))
		return
	}
	actor, _ := auth.FromContext(r.Context())
	detail, err := h.service.GetVersion(r.Context(), actor, id, number)
	if err != nil {
		respondError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, versionResponse(detail))
}

// SetStatus godoc
// @Summary      Triage an idea
// @Description  Discarded ideas leave the default list but still prevent duplicates.
// @Tags         ideas
// @Accept       json
// @Produce      json
// @Param        id      path string true "Idea ID"
// @Param        request body dtos.IdeaStatusRequest true "Status"
// @Success      204
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      404 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/ideas/{id}/status [patch]
func (h *IdeaHandler) SetStatus(w http.ResponseWriter, r *http.Request) {
	id, err := uuidParam(r, "id")
	if err != nil {
		respondError(w, r, err)
		return
	}
	var req dtos.IdeaStatusRequest
	if err := decodeJSON(w, r, &req); err != nil {
		respondError(w, r, err)
		return
	}
	actor, _ := auth.FromContext(r.Context())
	if _, err := h.service.SetStatus(r.Context(), actor, id, req.Status); err != nil {
		respondError(w, r, err)
		return
	}
	respondNoContent(w)
}

// Edit godoc
// @Summary      Edit an idea by hand
// @Description  Stores a new version with origin "manual"; evidence carries over.
// @Tags         ideas
// @Accept       json
// @Produce      json
// @Param        id      path string true "Idea ID"
// @Param        request body dtos.EditIdeaRequest true "New content"
// @Success      200 {object} dtos.IdeaEnvelope
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      404 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/ideas/{id} [put]
func (h *IdeaHandler) Edit(w http.ResponseWriter, r *http.Request) {
	id, err := uuidParam(r, "id")
	if err != nil {
		respondError(w, r, err)
		return
	}
	var req dtos.EditIdeaRequest
	if err := decodeJSON(w, r, &req); err != nil {
		respondError(w, r, err)
		return
	}
	c := req.Content
	actor, _ := auth.FromContext(r.Context())
	detail, err := h.service.Edit(r.Context(), actor, id, services.IdeaEdit{
		Title: req.Title, Summary: req.Summary, Difficulty: req.Difficulty, EffortWeeks: req.EffortWeeks,
		Content: models.IdeaContent{
			WhyNow: c.WhyNow, Audience: c.Audience, How: c.How, Differentiator: c.Differentiator, Stack: c.Stack,
			ArchitectureSteps: c.ArchitectureSteps, MVPPlan: c.MVPPlan, Monetization: c.Monetization, Competitors: c.Competitors,
		},
	})
	if err != nil {
		respondError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, ideaResponse(detail))
}

// Deepen godoc
// @Summary      Deepen an idea
// @Description  Queues a deeper version using recent sources. Poll the job.
// @Tags         ideas
// @Accept       json
// @Produce      json
// @Param        id      path string true "Idea ID"
// @Param        request body dtos.DeepenIdeaRequest true "What to deepen"
// @Success      202 {object} dtos.JobAcceptedEnvelope
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      404 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/ideas/{id}/deepen [post]
func (h *IdeaHandler) Deepen(w http.ResponseWriter, r *http.Request) {
	id, err := uuidParam(r, "id")
	if err != nil {
		respondError(w, r, err)
		return
	}
	var req dtos.DeepenIdeaRequest
	if err := decodeJSON(w, r, &req); err != nil {
		respondError(w, r, err)
		return
	}
	actor, _ := auth.FromContext(r.Context())
	jobID, err := h.service.RequestDeepen(r.Context(), actor, id, services.DeepenRequest{Aspects: req.Aspects, Instruction: req.Instruction})
	if err != nil {
		respondError(w, r, err)
		return
	}
	respondJSON(w, http.StatusAccepted, dtos.JobAccepted{JobID: jobID.String()})
}
