package services_test

import (
	"context"
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

// inlineTx runs the function without a real transaction.
type inlineTx struct{}

func (inlineTx) WithinTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

var fixedNow = time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)

type authDeps struct {
	users    *mocks.MockUserStore
	tokens   *mocks.MockUserTokenStore
	sessions *mocks.MockSessionStore
	settings *mocks.MockSetupStore
	prefs    *mocks.MockPreferenceSeeder
	issuer   *mocks.MockAccessIssuer
	limiter  *mocks.MockLimiter
	resend   *mocks.MockLimiter
	emails   *mocks.MockEmailQueue
	reset    *mocks.MockLimiter
	queue    *mocks.MockEnqueuer
}

func newAuthService(t *testing.T) (*services.AuthService, authDeps) {
	ctrl := gomock.NewController(t)
	d := authDeps{
		users: mocks.NewMockUserStore(ctrl), tokens: mocks.NewMockUserTokenStore(ctrl),
		sessions: mocks.NewMockSessionStore(ctrl), settings: mocks.NewMockSetupStore(ctrl),
		prefs:  mocks.NewMockPreferenceSeeder(ctrl),
		issuer: mocks.NewMockAccessIssuer(ctrl), limiter: mocks.NewMockLimiter(ctrl),
		resend: mocks.NewMockLimiter(ctrl), emails: mocks.NewMockEmailQueue(ctrl),
		reset: mocks.NewMockLimiter(ctrl), queue: mocks.NewMockEnqueuer(ctrl),
	}
	svc := services.NewAuthService(inlineTx{}, d.users, d.tokens, d.sessions, d.settings, d.prefs, d.issuer, d.limiter,
		d.resend, d.reset, d.emails, d.queue, "https://newsp.test", 30*24*time.Hour, func() time.Time { return fixedNow })
	return svc, d
}

func asAppErr(err error) (*apperrors.Error, bool) { return apperrors.As(err) }

func requireCode(t *testing.T, err error, code string) {
	t.Helper()
	appErr, ok := apperrors.As(err)
	require.True(t, ok, "expected app error, got %v", err)
	assert.Equal(t, code, appErr.Code)
}

func TestLoginDoesNotRevealUnknownEmail(t *testing.T) {
	t.Parallel()
	svc, d := newAuthService(t)
	d.limiter.EXPECT().Allow("login:ghost@example.com").Return(true, time.Duration(0))
	d.users.EXPECT().GetByEmail(gomock.Any(), "ghost@example.com").Return(models.User{}, repositories.ErrNotFound)

	_, err := svc.Login(context.Background(), " Ghost@Example.com ", "whatever-password")
	requireCode(t, err, "INVALID_CREDENTIALS")
}

func TestLoginRejectsDisabledUserWithSameError(t *testing.T) {
	t.Parallel()
	svc, d := newAuthService(t)
	hash, err := auth.HashPassword("correct horse battery")
	require.NoError(t, err)
	d.limiter.EXPECT().Allow(gomock.Any()).Return(true, time.Duration(0))
	d.users.EXPECT().GetByEmail(gomock.Any(), "ana@example.com").
		Return(models.User{ID: uuid.New(), PasswordHash: hash, Status: models.UserDisabled}, nil)

	_, err = svc.Login(context.Background(), "ana@example.com", "correct horse battery")
	requireCode(t, err, "INVALID_CREDENTIALS")
}

func TestLoginRateLimited(t *testing.T) {
	t.Parallel()
	svc, d := newAuthService(t)
	d.limiter.EXPECT().Allow(gomock.Any()).Return(false, 30*time.Second)

	_, err := svc.Login(context.Background(), "ana@example.com", "x")
	appErr, ok := apperrors.As(err)
	require.True(t, ok)
	assert.Equal(t, apperrors.KindRateLimited, appErr.Kind)
	assert.Equal(t, 30*time.Second, appErr.RetryAfter)
}

func TestRefreshReuseRevokesFamily(t *testing.T) {
	t.Parallel()
	svc, d := newAuthService(t)
	rotated := fixedNow.Add(-time.Minute)
	family := uuid.New()
	d.sessions.EXPECT().GetByHashForUpdate(gomock.Any(), auth.HashToken("stolen")).
		Return(models.RefreshToken{ID: uuid.New(), FamilyID: family, ExpiresAt: fixedNow.Add(time.Hour), RotatedAt: &rotated}, nil)
	d.sessions.EXPECT().RevokeFamily(gomock.Any(), family, fixedNow).Return(nil)

	_, err := svc.Refresh(context.Background(), "stolen")
	requireCode(t, err, "SESSION_INVALID")
}

func TestRefreshRejectsExpiredAndRevoked(t *testing.T) {
	t.Parallel()
	revokedAt := fixedNow.Add(-time.Hour)
	tests := []struct {
		name  string
		token models.RefreshToken
	}{
		{name: "expired", token: models.RefreshToken{ExpiresAt: fixedNow}},
		{name: "revoked", token: models.RefreshToken{ExpiresAt: fixedNow.Add(time.Hour), RevokedAt: &revokedAt}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc, d := newAuthService(t)
			d.sessions.EXPECT().GetByHashForUpdate(gomock.Any(), gomock.Any()).Return(tt.token, nil)
			_, err := svc.Refresh(context.Background(), "token")
			requireCode(t, err, "SESSION_INVALID")
		})
	}
}

