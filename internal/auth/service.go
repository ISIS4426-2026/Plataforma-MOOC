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

// Config carries the settings the service needs, kept separate from the global
// configuration so the package does not depend on how the process is wired.
type Config struct {
	AppBaseURL           string
	EmailVerificationTTL time.Duration
}

// Service implements public registration with email verification (issue #9).
type Service struct {
	users              domain.UserRepository
	verificationTokens domain.VerificationTokenRepository
	mailer             domain.Mailer
	cfg                Config
	logger             *slog.Logger
	now                func() time.Time
}

func NewService(
	users domain.UserRepository,
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
