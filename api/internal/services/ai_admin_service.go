package services

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dev-fritz/newsp/api/internal/apperrors"
	"github.com/dev-fritz/newsp/api/internal/auth"
	"github.com/dev-fritz/newsp/api/internal/models"
	"github.com/dev-fritz/newsp/api/internal/repositories"
	"github.com/dev-fritz/newsp/api/internal/workers"
	"github.com/dev-fritz/newsp/api/pkg/ai"
)

// JobReindexEmbeddings re-embeds items, ideas and topics after the instance
// embedding model changes.
const JobReindexEmbeddings = "embeddings.reindex"

var (
	errProviderNotFound = apperrors.NotFound("PROVIDER_NOT_FOUND", "provider not found")
	errModelNotFound    = apperrors.NotFound("MODEL_NOT_FOUND", "model not found")
)

var defaultBaseURLs = map[models.ProviderKind]string{
	models.ProviderOpenAI:    "https://api.openai.com/v1",
	models.ProviderAnthropic: "https://api.anthropic.com",
	models.ProviderGemini:    "https://generativelanguage.googleapis.com",
}

// ProviderInput creates a provider.
type ProviderInput struct {
	Name    string
	Kind    models.ProviderKind
	BaseURL string
	APIKey  string
}

// ProviderChange updates a provider; nil fields stay as they are. APIKey set to
// "" removes the key.
type ProviderChange struct {
	Name    *string
	BaseURL *string
	APIKey  *string
	Enabled *bool
}

// ModelInput registers a model of a provider.
type ModelInput struct {
	Name        string
	DisplayName string
	Capability  models.ModelCapability
}

// ModelChange updates a model; nil fields stay as they are.
type ModelChange struct {
	DisplayName *string
	Enabled     *bool
}

// ModelView is a model with its recent token usage.
type ModelView struct {
	Model        models.AIModel
	TokensLast7d int64
}

// ProviderView is a provider as admins see it: never the key, only a hint.
type ProviderView struct {
	Provider models.AIProvider
	KeyHint  string // last four characters, or empty without a key
	Models   []ModelView
}

// ConnectionResult is the outcome of a connection test.
type ConnectionResult struct {
	OK     bool
	Models []string
	Error  string
}

// SettingsChange updates instance settings; nil fields stay as they are. A
// pointer to uuid.Nil clears a model setting.
type SettingsChange struct {
	EmbeddingModelID        *uuid.UUID
	DefaultReportModelID    *uuid.UUID
	DefaultIdeaModelID      *uuid.UUID
	RetentionDays           *int
	IdeaSimilarityThreshold *float32
	SignupEnabled           *bool
	InactivityPauseDays     *int
	BYOKEnabled             *bool
}

var errMailerNotConfigured = apperrors.Unprocessable("MAILER_NOT_CONFIGURED",
	"public signup needs e-mail delivery: set SMTP_HOST and MAIL_FROM")

// SettingsResult reports the new settings and whether a reindex was started.
type SettingsResult struct {
	Settings     models.InstanceSettings
	ReindexJobID *uuid.UUID
}

// ChatModelOption is an enabled chat model users can pick.
type ChatModelOption struct {
	Model        models.AIModel
	ProviderName string
	Local        bool
	Own          bool // served by the user's own provider (BYOK)
}

// AIAdminService manages providers, models and instance AI settings.
type AIAdminService struct {
	tx       TxRunner
	store    AIStore
	settings SettingsStore
	box      SecretBox
	factory  ProviderFactory
	queue    Enqueuer
	// mailReady tells whether e-mails reach people; public signup needs it.
	mailReady bool
	now       func() time.Time
}

// NewAIAdminService builds an AIAdminService.
func NewAIAdminService(tx TxRunner, store AIStore, settings SettingsStore, box SecretBox, factory ProviderFactory, queue Enqueuer,
	mailReady bool, now func() time.Time,
) *AIAdminService {
	return &AIAdminService{tx: tx, store: store, settings: settings, box: box, factory: factory, queue: queue, mailReady: mailReady, now: now}
}

// MailReady reports whether e-mails reach people, so the settings page can
// explain why public signup cannot be turned on.
func (s *AIAdminService) MailReady() bool { return s.mailReady }

