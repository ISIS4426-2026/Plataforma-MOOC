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
	service  *auth.Service
	users    *fakeUserRepo
	sessions *fakeSessionRepo
	cache    *fakeSessionCache
	tokens   *fakeTokenRepo
	mailer   *fakeMailer
}

func newHarness(t *testing.T, cfg auth.Config) *harness {
	t.Helper()

	if cfg.AppBaseURL == "" {
		cfg.AppBaseURL = "http://localhost:8080"
	}
	if cfg.SessionTTL == 0 {
		cfg.SessionTTL = time.Hour
	}
	if cfg.EmailVerificationTTL == 0 {
		cfg.EmailVerificationTTL = 24 * time.Hour
	}

	h := &harness{
		users:    newFakeUserRepo(),
		sessions: newFakeSessionRepo(),
		cache:    newFakeSessionCache(),
		tokens:   newFakeTokenRepo(),
		mailer:   newFakeMailer(),
	}

	// Discard log output so a warning path under test does not pollute output.
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h.service = auth.NewService(h.users, h.sessions, h.cache, h.tokens, h.mailer, cfg, logger)

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

func TestLoginBeforeVerificationIsRejected(t *testing.T) {
	h := newHarness(t, auth.Config{})
	registerTestUser(t, h)

	_, err := h.service.Login(context.Background(), auth.LoginInput{
		Email:    testEmail,
		Password: testPassword,
	})
	if !errors.Is(err, auth.ErrEmailNotVerified) {
		t.Errorf("expected ErrEmailNotVerified, got %v", err)
	}
}

// The acceptance flow of issue #9: register, verify, then log in.
func TestRegisterVerifyLoginFlow(t *testing.T) {
	h := newHarness(t, auth.Config{})
	registerTestUser(t, h)

	raw := verificationTokenFromEmail(t, h.mailer)
	if _, err := h.service.VerifyEmail(context.Background(), raw); err != nil {
		t.Fatalf("VerifyEmail failed: %v", err)
	}

	result, err := h.service.Login(context.Background(), auth.LoginInput{
		Email:    testEmail,
		Password: testPassword,
	})
	if err != nil {
		t.Fatalf("Login failed after verification: %v", err)
	}

	if result.Token == "" {
		t.Error("expected a session token")
	}
	if result.Session.TokenHash != auth.HashToken(result.Token) {
		t.Error("the stored session must reference the hash of the issued token")
	}
	if result.Session.TokenHash == result.Token {
		t.Error("the raw session token was stored")
	}
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	h := newHarness(t, auth.Config{})
	registerTestUser(t, h)
	raw := verificationTokenFromEmail(t, h.mailer)
	if _, err := h.service.VerifyEmail(context.Background(), raw); err != nil {
		t.Fatalf("VerifyEmail failed: %v", err)
	}

	_, err := h.service.Login(context.Background(), auth.LoginInput{
		Email:    testEmail,
		Password: "contrasena-incorrecta",
	})
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("expected domain.ErrUnauthorized, got %v", err)
	}
}

// An unknown address must fail exactly like a wrong password, so login cannot
// be used to discover which addresses are registered.
func TestLoginRejectsUnknownEmailIndistinguishably(t *testing.T) {
	h := newHarness(t, auth.Config{})

	_, err := h.service.Login(context.Background(), auth.LoginInput{
		Email:    "desconocido@example.test",
		Password: testPassword,
	})
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("expected domain.ErrUnauthorized, got %v", err)
	}
}

func TestLoginRejectsSuspendedAccount(t *testing.T) {
	h := newHarness(t, auth.Config{})
	user := registerTestUser(t, h)
	raw := verificationTokenFromEmail(t, h.mailer)
	if _, err := h.service.VerifyEmail(context.Background(), raw); err != nil {
		t.Fatalf("VerifyEmail failed: %v", err)
	}

	user.Status = domain.UserStatusSuspended
	if err := h.users.Update(context.Background(), user); err != nil {
		t.Fatalf("failed to suspend the account: %v", err)
	}

	_, err := h.service.Login(context.Background(), auth.LoginInput{
		Email:    testEmail,
		Password: testPassword,
	})
	if !errors.Is(err, auth.ErrAccountSuspended) {
		t.Errorf("expected ErrAccountSuspended, got %v", err)
	}
}

// loginActiveUser registers, verifies and logs in, returning the raw token.
func loginActiveUser(t *testing.T, h *harness) *auth.LoginResult {
	t.Helper()

	registerTestUser(t, h)
	raw := verificationTokenFromEmail(t, h.mailer)
	if _, err := h.service.VerifyEmail(context.Background(), raw); err != nil {
		t.Fatalf("VerifyEmail failed: %v", err)
	}

	result, err := h.service.Login(context.Background(), auth.LoginInput{
		Email:    testEmail,
		Password: testPassword,
	})
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	return result
}

