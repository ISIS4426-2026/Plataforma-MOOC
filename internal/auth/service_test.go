package auth_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/auth"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
)

const (
	testEmail    = "estudiante@example.test"
	testPassword = "contrasena-segura-123"
	testName     = "Estudiante de Prueba"
)

type harness struct {
	service *auth.Service
	users   *fakeUserRepo
	tokens  *fakeTokenRepo
	mailer  *fakeMailer
}

func newHarness(t *testing.T, cfg auth.Config) *harness {
	t.Helper()

	if cfg.AppBaseURL == "" {
		cfg.AppBaseURL = "http://localhost:8080"
	}
	if cfg.EmailVerificationTTL == 0 {
		cfg.EmailVerificationTTL = 24 * time.Hour
	}

	h := &harness{
		users:  newFakeUserRepo(),
		tokens: newFakeTokenRepo(),
		mailer: newFakeMailer(),
	}

	// Discard log output so a warning path under test does not pollute output.
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h.service = auth.NewService(h.users, h.tokens, h.mailer, cfg, logger)

	return h
}

var tokenPattern = regexp.MustCompile(`token=([A-Za-z0-9_\-%]+)`)

// verificationTokenFromEmail pulls the activation token out of the message the
// user actually receives, so the tests exercise the same link a person clicks.
func verificationTokenFromEmail(t *testing.T, m *fakeMailer) string {
	t.Helper()

	message, ok := m.last()
	if !ok {
		t.Fatal("expected a verification email to have been sent")
	}

	match := tokenPattern.FindStringSubmatch(message.Body)
	if match == nil {
		t.Fatalf("no token found in the email body: %q", message.Body)
	}

	decoded, err := url.QueryUnescape(match[1])
	if err != nil {
		t.Fatalf("token in the link is not valid URL encoding: %v", err)
	}

	return decoded
}

func registerTestUser(t *testing.T, h *harness) *domain.User {
	t.Helper()

	user, err := h.service.Register(context.Background(), auth.RegisterInput{
		Email:    testEmail,
		Password: testPassword,
		FullName: testName,
	})
	if err != nil {
		t.Fatalf("Register returned an error: %v", err)
	}
	return user
}

// Point 1 of section 5.1: public registration creates students only.
func TestRegisterAlwaysCreatesAStudentPendingVerification(t *testing.T) {
	h := newHarness(t, auth.Config{})

	user := registerTestUser(t, h)

	if user.Role != domain.RoleStudent {
		t.Errorf("expected role %q, got %q", domain.RoleStudent, user.Role)
	}
	if user.Status != domain.UserStatusPendingVerification {
		t.Errorf("expected status %q, got %q", domain.UserStatusPendingVerification, user.Status)
	}
}

// The stored credential must never be the password itself.
func TestRegisterStoresOnlyAPasswordHash(t *testing.T) {
	h := newHarness(t, auth.Config{})

	user := registerTestUser(t, h)

	if user.PasswordHash == testPassword {
		t.Fatal("the plaintext password was stored")
	}
	if err := auth.VerifyPassword(user.PasswordHash, testPassword); err != nil {
		t.Errorf("the stored hash does not verify the original password: %v", err)
	}
}

func TestRegisterSendsAVerificationEmailWithAUsableLink(t *testing.T) {
	h := newHarness(t, auth.Config{AppBaseURL: "http://localhost:8080"})

	registerTestUser(t, h)

	message, ok := h.mailer.last()
	if !ok {
		t.Fatal("expected a verification email")
	}
	if message.To != testEmail {
		t.Errorf("expected the email to go to %q, got %q", testEmail, message.To)
	}
	if !strings.Contains(message.Body, "http://localhost:8080/api/v1/auth/verify?token=") {
		t.Errorf("expected an activation link in the body, got %q", message.Body)
	}

	// The raw token travels only in the message; the repository keeps its hash.
	raw := verificationTokenFromEmail(t, h.mailer)
	if _, stored := h.tokens.tokens[raw]; stored {
		t.Error("the raw token was stored instead of its hash")
	}
	if _, stored := h.tokens.tokens[auth.HashToken(raw)]; !stored {
		t.Error("expected the token hash to be stored")
	}
}

func TestRegisterNormalisesTheEmailAddress(t *testing.T) {
	h := newHarness(t, auth.Config{})

	user, err := h.service.Register(context.Background(), auth.RegisterInput{
		Email:    "  Estudiante@Example.TEST  ",
		Password: testPassword,
		FullName: testName,
	})
	if err != nil {
		t.Fatalf("Register returned an error: %v", err)
	}

	if user.Email != testEmail {
		t.Errorf("expected the email normalised to %q, got %q", testEmail, user.Email)
	}
}

// Normalisation must also close the door on registering the same address twice
// under different capitalisation.
func TestRegisterRejectsDuplicateEmailRegardlessOfCase(t *testing.T) {
	h := newHarness(t, auth.Config{})
	registerTestUser(t, h)

	_, err := h.service.Register(context.Background(), auth.RegisterInput{
		Email:    "ESTUDIANTE@EXAMPLE.TEST",
		Password: testPassword,
		FullName: testName,
	})
	if !errors.Is(err, domain.ErrConflict) {
		t.Errorf("expected domain.ErrConflict, got %v", err)
	}
}

