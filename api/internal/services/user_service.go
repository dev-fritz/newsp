package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/dev-fritz/newsp/api/internal/apperrors"
	"github.com/dev-fritz/newsp/api/internal/auth"
	"github.com/dev-fritz/newsp/api/internal/models"
	"github.com/dev-fritz/newsp/api/internal/repositories"
)

// linkTTL is how long invite and password reset links stay valid.
const linkTTL = 7 * 24 * time.Hour

var (
	errForbidden    = apperrors.Forbidden("FORBIDDEN", "you do not have permission to do this")
	errUserNotFound = apperrors.NotFound("USER_NOT_FOUND", "user not found")
)

// Link is a one-time URL shown once to the admin who created it.
type Link struct {
	ID        uuid.UUID
	URL       string
	ExpiresAt time.Time
	// Email is the address an invite was sent to; nil when no e-mail was queued.
	Email *string
}

// UserAccessChange updates a user's role and/or status.
type UserAccessChange struct {
	Role   *models.Role
	Status *models.UserStatus
}

// UserService implements user administration.
type UserService struct {
	tx         TxRunner
	users      UserStore
	tokens     UserTokenStore
	sessions   SessionStore
	emails     EmailQueue
	appBaseURL string
	now        func() time.Time
}

// NewUserService builds a UserService.
func NewUserService(tx TxRunner, users UserStore, tokens UserTokenStore, sessions SessionStore, emails EmailQueue, appBaseURL string, now func() time.Time) *UserService {
	return &UserService{tx: tx, users: users, tokens: tokens, sessions: sessions, emails: emails, appBaseURL: appBaseURL, now: now}
}

// List returns a page of users.
func (s *UserService) List(ctx context.Context, actor auth.Identity, limit, offset int) ([]models.User, int, error) {
	if !auth.Can(actor, auth.PermUsersManage) {
		return nil, 0, errForbidden
	}
	users, total, err := s.users.List(ctx, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	return users, total, nil
}

// UpdateAccess changes a user's role or status. Admins cannot change their
// own access, and the instance always keeps at least one active admin.
func (s *UserService) UpdateAccess(ctx context.Context, actor auth.Identity, userID uuid.UUID, change UserAccessChange) (models.User, error) {
	if !auth.Can(actor, auth.PermUsersManage) {
		return models.User{}, errForbidden
	}
	if actor.UserID == userID {
		return models.User{}, apperrors.Unprocessable("CANNOT_CHANGE_OWN_ACCESS", "you cannot change your own role or status")
	}

	now := s.now()
	var updated models.User
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		// Lock admins before the target, always in the same order, to avoid deadlocks.
		admins, err := s.users.LockActiveAdmins(ctx)
		if err != nil {
			return err
		}
		target, err := s.users.GetByIDForUpdate(ctx, userID)
		if errors.Is(err, repositories.ErrNotFound) {
			return errUserNotFound
		}
		if err != nil {
			return err
		}

		role, status := target.Role, target.Status
		if change.Role != nil {
			role = *change.Role
		}
		if change.Status != nil {
			status = *change.Status
		}
		wasActiveAdmin := target.Role == models.RoleAdmin && target.Status == models.UserActive
		staysActiveAdmin := role == models.RoleAdmin && status == models.UserActive
		if wasActiveAdmin && !staysActiveAdmin && len(admins) <= 1 {
			return apperrors.Unprocessable("LAST_ADMIN", "the instance needs at least one active admin")
		}

		if err := s.users.UpdateAccess(ctx, userID, role, status, now); err != nil {
			return err
		}
		if status == models.UserDisabled {
			if err := s.sessions.RevokeAllForUser(ctx, userID, now); err != nil {
				return err
			}
		}
		target.Role, target.Status, target.UpdatedAt = role, status, now
		updated = target
		return nil
	})
	if err != nil {
		return models.User{}, fmt.Errorf("update user access: %w", err)
	}
	return updated, nil
}

