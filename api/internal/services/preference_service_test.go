package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/dev-fritz/newsp/api/internal/apperrors"
	"github.com/dev-fritz/newsp/api/internal/auth"
	"github.com/dev-fritz/newsp/api/internal/models"
	"github.com/dev-fritz/newsp/api/internal/repositories"
	"github.com/dev-fritz/newsp/api/internal/services"
	"github.com/dev-fritz/newsp/api/tests/mocks"
)

func TestNextReportAt(t *testing.T) {
	t.Parallel()
	saoPaulo, err := time.LoadLocation("America/Sao_Paulo")
	require.NoError(t, err)
	newYork, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)

	tests := []struct {
		name string
		now  time.Time
		loc  *time.Location
		want time.Time
	}{
		{name: "later today", now: time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC), loc: saoPaulo, want: time.Date(2026, 10, 26, 11, 0, 0, 0, time.UTC)},
		{name: "already passed today", now: time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC), loc: saoPaulo, want: time.Date(2026, 10, 27, 11, 0, 0, 0, time.UTC)},
		{name: "exactly now moves to tomorrow", now: time.Date(2026, 10, 26, 11, 0, 0, 0, time.UTC), loc: saoPaulo, want: time.Date(2026, 10, 27, 11, 0, 0, 0, time.UTC)},
		// New York leaves DST on 2026-11-01: 08:00 is 12:00 UTC before and 13:00 UTC after.
		{name: "across a DST change", now: time.Date(2026, 10, 31, 13, 0, 0, 0, time.UTC), loc: newYork, want: time.Date(2026, 11, 1, 13, 0, 0, 0, time.UTC)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, services.NextReportAt(tt.now, tt.loc, 8, 0))
		})
	}
}

type prefDeps struct {
	prefs    *mocks.MockPreferenceStore
	accounts *mocks.MockAccountStore
	models   *mocks.MockModelLookup
	settings *mocks.MockSettingsStore
	embedder *mocks.MockTextEmbedder
}

func newPreferenceService(t *testing.T) (*services.PreferenceService, prefDeps) {
	ctrl := gomock.NewController(t)
	d := prefDeps{
		prefs: mocks.NewMockPreferenceStore(ctrl), accounts: mocks.NewMockAccountStore(ctrl),
		models: mocks.NewMockModelLookup(ctrl), settings: mocks.NewMockSettingsStore(ctrl), embedder: mocks.NewMockTextEmbedder(ctrl),
	}
	return services.NewPreferenceService(inlineTx{}, d.prefs, d.accounts, d.models, d.settings, d.embedder, func() time.Time { return fixedNow }), d
}

func validInput() services.PreferenceInput {
	return services.PreferenceInput{
		Topics: []string{"  AI   agents ", "ai agents", "Fintech"}, OutputLang: "pt-BR", Timezone: "America/Sao_Paulo",
		ReportEnabled: true, ReportTime: "08:00", IdeasPerDay: 3, Theme: "dark", UILocale: "pt-BR",
	}
}

func TestUpdatePreferencesValidation(t *testing.T) {
	t.Parallel()
	actor := auth.Identity{UserID: uuid.New(), Role: models.RoleMember}

	in := validInput()
	in.Timezone = "Mars/Olympus"
	in.ReportTime = "25:99"
	in.Topics = make([]string, 21)
	for i := range in.Topics {
		in.Topics[i] = uuid.NewString()
	}
	svc, d := newPreferenceService(t)
	disabled := uuid.New()
	in.ReportModelID = &disabled
	d.models.EXPECT().GetModel(gomock.Any(), disabled).Return(repositories.ModelWithProvider{
		Model: models.AIModel{Enabled: false, Capability: models.CapabilityChat}, Provider: models.AIProvider{Enabled: true},
	}, nil)

	_, err := svc.Update(context.Background(), actor, in)
	requireCode(t, err, "VALIDATION_FAILED")
	appErr, _ := asAppErr(err)
	assert.Equal(t, map[string]string{"topics": "max", "timezone": "timezone", "report_time": "time", "report_model_id": "unavailable"}, appErr.Fields)
}

func TestUpdatePreferencesSavesAndEmbedsTopics(t *testing.T) {
	t.Parallel()
	svc, d := newPreferenceService(t)
	actor := auth.Identity{UserID: uuid.New(), Role: models.RoleMember}

	d.prefs.EXPECT().Get(gomock.Any(), actor.UserID).Return(models.UserPreferences{}, repositories.ErrNotFound)
	d.prefs.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p models.UserPreferences) error {
		assert.Equal(t, []string{"AI agents", "Fintech"}, p.Topics, "trimmed and deduplicated")
		require.NotNil(t, p.NextReportAt)
		assert.Equal(t, time.Date(2026, 10, 27, 11, 0, 0, 0, time.UTC), *p.NextReportAt)
		return nil
	})
	d.accounts.EXPECT().UpdateLocale(gomock.Any(), actor.UserID, "pt-BR", fixedNow).Return(nil)
	d.embedder.EXPECT().Embed(gomock.Any(), models.PurposeEmbedding, []string{"Topics of interest: AI agents; Fintech"}).
		Return([][]float32{make([]float32, 768)}, "nomic-embed-text", nil)
	d.prefs.EXPECT().SetTopicsEmbedding(gomock.Any(), actor.UserID, gomock.Any(), "nomic-embed-text").Return(nil)
	// The view is re-read after saving.
	d.accounts.EXPECT().GetByID(gomock.Any(), actor.UserID).Return(models.User{Locale: "pt-BR"}, nil)
	model := "nomic-embed-text"
	d.prefs.EXPECT().Get(gomock.Any(), actor.UserID).Return(models.UserPreferences{Topics: []string{"AI agents", "Fintech"}, TopicsEmbeddingModel: &model}, nil)

	view, err := svc.Update(context.Background(), actor, validInput())
	require.NoError(t, err)
	assert.True(t, view.HasTopicsEmbedding)
}

