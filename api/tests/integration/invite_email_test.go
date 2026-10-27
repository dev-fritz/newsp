package integration

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInviteWithEmailIsSentAndBoundToTheAddress(t *testing.T) {
	t.Parallel()
	s, box := newMailServer(t)
	admin, _ := setupAdmin(t, s)

	res := s.Do(http.MethodPost, "/api/v1/admin/invites", admin, map[string]string{"role": "member", "email": "Ana@Example.com"})
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	assert.Equal(t, "ana@example.com", res.Data()["email"])
	assert.Equal(t, true, res.Data()["emailed"])
	link, _ := res.Data()["url"].(string)
	invite := tokenFromURL(t, link)

	runJobs(t, s)
	msgs := box.Messages()
	require.Len(t, msgs, 1)
	assert.Equal(t, "ana@example.com", msgs[0].To)
	assert.Contains(t, msgs[0].Text, link)
	assert.Equal(t, invite, box.LastToken("ana@example.com"))

	res = s.Do(http.MethodPost, "/api/v1/invites/inspect", "", map[string]string{"token": invite})
	require.Equal(t, http.StatusOK, res.Status, res.Body)
	assert.Equal(t, "ana@example.com", res.Data()["email"])

	res = s.Do(http.MethodGet, "/api/v1/admin/invites", admin, nil)
	require.Equal(t, http.StatusOK, res.Status, res.Body)
	require.Len(t, res.List(), 1)
	assert.Equal(t, "ana@example.com", res.List()[0]["email"])

	res = s.Do(http.MethodPost, "/api/v1/invites/accept", "", map[string]string{
		"token": invite, "name": "Ana", "email": "other@example.com", "password": strongPassword,
	})
	assert.Equal(t, http.StatusBadRequest, res.Status, res.Body)
	assert.Equal(t, "VALIDATION_FAILED", res.ErrorCode())
	errBody, _ := res.Body["error"].(map[string]any)
	assert.Equal(t, map[string]any{"email": "invite_mismatch"}, errBody["fields"])

	res = s.Do(http.MethodPost, "/api/v1/invites/accept", "", map[string]string{
		"token": invite, "name": "Ana", "email": "ANA@example.com", "password": strongPassword,
	})
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	user, _ := res.Data()["user"].(map[string]any)
	assert.Equal(t, "ana@example.com", user["email"])
	assert.Equal(t, true, user["email_verified"])
}

func TestInviteWithoutEmailStaysOpen(t *testing.T) {
	t.Parallel()
	s, box := newMailServer(t)
	admin, _ := setupAdmin(t, s)

	res := s.Do(http.MethodPost, "/api/v1/admin/invites", admin, map[string]string{"role": "member"})
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	assert.Equal(t, false, res.Data()["emailed"])
	assert.Nil(t, res.Data()["email"])
	invite := tokenFromURL(t, res.Data()["url"].(string))

	runJobs(t, s)
	assert.Empty(t, box.Messages())

	res = s.Do(http.MethodPost, "/api/v1/invites/inspect", "", map[string]string{"token": invite})
	require.Equal(t, http.StatusOK, res.Status, res.Body)
	assert.Nil(t, res.Data()["email"])

	res = s.Do(http.MethodPost, "/api/v1/invites/accept", "", map[string]string{
		"token": invite, "name": "Bia", "email": "bia@example.com", "password": strongPassword,
	})
	assert.Equal(t, http.StatusCreated, res.Status, res.Body)
}

func TestInviteToAnExistingAccountIsRefused(t *testing.T) {
	t.Parallel()
	s, box := newMailServer(t)
	admin, _ := setupAdmin(t, s)

	res := s.Do(http.MethodPost, "/api/v1/admin/invites", admin, map[string]string{"role": "member", "email": "Admin@Example.com"})
	assert.Equal(t, http.StatusConflict, res.Status, res.Body)
	assert.Equal(t, "EMAIL_TAKEN", res.ErrorCode())

	runJobs(t, s)
	assert.Empty(t, box.Messages(), "a refused invite sends nothing")
	res = s.Do(http.MethodGet, "/api/v1/admin/invites", admin, nil)
	assert.Empty(t, res.List(), "a refused invite stores nothing")
}
