package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dev-fritz/newsp/api/internal/apperrors"
	"github.com/dev-fritz/newsp/api/internal/auth"
	"github.com/dev-fritz/newsp/api/internal/models"
	"github.com/dev-fritz/newsp/api/internal/repositories"
	"github.com/dev-fritz/newsp/api/internal/workers"
)

// Errors shared by the auth flows. Messages never say whether an e-mail exists.
var (
	errInvalidCredentials = apperrors.Unauthorized("INVALID_CREDENTIALS", "invalid e-mail or password")
	errSessionInvalid     = apperrors.Unauthorized("SESSION_INVALID", "session expired, sign in again")
	errInvalidLink        = apperrors.NotFound("INVALID_LINK", "this link is invalid, expired or was already used")
	errSetupDone          = apperrors.Conflict("SETUP_ALREADY_DONE", "the instance is already set up")
	errEmailTaken         = apperrors.Conflict("EMAIL_TAKEN", "this e-mail is already in use")
	errWeakPassword       = apperrors.Validation("invalid request", map[string]string{"password": "min"})
	errSignupDisabled     = apperrors.Forbidden("SIGNUP_DISABLED", "public signup is disabled on this instance")
	errAlreadyVerified    = apperrors.Conflict("EMAIL_ALREADY_VERIFIED", "this e-mail is already verified")
)

// verificationTTL is how long an e-mail verification link stays valid.
const verificationTTL = 24 * time.Hour

// selfResetTTL is how long a self-service reset link stays valid. It is
// shorter than an admin-issued link, which may be delivered out of band.
const selfResetTTL = time.Hour

// JobPasswordReset looks up the account of a reset request and e-mails the link.
const JobPasswordReset = "auth.password_reset"

// passwordResetPayload is the payload of JobPasswordReset.
type passwordResetPayload struct {
	Email string `json:"email"`
}

// Session is the result of a successful sign-in or refresh.
type Session struct {
	User             models.User
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
}

// NewAccount carries the fields to create an account.
type NewAccount struct {
	Name     string
	Email    string
	Password string
}

// InviteInfo is what an invite link reveals before it is accepted.
type InviteInfo struct {
	Role      models.Role
	ExpiresAt time.Time
	// Email is the address the invite is bound to; nil for an open invite.
	Email *string
}

// AuthService implements setup, sign-in, sessions and one-time links.
type AuthService struct {
	tx           TxRunner
	users        UserStore
	tokens       UserTokenStore
	sessions     SessionStore
	settings     SetupStore
	prefs        PreferenceSeeder
	issuer       AccessIssuer
	loginLimiter Limiter
	// resendLimiter bounds verification e-mails per user.
	resendLimiter Limiter
	// resetLimiter bounds self-service reset requests per e-mail.
	resetLimiter Limiter
	emails       EmailQueue
	queue        Enqueuer
	appBaseURL   string
	refreshTTL   time.Duration
	now          func() time.Time
}

// NewAuthService builds an AuthService.
func NewAuthService(tx TxRunner, users UserStore, tokens UserTokenStore, sessions SessionStore, settings SetupStore,
	prefs PreferenceSeeder, issuer AccessIssuer, loginLimiter, resendLimiter, resetLimiter Limiter, emails EmailQueue,
	queue Enqueuer, appBaseURL string, refreshTTL time.Duration, now func() time.Time,
) *AuthService {
	return &AuthService{
		tx: tx, users: users, tokens: tokens, sessions: sessions, settings: settings, prefs: prefs,
		issuer: issuer, loginLimiter: loginLimiter, resendLimiter: resendLimiter, resetLimiter: resetLimiter,
		emails: emails, queue: queue, appBaseURL: appBaseURL, refreshTTL: refreshTTL, now: now,
	}
}

// SetupStatus is what the sign-in pages need before anyone is signed in.
type SetupStatus struct {
	SetupRequired bool
	SignupEnabled bool
	// PasswordResetEnabled tells whether "forgot password" can e-mail a link.
	PasswordResetEnabled bool
}

// Status reports whether the first admin still has to be created and whether
// visitors may create an account.
func (s *AuthService) Status(ctx context.Context) (SetupStatus, error) {
	settings, err := s.settings.Get(ctx)
	if err != nil {
		return SetupStatus{}, fmt.Errorf("setup status: %w", err)
	}
	return SetupStatus{
		SetupRequired:        settings.SetupCompletedAt == nil,
		SignupEnabled:        s.signupOpen(settings),
		PasswordResetEnabled: s.emails.Ready(),
	}, nil
}

