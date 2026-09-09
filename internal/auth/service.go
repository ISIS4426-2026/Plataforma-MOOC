package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
)

// MinPasswordBytes is the shortest password accepted at registration.
const MinPasswordBytes = 8

// dummyHash is compared against when no user matches, so a login attempt for an
// unknown address costs the same bcrypt work as one for a real account. Without
// it, response time alone reveals which addresses are registered.
const dummyHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

// ErrEmailNotVerified is returned when the credentials are correct but the
// account has not completed email verification.
//
// It is only ever returned after the password has been checked, so it cannot be
// used to discover which addresses are registered.
var ErrEmailNotVerified = errors.New("email address has not been verified")

// ErrAccountSuspended is returned when a suspended account presents valid
// credentials.
var ErrAccountSuspended = errors.New("account is suspended")

// Config carries the settings the service needs, kept separate from the global
// configuration so the package does not depend on how the process is wired.
type Config struct {
	AppBaseURL           string
	EmailVerificationTTL time.Duration
	SessionTTL           time.Duration
}

// Service implements public registration with email verification (issue #9) and
// login with revocable sessions (issue #10).
type Service struct {
	users              domain.UserRepository
	sessions           domain.SessionRepository
	sessionCache       domain.SessionCache
	verificationTokens domain.VerificationTokenRepository
	mailer             domain.Mailer
	cfg                Config
	logger             *slog.Logger
	now                func() time.Time
}

func NewService(
	users domain.UserRepository,
	sessions domain.SessionRepository,
	sessionCache domain.SessionCache,
	verificationTokens domain.VerificationTokenRepository,
	mailer domain.Mailer,
	cfg Config,
	logger *slog.Logger,
) *Service {
	if logger == nil {
		logger = slog.Default()
	}

	return &Service{
		users:              users,
		sessions:           sessions,
		sessionCache:       sessionCache,
		verificationTokens: verificationTokens,
		mailer:             mailer,
		cfg:                cfg,
		logger:             logger,
		now:                time.Now,
	}
}

// RegisterInput is the payload of a public registration.
//
// It carries no role: the public endpoint always creates students. Professors
// are created by administration only, which is point 1 of section 5.1.
type RegisterInput struct {
	Email    string
	Password string
	FullName string
}

// LoginInput is the payload of a login attempt. UserAgent and IPAddress are
// recorded on the session so it can be recognised in a session listing.
type LoginInput struct {
	Email     string
	Password  string
	UserAgent string
	IPAddress string
}

// Register creates a student account in pending_verification and mails a
// single-use activation link.
//
// The role is hardcoded rather than taken from input, so no request body can
// escalate a public registration into a professor or administrator account.
func (s *Service) Register(ctx context.Context, input RegisterInput) (*domain.User, error) {
	email, err := normaliseEmail(input.Email)
	if err != nil {
		return nil, err
	}

	fullName := strings.TrimSpace(input.FullName)
	if fullName == "" {
		return nil, fmt.Errorf("full name is required: %w", domain.ErrInvalidInput)
	}
	if len(input.Password) < MinPasswordBytes {
		return nil, fmt.Errorf("password must be at least %d characters: %w", MinPasswordBytes, domain.ErrInvalidInput)
	}

	passwordHash, err := HashPassword(input.Password)
	if err != nil {
		if errors.Is(err, ErrPasswordTooLong) {
			return nil, fmt.Errorf("%w: %w", domain.ErrInvalidInput, err)
		}
		return nil, err
	}

	user := &domain.User{
		Email:        email,
		PasswordHash: passwordHash,
		FullName:     fullName,
		Role:         domain.RoleStudent,
		Status:       domain.UserStatusPendingVerification,
	}

	if err := s.users.Create(ctx, user); err != nil {
		return nil, err
	}

	if err := s.issueVerificationToken(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

// ResendVerification mails a fresh activation link, superseding any previous
// one, for an account still awaiting verification.
//
// It reports success no matter what: an unknown address, an already active
// account and a genuine resend are indistinguishable to the caller, so the
// endpoint cannot be used to discover which addresses are registered or which
// accounts are already active.
func (s *Service) ResendVerification(ctx context.Context, rawEmail string) error {
	email, err := normaliseEmail(rawEmail)
	if err != nil {
		return nil
	}

	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil
		}
		return err
	}

	if user.Status != domain.UserStatusPendingVerification {
		return nil
	}

	return s.issueVerificationToken(ctx, user)
}

