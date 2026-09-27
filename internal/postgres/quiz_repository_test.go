package postgres_test

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/postgres"
)

// quizFixture es un curso en borrador con un recurso de tipo quiz, su
// cuestionario y un estudiante que pueda presentarlo.
//
// El curso se deja en borrador a proposito: la autoria de quizzes pasa por
// lockResourceAndCourse igual que el resto, y sobre un curso publicado seria
// rechazada. La inmutabilidad se comprueba aparte.
type quizFixture struct {
	quiz    *domain.Quiz
	student *domain.User
	author  *domain.User
	repo    *postgres.QuizRepository
}

func newQuizFixture(t *testing.T, db *sql.DB, preguntas int) *quizFixture {
	t.Helper()
	ctx := context.Background()

	users := postgres.NewUserRepository(db)
	courses := postgres.NewCourseRepository(db)
	modules := postgres.NewModuleRepository(db)
	units := postgres.NewUnitRepository(db)
	resources := postgres.NewResourceRepository(db)
	quizzes := postgres.NewQuizRepository(db)

	author := newAuthorUser(t, users)
	course := newDraftCourseForStructure(t, courses, author.ID)

	module := newModule(course.ID)
	if err := modules.Create(ctx, module, structureAuditEntry(author.ID, domain.AuditActionModuleCreated, "module:"+module.ID)); err != nil {
		t.Fatalf("failed to create module: %v", err)
	}
	unit := newUnit(module.ID)
	if err := units.Create(ctx, unit, structureAuditEntry(author.ID, domain.AuditActionUnitCreated, "unit:"+unit.ID)); err != nil {
		t.Fatalf("failed to create unit: %v", err)
	}
	resource := newResource(unit.ID)
	resource.Type = domain.ResourceTypeQuiz
	if err := resources.Create(ctx, resource, structureAuditEntry(author.ID, domain.AuditActionResourceCreated, "resource:"+resource.ID)); err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	created, err := quizzes.Create(ctx, &domain.Quiz{
		ResourceID: resource.ID, Title: "Evaluación", PassingScore: 70, MaxAttempts: 3,
	}, quizEntry(author.ID, "resource:"+resource.ID))
	if err != nil {
		t.Fatalf("failed to create quiz: %v", err)
	}

	// Cada pregunta tiene dos opciones y la primera es la correcta, para que
	// las pruebas puedan construir un envio perfecto sin leer la clave.
	for i := 0; i < preguntas; i++ {
		if _, err := quizzes.AddQuestion(ctx, created.ID, &domain.Question{
			Prompt: "¿Pregunta?",
			Options: []domain.QuestionOption{
				{Text: "Correcta", IsCorrect: true},
				{Text: "Incorrecta", IsCorrect: false},
			},
		}, quizEntry(author.ID, "quiz:"+created.ID)); err != nil {
			t.Fatalf("failed to add question %d: %v", i, err)
		}
	}

	student := newTestUser()
	student.Status = domain.UserStatusActive
	if err := users.Create(ctx, student); err != nil {
		t.Fatalf("failed to create the student: %v", err)
	}

	return &quizFixture{quiz: created, student: student, author: author, repo: quizzes}
}

func quizEntry(actorID, target string) *domain.AuditEntry {
	return &domain.AuditEntry{ActorID: &actorID, TargetResource: target}
}

// answersFor construye un envio eligiendo, en cada pregunta, la opcion
// correcta o una incorrecta.
//
// Selecciona por el valor de is_correct y no por indice a proposito: asi la
// prueba comprueba la calificacion y no el orden en que se devuelven las
// opciones, que es asunto de otra prueba.
func (f *quizFixture) answersFor(t *testing.T, acertando bool) []domain.Answer {
	t.Helper()
	full, err := f.repo.GetForAuthor(context.Background(), f.quiz.ID)
	if err != nil {
		t.Fatalf("GetForAuthor returned an error: %v", err)
	}
	answers := make([]domain.Answer, 0, len(full.Questions))
	for _, question := range full.Questions {
		elegida := ""
		for _, option := range question.Options {
			if option.IsCorrect == acertando {
				elegida = option.ID
				break
			}
		}
		if elegida == "" {
			t.Fatalf("la pregunta %s no tiene ninguna opción con is_correct=%v", question.ID, acertando)
		}
		answers = append(answers, domain.Answer{QuestionID: question.ID, SelectedOptionID: elegida})
	}
	return answers
}

func (f *quizFixture) entry() *domain.AuditEntry {
	return quizEntry(f.student.ID, "quiz:"+f.quiz.ID)
}

