package ai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testSchema = &Schema{Name: "idea", Schema: json.RawMessage(`{"type":"object","properties":{"title":{"type":"string"}},"required":["title"],"additionalProperties":false}`)}

// fakeServer answers one route and records the decoded request body and headers.
func fakeServer(t *testing.T, path string, status int, response string) (*httptest.Server, *map[string]any, *http.Header) {
	t.Helper()
	body := map[string]any{}
	header := http.Header{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			t.Errorf("unexpected path %s, want %s", r.URL.Path, path)
		}
		header = r.Header.Clone()
		raw, _ := io.ReadAll(r.Body)
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &body)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(srv.Close)
	return srv, &body, &header
}

func vector(n int) []float32 { return make([]float32, n) }

func vectorJSON(n int) string {
	raw, _ := json.Marshal(vector(n))
	return string(raw)
}

func TestOpenAICompatibleChat(t *testing.T) {
	t.Parallel()
	srv, body, header := fakeServer(t, "/v1/chat/completions", http.StatusOK,
		`{"choices":[{"message":{"content":"{\"title\":\"x\"}"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5}}`)
	p, err := New(Config{Kind: KindOpenAICompatible, BaseURL: srv.URL + "/v1"})
	require.NoError(t, err)

	res, err := p.Chat(context.Background(), ChatRequest{Model: "qwen3:14b", System: "be brief", Messages: []Message{{Role: RoleUser, Content: "hi"}}, Schema: testSchema})
	require.NoError(t, err)
	assert.JSONEq(t, `{"title":"x"}`, res.Text)
	assert.Equal(t, Usage{InputTokens: 10, OutputTokens: 5}, res.Usage)

	assert.Empty(t, header.Get("Authorization"), "local servers get no key")
	assert.InDelta(t, DefaultMaxTokens, (*body)["max_tokens"], 0)
	assert.NotContains(t, *body, "max_completion_tokens")
	msgs := (*body)["messages"].([]any)
	assert.Equal(t, "system", msgs[0].(map[string]any)["role"])
	rf := (*body)["response_format"].(map[string]any)
	assert.Equal(t, "json_schema", rf["type"])
}

func TestOpenAIEmbedAndKeys(t *testing.T) {
	t.Parallel()
	srv, body, header := fakeServer(t, "/v1/embeddings", http.StatusOK,
		`{"data":[{"index":1,"embedding":`+vectorJSON(768)+`},{"index":0,"embedding":`+vectorJSON(768)+`}],"usage":{"prompt_tokens":4}}`)
	p, err := New(Config{Kind: KindOpenAI, BaseURL: srv.URL + "/v1", APIKey: "sk-test"})
	require.NoError(t, err)

	res, err := p.Embed(context.Background(), EmbedRequest{Model: "text-embedding-3-small", Inputs: []string{"a", "b"}})
	require.NoError(t, err)
	assert.Len(t, res.Vectors, 2)
	assert.Equal(t, "Bearer sk-test", header.Get("Authorization"))
	assert.InDelta(t, 768, (*body)["dimensions"], 0, "OpenAI is asked for 768 dimensions")
}

func TestEmbedRejectsWrongDimensions(t *testing.T) {
	t.Parallel()
	srv, _, _ := fakeServer(t, "/embeddings", http.StatusOK, `{"data":[{"index":0,"embedding":`+vectorJSON(1024)+`}]}`)
	p, _ := New(Config{Kind: KindOpenAICompatible, BaseURL: srv.URL})
	_, err := p.Embed(context.Background(), EmbedRequest{Model: "mxbai", Inputs: []string{"a"}})
	assert.ErrorIs(t, err, ErrDimensions)
	assert.False(t, IsTemporary(err))
}