func TestUpdatePreferencesSurvivesEmbeddingFailure(t *testing.T) {
	t.Parallel()
	svc, d := newPreferenceService(t)
	actor := auth.Identity{UserID: uuid.New(), Role: models.RoleMember}
	in := validInput()
	in.ReportEnabled = false

	d.prefs.EXPECT().Get(gomock.Any(), actor.UserID).Return(models.UserPreferences{}, repositories.ErrNotFound)
	d.prefs.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p models.UserPreferences) error {
		assert.Nil(t, p.NextReportAt, "disabled reports are not scheduled")
		return nil
	})
	d.accounts.EXPECT().UpdateLocale(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	d.embedder.EXPECT().Embed(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, "", services.ErrEmbeddingModelNotSet)
	d.accounts.EXPECT().GetByID(gomock.Any(), actor.UserID).Return(models.User{Locale: "en"}, nil)
	d.prefs.EXPECT().Get(gomock.Any(), actor.UserID).Return(models.UserPreferences{}, nil)

	_, err := svc.Update(context.Background(), actor, in)
	require.NoError(t, err)
}

func TestEffectiveModelsFallBackToDefaults(t *testing.T) {
	t.Parallel()
	svc, d := newPreferenceService(t)
	userID, defReport, defIdea, own := uuid.New(), uuid.New(), uuid.New(), uuid.New()

	d.settings.EXPECT().Get(gomock.Any()).Return(models.InstanceSettings{DefaultReportModelID: &defReport, DefaultIdeaModelID: &defIdea}, nil)
	d.prefs.EXPECT().Get(gomock.Any(), userID).Return(models.UserPreferences{IdeaModelID: &own}, nil)
	d.models.EXPECT().GetModel(gomock.Any(), own).Return(modelOf(nil), nil)

	report, idea, err := svc.EffectiveModels(context.Background(), userID)
	require.NoError(t, err)
	assert.Equal(t, defReport, *report)
	assert.Equal(t, own, *idea)
}

// modelOf is an enabled chat model of a provider owned by owner (nil = instance).
func modelOf(owner *uuid.UUID) repositories.ModelWithProvider {
	providerID := uuid.New()
	return repositories.ModelWithProvider{
		Model:    models.AIModel{ID: uuid.New(), ProviderID: providerID, Capability: models.CapabilityChat, Enabled: true},
		Provider: models.AIProvider{ID: providerID, UserID: owner, Enabled: true},
	}
}

func TestEffectiveModelsWithBYOKModels(t *testing.T) {
	t.Parallel()
	userID, other, def, picked := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	tests := []struct {
		name  string
		owner uuid.UUID
		byok  bool
		want  uuid.UUID
	}{
		{"own model while BYOK is on", userID, true, picked},
		{"own model falls back while BYOK is off", userID, false, def},
		{"another user's model falls back", other, true, def},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc, d := newPreferenceService(t)
			d.settings.EXPECT().Get(gomock.Any()).Return(models.InstanceSettings{DefaultReportModelID: &def, BYOKEnabled: tt.byok}, nil)
			d.prefs.EXPECT().Get(gomock.Any(), userID).Return(models.UserPreferences{ReportModelID: &picked}, nil)
			d.models.EXPECT().GetModel(gomock.Any(), picked).Return(modelOf(&tt.owner), nil)

			report, idea, err := svc.EffectiveModels(context.Background(), userID)
			require.NoError(t, err)
			assert.Equal(t, tt.want, *report)
			assert.Nil(t, idea)
		})
	}
}

func TestUpdatePreferencesRejectsForeignOrDisabledBYOKModels(t *testing.T) {
	t.Parallel()
	actor := auth.Identity{UserID: uuid.New(), Role: models.RoleMember}
	other := uuid.New()
	tests := []struct {
		name  string
		owner *uuid.UUID
		byok  bool
	}{
		{"another user's model", &other, true},
		{"own model while BYOK is off", &actor.UserID, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc, d := newPreferenceService(t)
			picked := uuid.New()
			d.models.EXPECT().GetModel(gomock.Any(), picked).Return(modelOf(tt.owner), nil)
			d.settings.EXPECT().Get(gomock.Any()).Return(models.InstanceSettings{BYOKEnabled: tt.byok}, nil).AnyTimes()
			in := validInput()
			in.ReportModelID = &picked

			_, err := svc.Update(context.Background(), actor, in)
			appErr, ok := apperrors.As(err)
			require.True(t, ok, "expected validation error, got %v", err)
			assert.Equal(t, "unavailable", appErr.Fields["report_model_id"])
		})
	}
}

func TestGetPreferencesDefaults(t *testing.T) {
	t.Parallel()
	svc, d := newPreferenceService(t)
	actor := auth.Identity{UserID: uuid.New(), Role: models.RoleMember}
	d.accounts.EXPECT().GetByID(gomock.Any(), actor.UserID).Return(models.User{Locale: "en"}, nil)
	d.prefs.EXPECT().Get(gomock.Any(), actor.UserID).Return(models.UserPreferences{}, repositories.ErrNotFound)

	view, err := svc.Get(context.Background(), actor)
	require.NoError(t, err)
	assert.Equal(t, "en", view.Preferences.OutputLang, "output language defaults to the UI language")
	assert.Equal(t, "08:00", view.Preferences.ReportTime)
	assert.False(t, errors.Is(err, repositories.ErrNotFound))
}
