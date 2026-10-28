package handlers

import (
	"net/http"

	"github.com/dev-fritz/newsp/api/internal/auth"
	"github.com/dev-fritz/newsp/api/internal/dtos"
	"github.com/dev-fritz/newsp/api/internal/services"
)

// UserAIHandler serves the caller's own AI providers (BYOK) and the chat
// models the caller may pick.
type UserAIHandler struct {
	service *services.UserAIService
}

// NewUserAIHandler builds a UserAIHandler.
func NewUserAIHandler(service *services.UserAIService) *UserAIHandler {
	return &UserAIHandler{service: service}
}

// ChatModels godoc
// @Summary      Chat models available to pick
// @Description  Enabled chat models of the instance's enabled providers, plus the caller's own models while using own keys is allowed (own=true), for the profile screen.
// @Tags         ai
// @Produce      json
// @Success      200 {object} dtos.ChatModelListEnvelope
// @Failure      401 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/ai/models [get]
func (h *UserAIHandler) ChatModels(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.FromContext(r.Context())
	options, err := h.service.ChatModels(r.Context(), actor)
	if err != nil {
		respondError(w, r, err)
		return
	}
	out := make([]dtos.ChatModelResponse, len(options))
	for i, o := range options {
		out[i] = dtos.ChatModelResponse{
			ID: o.Model.ID.String(), DisplayName: o.Model.DisplayName, Name: o.Model.Name,
			ProviderName: o.ProviderName, Local: o.Local, Own: o.Own,
		}
	}
	respondJSON(w, http.StatusOK, out)
}

// ListProviders godoc
// @Summary      List my own AI providers with their models
// @Description  Works while own keys are turned off (byok_enabled=false), so the profile can explain why the section is hidden.
// @Tags         me-ai
// @Produce      json
// @Success      200 {object} dtos.OwnProvidersEnvelope
// @Failure      401 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/me/ai/providers [get]
func (h *UserAIHandler) ListProviders(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.FromContext(r.Context())
	res, err := h.service.List(r.Context(), actor)
	if err != nil {
		respondError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, dtos.OwnProvidersResponse{BYOKEnabled: res.Enabled, Providers: providerResponses(res.Providers)})
}

// CreateProvider godoc
// @Summary      Connect my own AI provider
// @Description  Only openai, anthropic and gemini, at their official URLs, one per kind. The key is encrypted and never returned. Returns 409 PROVIDER_KIND_TAKEN when the kind is already connected and 422 BYOK_DISABLED when an admin turned own keys off.
// @Tags         me-ai
// @Accept       json
// @Produce      json
// @Param        request body dtos.CreateOwnProviderRequest true "Provider"
// @Success      201 {object} dtos.ProviderEnvelope
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      409 {object} dtos.ErrorResponse
// @Failure      422 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/me/ai/providers [post]
func (h *UserAIHandler) CreateProvider(w http.ResponseWriter, r *http.Request) {
	var req dtos.CreateOwnProviderRequest
	if err := decodeJSON(w, r, &req); err != nil {
		respondError(w, r, err)
		return
	}
	actor, _ := auth.FromContext(r.Context())
	p, err := h.service.Create(r.Context(), actor, req.Kind, req.APIKey)
	if err != nil {
		respondError(w, r, err)
		return
	}
	respondJSON(w, http.StatusCreated, dtos.NewProviderResponse(p, ""))
}

// UpdateProvider godoc
// @Summary      Replace the key of, enable or disable my own AI provider
// @Description  The key can be replaced but not removed: delete the provider instead. Another user's or an instance provider answers 404.
// @Tags         me-ai
// @Accept       json
// @Produce      json
// @Param        id      path string true "Provider ID"
// @Param        request body dtos.UpdateOwnProviderRequest true "Changes"
// @Success      200 {object} dtos.ProviderEnvelope
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      404 {object} dtos.ErrorResponse
// @Failure      422 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/me/ai/providers/{id} [patch]
func (h *UserAIHandler) UpdateProvider(w http.ResponseWriter, r *http.Request) {
	id, err := uuidParam(r, "id")
	if err != nil {
		respondError(w, r, err)
		return
	}
	var req dtos.UpdateOwnProviderRequest
	if err := decodeJSON(w, r, &req); err != nil {
		respondError(w, r, err)
		return
	}
	actor, _ := auth.FromContext(r.Context())
	p, err := h.service.Update(r.Context(), actor, id, services.UserProviderChange{APIKey: req.APIKey, Enabled: req.Enabled})
	if err != nil {
		respondError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, dtos.NewProviderResponse(p, ""))
}

// DeleteProvider godoc
// @Summary      Delete my own AI provider, its key and its models
// @Description  Works while own keys are turned off, so users can always take their key back out.
// @Tags         me-ai
// @Param        id path string true "Provider ID"
// @Success      204
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      404 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/me/ai/providers/{id} [delete]
func (h *UserAIHandler) DeleteProvider(w http.ResponseWriter, r *http.Request) {
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

// TestConnection godoc
// @Summary      Test my own provider connection
// @Description  Lists the provider's models. A failed test is a 200 with ok=false and a readable error.
// @Tags         me-ai
// @Produce      json
// @Param        id path string true "Provider ID"
// @Success      200 {object} dtos.ConnectionTestEnvelope
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      404 {object} dtos.ErrorResponse
// @Failure      422 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/me/ai/providers/{id}/test [post]
func (h *UserAIHandler) TestConnection(w http.ResponseWriter, r *http.Request) {
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
// @Summary      Register a chat model of my own provider
// @Description  Users' models are always chat models.
// @Tags         me-ai
// @Accept       json
// @Produce      json
// @Param        id      path string true "Provider ID"
// @Param        request body dtos.CreateOwnModelRequest true "Model"
// @Success      201 {object} dtos.ModelEnvelope
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      404 {object} dtos.ErrorResponse
// @Failure      409 {object} dtos.ErrorResponse
// @Failure      422 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/me/ai/providers/{id}/models [post]
func (h *UserAIHandler) CreateModel(w http.ResponseWriter, r *http.Request) {
	id, err := uuidParam(r, "id")
	if err != nil {
		respondError(w, r, err)
		return
	}
	var req dtos.CreateOwnModelRequest
	if err := decodeJSON(w, r, &req); err != nil {
		respondError(w, r, err)
		return
	}
	actor, _ := auth.FromContext(r.Context())
	m, err := h.service.CreateModel(r.Context(), actor, id, req.Name, req.DisplayName)
	if err != nil {
		respondError(w, r, err)
		return
	}
	respondJSON(w, http.StatusCreated, dtos.NewModelResponse(m, 0))
}

// UpdateModel godoc
// @Summary      Rename, enable or disable a model of my own provider
// @Tags         me-ai
// @Accept       json
// @Produce      json
// @Param        id      path string true "Model ID"
// @Param        request body dtos.UpdateModelRequest true "Changes"
// @Success      200 {object} dtos.ModelEnvelope
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      404 {object} dtos.ErrorResponse
// @Failure      422 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/me/ai/models/{id} [patch]
func (h *UserAIHandler) UpdateModel(w http.ResponseWriter, r *http.Request) {
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
// @Summary      Delete a model of my own provider
// @Description  Preferences pointing at it fall back to the instance defaults.
// @Tags         me-ai
// @Param        id path string true "Model ID"
// @Success      204
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      404 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/me/ai/models/{id} [delete]
func (h *UserAIHandler) DeleteModel(w http.ResponseWriter, r *http.Request) {
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
