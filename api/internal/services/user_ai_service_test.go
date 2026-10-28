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

	"github.com/dev-fritz/newsp/api/internal/auth"
	"github.com/dev-fritz/newsp/api/internal/models"
	"github.com/dev-fritz/newsp/api/internal/repositories"
	"github.com/dev-fritz/newsp/api/internal/services"
	"github.com/dev-fritz/newsp/api/pkg/ai"
	"github.com/dev-fritz/newsp/api/tests/mocks"
)

type userAIDeps struct {
	store    *mocks.MockAIStore
	settings *mocks.MockSettingsStore
	box      *mocks.MockSecretBox
}

// newUserAIService builds the service with BYOK switched on or off.
func newUserAIService(t *testing.T, byok bool, provider ai.Provider) (*services.UserAIService, userAIDeps) {
	t.Helper()
	ctrl := gomock.NewController(t)
	d := userAIDeps{store: mocks.NewMockAIStore(ctrl), settings: mocks.NewMockSettingsStore(ctrl), box: mocks.NewMockSecretBox(ctrl)}
	d.settings.EXPECT().Get(gomock.Any()).Return(models.InstanceSettings{BYOKEnabled: byok}, nil).AnyTimes()
	factory := func(ai.Config) (ai.Provider, error) { return provider, nil }
	return services.NewUserAIService(d.store, d.settings, d.box, factory, func() time.Time { return fixedNow }), d
}

var member = auth.Identity{UserID: uuid.New(), Role: models.RoleMember}

func TestCreateOwnProviderUsesOfficialURLAndEncryptsKey(t *testing.T) {
	t.Parallel()
	svc, d := newUserAIService(t, true, nil)
	d.box.EXPECT().Seal([]byte("sk-secret-key")).Return([]byte("sealed"), nil)
	d.store.EXPECT().CreateProvider(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p models.AIProvider) error {
		require.NotNil(t, p.UserID)
		assert.Equal(t, member.UserID, *p.UserID)
		assert.Equal(t, "Anthropic", p.Name)
		assert.Equal(t, "https://api.anthropic.com", p.BaseURL)
		assert.Equal(t, []byte("sealed"), p.APIKeyEnc)
		return nil
	})

	_, err := svc.Create(context.Background(), member, models.ProviderAnthropic, "  sk-secret-key ")
	require.NoError(t, err)
}

func TestCreateOwnProviderValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		kind  models.ProviderKind
		key   string
		field string
	}{
		{"local servers are not allowed", models.ProviderOpenAICompatible, "k", "kind"},
		{"key is required", models.ProviderOpenAI, "  ", "api_key"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc, _ := newUserAIService(t, true, nil)
			_, err := svc.Create(context.Background(), member, tt.kind, tt.key)
			requireCode(t, err, "VALIDATION_FAILED")
		})
	}
}

func TestCreateOwnProviderOncePerKind(t *testing.T) {
	t.Parallel()
	svc, d := newUserAIService(t, true, nil)
	d.box.EXPECT().Seal(gomock.Any()).Return([]byte("sealed"), nil)
	d.store.EXPECT().CreateProvider(gomock.Any(), gomock.Any()).Return(repositories.ErrDuplicate)

	_, err := svc.Create(context.Background(), member, models.ProviderOpenAI, "sk-1")
	requireCode(t, err, "PROVIDER_KIND_TAKEN")
}

func TestBYOKDisabledBlocksChangesButNotDeletion(t *testing.T) {
	t.Parallel()
	svc, d := newUserAIService(t, false, nil)
	owner := member.UserID
	mp := modelOf(&owner)
	d.store.EXPECT().GetProvider(gomock.Any(), mp.Provider.ID).Return(mp.Provider, nil).AnyTimes()
	d.store.EXPECT().DeleteProvider(gomock.Any(), mp.Provider.ID).Return(nil)
	ctx, on := context.Background(), true

	_, err := svc.Create(ctx, member, models.ProviderOpenAI, "sk-1")
	requireCode(t, err, "BYOK_DISABLED")
	_, err = svc.Update(ctx, member, mp.Provider.ID, services.UserProviderChange{Enabled: &on})
	requireCode(t, err, "BYOK_DISABLED")
	_, err = svc.TestConnection(ctx, member, mp.Provider.ID)
	requireCode(t, err, "BYOK_DISABLED")
	_, err = svc.CreateModel(ctx, member, mp.Provider.ID, "gpt-x", "")
	requireCode(t, err, "BYOK_DISABLED")
	assert.NoError(t, svc.Delete(ctx, member, mp.Provider.ID))
}

func TestOwnProvidersOfOthersAreNotFound(t *testing.T) {
	t.Parallel()
	svc, d := newUserAIService(t, true, nil)
	other := uuid.New()
	foreign, instance := modelOf(&other), modelOf(nil)
	for _, mp := range []repositories.ModelWithProvider{foreign, instance} {
		d.store.EXPECT().GetProvider(gomock.Any(), mp.Provider.ID).Return(mp.Provider, nil).AnyTimes()
		d.store.EXPECT().GetModel(gomock.Any(), mp.Model.ID).Return(mp, nil).AnyTimes()
	}
	ctx, key, on := context.Background(), "sk-new", true

	for _, mp := range []repositories.ModelWithProvider{foreign, instance} {
		_, err := svc.Update(ctx, member, mp.Provider.ID, services.UserProviderChange{APIKey: &key})
		requireCode(t, err, "PROVIDER_NOT_FOUND")
		requireCode(t, svc.Delete(ctx, member, mp.Provider.ID), "PROVIDER_NOT_FOUND")
		_, err = svc.TestConnection(ctx, member, mp.Provider.ID)
		requireCode(t, err, "PROVIDER_NOT_FOUND")
		_, err = svc.CreateModel(ctx, member, mp.Provider.ID, "m", "")
		requireCode(t, err, "PROVIDER_NOT_FOUND")
		_, err = svc.UpdateModel(ctx, member, mp.Model.ID, services.ModelChange{Enabled: &on})
		requireCode(t, err, "MODEL_NOT_FOUND")
		requireCode(t, svc.DeleteModel(ctx, member, mp.Model.ID), "MODEL_NOT_FOUND")
	}
}

