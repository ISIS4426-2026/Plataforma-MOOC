package postgres_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/postgres"
)

// newTestDB provisions an isolated Postgres schema for the calling test and
// applies the real migrations into it.
//
// Isolation is not cosmetic here: `go test ./...` runs packages in parallel, and
// the migrations package drops and recreates every table in the default schema.
// Sharing that schema would make these tests fail intermittently depending on
// interleaving, so each run gets a private namespace that is dropped afterwards.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()

	baseURL := os.Getenv("DATABASE_URL")
	if baseURL == "" {
		t.Skip("DATABASE_URL not set; skipping postgres repository integration test")
	}

	admin, err := sql.Open("postgres", baseURL)
	if err != nil {
		t.Fatalf("failed to open postgres connection: %v", err)
	}
	defer admin.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := admin.PingContext(ctx); err != nil {
		t.Skipf("postgres not reachable: %v; skipping integration test", err)
	}

	// The uuid suffix keeps concurrent packages and repeated local runs from
	// colliding on the same schema name.
	schema := "repo_test_" + uuid.NewString()[:8]
	if _, err := admin.ExecContext(ctx, fmt.Sprintf("CREATE SCHEMA %q", schema)); err != nil {
		t.Fatalf("failed to create test schema: %v", err)
	}

	scopedURL, err := withSearchPath(baseURL, schema)
	if err != nil {
		t.Fatalf("failed to build scoped connection URL: %v", err)
	}

	db, err := sql.Open("postgres", scopedURL)
	if err != nil {
		t.Fatalf("failed to open scoped connection: %v", err)
	}

	t.Cleanup(func() {
		db.Close()

		cleanupCtx, cancelCleanup := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancelCleanup()

		cleanup, err := sql.Open("postgres", baseURL)
		if err != nil {
			t.Logf("failed to open cleanup connection: %v", err)
			return
		}
		defer cleanup.Close()

		if _, err := cleanup.ExecContext(cleanupCtx, fmt.Sprintf("DROP SCHEMA IF EXISTS %q CASCADE", schema)); err != nil {
			t.Logf("failed to drop test schema %s: %v", schema, err)
		}
	})

	// Applying the committed migrations rather than a hand-written DDL keeps the
	// tests honest: a schema change that breaks these queries fails here.
	//
	// The set is discovered by globbing rather than named explicitly: a
	// hardcoded list is exactly the failure mode the comment on
	// scripts/init-db.sh warns about — a migration added later silently isn't
	// applied here, and a test schema quietly diverges from what a real
	// deployment runs (a trigger that "works" in the suite because it was
	// never actually created in the schema under test, for instance).
	migrationsDir := filepath.Join("..", "..", "migrations")
	upPaths, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil {
		t.Fatalf("failed to list migration files: %v", err)
	}
	sort.Strings(upPaths)

	for _, path := range upPaths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read migration %s: %v", path, err)
		}
		if _, err := db.ExecContext(ctx, string(content)); err != nil {
			t.Fatalf("failed to apply migration %s: %v", path, err)
		}
	}

	return db
}

// withSearchPath appends the schema as a libpq runtime parameter so every
// pooled connection resolves unqualified table names inside it.
func withSearchPath(rawURL, schema string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()

	return parsed.String(), nil
}

func newTestUser() *domain.User {
	return &domain.User{
		// A unique address per user keeps the UNIQUE constraint from turning
		// unrelated tests into failures.
		Email:        fmt.Sprintf("student-%s@example.test", uuid.NewString()),
		PasswordHash: "$2a$10$abcdefghijklmnopqrstuvwxyz012345678901234567890123456",
		FullName:     "Estudiante de Prueba",
		Role:         domain.RoleStudent,
		Status:       domain.UserStatusPendingVerification,
	}
}

func TestUserRepositoryCreateAssignsGeneratedFields(t *testing.T) {
	db := newTestDB(t)
	repo := postgres.NewUserRepository(db)
	ctx := context.Background()

	user := newTestUser()
	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}

	if user.ID == "" {
		t.Error("expected Create to populate the database-generated id")
	}
	if user.CreatedAt.IsZero() {
		t.Error("expected Create to populate created_at")
	}
	if user.UpdatedAt.IsZero() {
		t.Error("expected Create to populate updated_at")
	}
}

func TestUserRepositoryCreateAndGetByIDRoundTrip(t *testing.T) {
	db := newTestDB(t)
	repo := postgres.NewUserRepository(db)
	ctx := context.Background()

	user := newTestUser()
	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}

	found, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetByID returned an error: %v", err)
	}

	if found.Email != user.Email {
		t.Errorf("expected email %q, got %q", user.Email, found.Email)
	}
	if found.Role != domain.RoleStudent {
		t.Errorf("expected role %q, got %q", domain.RoleStudent, found.Role)
	}
	if found.Status != domain.UserStatusPendingVerification {
		t.Errorf("expected status %q, got %q", domain.UserStatusPendingVerification, found.Status)
	}
	if found.PasswordHash != user.PasswordHash {
		t.Error("expected the stored password hash to round-trip unchanged")
	}
}

