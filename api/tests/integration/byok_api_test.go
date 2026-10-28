package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dev-fritz/newsp/api/internal/services"
	"github.com/dev-fritz/newsp/api/pkg/ai"
	"github.com/dev-fritz/newsp/api/tests/apitest"
)

const memberKey = "sk-member-secret-wxyz"

// byokServer starts an API whose provider clients keep the configured kind but
// talk to a fake OpenAI-compatible server: the official hosts users are
// limited to must never be reached from tests.
func byokServer(t *testing.T) *apitest.Server {
	t.Helper()
	fake := fakeOpenAI(t)
	factory := services.ProviderFactory(func(cfg ai.Config) (ai.Provider, error) {
		cfg.BaseURL = fake.URL + "/v1"
		return ai.New(cfg)
	})
	return apitest.NewWithAI(t, factory)
}

// inviteNamedMember accepts an invite and returns the new member's access token.
func inviteNamedMember(t *testing.T, s *apitest.Server, admin, name, email string) string {
	t.Helper()
	res := s.Do(http.MethodPost, "/api/v1/admin/invites", admin, map[string]string{"role": "member"})
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	res = s.Do(http.MethodPost, "/api/v1/invites/accept", "", map[string]string{
		"token": tokenFromURL(t, res.Data()["url"].(string)), "name": name, "email": email, "password": strongPassword,
	})
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	access, _ := tokens(res)
	return access
}

func preferencesBody(reportModelID string) map[string]any {
	return map[string]any{
		"topics": []string{}, "output_lang": "en", "timezone": "America/Sao_Paulo", "report_enabled": false,
		"report_time": "08:00", "ideas_per_day": 0, "theme": "dark", "excluded_source_ids": []string{},
		"ui_locale": "en", "report_model_id": reportModelID,
	}
}

func errorFields(res apitest.Response) map[string]any {
	e, _ := res.Body["error"].(map[string]any)
	f, _ := e["fields"].(map[string]any)
	return f
}

