package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dev-fritz/newsp/api/internal/bootstrap"
	"github.com/dev-fritz/newsp/api/pkg/ai"
	"github.com/dev-fritz/newsp/api/pkg/safehttp"
	"github.com/dev-fritz/newsp/api/tests/apitest"
)

// fakeAI is a deterministic provider. Embeddings are hashed bags of words, so
// texts sharing vocabulary are close, like with a real model. The next idea it
// proposes is set by the test.
type fakeAI struct {
	mu       sync.Mutex
	nextIdea map[string]any
	calls    map[string]int
	// sameRevision makes idea_revision return the idea text untouched.
	sameRevision bool
	// judgeDistinct makes the dedup judge call every candidate a new idea.
	judgeDistinct bool
	// lastIdeasPrompt is the last user prompt received by the project_ideas schema.
	lastIdeasPrompt string
}

func (f *fakeAI) factory(ai.Config) (ai.Provider, error) { return f, nil }

func (f *fakeAI) propose(idea map[string]any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextIdea = idea
}

func (f *fakeAI) ideasPrompt() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastIdeasPrompt
}

func (f *fakeAI) setSameRevision(on bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sameRevision = on
}

func (f *fakeAI) setJudgeDistinct(on bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.judgeDistinct = on
}

func (f *fakeAI) Chat(_ context.Context, req ai.ChatRequest) (ai.ChatResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.calls == nil {
		f.calls = map[string]int{}
	}
	name := req.Schema.Name
	f.calls[name]++
	var out any
	switch name {
	case "market_report":
		out = map[string]any{
			"title": "Agentes de código entram no fluxo de revisão", "summary": "Times usam agentes como primeiro revisor [1].",
			"what_changed": []string{},
			"sections": []map[string]string{
				{"heading": "Revisão por agentes", "body": "Relatos de uso diário [1][2]."},
				{"heading": "Sinais para quem constrói", "body": "Falta medir custo por feature [3]."},
			},
		}
	case "project_ideas":
		for _, m := range req.Messages {
			if m.Role == ai.RoleUser {
				f.lastIdeasPrompt = m.Content
			}
		}
		out = map[string]any{"ideas": []any{f.nextIdea}}
	case "same_idea":
		out = map[string]any{"match": 1, "reason": "same problem and audience"}
		if f.judgeDistinct {
			out = map[string]any{"match": 0, "reason": "different audience"}
		}
	case "idea_revision":
		revised := map[string]any{}
		for k, v := range f.nextIdea {
			revised[k] = v
		}
		if !f.sameRevision {
			revised["why_now"] = fmt.Sprint(f.nextIdea["why_now"]) + " Novos relatos reforçam o ruído [2]."
		}
		revised["evidence"] = []int{2}
		out = map[string]any{"idea": revised, "change_note": "Incorporated new evidence about noise."}
	default:
		return ai.ChatResponse{}, fmt.Errorf("unexpected schema %q", name)
	}
	raw, _ := json.Marshal(out)
	return ai.ChatResponse{Text: string(raw), Usage: ai.Usage{InputTokens: 100, OutputTokens: 50}}, nil
}

func (f *fakeAI) Embed(_ context.Context, req ai.EmbedRequest) (ai.EmbedResponse, error) {
	vectors := make([][]float32, len(req.Inputs))
	for i, text := range req.Inputs {
		v := make([]float32, ai.EmbeddingDimensions)
		for _, w := range strings.Fields(strings.ToLower(text)) {
			h := fnv.New32a()
			_, _ = h.Write([]byte(strings.Trim(w, ".,:;[]()")))
			v[h.Sum32()%ai.EmbeddingDimensions]++
		}
		var norm float64
		for _, x := range v {
			norm += float64(x * x)
		}
		for j := range v {
			v[j] /= float32(math.Sqrt(norm) + 1e-9)
		}
		vectors[i] = v
	}
	return ai.EmbedResponse{Vectors: vectors, Usage: ai.Usage{InputTokens: 10}}, nil
}

func (f *fakeAI) ListModels(context.Context) ([]string, error) {
	return []string{"fake-chat", "fake-embed"}, nil
}

