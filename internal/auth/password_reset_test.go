package auth_test

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/auth"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
)

const newPassword = "una-contrasena-nueva-999"

// resetTokenFromEmail pulls the recovery token out of the message the user
// receives, so the tests exercise the same link a person clicks.
func resetTokenFromEmail(t *testing.T, m *fakeMailer) string {
	t.Helper()

	message, ok := m.last()
	if !ok {
		t.Fatal("expected a password reset email to have been sent")
	}
	if !strings.Contains(message.Subject, "Restablece") {
		t.Fatalf("the last email is not a reset message: %q", message.Subject)
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

// requestReset drives a recovery request for the active test user and returns
// the token from the resulting email.
func requestReset(t *testing.T, h *harness) string {
	t.Helper()

	if err := h.service.RequestPasswordReset(context.Background(), testEmail); err != nil {
		t.Fatalf("RequestPasswordReset returned an error: %v", err)
	}
	return resetTokenFromEmail(t, h.mailer)
}

func TestRequestPasswordResetSendsALinkToAKnownAddress(t *testing.T) {
	h := newHarness(t, auth.Config{AppBaseURL: "http://localhost:8080"})
	loginActiveUser(t, h)

	raw := requestReset(t, h)

	message, _ := h.mailer.last()
	if message.To != testEmail {
		t.Errorf("expected the email to go to %q, got %q", testEmail, message.To)
	}
	if !strings.Contains(message.Body, "http://localhost:8080/api/v1/auth/password/reset?token=") {
		t.Errorf("expected a recovery link in the body, got %q", message.Body)
	}

	// Only the hash reaches storage; the raw token lives in the message alone.
	if _, stored := h.resetTokens.tokens[raw]; stored {
		t.Error("the raw token was stored instead of its hash")
	}
	if _, stored := h.resetTokens.tokens[auth.HashToken(raw)]; !stored {
		t.Error("expected the token hash to be stored")
	}
}

// Recovery tokens must live in their own store, never mixed with the
// verification ones.
func TestRequestPasswordResetDoesNotTouchTheVerificationStore(t *testing.T) {
	h := newHarness(t, auth.Config{})
	loginActiveUser(t, h)

	before := len(h.tokens.tokens)
	requestReset(t, h)

	if len(h.tokens.tokens) != before {
		t.Error("a recovery request wrote into the email verification store")
	}
	if len(h.resetTokens.tokens) != 1 {
		t.Errorf("expected exactly 1 reset token, got %d", len(h.resetTokens.tokens))
	}
}

// "La respuesta no revela si un correo existe o no en el sistema."
func TestRequestPasswordResetIsSilentForUnknownAddresses(t *testing.T) {
	h := newHarness(t, auth.Config{})

	if err := h.service.RequestPasswordReset(context.Background(), "nadie@example.test"); err != nil {
		t.Errorf("expected a silent success for an unknown address, got %v", err)
	}
	if _, sent := h.mailer.last(); sent {
		t.Error("no email should be sent for an unknown address")
	}
	if len(h.resetTokens.tokens) != 0 {
		t.Error("no token should be minted for an unknown address")
	}
}

// A malformed address must not be distinguishable either.
func TestRequestPasswordResetIsSilentForMalformedAddresses(t *testing.T) {
	h := newHarness(t, auth.Config{})

	if err := h.service.RequestPasswordReset(context.Background(), "no-es-un-correo"); err != nil {
		t.Errorf("expected a silent success for a malformed address, got %v", err)
	}
}

// A delivery failure must not surface either: answering with an error for a
// real address while answering success for an unknown one would leak exactly
// what the generic message is meant to hide.
func TestRequestPasswordResetHidesDeliveryFailures(t *testing.T) {
	h := newHarness(t, auth.Config{})
	loginActiveUser(t, h)
	h.mailer.err = errors.New("smtp unavailable")

	if err := h.service.RequestPasswordReset(context.Background(), testEmail); err != nil {
		t.Errorf("a delivery failure must not surface to the caller, got %v", err)
	}
}

func TestResetPasswordReplacesTheCredential(t *testing.T) {
	h := newHarness(t, auth.Config{})
	loginActiveUser(t, h)

	raw := requestReset(t, h)
	if err := h.service.ResetPassword(context.Background(), raw, newPassword); err != nil {
		t.Fatalf("ResetPassword returned an error: %v", err)
	}

	if _, err := h.service.Login(context.Background(), auth.LoginInput{
		Email: testEmail, Password: testPassword,
	}); !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("expected the old password to be rejected, got %v", err)
	}

	if _, err := h.service.Login(context.Background(), auth.LoginInput{
		Email: testEmail, Password: newPassword,
	}); err != nil {
		t.Errorf("expected the new password to work, got %v", err)
	}
}

// "Cambiar la contraseña invalida todas las sesiones activas previas."
func TestResetPasswordRevokesEveryExistingSession(t *testing.T) {
	h := newHarness(t, auth.Config{})
	first := loginActiveUser(t, h)

	// A second device.
	second, err := h.service.Login(context.Background(), auth.LoginInput{
		Email: testEmail, Password: testPassword,
	})
	if err != nil {
		t.Fatalf("second Login failed: %v", err)
	}

	raw := requestReset(t, h)
	if err := h.service.ResetPassword(context.Background(), raw, newPassword); err != nil {
		t.Fatalf("ResetPassword returned an error: %v", err)
	}

	for name, session := range map[string]*auth.LoginResult{"first": first, "second": second} {
		if _, _, err := h.service.Authenticate(context.Background(), session.Token); !errors.Is(err, domain.ErrUnauthorized) {
			t.Errorf("the %s session survived the password change: %v", name, err)
		}
	}

	// The cache must not keep a warm entry either.
	if len(h.cache.sessions) != 0 {
		t.Errorf("expected the session cache to be emptied, %d entries remain", len(h.cache.sessions))
	}
}

func TestResetPasswordTokenIsSingleUse(t *testing.T) {
	h := newHarness(t, auth.Config{})
	loginActiveUser(t, h)

	raw := requestReset(t, h)
	if err := h.service.ResetPassword(context.Background(), raw, newPassword); err != nil {
		t.Fatalf("first ResetPassword failed: %v", err)
	}

	err := h.service.ResetPassword(context.Background(), raw, "otra-contrasena-distinta")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected the replayed token to be rejected, got %v", err)
	}
}

