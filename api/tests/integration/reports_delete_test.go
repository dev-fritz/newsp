package integration

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dev-fritz/newsp/api/tests/apitest"
)

func TestDeleteReport(t *testing.T) {
	t.Parallel()
	s := apitest.New(t)
	db := s.Container.DB
	admin, _ := setupAdmin(t, s)
	member := inviteMember(t, s, admin)

	res := s.Do(http.MethodGet, "/api/v1/me", admin, nil)
	require.Equal(t, http.StatusOK, res.Status, res.Body)
	adminID := res.Data()["id"].(string)

	sourceID, itemID := uuid.New(), uuid.New()
	mustExec(t, db, `INSERT INTO sources (id, kind, name) VALUES ($1, 'hackernews', 'HN')`, sourceID)
	mustExec(t, db, `INSERT INTO items (id, source_id, external_id, url, title, published_at) VALUES ($1, $2, '1', 'https://x', 'T', now())`, itemID, sourceID)

	res = s.Do(http.MethodGet, "/api/v1/me", member, nil)
	require.Equal(t, http.StatusOK, res.Status, res.Body)
	memberID := res.Data()["id"].(string)

	// A user has at most one active report, so the running one belongs to the member.
	newReport := func(status string, cite bool) string {
		owner := adminID
		if status == "running" {
			owner = memberID
		}
		id := uuid.New()
		mustExec(t, db, `INSERT INTO reports (id, user_id, kind, status, window_start, window_end, output_lang)
			VALUES ($1, $2, 'manual', $3, now() - interval '1 day', now(), 'pt-BR')`, id, owner, status)
		if cite {
			mustExec(t, db, `INSERT INTO report_items (report_id, item_id, ref) VALUES ($1, $2, 1)`, id, itemID)
		}
		return id.String()
	}
	reports := func() int {
		return countRows(t, db, `SELECT count(*) FROM reports`)
	}

	done := newReport("done", true)
	failed := newReport("failed", false)
	pending := newReport("pending", false)
	running := newReport("running", false)

	t.Run("invalid id", func(t *testing.T) {
		res := s.Do(http.MethodDelete, "/api/v1/reports/not-a-uuid", admin, nil)
		assert.Equal(t, http.StatusBadRequest, res.Status, res.Body)
	})

	t.Run("requires auth", func(t *testing.T) {
		res := s.Do(http.MethodDelete, "/api/v1/reports/"+done, "", nil)
		assert.Equal(t, http.StatusUnauthorized, res.Status)
	})

	t.Run("another user gets 404 and the report remains", func(t *testing.T) {
		res := s.Do(http.MethodDelete, "/api/v1/reports/"+done, member, nil)
		assert.Equal(t, http.StatusNotFound, res.Status, res.Body)
		assert.Equal(t, "REPORT_NOT_FOUND", res.ErrorCode())
		assert.Equal(t, 4, reports())
	})

	t.Run("in progress reports answer 409 and remain", func(t *testing.T) {
		for id, token := range map[string]string{pending: admin, running: member} {
			res := s.Do(http.MethodDelete, "/api/v1/reports/"+id, token, nil)
			assert.Equal(t, http.StatusConflict, res.Status, res.Body)
			assert.Equal(t, "REPORT_IN_PROGRESS", res.ErrorCode())
		}
		assert.Equal(t, 4, reports())
	})

	t.Run("owner deletes a done report with citations", func(t *testing.T) {
		res := s.Do(http.MethodDelete, "/api/v1/reports/"+done, admin, nil)
		require.Equal(t, http.StatusNoContent, res.Status, res.Body)

		res = s.Do(http.MethodGet, "/api/v1/reports/"+done, admin, nil)
		assert.Equal(t, http.StatusNotFound, res.Status)
		assert.Zero(t, countRows(t, db, `SELECT count(*) FROM report_items WHERE report_id = $1`, done))
		assert.Equal(t, 1, countRows(t, db, `SELECT count(*) FROM items WHERE id = $1`, itemID), "cited items stay")

		res = s.Do(http.MethodDelete, "/api/v1/reports/"+done, admin, nil)
		assert.Equal(t, http.StatusNotFound, res.Status)
		assert.Equal(t, "REPORT_NOT_FOUND", res.ErrorCode())
	})

	t.Run("owner deletes a failed report", func(t *testing.T) {
		res := s.Do(http.MethodDelete, "/api/v1/reports/"+failed, admin, nil)
		assert.Equal(t, http.StatusNoContent, res.Status, res.Body)
		assert.Equal(t, 2, reports())
	})

}
