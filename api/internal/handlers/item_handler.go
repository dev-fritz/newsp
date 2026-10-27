package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dev-fritz/newsp/api/internal/apperrors"
	"github.com/dev-fritz/newsp/api/internal/dtos"
	"github.com/dev-fritz/newsp/api/internal/services"
)

// ItemHandler serves the item feed.
type ItemHandler struct {
	service *services.ItemService
}

// NewItemHandler builds an ItemHandler.
func NewItemHandler(service *services.ItemService) *ItemHandler {
	return &ItemHandler{service: service}
}

func itemResponse(v services.ItemView) dtos.ItemResponse {
	it := v.Item
	return dtos.ItemResponse{
		ID: it.ID.String(), Source: dtos.ItemSourceResponse{ID: it.SourceID.String(), Kind: v.SourceKind, Name: v.SourceName},
		URL: it.URL, Title: it.Title, Summary: it.Summary, Author: it.Author, PublishedAt: it.PublishedAt,
		CollectedAt: it.CollectedAt, Score: it.Score, Comments: it.Comments, Metrics: it.Metrics, Cited: v.Cited,
	}
}

// parseItemFilter reads the feed filters from the query string.
func parseItemFilter(r *http.Request) (services.ItemFilter, error) {
	q := r.URL.Query()
	fields := map[string]string{}
	var f services.ItemFilter

	if raw := q.Get("source_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			fields["source_id"] = "uuid"
		} else {
			f.SourceID = &id
		}
	}
	parseTime := func(name string) *time.Time {
		raw := q.Get(name)
		if raw == "" {
			return nil
		}
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			fields[name] = "datetime"
			return nil
		}
		return &t
	}
	f.Since, f.Until = parseTime("since"), parseTime("until")
	f.Query = strings.TrimSpace(q.Get("q"))
	f.Sort = services.ItemSort(q.Get("sort"))

	if len(fields) > 0 {
		return services.ItemFilter{}, apperrors.Validation("invalid query", fields)
	}
	return f, nil
}

// List godoc
// @Summary      Item feed
// @Description  Collected items, newest first by default. q is a full-text search (web search syntax); sort=engagement orders by score plus comments. cited tells whether a report or an idea cites the item.
// @Tags         items
// @Produce      json
// @Param        source_id query string false "Only items of this source"
// @Param        since     query string false "Published at or after (RFC 3339)"
// @Param        until     query string false "Published before (RFC 3339)"
// @Param        q         query string false "Full-text search (max 200 characters)"
// @Param        sort      query string false "Order" Enums(recent, engagement) default(recent)
// @Param        limit     query int    false "Page size (max 100)"
// @Param        offset    query int    false "Offset"
// @Success      200 {object} dtos.ItemListEnvelope
// @Failure      400 {object} dtos.ErrorResponse
// @Failure      401 {object} dtos.ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/items [get]
func (h *ItemHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := parsePage(r)
	if err != nil {
		respondError(w, r, err)
		return
	}
	filter, err := parseItemFilter(r)
	if err != nil {
		respondError(w, r, err)
		return
	}
	items, total, err := h.service.List(r.Context(), filter, limit, offset)
	if err != nil {
		respondError(w, r, err)
		return
	}
	out := make([]dtos.ItemResponse, len(items))
	for i, v := range items {
		out[i] = itemResponse(v)
	}
	respondPage(w, out, dtos.PageMeta{Total: total, Limit: limit, Offset: offset})
}
