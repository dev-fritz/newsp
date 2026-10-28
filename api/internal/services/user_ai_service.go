package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dev-fritz/newsp/api/internal/apperrors"
	"github.com/dev-fritz/newsp/api/internal/auth"
	"github.com/dev-fritz/newsp/api/internal/models"
	"github.com/dev-fritz/newsp/api/internal/repositories"
)

// ErrBYOKDisabled is returned when an admin switched off users' own AI keys.
var ErrBYOKDisabled = apperrors.Unprocessable("BYOK_DISABLED",
	"using your own AI keys is turned off on this instance")

var errProviderKindTaken = apperrors.Conflict("PROVIDER_KIND_TAKEN", "you already connected a provider of this kind")

// byokNames are the kinds a user may connect and the name their provider gets.
// They are hosted services on fixed official URLs: users never choose a base
// URL, so their providers cannot reach the instance's private network.
var byokNames = map[models.ProviderKind]string{
	models.ProviderOpenAI:    "OpenAI",
	models.ProviderAnthropic: "Anthropic",
	models.ProviderGemini:    "Gemini",
}

// UserProviders is what the profile shows: whether BYOK is on and the
// caller's own providers.
type UserProviders struct {
	Enabled   bool
	Providers []ProviderView
}

// UserProviderChange updates a user's provider; nil fields stay as they are.
// The key can be replaced but not removed: delete the provider instead.
type UserProviderChange struct {
	APIKey  *string
	Enabled *bool
}

// UserAIService lets users connect their own AI providers (BYOK) and lists the
// chat models each user may pick.
type UserAIService struct {
	store    AIStore
	settings SettingsStore
	box      SecretBox
	factory  ProviderFactory
	now      func() time.Time
}

// NewUserAIService builds a UserAIService.
func NewUserAIService(store AIStore, settings SettingsStore, box SecretBox, factory ProviderFactory, now func() time.Time) *UserAIService {
	return &UserAIService{store: store, settings: settings, box: box, factory: factory, now: now}
}

// ChatModels lists the chat models the caller may choose: the instance ones
// and, while BYOK is on, those of the caller's own providers.
func (s *UserAIService) ChatModels(ctx context.Context, actor auth.Identity) ([]ChatModelOption, error) {
	providers, err := s.store.ListProviders(ctx)
	if err != nil {
		return nil, fmt.Errorf("chat models: %w", err)
	}
	all, err := s.store.ListModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("chat models: %w", err)
	}
	out := chatOptions(providers, all)

	enabled, err := s.enabled(ctx)
	if err != nil || !enabled {
		return out, err
	}
	own, err := s.store.ListUserProviders(ctx, actor.UserID)
	if err != nil {
		return nil, fmt.Errorf("chat models: %w", err)
	}
	ownModels, err := s.store.ListUserModels(ctx, actor.UserID)
	if err != nil {
		return nil, fmt.Errorf("chat models: %w", err)
	}
	return append(out, chatOptions(own, ownModels)...), nil
}

// List returns the caller's providers with their models. It works while BYOK
// is off, so the profile can tell why the section is hidden.
func (s *UserAIService) List(ctx context.Context, actor auth.Identity) (UserProviders, error) {
	enabled, err := s.enabled(ctx)
	if err != nil {
		return UserProviders{}, fmt.Errorf("list own providers: %w", err)
	}
	providers, err := s.store.ListUserProviders(ctx, actor.UserID)
	if err != nil {
		return UserProviders{}, fmt.Errorf("list own providers: %w", err)
	}
	all, err := s.store.ListUserModels(ctx, actor.UserID)
	if err != nil {
		return UserProviders{}, fmt.Errorf("list own providers: %w", err)
	}
	views, err := providerViews(ctx, s.store, s.box, s.now(), providers, all)
	if err != nil {
		return UserProviders{}, fmt.Errorf("list own providers: %w", err)
	}
	return UserProviders{Enabled: enabled, Providers: views}, nil
}

