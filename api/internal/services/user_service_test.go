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
	"github.com/dev-fritz/newsp/api/tests/mocks"
)

func newUserService(t *testing.T) (*services.UserService, *mocks.MockUserStore, *mocks.MockUserTokenStore, *mocks.MockSessionStore) {
	svc, users, tokens, sessions, _ := newUserServiceWithEmails(t)
	return svc, users, tokens, sessions
}

func newUserServiceWithEmails(t *testing.T) (*services.UserService, *mocks.MockUserStore, *mocks.MockUserTokenStore, *mocks.MockSessionStore, *mocks.MockEmailQueue) {
	ctrl := gomock.NewController(t)
	users, tokens, sessions := mocks.NewMockUserStore(ctrl), mocks.NewMockUserTokenStore(ctrl), mocks.NewMockSessionStore(ctrl)
	emails := mocks.NewMockEmailQueue(ctrl)
	svc := services.NewUserService(inlineTx{}, users, tokens, sessions, emails, "https://newsp.example.com", func() time.Time { return fixedNow })
	return svc, users, tokens, sessions, emails
}

func ptr[T any](v T) *T { return &v }

func TestUpdateAccessRules(t *testing.T) {
	t.Parallel()
	admin := auth.Identity{UserID: uuid.New(), Role: models.RoleAdmin}
	member := auth.Identity{UserID: uuid.New(), Role: models.RoleMember}

	t.Run("member is forbidden", func(t *testing.T) {
		t.Parallel()
		svc, _, _, _ := newUserService(t)
		_, err := svc.UpdateAccess(context.Background(), member, uuid.New(), services.UserAccessChange{})
		requireCode(t, err, "FORBIDDEN")
	})

	t.Run("cannot change own access", func(t *testing.T) {
		t.Parallel()
		svc, _, _, _ := newUserService(t)
		_, err := svc.UpdateAccess(context.Background(), admin, admin.UserID, services.UserAccessChange{Status: ptr(models.UserDisabled)})
		requireCode(t, err, "CANNOT_CHANGE_OWN_ACCESS")
	})

	t.Run("keeps the last admin", func(t *testing.T) {
		t.Parallel()
		svc, users, _, _ := newUserService(t)
		target := models.User{ID: uuid.New(), Role: models.RoleAdmin, Status: models.UserActive}
		users.EXPECT().LockActiveAdmins(gomock.Any()).Return([]uuid.UUID{target.ID}, nil)
		users.EXPECT().GetByIDForUpdate(gomock.Any(), target.ID).Return(target, nil)

		_, err := svc.UpdateAccess(context.Background(), admin, target.ID, services.UserAccessChange{Role: ptr(models.RoleMember)})
		requireCode(t, err, "LAST_ADMIN")
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		svc, users, _, _ := newUserService(t)
		id := uuid.New()
		users.EXPECT().LockActiveAdmins(gomock.Any()).Return(nil, nil)
		users.EXPECT().GetByIDForUpdate(gomock.Any(), id).Return(models.User{}, repositories.ErrNotFound)

		_, err := svc.UpdateAccess(context.Background(), admin, id, services.UserAccessChange{})
		requireCode(t, err, "USER_NOT_FOUND")
	})

	t.Run("disabling revokes sessions", func(t *testing.T) {
		t.Parallel()
		svc, users, _, sessions := newUserService(t)
		target := models.User{ID: uuid.New(), Role: models.RoleMember, Status: models.UserActive}
		users.EXPECT().LockActiveAdmins(gomock.Any()).Return([]uuid.UUID{admin.UserID}, nil)
		users.EXPECT().GetByIDForUpdate(gomock.Any(), target.ID).Return(target, nil)
		users.EXPECT().UpdateAccess(gomock.Any(), target.ID, models.RoleMember, models.UserDisabled, fixedNow).Return(nil)
		sessions.EXPECT().RevokeAllForUser(gomock.Any(), target.ID, fixedNow).Return(nil)

		got, err := svc.UpdateAccess(context.Background(), admin, target.ID, services.UserAccessChange{Status: ptr(models.UserDisabled)})
		require.NoError(t, err)
		assert.Equal(t, models.UserDisabled, got.Status)
	})
}

