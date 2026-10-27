package handlers

import (
	"net/http"
	"strconv"

	"github.com/dev-fritz/newsp/api/internal/apperrors"
	"github.com/dev-fritz/newsp/api/internal/dtos"
)

// parsePage reads limit/offset from the query string, capping the limit.
func parsePage(r *http.Request) (limit, offset int, err error) {
	limit, offset = dtos.DefaultPageLimit, 0
	q := r.URL.Query()
	if raw := q.Get("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 {
			return 0, 0, apperrors.Validation("invalid query", map[string]string{"limit": "min"})
		}
		limit = min(limit, dtos.MaxPageLimit)
	}
	if raw := q.Get("offset"); raw != "" {
		offset, err = strconv.Atoi(raw)
		if err != nil || offset < 0 {
			return 0, 0, apperrors.Validation("invalid query", map[string]string{"offset": "min"})
		}
	}
	return limit, offset, nil
}
