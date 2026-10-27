package handlers

import (
	"net/http"

	"github.com/dev-fritz/newsp/api/internal/auth"
	"github.com/dev-fritz/newsp/api/internal/dtos"
	"github.com/dev-fritz/newsp/api/internal/services"
)

// AuthHandler serves setup, sign-in, sessions and one-time links.
type AuthHandler struct {
	service *services.AuthService
}

// NewAuthHandler builds an AuthHandler.
func NewAuthHandler(service *services.AuthService) *AuthHandler {
	return &AuthHandler{service: service}
}

func sessionResponse(s services.Session) dtos.SessionResponse {
	return dtos.SessionResponse{
		AccessToken: s.AccessToken, AccessExpiresAt: s.AccessExpiresAt,
		RefreshToken: s.RefreshToken, RefreshExpiresAt: s.RefreshExpiresAt,
		User: dtos.NewUserResponse(s.User),
	}
}

// SetupStatus godoc
// @Summary      First-run setup status
// @Tags         auth
// @Produce      json
// @Success      200 {object} dtos.SetupStatusEnvelope
// @Router       /api/v1/setup/status [get]
func (h *AuthHandler) SetupStatus(w http.ResponseWriter, r *http.Request) {
	status, err := h.service.Status(r.Context())
	if err != nil {
		respondError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, dtos.SetupStatusResponse{
		SetupRequired: status.SetupRequired, SignupEnabled: status.SignupEnabled, PasswordResetEnabled: status.PasswordResetEnabled,
	})
}

// Signup godoc
// @Summary      Create an account without an invite
// @Description  Works only when the admin enabled public signup and e-mail delivery is configured. The account is a member and its e-mail starts unverified: a verification link is e-mailed, and reports and ideas start after it is used.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body dtos.SignupRequest true "Account"
// @Success      201 {object} dtos.SessionEnvelope
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      403 {object} dtos.ErrorResponse
// @Failure      409 {object} dtos.ErrorResponse
// @Failure      429 {object} dtos.ErrorResponse
// @Router       /api/v1/auth/signup [post]
func (h *AuthHandler) Signup(w http.ResponseWriter, r *http.Request) {
	var req dtos.SignupRequest
	if err := decodeJSON(w, r, &req); err != nil {
		respondError(w, r, err)
		return
	}
	session, err := h.service.Signup(r.Context(), services.NewAccount{Name: req.Name, Email: req.Email, Password: req.Password}, req.Locale)
	if err != nil {
		respondError(w, r, err)
		return
	}
	respondJSON(w, http.StatusCreated, sessionResponse(session))
}

// VerifyEmail godoc
// @Summary      Confirm an e-mail address through its link
// @Tags         auth
// @Accept       json
// @Param        request body dtos.TokenRequest true "Verification token"
// @Success      204
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      404 {object} dtos.ErrorResponse
// @Failure      429 {object} dtos.ErrorResponse
// @Router       /api/v1/auth/verify-email [post]
func (h *AuthHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req dtos.TokenRequest
	if err := decodeJSON(w, r, &req); err != nil {
		respondError(w, r, err)
		return
	}
	if err := h.service.VerifyEmail(r.Context(), req.Token); err != nil {
		respondError(w, r, err)
		return
	}
	respondNoContent(w)
}

// ResendVerification godoc
// @Summary      E-mail a new verification link to the current user
// @Tags         me
// @Success      204
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      409 {object} dtos.ErrorResponse
// @Failure      429 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/me/verify-email/resend [post]
func (h *AuthHandler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.FromContext(r.Context())
	if err := h.service.ResendVerification(r.Context(), id); err != nil {
		respondError(w, r, err)
		return
	}
	respondNoContent(w)
}

// Setup godoc
// @Summary      Create the first admin
// @Description  Works once per instance; afterwards it returns 409.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body dtos.AccountRequest true "First admin"
// @Success      201 {object} dtos.SessionEnvelope
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      409 {object} dtos.ErrorResponse
// @Failure      429 {object} dtos.ErrorResponse
// @Router       /api/v1/setup [post]
func (h *AuthHandler) Setup(w http.ResponseWriter, r *http.Request) {
	var req dtos.AccountRequest
	if err := decodeJSON(w, r, &req); err != nil {
		respondError(w, r, err)
		return
	}
	session, err := h.service.Setup(r.Context(), services.NewAccount{Name: req.Name, Email: req.Email, Password: req.Password})
	if err != nil {
		respondError(w, r, err)
		return
	}
	respondJSON(w, http.StatusCreated, sessionResponse(session))
}

// Login godoc
// @Summary      Sign in
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body dtos.LoginRequest true "Credentials"
// @Success      200 {object} dtos.SessionEnvelope
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      429 {object} dtos.ErrorResponse
// @Router       /api/v1/auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req dtos.LoginRequest
	if err := decodeJSON(w, r, &req); err != nil {
		respondError(w, r, err)
		return
	}
	session, err := h.service.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		respondError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, sessionResponse(session))
}

// Refresh godoc
// @Summary      Rotate a refresh token
// @Description  Returns a new access token and a new refresh token. Reusing an old refresh token ends the session.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body dtos.RefreshRequest true "Refresh token"
// @Success      200 {object} dtos.SessionEnvelope
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Failure      429 {object} dtos.ErrorResponse
// @Router       /api/v1/auth/refresh [post]
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req dtos.RefreshRequest
	if err := decodeJSON(w, r, &req); err != nil {
		respondError(w, r, err)
		return
	}
	session, err := h.service.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		respondError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, sessionResponse(session))
}

