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

func newTestSession(userID string) *domain.Session {
	return &domain.Session{
		UserID:    userID,
		TokenHash: uuid.NewString(),
		UserAgent: "curl/8.0.0",
		IPAddress: "172.18.0.1",
		ExpiresAt: time.Now().Add(time.Hour),
	}
}

func TestSessionRepositoryCreateAndGetByTokenHash(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	sessions := postgres.NewSessionRepository(db)
	ctx := context.Background()

	user := newStoredUser(t, users)
	session := newTestSession(user.ID)

	if err := sessions.Create(ctx, session); err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}
	if session.ID == "" {
		t.Error("expected Create to populate the generated id")
	}
	if session.IsRevoked {
		t.Error("a new session must not be revoked")
	}

	found, err := sessions.GetByTokenHash(ctx, session.TokenHash)
	if err != nil {
		t.Fatalf("GetByTokenHash returned an error: %v", err)
	}
	if found.ID != session.ID {
		t.Errorf("expected session %q, got %q", session.ID, found.ID)
	}
	if found.UserAgent != "curl/8.0.0" {
		t.Errorf("expected the user agent to round-trip, got %q", found.UserAgent)
	}
	if found.IPAddress != "172.18.0.1" {
		t.Errorf("expected the ip address to round-trip, got %q", found.IPAddress)
	}
}

func TestSessionRepositoryGetByTokenHashReportsNotFound(t *testing.T) {
	db := newTestDB(t)
	sessions := postgres.NewSessionRepository(db)

	_, err := sessions.GetByTokenHash(context.Background(), uuid.NewString())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected domain.ErrNotFound, got %v", err)
	}
}

// Nullable audit columns must come back as empty strings rather than failing
// the scan when nothing was recorded.
func TestSessionRepositoryHandlesMissingAuditColumns(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	sessions := postgres.NewSessionRepository(db)
	ctx := context.Background()

	user := newStoredUser(t, users)
	session := newTestSession(user.ID)
	session.UserAgent = ""
	session.IPAddress = ""

	if err := sessions.Create(ctx, session); err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}

	found, err := sessions.GetByTokenHash(ctx, session.TokenHash)
	if err != nil {
		t.Fatalf("GetByTokenHash returned an error: %v", err)
	}
	if found.UserAgent != "" || found.IPAddress != "" {
		t.Errorf("expected empty audit fields, got %q / %q", found.UserAgent, found.IPAddress)
	}
	if found.RevokedAt != nil {
		t.Error("an active session must have no revocation timestamp")
	}
}

func TestSessionRepositoryRevoke(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	sessions := postgres.NewSessionRepository(db)
	ctx := context.Background()

	user := newStoredUser(t, users)
	session := newTestSession(user.ID)
	if err := sessions.Create(ctx, session); err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}

	if err := sessions.Revoke(ctx, session.ID); err != nil {
		t.Fatalf("Revoke returned an error: %v", err)
	}

	found, err := sessions.GetByTokenHash(ctx, session.TokenHash)
	if err != nil {
		t.Fatalf("GetByTokenHash returned an error: %v", err)
	}
	if !found.IsRevoked {
		t.Error("expected the session to be marked revoked")
	}
	if found.RevokedAt == nil {
		t.Error("expected a revocation timestamp for the audit trail")
	}
	if found.IsUsable(time.Now()) {
		t.Error("a revoked session must not be usable")
	}
}

// Revoking twice must not overwrite the original timestamp, so the second
// attempt reports that there was nothing left to revoke.
func TestSessionRepositoryRevokeIsNotRepeatable(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	sessions := postgres.NewSessionRepository(db)
	ctx := context.Background()

	user := newStoredUser(t, users)
	session := newTestSession(user.ID)
	if err := sessions.Create(ctx, session); err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}

	if err := sessions.Revoke(ctx, session.ID); err != nil {
		t.Fatalf("first Revoke returned an error: %v", err)
	}

	err := sessions.Revoke(ctx, session.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected domain.ErrNotFound on the second revoke, got %v", err)
	}
}

// Changing a password relies on this to log every other device out.
func TestSessionRepositoryRevokeAllForUser(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	sessions := postgres.NewSessionRepository(db)
	ctx := context.Background()

	user := newStoredUser(t, users)
	other := newStoredUser(t, users)

	for range 3 {
		if err := sessions.Create(ctx, newTestSession(user.ID)); err != nil {
			t.Fatalf("failed to create a session: %v", err)
		}
	}
	otherSession := newTestSession(other.ID)
	if err := sessions.Create(ctx, otherSession); err != nil {
		t.Fatalf("failed to create the other user's session: %v", err)
	}

	revoked, err := sessions.RevokeAllForUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("RevokeAllForUser returned an error: %v", err)
	}
	if revoked != 3 {
		t.Errorf("expected 3 sessions revoked, got %d", revoked)
	}

	active, err := sessions.ListActiveByUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListActiveByUser returned an error: %v", err)
	}
	if len(active) != 0 {
		t.Errorf("expected no active sessions left, got %d", len(active))
	}

	// Another user's sessions must be untouched.
	otherActive, err := sessions.ListActiveByUser(ctx, other.ID)
	if err != nil {
		t.Fatalf("ListActiveByUser returned an error: %v", err)
	}
	if len(otherActive) != 1 {
		t.Errorf("expected the other user to keep 1 active session, got %d", len(otherActive))
	}
}

// The listing must reflect what can actually authenticate: neither revoked nor
// expired sessions belong in it.
func TestSessionRepositoryListActiveExcludesRevokedAndExpired(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	sessions := postgres.NewSessionRepository(db)
	ctx := context.Background()

	user := newStoredUser(t, users)

	active := newTestSession(user.ID)
	if err := sessions.Create(ctx, active); err != nil {
		t.Fatalf("failed to create the active session: %v", err)
	}

	revoked := newTestSession(user.ID)
	if err := sessions.Create(ctx, revoked); err != nil {
		t.Fatalf("failed to create the session to revoke: %v", err)
	}
	if err := sessions.Revoke(ctx, revoked.ID); err != nil {
		t.Fatalf("failed to revoke: %v", err)
	}

	expired := newTestSession(user.ID)
	expired.ExpiresAt = time.Now().Add(-time.Minute)
	if err := sessions.Create(ctx, expired); err != nil {
		t.Fatalf("failed to create the expired session: %v", err)
	}

	listed, err := sessions.ListActiveByUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListActiveByUser returned an error: %v", err)
	}

	if len(listed) != 1 {
		t.Fatalf("expected exactly 1 active session, got %d", len(listed))
	}
	if listed[0].ID != active.ID {
		t.Errorf("expected session %q, got %q", active.ID, listed[0].ID)
	}
}