func TestOpenAIRefusalAndTruncation(t *testing.T) {
	t.Parallel()
	srv, _, _ := fakeServer(t, "/chat/completions", http.StatusOK, `{"choices":[{"message":{"content":null,"refusal":"no"},"finish_reason":"stop"}]}`)
	p, _ := New(Config{Kind: KindOpenAICompatible, BaseURL: srv.URL})
	_, err := p.Chat(context.Background(), ChatRequest{Model: "m"})
	assert.ErrorIs(t, err, ErrRefused)

	srv2, _, _ := fakeServer(t, "/chat/completions", http.StatusOK, `{"choices":[{"message":{"content":"{\"ti"},"finish_reason":"length"}]}`)
	p2, _ := New(Config{Kind: KindOpenAICompatible, BaseURL: srv2.URL})
	res, err := p2.Chat(context.Background(), ChatRequest{Model: "m"})
	require.NoError(t, err)
	assert.True(t, res.Truncated)
}

func TestAnthropicChat(t *testing.T) {
	t.Parallel()
	srv, body, header := fakeServer(t, "/v1/messages", http.StatusOK,
		`{"content":[{"type":"thinking","thinking":""},{"type":"text","text":"{\"title\":\"x\"}"}],"stop_reason":"end_turn","usage":{"input_tokens":12,"output_tokens":7}}`)
	p, err := New(Config{Kind: KindAnthropic, BaseURL: srv.URL, APIKey: "ant-key"})
	require.NoError(t, err)

	res, err := p.Chat(context.Background(), ChatRequest{Model: "claude-opus-5-5", System: "sys", Messages: []Message{{Role: RoleUser, Content: "hi"}}, Schema: testSchema})
	require.NoError(t, err)
	assert.JSONEq(t, `{"title":"x"}`, res.Text, "thinking blocks are skipped")
	assert.Equal(t, Usage{InputTokens: 12, OutputTokens: 7}, res.Usage)

	assert.Equal(t, "ant-key", header.Get("x-api-key"))
	assert.Equal(t, anthropicVersion, header.Get("anthropic-version"))
	assert.Equal(t, "sys", (*body)["system"])
	assert.NotContains(t, *body, "temperature", "current models reject sampling parameters")
	assert.NotContains(t, *body, "tool_choice", "structured output does not force tools")
	format := (*body)["output_config"].(map[string]any)["format"].(map[string]any)
	assert.Equal(t, "json_schema", format["type"])
}

func TestAnthropicRefusalAndErrors(t *testing.T) {
	t.Parallel()
	srv, _, _ := fakeServer(t, "/v1/messages", http.StatusOK, `{"content":[],"stop_reason":"refusal"}`)
	p, _ := New(Config{Kind: KindAnthropic, BaseURL: srv.URL})
	_, err := p.Chat(context.Background(), ChatRequest{Model: "m"})
	assert.ErrorIs(t, err, ErrRefused)

	_, err = p.Embed(context.Background(), EmbedRequest{})
	assert.ErrorIs(t, err, ErrUnsupported)

	overloaded, _, _ := fakeServer(t, "/v1/messages", 529, `{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`)
	p2, _ := New(Config{Kind: KindAnthropic, BaseURL: overloaded.URL})
	_, err = p2.Chat(context.Background(), ChatRequest{Model: "m"})
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, "Overloaded", apiErr.Message)
	assert.True(t, IsTemporary(err), "529 overload is retried")

	bad, _, _ := fakeServer(t, "/v1/messages", http.StatusBadRequest, `{"error":{"message":"bad model"}}`)
	p3, _ := New(Config{Kind: KindAnthropic, BaseURL: bad.URL})
	_, err = p3.Chat(context.Background(), ChatRequest{Model: "m"})
	assert.False(t, IsTemporary(err))
}