func TestBYOKProviderLifecycle(t *testing.T) {
	t.Parallel()
	s := byokServer(t)
	admin, _ := setupAdmin(t, s)
	member := inviteNamedMember(t, s, admin, "Ana", "ana@example.com")
	other := inviteNamedMember(t, s, admin, "Bia", "bia@example.com")

	res := s.Do(http.MethodGet, "/api/v1/me/ai/providers", member, nil)
	require.Equal(t, http.StatusOK, res.Status)
	assert.Equal(t, true, res.Data()["byok_enabled"])
	assert.Equal(t, []any{}, res.Data()["providers"], "providers is [] and never null")

	// Validation: only the hosted kinds, and the key is required.
	res = s.Do(http.MethodPost, "/api/v1/me/ai/providers", member, map[string]string{"kind": "openai_compatible", "api_key": memberKey})
	assert.Equal(t, http.StatusBadRequest, res.Status)
	res = s.Do(http.MethodPost, "/api/v1/me/ai/providers", member, map[string]string{"kind": "openai"})
	assert.Equal(t, http.StatusBadRequest, res.Status)
	res = s.Do(http.MethodPost, "/api/v1/me/ai/providers", "", map[string]string{"kind": "openai", "api_key": memberKey})
	assert.Equal(t, http.StatusUnauthorized, res.Status)

	// Create: fixed official URL, encrypted key, never echoed back.
	res = s.Do(http.MethodPost, "/api/v1/me/ai/providers", member, map[string]string{"kind": "openai", "api_key": memberKey})
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	providerID := res.Data()["id"].(string)
	assert.Equal(t, "https://api.openai.com/v1", res.Data()["base_url"])
	assert.Equal(t, true, res.Data()["has_api_key"])
	assert.NotContains(t, res.Body, "api_key")
	assert.NotContains(t, res.Data(), "api_key")

	var stored []byte
	require.NoError(t, s.Container.DB.QueryRowContext(context.Background(), `SELECT api_key_enc FROM ai_providers WHERE id = $1`, providerID).Scan(&stored))
	require.NotEmpty(t, stored)
	assert.NotContains(t, string(stored), memberKey, "keys are encrypted at rest")

	res = s.Do(http.MethodPost, "/api/v1/me/ai/providers", member, map[string]string{"kind": "openai", "api_key": "another"})
	assert.Equal(t, http.StatusConflict, res.Status)
	assert.Equal(t, "PROVIDER_KIND_TAKEN", res.ErrorCode())

	// Another user may connect the same kind: the limit is per user.
	res = s.Do(http.MethodPost, "/api/v1/me/ai/providers", other, map[string]string{"kind": "openai", "api_key": "sk-other-key-1234"})
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	otherProviderID := res.Data()["id"].(string)

	res = s.Do(http.MethodGet, "/api/v1/me/ai/providers", member, nil)
	require.Equal(t, http.StatusOK, res.Status)
	providers := res.Data()["providers"].([]any)
	require.Len(t, providers, 1, "only my own providers")
	assert.Equal(t, "••••wxyz", providers[0].(map[string]any)["key_hint"])
	assert.NotContains(t, res.Body, memberKey)

	// Test connection and register a chat model.
	res = s.Do(http.MethodPost, "/api/v1/me/ai/providers/"+providerID+"/test", member, nil)
	require.Equal(t, http.StatusOK, res.Status, res.Body)
	assert.Equal(t, true, res.Data()["ok"])
	assert.Equal(t, []any{"qwen3:14b", "nomic-embed-text"}, res.Data()["models"])

	res = s.Do(http.MethodPost, "/api/v1/me/ai/providers/"+providerID+"/models", member, map[string]string{"name": "qwen3:14b", "display_name": "My Qwen"})
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	modelID := res.Data()["id"].(string)
	assert.Equal(t, "chat", res.Data()["capability"])
	res = s.Do(http.MethodPost, "/api/v1/me/ai/providers/"+providerID+"/models", member, map[string]string{"name": "qwen3:14b"})
	assert.Equal(t, http.StatusConflict, res.Status)
	assert.Equal(t, "MODEL_EXISTS", res.ErrorCode())
	res = s.Do(http.MethodPost, "/api/v1/me/ai/providers/"+providerID+"/models", member, map[string]string{"display_name": "no name"})
	assert.Equal(t, http.StatusBadRequest, res.Status)

	// An instance model coexists in the picker, flagged own=false.
	res = s.Do(http.MethodPost, "/api/v1/admin/ai/providers", admin, map[string]string{"name": "Local", "kind": "openai_compatible", "base_url": "http://localhost:11434/v1"})
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	instanceProviderID := res.Data()["id"].(string)
	res = s.Do(http.MethodPost, "/api/v1/admin/ai/providers/"+instanceProviderID+"/models", admin, map[string]string{"name": "llama3", "capability": "chat"})
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	instanceModelID := res.Data()["id"].(string)

	own := func(token string) map[string]bool {
		res := s.Do(http.MethodGet, "/api/v1/ai/models", token, nil)
		require.Equal(t, http.StatusOK, res.Status)
		out := map[string]bool{}
		for _, m := range res.List() {
			out[m["id"].(string)] = m["own"].(bool)
		}
		return out
	}
	assert.Equal(t, map[string]bool{instanceModelID: false, modelID: true}, own(member))
	assert.Equal(t, map[string]bool{instanceModelID: false}, own(other), "other users do not see my models")
	assert.Equal(t, map[string]bool{instanceModelID: false}, own(admin))

	// Preferences accept my model, but not another user's.
	res = s.Do(http.MethodPut, "/api/v1/me/preferences", member, preferencesBody(modelID))
	require.Equal(t, http.StatusOK, res.Status, res.Body)
	assert.Equal(t, modelID, res.Data()["report_model_id"])
	res = s.Do(http.MethodPut, "/api/v1/me/preferences", other, preferencesBody(modelID))
	assert.Equal(t, http.StatusBadRequest, res.Status)
	assert.Equal(t, "unavailable", errorFields(res)["report_model_id"])

	// Other users cannot touch my provider or model, and answer 404 as if they did not exist.
	for _, c := range []struct {
		method, path string
		body         any
	}{
		{http.MethodPatch, "/api/v1/me/ai/providers/" + providerID, map[string]bool{"enabled": false}},
		{http.MethodDelete, "/api/v1/me/ai/providers/" + providerID, nil},
		{http.MethodPost, "/api/v1/me/ai/providers/" + providerID + "/test", nil},
		{http.MethodPost, "/api/v1/me/ai/providers/" + providerID + "/models", map[string]string{"name": "x"}},
		{http.MethodPatch, "/api/v1/me/ai/models/" + modelID, map[string]bool{"enabled": false}},
		{http.MethodDelete, "/api/v1/me/ai/models/" + modelID, nil},
	} {
		res = s.Do(c.method, c.path, other, c.body)
		assert.Equal(t, http.StatusNotFound, res.Status, "%s %s", c.method, c.path)
	}
	res = s.Do(http.MethodGet, "/api/v1/me/ai/providers", other, nil)
	providers = res.Data()["providers"].([]any)
	require.Len(t, providers, 1)
	assert.Equal(t, otherProviderID, providers[0].(map[string]any)["id"])
	assert.Empty(t, providers[0].(map[string]any)["models"])

	// The instance side is unreachable from /me, and mine from /admin.
	res = s.Do(http.MethodPatch, "/api/v1/me/ai/providers/"+instanceProviderID, member, map[string]bool{"enabled": false})
	assert.Equal(t, http.StatusNotFound, res.Status, "instance providers are not editable by users")
	res = s.Do(http.MethodDelete, "/api/v1/me/ai/providers/"+instanceProviderID, admin, nil)
	assert.Equal(t, http.StatusNotFound, res.Status, "not even by admins through /me")
	res = s.Do(http.MethodGet, "/api/v1/admin/ai/providers", admin, nil)
	require.Equal(t, http.StatusOK, res.Status)
	require.Len(t, res.List(), 1, "admins only list instance providers")
	assert.Equal(t, instanceProviderID, res.List()[0]["id"])
	res = s.Do(http.MethodPatch, "/api/v1/admin/ai/providers/"+providerID, admin, map[string]bool{"enabled": false})
	assert.Equal(t, http.StatusNotFound, res.Status)
	res = s.Do(http.MethodPatch, "/api/v1/admin/settings", admin, map[string]string{"default_report_model_id": modelID})
	assert.Equal(t, http.StatusBadRequest, res.Status, "instance defaults cannot point at a user's model")

	// A rejected key is reported readably; replacing it fixes the provider.
	res = s.Do(http.MethodPatch, "/api/v1/me/ai/providers/"+providerID, member, map[string]string{"api_key": "bad"})
	require.Equal(t, http.StatusOK, res.Status, res.Body)
	assert.Equal(t, true, res.Data()["has_api_key"])
	res = s.Do(http.MethodPost, "/api/v1/me/ai/providers/"+providerID+"/test", member, nil)
	require.Equal(t, http.StatusOK, res.Status)
	assert.Equal(t, false, res.Data()["ok"])
	assert.Contains(t, res.Data()["error"], "rejected the API key")
	res = s.Do(http.MethodPatch, "/api/v1/me/ai/providers/"+providerID, member, map[string]string{"api_key": ""})
	assert.Equal(t, http.StatusBadRequest, res.Status, "the key can be replaced, not removed")
	res = s.Do(http.MethodPatch, "/api/v1/me/ai/providers/"+providerID, member, map[string]string{"api_key": memberKey})
	require.Equal(t, http.StatusOK, res.Status)

	// Models can be renamed and disabled; disabled ones leave the picker.
	res = s.Do(http.MethodPatch, "/api/v1/me/ai/models/"+modelID, member, map[string]any{"display_name": "Renamed", "enabled": false})
	require.Equal(t, http.StatusOK, res.Status, res.Body)
	assert.Equal(t, "Renamed", res.Data()["display_name"])
	assert.Equal(t, map[string]bool{instanceModelID: false}, own(member))
	res = s.Do(http.MethodPatch, "/api/v1/me/ai/models/not-a-uuid", member, map[string]any{"enabled": true})
	assert.Equal(t, http.StatusBadRequest, res.Status)
	res = s.Do(http.MethodPatch, "/api/v1/me/ai/models/"+modelID, member, map[string]any{"enabled": true})
	require.Equal(t, http.StatusOK, res.Status)

	// Deleting a model or provider removes it from the list.
	res = s.Do(http.MethodDelete, "/api/v1/me/ai/models/"+modelID, member, nil)
	assert.Equal(t, http.StatusNoContent, res.Status)
	res = s.Do(http.MethodDelete, "/api/v1/me/ai/models/"+modelID, member, nil)
	assert.Equal(t, http.StatusNotFound, res.Status)
	res = s.Do(http.MethodDelete, "/api/v1/me/ai/providers/"+providerID, member, nil)
	assert.Equal(t, http.StatusNoContent, res.Status)
	res = s.Do(http.MethodGet, "/api/v1/me/ai/providers", member, nil)
	assert.Empty(t, res.Data()["providers"])
}