func TestRegisterRejectsInvalidInput(t *testing.T) {
	cases := map[string]auth.RegisterInput{
		"empty email":     {Email: "", Password: testPassword, FullName: testName},
		"malformed email": {Email: "not-an-email", Password: testPassword, FullName: testName},
		"short password":  {Email: testEmail, Password: "corta", FullName: testName},
		"empty name":      {Email: testEmail, Password: testPassword, FullName: "   "},
	}

	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t, auth.Config{})

			_, err := h.service.Register(context.Background(), input)
			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Errorf("expected domain.ErrInvalidInput, got %v", err)
			}
		})
	}
}

func TestVerifyEmailActivatesTheAccount(t *testing.T) {
	h := newHarness(t, auth.Config{})
	registerTestUser(t, h)

	raw := verificationTokenFromEmail(t, h.mailer)

	user, err := h.service.VerifyEmail(context.Background(), raw)
	if err != nil {
		t.Fatalf("VerifyEmail returned an error: %v", err)
	}

	if user.Status != domain.UserStatusActive {
		t.Errorf("expected status %q, got %q", domain.UserStatusActive, user.Status)
	}
}

// "El token no es reutilizable": a second redemption must fail.
func TestVerifyEmailTokenIsSingleUse(t *testing.T) {
	h := newHarness(t, auth.Config{})
	registerTestUser(t, h)

	raw := verificationTokenFromEmail(t, h.mailer)

	if _, err := h.service.VerifyEmail(context.Background(), raw); err != nil {
		t.Fatalf("first VerifyEmail failed: %v", err)
	}

	_, err := h.service.VerifyEmail(context.Background(), raw)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected the replayed token to be rejected with ErrNotFound, got %v", err)
	}
}

// "Ni válido después de expirar": a negative TTL yields an already expired token.
func TestVerifyEmailRejectsExpiredToken(t *testing.T) {
	h := newHarness(t, auth.Config{EmailVerificationTTL: -time.Minute})
	registerTestUser(t, h)

	raw := verificationTokenFromEmail(t, h.mailer)

	_, err := h.service.VerifyEmail(context.Background(), raw)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected an expired token to be rejected, got %v", err)
	}
}

func TestVerifyEmailRejectsUnknownToken(t *testing.T) {
	h := newHarness(t, auth.Config{})

	_, err := h.service.VerifyEmail(context.Background(), "un-token-inventado")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestVerifyEmailRejectsEmptyToken(t *testing.T) {
	h := newHarness(t, auth.Config{})

	_, err := h.service.VerifyEmail(context.Background(), "   ")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("expected domain.ErrInvalidInput, got %v", err)
	}
}

// Issuing a new link must retire the previous one.
func TestIssuingANewTokenInvalidatesThePreviousOne(t *testing.T) {
	h := newHarness(t, auth.Config{})
	user := registerTestUser(t, h)

	first := verificationTokenFromEmail(t, h.mailer)

	if err := h.service.ResendVerification(context.Background(), user.Email); err != nil {
		t.Fatalf("failed to re-issue the verification token: %v", err)
	}
	second := verificationTokenFromEmail(t, h.mailer)

	if first == second {
		t.Fatal("expected a different token to be issued")
	}

	if _, err := h.service.VerifyEmail(context.Background(), first); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected the superseded token to be rejected, got %v", err)
	}
	if _, err := h.service.VerifyEmail(context.Background(), second); err != nil {
		t.Errorf("expected the newest token to work, got %v", err)
	}
}

// Resending must not reveal whether the address is registered.
func TestResendVerificationIsSilentForUnknownAddresses(t *testing.T) {
	h := newHarness(t, auth.Config{})

	if err := h.service.ResendVerification(context.Background(), "nadie@example.test"); err != nil {
		t.Errorf("expected a silent success for an unknown address, got %v", err)
	}
	if _, sent := h.mailer.last(); sent {
		t.Error("no email should be sent for an unknown address")
	}
}

// Nor whether an account has already been activated.
func TestResendVerificationIsSilentForAlreadyActiveAccounts(t *testing.T) {
	h := newHarness(t, auth.Config{})
	registerTestUser(t, h)

	raw := verificationTokenFromEmail(t, h.mailer)
	if _, err := h.service.VerifyEmail(context.Background(), raw); err != nil {
		t.Fatalf("VerifyEmail failed: %v", err)
	}

	sentBefore := len(h.mailer.messages)
	if err := h.service.ResendVerification(context.Background(), testEmail); err != nil {
		t.Errorf("expected a silent success for an active account, got %v", err)
	}
	if len(h.mailer.messages) != sentBefore {
		t.Error("no new email should be sent for an already active account")
	}
}

// A registration whose email cannot be delivered must surface as an error: the
// account is already persisted and would otherwise be unusable with no signal
// to the caller.
func TestRegisterReportsAMailDeliveryFailure(t *testing.T) {
	h := newHarness(t, auth.Config{})
	h.mailer.err = errors.New("smtp unavailable")

	_, err := h.service.Register(context.Background(), auth.RegisterInput{
		Email:    testEmail,
		Password: testPassword,
		FullName: testName,
	})
	if err == nil {
		t.Fatal("expected Register to report the mail delivery failure")
	}
	if !strings.Contains(err.Error(), "send verification email") {
		t.Errorf("expected the error to identify the mail failure, got %v", err)
	}
}
