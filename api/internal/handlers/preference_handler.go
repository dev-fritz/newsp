package handlers

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/dev-fritz/newsp/api/internal/auth"
	"github.com/dev-fritz/newsp/api/internal/dtos"
	"github.com/dev-fritz/newsp/api/internal/services"
)

// PreferenceHandler serves the caller's preferences.
type PreferenceHandler struct {
	service *services.PreferenceService
}

// NewPreferenceHandler builds a PreferenceHandler.
func NewPreferenceHandler(service *services.PreferenceService) *PreferenceHandler {
	return &PreferenceHandler{service: service}
}

func preferencesResponse(v services.PreferenceView) dtos.PreferencesResponse {
	p := v.Preferences
	str := func(id *uuid.UUID) *string {
		if id == nil {
			return nil
		}
		s := id.String()
		return &s
	}
	excluded := make([]string, len(p.ExcludedSourceIDs))
	for i, id := range p.ExcludedSourceIDs {
		excluded[i] = id.String()
	}
	topics := p.Topics
	if topics == nil {
		topics = []string{}
	}
	return dtos.PreferencesResponse{
		Topics: topics, OutputLang: p.OutputLang, Timezone: p.Timezone, ReportEnabled: p.ReportEnabled,
		ReportTime: p.ReportTime, NextReportAt: p.NextReportAt, IdeasPerDay: p.IdeasPerDay,
		ReportModelID: str(p.ReportModelID), IdeaModelID: str(p.IdeaModelID), Theme: p.Theme,
		ExcludedSourceIDs: excluded, UILocale: v.UILocale, TopicsRanked: v.HasTopicsEmbedding,
	}
}

// Get godoc
// @Summary      My preferences
// @Description  Returns defaults until the user saves preferences.
// @Tags         me
// @Produce      json
// @Success      200 {object} dtos.PreferencesEnvelope
// @Failure      401 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/me/preferences [get]
func (h *PreferenceHandler) Get(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.FromContext(r.Context())
	view, err := h.service.Get(r.Context(), actor)
	if err != nil {
		respondError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, preferencesResponse(view))
}

// Update godoc
// @Summary      Replace my preferences
// @Description  Topics are deduplicated; the next daily report is scheduled in the given time zone.
// @Tags         me
// @Accept       json
// @Produce      json
// @Param        request body dtos.PreferencesRequest true "Preferences"
// @Success      200 {object} dtos.PreferencesEnvelope
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/me/preferences [put]
func (h *PreferenceHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req dtos.PreferencesRequest
	if err := decodeJSON(w, r, &req); err != nil {
		respondError(w, r, err)
		return
	}
	parse := func(raw *string) *uuid.UUID {
		if raw == nil {
			return nil
		}
		id := uuid.MustParse(*raw) // validated by the "uuid" tag
		return &id
	}
	excluded := make([]uuid.UUID, len(req.ExcludedSourceIDs))
	for i, raw := range req.ExcludedSourceIDs {
		excluded[i] = uuid.MustParse(raw)
	}
	actor, _ := auth.FromContext(r.Context())
	view, err := h.service.Update(r.Context(), actor, services.PreferenceInput{
		Topics: req.Topics, OutputLang: req.OutputLang, Timezone: req.Timezone, ReportEnabled: req.ReportEnabled,
		ReportTime: req.ReportTime, IdeasPerDay: req.IdeasPerDay, ReportModelID: parse(req.ReportModelID),
		IdeaModelID: parse(req.IdeaModelID), Theme: req.Theme, ExcludedSourceIDs: excluded, UILocale: req.UILocale,
	})
	if err != nil {
		respondError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, preferencesResponse(view))
}
