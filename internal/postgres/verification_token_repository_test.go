package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/postgres"
)

// newStoredUser persists a user so tokens have a valid owner to reference,
// since the token tables carry a foreign key to users.
func newStoredUser(t *testing.T, repo *postgres.UserRepository) *domain.User {
	t.Helper()

	user := newTestUser()
	user.Status = domain.UserStatusActive
	if err := repo.Create(context.Background(), user); err != nil {
		t.Fatalf("failed to create the owning user: %v", err)
	}
	return user
}

func newTestToken(userID string) *domain.VerificationToken {
	return &domain.VerificationToken{
		UserID:    userID,
		TokenHash: uuid.NewString(),
		ExpiresAt: time.Now().Add(time.Hour),
	}
}

func TestVerificationTokenRepositoryConsumeSucceedsOnce(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	tokens := postgres.NewEmailVerificationTokenRepository(db)
	ctx := context.Background()

	user := newStoredUser(t, users)
	token := newTestToken(user.ID)

	if err := tokens.Create(ctx, token); err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}
	if token.ID == "" {
		t.Error("expected Create to populate the generated id")
	}

	consumed, err := tokens.Consume(ctx, token.TokenHash, time.Now())
	if err != nil {
		t.Fatalf("Consume returned an error: %v", err)
	}
	if consumed.UserID != user.ID {
		t.Errorf("expected the token to belong to %q, got %q", user.ID, consumed.UserID)
	}
	if consumed.ConsumedAt == nil {
		t.Error("expected consumed_at to be recorded, so the single-use guarantee stays auditable")
	}
}

// The single-use guarantee: a replayed token must be rejected.
func TestVerificationTokenRepositoryConsumeIsSingleUse(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	tokens := postgres.NewEmailVerificationTokenRepository(db)
	ctx := context.Background()

	user := newStoredUser(t, users)
	token := newTestToken(user.ID)
	if err := tokens.Create(ctx, token); err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}

	if _, err := tokens.Consume(ctx, token.TokenHash, time.Now()); err != nil {
		t.Fatalf("first Consume failed: %v", err)
	}

	_, err := tokens.Consume(ctx, token.TokenHash, time.Now())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected the replay to be rejected with ErrNotFound, got %v", err)
	}
}

func TestVerificationTokenRepositoryConsumeRejectsExpiredToken(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	tokens := postgres.NewEmailVerificationTokenRepository(db)
	ctx := context.Background()

	user := newStoredUser(t, users)
	token := newTestToken(user.ID)
	token.ExpiresAt = time.Now().Add(-time.Minute)
	if err := tokens.Create(ctx, token); err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}

	_, err := tokens.Consume(ctx, token.TokenHash, time.Now())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected an expired token to be rejected, got %v", err)
	}
}

func TestVerificationTokenRepositoryConsumeRejectsUnknownToken(t *testing.T) {
	db := newTestDB(t)
	tokens := postgres.NewEmailVerificationTokenRepository(db)

	_, err := tokens.Consume(context.Background(), uuid.NewString(), time.Now())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected domain.ErrNotFound, got %v", err)
	}
}

func TestVerificationTokenRepositoryInvalidateForUser(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	tokens := postgres.NewEmailVerificationTokenRepository(db)
	ctx := context.Background()

	user := newStoredUser(t, users)

	first := newTestToken(user.ID)
	second := newTestToken(user.ID)
	for _, token := range []*domain.VerificationToken{first, second} {
		if err := tokens.Create(ctx, token); err != nil {
			t.Fatalf("Create returned an error: %v", err)
		}
	}

	if err := tokens.InvalidateForUser(ctx, user.ID, time.Now()); err != nil {
		t.Fatalf("InvalidateForUser returned an error: %v", err)
	}

	for name, token := range map[string]*domain.VerificationToken{"first": first, "second": second} {
		if _, err := tokens.Consume(ctx, token.TokenHash, time.Now()); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("expected the %s token to be invalidated, got %v", name, err)
		}
	}
}

// Email verification and password reset must not share a namespace: a token
// minted for one flow has to be unusable in the other, even with the same hash.
func TestVerificationAndResetTokensAreIsolated(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	verification := postgres.NewEmailVerificationTokenRepository(db)
	reset := postgres.NewPasswordResetTokenRepository(db)
	ctx := context.Background()

	user := newStoredUser(t, users)
	token := newTestToken(user.ID)

	if err := verification.Create(ctx, token); err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}

	// The same hash must not be redeemable through the reset repository.
	if _, err := reset.Consume(ctx, token.TokenHash, time.Now()); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("a verification token was accepted by the password reset flow, got %v", err)
	}

	// And it must still work in its own flow.
	if _, err := verification.Consume(ctx, token.TokenHash, time.Now()); err != nil {
		t.Errorf("the verification token should still be valid in its own flow: %v", err)
	}
}

// Invalidating one flow's tokens must not touch the other's.
func TestInvalidateForUserIsScopedToOneFlow(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	verification := postgres.NewEmailVerificationTokenRepository(db)
	reset := postgres.NewPasswordResetTokenRepository(db)
	ctx := context.Background()

	user := newStoredUser(t, users)

	verificationToken := newTestToken(user.ID)
	if err := verification.Create(ctx, verificationToken); err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}
	resetToken := newTestToken(user.ID)
	if err := reset.Create(ctx, resetToken); err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}

	if err := verification.InvalidateForUser(ctx, user.ID, time.Now()); err != nil {
		t.Fatalf("InvalidateForUser returned an error: %v", err)
	}

	if _, err := reset.Consume(ctx, resetToken.TokenHash, time.Now()); err != nil {
		t.Errorf("the password reset token must survive invalidating verification tokens: %v", err)
	}
}
