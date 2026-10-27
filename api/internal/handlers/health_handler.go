package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/dev-fritz/newsp/api/internal/apperrors"
	"github.com/dev-fritz/newsp/api/internal/dtos"
)

// Pinger checks a critical dependency (the database).
type Pinger interface {
	PingContext(ctx context.Context) error
}

// HealthHandler serves liveness and readiness probes.
type HealthHandler struct {
	db Pinger
}

// NewHealthHandler builds a HealthHandler.
func NewHealthHandler(db Pinger) *HealthHandler {
	return &HealthHandler{db: db}
}

// Live godoc
// @Summary      Liveness probe
// @Tags         health
// @Produce      json
// @Success      200 {object} dtos.HealthEnvelope
// @Router       /healthz [get]
func (h *HealthHandler) Live(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, dtos.HealthStatus{Status: "ok"})
}

// Ready godoc
// @Summary      Readiness probe
// @Description  Checks that the database answers.
// @Tags         health
// @Produce      json
// @Success      200 {object} dtos.HealthEnvelope
// @Failure      503 {object} dtos.ErrorResponse
// @Router       /readyz [get]
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.db.PingContext(ctx); err != nil {
		respondError(w, r, apperrors.Unavailable("DATABASE_UNAVAILABLE", "database unavailable").Wrap(err))
		return
	}
	respondJSON(w, http.StatusOK, dtos.HealthStatus{Status: "ok"})
}
