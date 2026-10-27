package handlers

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/dev-fritz/newsp/api/internal/auth"
	"github.com/dev-fritz/newsp/api/internal/dtos"
	"github.com/dev-fritz/newsp/api/internal/models"
	"github.com/dev-fritz/newsp/api/internal/services"
)

// AIHandler serves AI provider, model and instance settings administration.
type AIHandler struct {
	service *services.AIAdminService
}

// NewAIHandler builds an AIHandler.
func NewAIHandler(service *services.AIAdminService) *AIHandler {
	return &AIHandler{service: service}
}

// ListProviders godoc
// @Summary      List AI providers with their models
// @Tags         admin-ai
// @Produce      json
// @Success      200 {object} dtos.ProviderListEnvelope
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      403 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/admin/ai/providers [get]
func (h *AIHandler) ListProviders(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.FromContext(r.Context())
	views, err := h.service.ListProviders(r.Context(), actor)
	if err != nil {
		respondError(w, r, err)
		return
	}
	out := make([]dtos.ProviderResponse, len(views))
	for i, v := range views {
		out[i] = dtos.NewProviderResponse(v.Provider, v.KeyHint)
		for _, m := range v.Models {
			out[i].Models = append(out[i].Models, dtos.NewModelResponse(m.Model, m.TokensLast7d))
		}
	}
	respondJSON(w, http.StatusOK, out)
}

// CreateProvider godoc
// @Summary      Register an AI provider
// @Description  Use kind openai_compatible for Ollama, llama.cpp, LM Studio or vLLM (base_url required). The key is encrypted and never returned.
// @Tags         admin-ai
// @Accept       json
// @Produce      json
// @Param        request body dtos.CreateProviderRequest true "Provider"
// @Success      201 {object} dtos.ProviderEnvelope
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      403 {object} dtos.ErrorResponse
// @Failure      409 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/admin/ai/providers [post]
func (h *AIHandler) CreateProvider(w http.ResponseWriter, r *http.Request) {
	var req dtos.CreateProviderRequest
	if err := decodeJSON(w, r, &req); err != nil {
		respondError(w, r, err)
		return
	}
	actor, _ := auth.FromContext(r.Context())
	p, err := h.service.CreateProvider(r.Context(), actor, services.ProviderInput{Name: req.Name, Kind: req.Kind, BaseURL: req.BaseURL, APIKey: req.APIKey})
	if err != nil {
		respondError(w, r, err)
		return
	}
	respondJSON(w, http.StatusCreated, dtos.NewProviderResponse(p, ""))
}

// UpdateProvider godoc
// @Summary      Update an AI provider
// @Tags         admin-ai
// @Accept       json
// @Produce      json
// @Param        id      path string true "Provider ID"
// @Param        request body dtos.UpdateProviderRequest true "Changes"
// @Success      200 {object} dtos.ProviderEnvelope
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      403 {object} dtos.ErrorResponse
// @Failure      404 {object} dtos.ErrorResponse
// @Failure      409 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/admin/ai/providers/{id} [patch]
func (h *AIHandler) UpdateProvider(w http.ResponseWriter, r *http.Request) {
	id, err := uuidParam(r, "id")
	if err != nil {
		respondError(w, r, err)
		return
	}
	var req dtos.UpdateProviderRequest
	if err := decodeJSON(w, r, &req); err != nil {
		respondError(w, r, err)
		return
	}
	actor, _ := auth.FromContext(r.Context())
	p, err := h.service.UpdateProvider(r.Context(), actor, id, services.ProviderChange{Name: req.Name, BaseURL: req.BaseURL, APIKey: req.APIKey, Enabled: req.Enabled})
	if err != nil {
		respondError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, dtos.NewProviderResponse(p, ""))
}

// DeleteProvider godoc
// @Summary      Delete an AI provider and its models
// @Tags         admin-ai
// @Param        id path string true "Provider ID"
// @Success      204
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      403 {object} dtos.ErrorResponse
// @Failure      404 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/admin/ai/providers/{id} [delete]
func (h *AIHandler) DeleteProvider(w http.ResponseWriter, r *http.Request) {
	id, err := uuidParam(r, "id")
	if err != nil {
		respondError(w, r, err)
		return
	}
	actor, _ := auth.FromContext(r.Context())
	if err := h.service.DeleteProvider(r.Context(), actor, id); err != nil {
		respondError(w, r, err)
		return
	}
	respondNoContent(w)
}

