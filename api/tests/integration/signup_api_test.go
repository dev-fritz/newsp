package integration

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dev-fritz/newsp/api/tests/apitest"
)

func enableSignup(t *testing.T, s *apitest.Server, admin string) {
	t.Helper()
	res := s.Do(http.MethodPatch, "/api/v1/admin/settings", admin, map[string]any{"signup_enabled": true})
	require.Equal(t, http.StatusOK, res.Status, res.Body)
	assert.Equal(t, true, res.Data()["signup_enabled"])
	assert.Equal(t, true, res.Data()["mail_configured"])
}

func signup(s *apitest.Server, email string) apitest.Response {
	return s.Do(http.MethodPost, "/api/v1/auth/signup", "", map[string]string{
		"name": "Bia", "email": email, "password": strongPassword, "locale": "en",
	})
}

func TestSignupIsOffByDefault(t *testing.T) {
	t.Parallel()
	s, _ := newMailServer(t)
	setupAdmin(t, s)

	res := s.Do(http.MethodGet, "/api/v1/setup/status", "", nil)
	assert.Equal(t, false, res.Data()["signup_enabled"])
	assert.Equal(t, true, res.Data()["password_reset_enabled"], "development prints e-mails to the log")

	res = signup(s, "bia@example.com")
	assert.Equal(t, http.StatusForbidden, res.Status)
	assert.Equal(t, "SIGNUP_DISABLED", res.ErrorCode())
}

func TestSignupAndVerifyEmail(t *testing.T) {
	t.Parallel()
	s, box := newMailServer(t)
	admin, _ := setupAdmin(t, s)
	enableSignup(t, s, admin)

	res := s.Do(http.MethodGet, "/api/v1/setup/status", "", nil)
	assert.Equal(t, true, res.Data()["signup_enabled"])

	res = signup(s, "Bia@Example.com")
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	member, _ := tokens(res)
	user, _ := res.Data()["user"].(map[string]any)
	assert.Equal(t, "member", user["role"])
	assert.Equal(t, false, user["email_verified"])

	res = signup(s, "bia@example.com")
	assert.Equal(t, http.StatusConflict, res.Status)
	assert.Equal(t, "EMAIL_TAKEN", res.ErrorCode())

	runJobs(t, s)
	msgs := box.Messages()
	require.Len(t, msgs, 1)
	assert.Equal(t, "Confirm your e-mail on newsp", msgs[0].Subject)
	token := box.LastToken("bia@example.com")
	require.NotEmpty(t, token)

	res = s.Do(http.MethodPost, "/api/v1/auth/verify-email", "", map[string]string{"token": token})
	assert.Equal(t, http.StatusNoContent, res.Status, res.Body)
	res = s.Do(http.MethodGet, "/api/v1/me", member, nil)
	assert.Equal(t, true, res.Data()["email_verified"])

	res = s.Do(http.MethodPost, "/api/v1/auth/verify-email", "", map[string]string{"token": token})
	assert.Equal(t, http.StatusNotFound, res.Status)
	assert.Equal(t, "INVALID_LINK", res.ErrorCode())

	res = s.Do(http.MethodPost, "/api/v1/me/verify-email/resend", member, nil)
	assert.Equal(t, http.StatusConflict, res.Status)
	assert.Equal(t, "EMAIL_ALREADY_VERIFIED", res.ErrorCode())
}

func TestResendVerificationIsRateLimited(t *testing.T) {
	t.Parallel()
	s, box := newMailServer(t)
	admin, _ := setupAdmin(t, s)
	enableSignup(t, s, admin)
	res := signup(s, "bia@example.com")
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	member, _ := tokens(res)

	for range 3 {
		res = s.Do(http.MethodPost, "/api/v1/me/verify-email/resend", member, nil)
		require.Equal(t, http.StatusNoContent, res.Status, res.Body)
	}
	res = s.Do(http.MethodPost, "/api/v1/me/verify-email/resend", member, nil)
	assert.Equal(t, http.StatusTooManyRequests, res.Status)
	assert.NotEmpty(t, res.Header.Get("Retry-After"))

	runJobs(t, s)
	assert.Len(t, box.Messages(), 4, "signup plus three resends")
}

func TestSignupIsRateLimitedPerIP(t *testing.T) {
	t.Parallel()
	s, _ := newMailServer(t)
	admin, _ := setupAdmin(t, s)
	enableSignup(t, s, admin)

	for i := range 5 {
		res := signup(s, "user"+string(rune('a'+i))+"@example.com")
		require.Equal(t, http.StatusCreated, res.Status, res.Body)
	}
	res := signup(s, "user-f@example.com")
	assert.Equal(t, http.StatusTooManyRequests, res.Status)
}

func TestInvitedAccountsStartVerified(t *testing.T) {
	t.Parallel()
	s := apitest.New(t)
	admin, _ := setupAdmin(t, s)
	member := inviteMember(t, s, admin)

	res := s.Do(http.MethodGet, "/api/v1/me", member, nil)
	assert.Equal(t, true, res.Data()["email_verified"])
	res = s.Do(http.MethodGet, "/api/v1/me", admin, nil)
	assert.Equal(t, true, res.Data()["email_verified"])
}