func idea(title, summary, why, how string) map[string]any {
	return map[string]any{
		"title": title, "summary": summary, "why_now": why, "how": how,
		"audience": []map[string]string{
			{"kind": "primary", "label": "Tech leads", "description": "Times de 5 a 30 devs"},
			{"kind": "out_of_scope", "label": "Grandes empresas", "description": "Já têm plataforma"},
		},
		"differentiator": "Aprende com o time", "stack": []string{"Go", "PostgreSQL"},
		"architecture_steps": []string{"Webhook", "Fila", "Revisor"},
		"mvp_plan": []map[string]string{
			{"period": "Semanas 1-2", "title": "Ingestão", "description": "GitHub App"},
			{"period": "Semanas 3-4", "title": "Regras", "description": "Regras aprovadas"},
		},
		"monetization": "Assinatura por dev", "competitors": []map[string]string{{"name": "Revisores genéricos", "strengths": "Amplos", "gap": "Ruído"}},
		"difficulty": "intermediate", "effort_weeks": 6, "evidence": []int{1},
	}
}

var (
	reviewIdea = idea("Revisor de PR que aprende o padrão do time",
		"Agente de revisão de pull requests que aprende as convenções do time.",
		"Times usam agentes de revisão de pull requests mas reclamam de ruído [1].",
		"GitHub App que indexa revisões antigas do time e aplica regras aprendidas em cada pull request.")
	reviewTwin = idea("Revisor de pull requests treinado nas convenções do time",
		"Agente de revisão de pull requests que aprende as convenções e regras do time.",
		"Times usam agentes de revisão de pull requests e reclamam de ruído nas revisões [1].",
		"GitHub App que indexa revisões antigas do time e aplica regras aprendidas a cada pull request novo.")
	costIdea = idea("Painel de custo por funcionalidade para apps com LLM",
		"SDK que mede quanto cada funcionalidade gasta com chamadas de modelos.",
		"Startups pagam APIs de modelos sem saber onde o dinheiro vai [3].",
		"SDK leve marca cada chamada com a funcionalidade e um painel agrega custo por usuário.")
)

