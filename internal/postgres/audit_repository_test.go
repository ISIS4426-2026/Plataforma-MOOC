package postgres_test

import (
	"context"
	"testing"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/postgres"
)

func recordTestEntry(t *testing.T, repo *postgres.AuditRepository, actorID string, action domain.AuditAction, target string) *domain.AuditEntry {
	t.Helper()

	entry := &domain.AuditEntry{
		ActorID:        &actorID,
		Action:         action,
		TargetResource: target,
	}
	if err := repo.Record(context.Background(), entry); err != nil {
		t.Fatalf("failed to record audit entry: %v", err)
	}
	return entry
}

// The acceptance test issue #18 asks for directly: any attempt to modify an
// existing audit entry always fails.
func TestAuditLogsRejectsUpdate(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	audit := postgres.NewAuditRepository(db)
	ctx := context.Background()

	actor := newTestUser()
	if err := users.Create(ctx, actor); err != nil {
		t.Fatalf("failed to create the actor: %v", err)
	}
	entry := recordTestEntry(t, audit, actor.ID, domain.AuditActionLoginFailed, "user:"+actor.ID)

	_, err := db.ExecContext(ctx, `UPDATE audit_logs SET action = 'user.role_changed' WHERE id = $1`, entry.ID)
	if err == nil {
		t.Fatal("UPDATE on an existing audit entry succeeded, want it rejected")
	}
}

func TestAuditLogsRejectsDelete(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	audit := postgres.NewAuditRepository(db)
	ctx := context.Background()

	actor := newTestUser()
	if err := users.Create(ctx, actor); err != nil {
		t.Fatalf("failed to create the actor: %v", err)
	}
	entry := recordTestEntry(t, audit, actor.ID, domain.AuditActionLoginFailed, "user:"+actor.ID)

	_, err := db.ExecContext(ctx, `DELETE FROM audit_logs WHERE id = $1`, entry.ID)
	if err == nil {
		t.Fatal("DELETE of an existing audit entry succeeded, want it rejected")
	}
}

func TestAuditRepositoryListFiltersByActionPrefixAndActor(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	audit := postgres.NewAuditRepository(db)
	ctx := context.Background()

	actor := newTestUser()
	if err := users.Create(ctx, actor); err != nil {
		t.Fatalf("failed to create the actor: %v", err)
	}
	other := newTestUser()
	if err := users.Create(ctx, other); err != nil {
		t.Fatalf("failed to create the other actor: %v", err)
	}

	recordTestEntry(t, audit, actor.ID, domain.AuditActionCourseNewVersion, "course:c1")
	recordTestEntry(t, audit, actor.ID, domain.AuditActionCourseNewVersion, "course:c1")
	recordTestEntry(t, audit, actor.ID, domain.AuditActionLoginFailed, "user:"+actor.ID)
	recordTestEntry(t, audit, other.ID, domain.AuditActionCourseNewVersion, "course:c2")

	page, err := audit.List(ctx, domain.AuditFilter{ActorID: actor.ID, ActionPrefix: "course."})
	if err != nil {
		t.Fatalf("List returned an error: %v", err)
	}

	if len(page.Entries) != 2 {
		t.Fatalf("got %d entries, want 2 (the two course.* entries recorded for actor)", len(page.Entries))
	}
	for _, e := range page.Entries {
		if e.ActorID == nil || *e.ActorID != actor.ID {
			t.Errorf("entry actor = %v, want %q", e.ActorID, actor.ID)
		}
		if e.Action != domain.AuditActionCourseNewVersion {
			t.Errorf("entry action = %q, want a course.* action", e.Action)
		}
	}
}

func TestAuditRepositoryListByTargetResource(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	audit := postgres.NewAuditRepository(db)
	ctx := context.Background()

	actor := newTestUser()
	if err := users.Create(ctx, actor); err != nil {
		t.Fatalf("failed to create the actor: %v", err)
	}

	recordTestEntry(t, audit, actor.ID, domain.AuditActionCourseNewVersion, "course:target-1")
	recordTestEntry(t, audit, actor.ID, domain.AuditActionCourseNewVersion, "course:target-1")
	recordTestEntry(t, audit, actor.ID, domain.AuditActionCourseNewVersion, "course:target-2")

	page, err := audit.List(ctx, domain.AuditFilter{TargetResource: "course:target-1"})
	if err != nil {
		t.Fatalf("List returned an error: %v", err)
	}
	if len(page.Entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(page.Entries))
	}
}

func TestAuditRepositoryListPaginates(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	audit := postgres.NewAuditRepository(db)
	ctx := context.Background()

	actor := newTestUser()
	if err := users.Create(ctx, actor); err != nil {
		t.Fatalf("failed to create the actor: %v", err)
	}

	const total = 5
	for i := 0; i < total; i++ {
		recordTestEntry(t, audit, actor.ID, domain.AuditActionCourseNewVersion, "course:page-test")
	}

	first, err := audit.List(ctx, domain.AuditFilter{TargetResource: "course:page-test", Limit: 3})
	if err != nil {
		t.Fatalf("List returned an error: %v", err)
	}
	if len(first.Entries) != 3 || !first.HasMore {
		t.Fatalf("first page: got %d entries, HasMore=%v; want 3 entries and HasMore=true", len(first.Entries), first.HasMore)
	}

	second, err := audit.List(ctx, domain.AuditFilter{TargetResource: "course:page-test", Limit: 3, Cursor: first.NextCursor})
	if err != nil {
		t.Fatalf("second List returned an error: %v", err)
	}
	if len(second.Entries) != 2 || second.HasMore {
		t.Fatalf("second page: got %d entries, HasMore=%v; want 2 entries and HasMore=false", len(second.Entries), second.HasMore)
	}

	seen := make(map[string]bool, total)
	for _, e := range append(first.Entries, second.Entries...) {
		if seen[e.ID] {
			t.Errorf("entry %s appeared on both pages", e.ID)
		}
		seen[e.ID] = true
	}
	if len(seen) != total {
		t.Errorf("saw %d distinct entries across both pages, want %d", len(seen), total)
	}
}
