// Package models holds the domain entities persisted by the repositories.
package models

import (
	"time"

	"github.com/google/uuid"
)

// Role is a user's permission level in the instance.
type Role string

// User roles.
const (
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)

// UserStatus tells whether a user can sign in.
type UserStatus string

// User statuses.
const (
	UserActive   UserStatus = "active"
	UserDisabled UserStatus = "disabled"
)

// User is a person with access to the instance.
type User struct {
	ID           uuid.UUID
	Email        string
	Name         string
	PasswordHash string
	Role         Role
	Status       UserStatus
	Locale       string
	LastSeenAt   *time.Time
	// EmailVerifiedAt is nil until the owner proves control of the address.
	EmailVerifiedAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// UserTokenKind distinguishes one-time links.
type UserTokenKind string

// One-time link kinds.
const (
	TokenInvite            UserTokenKind = "invite"
	TokenPasswordReset     UserTokenKind = "password_reset"
	TokenEmailVerification UserTokenKind = "email_verification" //nolint:gosec // token kind label, not a credential
)

// UserToken is a one-time link (invite or password reset), stored hashed.
type UserToken struct {
	ID        uuid.UUID
	Kind      UserTokenKind
	TokenHash []byte
	UserID    *uuid.UUID // password reset target
	Role      *Role      // invite role
	Email     *string    // invite destination e-mail
	CreatedBy *uuid.UUID
	ExpiresAt time.Time
	UsedAt    *time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}

// RefreshToken is an opaque, rotated session token, stored hashed.
type RefreshToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	FamilyID  uuid.UUID
	TokenHash []byte
	ExpiresAt time.Time
	RotatedAt *time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}