// rssFeed serves fresh items so they fall inside every window.
func rssFeed(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		now := time.Now().UTC()
		var b strings.Builder
		b.WriteString(`<?xml version="1.0"?><rss version="2.0"><channel><title>Tech</title>`)
		titles := []string{"Ask HN: who uses AI code review?", "Our rules for AI reviewers", "What LLM features really cost"}
		for i, title := range titles {
			fmt.Fprintf(&b, `<item><title>%s</title><link>https://news.example.com/%d</link><guid>g%d</guid><description>%s details</description><pubDate>%s</pubDate></item>`,
				title, i, i, title, now.Add(-time.Duration(i+1)*time.Hour).Format(time.RFC1123Z))
		}
		b.WriteString(`</channel></rss>`)
		_, _ = w.Write([]byte(b.String()))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func drain(t *testing.T, s *apitest.Server) {
	t.Helper()
	runner, _ := bootstrap.NewWorker(s.Container)
	for range 50 {
		worked, err := runner.RunOnce(context.Background(), "test", runner.Kinds())
		require.NoError(t, err)
		if !worked {
			return
		}
	}
	t.Fatal("jobs did not drain")
}

func TestGenerationJourney(t *testing.T) {
	t.Parallel()
	fake := &fakeAI{}
	s := apitest.NewWithOptions(t, bootstrap.Options{
		AIFactory:  fake.factory,
		HTTPClient: safehttp.NewClient(safehttp.Options{AllowPrivate: true}),
	})
	admin, _ := setupAdmin(t, s)
	feed := rssFeed(t)

	// Instance setup: provider, models, defaults, one source.
	res := s.Do(http.MethodPost, "/api/v1/admin/ai/providers", admin, map[string]string{"name": "Local", "kind": "openai_compatible", "base_url": "http://ollama:11434/v1"})
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	providerID := res.Data()["id"].(string)
	res = s.Do(http.MethodPost, "/api/v1/admin/ai/providers/"+providerID+"/models", admin, map[string]string{"name": "fake-chat", "capability": "chat"})
	chatID := res.Data()["id"].(string)
	res = s.Do(http.MethodPost, "/api/v1/admin/ai/providers/"+providerID+"/models", admin, map[string]string{"name": "fake-embed", "capability": "embedding"})
	embedID := res.Data()["id"].(string)
	res = s.Do(http.MethodPatch, "/api/v1/admin/settings", admin, map[string]any{
		"embedding_model_id": embedID, "default_report_model_id": chatID, "default_idea_model_id": chatID,
	})
	require.Equal(t, http.StatusOK, res.Status, res.Body)
	res = s.Do(http.MethodPost, "/api/v1/admin/sources", admin, map[string]any{"kind": "rss", "name": "Tech", "config": map[string]string{"url": feed.URL}})
	require.Equal(t, http.StatusCreated, res.Status, res.Body)
	sourceID := res.Data()["id"].(string)
	res = s.Do(http.MethodPost, "/api/v1/admin/sources/"+sourceID+"/collect", admin, nil)
	require.Equal(t, http.StatusAccepted, res.Status)

	// Preferences: topics get embedded right away.
	res = s.Do(http.MethodPut, "/api/v1/me/preferences", admin, map[string]any{
		"topics": []string{"revisão de código", "custo de LLM"}, "output_lang": "pt-BR", "timezone": "America/Sao_Paulo",
		"report_enabled": true, "report_time": "08:00", "ideas_per_day": 2, "theme": "dark", "excluded_source_ids": []string{}, "ui_locale": "pt-BR",
	})
	require.Equal(t, http.StatusOK, res.Status, res.Body)
	assert.Equal(t, true, res.Data()["topics_ranked"])
	assert.NotNil(t, res.Data()["next_report_at"])
	drain(t, s) // collection, reindex and item embeddings

	res = s.Do(http.MethodGet, "/api/v1/items", admin, nil)
	require.Len(t, res.List(), 3)

	// Report.
	res = s.Do(http.MethodPost, "/api/v1/reports", admin, map[string]any{"window_hours": 72})
	require.Equal(t, http.StatusAccepted, res.Status, res.Body)
	reportID := res.Data()["report"].(map[string]any)["id"].(string)
	res = s.Do(http.MethodPost, "/api/v1/reports", admin, map[string]any{"window_hours": 72})
	assert.Equal(t, http.StatusConflict, res.Status, "one report at a time")
	drain(t, s)
	res = s.Do(http.MethodGet, "/api/v1/reports/"+reportID, admin, nil)
	require.Equal(t, http.StatusOK, res.Status)
	require.Equal(t, "done", res.Data()["status"], res.Body)
	assert.Contains(t, res.Data()["content_md"], "## 1. Revisão por agentes")
	assert.Len(t, res.Data()["citations"], 3)
	for _, c := range res.Data()["citations"].([]any) {
		assert.IsType(t, "", c.(map[string]any)["excerpt"], "citations carry a preview for the hover card")
	}

	listIdeas := func() []map[string]any {
		r := s.Do(http.MethodGet, "/api/v1/ideas?status=all", admin, nil)
		require.Equal(t, http.StatusOK, r.Status)
		return r.List()
	}
	generate := func(proposal map[string]any) {
		fake.propose(proposal)
		r := s.Do(http.MethodPost, "/api/v1/ideas/generate", admin, map[string]any{"count": 1})
		require.Equal(t, http.StatusAccepted, r.Status, r.Body)
		drain(t, s)
	}

	// First idea: created as v1.
	generate(reviewIdea)
	ideas := listIdeas()
	require.Len(t, ideas, 1)
	ideaID := ideas[0]["id"].(string)
	assert.InDelta(t, 1, ideas[0]["current_version"], 0)

	// A near-identical proposal becomes v2 of the same idea, never a duplicate.
	generate(reviewTwin)
	ideas = listIdeas()
	require.Len(t, ideas, 1, "the twin was merged, not duplicated")
	assert.Equal(t, "merged", ideas[0]["last_origin"])
	assert.InDelta(t, 2, ideas[0]["current_version"], 0)
	assert.InDelta(t, 1, ideas[0]["merged_candidates"], 0)

	res = s.Do(http.MethodGet, "/api/v1/ideas/"+ideaID, admin, nil)
	require.Equal(t, http.StatusOK, res.Status)
	current := res.Data()["current"].(map[string]any)
	assert.Equal(t, "Incorporated new evidence about noise.", current["change_note"])
	assert.NotNil(t, current["similarity"])
	assert.Len(t, current["evidence"], 2, "evidence accumulates across versions")
	for _, e := range current["evidence"].([]any) {
		assert.IsType(t, "", e.(map[string]any)["excerpt"], "evidence carries a preview for the hover card")
	}
	assert.Contains(t, current["content"].(map[string]any)["why_now"], "[E2]", "new citations get stable labels")
	assert.Len(t, res.Data()["versions"], 2)

	// Discarded ideas still block duplicates and are not resurrected.
	res = s.Do(http.MethodPatch, "/api/v1/ideas/"+ideaID+"/status", admin, map[string]string{"status": "discarded"})
	require.Equal(t, http.StatusNoContent, res.Status)
	generate(reviewTwin)
	ideas = listIdeas()
	require.Len(t, ideas, 1)
	assert.Equal(t, "discarded", ideas[0]["status"])
	assert.InDelta(t, 3, ideas[0]["current_version"], 0)
	res = s.Do(http.MethodGet, "/api/v1/ideas", admin, nil)
	assert.Empty(t, res.List(), "the default list hides discarded ideas")

	// A different idea is created.
	generate(costIdea)
	require.Len(t, listIdeas(), 2)

	// The judge has the last word: a close proposal it calls distinct is stored
	// as a new idea instead of colliding with the one it was compared to.
	fake.setJudgeDistinct(true)
	fake.propose(reviewTwin)
	res = s.Do(http.MethodPost, "/api/v1/ideas/generate", admin, map[string]any{"count": 1})
	require.Equal(t, http.StatusAccepted, res.Status, res.Body)
	distinctJobID := res.Data()["job_id"].(string)
	drain(t, s)
	fake.setJudgeDistinct(false)
	res = s.Do(http.MethodGet, "/api/v1/jobs/"+distinctJobID, admin, nil)
	require.Equal(t, http.StatusOK, res.Status, res.Body)
	assert.Equal(t, "done", res.Data()["status"], res.Body)
	assert.Equal(t, "done", res.Data()["result"].(map[string]any)["status"], res.Body)
	require.Len(t, listIdeas(), 3, "the idea the judge called distinct was created")

	// A deepen that returns the same text creates no version and reports NO_CHANGES.
	res = s.Do(http.MethodGet, "/api/v1/ideas/"+ideaID, admin, nil)
	require.Equal(t, http.StatusOK, res.Status)
	versionBefore := res.Data()["current_version"]
	versionsBefore := len(res.Data()["versions"].([]any))
	fake.mu.Lock()
	revisionsBefore := fake.calls["idea_revision"]
	fake.mu.Unlock()

	// The fake echoes nextIdea, so point it at the stored text for an identical revision.
	stored := res.Data()["current"].(map[string]any)
	rawContent, err := json.Marshal(stored["content"])
	require.NoError(t, err)
	var stable map[string]any
	require.NoError(t, json.Unmarshal(regexp.MustCompile(`\s*\[E\d+\]`).ReplaceAll(rawContent, nil), &stable))
	stable["title"], stable["summary"] = stored["title"], stored["summary"]
	stable["difficulty"], stable["effort_weeks"] = stored["difficulty"], stored["effort_weeks"]
	stable["evidence"] = []int{1}
	fake.propose(stable)
	fake.setSameRevision(true)
	res = s.Do(http.MethodPost, "/api/v1/ideas/"+ideaID+"/deepen", admin, map[string]any{"aspects": []string{"architecture"}})
	require.Equal(t, http.StatusAccepted, res.Status, res.Body)
	unchangedJobID := res.Data()["job_id"].(string)
	drain(t, s)
	fake.setSameRevision(false)
	fake.propose(costIdea)

	res = s.Do(http.MethodGet, "/api/v1/jobs/"+unchangedJobID, admin, nil)
	require.Equal(t, http.StatusOK, res.Status, res.Body)
	assert.Equal(t, "done", res.Data()["status"])
	result, ok := res.Data()["result"].(map[string]any)
	require.True(t, ok, res.Body)
	assert.Equal(t, "failed", result["status"])
	assert.Equal(t, "NO_CHANGES", result["error"])
	assert.Empty(t, result["created"])
	assert.Empty(t, result["merged"])

	res = s.Do(http.MethodGet, "/api/v1/ideas/"+ideaID, admin, nil)
	require.Equal(t, http.StatusOK, res.Status)
	assert.Equal(t, versionBefore, res.Data()["current_version"], "no version without a change")
	assert.Len(t, res.Data()["versions"], versionsBefore)
	fake.mu.Lock()
	assert.Equal(t, revisionsBefore+2, fake.calls["idea_revision"], "the attempt and the retry")
	fake.mu.Unlock()

	// Deepen and manual edit add versions.
	res = s.Do(http.MethodPost, "/api/v1/ideas/"+ideaID+"/deepen", admin, map[string]any{"aspects": []string{"architecture"}})
	require.Equal(t, http.StatusAccepted, res.Status, res.Body)
	drain(t, s)
	res = s.Do(http.MethodGet, "/api/v1/ideas/"+ideaID, admin, nil)
	assert.Equal(t, "deepened", res.Data()["current"].(map[string]any)["origin"])

	content := res.Data()["current"].(map[string]any)["content"]
	res = s.Do(http.MethodPut, "/api/v1/ideas/"+ideaID, admin, map[string]any{
		"title": "Revisor de PR do time (editado)", "summary": "Resumo editado.", "difficulty": "advanced", "effort_weeks": 8, "content": content,
	})
	require.Equal(t, http.StatusOK, res.Status, res.Body)
	assert.Equal(t, "manual", res.Data()["current"].(map[string]any)["origin"])
	assert.InDelta(t, 5, res.Data()["current_version"], 0)

	res = s.Do(http.MethodGet, "/api/v1/ideas/"+ideaID+"/versions/1", admin, nil)
	require.Equal(t, http.StatusOK, res.Status)
	assert.Equal(t, "generated", res.Data()["origin"])

	fake.mu.Lock()
	assert.Equal(t, 3, fake.calls["same_idea"], "the judge ran for both twins and the distinct one, not for the different idea")
	assert.Equal(t, 5, fake.calls["idea_revision"], "two merges, two unchanged deepen attempts and one deepen")
	fake.mu.Unlock()

	// Another user sees none of it.
	res = s.Do(http.MethodPost, "/api/v1/admin/invites", admin, map[string]string{"role": "member"})
	res = s.Do(http.MethodPost, "/api/v1/invites/accept", "", map[string]string{
		"token": tokenFromURL(t, res.Data()["url"].(string)), "name": "Ana", "email": "ana@example.com", "password": strongPassword,
	})
	member, _ := tokens(res)
	for _, path := range []string{"/api/v1/ideas/" + ideaID, "/api/v1/reports/" + reportID, "/api/v1/ideas/" + ideaID + "/versions/1"} {
		assert.Equal(t, http.StatusNotFound, s.Do(http.MethodGet, path, member, nil).Status, path)
	}
	assert.Empty(t, s.Do(http.MethodGet, "/api/v1/ideas", member, nil).List())
	assert.Equal(t, http.StatusNotFound, s.Do(http.MethodPost, "/api/v1/ideas/"+ideaID+"/deepen", member, map[string]any{}).Status)

	// Ideas from a report: the sources start with the items the report cited.
	res = s.Do(http.MethodGet, "/api/v1/reports/"+reportID, admin, nil)
	require.Equal(t, http.StatusOK, res.Status)
	citations, ok := res.Data()["citations"].([]any)
	require.True(t, ok, res.Body)
	require.NotEmpty(t, citations)
	fake.propose(costIdea)
	res = s.Do(http.MethodPost, "/api/v1/ideas/generate", admin, map[string]any{"count": 1, "report_id": reportID})
	require.Equal(t, http.StatusAccepted, res.Status, res.Body)
	fromReportJobID := res.Data()["job_id"].(string)
	drain(t, s)
	res = s.Do(http.MethodGet, "/api/v1/jobs/"+fromReportJobID, admin, nil)
	require.Equal(t, http.StatusOK, res.Status, res.Body)
	assert.Equal(t, "done", res.Data()["status"])
	result, ok = res.Data()["result"].(map[string]any)
	require.True(t, ok, res.Body)
	assert.Equal(t, "done", result["status"], res.Body)

	prompt := fake.ideasPrompt()
	previous := -1
	for i, c := range citations {
		title := c.(map[string]any)["title"].(string)
		at := strings.Index(prompt, fmt.Sprintf("\n[%d] %s\n", i+1, title))
		assert.Greater(t, at, previous, "cited item %d is source [%d], in the report's order", i+1, i+1)
		previous = at
	}

	// Only the owner can use a finished report, and the id must be a uuid.
	res = s.Do(http.MethodPost, "/api/v1/ideas/generate", member, map[string]any{"count": 1, "report_id": reportID})
	assert.Equal(t, http.StatusNotFound, res.Status, res.Body)
	assert.Equal(t, "REPORT_NOT_FOUND", res.ErrorCode())
	res = s.Do(http.MethodPost, "/api/v1/ideas/generate", admin, map[string]any{"count": 1, "report_id": "nao-e-uuid"})
	assert.Equal(t, http.StatusBadRequest, res.Status, res.Body)
}