// signupOpen tells whether public signup works: the admin turned it on, the
// instance is set up, and verification e-mails can reach people.
func (s *AuthService) signupOpen(settings models.InstanceSettings) bool {
	return settings.SignupEnabled && settings.SetupCompletedAt != nil && s.emails.Ready()
}

// Signup creates a member account without an invite and signs it in. The
// e-mail starts unverified: the account works, but no report or idea is
// generated until the owner clicks the link sent to the address.
func (s *AuthService) Signup(ctx context.Context, in NewAccount, locale string) (Session, error) {
	settings, err := s.settings.Get(ctx)
	if err != nil {
		return Session{}, fmt.Errorf("signup: %w", err)
	}
	if !s.signupOpen(settings) {
		return Session{}, errSignupDisabled
	}
	hash, err := hashPassword(in.Password)
	if err != nil {
		return Session{}, err
	}
	now := s.now()
	user := newUser(in, hash, models.RoleMember, now)
	user.EmailVerifiedAt = nil
	if locale == "en" {
		user.Locale = locale
	}

	var session Session
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.createUser(ctx, user); err != nil {
			return err
		}
		if err := s.sendVerification(ctx, user); err != nil {
			return err
		}
		if err := s.users.TouchLastSeen(ctx, user.ID, now); err != nil {
			return err
		}
		session, err = s.startSession(ctx, user, uuid.New())
		return err
	})
	if err != nil {
		return Session{}, fmt.Errorf("signup: %w", err)
	}
	return session, nil
}

// VerifyEmail consumes a verification link and marks the address verified.
func (s *AuthService) VerifyEmail(ctx context.Context, token string) error {
	now := s.now()
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		t, err := s.tokens.GetUsableForUpdate(ctx, models.TokenEmailVerification, auth.HashToken(token), now)
		if errors.Is(err, repositories.ErrNotFound) {
			return errInvalidLink
		}
		if err != nil {
			return err
		}
		user, err := s.users.GetByIDForUpdate(ctx, *t.UserID)
		if err != nil {
			return err
		}
		// The link proves control of the address it was sent to.
		if user.Status != models.UserActive || t.Email == nil || *t.Email != user.Email {
			return errInvalidLink
		}
		if err := s.users.MarkEmailVerified(ctx, user.ID, now); err != nil {
			return err
		}
		return s.tokens.MarkUsed(ctx, t.ID, now)
	})
	if err != nil {
		return fmt.Errorf("verify email: %w", err)
	}
	return nil
}

// ResendVerification sends a new verification link to the caller.
func (s *AuthService) ResendVerification(ctx context.Context, id auth.Identity) error {
	user, err := s.Me(ctx, id)
	if err != nil {
		return err
	}
	if user.EmailVerifiedAt != nil {
		return errAlreadyVerified
	}
	if ok, wait := s.resendLimiter.Allow("verify:" + user.ID.String()); !ok {
		return apperrors.RateLimited(wait)
	}
	if err := s.tx.WithinTx(ctx, func(ctx context.Context) error { return s.sendVerification(ctx, user) }); err != nil {
		return fmt.Errorf("resend verification: %w", err)
	}
	return nil
}

// sendVerification stores a verification link for user and queues the e-mail.
// Run it inside a transaction.
func (s *AuthService) sendVerification(ctx context.Context, user models.User) error {
	plain, hash, err := auth.NewOpaqueToken()
	if err != nil {
		return err
	}
	now := s.now()
	email := user.Email
	t := models.UserToken{
		ID: uuid.New(), Kind: models.TokenEmailVerification, TokenHash: hash, UserID: &user.ID, Email: &email,
		ExpiresAt: now.Add(verificationTTL), CreatedAt: now,
	}
	if err := s.tokens.Create(ctx, t); err != nil {
		return err
	}
	return s.emails.Enqueue(ctx, &user.ID, Email{
		Kind: EmailVerification, To: user.Email, Name: user.Name, Locale: user.Locale,
		Link: s.appBaseURL + "/verify-email/" + plain, ValidFor: verificationTTL,
	})
}

