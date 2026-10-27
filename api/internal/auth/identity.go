// Package auth holds identity, permissions and the token and password
// primitives used by the auth services and middlewares.
package auth

import (
	"context"

	"github.com/google/uuid"

	"github.com/dev-fritz/newsp/api/internal/models"
)

// Identity is the authenticated caller, set on the context by the middleware.
type Identity struct {
	UserID uuid.UUID
	Role   models.Role
}

type identityKey struct{}

// WithIdentity returns a context carrying id.
func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, identityKey{}, id)
}

// FromContext returns the caller identity, if the request is authenticated.
func FromContext(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(identityKey{}).(Identity)
	return id, ok
}