func TestUserRepositoryGetByEmail(t *testing.T) {
	db := newTestDB(t)
	repo := postgres.NewUserRepository(db)
	ctx := context.Background()

	user := newTestUser()
	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}

	found, err := repo.GetByEmail(ctx, user.Email)
	if err != nil {
		t.Fatalf("GetByEmail returned an error: %v", err)
	}
	if found.ID != user.ID {
		t.Errorf("expected id %q, got %q", user.ID, found.ID)
	}
}

// Registering an address that already exists must surface as a domain conflict
// rather than a driver-specific error, so the HTTP layer can map it to 409
// without importing the database package.
func TestUserRepositoryCreateRejectsDuplicateEmailAsConflict(t *testing.T) {
	db := newTestDB(t)
	repo := postgres.NewUserRepository(db)
	ctx := context.Background()

	first := newTestUser()
	if err := repo.Create(ctx, first); err != nil {
		t.Fatalf("first Create returned an error: %v", err)
	}

	second := newTestUser()
	second.Email = first.Email

	err := repo.Create(ctx, second)
	if !errors.Is(err, domain.ErrConflict) {
		t.Errorf("expected domain.ErrConflict, got %v", err)
	}
}

func TestUserRepositoryGetByIDReportsNotFound(t *testing.T) {
	db := newTestDB(t)
	repo := postgres.NewUserRepository(db)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, uuid.NewString())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected domain.ErrNotFound, got %v", err)
	}
}

func TestUserRepositoryGetByEmailReportsNotFound(t *testing.T) {
	db := newTestDB(t)
	repo := postgres.NewUserRepository(db)
	ctx := context.Background()

	_, err := repo.GetByEmail(ctx, "nobody-"+uuid.NewString()+"@example.test")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected domain.ErrNotFound, got %v", err)
	}
}

func TestUserRepositoryUpdatePersistsChanges(t *testing.T) {
	db := newTestDB(t)
	repo := postgres.NewUserRepository(db)
	ctx := context.Background()

	user := newTestUser()
	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}
	createdAt := user.UpdatedAt

	// Activation after email verification is the transition issue #9 depends on.
	user.Status = domain.UserStatusActive
	user.FullName = "Nombre Actualizado"
	if err := repo.Update(ctx, user); err != nil {
		t.Fatalf("Update returned an error: %v", err)
	}

	found, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetByID returned an error: %v", err)
	}

	if found.Status != domain.UserStatusActive {
		t.Errorf("expected status %q, got %q", domain.UserStatusActive, found.Status)
	}
	if found.FullName != "Nombre Actualizado" {
		t.Errorf("expected the updated name, got %q", found.FullName)
	}
	if found.UpdatedAt.Before(createdAt) {
		t.Error("expected updated_at to be refreshed by Update")
	}
}

func TestUserRepositoryUpdateReportsNotFound(t *testing.T) {
	db := newTestDB(t)
	repo := postgres.NewUserRepository(db)
	ctx := context.Background()

	ghost := newTestUser()
	ghost.ID = uuid.NewString()

	err := repo.Update(ctx, ghost)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected domain.ErrNotFound, got %v", err)
	}
}

// CountActiveAdmins backs the "last active administrator" rule of issue #12, so
// it must count only administrators that are actually active.
func TestUserRepositoryCountActiveAdmins(t *testing.T) {
	db := newTestDB(t)
	repo := postgres.NewUserRepository(db)
	ctx := context.Background()

	initial, err := repo.CountActiveAdmins(ctx)
	if err != nil {
		t.Fatalf("CountActiveAdmins returned an error: %v", err)
	}
	if initial != 0 {
		t.Fatalf("expected an empty schema to report 0 active admins, got %d", initial)
	}

	activeAdmin := newTestUser()
	activeAdmin.Role = domain.RoleAdmin
	activeAdmin.Status = domain.UserStatusActive
	if err := repo.Create(ctx, activeAdmin); err != nil {
		t.Fatalf("failed to create the active admin: %v", err)
	}

	suspendedAdmin := newTestUser()
	suspendedAdmin.Role = domain.RoleAdmin
	suspendedAdmin.Status = domain.UserStatusSuspended
	if err := repo.Create(ctx, suspendedAdmin); err != nil {
		t.Fatalf("failed to create the suspended admin: %v", err)
	}

	activeStudent := newTestUser()
	activeStudent.Status = domain.UserStatusActive
	if err := repo.Create(ctx, activeStudent); err != nil {
		t.Fatalf("failed to create the active student: %v", err)
	}

	count, err := repo.CountActiveAdmins(ctx)
	if err != nil {
		t.Fatalf("CountActiveAdmins returned an error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 active admin, got %d", count)
	}
}