// Setup creates the first admin. It works exactly once per instance.
func (s *AuthService) Setup(ctx context.Context, in NewAccount) (Session, error) {
	hash, err := hashPassword(in.Password)
	if err != nil {
		return Session{}, err
	}
	now := s.now()
	user := newUser(in, hash, models.RoleAdmin, now)

	var session Session
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		settings, err := s.settings.GetForUpdate(ctx)
		if err != nil {
			return err
		}
		if settings.SetupCompletedAt != nil {
			return errSetupDone
		}
		if err := s.createUser(ctx, user); err != nil {
			return err
		}
		if err := s.settings.MarkSetupCompleted(ctx, now); err != nil {
			return err
		}
		session, err = s.startSession(ctx, user, uuid.New())
		return err
	})
	if err != nil {
		return Session{}, fmt.Errorf("setup: %w", err)
	}
	return session, nil
}

// Login checks credentials and starts a new session family.
func (s *AuthService) Login(ctx context.Context, email, password string) (Session, error) {
	email = normalizeEmail(email)
	if ok, wait := s.loginLimiter.Allow("login:" + email); !ok {
		return Session{}, apperrors.RateLimited(wait)
	}

	user, err := s.users.GetByEmail(ctx, email)
	if errors.Is(err, repositories.ErrNotFound) {
		auth.CheckPassword(auth.DummyPasswordHash, password) // same cost as a real check
		return Session{}, errInvalidCredentials
	}
	if err != nil {
		return Session{}, fmt.Errorf("login: %w", err)
	}
	if !auth.CheckPassword(user.PasswordHash, password) || user.Status != models.UserActive {
		return Session{}, errInvalidCredentials
	}

	session, err := s.startSession(ctx, user, uuid.New())
	if err != nil {
		return Session{}, fmt.Errorf("login: %w", err)
	}
	if err := s.users.TouchLastSeen(ctx, user.ID, s.now()); err != nil {
		return Session{}, fmt.Errorf("login: %w", err)
	}
	return session, nil
}

// Refresh rotates a refresh token. Presenting a token that was already rotated
// means it leaked, so the whole family is revoked.
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (Session, error) {
	now := s.now()
	var (
		session Session
		revoke  *uuid.UUID // family to revoke after the transaction
	)
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		current, err := s.sessions.GetByHashForUpdate(ctx, auth.HashToken(refreshToken))
		if errors.Is(err, repositories.ErrNotFound) {
			return errSessionInvalid
		}
		if err != nil {
			return err
		}
		switch {
		case current.RevokedAt != nil, !now.Before(current.ExpiresAt):
			return errSessionInvalid
		case current.RotatedAt != nil:
			revoke = &current.FamilyID
			return nil
		}

		user, err := s.users.GetByID(ctx, current.UserID)
		if err != nil {
			return err
		}
		if user.Status != models.UserActive {
			revoke = &current.FamilyID
			return nil
		}
		if err := s.sessions.MarkRotated(ctx, current.ID, now); err != nil {
			return err
		}
		// A refresh is a visit: it keeps the account out of the inactivity pause.
		if err := s.users.TouchLastSeen(ctx, user.ID, now); err != nil {
			return err
		}
		session, err = s.startSession(ctx, user, current.FamilyID)
		return err
	})
	if err != nil {
		return Session{}, fmt.Errorf("refresh: %w", err)
	}
	if revoke != nil {
		if err := s.sessions.RevokeFamily(ctx, *revoke, now); err != nil {
			return Session{}, fmt.Errorf("refresh: %w", err)
		}
		return Session{}, errSessionInvalid
	}
	return session, nil
}

// Logout ends the session family of a refresh token. Unknown tokens are ignored.
func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		current, err := s.sessions.GetByHashForUpdate(ctx, auth.HashToken(refreshToken))
		if errors.Is(err, repositories.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		return s.sessions.RevokeFamily(ctx, current.FamilyID, s.now())
	})
	if err != nil {
		return fmt.Errorf("logout: %w", err)
	}
	return nil
}

// Me returns the caller's account.
func (s *AuthService) Me(ctx context.Context, id auth.Identity) (models.User, error) {
	user, err := s.users.GetByID(ctx, id.UserID)
	if errors.Is(err, repositories.ErrNotFound) {
		return models.User{}, errSessionInvalid
	}
	if err != nil {
		return models.User{}, fmt.Errorf("me: %w", err)
	}
	if user.Status != models.UserActive {
		return models.User{}, errSessionInvalid
	}
	return user, nil
}

