package postgres_test

import (
	"context"
	"sync"
	"testing"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/postgres"
)

// E2 (#217): ocho inscripciones simultaneas del mismo estudiante dejan una fila y
// un solo evento. Antes de la correccion la fila era una pero los ocho
// registraban «enrollment.created», porque en la primera inscripcion FOR UPDATE
// no tiene fila que bloquear.
func TestEnrollmentRepositoryConcurrentFirstEnrollmentIsOneRowAndOneEvent(t *testing.T) {
	db := newTestDB(t)
	fixture := newProgressFixture(t, db, 1)
	repo := postgres.NewEnrollmentRepository(db)
	ctx := context.Background()

	users := postgres.NewUserRepository(db)

	// Una sola ronda no basta: contra una base local la ventana es de microsegundos
	// y casi nunca se abre. Con varios estudiantes y muchas llamadas por estudiante
	// la carrera aparece de forma fiable sin la correccion.
	const (
		students = 12
		callers  = 16
	)
	for n := 0; n < students; n++ {
		student := newTestUser()
		student.Status = domain.UserStatusActive
		if err := users.Create(ctx, student); err != nil {
			t.Fatalf("failed to create the student: %v", err)
		}

		var wg sync.WaitGroup
		errs := make(chan error, callers)
		start := make(chan struct{})
		for i := 0; i < callers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				actor := student.ID
				_, err := repo.Enroll(ctx, student.ID, fixture.course.StableID, &domain.AuditEntry{
					ActorID: &actor, TargetResource: "course:" + fixture.course.StableID,
				})
				errs <- err
			}()
		}
		close(start)
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("a concurrent Enroll returned an error: %v", err)
			}
		}

		var rows, events int
		if err := db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM enrollments WHERE student_id = $1 AND course_stable_id = $2`,
			student.ID, fixture.course.StableID,
		).Scan(&rows); err != nil {
			t.Fatalf("count enrollments: %v", err)
		}
		if err := db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM audit_logs WHERE actor_id = $1 AND target_resource = $2 AND action LIKE 'enrollment.%'`,
			student.ID, "course:"+fixture.course.StableID,
		).Scan(&events); err != nil {
			t.Fatalf("count audit entries: %v", err)
		}

		if rows != 1 || events != 1 {
			t.Fatalf("student %d: enrollment rows = %d (want 1), audit entries = %d (want 1)", n, rows, events)
		}
	}
}
