package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const resetRequestPath = "/api/v1/password-resets/request"

func TestSelfServicePasswordReset(t *testing.T) {
	t.Parallel()
	s, box := newMailServer(t)
	admin, _ := setupAdmin(t, s)
	inviteMember(t, s, admin)

	res := s.Do(http.MethodPost, resetRequestPath, "", map[string]string{"email": "Ana@Example.com"})
	require.Equal(t, http.StatusAccepted, res.Status, res.Body)
	assert.Empty(t, res.Body)
	assert.Empty(t, box.Messages(), "nothing is sent before the worker runs")

	runJobs(t, s)
	require.Len(t, box.Messages(), 1)
	token := box.LastToken("ana@example.com")
	require.NotEmpty(t, token)

	res = s.Do(http.MethodPost, "/api/v1/password-resets/inspect", "", map[string]string{"token": token})
	assert.Equal(t, http.StatusNoContent, res.Status, res.Body)

	const newPassword = "a brand new passphrase"
	res = s.Do(http.MethodPost, "/api/v1/password-resets/confirm", "", map[string]string{"token": token, "password": newPassword})
	require.Equal(t, http.StatusNoContent, res.Status, res.Body)

	res = s.Do(http.MethodPost, "/api/v1/auth/login", "", map[string]string{"email": "ana@example.com", "password": newPassword})
	assert.Equal(t, http.StatusOK, res.Status, res.Body)

	res = s.Do(http.MethodPost, "/api/v1/password-resets/confirm", "", map[string]string{"token": token, "password": newPassword})
	assert.Equal(t, http.StatusNotFound, res.Status, "reset links are single use")
}

func TestPasswordResetForUnknownEmailLooksTheSame(t *testing.T) {
	t.Parallel()
	s, box := newMailServer(t)
	setupAdmin(t, s)

	res := s.Do(http.MethodPost, resetRequestPath, "", map[string]string{"email": "ghost@example.com"})
	assert.Equal(t, http.StatusAccepted, res.Status, res.Body)
	assert.Empty(t, res.Body)

	runJobs(t, s)
	assert.Empty(t, box.Messages())
	assert.JSONEq(t, `{"skipped":"no active account"}`, jobResult(t, s.Container.DB, "auth.password_reset"))
}

func TestPasswordResetLimitDoesNotDependOnTheAccount(t *testing.T) {
	t.Parallel()
	s, _ := newMailServer(t)

	for range 3 {
		res := s.Do(http.MethodPost, resetRequestPath, "", map[string]string{"email": "ghost@example.com"})
		require.Equal(t, http.StatusAccepted, res.Status, res.Body)
	}
	res := s.Do(http.MethodPost, resetRequestPath, "", map[string]string{"email": "ghost@example.com"})
	assert.Equal(t, http.StatusTooManyRequests, res.Status, "same answer as for a real account")
}

func TestPasswordResetRejectsInvalidEmail(t *testing.T) {
	t.Parallel()
	s, _ := newMailServer(t)

	res := s.Do(http.MethodPost, resetRequestPath, "", map[string]string{"email": "not-an-email"})
	assert.Equal(t, http.StatusBadRequest, res.Status)
}

func TestPasswordResetIsRateLimitedPerEmail(t *testing.T) {
	t.Parallel()
	s, box := newMailServer(t)
	admin, _ := setupAdmin(t, s)
	inviteMember(t, s, admin)

	for range 3 {
		res := s.Do(http.MethodPost, resetRequestPath, "", map[string]string{"email": "ana@example.com"})
		require.Equal(t, http.StatusAccepted, res.Status, res.Body)
	}
	res := s.Do(http.MethodPost, resetRequestPath, "", map[string]string{"email": "ana@example.com"})
	assert.Equal(t, http.StatusTooManyRequests, res.Status)
	assert.NotEmpty(t, res.Header.Get("Retry-After"))

	runJobs(t, s)
	assert.Len(t, box.Messages(), 1, "repeated requests collapse into one job")
}

func TestSelfServiceResetKeepsAdminLinkValid(t *testing.T) {
	t.Parallel()
	s, box := newMailServer(t)
	admin, _ := setupAdmin(t, s)
	inviteMember(t, s, admin)
	var memberID string
	require.NoError(t, s.Container.DB.QueryRowContext(context.Background(),
		`SELECT id::text FROM users WHERE email = 'ana@example.com'`).Scan(&memberID))

	res := s.Do(http.MethodPost, "/api/v1/admin/users/"+memberID+"/password-reset", admin, nil)
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	adminLink := tokenFromURL(t, res.Data()["url"].(string))

	// First self-service request, then a second one that must replace it.
	res = s.Do(http.MethodPost, resetRequestPath, "", map[string]string{"email": "ana@example.com"})
	require.Equal(t, http.StatusAccepted, res.Status, res.Body)
	runJobs(t, s)
	firstSelf := box.LastToken("ana@example.com")
	require.NotEmpty(t, firstSelf)

	res = s.Do(http.MethodPost, resetRequestPath, "", map[string]string{"email": "ana@example.com"})
	require.Equal(t, http.StatusAccepted, res.Status, res.Body)
	runJobs(t, s)
	secondSelf := box.LastToken("ana@example.com")
	require.NotEmpty(t, secondSelf)
	require.NotEqual(t, firstSelf, secondSelf)

	inspect := func(token string) int {
		return s.Do(http.MethodPost, "/api/v1/password-resets/inspect", "", map[string]string{"token": token}).Status
	}
	assert.Equal(t, http.StatusNoContent, inspect(adminLink), "an admin link survives self-service requests")
	assert.Equal(t, http.StatusNoContent, inspect(secondSelf), "the newest self-service link works")
	assert.Equal(t, http.StatusNotFound, inspect(firstSelf), "an older self-service link is replaced")
}