func TestGeminiChatAndEmbed(t *testing.T) {
	t.Parallel()
	srv, body, header := fakeServer(t, "/v1beta/models/gemini-flash:generateContent", http.StatusOK,
		`{"candidates":[{"content":{"parts":[{"text":"{\"title\":"},{"text":"\"x\"}"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":3,"candidatesTokenCount":2}}`)
	p, _ := New(Config{Kind: KindGemini, BaseURL: srv.URL, APIKey: "g-key"})
	res, err := p.Chat(context.Background(), ChatRequest{Model: "gemini-flash", System: "sys", Messages: []Message{{Role: RoleUser, Content: "hi"}, {Role: RoleAssistant, Content: "ok"}}, Schema: testSchema})
	require.NoError(t, err)
	assert.JSONEq(t, `{"title":"x"}`, res.Text)
	assert.Equal(t, "g-key", header.Get("x-goog-api-key"))
	contents := (*body)["contents"].([]any)
	assert.Equal(t, "model", contents[1].(map[string]any)["role"])
	gen := (*body)["generationConfig"].(map[string]any)
	assert.Equal(t, "application/json", gen["responseMimeType"])

	emb, ebody, _ := fakeServer(t, "/v1beta/models/text-embedding-004:batchEmbedContents", http.StatusOK,
		`{"embeddings":[{"values":`+vectorJSON(768)+`}]}`)
	pe, _ := New(Config{Kind: KindGemini, BaseURL: emb.URL})
	out, err := pe.Embed(context.Background(), EmbedRequest{Model: "text-embedding-004", Inputs: []string{"a"}})
	require.NoError(t, err)
	assert.Len(t, out.Vectors, 1)
	req := (*ebody)["requests"].([]any)[0].(map[string]any)
	assert.InDelta(t, 768, req["outputDimensionality"], 0)
}

func TestAPIErrorKeyRejected(t *testing.T) {
	t.Parallel()
	geminiInvalidKey := `{"error":{"code":400,"message":"API key not valid. Please pass a valid API key.","status":"INVALID_ARGUMENT","details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"API_KEY_INVALID","domain":"googleapis.com","metadata":{"service":"generativelanguage.googleapis.com"}}]}}`
	otherBadRequest := `{"error":{"code":400,"message":"Invalid JSON payload","status":"INVALID_ARGUMENT","details":[{"@type":"type.googleapis.com/google.rpc.BadRequest"}]}}`

	tests := []struct {
		name       string
		kind       Kind
		path       string
		status     int
		body       string
		wantReason string
		wantReject bool
	}{
		{"gemini invalid key", KindGemini, "/v1beta/models/m:generateContent", http.StatusBadRequest, geminiInvalidKey, "API_KEY_INVALID", true},
		{"gemini other 400", KindGemini, "/v1beta/models/m:generateContent", http.StatusBadRequest, otherBadRequest, "", false},
		{"openai 401", KindOpenAI, "/v1/chat/completions", http.StatusUnauthorized, `{"error":{"message":"invalid key"}}`, "", true},
		{"openai 403", KindOpenAI, "/v1/chat/completions", http.StatusForbidden, `{"error":"forbidden"}`, "", true},
		{"openai plain 400", KindOpenAI, "/v1/chat/completions", http.StatusBadRequest, `{"error":{"message":"bad model"}}`, "", false},
		{"non-json body", KindOpenAI, "/v1/chat/completions", http.StatusBadRequest, `<html>oops</html>`, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv, _, _ := fakeServer(t, tt.path, tt.status, tt.body)
			baseURL := srv.URL
			if tt.kind == KindOpenAI {
				baseURL += "/v1"
			}
			p, err := New(Config{Kind: tt.kind, BaseURL: baseURL, APIKey: "k"})
			require.NoError(t, err)
			_, err = p.Chat(context.Background(), ChatRequest{Model: "m"})
			var apiErr *APIError
			require.True(t, errors.As(err, &apiErr), "got %v", err)
			assert.Equal(t, tt.wantReason, apiErr.Reason)
			assert.Equal(t, tt.wantReject, apiErr.KeyRejected())
			assert.False(t, apiErr.Temporary(), "a refused key or bad request is never retried")
			assert.False(t, IsTemporary(err))
		})
	}

	gemini, _, _ := fakeServer(t, "/v1beta/models/m:generateContent", http.StatusBadRequest, geminiInvalidKey)
	p, _ := New(Config{Kind: KindGemini, BaseURL: gemini.URL, APIKey: "k"})
	_, err := p.Chat(context.Background(), ChatRequest{Model: "m"})
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, "API key not valid. Please pass a valid API key.", apiErr.Message)
}

