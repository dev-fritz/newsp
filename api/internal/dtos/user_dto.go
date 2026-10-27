package dtos

import (
	"time"

	"github.com/dev-fritz/newsp/api/internal/models"
)

// UpdateUserAccessRequest changes a user's role and/or status (admin only).
type UpdateUserAccessRequest struct {
	Role   *models.Role       `json:"role" validate:"omitempty,oneof=admin member" enums:"admin,member"`
	Status *models.UserStatus `json:"status" validate:"omitempty,oneof=active disabled" enums:"active,disabled"`
}

// CreateInviteRequest creates an invite link. With an e-mail, the invite is
// bound to that address and the link is also sent there.
type CreateInviteRequest struct {
	Role  models.Role `json:"role" validate:"required,oneof=admin member" enums:"admin,member"`
	Email string      `json:"email" validate:"omitempty,email,max=254" example:"ana@example.com"`
}

// LinkResponse is a one-time link, shown only once.
type LinkResponse struct {
	ID        string    `json:"id"`
	URL       string    `json:"url" example:"https://newsp.example.com/invite/k3Jx9..."`
	ExpiresAt time.Time `json:"expires_at"`
}

// InviteLinkResponse is a new invite link, shown only once. Emailed tells
// whether the link was also queued for delivery to Email.
type InviteLinkResponse struct {
	LinkResponse
	Email   *string `json:"email,omitempty"`
	Emailed bool    `json:"emailed"`
}

// InviteResponse is a pending invite (without its secret link).
type InviteResponse struct {
	ID        string      `json:"id"`
	Role      models.Role `json:"role" enums:"admin,member"`
	Email     *string     `json:"email,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
	ExpiresAt time.Time   `json:"expires_at"`
}

// NewInviteResponse maps a pending invite.
func NewInviteResponse(t models.UserToken) InviteResponse {
	resp := InviteResponse{ID: t.ID.String(), Email: t.Email, CreatedAt: t.CreatedAt, ExpiresAt: t.ExpiresAt}
	if t.Role != nil {
		resp.Role = *t.Role
	}
	return resp
}

// UserListEnvelope is a page of users.
type UserListEnvelope struct {
	Data []UserResponse `json:"data"`
	Meta PageMeta       `json:"meta"`
}

// InviteListEnvelope is a page of pending invites.
type InviteListEnvelope struct {
	Data []InviteResponse `json:"data"`
	Meta PageMeta         `json:"meta"`
}

// LinkEnvelope wraps LinkResponse.
type LinkEnvelope struct {
	Data LinkResponse `json:"data"`
}

// InviteLinkEnvelope wraps InviteLinkResponse.
type InviteLinkEnvelope struct {
	Data InviteLinkResponse `json:"data"`
}