// TestConnection godoc
// @Summary      Test a provider connection
// @Description  Lists the provider's models. A failed test is a 200 with ok=false and a readable error.
// @Tags         admin-ai
// @Produce      json
// @Param        id path string true "Provider ID"
// @Success      200 {object} dtos.ConnectionTestEnvelope
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      403 {object} dtos.ErrorResponse
// @Failure      404 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/admin/ai/providers/{id}/test [post]
func (h *AIHandler) TestConnection(w http.ResponseWriter, r *http.Request) {
	id, err := uuidParam(r, "id")
	if err != nil {
		respondError(w, r, err)
		return
	}
	actor, _ := auth.FromContext(r.Context())
	res, err := h.service.TestConnection(r.Context(), actor, id)
	if err != nil {
		respondError(w, r, err)
		return
	}
	if res.Models == nil {
		res.Models = []string{}
	}
	respondJSON(w, http.StatusOK, dtos.ConnectionTestResponse{OK: res.OK, Models: res.Models, Error: res.Error})
}

// CreateModel godoc
// @Summary      Register a model of a provider
// @Tags         admin-ai
// @Accept       json
// @Produce      json
// @Param        id      path string true "Provider ID"
// @Param        request body dtos.CreateModelRequest true "Model"
// @Success      201 {object} dtos.ModelEnvelope
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      403 {object} dtos.ErrorResponse
// @Failure      404 {object} dtos.ErrorResponse
// @Failure      409 {object} dtos.ErrorResponse
// @Failure      422 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/admin/ai/providers/{id}/models [post]
func (h *AIHandler) CreateModel(w http.ResponseWriter, r *http.Request) {
	id, err := uuidParam(r, "id")
	if err != nil {
		respondError(w, r, err)
		return
	}
	var req dtos.CreateModelRequest
	if err := decodeJSON(w, r, &req); err != nil {
		respondError(w, r, err)
		return
	}
	actor, _ := auth.FromContext(r.Context())
	m, err := h.service.CreateModel(r.Context(), actor, id, services.ModelInput{Name: req.Name, DisplayName: req.DisplayName, Capability: req.Capability})
	if err != nil {
		respondError(w, r, err)
		return
	}
	respondJSON(w, http.StatusCreated, dtos.NewModelResponse(m, 0))
}

// UpdateModel godoc
// @Summary      Rename, enable or disable a model
// @Tags         admin-ai
// @Accept       json
// @Produce      json
// @Param        id      path string true "Model ID"
// @Param        request body dtos.UpdateModelRequest true "Changes"
// @Success      200 {object} dtos.ModelEnvelope
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      403 {object} dtos.ErrorResponse
// @Failure      404 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/admin/ai/models/{id} [patch]
func (h *AIHandler) UpdateModel(w http.ResponseWriter, r *http.Request) {
	id, err := uuidParam(r, "id")
	if err != nil {
		respondError(w, r, err)
		return
	}
	var req dtos.UpdateModelRequest
	if err := decodeJSON(w, r, &req); err != nil {
		respondError(w, r, err)
		return
	}
	actor, _ := auth.FromContext(r.Context())
	m, err := h.service.UpdateModel(r.Context(), actor, id, services.ModelChange{DisplayName: req.DisplayName, Enabled: req.Enabled})
	if err != nil {
		respondError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, dtos.NewModelResponse(m, 0))
}

// DeleteModel godoc
// @Summary      Delete a model
// @Tags         admin-ai
// @Param        id path string true "Model ID"
// @Success      204
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      403 {object} dtos.ErrorResponse
// @Failure      404 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/admin/ai/models/{id} [delete]
func (h *AIHandler) DeleteModel(w http.ResponseWriter, r *http.Request) {
	id, err := uuidParam(r, "id")
	if err != nil {
		respondError(w, r, err)
		return
	}
	actor, _ := auth.FromContext(r.Context())
	if err := h.service.DeleteModel(r.Context(), actor, id); err != nil {
		respondError(w, r, err)
		return
	}
	respondNoContent(w)
}