// issueVerificationToken retires any outstanding link and mails a fresh one, so
// only the most recent message works.
func (s *Service) issueVerificationToken(ctx context.Context, user *domain.User) error {
	now := s.now()

	if err := s.verificationTokens.InvalidateForUser(ctx, user.ID, now); err != nil {
		return err
	}

	raw, hash, err := GenerateToken()
	if err != nil {
		return err
	}

	token := &domain.VerificationToken{
		UserID:    user.ID,
		TokenHash: hash,
		ExpiresAt: now.Add(s.cfg.EmailVerificationTTL),
	}
	if err := s.verificationTokens.Create(ctx, token); err != nil {
		return err
	}

	subject := "Confirma tu cuenta en la Plataforma MOOC"
	body := s.verificationBody(user.FullName, raw)

	if err := s.mailer.Send(ctx, user.Email, subject, body); err != nil {
		// Registration has already been persisted, so a mail failure must be
		// reported rather than swallowed: otherwise the account exists with no
		// way to activate it and the caller believes it succeeded.
		return fmt.Errorf("send verification email: %w", err)
	}

	return nil
}

func (s *Service) verificationBody(fullName, rawToken string) string {
	link := fmt.Sprintf("%s/api/v1/auth/verify?token=%s",
		strings.TrimRight(s.cfg.AppBaseURL, "/"),
		url.QueryEscape(rawToken),
	)

	return fmt.Sprintf(`Hola %s,

Para activar tu cuenta en la Plataforma MOOC, abre el siguiente enlace:

%s

El enlace es de un solo uso y vence en %s.

Si no creaste esta cuenta, puedes ignorar este mensaje.
`, fullName, link, s.cfg.EmailVerificationTTL)
}

// VerifyEmail consumes an activation token and moves the account to active.
//
// Consumption happens before the account is touched, and the repository makes it
// atomic, so a token replayed concurrently activates the account exactly once.
func (s *Service) VerifyEmail(ctx context.Context, rawToken string) (*domain.User, error) {
	if strings.TrimSpace(rawToken) == "" {
		return nil, fmt.Errorf("token is required: %w", domain.ErrInvalidInput)
	}

	token, err := s.verificationTokens.Consume(ctx, HashToken(rawToken), s.now())
	if err != nil {
		return nil, err
	}

	user, err := s.users.GetByID(ctx, token.UserID)
	if err != nil {
		return nil, err
	}

	// A token is only meaningful for an account awaiting verification. A
	// suspended account must not be revived by an old activation link.
	if user.Status != domain.UserStatusPendingVerification {
		return user, nil
	}

	user.Status = domain.UserStatusActive
	if err := s.users.Update(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

// LoginResult carries everything a successful login produces. Token is the raw
// session credential and is returned exactly once, here; it is never stored and
// must never be logged.
type LoginResult struct {
	Session *domain.Session
	User    *domain.User
	Token   string
}

// Login verifies credentials and opens a session.
//
// The session is written to PostgreSQL first, which is the source of truth, and
// then cached in Redis. A cache failure is logged but does not fail the login:
// the session is already durable and the next request falls back to the
// database.
func (s *Service) Login(ctx context.Context, input LoginInput) (*LoginResult, error) {
	email, err := normaliseEmail(input.Email)
	if err != nil {
		// An unparseable address is simply a failed attempt; reporting a
		// validation error here would distinguish it from a wrong password.
		return nil, domain.ErrUnauthorized
	}

	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			// Spend the same bcrypt time as a real comparison would.
			_ = VerifyPassword(dummyHash, input.Password)
			return nil, domain.ErrUnauthorized
		}
		return nil, err
	}

	if err := VerifyPassword(user.PasswordHash, input.Password); err != nil {
		return nil, domain.ErrUnauthorized
	}

	// Status is inspected only after the password checks out, so these errors
	// cannot be used to enumerate accounts.
	switch user.Status {
	case domain.UserStatusPendingVerification:
		return nil, ErrEmailNotVerified
	case domain.UserStatusSuspended:
		return nil, ErrAccountSuspended
	case domain.UserStatusActive:
	default:
		return nil, domain.ErrUnauthorized
	}

	raw, hash, err := GenerateToken()
	if err != nil {
		return nil, err
	}

	now := s.now()
	session := &domain.Session{
		UserID:    user.ID,
		TokenHash: hash,
		UserAgent: input.UserAgent,
		IPAddress: input.IPAddress,
		ExpiresAt: now.Add(s.cfg.SessionTTL),
	}

	if err := s.sessions.Create(ctx, session); err != nil {
		return nil, err
	}

	s.cacheSession(ctx, session, now)

	return &LoginResult{Session: session, User: user, Token: raw}, nil
}

