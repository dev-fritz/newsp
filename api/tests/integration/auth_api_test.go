package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dev-fritz/newsp/api/internal/repositories"
	"github.com/dev-fritz/newsp/api/tests/apitest"
)

const strongPassword = "correct horse battery staple"

func tokens(r apitest.Response) (access, refresh string) {
	d := r.Data()
	access, _ = d["access_token"].(string)
	refresh, _ = d["refresh_token"].(string)
	return access, refresh
}

func setupAdmin(t *testing.T, s *apitest.Server) (access, refresh string) {
	t.Helper()
	res := s.Do(http.MethodPost, "/api/v1/setup", "", map[string]string{
		"name": "Admin", "email": "admin@example.com", "password": strongPassword,
	})
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	return tokens(res)
}

// tokenFromURL extracts the secret from a one-time link.
func tokenFromURL(t *testing.T, url string) string {
	t.Helper()
	i := strings.LastIndex(url, "/")
	require.Positive(t, i)
	return url[i+1:]
}

func TestSetupFlow(t *testing.T) {
	t.Parallel()
	s := apitest.New(t)

	res := s.Do(http.MethodGet, "/api/v1/setup/status", "", nil)
	assert.Equal(t, true, res.Data()["setup_required"])

	res = s.Do(http.MethodPost, "/api/v1/setup", "", map[string]string{"name": "A", "email": "a@example.com", "password": "short"})
	assert.Equal(t, http.StatusBadRequest, res.Status)

	access, _ := setupAdmin(t, s)

	res = s.Do(http.MethodGet, "/api/v1/setup/status", "", nil)
	assert.Equal(t, false, res.Data()["setup_required"])

	res = s.Do(http.MethodPost, "/api/v1/setup", "", map[string]string{"name": "B", "email": "b@example.com", "password": strongPassword})
	assert.Equal(t, http.StatusConflict, res.Status)
	assert.Equal(t, "SETUP_ALREADY_DONE", res.ErrorCode())

	res = s.Do(http.MethodGet, "/api/v1/me", access, nil)
	require.Equal(t, http.StatusOK, res.Status)
	assert.Equal(t, "admin", res.Data()["role"])
	assert.NotEmpty(t, res.Data()["permissions"])
	assert.NotContains(t, res.Data(), "password_hash")
}

func TestLoginAndRefreshRotation(t *testing.T) {
	t.Parallel()
	s := apitest.New(t)
	setupAdmin(t, s)

	res := s.Do(http.MethodPost, "/api/v1/auth/login", "", map[string]string{"email": "admin@example.com", "password": "wrong password here"})
	assert.Equal(t, http.StatusUnauthorized, res.Status)
	assert.Equal(t, "INVALID_CREDENTIALS", res.ErrorCode())

	res = s.Do(http.MethodPost, "/api/v1/auth/login", "", map[string]string{"email": "nobody@example.com", "password": strongPassword})
	assert.Equal(t, "INVALID_CREDENTIALS", res.ErrorCode(), "unknown e-mails get the same answer")

	res = s.Do(http.MethodPost, "/api/v1/auth/login", "", map[string]string{"email": "ADMIN@example.com", "password": strongPassword})
	require.Equal(t, http.StatusOK, res.Status)
	_, refresh1 := tokens(res)

	res = s.Do(http.MethodPost, "/api/v1/auth/refresh", "", map[string]string{"refresh_token": refresh1})
	require.Equal(t, http.StatusOK, res.Status)
	access2, refresh2 := tokens(res)
	assert.NotEqual(t, refresh1, refresh2)

	// Reusing the rotated token signals theft: the whole family dies.
	res = s.Do(http.MethodPost, "/api/v1/auth/refresh", "", map[string]string{"refresh_token": refresh1})
	assert.Equal(t, http.StatusUnauthorized, res.Status)
	res = s.Do(http.MethodPost, "/api/v1/auth/refresh", "", map[string]string{"refresh_token": refresh2})
	assert.Equal(t, http.StatusUnauthorized, res.Status, "the newest token of the family is revoked too")

	// The access token stays valid until it expires (short TTL by design).
	res = s.Do(http.MethodGet, "/api/v1/me", access2, nil)
	assert.Equal(t, http.StatusOK, res.Status)
}

func TestLogout(t *testing.T) {
	t.Parallel()
	s := apitest.New(t)
	_, refresh := setupAdmin(t, s)

	res := s.Do(http.MethodPost, "/api/v1/auth/logout", "", map[string]string{"refresh_token": refresh})
	assert.Equal(t, http.StatusNoContent, res.Status)
	res = s.Do(http.MethodPost, "/api/v1/auth/refresh", "", map[string]string{"refresh_token": refresh})
	assert.Equal(t, http.StatusUnauthorized, res.Status)
}