func TestResetPasswordRejectsExpiredToken(t *testing.T) {
	h := newHarness(t, auth.Config{PasswordResetTTL: -time.Minute})
	loginActiveUser(t, h)

	raw := requestReset(t, h)

	err := h.service.ResetPassword(context.Background(), raw, newPassword)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected an expired token to be rejected, got %v", err)
	}
}

func TestResetPasswordRejectsUnknownToken(t *testing.T) {
	h := newHarness(t, auth.Config{})

	err := h.service.ResetPassword(context.Background(), "un-token-inventado", newPassword)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected domain.ErrNotFound, got %v", err)
	}
}

func TestResetPasswordRejectsEmptyToken(t *testing.T) {
	h := newHarness(t, auth.Config{})

	err := h.service.ResetPassword(context.Background(), "   ", newPassword)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("expected domain.ErrInvalidInput, got %v", err)
	}
}

// A rejected password must not burn the link: the user has to be able to retry
// with an acceptable one.
func TestResetPasswordDoesNotConsumeTheTokenOnInvalidPassword(t *testing.T) {
	h := newHarness(t, auth.Config{})
	loginActiveUser(t, h)

	raw := requestReset(t, h)

	if err := h.service.ResetPassword(context.Background(), raw, "corta"); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected domain.ErrInvalidInput, got %v", err)
	}

	if err := h.service.ResetPassword(context.Background(), raw, newPassword); err != nil {
		t.Errorf("the token was burned by a rejected password: %v", err)
	}
}

// Requesting a new link must retire the previous one.
func TestRequestPasswordResetSupersedesThePreviousLink(t *testing.T) {
	h := newHarness(t, auth.Config{})
	loginActiveUser(t, h)

	first := requestReset(t, h)
	second := requestReset(t, h)

	if first == second {
		t.Fatal("expected a different token to be issued")
	}
	if err := h.service.ResetPassword(context.Background(), first, newPassword); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected the superseded link to be rejected, got %v", err)
	}
	if err := h.service.ResetPassword(context.Background(), second, newPassword); err != nil {
		t.Errorf("expected the newest link to work, got %v", err)
	}
}

// A verification token must not be redeemable as a recovery one.
func TestVerificationTokenCannotResetAPassword(t *testing.T) {
	h := newHarness(t, auth.Config{})
	registerTestUser(t, h)
	verification := verificationTokenFromEmail(t, h.mailer)

	err := h.service.ResetPassword(context.Background(), verification, newPassword)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("a verification token was accepted by the recovery flow, got %v", err)
	}
}