func TestAuthenticateResolvesAnActiveSession(t *testing.T) {
	h := newHarness(t, auth.Config{})
	result := loginActiveUser(t, h)

	session, user, err := h.service.Authenticate(context.Background(), result.Token)
	if err != nil {
		t.Fatalf("Authenticate returned an error: %v", err)
	}

	if session.ID != result.Session.ID {
		t.Errorf("expected session %q, got %q", result.Session.ID, session.ID)
	}
	if user.Email != testEmail {
		t.Errorf("expected user %q, got %q", testEmail, user.Email)
	}
}

// "Logout invalida la sesión de forma inmediata": the very next request fails.
func TestLogoutRevokesTheSessionImmediately(t *testing.T) {
	h := newHarness(t, auth.Config{})
	result := loginActiveUser(t, h)

	if err := h.service.Logout(context.Background(), result.Token); err != nil {
		t.Fatalf("Logout returned an error: %v", err)
	}

	_, _, err := h.service.Authenticate(context.Background(), result.Token)
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("expected the revoked token to be rejected at once, got %v", err)
	}
}

// Revocation must survive a warm cache: the entry is dropped, not left to expire.
func TestRevokeSessionInvalidatesTheCachedEntry(t *testing.T) {
	h := newHarness(t, auth.Config{})
	result := loginActiveUser(t, h)

	if err := h.service.RevokeSession(context.Background(), result.User.ID, result.Session.ID); err != nil {
		t.Fatalf("RevokeSession returned an error: %v", err)
	}

	if _, ok := h.cache.sessions[result.Session.TokenHash]; ok {
		t.Error("the cached session outlived its revocation")
	}

	_, _, err := h.service.Authenticate(context.Background(), result.Token)
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("expected the revoked session to be rejected, got %v", err)
	}
}

// Losing Redis must cost latency, not sessions.
func TestAuthenticateFallsBackToTheDatabaseOnCacheMiss(t *testing.T) {
	h := newHarness(t, auth.Config{})
	result := loginActiveUser(t, h)

	// Simulate a cache flush.
	h.cache.sessions = make(map[string]*domain.Session)

	session, _, err := h.service.Authenticate(context.Background(), result.Token)
	if err != nil {
		t.Fatalf("expected the database fallback to resolve the session, got %v", err)
	}
	if session.ID != result.Session.ID {
		t.Errorf("expected session %q, got %q", result.Session.ID, session.ID)
	}

	// The fallback should have repopulated the cache.
	if _, ok := h.cache.sessions[result.Session.TokenHash]; !ok {
		t.Error("expected the cache to be warmed after the fallback")
	}
}

func TestAuthenticateRejectsAnExpiredSession(t *testing.T) {
	h := newHarness(t, auth.Config{SessionTTL: -time.Minute})
	registerTestUser(t, h)
	raw := verificationTokenFromEmail(t, h.mailer)
	if _, err := h.service.VerifyEmail(context.Background(), raw); err != nil {
		t.Fatalf("VerifyEmail failed: %v", err)
	}

	result, err := h.service.Login(context.Background(), auth.LoginInput{
		Email:    testEmail,
		Password: testPassword,
	})
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	_, _, err = h.service.Authenticate(context.Background(), result.Token)
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("expected an expired session to be rejected, got %v", err)
	}
}

// Suspending a user must take effect on sessions that are already open.
func TestAuthenticateRejectsSessionsOfASuspendedUser(t *testing.T) {
	h := newHarness(t, auth.Config{})
	result := loginActiveUser(t, h)

	suspended := result.User
	suspended.Status = domain.UserStatusSuspended
	if err := h.users.Update(context.Background(), suspended); err != nil {
		t.Fatalf("failed to suspend the account: %v", err)
	}

	_, _, err := h.service.Authenticate(context.Background(), result.Token)
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("expected the session of a suspended user to be rejected, got %v", err)
	}
}

// A session id belonging to somebody else must not be revocable, and must not
// be distinguishable from one that does not exist.
func TestRevokeSessionOfAnotherUserIsNotFound(t *testing.T) {
	h := newHarness(t, auth.Config{})
	result := loginActiveUser(t, h)

	err := h.service.RevokeSession(context.Background(), "otro-usuario", result.Session.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected domain.ErrNotFound, got %v", err)
	}

	// The session must still work for its owner.
	if _, _, err := h.service.Authenticate(context.Background(), result.Token); err != nil {
		t.Errorf("the session was affected by another user's revocation attempt: %v", err)
	}
}

func TestVerifyEmailRejectsEmptyToken(t *testing.T) {
	h := newHarness(t, auth.Config{})

	_, err := h.service.VerifyEmail(context.Background(), "   ")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("expected domain.ErrInvalidInput, got %v", err)
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