func TestGeminiSafetyBlock(t *testing.T) {
	t.Parallel()
	srv, _, _ := fakeServer(t, "/v1beta/models/m:generateContent", http.StatusOK, `{"promptFeedback":{"blockReason":"SAFETY"}}`)
	p, _ := New(Config{Kind: KindGemini, BaseURL: srv.URL})
	_, err := p.Chat(context.Background(), ChatRequest{Model: "m"})
	assert.ErrorIs(t, err, ErrRefused)
}

func TestListModels(t *testing.T) {
	t.Parallel()
	srv, _, _ := fakeServer(t, "/v1/models", http.StatusOK, `{"data":[{"id":"qwen3:14b"},{"id":"nomic-embed-text"}]}`)
	p, _ := New(Config{Kind: KindOpenAICompatible, BaseURL: srv.URL + "/v1"})
	ids, err := p.ListModels(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"qwen3:14b", "nomic-embed-text"}, ids)

	gsrv, _, _ := fakeServer(t, "/v1beta/models", http.StatusOK, `{"models":[{"name":"models/gemini-flash"}]}`)
	pg, _ := New(Config{Kind: KindGemini, BaseURL: gsrv.URL})
	ids, err = pg.ListModels(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"gemini-flash"}, ids)
}

func TestNewValidates(t *testing.T) {
	t.Parallel()
	_, err := New(Config{Kind: KindOpenAICompatible})
	assert.Error(t, err, "compatible providers need a base URL")
	_, err = New(Config{Kind: "cohere"})
	assert.Error(t, err)
}

// scripted returns canned chat responses in order and records requests.
type scripted struct {
	responses []ChatResponse
	requests  []ChatRequest
}

func (s *scripted) Chat(_ context.Context, req ChatRequest) (ChatResponse, error) {
	s.requests = append(s.requests, req)
	res := s.responses[0]
	s.responses = s.responses[1:]
	return res, nil
}
func (s *scripted) Embed(context.Context, EmbedRequest) (EmbedResponse, error) {
	return EmbedResponse{}, nil
}
func (s *scripted) ListModels(context.Context) ([]string, error) { return nil, nil }

func TestGenerateJSONRetriesOnceWithFeedback(t *testing.T) {
	t.Parallel()
	p := &scripted{responses: []ChatResponse{
		{Text: "Sure! Here it is: {title: x}", Usage: Usage{InputTokens: 5, OutputTokens: 5}},
		{Text: "```json\n{\"title\":\"Fixed\"}\n```", Usage: Usage{InputTokens: 9, OutputTokens: 3}},
	}}
	var out struct {
		Title string `json:"title"`
	}
	usage, err := GenerateJSON(context.Background(), p, ChatRequest{Model: "m", Schema: testSchema,
		Messages: []Message{{Role: RoleUser, Content: "give me an idea"}}}, &out, nil)
	require.NoError(t, err)
	assert.Equal(t, "Fixed", out.Title)
	assert.Equal(t, Usage{InputTokens: 14, OutputTokens: 8}, usage, "usage of both attempts is counted")
	require.Len(t, p.requests, 2)
	retry := p.requests[1].Messages
	require.Len(t, retry, 3)
	assert.Contains(t, retry[2].Content, "invalid JSON")
}

func TestGenerateJSONRunsValidation(t *testing.T) {
	t.Parallel()
	p := &scripted{responses: []ChatResponse{{Text: `{"title":""}`}, {Text: `{"title":""}`}}}
	var out struct {
		Title string `json:"title"`
	}
	_, err := GenerateJSON(context.Background(), p, ChatRequest{Model: "m", Schema: testSchema}, &out, func() error {
		if out.Title == "" {
			return errors.New("title must not be empty")
		}
		return nil
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "title must not be empty")
	assert.Len(t, p.requests, 2, "one retry, then give up")
}