// Create connects a hosted provider with the caller's key, at most one per kind.
func (s *UserAIService) Create(ctx context.Context, actor auth.Identity, kind models.ProviderKind, apiKey string) (models.AIProvider, error) {
	if err := s.requireEnabled(ctx); err != nil {
		return models.AIProvider{}, fmt.Errorf("create own provider: %w", err)
	}
	name, ok := byokNames[kind]
	fields := map[string]string{}
	if !ok {
		fields["kind"] = "oneof"
	}
	if strings.TrimSpace(apiKey) == "" {
		fields["api_key"] = "required"
	}
	if len(fields) > 0 {
		return models.AIProvider{}, apperrors.Validation("invalid request", fields)
	}
	now := s.now()
	owner := actor.UserID
	p := models.AIProvider{
		ID: uuid.New(), UserID: &owner, Name: name, Kind: kind, BaseURL: defaultBaseURLs[kind],
		Enabled: true, CreatedAt: now, UpdatedAt: now,
	}
	var err error
	if p.APIKeyEnc, err = sealKey(s.box, apiKey); err != nil {
		return models.AIProvider{}, err
	}
	if err := s.store.CreateProvider(ctx, p); err != nil {
		if errors.Is(err, repositories.ErrDuplicate) {
			return models.AIProvider{}, errProviderKindTaken
		}
		return models.AIProvider{}, fmt.Errorf("create own provider: %w", err)
	}
	return p, nil
}

// Update replaces the key or switches the caller's provider on or off.
func (s *UserAIService) Update(ctx context.Context, actor auth.Identity, id uuid.UUID, change UserProviderChange) (models.AIProvider, error) {
	if err := s.requireEnabled(ctx); err != nil {
		return models.AIProvider{}, fmt.Errorf("update own provider: %w", err)
	}
	p, err := s.ownProvider(ctx, actor, id)
	if err != nil {
		return models.AIProvider{}, fmt.Errorf("update own provider: %w", err)
	}
	if change.APIKey != nil {
		if strings.TrimSpace(*change.APIKey) == "" {
			return models.AIProvider{}, apperrors.Validation("invalid request", map[string]string{"api_key": "required"})
		}
		if p.APIKeyEnc, err = sealKey(s.box, *change.APIKey); err != nil {
			return models.AIProvider{}, err
		}
	}
	if change.Enabled != nil {
		p.Enabled = *change.Enabled
	}
	p.UpdatedAt = s.now()
	if err := s.store.UpdateProvider(ctx, p); err != nil {
		return models.AIProvider{}, fmt.Errorf("update own provider: %w", err)
	}
	return p, nil
}

// Delete removes the caller's provider, its key and its models. It works while
// BYOK is off, so users can always take their key back out.
func (s *UserAIService) Delete(ctx context.Context, actor auth.Identity, id uuid.UUID) error {
	if _, err := s.ownProvider(ctx, actor, id); err != nil {
		return fmt.Errorf("delete own provider: %w", err)
	}
	err := s.store.DeleteProvider(ctx, id)
	if errors.Is(err, repositories.ErrNotFound) {
		return errProviderNotFound
	}
	if err != nil {
		return fmt.Errorf("delete own provider: %w", err)
	}
	return nil
}

// TestConnection lists the models of the caller's provider.
func (s *UserAIService) TestConnection(ctx context.Context, actor auth.Identity, id uuid.UUID) (ConnectionResult, error) {
	if err := s.requireEnabled(ctx); err != nil {
		return ConnectionResult{}, fmt.Errorf("test own provider: %w", err)
	}
	p, err := s.ownProvider(ctx, actor, id)
	if err != nil {
		return ConnectionResult{}, fmt.Errorf("test own provider: %w", err)
	}
	result, err := checkConnection(ctx, s.store, s.box, s.factory, s.now(), p)
	if err != nil {
		return ConnectionResult{}, fmt.Errorf("test own provider: %w", err)
	}
	return result, nil
}