func TestProtectedRoutes(t *testing.T) {
	t.Parallel()
	s := apitest.New(t)

	res := s.Do(http.MethodGet, "/api/v1/me", "", nil)
	assert.Equal(t, http.StatusUnauthorized, res.Status)
	res = s.Do(http.MethodGet, "/api/v1/me", "forged.token.value", nil)
	assert.Equal(t, http.StatusUnauthorized, res.Status)
	res = s.Do(http.MethodGet, "/api/v1/admin/users", "", nil)
	assert.Equal(t, http.StatusUnauthorized, res.Status)
}

func TestInviteFlowAndAuthorization(t *testing.T) {
	t.Parallel()
	s := apitest.New(t)
	admin, _ := setupAdmin(t, s)

	res := s.Do(http.MethodPost, "/api/v1/admin/invites", admin, map[string]string{"role": "member"})
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	url, _ := res.Data()["url"].(string)
	assert.True(t, strings.HasPrefix(url, "https://newsp.test/invite/"))
	token := tokenFromURL(t, url)

	res = s.Do(http.MethodGet, "/api/v1/admin/invites", admin, nil)
	require.Equal(t, http.StatusOK, res.Status)
	require.Len(t, res.List(), 1)
	assert.NotContains(t, res.List()[0], "url", "pending invites never expose the link again")

	res = s.Do(http.MethodPost, "/api/v1/invites/inspect", "", map[string]string{"token": token})
	require.Equal(t, http.StatusOK, res.Status)
	assert.Equal(t, "member", res.Data()["role"])

	res = s.Do(http.MethodPost, "/api/v1/invites/accept", "", map[string]string{
		"token": token, "name": "Ana", "email": "admin@example.com", "password": strongPassword,
	})
	assert.Equal(t, http.StatusConflict, res.Status, "taken e-mail")
	assert.Equal(t, "EMAIL_TAKEN", res.ErrorCode())

	res = s.Do(http.MethodPost, "/api/v1/invites/accept", "", map[string]string{
		"token": token, "name": "Ana", "email": "ana@example.com", "password": strongPassword,
	})
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	member, _ := tokens(res)
	assert.Equal(t, "member", res.Data()["user"].(map[string]any)["role"])

	res = s.Do(http.MethodPost, "/api/v1/invites/accept", "", map[string]string{
		"token": token, "name": "Eve", "email": "eve@example.com", "password": strongPassword,
	})
	assert.Equal(t, http.StatusNotFound, res.Status, "invites are single use")
	assert.Equal(t, "INVALID_LINK", res.ErrorCode())

	// A member cannot reach any admin endpoint.
	for _, call := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/admin/users"},
		{http.MethodPost, "/api/v1/admin/invites"},
		{http.MethodGet, "/api/v1/admin/invites"},
	} {
		res = s.Do(call.method, call.path, member, map[string]string{"role": "admin"})
		assert.Equal(t, http.StatusForbidden, res.Status, call.path)
	}
}

func TestPasswordResetEndsSessions(t *testing.T) {
	t.Parallel()
	s := apitest.New(t)
	admin, _ := setupAdmin(t, s)

	res := s.Do(http.MethodPost, "/api/v1/admin/invites", admin, map[string]string{"role": "member"})
	invite := tokenFromURL(t, res.Data()["url"].(string))
	res = s.Do(http.MethodPost, "/api/v1/invites/accept", "", map[string]string{
		"token": invite, "name": "Ana", "email": "ana@example.com", "password": strongPassword,
	})
	_, memberRefresh := tokens(res)
	memberID := res.Data()["user"].(map[string]any)["id"].(string)

	res = s.Do(http.MethodPost, "/api/v1/admin/users/"+memberID+"/password-reset", admin, nil)
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	reset := tokenFromURL(t, res.Data()["url"].(string))

	res = s.Do(http.MethodPost, "/api/v1/password-resets/inspect", "", map[string]string{"token": reset})
	assert.Equal(t, http.StatusNoContent, res.Status)

	const newPassword = "a brand new passphrase"
	res = s.Do(http.MethodPost, "/api/v1/password-resets/confirm", "", map[string]string{"token": reset, "password": newPassword})
	require.Equal(t, http.StatusNoContent, res.Status, res.Body)

	res = s.Do(http.MethodPost, "/api/v1/auth/refresh", "", map[string]string{"refresh_token": memberRefresh})
	assert.Equal(t, http.StatusUnauthorized, res.Status, "old sessions end after a reset")

	res = s.Do(http.MethodPost, "/api/v1/auth/login", "", map[string]string{"email": "ana@example.com", "password": newPassword})
	assert.Equal(t, http.StatusOK, res.Status)

	res = s.Do(http.MethodPost, "/api/v1/password-resets/confirm", "", map[string]string{"token": reset, "password": newPassword})
	assert.Equal(t, http.StatusNotFound, res.Status, "reset links are single use")
}

