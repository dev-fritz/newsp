package ai

import (
	"context"
	"net/http"
	"strings"
)

// anthropicVersion pins the Messages API version.
const anthropicVersion = "2026-10-21"

// anthropic speaks the Anthropic Messages API.
type anthropic struct {
	cfg Config
}

func (a *anthropic) header() http.Header {
	h := http.Header{}
	h.Set("x-api-key", a.cfg.APIKey)
	h.Set("anthropic-version", anthropicVersion)
	return h
}

func (a *anthropic) url(path string) string {
	return strings.TrimRight(a.cfg.BaseURL, "/") + path
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model        string             `json:"model"`
	MaxTokens    int                `json:"max_tokens"`
	System       string             `json:"system,omitempty"`
	Messages     []anthropicMessage `json:"messages"`
	OutputConfig map[string]any     `json:"output_config,omitempty"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// Chat implements Provider. Structured output uses output_config.format, the
// API's native JSON-schema mode (forced tool use is rejected by current models).
// Sampling parameters are not sent: current models reject them.
func (a *anthropic) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	body := anthropicRequest{Model: req.Model, MaxTokens: maxTokens(req.MaxTokens), System: req.System}
	for _, m := range req.Messages {
		body.Messages = append(body.Messages, anthropicMessage{Role: string(m.Role), Content: m.Content})
	}
	if req.Schema != nil {
		body.OutputConfig = map[string]any{
			"format": map[string]any{"type": "json_schema", "schema": req.Schema.Schema},
		}
	}

	var res anthropicResponse
	if err := doJSON(ctx, a.cfg.HTTPClient, KindAnthropic, http.MethodPost, a.url("/v1/messages"), a.header(), body, &res); err != nil {
		return ChatResponse{}, err
	}
	if res.StopReason == "refusal" {
		return ChatResponse{}, ErrRefused
	}
	// Thinking blocks may precede the answer; only text blocks are the reply.
	var text strings.Builder
	for _, block := range res.Content {
		if block.Type == "text" {
			text.WriteString(block.Text)
		}
	}
	return ChatResponse{
		Text:      text.String(),
		Truncated: res.StopReason == "max_tokens",
		Usage:     Usage{InputTokens: res.Usage.InputTokens, OutputTokens: res.Usage.OutputTokens},
	}, nil
}

// Embed implements Provider. Anthropic has no embeddings endpoint; pick an
// embedding model from another provider for the instance.
func (a *anthropic) Embed(context.Context, EmbedRequest) (EmbedResponse, error) {
	return EmbedResponse{}, ErrUnsupported
}

// ListModels implements Provider.
func (a *anthropic) ListModels(ctx context.Context) ([]string, error) {
	var res struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := doJSON(ctx, a.cfg.HTTPClient, KindAnthropic, http.MethodGet, a.url("/v1/models?limit=1000"), a.header(), nil, &res); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(res.Data))
	for _, m := range res.Data {
		ids = append(ids, m.ID)
	}
	return ids, nil
}
