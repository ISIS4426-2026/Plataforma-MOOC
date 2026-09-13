package postgres_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/auth"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/postgres"
)

// TestSyntheticDataSeed verifies that scripts/seeds/synthetic_data.sql
// loads a fully deterministic dataset containing users across all 3 roles,
// courses across different states (published, draft, unpublished), valid hierarchy,
// and verifies that clean_data.sql successfully cleans all tables.
func TestSyntheticDataSeed(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	// Locate seed files relative to repo root
	repoRoot := filepath.Join("..", "..")
	seedSQLPath := filepath.Join(repoRoot, "scripts", "seeds", "synthetic_data.sql")
	cleanSQLPath := filepath.Join(repoRoot, "scripts", "seeds", "clean_data.sql")

	seedSQL, err := os.ReadFile(seedSQLPath)
	if err != nil {
		t.Fatalf("failed to read seed sql at %s: %v", seedSQLPath, err)
	}

	cleanSQL, err := os.ReadFile(cleanSQLPath)
	if err != nil {
		t.Fatalf("failed to read clean sql at %s: %v", cleanSQLPath, err)
	}

	// 1. Execute Seed
	if _, err := db.ExecContext(ctx, string(seedSQL)); err != nil {
		t.Fatalf("failed to execute synthetic_data.sql: %v", err)
	}

	// 2. Validate Users across all 3 roles
	usersRepo := postgres.NewUserRepository(db)

	admin, err := usersRepo.GetByEmail(ctx, "admin@plataforma-mooc.test")
	if err != nil {
		t.Fatalf("expected seeded admin to exist: %v", err)
	}
	if admin.Role != domain.RoleAdmin || admin.ID != "a0000000-0000-0000-0000-000000000001" {
		t.Errorf("unexpected admin fields: id=%s, role=%s", admin.ID, admin.Role)
	}
	if err := auth.VerifyPassword(admin.PasswordHash, "Password123!"); err != nil {
		t.Errorf("admin password hash did not verify against 'Password123!': %v", err)
	}

	prof, err := usersRepo.GetByEmail(ctx, "profesor1@plataforma-mooc.test")
	if err != nil {
		t.Fatalf("expected seeded professor to exist: %v", err)
	}
	if prof.Role != domain.RoleProfessor || prof.ID != "b0000000-0000-0000-0000-000000000001" {
		t.Errorf("unexpected professor fields: id=%s, role=%s", prof.ID, prof.Role)
	}

	student, err := usersRepo.GetByEmail(ctx, "estudiante1@plataforma-mooc.test")
	if err != nil {
		t.Fatalf("expected seeded student to exist: %v", err)
	}
	if student.Role != domain.RoleStudent || student.ID != "c0000000-0000-0000-0000-000000000001" {
		t.Errorf("unexpected student fields: id=%s, role=%s", student.ID, student.Role)
	}

	pendingStudent, err := usersRepo.GetByEmail(ctx, "estudiante.pendiente@plataforma-mooc.test")
	if err != nil {
		t.Fatalf("expected seeded pending student to exist: %v", err)
	}
	if pendingStudent.Status != domain.UserStatusPendingVerification {
		t.Errorf("expected pending_verification, got: %s", pendingStudent.Status)
	}

	suspendedStudent, err := usersRepo.GetByEmail(ctx, "estudiante.suspendido@plataforma-mooc.test")
	if err != nil {
		t.Fatalf("expected seeded suspended student to exist: %v", err)
	}
	if suspendedStudent.Status != domain.UserStatusSuspended {
		t.Errorf("expected suspended, got: %s", suspendedStudent.Status)
	}

	// 3. Validate Courses in distinct states (published, draft, unpublished)
	courseRepo := postgres.NewCourseRepository(db)

	publishedCourse, err := courseRepo.GetByID(ctx, "d0000000-0000-0000-0000-000000000001")
	if err != nil {
		t.Fatalf("expected published course to exist: %v", err)
	}
	if publishedCourse.Status != domain.CourseStatusPublished {
		t.Errorf("expected course status 'published', got: %s", publishedCourse.Status)
	}
	if publishedCourse.StableID != "e0000000-0000-0000-0000-000000000001" {
		t.Errorf("expected stable_id 'e0000000-0000-0000-0000-000000000001', got: %s", publishedCourse.StableID)
	}

	draftCourse, err := courseRepo.GetByID(ctx, "d0000000-0000-0000-0000-000000000002")
	if err != nil {
		t.Fatalf("expected draft course to exist: %v", err)
	}
	if draftCourse.Status != domain.CourseStatusDraft {
		t.Errorf("expected course status 'draft', got: %s", draftCourse.Status)
	}

	unpubCourse, err := courseRepo.GetByID(ctx, "d0000000-0000-0000-0000-000000000004")
	if err != nil {
		t.Fatalf("expected unpublished course to exist: %v", err)
	}
	if unpubCourse.Status != domain.CourseStatusUnpublished {
		t.Errorf("expected course status 'unpublished', got: %s", unpubCourse.Status)
	}

	// 4. Validate Structural Hierarchy of Published Course
	moduleRepo := postgres.NewModuleRepository(db)
	modules, err := moduleRepo.ListByCourse(ctx, publishedCourse.ID)
	if err != nil {
		t.Fatalf("failed to list modules: %v", err)
	}
	if len(modules) != 2 {
		t.Errorf("expected 2 modules in published course, got: %d", len(modules))
	}

	unitRepo := postgres.NewUnitRepository(db)
	units, err := unitRepo.ListByModule(ctx, modules[0].ID)
	if err != nil {
		t.Fatalf("failed to list units: %v", err)
	}
	if len(units) != 1 {
		t.Errorf("expected 1 unit in module 1, got: %d", len(units))
	}

	resourceRepo := postgres.NewResourceRepository(db)
	resources, err := resourceRepo.ListByUnit(ctx, units[0].ID)
	if err != nil {
		t.Fatalf("failed to list resources: %v", err)
	}
	if len(resources) != 2 {
		t.Errorf("expected 2 resources in unit 1.1, got: %d", len(resources))
	}

	// 5. Validate Quizzes and Options exist
	var questionCount int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM quiz_questions WHERE quiz_id = 'fa000000-0000-0000-0000-000000000001'").Scan(&questionCount)
	if err != nil || questionCount != 2 {
		t.Errorf("expected 2 quiz questions, got %d (err: %v)", questionCount, err)
	}

	// 6. Validate Badges and Student Progress
	var badgeCount int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM badges WHERE student_id = 'c0000000-0000-0000-0000-000000000002'").Scan(&badgeCount)
	if err != nil || badgeCount != 1 {
		t.Errorf("expected 1 badge for student 2, got %d (err: %v)", badgeCount, err)
	}

	// 7. Validate Idempotent re-run of Seed
	if _, err := db.ExecContext(ctx, string(seedSQL)); err != nil {
		t.Fatalf("seed re-execution must be idempotent: %v", err)
	}

	// 8. Validate Clean Script (TRUNCATE CASCADE)
	if _, err := db.ExecContext(ctx, string(cleanSQL)); err != nil {
		t.Fatalf("failed to execute clean_data.sql: %v", err)
	}

	var remainingUsers int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&remainingUsers); err != nil {
		t.Fatalf("failed to count users after clean: %v", err)
	}
	if remainingUsers != 0 {
		t.Errorf("expected 0 users after clean, got: %d", remainingUsers)
	}
}
