package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/dev-fritz/newsp/api/internal/auth"
	"github.com/dev-fritz/newsp/api/internal/models"
	"github.com/dev-fritz/newsp/api/internal/services"
	"github.com/dev-fritz/newsp/api/tests/mocks"
)

func newSettingsService(t *testing.T, mailReady bool) (*services.AIAdminService, *mocks.MockSettingsStore) {
	t.Helper()
	ctrl := gomock.NewController(t)
	settings := mocks.NewMockSettingsStore(ctrl)
	svc := services.NewAIAdminService(inlineTx{}, mocks.NewMockAIStore(ctrl), settings, nil, nil, mocks.NewMockEnqueuer(ctrl),
		mailReady, func() time.Time { return fixedNow })
	return svc, settings
}

var adminActor = auth.Identity{UserID: uuid.New(), Role: models.RoleAdmin}

func TestSignupNeedsEmailDelivery(t *testing.T) {
	t.Parallel()
	svc, settings := newSettingsService(t, false)
	settings.EXPECT().GetForUpdate(gomock.Any()).Return(models.InstanceSettings{}, nil)
	on := true

	_, err := svc.UpdateSettings(context.Background(), adminActor, services.SettingsChange{SignupEnabled: &on})
	requireCode(t, err, "MAILER_NOT_CONFIGURED")
}

func TestAccountSettingsAreStored(t *testing.T) {
	t.Parallel()
	svc, settings := newSettingsService(t, true)
	settings.EXPECT().GetForUpdate(gomock.Any()).Return(models.InstanceSettings{InactivityPauseDays: 30}, nil)
	settings.EXPECT().UpdateAI(gomock.Any(), gomock.Any()).Return(nil)
	settings.EXPECT().UpdateAccounts(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, s models.InstanceSettings) error {
		assert.True(t, s.SignupEnabled)
		assert.Equal(t, 0, s.InactivityPauseDays)
		return nil
	})
	on, days := true, 0

	res, err := svc.UpdateSettings(context.Background(), adminActor, services.SettingsChange{SignupEnabled: &on, InactivityPauseDays: &days})
	require.NoError(t, err)
	assert.True(t, res.Settings.SignupEnabled)
}

func TestTurningSignupOffWorksWithoutEmail(t *testing.T) {
	t.Parallel()
	svc, settings := newSettingsService(t, false)
	settings.EXPECT().GetForUpdate(gomock.Any()).Return(models.InstanceSettings{SignupEnabled: true}, nil)
	settings.EXPECT().UpdateAI(gomock.Any(), gomock.Any()).Return(nil)
	settings.EXPECT().UpdateAccounts(gomock.Any(), gomock.Any()).Return(nil)
	off := false

	res, err := svc.UpdateSettings(context.Background(), adminActor, services.SettingsChange{SignupEnabled: &off})
	require.NoError(t, err)
	assert.False(t, res.Settings.SignupEnabled)
}