// Authenticate resolves a raw session token to its session and user.
//
// Redis answers the common case; a miss falls back to PostgreSQL and warms the
// cache again, so a Redis restart costs latency instead of logging everyone out.
func (s *Service) Authenticate(ctx context.Context, rawToken string) (*domain.Session, *domain.User, error) {
	if strings.TrimSpace(rawToken) == "" {
		return nil, nil, domain.ErrUnauthorized
	}

	hash := HashToken(rawToken)
	now := s.now()

	session, err := s.sessionCache.GetByTokenHash(ctx, hash)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		s.logger.WarnContext(ctx, "session cache lookup failed, falling back to database",
			slog.String("error", err.Error()),
		)
		session = nil
	}

	if session == nil {
		session, err = s.sessions.GetByTokenHash(ctx, hash)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return nil, nil, domain.ErrUnauthorized
			}
			return nil, nil, err
		}

		if session.IsUsable(now) {
			s.cacheSession(ctx, session, now)
		}
	}

	if !session.IsUsable(now) {
		return nil, nil, domain.ErrUnauthorized
	}

	user, err := s.users.GetByID(ctx, session.UserID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, nil, domain.ErrUnauthorized
		}
		return nil, nil, err
	}

	// A session must not outlive the account's right to use it: suspending a
	// user takes effect on their existing sessions too.
	if user.Status != domain.UserStatusActive {
		return nil, nil, domain.ErrUnauthorized
	}

	return session, user, nil
}

// Logout revokes the session behind the presented token.
//
// The cache entry is dropped after the durable write, so there is no window in
// which the database says revoked while Redis still serves the session.
func (s *Service) Logout(ctx context.Context, rawToken string) error {
	hash := HashToken(rawToken)

	session, err := s.sessions.GetByTokenHash(ctx, hash)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.ErrUnauthorized
		}
		return err
	}

	if err := s.sessions.Revoke(ctx, session.ID); err != nil && !errors.Is(err, domain.ErrNotFound) {
		return err
	}

	if err := s.sessionCache.Delete(ctx, hash); err != nil {
		return fmt.Errorf("drop cached session: %w", err)
	}

	return nil
}

// ListSessions returns the caller's active sessions.
func (s *Service) ListSessions(ctx context.Context, userID string) ([]*domain.Session, error) {
	return s.sessions.ListActiveByUser(ctx, userID)
}

// RevokeSession revokes one of the caller's own sessions.
//
// Ownership is checked before the session is touched, and a session belonging to
// somebody else reports ErrNotFound rather than ErrForbidden so the endpoint
// cannot be used to probe which session ids exist.
func (s *Service) RevokeSession(ctx context.Context, userID string, sessionID string) error {
	sessions, err := s.sessions.ListActiveByUser(ctx, userID)
	if err != nil {
		return err
	}

	var target *domain.Session
	for _, session := range sessions {
		if session.ID == sessionID {
			target = session
			break
		}
	}
	if target == nil {
		return domain.ErrNotFound
	}

	if err := s.sessions.Revoke(ctx, target.ID); err != nil {
		return err
	}

	if err := s.sessionCache.Delete(ctx, target.TokenHash); err != nil {
		return fmt.Errorf("drop cached session: %w", err)
	}

	return nil
}

// cacheSession stores the session for its remaining lifetime. A cache failure is
// never fatal: PostgreSQL already holds the session.
func (s *Service) cacheSession(ctx context.Context, session *domain.Session, now time.Time) {
	if err := s.sessionCache.Save(ctx, session, session.ExpiresAt.Sub(now)); err != nil {
		s.logger.WarnContext(ctx, "failed to cache session",
			slog.String("session_id", session.ID),
			slog.String("error", err.Error()),
		)
	}
}

// normaliseEmail trims and lowercases the address so lookup and insertion agree
// on a single representation, and validates it is parseable.
func normaliseEmail(raw string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return "", fmt.Errorf("email is required: %w", domain.ErrInvalidInput)
	}

	address, err := mail.ParseAddress(trimmed)
	if err != nil {
		return "", fmt.Errorf("email is not a valid address: %w", domain.ErrInvalidInput)
	}

	// ParseAddress accepts display names such as "Name <a@b.c>"; only the
	// address itself is stored.
	return address.Address, nil
}