// ListProviders returns every provider with its models and usage.
func (s *AIAdminService) ListProviders(ctx context.Context, actor auth.Identity) ([]ProviderView, error) {
	if !auth.Can(actor, auth.PermProvidersManage) {
		return nil, errForbidden
	}
	providers, err := s.store.ListProviders(ctx)
	if err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
	}
	allModels, err := s.store.ListModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
	}
	views, err := providerViews(ctx, s.store, s.box, s.now(), providers, allModels)
	if err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
	}
	return views, nil
}

// providerViews groups models under their providers with the token usage of
// the last 7 days and a key hint.
func providerViews(ctx context.Context, store AIStore, box SecretBox, now time.Time,
	providers []models.AIProvider, allModels []models.AIModel,
) ([]ProviderView, error) {
	usage, err := store.TokensSince(ctx, now.Add(-7*24*time.Hour))
	if err != nil {
		return nil, err
	}
	views := make([]ProviderView, 0, len(providers))
	index := make(map[uuid.UUID]int, len(providers))
	for _, p := range providers {
		index[p.ID] = len(views)
		views = append(views, ProviderView{Provider: p, KeyHint: keyHint(box, p)})
	}
	for _, m := range allModels {
		if i, ok := index[m.ProviderID]; ok {
			views[i].Models = append(views[i].Models, ModelView{Model: m, TokensLast7d: usage[m.ID]})
		}
	}
	return views, nil
}

func keyHint(box SecretBox, p models.AIProvider) string {
	if len(p.APIKeyEnc) == 0 {
		return ""
	}
	plain, err := box.Open(p.APIKeyEnc)
	if err != nil || len(plain) < 8 {
		return "••••"
	}
	return "••••" + string(plain[len(plain)-4:])
}

// CreateProvider registers a provider with an encrypted key.
func (s *AIAdminService) CreateProvider(ctx context.Context, actor auth.Identity, in ProviderInput) (models.AIProvider, error) {
	if !auth.Can(actor, auth.PermProvidersManage) {
		return models.AIProvider{}, errForbidden
	}
	baseURL, err := normalizeBaseURL(in.Kind, in.BaseURL)
	if err != nil {
		return models.AIProvider{}, err
	}
	now := s.now()
	p := models.AIProvider{
		ID: uuid.New(), Name: strings.TrimSpace(in.Name), Kind: in.Kind, BaseURL: baseURL,
		Enabled: true, CreatedAt: now, UpdatedAt: now,
	}
	if p.APIKeyEnc, err = sealKey(s.box, in.APIKey); err != nil {
		return models.AIProvider{}, err
	}
	if err := s.store.CreateProvider(ctx, p); err != nil {
		if errors.Is(err, repositories.ErrDuplicate) {
			return models.AIProvider{}, apperrors.Conflict("PROVIDER_NAME_TAKEN", "a provider with this name already exists")
		}
		return models.AIProvider{}, fmt.Errorf("create provider: %w", err)
	}
	return p, nil
}

// UpdateProvider changes a provider.
func (s *AIAdminService) UpdateProvider(ctx context.Context, actor auth.Identity, id uuid.UUID, change ProviderChange) (models.AIProvider, error) {
	if !auth.Can(actor, auth.PermProvidersManage) {
		return models.AIProvider{}, errForbidden
	}
	p, err := s.instanceProvider(ctx, id)
	if err != nil {
		return models.AIProvider{}, fmt.Errorf("update provider: %w", err)
	}
	if change.Name != nil {
		p.Name = strings.TrimSpace(*change.Name)
	}
	if change.BaseURL != nil {
		if p.BaseURL, err = normalizeBaseURL(p.Kind, *change.BaseURL); err != nil {
			return models.AIProvider{}, err
		}
	}
	if change.APIKey != nil {
		if p.APIKeyEnc, err = sealKey(s.box, *change.APIKey); err != nil {
			return models.AIProvider{}, err
		}
	}
	if change.Enabled != nil {
		p.Enabled = *change.Enabled
	}
	p.UpdatedAt = s.now()
	if err := s.store.UpdateProvider(ctx, p); err != nil {
		if errors.Is(err, repositories.ErrDuplicate) {
			return models.AIProvider{}, apperrors.Conflict("PROVIDER_NAME_TAKEN", "a provider with this name already exists")
		}
		return models.AIProvider{}, fmt.Errorf("update provider: %w", err)
	}
	return p, nil
}

