package ai

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

// gemini speaks the Google Generative Language API.
type gemini struct {
	cfg Config
}

func (g *gemini) header() http.Header {
	h := http.Header{}
	h.Set("x-goog-api-key", g.cfg.APIKey)
	return h
}

func (g *gemini) url(path string) string {
	return strings.TrimRight(g.cfg.BaseURL, "/") + "/v1beta" + path
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiRequest struct {
	Contents          []geminiContent `json:"contents"`
	SystemInstruction *geminiContent  `json:"systemInstruction,omitempty"`
	GenerationConfig  map[string]any  `json:"generationConfig"`
}

type geminiResponse struct {
	Candidates []struct {
		Content      geminiContent `json:"content"`
		FinishReason string        `json:"finishReason"`
	} `json:"candidates"`
	PromptFeedback struct {
		BlockReason string `json:"blockReason"`
	} `json:"promptFeedback"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
	} `json:"usageMetadata"`
}

// Chat implements Provider.
func (g *gemini) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	body := geminiRequest{GenerationConfig: map[string]any{"maxOutputTokens": maxTokens(req.MaxTokens)}}
	if req.System != "" {
		body.SystemInstruction = &geminiContent{Parts: []geminiPart{{Text: req.System}}}
	}
	for _, m := range req.Messages {
		role := "user"
		if m.Role == RoleAssistant {
			role = "model"
		}
		body.Contents = append(body.Contents, geminiContent{Role: role, Parts: []geminiPart{{Text: m.Content}}})
	}
	if req.Schema != nil {
		body.GenerationConfig["responseMimeType"] = "application/json"
		body.GenerationConfig["responseJsonSchema"] = req.Schema.Schema
	}

	var res geminiResponse
	path := "/models/" + url.PathEscape(req.Model) + ":generateContent"
	if err := doJSON(ctx, g.cfg.HTTPClient, KindGemini, http.MethodPost, g.url(path), g.header(), body, &res); err != nil {
		return ChatResponse{}, err
	}
	if res.PromptFeedback.BlockReason != "" || len(res.Candidates) == 0 {
		return ChatResponse{}, ErrRefused
	}
	c := res.Candidates[0]
	switch c.FinishReason {
	case "SAFETY", "PROHIBITED_CONTENT", "BLOCKLIST", "SPII", "RECITATION":
		return ChatResponse{}, ErrRefused
	}
	var text strings.Builder
	for _, p := range c.Content.Parts {
		text.WriteString(p.Text)
	}
	return ChatResponse{
		Text:      text.String(),
		Truncated: c.FinishReason == "MAX_TOKENS",
		Usage:     Usage{InputTokens: res.UsageMetadata.PromptTokenCount, OutputTokens: res.UsageMetadata.CandidatesTokenCount},
	}, nil
}

// Embed implements Provider.
func (g *gemini) Embed(ctx context.Context, req EmbedRequest) (EmbedResponse, error) {
	model := "models/" + req.Model
	requests := make([]map[string]any, len(req.Inputs))
	for i, text := range req.Inputs {
		requests[i] = map[string]any{
			"model":                model,
			"content":              geminiContent{Parts: []geminiPart{{Text: text}}},
			"outputDimensionality": EmbeddingDimensions,
		}
	}
	var res struct {
		Embeddings []struct {
			Values []float32 `json:"values"`
		} `json:"embeddings"`
	}
	path := "/models/" + url.PathEscape(req.Model) + ":batchEmbedContents"
	if err := doJSON(ctx, g.cfg.HTTPClient, KindGemini, http.MethodPost, g.url(path), g.header(), map[string]any{"requests": requests}, &res); err != nil {
		return EmbedResponse{}, err
	}
	if len(res.Embeddings) != len(req.Inputs) {
		return EmbedResponse{}, &APIError{Provider: KindGemini, Status: http.StatusBadGateway, Message: "embedding count does not match inputs"}
	}
	vectors := make([][]float32, len(res.Embeddings))
	for i, e := range res.Embeddings {
		vectors[i] = e.Values
	}
	if err := checkDimensions(vectors); err != nil {
		return EmbedResponse{}, err
	}
	return EmbedResponse{Vectors: vectors}, nil
}

// ListModels implements Provider.
func (g *gemini) ListModels(ctx context.Context) ([]string, error) {
	var res struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := doJSON(ctx, g.cfg.HTTPClient, KindGemini, http.MethodGet, g.url("/models?pageSize=1000"), g.header(), nil, &res); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(res.Models))
	for _, m := range res.Models {
		ids = append(ids, strings.TrimPrefix(m.Name, "models/"))
	}
	return ids, nil
}
