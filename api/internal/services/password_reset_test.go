package services_test

import (
	"context"
	"encoding/json"
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
	"github.com/dev-fritz/newsp/api/internal/workers"
)

func resetJob(t *testing.T, email string) models.Job {
	t.Helper()
	raw, err := json.Marshal(map[string]string{"email": email})
	require.NoError(t, err)
	return models.Job{ID: uuid.New(), Kind: services.JobPasswordReset, Payload: raw}
}

func TestRequestPasswordResetQueuesWithoutLookingUpTheUser(t *testing.T) {
	t.Parallel()
	svc, d := newAuthService(t)
	d.reset.EXPECT().Allow("reset:ana@example.com").Return(true, time.Duration(0))
	d.emails.EXPECT().Ready().Return(true)
	// No EXPECT on d.users: any lookup fails the test.
	d.queue.EXPECT().Enqueue(gomock.Any(), services.JobPasswordReset, gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, payload any, opts workers.EnqueueOptions) (uuid.UUID, error) {
			raw, err := json.Marshal(payload)
			require.NoError(t, err)
			assert.JSONEq(t, `{"email":"ana@example.com"}`, string(raw))
			assert.Equal(t, "auth.password_reset:ana@example.com", opts.DedupeKey)
			return uuid.New(), nil
		})

	require.NoError(t, svc.RequestPasswordReset(context.Background(), " Ana@Example.com "))
}

func TestRequestPasswordResetDoesNothingWithoutMailer(t *testing.T) {
	t.Parallel()
	svc, d := newAuthService(t)
	d.reset.EXPECT().Allow("reset:ana@example.com").Return(true, time.Duration(0))
	d.emails.EXPECT().Ready().Return(false)
	// No EXPECT on d.queue: any Enqueue fails the test.

	require.NoError(t, svc.RequestPasswordReset(context.Background(), "ana@example.com"))
}

func TestRequestPasswordResetIsRateLimited(t *testing.T) {
	t.Parallel()
	svc, d := newAuthService(t)
	d.reset.EXPECT().Allow("reset:ana@example.com").Return(false, time.Minute)

	err := svc.RequestPasswordReset(context.Background(), "ana@example.com")
	requireCode(t, err, "RATE_LIMITED")
}

func TestSendPasswordResetSkipsUnknownEmail(t *testing.T) {
	t.Parallel()
	svc, d := newAuthService(t)
	d.users.EXPECT().GetByEmail(gomock.Any(), "ghost@example.com").Return(models.User{}, repositories.ErrNotFound)

	res, err := svc.SendPasswordReset(context.Background(), resetJob(t, "ghost@example.com"))
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"skipped": "no active account"}, res)
}

func TestSendPasswordResetSkipsDisabledUser(t *testing.T) {
	t.Parallel()
	svc, d := newAuthService(t)
	d.users.EXPECT().GetByEmail(gomock.Any(), "ana@example.com").
		Return(models.User{ID: uuid.New(), Email: "ana@example.com", Status: models.UserDisabled}, nil)

	res, err := svc.SendPasswordReset(context.Background(), resetJob(t, "ana@example.com"))
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"skipped": "no active account"}, res)
}

func TestSendPasswordResetCreatesTokenAndQueuesEmail(t *testing.T) {
	t.Parallel()
	svc, d := newAuthService(t)
	user := models.User{ID: uuid.New(), Email: "ana@example.com", Name: "Ana", Locale: "en", Status: models.UserActive}
	d.users.EXPECT().GetByEmail(gomock.Any(), "ana@example.com").Return(user, nil)

	var token models.UserToken
	gomock.InOrder(
		d.tokens.EXPECT().RevokePendingSelfResets(gomock.Any(), user.ID, fixedNow).Return(nil),
		d.tokens.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, t models.UserToken) error {
			token = t
			return nil
		}),
		d.emails.EXPECT().Enqueue(gomock.Any(), &user.ID, gomock.Any()).DoAndReturn(
			func(_ context.Context, _ *uuid.UUID, e services.Email) error {
				assert.Equal(t, services.EmailPasswordReset, e.Kind)
				assert.Equal(t, "ana@example.com", e.To)
				assert.Equal(t, "Ana", e.Name)
				assert.Equal(t, "en", e.Locale)
				assert.Equal(t, time.Hour, e.ValidFor)
				const prefix = "https://newsp.test/reset-password/"
				require.Greater(t, len(e.Link), len(prefix))
				assert.Equal(t, prefix, e.Link[:len(prefix)])
				assert.Equal(t, token.TokenHash, auth.HashToken(e.Link[len(prefix):]), "the link carries the stored token")
				return nil
			}),
	)

	res, err := svc.SendPasswordReset(context.Background(), resetJob(t, "ana@example.com"))
	require.NoError(t, err)
	assert.NotContains(t, res, "email")
	assert.Equal(t, models.TokenPasswordReset, token.Kind)
	assert.Equal(t, &user.ID, token.UserID)
	assert.Nil(t, token.CreatedBy)
	assert.Equal(t, fixedNow.Add(time.Hour), token.ExpiresAt)
}

func TestSendPasswordResetRejectsBadPayloadPermanently(t *testing.T) {
	t.Parallel()
	svc, _ := newAuthService(t)

	for name, payload := range map[string]string{"malformed": `{"email":`, "no e-mail": `{}`} {
		_, err := svc.SendPasswordReset(context.Background(), models.Job{Kind: services.JobPasswordReset, Payload: []byte(payload)})
		require.Error(t, err, name)
		assert.True(t, workers.IsPermanent(err), name)
	}
}