// DeleteProvider removes a provider and its models.
func (s *AIAdminService) DeleteProvider(ctx context.Context, actor auth.Identity, id uuid.UUID) error {
	if !auth.Can(actor, auth.PermProvidersManage) {
		return errForbidden
	}
	if _, err := s.instanceProvider(ctx, id); err != nil {
		return fmt.Errorf("delete provider: %w", err)
	}
	err := s.store.DeleteProvider(ctx, id)
	if errors.Is(err, repositories.ErrNotFound) {
		return errProviderNotFound
	}
	if err != nil {
		return fmt.Errorf("delete provider: %w", err)
	}
	return nil
}

// TestConnection lists the provider's models and records the outcome.
func (s *AIAdminService) TestConnection(ctx context.Context, actor auth.Identity, id uuid.UUID) (ConnectionResult, error) {
	if !auth.Can(actor, auth.PermProvidersManage) {
		return ConnectionResult{}, errForbidden
	}
	p, err := s.instanceProvider(ctx, id)
	if err != nil {
		return ConnectionResult{}, fmt.Errorf("test connection: %w", err)
	}
	result, err := checkConnection(ctx, s.store, s.box, s.factory, s.now(), p)
	if err != nil {
		return ConnectionResult{}, fmt.Errorf("test connection: %w", err)
	}
	return result, nil
}

// checkConnection lists the provider's models and records the outcome on it.
// A provider failure is a result, not an error.
func checkConnection(ctx context.Context, store AIStore, box SecretBox, factory ProviderFactory, now time.Time,
	p models.AIProvider,
) (ConnectionResult, error) {
	result := ConnectionResult{}
	client, err := providerClient(p, box, factory)
	if err == nil {
		callCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		result.Models, err = client.ListModels(callCtx)
		cancel()
	}
	var checkErr *string
	if err != nil {
		result.Error = connectionError(err)
		checkErr = &result.Error
	} else {
		result.OK = true
	}
	if err := store.SetProviderCheck(ctx, p.ID, now, checkErr); err != nil {
		return ConnectionResult{}, err
	}
	return result, nil
}

// connectionError turns a provider failure into a message for the admin.
func connectionError(err error) string {
	var apiErr *ai.APIError
	switch {
	case errors.As(err, &apiErr) && apiErr.KeyRejected():
		return fmt.Sprintf("the provider rejected the API key (HTTP %d)", apiErr.Status)
	case errors.As(err, &apiErr):
		return fmt.Sprintf("the provider answered HTTP %d: %s", apiErr.Status, apiErr.Message)
	case errors.Is(err, context.DeadlineExceeded):
		return "the provider did not answer in time"
	default:
		return "could not reach the provider; check the base URL and the network"
	}
}

