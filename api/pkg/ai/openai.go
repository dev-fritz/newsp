package ai

import (
	"context"
	"net/http"
	"strings"
)

// openAI speaks the OpenAI chat completions protocol, which Ollama, llama.cpp,
// LM Studio and vLLM also serve.
type openAI struct {
	cfg Config
}

func (o *openAI) header() http.Header {
	h := http.Header{}
	if o.cfg.APIKey != "" {
		h.Set("Authorization", "Bearer "+o.cfg.APIKey)
	}
	return h
}

func (o *openAI) url(path string) string {
	return strings.TrimRight(o.cfg.BaseURL, "/") + path
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatRequest struct {
	Model               string          `json:"model"`
	Messages            []openAIMessage `json:"messages"`
	MaxTokens           int             `json:"max_tokens,omitempty"`
	MaxCompletionTokens int             `json:"max_completion_tokens,omitempty"`
	ResponseFormat      any             `json:"response_format,omitempty"`
}

type openAIChatResponse struct {
	Choices []struct {
		Message struct {
			Content *string `json:"content"`
			Refusal *string `json:"refusal"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// Chat implements Provider.
func (o *openAI) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	body := openAIChatRequest{Model: req.Model}
	if req.System != "" {
		body.Messages = append(body.Messages, openAIMessage{Role: "system", Content: req.System})
	}
	for _, m := range req.Messages {
		body.Messages = append(body.Messages, openAIMessage{Role: string(m.Role), Content: m.Content})
	}
	// OpenAI's own API renamed the limit; compatible servers keep max_tokens.
	if o.cfg.Kind == KindOpenAI {
		body.MaxCompletionTokens = maxTokens(req.MaxTokens)
	} else {
		body.MaxTokens = maxTokens(req.MaxTokens)
	}
	if req.Schema != nil {
		body.ResponseFormat = map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   req.Schema.Name,
				"schema": req.Schema.Schema,
				"strict": true,
			},
		}
	}

	var res openAIChatResponse
	if err := doJSON(ctx, o.cfg.HTTPClient, o.cfg.Kind, http.MethodPost, o.url("/chat/completions"), o.header(), body, &res); err != nil {
		return ChatResponse{}, err
	}
	if len(res.Choices) == 0 {
		return ChatResponse{}, &APIError{Provider: o.cfg.Kind, Status: http.StatusBadGateway, Message: "response has no choices"}
	}
	choice := res.Choices[0]
	if choice.Message.Refusal != nil && *choice.Message.Refusal != "" {
		return ChatResponse{}, ErrRefused
	}
	text := ""
	if choice.Message.Content != nil {
		text = *choice.Message.Content
	}
	return ChatResponse{
		Text:      text,
		Truncated: choice.FinishReason == "length",
		Usage:     Usage{InputTokens: res.Usage.PromptTokens, OutputTokens: res.Usage.CompletionTokens},
	}, nil
}

type openAIEmbedResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
	} `json:"usage"`
}

// Embed implements Provider.
func (o *openAI) Embed(ctx context.Context, req EmbedRequest) (EmbedResponse, error) {
	body := map[string]any{"model": req.Model, "input": req.Inputs}
	if o.cfg.Kind == KindOpenAI {
		// text-embedding-3-* can shorten vectors natively; local models already
		// produce 768 (nomic-embed-text) and some servers reject the field.
		body["dimensions"] = EmbeddingDimensions
	}
	var res openAIEmbedResponse
	if err := doJSON(ctx, o.cfg.HTTPClient, o.cfg.Kind, http.MethodPost, o.url("/embeddings"), o.header(), body, &res); err != nil {
		return EmbedResponse{}, err
	}
	if len(res.Data) != len(req.Inputs) {
		return EmbedResponse{}, &APIError{Provider: o.cfg.Kind, Status: http.StatusBadGateway, Message: "embedding count does not match inputs"}
	}
	vectors := make([][]float32, len(res.Data))
	for _, d := range res.Data {
		if d.Index < 0 || d.Index >= len(vectors) {
			return EmbedResponse{}, &APIError{Provider: o.cfg.Kind, Status: http.StatusBadGateway, Message: "embedding index out of range"}
		}
		vectors[d.Index] = d.Embedding
	}
	if err := checkDimensions(vectors); err != nil {
		return EmbedResponse{}, err
	}
	return EmbedResponse{Vectors: vectors, Usage: Usage{InputTokens: res.Usage.PromptTokens}}, nil
}

// ListModels implements Provider.
func (o *openAI) ListModels(ctx context.Context) ([]string, error) {
	var res struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := doJSON(ctx, o.cfg.HTTPClient, o.cfg.Kind, http.MethodGet, o.url("/models"), o.header(), nil, &res); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(res.Data))
	for _, m := range res.Data {
		ids = append(ids, m.ID)
	}
	return ids, nil
}