func TestCreateInviteBuildsOneTimeLink(t *testing.T) {
	t.Parallel()
	svc, _, tokens, _ := newUserService(t)
	admin := auth.Identity{UserID: uuid.New(), Role: models.RoleAdmin}

	var stored models.UserToken
	tokens.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, tok models.UserToken) error {
		stored = tok
		return nil
	})

	link, err := svc.CreateInvite(context.Background(), admin, models.RoleMember, "")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(link.URL, "https://newsp.example.com/invite/"))
	plain := strings.TrimPrefix(link.URL, "https://newsp.example.com/invite/")
	assert.Equal(t, auth.HashToken(plain), stored.TokenHash, "only the hash is stored")
	assert.Equal(t, fixedNow.Add(7*24*time.Hour), link.ExpiresAt)
	assert.Equal(t, models.RoleMember, *stored.Role)
	assert.Nil(t, stored.Email)
	assert.Nil(t, link.Email)
}

func TestCreateInviteWithEmailQueuesIt(t *testing.T) {
	t.Parallel()
	svc, users, tokens, _, emails := newUserServiceWithEmails(t)
	admin := auth.Identity{UserID: uuid.New(), Role: models.RoleAdmin}

	emails.EXPECT().Ready().Return(true)
	users.EXPECT().GetByEmail(gomock.Any(), "ana@example.com").Return(models.User{}, repositories.ErrNotFound)
	users.EXPECT().GetByID(gomock.Any(), admin.UserID).Return(models.User{ID: admin.UserID, Locale: "en"}, nil)
	var stored models.UserToken
	tokens.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, tok models.UserToken) error {
		stored = tok
		return nil
	})
	var sent services.Email
	emails.EXPECT().Enqueue(gomock.Any(), nil, gomock.Any()).DoAndReturn(func(_ context.Context, _ *uuid.UUID, e services.Email) error {
		sent = e
		return nil
	})

	link, err := svc.CreateInvite(context.Background(), admin, models.RoleMember, "  Ana@Example.com ")
	require.NoError(t, err)
	require.NotNil(t, stored.Email)
	assert.Equal(t, "ana@example.com", *stored.Email)
	require.NotNil(t, link.Email)
	assert.Equal(t, "ana@example.com", *link.Email)
	assert.Equal(t, services.EmailInvite, sent.Kind)
	assert.Equal(t, "ana@example.com", sent.To)
	assert.Equal(t, "en", sent.Locale, "the e-mail speaks the inviting admin's language")
	assert.Equal(t, link.URL, sent.Link)
	assert.Equal(t, 7*24*time.Hour, sent.ValidFor)
}

func TestCreateInviteWithEmailRefusals(t *testing.T) {
	t.Parallel()
	admin := auth.Identity{UserID: uuid.New(), Role: models.RoleAdmin}

	t.Run("e-mail already has an account", func(t *testing.T) {
		t.Parallel()
		svc, users, _, _, emails := newUserServiceWithEmails(t)
		emails.EXPECT().Ready().Return(true)
		users.EXPECT().GetByEmail(gomock.Any(), "ana@example.com").Return(models.User{ID: uuid.New()}, nil)
		_, err := svc.CreateInvite(context.Background(), admin, models.RoleMember, "ana@example.com")
		requireCode(t, err, "EMAIL_TAKEN")
	})

	t.Run("mailer not configured", func(t *testing.T) {
		t.Parallel()
		svc, _, _, _, emails := newUserServiceWithEmails(t)
		emails.EXPECT().Ready().Return(false)
		_, err := svc.CreateInvite(context.Background(), admin, models.RoleMember, "ana@example.com")
		requireCode(t, err, "MAILER_NOT_CONFIGURED")
	})

	t.Run("a link without e-mail needs no mailer", func(t *testing.T) {
		t.Parallel()
		svc, _, tokens, _, _ := newUserServiceWithEmails(t)
		tokens.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
		_, err := svc.CreateInvite(context.Background(), admin, models.RoleMember, "")
		require.NoError(t, err)
	})
}

func TestCreatePasswordResetForDisabledUser(t *testing.T) {
	t.Parallel()
	svc, users, _, _ := newUserService(t)
	admin := auth.Identity{UserID: uuid.New(), Role: models.RoleAdmin}
	id := uuid.New()
	users.EXPECT().GetByID(gomock.Any(), id).Return(models.User{ID: id, Status: models.UserDisabled}, nil)

	_, err := svc.CreatePasswordReset(context.Background(), admin, id)
	requireCode(t, err, "USER_DISABLED")
}
