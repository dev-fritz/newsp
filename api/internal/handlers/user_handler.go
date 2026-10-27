package handlers

import (
	"net/http"

	"github.com/dev-fritz/newsp/api/internal/auth"
	"github.com/dev-fritz/newsp/api/internal/dtos"
	"github.com/dev-fritz/newsp/api/internal/services"
)

// UserHandler serves user administration.
type UserHandler struct {
	service *services.UserService
}

// NewUserHandler builds a UserHandler.
func NewUserHandler(service *services.UserService) *UserHandler {
	return &UserHandler{service: service}
}

func linkResponse(l services.Link) dtos.LinkResponse {
	return dtos.LinkResponse{ID: l.ID.String(), URL: l.URL, ExpiresAt: l.ExpiresAt}
}

// List godoc
// @Summary      List users
// @Tags         admin
// @Produce      json
// @Param        limit  query int false "Page size (max 100)"
// @Param        offset query int false "Offset"
// @Success      200 {object} dtos.UserListEnvelope
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      403 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/admin/users [get]
func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := parsePage(r)
	if err != nil {
		respondError(w, r, err)
		return
	}
	actor, _ := auth.FromContext(r.Context())
	users, total, err := h.service.List(r.Context(), actor, limit, offset)
	if err != nil {
		respondError(w, r, err)
		return
	}
	out := make([]dtos.UserResponse, len(users))
	for i, u := range users {
		out[i] = dtos.NewUserResponse(u)
	}
	respondPage(w, out, dtos.PageMeta{Total: total, Limit: limit, Offset: offset})
}

// UpdateAccess godoc
// @Summary      Change a user's role or status
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        id      path string true "User ID"
// @Param        request body dtos.UpdateUserAccessRequest true "Changes"
// @Success      200 {object} dtos.UserEnvelope
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      403 {object} dtos.ErrorResponse
// @Failure      404 {object} dtos.ErrorResponse
// @Failure      422 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/admin/users/{id} [patch]
func (h *UserHandler) UpdateAccess(w http.ResponseWriter, r *http.Request) {
	id, err := uuidParam(r, "id")
	if err != nil {
		respondError(w, r, err)
		return
	}
	var req dtos.UpdateUserAccessRequest
	if err := decodeJSON(w, r, &req); err != nil {
		respondError(w, r, err)
		return
	}
	actor, _ := auth.FromContext(r.Context())
	user, err := h.service.UpdateAccess(r.Context(), actor, id, services.UserAccessChange{Role: req.Role, Status: req.Status})
	if err != nil {
		respondError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, dtos.NewUserResponse(user))
}

// CreatePasswordReset godoc
// @Summary      Create a password reset link for a user
// @Description  The link is shown only once and replaces older reset links.
// @Tags         admin
// @Produce      json
// @Param        id path string true "User ID"
// @Success      201 {object} dtos.LinkEnvelope
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      403 {object} dtos.ErrorResponse
// @Failure      404 {object} dtos.ErrorResponse
// @Failure      422 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/admin/users/{id}/password-reset [post]
func (h *UserHandler) CreatePasswordReset(w http.ResponseWriter, r *http.Request) {
	id, err := uuidParam(r, "id")
	if err != nil {
		respondError(w, r, err)
		return
	}
	actor, _ := auth.FromContext(r.Context())
	link, err := h.service.CreatePasswordReset(r.Context(), actor, id)
	if err != nil {
		respondError(w, r, err)
		return
	}
	respondJSON(w, http.StatusCreated, linkResponse(link))
}

// CreateInvite godoc
// @Summary      Create an invite link
// @Description  The link is shown only once. With an e-mail, the invite is bound to that address and the link is also e-mailed (emailed = true); an address that already has an account returns 409 EMAIL_TAKEN, and an instance without e-mail delivery returns 422 MAILER_NOT_CONFIGURED.
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        request body dtos.CreateInviteRequest true "Invite"
// @Success      201 {object} dtos.InviteLinkEnvelope
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      403 {object} dtos.ErrorResponse
// @Failure      409 {object} dtos.ErrorResponse
// @Failure      422 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/admin/invites [post]
func (h *UserHandler) CreateInvite(w http.ResponseWriter, r *http.Request) {
	var req dtos.CreateInviteRequest
	if err := decodeJSON(w, r, &req); err != nil {
		respondError(w, r, err)
		return
	}
	actor, _ := auth.FromContext(r.Context())
	link, err := h.service.CreateInvite(r.Context(), actor, req.Role, req.Email)
	if err != nil {
		respondError(w, r, err)
		return
	}
	respondJSON(w, http.StatusCreated, dtos.InviteLinkResponse{
		LinkResponse: linkResponse(link), Email: link.Email, Emailed: link.Email != nil,
	})
}

// ListInvites godoc
// @Summary      List pending invites
// @Tags         admin
// @Produce      json
// @Param        limit  query int false "Page size (max 100)"
// @Param        offset query int false "Offset"
// @Success      200 {object} dtos.InviteListEnvelope
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      403 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/admin/invites [get]
func (h *UserHandler) ListInvites(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := parsePage(r)
	if err != nil {
		respondError(w, r, err)
		return
	}
	actor, _ := auth.FromContext(r.Context())
	invites, total, err := h.service.ListInvites(r.Context(), actor, limit, offset)
	if err != nil {
		respondError(w, r, err)
		return
	}
	out := make([]dtos.InviteResponse, len(invites))
	for i, inv := range invites {
		out[i] = dtos.NewInviteResponse(inv)
	}
	respondPage(w, out, dtos.PageMeta{Total: total, Limit: limit, Offset: offset})
}

// RevokeInvite godoc
// @Summary      Revoke a pending invite
// @Tags         admin
// @Param        id path string true "Invite ID"
// @Success      204
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      403 {object} dtos.ErrorResponse
// @Failure      404 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/admin/invites/{id} [delete]
func (h *UserHandler) RevokeInvite(w http.ResponseWriter, r *http.Request) {
	id, err := uuidParam(r, "id")
	if err != nil {
		respondError(w, r, err)
		return
	}
	actor, _ := auth.FromContext(r.Context())
	if err := h.service.RevokeInvite(r.Context(), actor, id); err != nil {
		respondError(w, r, err)
		return
	}
	respondNoContent(w)
}
