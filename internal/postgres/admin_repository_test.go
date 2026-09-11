package postgres_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/postgres"
)

// newAdminUser stores an active administrator.
func newAdminUser(t *testing.T, repo *postgres.UserRepository) *domain.User {
	t.Helper()

	user := newTestUser()
	user.Role = domain.RoleAdmin
	user.Status = domain.UserStatusActive
	if err := repo.Create(context.Background(), user); err != nil {
		t.Fatalf("failed to create the administrator: %v", err)
	}
	return user
}

func auditEntryFor(actorID, targetID string, action domain.AuditAction) *domain.AuditEntry {
	return &domain.AuditEntry{
		ActorID:        &actorID,
		Action:         action,
		TargetResource: "user:" + targetID,
		IPAddress:      "172.18.0.1",
		UserAgent:      "prueba",
	}
}

// The edge case the issue asks for explicitly: the last active administrator
// cannot be suspended.
func TestChangeStatusRefusesToSuspendTheLastActiveAdmin(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	admins := postgres.NewAdminRepository(db)
	ctx := context.Background()

	onlyAdmin := newAdminUser(t, users)

	_, err := admins.ChangeStatus(ctx, onlyAdmin.ID, domain.UserStatusSuspended, domain.ChangeOptions{},
		auditEntryFor(onlyAdmin.ID, onlyAdmin.ID, domain.AuditActionUserSuspended))

	if !errors.Is(err, domain.ErrLastAdminProtected) {
		t.Fatalf("expected domain.ErrLastAdminProtected, got %v", err)
	}

	// And the account must be untouched.
	current, err := users.GetByID(ctx, onlyAdmin.ID)
	if err != nil {
		t.Fatalf("GetByID returned an error: %v", err)
	}
	if current.Status != domain.UserStatusActive {
		t.Errorf("the administrator was suspended anyway, status is %q", current.Status)
	}
}

// Nor demoted, which removes them from the active administrator set just as
// effectively as a suspension.
func TestChangeRoleRefusesToDemoteTheLastActiveAdmin(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	admins := postgres.NewAdminRepository(db)
	ctx := context.Background()

	onlyAdmin := newAdminUser(t, users)

	_, err := admins.ChangeRole(ctx, onlyAdmin.ID, domain.RoleStudent, domain.ChangeOptions{},
		auditEntryFor(onlyAdmin.ID, onlyAdmin.ID, domain.AuditActionUserRoleChanged))

	if !errors.Is(err, domain.ErrLastAdminProtected) {
		t.Fatalf("expected domain.ErrLastAdminProtected, got %v", err)
	}

	current, err := users.GetByID(ctx, onlyAdmin.ID)
	if err != nil {
		t.Fatalf("GetByID returned an error: %v", err)
	}
	if current.Role != domain.RoleAdmin {
		t.Errorf("the administrator was demoted anyway, role is %q", current.Role)
	}
}

// A suspended administrator does not count towards the active set, so the rule
// must look at status and not only at role.
func TestSuspendedAdminDoesNotCountAsTheLastOne(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	admins := postgres.NewAdminRepository(db)
	ctx := context.Background()

	active := newAdminUser(t, users)

	suspended := newTestUser()
	suspended.Role = domain.RoleAdmin
	suspended.Status = domain.UserStatusSuspended
	if err := users.Create(ctx, suspended); err != nil {
		t.Fatalf("failed to create the suspended administrator: %v", err)
	}

	// Only one administrator is active, so suspending it must still be refused.
	_, err := admins.ChangeStatus(ctx, active.ID, domain.UserStatusSuspended, domain.ChangeOptions{},
		auditEntryFor(active.ID, active.ID, domain.AuditActionUserSuspended))

	if !errors.Is(err, domain.ErrLastAdminProtected) {
		t.Errorf("a suspended administrator must not count as a replacement, got %v", err)
	}
}

// With a spare administrator the change goes through.
func TestChangeStatusSuspendsAnAdminWhenAnotherRemains(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	admins := postgres.NewAdminRepository(db)
	ctx := context.Background()

	first := newAdminUser(t, users)
	newAdminUser(t, users)

	updated, err := admins.ChangeStatus(ctx, first.ID, domain.UserStatusSuspended, domain.ChangeOptions{},
		auditEntryFor(first.ID, first.ID, domain.AuditActionUserSuspended))
	if err != nil {
		t.Fatalf("ChangeStatus returned an error: %v", err)
	}
	if updated.Status != domain.UserStatusSuspended {
		t.Errorf("expected the account suspended, got %q", updated.Status)
	}
}

// Reactivating never removes anyone from the active set, so the rule must not
// stand in its way.
func TestChangeStatusAlwaysAllowsReactivation(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	admins := postgres.NewAdminRepository(db)
	ctx := context.Background()

	suspended := newTestUser()
	suspended.Role = domain.RoleAdmin
	suspended.Status = domain.UserStatusSuspended
	if err := users.Create(ctx, suspended); err != nil {
		t.Fatalf("failed to create the suspended administrator: %v", err)
	}

	updated, err := admins.ChangeStatus(ctx, suspended.ID, domain.UserStatusActive, domain.ChangeOptions{},
		auditEntryFor(suspended.ID, suspended.ID, domain.AuditActionUserReactivated))
	if err != nil {
		t.Fatalf("ChangeStatus returned an error: %v", err)
	}
	if updated.Status != domain.UserStatusActive {
		t.Errorf("expected the account active, got %q", updated.Status)
	}
}

