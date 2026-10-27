// Package ai is a provider-agnostic client for chat and embedding models.
//
// Three wire protocols cover every supported provider: OpenAI-compatible
// (OpenAI, Ollama, llama.cpp, LM Studio, vLLM), Anthropic and Gemini. Clients
// are hand-written over net/http so they share one small surface and stay easy
// to test.
package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// EmbeddingDimensions is fixed for the whole instance: the database columns are
// vector(768), and every supported embedding model can produce 768 dimensions.
const EmbeddingDimensions = 768

// Kind is the wire protocol of a provider.
type Kind string

// Supported provider kinds.
const (
	KindOpenAICompatible Kind = "openai_compatible"
	KindOpenAI           Kind = "openai"
	KindAnthropic        Kind = "anthropic"
	KindGemini           Kind = "gemini"
)

// Role of a chat message.
type Role string

// Chat roles. The system prompt goes in ChatRequest.System.
const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message is one chat turn.
type Message struct {
	Role    Role
	Content string
}

// Schema asks the model for JSON matching a JSON Schema. Keep schemas within
// the subset every provider accepts: objects with additionalProperties false
// and every property required, no numeric or length constraints.
type Schema struct {
	Name   string
	Schema json.RawMessage
}

// ChatRequest is a provider-neutral chat completion request.
type ChatRequest struct {
	Model     string
	System    string
	Messages  []Message
	MaxTokens int     // 0 means DefaultMaxTokens
	Schema    *Schema // nil for free text
}

// DefaultMaxTokens leaves room for long reports and ideas.
const DefaultMaxTokens = 16000

// Usage counts tokens of one call.
type Usage struct {
	InputTokens  int
	OutputTokens int
}

// ChatResponse is the model's answer.
type ChatResponse struct {
	Text  string
	Usage Usage
	// Truncated is true when the model stopped at the token limit.
	Truncated bool
}

// EmbedRequest embeds a batch of texts.
type EmbedRequest struct {
	Model  string
	Inputs []string
}

// EmbedResponse holds one vector per input, in order.
type EmbedResponse struct {
	Vectors [][]float32
	Usage   Usage
}

// Provider is the surface every protocol implements.
type Provider interface {
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
	Embed(ctx context.Context, req EmbedRequest) (EmbedResponse, error)
	ListModels(ctx context.Context) ([]string, error)
}

// Errors that are never worth retrying.
var (
	ErrRefused     = errors.New("the model declined to answer")
	ErrUnsupported = errors.New("not supported by this provider")
	ErrDimensions  = fmt.Errorf("embedding model must return %d dimensions", EmbeddingDimensions)
)

// APIError is a non-2xx answer from a provider.
type APIError struct {
	Provider Kind
	Status   int
	Message  string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s: HTTP %d: %s", e.Provider, e.Status, e.Message)
}

// Temporary reports whether retrying later may succeed: rate limits, timeouts,
// overload (Anthropic's 529) and server errors.
func (e *APIError) Temporary() bool {
	return e.Status == http.StatusTooManyRequests || e.Status == http.StatusRequestTimeout || e.Status >= 500
}

// IsTemporary reports whether err is a transient provider or network failure.
func IsTemporary(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Temporary()
	}
	return !errors.Is(err, ErrRefused) && !errors.Is(err, ErrUnsupported) && !errors.Is(err, ErrDimensions)
}

// Config builds a provider client.
type Config struct {
	Kind    Kind
	BaseURL string // empty uses the provider default (required for openai_compatible)
	APIKey  string
	// HTTPClient defaults to a client with a generous timeout: local models
	// on modest hardware can take minutes to write a report.
	HTTPClient *http.Client
}

// New returns the client for cfg.Kind.
func New(cfg Config) (Provider, error) {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 10 * time.Minute}
	}
	switch cfg.Kind {
	case KindOpenAICompatible:
		if cfg.BaseURL == "" {
			return nil, errors.New("base URL is required for OpenAI-compatible providers")
		}
		return &openAI{cfg: cfg}, nil
	case KindOpenAI:
		if cfg.BaseURL == "" {
			cfg.BaseURL = "https://api.openai.com/v1"
		}
		return &openAI{cfg: cfg}, nil
	case KindAnthropic:
		if cfg.BaseURL == "" {
			cfg.BaseURL = "https://api.anthropic.com"
		}
		return &anthropic{cfg: cfg}, nil
	case KindGemini:
		if cfg.BaseURL == "" {
			cfg.BaseURL = "https://generativelanguage.googleapis.com"
		}
		return &gemini{cfg: cfg}, nil
	default:
		return nil, fmt.Errorf("unknown provider kind %q", cfg.Kind)
	}
}

func checkDimensions(vectors [][]float32) error {
	for _, v := range vectors {
		if len(v) != EmbeddingDimensions {
			return fmt.Errorf("%w (got %d)", ErrDimensions, len(v))
		}
	}
	return nil
}

func maxTokens(n int) int {
	if n <= 0 {
		return DefaultMaxTokens
	}
	return n
}
