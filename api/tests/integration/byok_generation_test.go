package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dev-fritz/newsp/api/internal/bootstrap"
	"github.com/dev-fritz/newsp/api/internal/services"
	"github.com/dev-fritz/newsp/api/pkg/ai"
	"github.com/dev-fritz/newsp/api/pkg/safehttp"
	"github.com/dev-fritz/newsp/api/tests/apitest"
)

const instanceKey = "instance-key"

// keyedOpenAI is an OpenAI-compatible fake that counts chat calls per bearer
// key. The key "bad" is rejected with a 401; any other key gets a valid report.
type keyedOpenAI struct {
	*httptest.Server
	mu    sync.Mutex
	chats map[string]int
}

func (f *keyedOpenAI) chatCalls(key string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.chats["Bearer "+key]
}

func newKeyedOpenAI(t *testing.T) *keyedOpenAI {
	t.Helper()
	f := &keyedOpenAI{chats: map[string]int{}}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if r.URL.Path == "/v1/chat/completions" {
			f.mu.Lock()
			f.chats[auth]++
			f.mu.Unlock()
		}
		if auth == "Bearer bad" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"invalid key"}}`))
			return
		}
		switch r.URL.Path {
		case "/v1/embeddings":
			var req struct {
				Input []string `json:"input"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			data := make([]map[string]any, len(req.Input))
			for i := range req.Input {
				vec := make([]float32, ai.EmbeddingDimensions)
				vec[0] = 1
				data[i] = map[string]any{"index": i, "embedding": vec}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
		case "/v1/chat/completions":
			report, _ := json.Marshal(map[string]any{
				"title": "Instance report", "summary": "Written by the instance model [1].", "what_changed": []string{},
				"sections": []map[string]string{{"heading": "Signals", "body": "Something happened [1]."}},
			})
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{{"message": map[string]any{"content": string(report)}, "finish_reason": "stop"}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.Close)
	return f
}

// TestBYOKRejectedKeyFailsReportWithoutFallback proves the product rule end to
// end: a user's own provider rejecting the key fails the report with
// AI_KEY_REJECTED, the job is not retried, and the instance model is never
// used as a silent fallback.
func TestBYOKRejectedKeyFailsReportWithoutFallback(t *testing.T) {
	t.Parallel()
	fake := newKeyedOpenAI(t)
	factory := services.ProviderFactory(func(cfg ai.Config) (ai.Provider, error) {
		cfg.BaseURL = fake.URL + "/v1"
		return ai.New(cfg)
	})
	s := apitest.NewWithOptions(t, bootstrap.Options{
		AIFactory:  factory,
		HTTPClient: safehttp.NewClient(safehttp.Options{AllowPrivate: true}),
	})
	admin, _ := setupAdmin(t, s)
	feed := rssFeed(t)

	// Instance setup: a working chat model as the default, an embedding model, one source.
	res := s.Do(http.MethodPost, "/api/v1/admin/ai/providers", admin, map[string]string{
		"name": "Instance", "kind": "openai_compatible", "base_url": fake.URL + "/v1", "api_key": instanceKey,
	})
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	providerID := res.Data()["id"].(string)
	res = s.Do(http.MethodPost, "/api/v1/admin/ai/providers/"+providerID+"/models", admin, map[string]string{"name": "instance-chat", "capability": "chat"})
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	instanceChatID := res.Data()["id"].(string)
	res = s.Do(http.MethodPost, "/api/v1/admin/ai/providers/"+providerID+"/models", admin, map[string]string{"name": "instance-embed", "capability": "embedding"})
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	embedID := res.Data()["id"].(string)
	res = s.Do(http.MethodPatch, "/api/v1/admin/settings", admin, map[string]any{
		"embedding_model_id": embedID, "default_report_model_id": instanceChatID, "default_idea_model_id": instanceChatID,
	})
	require.Equal(t, http.StatusOK, res.Status, res.Body)
	res = s.Do(http.MethodPost, "/api/v1/admin/sources", admin, map[string]any{"kind": "rss", "name": "Tech", "config": map[string]string{"url": feed.URL}})
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	res = s.Do(http.MethodPost, "/api/v1/admin/sources/"+res.Data()["id"].(string)+"/collect", admin, nil)
	require.Equal(t, http.StatusAccepted, res.Status, res.Body)
	runJobs(t, s) // collection and item embeddings

	// The member connects their own provider with a key the fake rejects.
	member := inviteNamedMember(t, s, admin, "Ana", "ana@example.com")
	res = s.Do(http.MethodPost, "/api/v1/me/ai/providers", member, map[string]string{"kind": "openai", "api_key": "bad"})
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	ownProviderID := res.Data()["id"].(string)
	res = s.Do(http.MethodPost, "/api/v1/me/ai/providers/"+ownProviderID+"/models", member, map[string]string{"name": "own-chat"})
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	ownModelID := res.Data()["id"].(string)
	res = s.Do(http.MethodPut, "/api/v1/me/preferences", member, preferencesBody(ownModelID))
	require.Equal(t, http.StatusOK, res.Status, res.Body)
	runJobs(t, s)

	require.Zero(t, fake.chatCalls(instanceKey), "no chat call before the report")

	res = s.Do(http.MethodPost, "/api/v1/reports", member, map[string]any{"window_hours": 72})
	require.Equal(t, http.StatusAccepted, res.Status, res.Body)
	reportID := res.Data()["report"].(map[string]any)["id"].(string)
	jobID := res.Data()["job_id"].(string)
	runJobs(t, s)

	res = s.Do(http.MethodGet, "/api/v1/reports/"+reportID, member, nil)
	require.Equal(t, http.StatusOK, res.Status, res.Body)
	assert.Equal(t, "failed", res.Data()["status"], res.Body)
	assert.Equal(t, "AI_KEY_REJECTED", res.Data()["error"], res.Body)

	res = s.Do(http.MethodGet, "/api/v1/jobs/"+jobID, member, nil)
	require.Equal(t, http.StatusOK, res.Status, res.Body)
	assert.Equal(t, "done", res.Data()["status"], "the report failed, the job itself finished")
	assert.InDelta(t, 1, res.Data()["attempts"], 0, "a rejected key is not retried")

	assert.Equal(t, 1, fake.chatCalls("bad"), "the rejected key was tried exactly once")
	assert.Zero(t, fake.chatCalls(instanceKey), "never silently falls back to an instance model")
	var instanceUsage int
	require.NoError(t, s.Container.DB.QueryRowContext(context.Background(), `SELECT count(*) FROM ai_usage WHERE model_id = $1`, instanceChatID).Scan(&instanceUsage))
	assert.Zero(t, instanceUsage, "no usage is recorded for the instance chat model")
}
