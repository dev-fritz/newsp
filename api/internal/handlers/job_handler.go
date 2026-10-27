package handlers

import (
	"net/http"

	"github.com/dev-fritz/newsp/api/internal/auth"
	"github.com/dev-fritz/newsp/api/internal/dtos"
	"github.com/dev-fritz/newsp/api/internal/services"
)

// JobHandler serves job status.
type JobHandler struct {
	service *services.JobService
}

// NewJobHandler builds a JobHandler.
func NewJobHandler(service *services.JobService) *JobHandler {
	return &JobHandler{service: service}
}

// Get godoc
// @Summary      Job status
// @Description  Poll until status is done, failed or canceled. Users see their own jobs; admins also see system jobs.
// @Tags         jobs
// @Produce      json
// @Param        id path string true "Job ID"
// @Success      200 {object} dtos.JobEnvelope
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      404 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/jobs/{id} [get]
func (h *JobHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuidParam(r, "id")
	if err != nil {
		respondError(w, r, err)
		return
	}
	actor, _ := auth.FromContext(r.Context())
	view, err := h.service.Get(r.Context(), actor, id)
	if err != nil {
		respondError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, dtos.NewJobResponse(view.Job, view.ShowError))
}