func TestUserAdministration(t *testing.T) {
	t.Parallel()
	s := apitest.New(t)
	admin, _ := setupAdmin(t, s)

	res := s.Do(http.MethodGet, "/api/v1/me", admin, nil)
	adminID := res.Data()["id"].(string)

	res = s.Do(http.MethodPatch, "/api/v1/admin/users/"+adminID, admin, map[string]string{"status": "disabled"})
	assert.Equal(t, http.StatusUnprocessableEntity, res.Status)
	assert.Equal(t, "CANNOT_CHANGE_OWN_ACCESS", res.ErrorCode())

	res = s.Do(http.MethodPost, "/api/v1/admin/invites", admin, map[string]string{"role": "member"})
	invite := tokenFromURL(t, res.Data()["url"].(string))
	res = s.Do(http.MethodPost, "/api/v1/invites/accept", "", map[string]string{
		"token": invite, "name": "Ana", "email": "ana@example.com", "password": strongPassword,
	})
	_, memberRefresh := tokens(res)
	memberID := res.Data()["user"].(map[string]any)["id"].(string)

	res = s.Do(http.MethodPatch, "/api/v1/admin/users/"+memberID, admin, map[string]string{"status": "disabled"})
	require.Equal(t, http.StatusOK, res.Status, res.Body)
	assert.Equal(t, "disabled", res.Data()["status"])

	res = s.Do(http.MethodPost, "/api/v1/auth/refresh", "", map[string]string{"refresh_token": memberRefresh})
	assert.Equal(t, http.StatusUnauthorized, res.Status, "disabling ends sessions")
	res = s.Do(http.MethodPost, "/api/v1/auth/login", "", map[string]string{"email": "ana@example.com", "password": strongPassword})
	assert.Equal(t, http.StatusUnauthorized, res.Status)

	res = s.Do(http.MethodPatch, "/api/v1/admin/users/"+memberID, admin, map[string]string{"role": "owner"})
	assert.Equal(t, http.StatusBadRequest, res.Status)
	res = s.Do(http.MethodPatch, "/api/v1/admin/users/not-a-uuid", admin, map[string]string{"status": "active"})
	assert.Equal(t, http.StatusBadRequest, res.Status)
	res = s.Do(http.MethodPatch, "/api/v1/admin/users/00000000-0000-0000-0000-000000000000", admin, map[string]string{"status": "active"})
	assert.Equal(t, http.StatusNotFound, res.Status)

	res = s.Do(http.MethodGet, "/api/v1/admin/users?limit=1", admin, nil)
	require.Equal(t, http.StatusOK, res.Status)
	assert.Len(t, res.List(), 1)
	assert.InDelta(t, 2, res.Body["meta"].(map[string]any)["total"], 0)
}

// New accounts get their default preferences right away, so the scheduler
// sends the daily report without a visit to the profile.
func TestNewAccountsAreScheduledForReports(t *testing.T) {
	t.Parallel()
	s := apitest.New(t)
	admin, _ := setupAdmin(t, s)

	res := s.Do(http.MethodPost, "/api/v1/admin/invites", admin, map[string]string{"role": "member"})
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	url, _ := res.Data()["url"].(string)
	res = s.Do(http.MethodPost, "/api/v1/invites/accept", "", map[string]string{
		"token": tokenFromURL(t, url), "name": "Ana", "email": "ana@example.com", "password": strongPassword,
	})
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	member, _ := tokens(res)

	for _, access := range []string{admin, member} {
		res = s.Do(http.MethodGet, "/api/v1/me/preferences", access, nil)
		require.Equal(t, http.StatusOK, res.Status, res.Body)
		assert.Equal(t, true, res.Data()["report_enabled"])
		assert.NotNil(t, res.Data()["next_report_at"], "the first report is scheduled")
	}

	due, err := repositories.NewPreferenceRepository(s.Container.DB).
		DueReportUsers(context.Background(), time.Now().Add(25*time.Hour), 10)
	require.NoError(t, err)
	assert.Len(t, due, 2, "both accounts are due within a day")
}
