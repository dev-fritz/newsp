package handlers

import (
	"net/http"

	"github.com/dev-fritz/newsp/api/internal/auth"
	"github.com/dev-fritz/newsp/api/internal/dtos"
	"github.com/dev-fritz/newsp/api/internal/services"
)

// SourceHandler serves source administration and the public source list.
type SourceHandler struct {
	service *services.SourceService
}

// NewSourceHandler builds a SourceHandler.
func NewSourceHandler(service *services.SourceService) *SourceHandler {
	return &SourceHandler{service: service}
}

// List godoc
// @Summary      List sources with their collection status
// @Description  Every source by name, with the number of items collected in the last 24 hours.
// @Tags         admin-sources
// @Produce      json
// @Success      200 {object} dtos.SourceListEnvelope
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      403 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/admin/sources [get]
func (h *SourceHandler) List(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.FromContext(r.Context())
	views, err := h.service.List(r.Context(), actor)
	if err != nil {
		respondError(w, r, err)
		return
	}
	out := make([]dtos.SourceResponse, len(views))
	for i, v := range views {
		out[i] = dtos.NewSourceResponse(v.Source, v.Items)
	}
	respondJSON(w, http.StatusOK, out)
}

// Create godoc
// @Summary      Register a source
// @Description  The config depends on the kind and is validated and completed with defaults by its connector; invalid fields are reported as config.<field>. Kinds that take a credential (github, producthunt, bluesky search) receive it in secret, which is encrypted and never returned (see has_secret).
// @Tags         admin-sources
// @Accept       json
// @Produce      json
// @Param        request body dtos.CreateSourceRequest true "Source"
// @Success      201 {object} dtos.SourceEnvelope
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      403 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/admin/sources [post]
func (h *SourceHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dtos.CreateSourceRequest
	if err := decodeJSON(w, r, &req); err != nil {
		respondError(w, r, err)
		return
	}
	actor, _ := auth.FromContext(r.Context())
	view, err := h.service.Create(r.Context(), actor, services.SourceInput{
		Kind: req.Kind, Name: req.Name, Config: req.Config, Secret: req.Secret, IntervalMinutes: req.IntervalMinutes, Enabled: req.Enabled,
	})
	if err != nil {
		respondError(w, r, err)
		return
	}
	respondJSON(w, http.StatusCreated, dtos.NewSourceResponse(view.Source, view.Items))
}

// Update godoc
// @Summary      Update a source
// @Description  Omitted fields stay as they are. Changing the config re-validates it; enabling a disabled source schedules it to run soon. secret replaces the credential and an empty secret removes it.
// @Tags         admin-sources
// @Accept       json
// @Produce      json
// @Param        id      path string true "Source ID"
// @Param        request body dtos.UpdateSourceRequest true "Changes"
// @Success      200 {object} dtos.SourceEnvelope
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      403 {object} dtos.ErrorResponse
// @Failure      404 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/admin/sources/{id} [patch]
func (h *SourceHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuidParam(r, "id")
	if err != nil {
		respondError(w, r, err)
		return
	}
	var req dtos.UpdateSourceRequest
	if err := decodeJSON(w, r, &req); err != nil {
		respondError(w, r, err)
		return
	}
	actor, _ := auth.FromContext(r.Context())
	view, err := h.service.Update(r.Context(), actor, id, services.SourceChange{
		Name: req.Name, Config: req.Config, Secret: req.Secret, IntervalMinutes: req.IntervalMinutes, Enabled: req.Enabled,
	})
	if err != nil {
		respondError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, dtos.NewSourceResponse(view.Source, view.Items))
}

// Delete godoc
// @Summary      Delete a source and its items
// @Description  Fails with SOURCE_HAS_CITED_ITEMS while reports or ideas cite its items; disable the source instead.
// @Tags         admin-sources
// @Param        id path string true "Source ID"
// @Success      204
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      403 {object} dtos.ErrorResponse
// @Failure      404 {object} dtos.ErrorResponse
// @Failure      409 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/admin/sources/{id} [delete]
func (h *SourceHandler) Delete(w http.ResponseWriter, r *http.Request) {
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

// Collect godoc
// @Summary      Collect a source now
// @Description  Queues a collection ahead of the schedule. Poll the job at /api/v1/jobs/{id}.
// @Tags         admin-sources
// @Produce      json
// @Param        id path string true "Source ID"
// @Success      202 {object} dtos.JobAcceptedEnvelope
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      403 {object} dtos.ErrorResponse
// @Failure      404 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/admin/sources/{id}/collect [post]
func (h *SourceHandler) Collect(w http.ResponseWriter, r *http.Request) {
	id, err := uuidParam(r, "id")
	if err != nil {
		respondError(w, r, err)
		return
	}
	actor, _ := auth.FromContext(r.Context())
	jobID, err := h.service.CollectNow(r.Context(), actor, id)
	if err != nil {
		respondError(w, r, err)
		return
	}
	respondJSON(w, http.StatusAccepted, dtos.JobAccepted{JobID: jobID.String()})
}

// ListPublic godoc
// @Summary      List sources
// @Description  Id, kind, name and enabled flag of every source, for feed filters.
// @Tags         sources
// @Produce      json
// @Success      200 {object} dtos.SourceSummaryListEnvelope
// @Failure      401 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/sources [get]
func (h *SourceHandler) ListPublic(w http.ResponseWriter, r *http.Request) {
	list, err := h.service.ListBasic(r.Context())
	if err != nil {
		respondError(w, r, err)
		return
	}
	out := make([]dtos.SourceSummary, len(list))
	for i, s := range list {
		out[i] = dtos.NewSourceSummary(s)
	}
	respondJSON(w, http.StatusOK, out)
}