// Suspending an ordinary account is unaffected by the administrator rule.
func TestChangeStatusSuspendsANonAdmin(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	admins := postgres.NewAdminRepository(db)
	ctx := context.Background()

	// A single administrator exists and must stay untouched.
	newAdminUser(t, users)
	student := newStoredUser(t, users)

	updated, err := admins.ChangeStatus(ctx, student.ID, domain.UserStatusSuspended, domain.ChangeOptions{},
		auditEntryFor(student.ID, student.ID, domain.AuditActionUserSuspended))
	if err != nil {
		t.Fatalf("ChangeStatus returned an error: %v", err)
	}
	if updated.Status != domain.UserStatusSuspended {
		t.Errorf("expected the account suspended, got %q", updated.Status)
	}
}

// Two administrators being removed at the same time must not both succeed.
//
// This is the reason the rule is enforced under a row lock rather than by
// counting first and updating afterwards: without it each transaction would see
// the other administrator as still active and both would go through.
func TestConcurrentDemotionsCannotLeaveThePlatformWithoutAdmins(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	admins := postgres.NewAdminRepository(db)
	ctx := context.Background()

	first := newAdminUser(t, users)
	second := newAdminUser(t, users)

	var wg sync.WaitGroup
	results := make([]error, 2)

	for i, target := range []*domain.User{first, second} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := admins.ChangeStatus(ctx, target.ID, domain.UserStatusSuspended, domain.ChangeOptions{},
				auditEntryFor(target.ID, target.ID, domain.AuditActionUserSuspended))
			results[i] = err
		}()
	}
	wg.Wait()

	succeeded := 0
	for _, err := range results {
		if err == nil {
			succeeded++
		} else if !errors.Is(err, domain.ErrLastAdminProtected) {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if succeeded != 1 {
		t.Errorf("expected exactly one suspension to succeed, %d did", succeeded)
	}

	remaining, err := users.CountActiveAdmins(ctx)
	if err != nil {
		t.Fatalf("CountActiveAdmins returned an error: %v", err)
	}
	if remaining != 1 {
		t.Errorf("expected 1 active administrator left, got %d", remaining)
	}
}

func TestChangeStatusReportsNotFound(t *testing.T) {
	db := newTestDB(t)
	admins := postgres.NewAdminRepository(db)

	_, err := admins.ChangeStatus(context.Background(), uuid.NewString(), domain.UserStatusSuspended, domain.ChangeOptions{},
		auditEntryFor(uuid.NewString(), uuid.NewString(), domain.AuditActionUserSuspended))

	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected domain.ErrNotFound, got %v", err)
	}
}

// "Toda acción queda reflejada en el registro de auditoría": the entry is
// written in the same transaction as the change.
func TestAdministrativeChangesAreAudited(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	admins := postgres.NewAdminRepository(db)
	ctx := context.Background()

	actor := newAdminUser(t, users)
	target := newStoredUser(t, users)

	entry := auditEntryFor(actor.ID, target.ID, domain.AuditActionUserRoleChanged)
	if _, err := admins.ChangeRole(ctx, target.ID, domain.RoleProfessor, domain.ChangeOptions{}, entry); err != nil {
		t.Fatalf("ChangeRole returned an error: %v", err)
	}

	if entry.ID == "" {
		t.Error("expected the audit entry to be assigned an id")
	}
	if entry.CreatedAt.IsZero() {
		t.Error("expected the audit entry to be timestamped")
	}

	var (
		action    string
		targetRes string
		actorID   string
		details   []byte
	)
	err := db.QueryRowContext(ctx,
		`SELECT action, target_resource, actor_id, details FROM audit_logs WHERE id = $1`,
		entry.ID,
	).Scan(&action, &targetRes, &actorID, &details)
	if err != nil {
		t.Fatalf("the audit entry was not persisted: %v", err)
	}

	if action != string(domain.AuditActionUserRoleChanged) {
		t.Errorf("expected action %q, got %q", domain.AuditActionUserRoleChanged, action)
	}
	if targetRes != "user:"+target.ID {
		t.Errorf("expected target %q, got %q", "user:"+target.ID, targetRes)
	}
	if actorID != actor.ID {
		t.Errorf("expected actor %q, got %q", actor.ID, actorID)
	}

	// The transition is recorded so the trail says what changed, not only that
	// something did.
	rendered := string(details)
	for _, expected := range []string{string(domain.RoleStudent), string(domain.RoleProfessor)} {
		if !strings.Contains(rendered, expected) {
			t.Errorf("expected the details to record %q, got %s", expected, rendered)
		}
	}
}

// A refused change must leave no audit entry: the transaction that would have
// written it never commits.
func TestARefusedChangeWritesNoAuditEntry(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	admins := postgres.NewAdminRepository(db)
	ctx := context.Background()

	onlyAdmin := newAdminUser(t, users)

	entry := auditEntryFor(onlyAdmin.ID, onlyAdmin.ID, domain.AuditActionUserSuspended)
	if _, err := admins.ChangeStatus(ctx, onlyAdmin.ID, domain.UserStatusSuspended, domain.ChangeOptions{}, entry); !errors.Is(err, domain.ErrLastAdminProtected) {
		t.Fatalf("expected the change to be refused, got %v", err)
	}

	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_logs`).Scan(&count); err != nil {
		t.Fatalf("failed to count audit entries: %v", err)
	}
	if count != 0 {
		t.Errorf("a refused change left %d audit entries behind", count)
	}
}
