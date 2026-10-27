package dtos

import (
	"time"

	"github.com/dev-fritz/newsp/api/internal/auth"
	"github.com/dev-fritz/newsp/api/internal/models"
)

// AccountRequest creates an account (first-run setup).
type AccountRequest struct {
	Name     string `json:"name" validate:"required,max=100"`
	Email    string `json:"email" validate:"required,email,max=254"`
	Password string `json:"password" validate:"required,max=72"`
}

// SignupRequest creates an account through public signup.
type SignupRequest struct {
	Name     string `json:"name" validate:"required,max=100"`
	Email    string `json:"email" validate:"required,email,max=254"`
	Password string `json:"password" validate:"required,max=72"`
	// Locale is the language of the account and its e-mails; pt-BR by default.
	Locale string `json:"locale" validate:"omitempty,oneof=pt-BR en" example:"pt-BR"`
}

// LoginRequest signs in with e-mail and password.
type LoginRequest struct {
	Email    string `json:"email" validate:"required,max=254"`
	Password string `json:"password" validate:"required,max=72"`
}

// RefreshRequest carries a refresh token.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required,max=128"`
}

// TokenRequest carries a one-time link token.
type TokenRequest struct {
	Token string `json:"token" validate:"required,max=128"`
}

// AcceptInviteRequest creates the invited account.
type AcceptInviteRequest struct {
	Token    string `json:"token" validate:"required,max=128"`
	Name     string `json:"name" validate:"required,max=100"`
	Email    string `json:"email" validate:"required,email,max=254"`
	Password string `json:"password" validate:"required,max=72"`
}

// ResetPasswordRequest sets a new password through a reset link.
type ResetPasswordRequest struct {
	Token    string `json:"token" validate:"required,max=128"`
	Password string `json:"password" validate:"required,max=72"`
}

// PasswordResetRequest asks for a reset link by e-mail.
type PasswordResetRequest struct {
	Email string `json:"email" validate:"required,email,max=254"`
}

// ChangePasswordRequest changes the caller's password.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required,max=72"`
	NewPassword     string `json:"new_password" validate:"required,max=72"`
}

// UserResponse is a user as seen by clients.
type UserResponse struct {
	ID            string            `json:"id" example:"6f1c1d0e-5a4b-4f1e-9b1c-2a3d4e5f6a7b"`
	Email         string            `json:"email" example:"ana@example.com"`
	Name          string            `json:"name" example:"Ana Costa"`
	Role          models.Role       `json:"role" example:"member" enums:"admin,member"`
	Status        models.UserStatus `json:"status" example:"active" enums:"active,disabled"`
	Locale        string            `json:"locale" example:"pt-BR"`
	EmailVerified bool              `json:"email_verified"`
	Permissions   []auth.Permission `json:"permissions"`
	LastSeenAt    *time.Time        `json:"last_seen_at"`
	CreatedAt     time.Time         `json:"created_at"`
}

// NewUserResponse maps a user.
func NewUserResponse(u models.User) UserResponse {
	return UserResponse{
		ID: u.ID.String(), Email: u.Email, Name: u.Name, Role: u.Role, Status: u.Status,
		Locale: u.Locale, EmailVerified: u.EmailVerifiedAt != nil, Permissions: auth.Permissions(u.Role), LastSeenAt: u.LastSeenAt, CreatedAt: u.CreatedAt,
	}
}

// SessionResponse is returned by every flow that signs a user in.
type SessionResponse struct {
	AccessToken      string       `json:"access_token"`
	AccessExpiresAt  time.Time    `json:"access_expires_at"`
	RefreshToken     string       `json:"refresh_token"`
	RefreshExpiresAt time.Time    `json:"refresh_expires_at"`
	User             UserResponse `json:"user"`
}

// SetupStatusResponse tells whether the first-run setup is pending and
// whether visitors may create an account.
type SetupStatusResponse struct {
	SetupRequired bool `json:"setup_required"`
	SignupEnabled bool `json:"signup_enabled"`
	// PasswordResetEnabled tells whether "forgot password" can e-mail a link.
	PasswordResetEnabled bool `json:"password_reset_enabled"`
}

// InviteInfoResponse describes a usable invite link.
type InviteInfoResponse struct {
	Role      models.Role `json:"role" enums:"admin,member"`
	Email     *string     `json:"email"`
	ExpiresAt time.Time   `json:"expires_at"`
}

// SessionEnvelope wraps SessionResponse.
type SessionEnvelope struct {
	Data SessionResponse `json:"data"`
}

// UserEnvelope wraps UserResponse.
type UserEnvelope struct {
	Data UserResponse `json:"data"`
}

// SetupStatusEnvelope wraps SetupStatusResponse.
type SetupStatusEnvelope struct {
	Data SetupStatusResponse `json:"data"`
}

// InviteInfoEnvelope wraps InviteInfoResponse.
type InviteInfoEnvelope struct {
	Data InviteInfoResponse `json:"data"`
}