func TestBYOKSwitchedOffByAdmin(t *testing.T) {
	t.Parallel()
	s := byokServer(t)
	admin, _ := setupAdmin(t, s)
	member := inviteNamedMember(t, s, admin, "Ana", "ana@example.com")

	res := s.Do(http.MethodGet, "/api/v1/admin/settings", admin, nil)
	require.Equal(t, http.StatusOK, res.Status)
	assert.Equal(t, true, res.Data()["byok_enabled"], "own keys are on by default")

	res = s.Do(http.MethodPost, "/api/v1/me/ai/providers", member, map[string]string{"kind": "openai", "api_key": memberKey})
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	providerID := res.Data()["id"].(string)
	res = s.Do(http.MethodPost, "/api/v1/me/ai/providers/"+providerID+"/models", member, map[string]string{"name": "gpt-x"})
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	modelID := res.Data()["id"].(string)

	res = s.Do(http.MethodPatch, "/api/v1/admin/settings", member, map[string]any{"byok_enabled": false})
	assert.Equal(t, http.StatusForbidden, res.Status, "only admins switch it")
	res = s.Do(http.MethodPatch, "/api/v1/admin/settings", admin, map[string]any{"byok_enabled": false})
	require.Equal(t, http.StatusOK, res.Status, res.Body)
	assert.Equal(t, false, res.Data()["byok_enabled"])
	res = s.Do(http.MethodGet, "/api/v1/admin/settings", admin, nil)
	assert.Equal(t, false, res.Data()["byok_enabled"])

	// The provider is still listed, flagged as off.
	res = s.Do(http.MethodGet, "/api/v1/me/ai/providers", member, nil)
	require.Equal(t, http.StatusOK, res.Status)
	assert.Equal(t, false, res.Data()["byok_enabled"])
	assert.Len(t, res.Data()["providers"], 1)

	// Everything but listing and deleting is refused.
	for _, c := range []struct {
		method, path string
		body         any
	}{
		{http.MethodPost, "/api/v1/me/ai/providers", map[string]string{"kind": "anthropic", "api_key": "sk-ant-1234"}},
		{http.MethodPatch, "/api/v1/me/ai/providers/" + providerID, map[string]bool{"enabled": false}},
		{http.MethodPost, "/api/v1/me/ai/providers/" + providerID + "/test", nil},
		{http.MethodPost, "/api/v1/me/ai/providers/" + providerID + "/models", map[string]string{"name": "gpt-y"}},
		{http.MethodPatch, "/api/v1/me/ai/models/" + modelID, map[string]bool{"enabled": false}},
	} {
		res = s.Do(c.method, c.path, member, c.body)
		assert.Equal(t, http.StatusUnprocessableEntity, res.Status, "%s %s", c.method, c.path)
		assert.Equal(t, "BYOK_DISABLED", res.ErrorCode(), "%s %s", c.method, c.path)
	}

	res = s.Do(http.MethodGet, "/api/v1/ai/models", member, nil)
	require.Equal(t, http.StatusOK, res.Status)
	assert.Empty(t, res.List(), "own models leave the picker")

	res = s.Do(http.MethodDelete, "/api/v1/me/ai/models/"+modelID, member, nil)
	assert.Equal(t, http.StatusNoContent, res.Status, "users can always remove their models")
	res = s.Do(http.MethodDelete, "/api/v1/me/ai/providers/"+providerID, member, nil)
	assert.Equal(t, http.StatusNoContent, res.Status, "users can always take their key back out")

	// Turning it back on allows connecting again.
	res = s.Do(http.MethodPatch, "/api/v1/admin/settings", admin, map[string]any{"byok_enabled": true})
	require.Equal(t, http.StatusOK, res.Status)
	res = s.Do(http.MethodPost, "/api/v1/me/ai/providers", member, map[string]string{"kind": "openai", "api_key": memberKey})
	assert.Equal(t, http.StatusCreated, res.Status)
}

func TestBYOKProvidersFollowTheUser(t *testing.T) {
	t.Parallel()
	s := byokServer(t)
	admin, _ := setupAdmin(t, s)
	member := inviteNamedMember(t, s, admin, "Ana", "ana@example.com")

	res := s.Do(http.MethodPost, "/api/v1/me/ai/providers", member, map[string]string{"kind": "gemini", "api_key": memberKey})
	require.Equal(t, http.StatusCreated, res.Status, res.Body)

	// Deleting the account cascades to the provider and its key.
	_, err := s.Container.DB.ExecContext(context.Background(), `DELETE FROM users WHERE email = $1`, "ana@example.com")
	require.NoError(t, err)
	var count int
	require.NoError(t, s.Container.DB.QueryRowContext(context.Background(), `SELECT count(*) FROM ai_providers WHERE user_id IS NOT NULL`).Scan(&count))
	assert.Zero(t, count)
}