// Tercer criterio de aceptacion: las opciones correctas no aparecen en ninguna
// respuesta al estudiante. Se comprueba en la capa que las carga, no en la que
// las serializa: la consulta del estudiante no selecciona is_correct.
func TestQuizRepositoryStudentViewNeverCarriesTheKey(t *testing.T) {
	db := newTestDB(t)
	fixture := newQuizFixture(t, db, 3)
	ctx := context.Background()

	autor, err := fixture.repo.GetForAuthor(ctx, fixture.quiz.ID)
	if err != nil {
		t.Fatalf("GetForAuthor returned an error: %v", err)
	}
	// Primero, que la clave exista de verdad: si no, la prueba del estudiante
	// pasaria por vacuidad.
	correctas := 0
	for _, question := range autor.Questions {
		for _, option := range question.Options {
			if option.IsCorrect {
				correctas++
			}
		}
	}
	if correctas != 3 {
		t.Fatalf("el autor ve %d opciones correctas, se esperaban 3", correctas)
	}

	estudiante, err := fixture.repo.GetForStudent(ctx, fixture.quiz.ID)
	if err != nil {
		t.Fatalf("GetForStudent returned an error: %v", err)
	}
	if len(estudiante.Questions) != 3 {
		t.Fatalf("el estudiante ve %d preguntas, se esperaban 3", len(estudiante.Questions))
	}
	for _, question := range estudiante.Questions {
		if len(question.Options) != 2 {
			t.Errorf("la pregunta %s tiene %d opciones, se esperaban 2", question.ID, len(question.Options))
		}
		for _, option := range question.Options {
			if option.IsCorrect {
				t.Errorf("la opción %s llega al estudiante marcada como correcta", option.ID)
			}
		}
	}
}

func TestQuizRepositoryGradesOnTheServer(t *testing.T) {
	db := newTestDB(t)
	fixture := newQuizFixture(t, db, 4)
	ctx := context.Background()

	perfecto, err := fixture.repo.Grade(ctx, fixture.quiz.ID, fixture.student.ID, fixture.answersFor(t, true), fixture.entry())
	if err != nil {
		t.Fatalf("Grade returned an error: %v", err)
	}
	if perfecto.Submission.Score != 100 || !perfecto.Submission.Passed {
		t.Errorf("respuestas todas correctas dieron %d y passed=%v", perfecto.Submission.Score, perfecto.Submission.Passed)
	}
	if perfecto.Submission.Attempt != 1 {
		t.Errorf("Attempt = %d en el primer envío, se esperaba 1", perfecto.Submission.Attempt)
	}
	if perfecto.AttemptsRemaining != 2 {
		t.Errorf("AttemptsRemaining = %d, se esperaban 2", perfecto.AttemptsRemaining)
	}

	nulo, err := fixture.repo.Grade(ctx, fixture.quiz.ID, fixture.student.ID, fixture.answersFor(t, false), fixture.entry())
	if err != nil {
		t.Fatalf("Grade returned an error: %v", err)
	}
	if nulo.Submission.Score != 0 || nulo.Submission.Passed {
		t.Errorf("respuestas todas incorrectas dieron %d y passed=%v", nulo.Submission.Score, nulo.Submission.Passed)
	}
	if nulo.Submission.Attempt != 2 {
		t.Errorf("Attempt = %d en el segundo envío, se esperaba 2", nulo.Submission.Attempt)
	}
}

// Una opcion correcta de otro quiz no debe sumar punto. Es el agujero que deja
// calificar comparando solo por id de opcion, sin comprobar a que quiz
// pertenece.
func TestQuizRepositoryIgnoresOptionsFromAnotherQuiz(t *testing.T) {
	db := newTestDB(t)
	propio := newQuizFixture(t, db, 2)
	ajeno := newQuizFixture(t, db, 2)
	ctx := context.Background()

	// Responder las preguntas del quiz propio con las opciones correctas del
	// ajeno.
	otras := ajeno.answersFor(t, true)
	mias := propio.answersFor(t, true)
	mezcladas := make([]domain.Answer, 0, len(mias))
	for i := range mias {
		mezcladas = append(mezcladas, domain.Answer{
			QuestionID: mias[i].QuestionID, SelectedOptionID: otras[i].SelectedOptionID,
		})
	}

	grade, err := propio.repo.Grade(ctx, propio.quiz.ID, propio.student.ID, mezcladas, propio.entry())
	if err != nil {
		t.Fatalf("Grade returned an error: %v", err)
	}
	if grade.Submission.Score != 0 {
		t.Errorf("responder con opciones de otro quiz dio %d, se esperaba 0", grade.Submission.Score)
	}
}

// Segundo criterio de aceptacion: superar el limite devuelve conflicto y no
// consume un intento extra.
func TestQuizRepositoryRefusesBeyondTheAttemptLimit(t *testing.T) {
	db := newTestDB(t)
	fixture := newQuizFixture(t, db, 2)
	ctx := context.Background()

	for i := 1; i <= 3; i++ {
		grade, err := fixture.repo.Grade(ctx, fixture.quiz.ID, fixture.student.ID, fixture.answersFor(t, false), fixture.entry())
		if err != nil {
			t.Fatalf("el intento %d falló: %v", i, err)
		}
		if grade.Submission.Attempt != i {
			t.Errorf("Attempt = %d en el intento %d", grade.Submission.Attempt, i)
		}
	}

	_, err := fixture.repo.Grade(ctx, fixture.quiz.ID, fixture.student.ID, fixture.answersFor(t, true), fixture.entry())
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("el cuarto intento devolvió %v, se esperaba ErrConflict", err)
	}

	// Y no dejó rastro: siguen siendo tres.
	envios, err := fixture.repo.ListSubmissions(ctx, fixture.student.ID, fixture.quiz.ID)
	if err != nil {
		t.Fatalf("ListSubmissions returned an error: %v", err)
	}
	if len(envios) != 3 {
		t.Errorf("hay %d envíos registrados, se esperaban 3", len(envios))
	}
}

