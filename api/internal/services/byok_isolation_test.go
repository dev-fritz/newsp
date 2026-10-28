package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/dev-fritz/newsp/api/internal/models"
	"github.com/dev-fritz/newsp/api/internal/services"
	"github.com/dev-fritz/newsp/api/pkg/ai"
	"github.com/dev-fritz/newsp/api/tests/mocks"
)

// stubProvider answers every call with an empty success.
type stubProvider struct{}

func (stubProvider) Chat(context.Context, ai.ChatRequest) (ai.ChatResponse, error) {
	return ai.ChatResponse{}, nil
}

func (stubProvider) Embed(context.Context, ai.EmbedRequest) (ai.EmbedResponse, error) {
	return ai.EmbedResponse{}, nil
}

func (stubProvider) ListModels(context.Context) ([]string, error) { return nil, nil }

func TestGatewayServesBYOKModelsOnlyToTheirOwner(t *testing.T) {
	t.Parallel()
	owner, other := uuid.New(), uuid.New()
	tests := []struct {
		name   string
		caller *uuid.UUID
		byok   bool
		wantOK bool
	}{
		{"owner while BYOK is on", &owner, true, true},
		{"owner while BYOK is off", &owner, false, false},
		{"another user", &other, true, false},
		{"no user (system call)", nil, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			store, settings := mocks.NewMockAIStore(ctrl), mocks.NewMockSettingsStore(ctrl)
			mp := modelOf(&owner)
			store.EXPECT().GetModel(gomock.Any(), mp.Model.ID).Return(mp, nil)
			settings.EXPECT().Get(gomock.Any()).Return(models.InstanceSettings{BYOKEnabled: tt.byok}, nil).AnyTimes()
			gw := services.NewAIGateway(store, settings, nil,
				func(ai.Config) (ai.Provider, error) { return stubProvider{}, nil }, func() time.Time { return fixedNow })

			_, _, err := gw.Chat(context.Background(), mp.Model.ID, models.PurposeReport, tt.caller, ai.ChatRequest{})
			if tt.wantOK {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, services.ErrModelUnavailable)
			}
		})
	}
}

func TestAdminCannotReachBYOKProviders(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	store := mocks.NewMockAIStore(ctrl)
	svc := services.NewAIAdminService(inlineTx{}, store, mocks.NewMockSettingsStore(ctrl), nil, nil, nil, true, func() time.Time { return fixedNow })
	owner := uuid.New()
	mp := modelOf(&owner)
	store.EXPECT().GetProvider(gomock.Any(), mp.Provider.ID).Return(mp.Provider, nil).AnyTimes()
	store.EXPECT().GetModel(gomock.Any(), mp.Model.ID).Return(mp, nil).AnyTimes()
	ctx, name, on := context.Background(), "x", true

	_, err := svc.UpdateProvider(ctx, adminActor, mp.Provider.ID, services.ProviderChange{Name: &name})
	requireCode(t, err, "PROVIDER_NOT_FOUND")
	requireCode(t, svc.DeleteProvider(ctx, adminActor, mp.Provider.ID), "PROVIDER_NOT_FOUND")
	_, err = svc.TestConnection(ctx, adminActor, mp.Provider.ID)
	requireCode(t, err, "PROVIDER_NOT_FOUND")
	_, err = svc.CreateModel(ctx, adminActor, mp.Provider.ID, services.ModelInput{Name: "m", Capability: models.CapabilityChat})
	requireCode(t, err, "PROVIDER_NOT_FOUND")
	_, err = svc.UpdateModel(ctx, adminActor, mp.Model.ID, services.ModelChange{Enabled: &on})
	requireCode(t, err, "MODEL_NOT_FOUND")
	requireCode(t, svc.DeleteModel(ctx, adminActor, mp.Model.ID), "MODEL_NOT_FOUND")
}

func TestInstanceDefaultsRefuseBYOKModels(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	store, settings := mocks.NewMockAIStore(ctrl), mocks.NewMockSettingsStore(ctrl)
	svc := services.NewAIAdminService(inlineTx{}, store, settings, nil, nil, nil, true, func() time.Time { return fixedNow })
	owner := uuid.New()
	mp := modelOf(&owner)
	settings.EXPECT().GetForUpdate(gomock.Any()).Return(models.InstanceSettings{}, nil)
	store.EXPECT().GetModel(gomock.Any(), mp.Model.ID).Return(mp, nil)

	_, err := svc.UpdateSettings(context.Background(), adminActor, services.SettingsChange{DefaultReportModelID: &mp.Model.ID})
	requireCode(t, err, "VALIDATION_FAILED")
}

func TestBYOKToggleIsStored(t *testing.T) {
	t.Parallel()
	svc, settings := newSettingsService(t, true)
	settings.EXPECT().GetForUpdate(gomock.Any()).Return(models.InstanceSettings{BYOKEnabled: true}, nil)
	settings.EXPECT().UpdateAI(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, s models.InstanceSettings) error {
		assert.False(t, s.BYOKEnabled)
		return nil
	})
	settings.EXPECT().UpdateAccounts(gomock.Any(), gomock.Any()).Return(nil)
	off := false

	res, err := svc.UpdateSettings(context.Background(), adminActor, services.SettingsChange{BYOKEnabled: &off})
	require.NoError(t, err)
	assert.False(t, res.Settings.BYOKEnabled)
}