// CreateModel registers a chat model of the caller's provider. Embeddings stay
// instance-wide: vectors of different models cannot be compared.
func (s *UserAIService) CreateModel(ctx context.Context, actor auth.Identity, providerID uuid.UUID, name, displayName string) (models.AIModel, error) {
	if err := s.requireEnabled(ctx); err != nil {
		return models.AIModel{}, fmt.Errorf("create own model: %w", err)
	}
	if _, err := s.ownProvider(ctx, actor, providerID); err != nil {
		return models.AIModel{}, fmt.Errorf("create own model: %w", err)
	}
	now := s.now()
	name = strings.TrimSpace(name)
	if name == "" {
		return models.AIModel{}, apperrors.Validation("invalid request", map[string]string{"name": "required"})
	}
	display := strings.TrimSpace(displayName)
	if display == "" {
		display = name
	}
	m := models.AIModel{
		ID: uuid.New(), ProviderID: providerID, Name: name, DisplayName: display,
		Capability: models.CapabilityChat, Enabled: true, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.store.CreateModel(ctx, m); err != nil {
		if errors.Is(err, repositories.ErrDuplicate) {
			return models.AIModel{}, apperrors.Conflict("MODEL_EXISTS", "this model is already registered for the provider")
		}
		return models.AIModel{}, fmt.Errorf("create own model: %w", err)
	}
	return m, nil
}

// UpdateModel renames, enables or disables a model of the caller's provider.
func (s *UserAIService) UpdateModel(ctx context.Context, actor auth.Identity, id uuid.UUID, change ModelChange) (models.AIModel, error) {
	if err := s.requireEnabled(ctx); err != nil {
		return models.AIModel{}, fmt.Errorf("update own model: %w", err)
	}
	m, err := s.ownModel(ctx, actor, id)
	if err != nil {
		return models.AIModel{}, fmt.Errorf("update own model: %w", err)
	}
	if change.DisplayName != nil {
		m.DisplayName = strings.TrimSpace(*change.DisplayName)
		if m.DisplayName == "" {
			return models.AIModel{}, apperrors.Validation("invalid request", map[string]string{"display_name": "required"})
		}
	}
	if change.Enabled != nil {
		m.Enabled = *change.Enabled
	}
	m.UpdatedAt = s.now()
	if err := s.store.UpdateModel(ctx, m); err != nil {
		return models.AIModel{}, fmt.Errorf("update own model: %w", err)
	}
	return m, nil
}

// DeleteModel removes a model of the caller's provider; preferences pointing at
// it fall back to the instance defaults.
func (s *UserAIService) DeleteModel(ctx context.Context, actor auth.Identity, id uuid.UUID) error {
	if _, err := s.ownModel(ctx, actor, id); err != nil {
		return fmt.Errorf("delete own model: %w", err)
	}
	err := s.store.DeleteModel(ctx, id)
	if errors.Is(err, repositories.ErrNotFound) {
		return errModelNotFound
	}
	if err != nil {
		return fmt.Errorf("delete own model: %w", err)
	}
	return nil
}

// ownProvider loads a provider owned by the caller. Instance providers and
// other users' providers answer not found.
func (s *UserAIService) ownProvider(ctx context.Context, actor auth.Identity, id uuid.UUID) (models.AIProvider, error) {
	p, err := s.store.GetProvider(ctx, id)
	if errors.Is(err, repositories.ErrNotFound) || (err == nil && !ownedBy(p, actor)) {
		return models.AIProvider{}, errProviderNotFound
	}
	return p, err
}

// ownModel loads a model of a provider owned by the caller; see ownProvider.
func (s *UserAIService) ownModel(ctx context.Context, actor auth.Identity, id uuid.UUID) (models.AIModel, error) {
	mp, err := s.store.GetModel(ctx, id)
	if errors.Is(err, repositories.ErrNotFound) || (err == nil && !ownedBy(mp.Provider, actor)) {
		return models.AIModel{}, errModelNotFound
	}
	return mp.Model, err
}

func ownedBy(p models.AIProvider, actor auth.Identity) bool {
	return p.UserID != nil && *p.UserID == actor.UserID
}

func (s *UserAIService) enabled(ctx context.Context) (bool, error) {
	settings, err := s.settings.Get(ctx)
	if err != nil {
		return false, err
	}
	return settings.BYOKEnabled, nil
}

func (s *UserAIService) requireEnabled(ctx context.Context) error {
	enabled, err := s.enabled(ctx)
	if err != nil {
		return err
	}
	if !enabled {
		return ErrBYOKDisabled
	}
	return nil
}