// ChangePassword replaces the caller's password, ends every other session and
// returns a fresh session for the current device.
func (s *AuthService) ChangePassword(ctx context.Context, id auth.Identity, current, next string) (Session, error) {
	user, err := s.Me(ctx, id)
	if err != nil {
		return Session{}, err
	}
	if !auth.CheckPassword(user.PasswordHash, current) {
		return Session{}, apperrors.Validation("invalid request", map[string]string{"current_password": "invalid"})
	}
	hash, err := hashPassword(next)
	if err != nil {
		return Session{}, apperrors.Validation("invalid request", map[string]string{"new_password": "min"})
	}

	now := s.now()
	var session Session
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.users.UpdatePassword(ctx, user.ID, hash, now); err != nil {
			return err
		}
		if err := s.sessions.RevokeAllForUser(ctx, user.ID, now); err != nil {
			return err
		}
		session, err = s.startSession(ctx, user, uuid.New())
		return err
	})
	if err != nil {
		return Session{}, fmt.Errorf("change password: %w", err)
	}
	return session, nil
}

// InspectInvite tells the invite page whether a link is still usable.
func (s *AuthService) InspectInvite(ctx context.Context, token string) (InviteInfo, error) {
	t, err := s.tokens.GetUsable(ctx, models.TokenInvite, auth.HashToken(token), s.now())
	if errors.Is(err, repositories.ErrNotFound) {
		return InviteInfo{}, errInvalidLink
	}
	if err != nil {
		return InviteInfo{}, fmt.Errorf("inspect invite: %w", err)
	}
	return InviteInfo{Role: *t.Role, ExpiresAt: t.ExpiresAt, Email: t.Email}, nil
}

// AcceptInvite creates the invited account and signs it in.
func (s *AuthService) AcceptInvite(ctx context.Context, token string, in NewAccount) (Session, error) {
	hash, err := hashPassword(in.Password)
	if err != nil {
		return Session{}, err
	}
	now := s.now()

	var session Session
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		invite, err := s.tokens.GetUsableForUpdate(ctx, models.TokenInvite, auth.HashToken(token), now)
		if errors.Is(err, repositories.ErrNotFound) {
			return errInvalidLink
		}
		if err != nil {
			return err
		}
		// An invite sent to an address only creates the account for that address.
		if invite.Email != nil && normalizeEmail(in.Email) != normalizeEmail(*invite.Email) {
			return apperrors.Validation("invalid request", map[string]string{"email": "invite_mismatch"})
		}
		user := newUser(in, hash, *invite.Role, now)
		if err := s.createUser(ctx, user); err != nil {
			return err
		}
		if err := s.tokens.MarkUsed(ctx, invite.ID, now); err != nil {
			return err
		}
		session, err = s.startSession(ctx, user, uuid.New())
		return err
	})
	if err != nil {
		return Session{}, fmt.Errorf("accept invite: %w", err)
	}
	return session, nil
}

// RequestPasswordReset starts a self-service reset. It never looks the account
// up, so the answer and its timing are the same for any e-mail; the job does
// the real work. The only refusal is the per-address limit, which is equally
// blind to whether the account exists.
func (s *AuthService) RequestPasswordReset(ctx context.Context, email string) error {
	email = normalizeEmail(email)
	if ok, wait := s.resetLimiter.Allow("reset:" + email); !ok {
		return apperrors.RateLimited(wait)
	}
	_, err := s.queue.Enqueue(ctx, JobPasswordReset, passwordResetPayload{Email: email},
		workers.EnqueueOptions{DedupeKey: JobPasswordReset + ":" + email})
	if err != nil {
		return fmt.Errorf("request password reset: enqueue: %w", err)
	}
	return nil
}

// SendPasswordReset is the JobPasswordReset handler. Unknown and inactive
// accounts are skipped silently, and the e-mail stays out of the job result.
func (s *AuthService) SendPasswordReset(ctx context.Context, job models.Job) (any, error) {
	var p passwordResetPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return nil, workers.Permanent(fmt.Errorf("decode password reset payload: %w", err))
	}
	if p.Email == "" {
		return nil, workers.Permanent(errors.New("password reset payload has no e-mail"))
	}
	user, err := s.users.GetByEmail(ctx, p.Email)
	if errors.Is(err, repositories.ErrNotFound) {
		return map[string]string{"skipped": "no active account"}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("send password reset: %w", err)
	}
	if user.Status != models.UserActive {
		return map[string]string{"skipped": "no active account"}, nil
	}

	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		now := s.now()
		// Only the newest link works.
		if err := s.tokens.RevokePendingResets(ctx, user.ID, now); err != nil {
			return err
		}
		plain, hash, err := auth.NewOpaqueToken()
		if err != nil {
			return err
		}
		t := models.UserToken{
			ID: uuid.New(), Kind: models.TokenPasswordReset, TokenHash: hash, UserID: &user.ID,
			ExpiresAt: now.Add(selfResetTTL), CreatedAt: now,
		}
		if err := s.tokens.Create(ctx, t); err != nil {
			return err
		}
		return s.emails.Enqueue(ctx, &user.ID, Email{
			Kind: EmailPasswordReset, To: user.Email, Name: user.Name, Locale: user.Locale,
			Link: s.appBaseURL + "/reset-password/" + plain, ValidFor: selfResetTTL,
		})
	})
	if err != nil {
		return nil, fmt.Errorf("send password reset: %w", err)
	}
	return map[string]string{"status": "sent"}, nil
}