// CreateInvite creates a one-time invite link for a new account with role.
// With an e-mail, the invite is bound to that address and the link is also
// e-mailed; the token and the queued message share one transaction.
func (s *UserService) CreateInvite(ctx context.Context, actor auth.Identity, role models.Role, email string) (Link, error) {
	if !auth.Can(actor, auth.PermUsersManage) {
		return Link{}, errForbidden
	}
	email = normalizeEmail(email)
	if email != "" && !s.emails.Ready() {
		return Link{}, errMailerNotConfigured
	}
	plain, hash, err := auth.NewOpaqueToken()
	if err != nil {
		return Link{}, err
	}
	now := s.now()
	token := models.UserToken{
		ID: uuid.New(), Kind: models.TokenInvite, TokenHash: hash, Role: &role,
		CreatedBy: &actor.UserID, ExpiresAt: now.Add(linkTTL), CreatedAt: now,
	}
	link := Link{ID: token.ID, URL: s.appBaseURL + "/invite/" + plain, ExpiresAt: token.ExpiresAt}
	if email == "" {
		if err := s.tokens.Create(ctx, token); err != nil {
			return Link{}, fmt.Errorf("create invite: %w", err)
		}
		return link, nil
	}

	token.Email = &email
	link.Email = &email
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		_, err := s.users.GetByEmail(ctx, email)
		if err == nil {
			return errEmailTaken
		}
		if !errors.Is(err, repositories.ErrNotFound) {
			return err
		}
		inviter, err := s.users.GetByID(ctx, actor.UserID)
		if err != nil {
			return err
		}
		if err := s.tokens.Create(ctx, token); err != nil {
			return err
		}
		return s.emails.Enqueue(ctx, nil, Email{
			Kind: EmailInvite, To: email, Locale: inviter.Locale, Link: link.URL, ValidFor: linkTTL,
		})
	})
	if err != nil {
		return Link{}, fmt.Errorf("create invite: %w", err)
	}
	return link, nil
}

// ListInvites returns a page of pending invites.
func (s *UserService) ListInvites(ctx context.Context, actor auth.Identity, limit, offset int) ([]models.UserToken, int, error) {
	if !auth.Can(actor, auth.PermUsersManage) {
		return nil, 0, errForbidden
	}
	invites, total, err := s.tokens.ListPendingInvites(ctx, s.now(), limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list invites: %w", err)
	}
	return invites, total, nil
}

// RevokeInvite cancels a pending invite.
func (s *UserService) RevokeInvite(ctx context.Context, actor auth.Identity, inviteID uuid.UUID) error {
	if !auth.Can(actor, auth.PermUsersManage) {
		return errForbidden
	}
	err := s.tokens.RevokeInvite(ctx, inviteID, s.now())
	if errors.Is(err, repositories.ErrNotFound) {
		return apperrors.NotFound("INVITE_NOT_FOUND", "invite not found or no longer pending")
	}
	if err != nil {
		return fmt.Errorf("revoke invite: %w", err)
	}
	return nil
}

// CreatePasswordReset creates a reset link for a user, replacing older ones.
func (s *UserService) CreatePasswordReset(ctx context.Context, actor auth.Identity, userID uuid.UUID) (Link, error) {
	if !auth.Can(actor, auth.PermUsersManage) {
		return Link{}, errForbidden
	}
	plain, hash, err := auth.NewOpaqueToken()
	if err != nil {
		return Link{}, err
	}
	now := s.now()
	token := models.UserToken{
		ID: uuid.New(), Kind: models.TokenPasswordReset, TokenHash: hash, UserID: &userID,
		CreatedBy: &actor.UserID, ExpiresAt: now.Add(linkTTL), CreatedAt: now,
	}
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		target, err := s.users.GetByID(ctx, userID)
		if errors.Is(err, repositories.ErrNotFound) {
			return errUserNotFound
		}
		if err != nil {
			return err
		}
		if target.Status != models.UserActive {
			return apperrors.Unprocessable("USER_DISABLED", "reactivate the user before resetting the password")
		}
		if err := s.tokens.RevokePendingResets(ctx, userID, now); err != nil {
			return err
		}
		return s.tokens.Create(ctx, token)
	})
	if err != nil {
		return Link{}, fmt.Errorf("create password reset: %w", err)
	}
	return Link{ID: token.ID, URL: s.appBaseURL + "/reset-password/" + plain, ExpiresAt: token.ExpiresAt}, nil
}