func TestUpdateOwnProviderCannotClearKey(t *testing.T) {
	t.Parallel()
	svc, d := newUserAIService(t, true, nil)
	owner := member.UserID
	mp := modelOf(&owner)
	d.store.EXPECT().GetProvider(gomock.Any(), mp.Provider.ID).Return(mp.Provider, nil)
	empty := ""

	_, err := svc.Update(context.Background(), member, mp.Provider.ID, services.UserProviderChange{APIKey: &empty})
	requireCode(t, err, "VALIDATION_FAILED")
}

func TestOwnModelsAreChatOnly(t *testing.T) {
	t.Parallel()
	svc, d := newUserAIService(t, true, nil)
	owner := member.UserID
	mp := modelOf(&owner)
	d.store.EXPECT().GetProvider(gomock.Any(), mp.Provider.ID).Return(mp.Provider, nil)
	d.store.EXPECT().CreateModel(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, m models.AIModel) error {
		assert.Equal(t, models.CapabilityChat, m.Capability)
		assert.Equal(t, "gpt-x", m.DisplayName, "display name defaults to the model id")
		return nil
	})

	_, err := svc.CreateModel(context.Background(), member, mp.Provider.ID, " gpt-x ", "")
	require.NoError(t, err)
}

func TestOwnModelsRejectBlankNames(t *testing.T) {
	t.Parallel()
	owner := member.UserID
	mp := modelOf(&owner)

	t.Run("create with a blank name", func(t *testing.T) {
		t.Parallel()
		svc, d := newUserAIService(t, true, nil)
		d.store.EXPECT().GetProvider(gomock.Any(), mp.Provider.ID).Return(mp.Provider, nil)
		// CreateModel on the store is not expected.
		_, err := svc.CreateModel(context.Background(), member, mp.Provider.ID, "   ", "")
		requireCode(t, err, "VALIDATION_FAILED")
		appErr, _ := asAppErr(err)
		assert.Equal(t, map[string]string{"name": "required"}, appErr.Fields)
	})

	t.Run("update with a blank display name", func(t *testing.T) {
		t.Parallel()
		svc, d := newUserAIService(t, true, nil)
		d.store.EXPECT().GetModel(gomock.Any(), mp.Model.ID).Return(mp, nil)
		// UpdateModel on the store is not expected.
		blank := "  "
		_, err := svc.UpdateModel(context.Background(), member, mp.Model.ID, services.ModelChange{DisplayName: &blank})
		requireCode(t, err, "VALIDATION_FAILED")
		appErr, _ := asAppErr(err)
		assert.Equal(t, map[string]string{"display_name": "required"}, appErr.Fields)
	})
}

// failingProvider rejects every key.
type failingProvider struct{ stubProvider }

func (failingProvider) ListModels(context.Context) ([]string, error) {
	return nil, &ai.APIError{Status: 401, Message: "invalid key"}
}

func TestTestOwnProviderRecordsRejectedKey(t *testing.T) {
	t.Parallel()
	svc, d := newUserAIService(t, true, failingProvider{})
	owner := member.UserID
	mp := modelOf(&owner)
	d.store.EXPECT().GetProvider(gomock.Any(), mp.Provider.ID).Return(mp.Provider, nil)
	d.store.EXPECT().SetProviderCheck(gomock.Any(), mp.Provider.ID, fixedNow, gomock.Not(gomock.Nil())).Return(nil)

	res, err := svc.TestConnection(context.Background(), member, mp.Provider.ID)
	require.NoError(t, err)
	assert.False(t, res.OK)
	assert.Contains(t, res.Error, "rejected the API key")
}

func TestChatModelsIncludeOwnOnlyWhileBYOKIsOn(t *testing.T) {
	t.Parallel()
	owner := member.UserID
	inst, own := modelOf(nil), modelOf(&owner)
	for _, byok := range []bool{true, false} {
		svc, d := newUserAIService(t, byok, nil)
		d.store.EXPECT().ListProviders(gomock.Any()).Return([]models.AIProvider{inst.Provider}, nil)
		d.store.EXPECT().ListModels(gomock.Any()).Return([]models.AIModel{inst.Model}, nil)
		if byok {
			d.store.EXPECT().ListUserProviders(gomock.Any(), owner).Return([]models.AIProvider{own.Provider}, nil)
			d.store.EXPECT().ListUserModels(gomock.Any(), owner).Return([]models.AIModel{own.Model}, nil)
		}

		options, err := svc.ChatModels(context.Background(), member)
		require.NoError(t, err)
		if byok {
			require.Len(t, options, 2)
			assert.False(t, options[0].Own)
			assert.True(t, options[1].Own)
		} else {
			require.Len(t, options, 1)
		}
	}
}

func TestListOwnProvidersPropagatesStoreErrors(t *testing.T) {
	t.Parallel()
	svc, d := newUserAIService(t, true, nil)
	boom := errors.New("db down")
	d.store.EXPECT().ListUserProviders(gomock.Any(), member.UserID).Return(nil, boom)

	_, err := svc.List(context.Background(), member)
	assert.ErrorIs(t, err, boom)
}
