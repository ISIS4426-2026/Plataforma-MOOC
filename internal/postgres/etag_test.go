package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/postgres"
)

// The tag must change exactly when the resource does, or a conditional write
// either never fires or fires on every request.
func TestETagChangesWhenTheResourceChanges(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	admins := postgres.NewAdminRepository(db)
	ctx := context.Background()

	newAdminUser(t, users)
	target := newStoredUser(t, users)

	before := target.ETag()

	updated, err := admins.ChangeRole(ctx, target.ID, domain.RoleProfessor, domain.ChangeOptions{},
		auditEntryFor(target.ID, target.ID, domain.AuditActionUserRoleChanged))
	if err != nil {
		t.Fatalf("ChangeRole returned an error: %v", err)
	}

	if updated.ETag() == before {
		t.Error("the tag did not change after a write, so a conditional request could never detect it")
	}
}

// Reading the same unchanged resource twice must yield the same tag, or every
// conditional write would be refused.
func TestETagIsStableWhileTheResourceIsUnchanged(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	admins := postgres.NewAdminRepository(db)
	ctx := context.Background()

	stored := newStoredUser(t, users)

	first, err := admins.GetUser(ctx, stored.ID)
	if err != nil {
		t.Fatalf("GetUser returned an error: %v", err)
	}
	second, err := admins.GetUser(ctx, stored.ID)
	if err != nil {
		t.Fatalf("GetUser returned an error: %v", err)
	}

	if first.ETag() != second.ETag() {
		t.Errorf("the tag is unstable: %s then %s", first.ETag(), second.ETag())
	}
}

// Two different resources must not share a tag, or a client could pass the
// precondition of one while holding the version of another.
func TestETagIsSpecificToTheResource(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)

	first := newStoredUser(t, users)
	second := newStoredUser(t, users)

	if first.ETag() == second.ETag() {
		t.Error("two different users produced the same tag")
	}
}

// The acceptance criterion: a write carrying a stale If-Match is refused.
func TestChangeWithAStaleETagIsRefused(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	admins := postgres.NewAdminRepository(db)
	ctx := context.Background()

	newAdminUser(t, users)
	target := newStoredUser(t, users)

	stale := target.ETag()

	// Somebody else changes the resource first.
	if _, err := admins.ChangeRole(ctx, target.ID, domain.RoleProfessor, domain.ChangeOptions{},
		auditEntryFor(target.ID, target.ID, domain.AuditActionUserRoleChanged)); err != nil {
		t.Fatalf("the first change failed: %v", err)
	}

	// The second writer still holds the version from before that change.
	_, err := admins.ChangeStatus(ctx, target.ID, domain.UserStatusSuspended,
		domain.ChangeOptions{ExpectedETag: stale},
		auditEntryFor(target.ID, target.ID, domain.AuditActionUserSuspended))

	if !errors.Is(err, domain.ErrPreconditionFailed) {
		t.Fatalf("expected domain.ErrPreconditionFailed, got %v", err)
	}

	// And the resource must be untouched by the refused write.
	current, err := admins.GetUser(ctx, target.ID)
	if err != nil {
		t.Fatalf("GetUser returned an error: %v", err)
	}
	if current.Status != domain.UserStatusActive {
		t.Errorf("the refused write was applied anyway, status is %q", current.Status)
	}
}

// A refused precondition must leave no audit entry: the transaction that would
// have written it never commits.
func TestARefusedPreconditionWritesNoAuditEntry(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	admins := postgres.NewAdminRepository(db)
	ctx := context.Background()

	newAdminUser(t, users)
	target := newStoredUser(t, users)

	_, err := admins.ChangeStatus(ctx, target.ID, domain.UserStatusSuspended,
		domain.ChangeOptions{ExpectedETag: `"una-version-que-no-existe"`},
		auditEntryFor(target.ID, target.ID, domain.AuditActionUserSuspended))
	if !errors.Is(err, domain.ErrPreconditionFailed) {
		t.Fatalf("expected the change to be refused, got %v", err)
	}

	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_logs`).Scan(&count); err != nil {
		t.Fatalf("failed to count audit entries: %v", err)
	}
	if count != 0 {
		t.Errorf("a refused precondition left %d audit entries behind", count)
	}
}

// A current tag lets the write through.
func TestChangeWithACurrentETagSucceeds(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	admins := postgres.NewAdminRepository(db)
	ctx := context.Background()

	newAdminUser(t, users)
	target := newStoredUser(t, users)

	current, err := admins.GetUser(ctx, target.ID)
	if err != nil {
		t.Fatalf("GetUser returned an error: %v", err)
	}

	updated, err := admins.ChangeStatus(ctx, target.ID, domain.UserStatusSuspended,
		domain.ChangeOptions{ExpectedETag: current.ETag()},
		auditEntryFor(target.ID, target.ID, domain.AuditActionUserSuspended))
	if err != nil {
		t.Fatalf("ChangeStatus returned an error: %v", err)
	}
	if updated.Status != domain.UserStatusSuspended {
		t.Errorf("expected the account suspended, got %q", updated.Status)
	}
}

// Without a precondition the write applies regardless, which keeps If-Match
// optional for clients that do not implement optimistic concurrency.
func TestChangeWithoutAPreconditionApplies(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	admins := postgres.NewAdminRepository(db)
	ctx := context.Background()

	newAdminUser(t, users)
	target := newStoredUser(t, users)

	if _, err := admins.ChangeStatus(ctx, target.ID, domain.UserStatusSuspended, domain.ChangeOptions{},
		auditEntryFor(target.ID, target.ID, domain.AuditActionUserSuspended)); err != nil {
		t.Errorf("a change without a precondition must apply, got %v", err)
	}
}

func TestGetUserReportsNotFound(t *testing.T) {
	db := newTestDB(t)
	admins := postgres.NewAdminRepository(db)

	_, err := admins.GetUser(context.Background(), "3fa85f64-5717-4562-b3fc-2c963f66afa6")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected domain.ErrNotFound, got %v", err)
	}
}
