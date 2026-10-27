package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/dev-fritz/newsp/api/internal/apperrors"
	"github.com/dev-fritz/newsp/api/internal/models"
	"github.com/dev-fritz/newsp/api/internal/repositories"
	"github.com/dev-fritz/newsp/api/pkg/ai"
)

// Errors raised when the configured models cannot be used.
var (
	ErrModelUnavailable = apperrors.Unprocessable("AI_MODEL_UNAVAILABLE",
		"the selected AI model is not available; ask an admin to enable it")
	ErrEmbeddingModelNotSet = apperrors.Unprocessable("EMBEDDING_MODEL_NOT_SET",
		"no embedding model is configured for this instance")
)

// embedBatchSize bounds one embeddings request.
const embedBatchSize = 64

// ProviderFactory builds a provider client; tests swap it for fakes.
type ProviderFactory func(cfg ai.Config) (ai.Provider, error)

// ResolvedModel identifies the model that served a call.
type ResolvedModel struct {
	ID          uuid.UUID
	Name        string // model id at the provider
	DisplayName string
}

// AIGateway is how the rest of the application talks to AI models: it resolves
// a configured model, decrypts its provider key, calls it and accounts for usage.
type AIGateway struct {
	store    AIStore
	settings SettingsStore
	box      SecretBox
	factory  ProviderFactory
	now      func() time.Time
}

// NewAIGateway builds an AIGateway.
func NewAIGateway(store AIStore, settings SettingsStore, box SecretBox, factory ProviderFactory, now func() time.Time) *AIGateway {
	return &AIGateway{store: store, settings: settings, box: box, factory: factory, now: now}
}

// client resolves an enabled model of the wanted capability.
func (g *AIGateway) client(ctx context.Context, modelID uuid.UUID, capability models.ModelCapability) (ai.Provider, ResolvedModel, error) {
	mp, err := g.store.GetModel(ctx, modelID)
	if errors.Is(err, repositories.ErrNotFound) {
		return nil, ResolvedModel{}, ErrModelUnavailable
	}
	if err != nil {
		return nil, ResolvedModel{}, err
	}
	if !mp.Model.Enabled || !mp.Provider.Enabled || mp.Model.Capability != capability {
		return nil, ResolvedModel{}, ErrModelUnavailable
	}
	provider, err := providerClient(mp.Provider, g.box, g.factory)
	if err != nil {
		return nil, ResolvedModel{}, err
	}
	return provider, ResolvedModel{ID: mp.Model.ID, Name: mp.Model.Name, DisplayName: mp.Model.DisplayName}, nil
}

// providerClient decrypts a provider's key and builds its client.
func providerClient(p models.AIProvider, box SecretBox, factory ProviderFactory) (ai.Provider, error) {
	key := ""
	if len(p.APIKeyEnc) > 0 {
		plain, err := box.Open(p.APIKeyEnc)
		if err != nil {
			return nil, fmt.Errorf("decrypt key of provider %s: %w", p.ID, err)
		}
		key = string(plain)
	}
	return factory(ai.Config{Kind: ai.Kind(p.Kind), BaseURL: p.BaseURL, APIKey: key})
}

// Chat sends req to the chat model modelID and records its usage.
func (g *AIGateway) Chat(ctx context.Context, modelID uuid.UUID, purpose models.UsagePurpose, userID *uuid.UUID, req ai.ChatRequest) (ai.ChatResponse, ResolvedModel, error) {
	provider, model, err := g.client(ctx, modelID, models.CapabilityChat)
	if err != nil {
		return ai.ChatResponse{}, model, err
	}
	req.Model = model.Name
	res, err := provider.Chat(ctx, req)
	g.record(ctx, model, purpose, userID, res.Usage)
	return res, model, err
}

// GenerateJSON asks the chat model modelID for JSON matching req.Schema.
func (g *AIGateway) GenerateJSON(ctx context.Context, modelID uuid.UUID, purpose models.UsagePurpose, userID *uuid.UUID,
	req ai.ChatRequest, dst any, validate func() error,
) (ResolvedModel, error) {
	provider, model, err := g.client(ctx, modelID, models.CapabilityChat)
	if err != nil {
		return model, err
	}
	req.Model = model.Name
	usage, err := ai.GenerateJSON(ctx, provider, req, dst, validate)
	g.record(ctx, model, purpose, userID, usage)
	return model, err
}

// EmbeddingModel returns the instance embedding model.
func (g *AIGateway) EmbeddingModel(ctx context.Context) (ResolvedModel, error) {
	_, model, err := g.embedder(ctx)
	return model, err
}

func (g *AIGateway) embedder(ctx context.Context) (ai.Provider, ResolvedModel, error) {
	settings, err := g.settings.Get(ctx)
	if err != nil {
		return nil, ResolvedModel{}, err
	}
	if settings.EmbeddingModelID == nil {
		return nil, ResolvedModel{}, ErrEmbeddingModelNotSet
	}
	return g.client(ctx, *settings.EmbeddingModelID, models.CapabilityEmbedding)
}

// Embed embeds texts with the instance embedding model, in batches. It returns
// one 768-dimension vector per text and the model name to store beside them.
func (g *AIGateway) Embed(ctx context.Context, purpose models.UsagePurpose, texts []string) ([][]float32, string, error) {
	provider, model, err := g.embedder(ctx)
	if err != nil {
		return nil, "", err
	}
	vectors := make([][]float32, 0, len(texts))
	for start := 0; start < len(texts); start += embedBatchSize {
		end := min(start+embedBatchSize, len(texts))
		res, err := provider.Embed(ctx, ai.EmbedRequest{Model: model.Name, Inputs: texts[start:end]})
		g.record(ctx, model, purpose, nil, res.Usage)
		if err != nil {
			return nil, "", err
		}
		vectors = append(vectors, res.Vectors...)
	}
	return vectors, model.Name, nil
}

// record stores usage; accounting failures never fail the AI call itself.
func (g *AIGateway) record(ctx context.Context, model ResolvedModel, purpose models.UsagePurpose, userID *uuid.UUID, usage ai.Usage) {
	if usage.InputTokens == 0 && usage.OutputTokens == 0 {
		return
	}
	id := model.ID
	_ = g.store.RecordUsage(context.WithoutCancel(ctx), models.AIUsage{
		ID: uuid.New(), UserID: userID, ModelID: &id, ModelName: model.Name, Purpose: purpose,
		InputTokens: usage.InputTokens, OutputTokens: usage.OutputTokens, CreatedAt: g.now(),
	})
}
