package services_test

import (
	"context"
	"strings"
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
)

func openSignup() models.InstanceSettings {
	done := fixedNow.Add(-time.Hour)
	return models.InstanceSettings{SignupEnabled: true, SetupCompletedAt: &done}
}

var newcomer = services.NewAccount{Name: "Bia", Email: " Bia@Example.com ", Password: "correct horse battery"}

func TestSignupIsRefusedUnlessOpen(t *testing.T) {
	t.Parallel()
	done := fixedNow
	cases := map[string]struct {
		settings  models.InstanceSettings
		mailReady bool
	}{
		"turned off":     {models.InstanceSettings{SetupCompletedAt: &done}, true},
		"before setup":   {models.InstanceSettings{SignupEnabled: true}, true},
		"without e-mail": {openSignup(), false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			svc, d := newAuthService(t)
			d.settings.EXPECT().Get(gomock.Any()).Return(tc.settings, nil)
			d.emails.EXPECT().Ready().Return(tc.mailReady).AnyTimes()

			_, err := svc.Signup(context.Background(), newcomer, "")
			requireCode(t, err, "SIGNUP_DISABLED")
		})
	}
}

func TestSignupCreatesUnverifiedMemberAndSendsLink(t *testing.T) {
	t.Parallel()
	svc, d := newAuthService(t)
	d.settings.EXPECT().Get(gomock.Any()).Return(openSignup(), nil)
	d.emails.EXPECT().Ready().Return(true)

	var created models.User
	d.users.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, u models.User) error {
		created = u
		return nil
	})
	d.prefs.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
	var token models.UserToken
	d.tokens.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, tk models.UserToken) error {
		token = tk
		return nil
	})
	var email services.Email
	d.emails.EXPECT().Enqueue(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, uid *uuid.UUID, e services.Email) error {
		assert.Equal(t, created.ID, *uid)
		email = e
		return nil
	})
	d.users.EXPECT().TouchLastSeen(gomock.Any(), gomock.Any(), fixedNow).Return(nil)
	d.issuer.EXPECT().Issue(gomock.Any(), models.RoleMember).Return("access", fixedNow.Add(time.Minute), nil)
	d.sessions.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	session, err := svc.Signup(context.Background(), newcomer, "en")
	require.NoError(t, err)

	assert.Equal(t, "bia@example.com", created.Email)
	assert.Equal(t, models.RoleMember, created.Role)
	assert.Equal(t, "en", created.Locale)
	assert.Nil(t, created.EmailVerifiedAt)
	assert.Equal(t, created.ID, session.User.ID)

	assert.Equal(t, models.TokenEmailVerification, token.Kind)
	assert.Equal(t, created.ID, *token.UserID)
	assert.Equal(t, "bia@example.com", *token.Email)
	assert.Equal(t, fixedNow.Add(24*time.Hour), token.ExpiresAt)

	assert.Equal(t, services.EmailVerification, email.Kind)
	assert.Equal(t, "bia@example.com", email.To)
	assert.Equal(t, "en", email.Locale)
	assert.Equal(t, 24*time.Hour, email.ValidFor)
	require.True(t, strings.HasPrefix(email.Link, "https://newsp.test/verify-email/"))
	plain := strings.TrimPrefix(email.Link, "https://newsp.test/verify-email/")
	assert.Equal(t, auth.HashToken(plain), token.TokenHash, "the e-mailed link matches the stored hash")
}

func TestSignupIgnoresUnknownLocale(t *testing.T) {
	t.Parallel()
	svc, d := newAuthService(t)
	d.settings.EXPECT().Get(gomock.Any()).Return(openSignup(), nil)
	d.emails.EXPECT().Ready().Return(true)
	d.users.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, u models.User) error {
		assert.Equal(t, "pt-BR", u.Locale)
		return repositories.ErrDuplicate
	})

	_, err := svc.Signup(context.Background(), newcomer, "fr")
	requireCode(t, err, "EMAIL_TAKEN")
}

func TestVerifyEmail(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	address := "bia@example.com"
	other := "old@example.com"
	cases := map[string]struct {
		user    models.User
		email   *string
		wantErr string
	}{
		"verifies":      {models.User{ID: userID, Email: address, Status: models.UserActive}, &address, ""},
		"disabled user": {models.User{ID: userID, Email: address, Status: models.UserDisabled}, &address, "INVALID_LINK"},
		"other address": {models.User{ID: userID, Email: address, Status: models.UserActive}, &other, "INVALID_LINK"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			svc, d := newAuthService(t)
			tokenID := uuid.New()
			d.tokens.EXPECT().GetUsableForUpdate(gomock.Any(), models.TokenEmailVerification, auth.HashToken("tok"), fixedNow).
				Return(models.UserToken{ID: tokenID, UserID: &userID, Email: tc.email}, nil)
			d.users.EXPECT().GetByIDForUpdate(gomock.Any(), userID).Return(tc.user, nil)
			if tc.wantErr == "" {
				d.users.EXPECT().MarkEmailVerified(gomock.Any(), userID, fixedNow).Return(nil)
				d.tokens.EXPECT().MarkUsed(gomock.Any(), tokenID, fixedNow).Return(nil)
			}

			err := svc.VerifyEmail(context.Background(), "tok")
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			requireCode(t, err, tc.wantErr)
		})
	}
}

func TestVerifyEmailInvalidLink(t *testing.T) {
	t.Parallel()
	svc, d := newAuthService(t)
	d.tokens.EXPECT().GetUsableForUpdate(gomock.Any(), models.TokenEmailVerification, gomock.Any(), fixedNow).
		Return(models.UserToken{}, repositories.ErrNotFound)

	requireCode(t, svc.VerifyEmail(context.Background(), "used"), "INVALID_LINK")
}

func TestResendVerification(t *testing.T) {
	t.Parallel()
	verified := fixedNow
	userID := uuid.New()
	id := auth.Identity{UserID: userID, Role: models.RoleMember}

	t.Run("already verified", func(t *testing.T) {
		t.Parallel()
		svc, d := newAuthService(t)
		d.users.EXPECT().GetByID(gomock.Any(), userID).
			Return(models.User{ID: userID, Status: models.UserActive, EmailVerifiedAt: &verified}, nil)
		requireCode(t, svc.ResendVerification(context.Background(), id), "EMAIL_ALREADY_VERIFIED")
	})
	t.Run("rate limited", func(t *testing.T) {
		t.Parallel()
		svc, d := newAuthService(t)
		d.users.EXPECT().GetByID(gomock.Any(), userID).Return(models.User{ID: userID, Status: models.UserActive}, nil)
		d.resend.EXPECT().Allow("verify:"+userID.String()).Return(false, time.Minute)
		requireCode(t, svc.ResendVerification(context.Background(), id), "RATE_LIMITED")
	})
	t.Run("sends a new link", func(t *testing.T) {
		t.Parallel()
		svc, d := newAuthService(t)
		d.users.EXPECT().GetByID(gomock.Any(), userID).
			Return(models.User{ID: userID, Email: "bia@example.com", Locale: "pt-BR", Status: models.UserActive}, nil)
		d.resend.EXPECT().Allow(gomock.Any()).Return(true, time.Duration(0))
		d.tokens.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
		d.emails.EXPECT().Enqueue(gomock.Any(), &userID, gomock.Any()).Return(nil)
		require.NoError(t, svc.ResendVerification(context.Background(), id))
	})
}