func TestRefreshRotatesWithinFamily(t *testing.T) {
	t.Parallel()
	svc, d := newAuthService(t)
	user := models.User{ID: uuid.New(), Role: models.RoleMember, Status: models.UserActive}
	current := models.RefreshToken{ID: uuid.New(), UserID: user.ID, FamilyID: uuid.New(), ExpiresAt: fixedNow.Add(time.Hour)}

	d.sessions.EXPECT().GetByHashForUpdate(gomock.Any(), gomock.Any()).Return(current, nil)
	d.users.EXPECT().GetByID(gomock.Any(), user.ID).Return(user, nil)
	d.sessions.EXPECT().MarkRotated(gomock.Any(), current.ID, fixedNow).Return(nil)
	d.users.EXPECT().TouchLastSeen(gomock.Any(), user.ID, fixedNow).Return(nil)
	d.issuer.EXPECT().Issue(user.ID, models.RoleMember).Return("access", fixedNow.Add(15*time.Minute), nil)
	d.sessions.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, tok models.RefreshToken) error {
		assert.Equal(t, current.FamilyID, tok.FamilyID, "rotation keeps the family")
		return nil
	})

	session, err := svc.Refresh(context.Background(), "token")
	require.NoError(t, err)
	assert.Equal(t, "access", session.AccessToken)
	assert.NotEmpty(t, session.RefreshToken)
}

func TestRefreshForDisabledUserRevokesFamily(t *testing.T) {
	t.Parallel()
	svc, d := newAuthService(t)
	userID := uuid.New()
	current := models.RefreshToken{ID: uuid.New(), UserID: userID, FamilyID: uuid.New(), ExpiresAt: fixedNow.Add(time.Hour)}
	d.sessions.EXPECT().GetByHashForUpdate(gomock.Any(), gomock.Any()).Return(current, nil)
	d.users.EXPECT().GetByID(gomock.Any(), userID).Return(models.User{ID: userID, Status: models.UserDisabled}, nil)
	d.sessions.EXPECT().RevokeFamily(gomock.Any(), current.FamilyID, fixedNow).Return(nil)

	_, err := svc.Refresh(context.Background(), "token")
	requireCode(t, err, "SESSION_INVALID")
}

func TestSetupOnlyOnce(t *testing.T) {
	t.Parallel()
	svc, d := newAuthService(t)
	done := fixedNow
	d.settings.EXPECT().GetForUpdate(gomock.Any()).Return(models.InstanceSettings{SetupCompletedAt: &done}, nil)

	_, err := svc.Setup(context.Background(), services.NewAccount{Name: "A", Email: "a@b.c", Password: "correct horse battery"})
	requireCode(t, err, "SETUP_ALREADY_DONE")
}

func TestSetupRejectsWeakPassword(t *testing.T) {
	t.Parallel()
	svc, _ := newAuthService(t)
	_, err := svc.Setup(context.Background(), services.NewAccount{Name: "A", Email: "a@b.c", Password: "short"})
	requireCode(t, err, "VALIDATION_FAILED")
}

func TestAcceptInviteInvalidLink(t *testing.T) {
	t.Parallel()
	svc, d := newAuthService(t)
	d.tokens.EXPECT().GetUsableForUpdate(gomock.Any(), models.TokenInvite, auth.HashToken("used"), fixedNow).
		Return(models.UserToken{}, repositories.ErrNotFound)

	_, err := svc.AcceptInvite(context.Background(), "used", services.NewAccount{Name: "A", Email: "a@b.c", Password: "correct horse battery"})
	requireCode(t, err, "INVALID_LINK")
}

func TestAcceptInviteBoundToEmail(t *testing.T) {
	t.Parallel()
	invite := models.UserToken{ID: uuid.New(), Kind: models.TokenInvite, Role: ptr(models.RoleMember), Email: ptr("ana@example.com")}
	in := func(email string) services.NewAccount {
		return services.NewAccount{Name: "Ana", Email: email, Password: "correct horse battery"}
	}

	t.Run("another e-mail is refused", func(t *testing.T) {
		t.Parallel()
		svc, d := newAuthService(t)
		d.tokens.EXPECT().GetUsableForUpdate(gomock.Any(), models.TokenInvite, auth.HashToken("tok"), fixedNow).Return(invite, nil)

		_, err := svc.AcceptInvite(context.Background(), "tok", in("other@example.com"))
		requireCode(t, err, "VALIDATION_FAILED")
		appErr, _ := asAppErr(err)
		assert.Equal(t, map[string]string{"email": "invite_mismatch"}, appErr.Fields)
	})

	t.Run("same e-mail with other case and spaces is accepted", func(t *testing.T) {
		t.Parallel()
		svc, d := newAuthService(t)
		d.tokens.EXPECT().GetUsableForUpdate(gomock.Any(), models.TokenInvite, auth.HashToken("tok"), fixedNow).Return(invite, nil)
		d.users.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
		d.prefs.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
		d.tokens.EXPECT().MarkUsed(gomock.Any(), invite.ID, fixedNow).Return(nil)
		d.issuer.EXPECT().Issue(gomock.Any(), models.RoleMember).Return("access", fixedNow.Add(time.Hour), nil)
		d.sessions.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

		_, err := svc.AcceptInvite(context.Background(), "tok", in("  Ana@Example.COM "))
		require.NoError(t, err)
	})
}