// Primer criterio de aceptacion, en la capa donde se garantiza: varios envios
// simultaneos no pueden producir mas intentos de los permitidos.
//
// El middleware de idempotencia cubre el caso de la misma Idempotency-Key. Esto
// cubre el que queda debajo: peticiones distintas que llegan a la vez y cuyo
// conteo de intentos se solapa.
func TestQuizRepositoryConcurrentSubmissionsRespectTheLimit(t *testing.T) {
	db := newTestDB(t)
	db.SetMaxOpenConns(4)
	fixture := newQuizFixture(t, db, 2)

	const simultaneos = 8
	respuestas := fixture.answersFor(t, true)

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		exitosos int
		otros    []error
	)
	for i := 0; i < simultaneos; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := fixture.repo.Grade(context.Background(), fixture.quiz.ID, fixture.student.ID, respuestas, fixture.entry())
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				exitosos++
			case errors.Is(err, domain.ErrConflict):
				// Esperado: o agotó intentos, o perdió la carrera por el número.
			default:
				otros = append(otros, err)
			}
		}()
	}
	wg.Wait()

	for _, err := range otros {
		t.Errorf("un envío concurrente falló con un error inesperado: %v", err)
	}
	if exitosos > 3 {
		t.Errorf("%d envíos tuvieron éxito con un límite de 3 intentos", exitosos)
	}

	envios, err := fixture.repo.ListSubmissions(context.Background(), fixture.student.ID, fixture.quiz.ID)
	if err != nil {
		t.Fatalf("ListSubmissions returned an error: %v", err)
	}
	if len(envios) > 3 {
		t.Errorf("quedaron %d envíos registrados con un límite de 3", len(envios))
	}
	// Y los números de intento son únicos y consecutivos desde 1.
	vistos := map[int]bool{}
	for _, envio := range envios {
		if vistos[envio.Attempt] {
			t.Errorf("el intento %d aparece dos veces", envio.Attempt)
		}
		vistos[envio.Attempt] = true
	}
}

// Un quiz sin preguntas no se aprueba por omisión: calificar con 100 lo
// convertiría en un aprobado automático.
func TestQuizRepositoryEmptyQuizDoesNotPass(t *testing.T) {
	db := newTestDB(t)
	fixture := newQuizFixture(t, db, 0)

	grade, err := fixture.repo.Grade(context.Background(), fixture.quiz.ID, fixture.student.ID,
		[]domain.Answer{{QuestionID: "x", SelectedOptionID: "y"}}, fixture.entry())
	if err != nil {
		t.Fatalf("Grade returned an error: %v", err)
	}
	if grade.Submission.Score != 0 || grade.Submission.Passed {
		t.Errorf("un quiz sin preguntas dio %d y passed=%v", grade.Submission.Score, grade.Submission.Passed)
	}
}

func TestQuizRepositoryRefusesASecondQuizOnTheSameResource(t *testing.T) {
	db := newTestDB(t)
	fixture := newQuizFixture(t, db, 1)

	_, err := fixture.repo.Create(context.Background(), &domain.Quiz{
		ResourceID: fixture.quiz.ResourceID, Title: "Otro", PassingScore: 50, MaxAttempts: 1,
	}, quizEntry(fixture.author.ID, "resource:"+fixture.quiz.ResourceID))

	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("crear un segundo quiz devolvió %v, se esperaba ErrConflict", err)
	}
}

// El orden de las opciones lo fija el autor y no cambia entre lecturas. Antes
// no era asi: created_at es la hora de inicio de la transaccion, todas las
// opciones de una pregunta empataban y el desempate caia en un UUID aleatorio.
func TestQuizRepositoryOptionOrderIsStable(t *testing.T) {
	db := newTestDB(t)
	fixture := newQuizFixture(t, db, 1)
	ctx := context.Background()

	var primera []string
	for lectura := 0; lectura < 5; lectura++ {
		quiz, err := fixture.repo.GetForStudent(ctx, fixture.quiz.ID)
		if err != nil {
			t.Fatalf("GetForStudent returned an error: %v", err)
		}
		orden := make([]string, 0, len(quiz.Questions[0].Options))
		for i, option := range quiz.Questions[0].Options {
			if option.Position != i {
				t.Errorf("la opción %d dice tener posición %d", i, option.Position)
			}
			orden = append(orden, option.Text)
		}
		if lectura == 0 {
			primera = orden
			continue
		}
		for i := range orden {
			if orden[i] != primera[i] {
				t.Fatalf("la lectura %d devolvió las opciones en otro orden: %v frente a %v", lectura, orden, primera)
			}
		}
	}
}