// InspectPasswordReset tells the reset page whether a link is still usable.
func (s *AuthService) InspectPasswordReset(ctx context.Context, token string) error {
	_, err := s.tokens.GetUsable(ctx, models.TokenPasswordReset, auth.HashToken(token), s.now())
	if errors.Is(err, repositories.ErrNotFound) {
		return errInvalidLink
	}
	if err != nil {
		return fmt.Errorf("inspect password reset: %w", err)
	}
	return nil
}

// ResetPassword sets a new password through a reset link and ends all sessions.
func (s *AuthService) ResetPassword(ctx context.Context, token, password string) error {
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	now := s.now()
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		reset, err := s.tokens.GetUsableForUpdate(ctx, models.TokenPasswordReset, auth.HashToken(token), now)
		if errors.Is(err, repositories.ErrNotFound) {
			return errInvalidLink
		}
		if err != nil {
			return err
		}
		user, err := s.users.GetByIDForUpdate(ctx, *reset.UserID)
		if err != nil {
			return err
		}
		if user.Status != models.UserActive {
			return errInvalidLink
		}
		if err := s.users.UpdatePassword(ctx, user.ID, hash, now); err != nil {
			return err
		}
		if err := s.tokens.MarkUsed(ctx, reset.ID, now); err != nil {
			return err
		}
		return s.sessions.RevokeAllForUser(ctx, user.ID, now)
	})
	if err != nil {
		return fmt.Errorf("reset password: %w", err)
	}
	return nil
}

// createUser stores the account with its default preferences, so scheduled
// reports and ideas start without a visit to the profile.
func (s *AuthService) createUser(ctx context.Context, u models.User) error {
	err := s.users.Create(ctx, u)
	if errors.Is(err, repositories.ErrDuplicate) {
		return errEmailTaken
	}
	if err != nil {
		return err
	}
	prefs, err := initialPreferences(u, s.now())
	if err != nil {
		return err
	}
	return s.prefs.Save(ctx, prefs)
}

// startSession issues an access token and a refresh token in family.
func (s *AuthService) startSession(ctx context.Context, user models.User, family uuid.UUID) (Session, error) {
	access, accessExp, err := s.issuer.Issue(user.ID, user.Role)
	if err != nil {
		return Session{}, err
	}
	plain, hash, err := auth.NewOpaqueToken()
	if err != nil {
		return Session{}, err
	}
	now := s.now()
	refresh := models.RefreshToken{
		ID: uuid.New(), UserID: user.ID, FamilyID: family, TokenHash: hash,
		ExpiresAt: now.Add(s.refreshTTL), CreatedAt: now,
	}
	if err := s.sessions.Create(ctx, refresh); err != nil {
		return Session{}, err
	}
	return Session{
		User: user, AccessToken: access, AccessExpiresAt: accessExp,
		RefreshToken: plain, RefreshExpiresAt: refresh.ExpiresAt,
	}, nil
}

func newUser(in NewAccount, hash string, role models.Role, now time.Time) models.User {
	return models.User{
		ID: uuid.New(), Email: normalizeEmail(in.Email), Name: strings.TrimSpace(in.Name),
		PasswordHash: hash, Role: role, Status: models.UserActive, Locale: "pt-BR",
		EmailVerifiedAt: &now, CreatedAt: now, UpdatedAt: now,
	}
}

func hashPassword(password string) (string, error) {
	hash, err := auth.HashPassword(password)
	if errors.Is(err, auth.ErrWeakPassword) {
		return "", errWeakPassword
	}
	return hash, err
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
