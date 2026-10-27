package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dev-fritz/newsp/api/internal/models"
	"github.com/dev-fritz/newsp/api/tests/apitest"
)

// fakeOpenAI serves /v1/models, /v1/embeddings and /v1/chat/completions.
// A request with "Authorization: Bearer bad" gets a 401.
func fakeOpenAI(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer bad" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"invalid key"}}`))
			return
		}
		switch r.URL.Path {
		case "/v1/models":
			_, _ = w.Write([]byte(`{"data":[{"id":"qwen3:14b"},{"id":"nomic-embed-text"}]}`))
		case "/v1/embeddings":
			var req struct {
				Input []string `json:"input"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			data := make([]map[string]any, len(req.Input))
			for i := range req.Input {
				data[i] = map[string]any{"index": i, "embedding": make([]float32, 768)}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": data, "usage": map[string]int{"prompt_tokens": 11}})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestAIProviderAdministration(t *testing.T) {
	t.Parallel()
	s := apitest.New(t)
	admin, _ := setupAdmin(t, s)
	fake := fakeOpenAI(t)

	res := s.Do(http.MethodPost, "/api/v1/admin/ai/providers", admin, map[string]string{"name": "Ollama", "kind": "openai_compatible"})
	assert.Equal(t, http.StatusBadRequest, res.Status, "compatible providers need a base URL")

	res = s.Do(http.MethodPost, "/api/v1/admin/ai/providers", admin, map[string]string{
		"name": "Ollama", "kind": "openai_compatible", "base_url": fake.URL + "/v1/",
	})
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	ollamaID := res.Data()["id"].(string)
	assert.Equal(t, fake.URL+"/v1", res.Data()["base_url"], "trailing slash trimmed")
	assert.Equal(t, false, res.Data()["has_api_key"])

	res = s.Do(http.MethodPost, "/api/v1/admin/ai/providers", admin, map[string]string{"name": "ollama", "kind": "openai_compatible", "base_url": fake.URL})
	assert.Equal(t, http.StatusConflict, res.Status, "names are unique, case-insensitively")

	res = s.Do(http.MethodPost, "/api/v1/admin/ai/providers", admin, map[string]string{"name": "Anthropic", "kind": "anthropic", "api_key": "sk-ant-secret-abcd"})
	require.Equal(t, http.StatusCreated, res.Status)
	anthropicID := res.Data()["id"].(string)
	assert.Equal(t, "https://api.anthropic.com", res.Data()["base_url"], "default base URL")
	assert.NotContains(t, res.Body, "api_key")

	var stored []byte
	require.NoError(t, s.Container.DB.QueryRowContext(context.Background(), `SELECT api_key_enc FROM ai_providers WHERE id = $1`, anthropicID).Scan(&stored))
	assert.NotContains(t, string(stored), "sk-ant-secret", "keys are encrypted at rest")

	res = s.Do(http.MethodPost, "/api/v1/admin/ai/providers/"+ollamaID+"/test", admin, nil)
	require.Equal(t, http.StatusOK, res.Status)
	assert.Equal(t, true, res.Data()["ok"])
	assert.Equal(t, []any{"qwen3:14b", "nomic-embed-text"}, res.Data()["models"])

	res = s.Do(http.MethodPost, "/api/v1/admin/ai/providers", admin, map[string]string{"name": "Bad", "kind": "openai_compatible", "base_url": fake.URL + "/v1", "api_key": "bad"})
	badID := res.Data()["id"].(string)
	res = s.Do(http.MethodPost, "/api/v1/admin/ai/providers/"+badID+"/test", admin, nil)
	require.Equal(t, http.StatusOK, res.Status)
	assert.Equal(t, false, res.Data()["ok"])
	assert.Contains(t, res.Data()["error"], "rejected the API key")

	res = s.Do(http.MethodPost, "/api/v1/admin/ai/providers/"+anthropicID+"/models", admin, map[string]string{"name": "x", "capability": "embedding"})
	assert.Equal(t, http.StatusUnprocessableEntity, res.Status)

	res = s.Do(http.MethodPost, "/api/v1/admin/ai/providers/"+ollamaID+"/models", admin, map[string]string{"name": "qwen3:14b", "capability": "chat"})
	require.Equal(t, http.StatusCreated, res.Status)
	chatID := res.Data()["id"].(string)
	res = s.Do(http.MethodPost, "/api/v1/admin/ai/providers/"+ollamaID+"/models", admin, map[string]string{"name": "nomic-embed-text", "display_name": "Nomic", "capability": "embedding"})
	require.Equal(t, http.StatusCreated, res.Status)
	embedID := res.Data()["id"].(string)

	res = s.Do(http.MethodGet, "/api/v1/admin/ai/providers", admin, nil)
	require.Equal(t, http.StatusOK, res.Status)
	byName := map[string]map[string]any{}
	for _, p := range res.List() {
		byName[p["name"].(string)] = p
	}
	assert.Equal(t, "••••abcd", byName["Anthropic"]["key_hint"])
	assert.Len(t, byName["Ollama"]["models"], 2)
	assert.NotNil(t, byName["Bad"]["last_error"])

	// Settings: capability is enforced and a new embedding model starts a reindex.
	res = s.Do(http.MethodPatch, "/api/v1/admin/settings", admin, map[string]string{"embedding_model_id": chatID})
	assert.Equal(t, http.StatusBadRequest, res.Status)
	res = s.Do(http.MethodPatch, "/api/v1/admin/settings", admin, map[string]any{
		"embedding_model_id": embedID, "default_report_model_id": chatID, "retention_days": 30,
	})
	require.Equal(t, http.StatusOK, res.Status, res.Body)
	assert.NotEmpty(t, res.Data()["reindex_job_id"])
	assert.InDelta(t, 30, res.Data()["retention_days"], 0)
	res = s.Do(http.MethodPatch, "/api/v1/admin/settings", admin, map[string]any{"retention_days": 1})
	assert.Equal(t, http.StatusBadRequest, res.Status)

	// The gateway embeds through the configured model and accounts for tokens.
	vectors, model, err := s.Container.AI.Embed(context.Background(), models.PurposeEmbedding, []string{"a", "b", "c"})
	require.NoError(t, err)
	assert.Len(t, vectors, 3)
	assert.Equal(t, "nomic-embed-text", model)
	res = s.Do(http.MethodGet, "/api/v1/admin/ai/providers", admin, nil)
	for _, p := range res.List() {
		if p["name"] != "Ollama" {
			continue
		}
		for _, m := range p["models"].([]any) {
			if m.(map[string]any)["capability"] == "embedding" {
				assert.InDelta(t, 11, m.(map[string]any)["tokens_last_7d"], 0)
			}
		}
	}
}

func TestChatModelsForMembers(t *testing.T) {
	t.Parallel()
	s := apitest.New(t)
	admin, _ := setupAdmin(t, s)
	fake := fakeOpenAI(t)

	res := s.Do(http.MethodPost, "/api/v1/admin/ai/providers", admin, map[string]string{"name": "Local", "kind": "openai_compatible", "base_url": fake.URL + "/v1"})
	providerID := res.Data()["id"].(string)
	res = s.Do(http.MethodPost, "/api/v1/admin/ai/providers/"+providerID+"/models", admin, map[string]string{"name": "qwen3:14b", "display_name": "Qwen 3 14B", "capability": "chat"})
	chatID := res.Data()["id"].(string)
	s.Do(http.MethodPost, "/api/v1/admin/ai/providers/"+providerID+"/models", admin, map[string]string{"name": "nomic-embed-text", "capability": "embedding"})

	res = s.Do(http.MethodPost, "/api/v1/admin/invites", admin, map[string]string{"role": "member"})
	res = s.Do(http.MethodPost, "/api/v1/invites/accept", "", map[string]string{
		"token": tokenFromURL(t, res.Data()["url"].(string)), "name": "Ana", "email": "ana@example.com", "password": strongPassword,
	})
	member, _ := tokens(res)

	res = s.Do(http.MethodGet, "/api/v1/ai/models", member, nil)
	require.Equal(t, http.StatusOK, res.Status)
	require.Len(t, res.List(), 1, "only chat models")
	assert.Equal(t, "Qwen 3 14B", res.List()[0]["display_name"])
	assert.Equal(t, true, res.List()[0]["local"])

	res = s.Do(http.MethodGet, "/api/v1/admin/ai/providers", member, nil)
	assert.Equal(t, http.StatusForbidden, res.Status)
	res = s.Do(http.MethodPatch, "/api/v1/admin/settings", member, map[string]any{"retention_days": 30})
	assert.Equal(t, http.StatusForbidden, res.Status)

	s.Do(http.MethodPatch, "/api/v1/admin/ai/models/"+chatID, admin, map[string]bool{"enabled": false})
	res = s.Do(http.MethodGet, "/api/v1/ai/models", member, nil)
	assert.Empty(t, res.List(), "disabled models disappear")

	res = s.Do(http.MethodDelete, "/api/v1/admin/ai/providers/"+providerID, admin, nil)
	assert.Equal(t, http.StatusNoContent, res.Status)
	res = s.Do(http.MethodGet, "/api/v1/admin/ai/providers", admin, nil)
	assert.Empty(t, res.List())
	assert.False(t, strings.Contains(res.Header.Get("Content-Type"), "html"))
}
