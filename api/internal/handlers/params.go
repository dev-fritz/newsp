package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/dev-fritz/newsp/api/internal/apperrors"
)

// uuidParam parses a UUID path parameter; an invalid value is a 400.
//
//nolint:unparam // shared by every route with a UUID path parameter
func uuidParam(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		return uuid.Nil, apperrors.Validation("invalid path parameter", map[string]string{name: "uuid"})
	}
	return id, nil
}