// ChatModels godoc
// @Summary      Chat models available to pick
// @Description  Enabled chat models of enabled providers, for the profile screen.
// @Tags         ai
// @Produce      json
// @Success      200 {object} dtos.ChatModelListEnvelope
// @Failure      401 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/ai/models [get]
func (h *AIHandler) ChatModels(w http.ResponseWriter, r *http.Request) {
	options, err := h.service.ChatModels(r.Context())
	if err != nil {
		respondError(w, r, err)
		return
	}
	out := make([]dtos.ChatModelResponse, len(options))
	for i, o := range options {
		out[i] = dtos.ChatModelResponse{ID: o.Model.ID.String(), DisplayName: o.Model.DisplayName, Name: o.Model.Name, ProviderName: o.ProviderName, Local: o.Local}
	}
	respondJSON(w, http.StatusOK, out)
}

func settingsResponse(s models.InstanceSettings, reindex *uuid.UUID, mailReady bool) dtos.SettingsResponse {
	str := func(id *uuid.UUID) *string {
		if id == nil {
			return nil
		}
		v := id.String()
		return &v
	}
	return dtos.SettingsResponse{
		EmbeddingModelID: str(s.EmbeddingModelID), DefaultReportModelID: str(s.DefaultReportModelID),
		DefaultIdeaModelID: str(s.DefaultIdeaModelID), RetentionDays: s.RetentionDays,
		IdeaSimilarityThreshold: s.IdeaSimilarityThreshold, SignupEnabled: s.SignupEnabled,
		InactivityPauseDays: s.InactivityPauseDays, MailConfigured: mailReady, UpdatedAt: s.UpdatedAt, ReindexJobID: str(reindex),
	}
}

// GetSettings godoc
// @Summary      Instance settings
// @Tags         admin-settings
// @Produce      json
// @Success      200 {object} dtos.SettingsEnvelope
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      403 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/admin/settings [get]
func (h *AIHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.FromContext(r.Context())
	s, err := h.service.GetSettings(r.Context(), actor)
	if err != nil {
		respondError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, settingsResponse(s, nil, h.service.MailReady()))
}

// UpdateSettings godoc
// @Summary      Update instance settings
// @Description  Changing the embedding model starts a reindex job; its id is returned as reindex_job_id. Turning signup_enabled on without e-mail delivery returns 422 MAILER_NOT_CONFIGURED.
// @Tags         admin-settings
// @Accept       json
// @Produce      json
// @Param        request body dtos.UpdateSettingsRequest true "Changes"
// @Success      200 {object} dtos.SettingsEnvelope
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      403 {object} dtos.ErrorResponse
// @Failure      422 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/admin/settings [patch]
func (h *AIHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req dtos.UpdateSettingsRequest
	if err := decodeJSON(w, r, &req); err != nil {
		respondError(w, r, err)
		return
	}
	parse := func(raw *string) *uuid.UUID {
		if raw == nil {
			return nil
		}
		if *raw == "" {
			return &uuid.Nil
		}
		id := uuid.MustParse(*raw) // validated by the "uuid" tag
		return &id
	}
	actor, _ := auth.FromContext(r.Context())
	res, err := h.service.UpdateSettings(r.Context(), actor, services.SettingsChange{
		EmbeddingModelID: parse(req.EmbeddingModelID), DefaultReportModelID: parse(req.DefaultReportModelID),
		DefaultIdeaModelID: parse(req.DefaultIdeaModelID), RetentionDays: req.RetentionDays,
		IdeaSimilarityThreshold: req.IdeaSimilarityThreshold, SignupEnabled: req.SignupEnabled,
		InactivityPauseDays: req.InactivityPauseDays,
	})
	if err != nil {
		respondError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, settingsResponse(res.Settings, res.ReindexJobID, h.service.MailReady()))
}