// Logout godoc
// @Summary      Sign out
// @Tags         auth
// @Accept       json
// @Param        request body dtos.RefreshRequest true "Refresh token"
// @Success      204
// @Failure      400 {object} dtos.ErrorResponse
// @Router       /api/v1/auth/logout [post]
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req dtos.RefreshRequest
	if err := decodeJSON(w, r, &req); err != nil {
		respondError(w, r, err)
		return
	}
	if err := h.service.Logout(r.Context(), req.RefreshToken); err != nil {
		respondError(w, r, err)
		return
	}
	respondNoContent(w)
}

// InspectInvite godoc
// @Summary      Check an invite link
// @Description  email is set when the invite is bound to an address; accepting it then requires that same e-mail.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body dtos.TokenRequest true "Invite token"
// @Success      200 {object} dtos.InviteInfoEnvelope
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      404 {object} dtos.ErrorResponse
// @Failure      429 {object} dtos.ErrorResponse
// @Router       /api/v1/invites/inspect [post]
func (h *AuthHandler) InspectInvite(w http.ResponseWriter, r *http.Request) {
	var req dtos.TokenRequest
	if err := decodeJSON(w, r, &req); err != nil {
		respondError(w, r, err)
		return
	}
	info, err := h.service.InspectInvite(r.Context(), req.Token)
	if err != nil {
		respondError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, dtos.InviteInfoResponse{Role: info.Role, Email: info.Email, ExpiresAt: info.ExpiresAt})
}

// AcceptInvite godoc
// @Summary      Accept an invite and create the account
// @Description  When the invite is bound to an e-mail, a different address returns 400 with fields.email = invite_mismatch.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body dtos.AcceptInviteRequest true "Invite and account"
// @Success      201 {object} dtos.SessionEnvelope
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      404 {object} dtos.ErrorResponse
// @Failure      409 {object} dtos.ErrorResponse
// @Failure      429 {object} dtos.ErrorResponse
// @Router       /api/v1/invites/accept [post]
func (h *AuthHandler) AcceptInvite(w http.ResponseWriter, r *http.Request) {
	var req dtos.AcceptInviteRequest
	if err := decodeJSON(w, r, &req); err != nil {
		respondError(w, r, err)
		return
	}
	session, err := h.service.AcceptInvite(r.Context(), req.Token,
		services.NewAccount{Name: req.Name, Email: req.Email, Password: req.Password})
	if err != nil {
		respondError(w, r, err)
		return
	}
	respondJSON(w, http.StatusCreated, sessionResponse(session))
}

// InspectPasswordReset godoc
// @Summary      Check a password reset link
// @Tags         auth
// @Accept       json
// @Param        request body dtos.TokenRequest true "Reset token"
// @Success      204
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      404 {object} dtos.ErrorResponse
// @Failure      429 {object} dtos.ErrorResponse
// @Router       /api/v1/password-resets/inspect [post]
func (h *AuthHandler) InspectPasswordReset(w http.ResponseWriter, r *http.Request) {
	var req dtos.TokenRequest
	if err := decodeJSON(w, r, &req); err != nil {
		respondError(w, r, err)
		return
	}
	if err := h.service.InspectPasswordReset(r.Context(), req.Token); err != nil {
		respondError(w, r, err)
		return
	}
	respondNoContent(w)
}

// RequestPasswordReset godoc
// @Summary      Ask for a password reset link by e-mail
// @Description  Always answers 202 with no body, whether or not the e-mail belongs to an account; the link is sent only to active accounts, and the answer and its timing do not depend on that. Limited per e-mail and per client IP.
// @Tags         auth
// @Accept       json
// @Param        request body dtos.PasswordResetRequest true "E-mail"
// @Success      202
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      429 {object} dtos.ErrorResponse
// @Router       /api/v1/password-resets/request [post]
func (h *AuthHandler) RequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	var req dtos.PasswordResetRequest
	if err := decodeJSON(w, r, &req); err != nil {
		respondError(w, r, err)
		return
	}
	if err := h.service.RequestPasswordReset(r.Context(), req.Email); err != nil {
		respondError(w, r, err)
		return
	}
	respondAccepted(w)
}

// ResetPassword godoc
// @Summary      Set a new password through a reset link
// @Description  Ends every session of the user.
// @Tags         auth
// @Accept       json
// @Param        request body dtos.ResetPasswordRequest true "Reset"
// @Success      204
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      404 {object} dtos.ErrorResponse
// @Failure      429 {object} dtos.ErrorResponse
// @Router       /api/v1/password-resets/confirm [post]
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req dtos.ResetPasswordRequest
	if err := decodeJSON(w, r, &req); err != nil {
		respondError(w, r, err)
		return
	}
	if err := h.service.ResetPassword(r.Context(), req.Token, req.Password); err != nil {
		respondError(w, r, err)
		return
	}
	respondNoContent(w)
}

// Me godoc
// @Summary      Current user
// @Tags         me
// @Produce      json
// @Success      200 {object} dtos.UserEnvelope
// @Failure      401 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/me [get]
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.FromContext(r.Context())
	user, err := h.service.Me(r.Context(), id)
	if err != nil {
		respondError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, dtos.NewUserResponse(user))
}

// ChangePassword godoc
// @Summary      Change the current user's password
// @Description  Ends every other session and returns a new session for this device.
// @Tags         me
// @Accept       json
// @Produce      json
// @Param        request body dtos.ChangePasswordRequest true "Passwords"
// @Success      200 {object} dtos.SessionEnvelope
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/me/password [put]
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var req dtos.ChangePasswordRequest
	if err := decodeJSON(w, r, &req); err != nil {
		respondError(w, r, err)
		return
	}
	id, _ := auth.FromContext(r.Context())
	session, err := h.service.ChangePassword(r.Context(), id, req.CurrentPassword, req.NewPassword)
	if err != nil {
		respondError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, sessionResponse(session))
}