// CreateModel registers a model of a provider.
func (s *AIAdminService) CreateModel(ctx context.Context, actor auth.Identity, providerID uuid.UUID, in ModelInput) (models.AIModel, error) {
	if !auth.Can(actor, auth.PermProvidersManage) {
		return models.AIModel{}, errForbidden
	}
	p, err := s.instanceProvider(ctx, providerID)
	if err != nil {
		return models.AIModel{}, fmt.Errorf("create model: %w", err)
	}
	if in.Capability == models.CapabilityEmbedding && p.Kind == models.ProviderAnthropic {
		return models.AIModel{}, apperrors.Unprocessable("EMBEDDINGS_UNSUPPORTED", "Anthropic does not offer embedding models")
	}
	now := s.now()
	display := strings.TrimSpace(in.DisplayName)
	if display == "" {
		display = in.Name
	}
	m := models.AIModel{
		ID: uuid.New(), ProviderID: providerID, Name: strings.TrimSpace(in.Name), DisplayName: display,
		Capability: in.Capability, Enabled: true, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.store.CreateModel(ctx, m); err != nil {
		if errors.Is(err, repositories.ErrDuplicate) {
			return models.AIModel{}, apperrors.Conflict("MODEL_EXISTS", "this model is already registered for the provider")
		}
		return models.AIModel{}, fmt.Errorf("create model: %w", err)
	}
	return m, nil
}

// UpdateModel renames, enables or disables a model.
func (s *AIAdminService) UpdateModel(ctx context.Context, actor auth.Identity, id uuid.UUID, change ModelChange) (models.AIModel, error) {
	if !auth.Can(actor, auth.PermProvidersManage) {
		return models.AIModel{}, errForbidden
	}
	m, err := s.instanceModel(ctx, id)
	if err != nil {
		return models.AIModel{}, fmt.Errorf("update model: %w", err)
	}
	if change.DisplayName != nil {
		m.DisplayName = strings.TrimSpace(*change.DisplayName)
	}
	if change.Enabled != nil {
		m.Enabled = *change.Enabled
	}
	m.UpdatedAt = s.now()
	if err := s.store.UpdateModel(ctx, m); err != nil {
		return models.AIModel{}, fmt.Errorf("update model: %w", err)
	}
	return m, nil
}

// DeleteModel removes a model.
func (s *AIAdminService) DeleteModel(ctx context.Context, actor auth.Identity, id uuid.UUID) error {
	if !auth.Can(actor, auth.PermProvidersManage) {
		return errForbidden
	}
	if _, err := s.instanceModel(ctx, id); err != nil {
		return fmt.Errorf("delete model: %w", err)
	}
	err := s.store.DeleteModel(ctx, id)
	if errors.Is(err, repositories.ErrNotFound) {
		return errModelNotFound
	}
	if err != nil {
		return fmt.Errorf("delete model: %w", err)
	}
	return nil
}

// instanceProvider loads an admin-managed provider. Providers owned by users
// (BYOK) answer not found: admins neither see nor change them.
func (s *AIAdminService) instanceProvider(ctx context.Context, id uuid.UUID) (models.AIProvider, error) {
	p, err := s.store.GetProvider(ctx, id)
	if errors.Is(err, repositories.ErrNotFound) || (err == nil && p.UserID != nil) {
		return models.AIProvider{}, errProviderNotFound
	}
	return p, err
}

// instanceModel loads a model of an admin-managed provider; see instanceProvider.
func (s *AIAdminService) instanceModel(ctx context.Context, id uuid.UUID) (models.AIModel, error) {
	mp, err := s.store.GetModel(ctx, id)
	if errors.Is(err, repositories.ErrNotFound) || (err == nil && mp.Provider.UserID != nil) {
		return models.AIModel{}, errModelNotFound
	}
	return mp.Model, err
}

// chatOptions keeps the enabled chat models of enabled providers.
func chatOptions(providers []models.AIProvider, all []models.AIModel) []ChatModelOption {
	byID := make(map[uuid.UUID]models.AIProvider, len(providers))
	for _, p := range providers {
		byID[p.ID] = p
	}
	var out []ChatModelOption
	for _, m := range all {
		p, ok := byID[m.ProviderID]
		if !ok || !p.Enabled || !m.Enabled || m.Capability != models.CapabilityChat {
			continue
		}
		out = append(out, ChatModelOption{
			Model: m, ProviderName: p.Name, Local: p.Kind == models.ProviderOpenAICompatible, Own: p.UserID != nil,
		})
	}
	return out
}

// GetSettings returns the instance settings.
func (s *AIAdminService) GetSettings(ctx context.Context, actor auth.Identity) (models.InstanceSettings, error) {
	if !auth.Can(actor, auth.PermSettingsManage) {
		return models.InstanceSettings{}, errForbidden
	}
	settings, err := s.settings.Get(ctx)
	if err != nil {
		return models.InstanceSettings{}, fmt.Errorf("get settings: %w", err)
	}
	return settings, nil
}

// UpdateSettings changes instance settings. Changing the embedding model
// starts a reindex, because vectors from different models are not comparable.
func (s *AIAdminService) UpdateSettings(ctx context.Context, actor auth.Identity, change SettingsChange) (SettingsResult, error) {
	if !auth.Can(actor, auth.PermSettingsManage) {
		return SettingsResult{}, errForbidden
	}
	var result SettingsResult
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		settings, err := s.settings.GetForUpdate(ctx)
		if err != nil {
			return err
		}
		previousEmbedding := settings.EmbeddingModelID

		fields := map[string]string{}
		set := func(field string, dst **uuid.UUID, id *uuid.UUID, capability models.ModelCapability) error {
			if id == nil {
				return nil
			}
			if *id == uuid.Nil {
				*dst = nil
				return nil
			}
			ok, err := s.modelUsable(ctx, *id, capability)
			if err != nil {
				return err
			}
			if !ok {
				fields[field] = "unavailable"
				return nil
			}
			v := *id
			*dst = &v
			return nil
		}
		if err := set("embedding_model_id", &settings.EmbeddingModelID, change.EmbeddingModelID, models.CapabilityEmbedding); err != nil {
			return err
		}
		if err := set("default_report_model_id", &settings.DefaultReportModelID, change.DefaultReportModelID, models.CapabilityChat); err != nil {
			return err
		}
		if err := set("default_idea_model_id", &settings.DefaultIdeaModelID, change.DefaultIdeaModelID, models.CapabilityChat); err != nil {
			return err
		}
		if len(fields) > 0 {
			return apperrors.Validation("pick an enabled model with the right capability", fields)
		}
		if change.RetentionDays != nil {
			settings.RetentionDays = *change.RetentionDays
		}
		if change.BYOKEnabled != nil {
			settings.BYOKEnabled = *change.BYOKEnabled
		}
		if change.IdeaSimilarityThreshold != nil {
			settings.IdeaSimilarityThreshold = *change.IdeaSimilarityThreshold
		}
		if change.SignupEnabled != nil {
			if *change.SignupEnabled && !s.mailReady {
				return errMailerNotConfigured
			}
			settings.SignupEnabled = *change.SignupEnabled
		}
		if change.InactivityPauseDays != nil {
			settings.InactivityPauseDays = *change.InactivityPauseDays
		}
		settings.UpdatedAt = s.now()
		if err := s.settings.UpdateAI(ctx, settings); err != nil {
			return err
		}
		if err := s.settings.UpdateAccounts(ctx, settings); err != nil {
			return err
		}
		result.Settings = settings

		if settings.EmbeddingModelID != nil && !sameID(previousEmbedding, settings.EmbeddingModelID) {
			jobID, err := s.queue.Enqueue(ctx, JobReindexEmbeddings, struct{}{}, workers.EnqueueOptions{DedupeKey: JobReindexEmbeddings})
			if err != nil {
				return err
			}
			result.ReindexJobID = &jobID
		}
		return nil
	})
	if err != nil {
		return SettingsResult{}, fmt.Errorf("update settings: %w", err)
	}
	return result, nil
}

func (s *AIAdminService) modelUsable(ctx context.Context, id uuid.UUID, capability models.ModelCapability) (bool, error) {
	mp, err := s.store.GetModel(ctx, id)
	if errors.Is(err, repositories.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return mp.Provider.UserID == nil && mp.Model.Enabled && mp.Provider.Enabled && mp.Model.Capability == capability, nil
}

func sealKey(box SecretBox, key string) ([]byte, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, nil
	}
	sealed, err := box.Seal([]byte(key))
	if err != nil {
		return nil, fmt.Errorf("encrypt provider key: %w", err)
	}
	return sealed, nil
}

// normalizeBaseURL applies the provider default and requires an absolute
// http(s) URL. Private addresses are allowed on purpose: local model servers
// (Ollama, llama.cpp) usually live on the LAN, and only admins set this value.
func normalizeBaseURL(kind models.ProviderKind, raw string) (string, error) {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	if raw == "" {
		def, ok := defaultBaseURLs[kind]
		if !ok {
			return "", apperrors.Validation("invalid request", map[string]string{"base_url": "required"})
		}
		return def, nil
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return "", apperrors.Validation("invalid request", map[string]string{"base_url": "url"})
	}
	return raw, nil
}

func sameID(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
