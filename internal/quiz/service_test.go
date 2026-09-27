package quiz_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/quiz"
)

const (
	studentID  = "11111111-1111-1111-1111-111111111111"
	authorID   = "22222222-2222-2222-2222-222222222222"
	resourceID = "33333333-3333-3333-3333-333333333333"
	quizID     = "44444444-4444-4444-4444-444444444444"
	courseID   = "55555555-5555-5555-5555-555555555555"
	stableID   = "66666666-6666-6666-6666-666666666666"
)

// ---- dobles -------------------------------------------------------------

type fakeQuizRepo struct {
	quiz *domain.Quiz

	graded    int
	createdQ  *domain.Question
	createErr error
	gradeErr  error
	grade     *domain.QuizGrade

	// forAuthorCalls y forStudentCalls distinguen cual de las dos lecturas se
	// uso, que es lo que separa ver la clave de no verla.
	forAuthorCalls  int
	forStudentCalls int
}

func (f *fakeQuizRepo) Create(_ context.Context, q *domain.Quiz, _ *domain.AuditEntry) (*domain.Quiz, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	return q, nil
}

func (f *fakeQuizRepo) AddQuestion(_ context.Context, _ string, q *domain.Question, _ *domain.AuditEntry) (*domain.Question, error) {
	f.createdQ = q
	return q, nil
}

func (f *fakeQuizRepo) GetForAuthor(context.Context, string) (*domain.Quiz, error) {
	f.forAuthorCalls++
	return f.quiz, nil
}

func (f *fakeQuizRepo) GetForStudent(context.Context, string) (*domain.Quiz, error) {
	f.forStudentCalls++
	return f.quiz, nil
}

func (f *fakeQuizRepo) GetByResourceID(context.Context, string) (*domain.Quiz, error) {
	return f.quiz, nil
}

func (f *fakeQuizRepo) Grade(context.Context, string, string, []domain.Answer, *domain.AuditEntry) (*domain.QuizGrade, error) {
	if f.gradeErr != nil {
		return nil, f.gradeErr
	}
	f.graded++
	if f.grade != nil {
		return f.grade, nil
	}
	return &domain.QuizGrade{
		Submission:        &domain.QuizSubmission{ID: "sub-1", QuizID: quizID, Attempt: 1, Score: 100, Passed: true},
		AttemptsUsed:      1,
		AttemptsRemaining: 2,
	}, nil
}

func (f *fakeQuizRepo) ListSubmissions(context.Context, string, string) ([]*domain.QuizSubmission, error) {
	return nil, nil
}

type fakeLookups struct{}

func (f *fakeLookups) resource() *domain.Resource {
	return &domain.Resource{ID: resourceID, UnitID: "unit", Type: domain.ResourceTypeQuiz}
}

type resourceLookup struct{ res *domain.Resource }

func (r *resourceLookup) GetByID(context.Context, string) (*domain.Resource, error) {
	return r.res, nil
}

type unitLookup struct{}

func (unitLookup) GetByID(context.Context, string) (*domain.Unit, error) {
	return &domain.Unit{ID: "unit", ModuleID: "module"}, nil
}

type moduleLookup struct{}

func (moduleLookup) GetByID(context.Context, string) (*domain.Module, error) {
	return &domain.Module{ID: "module", CourseID: courseID}, nil
}

type courseLookup struct{ course *domain.Course }

func (c *courseLookup) GetByID(context.Context, string) (*domain.Course, error) { return c.course, nil }

type fakeAccess struct {
	active bool
	asked  int
}

func (f *fakeAccess) IsActiveIn(context.Context, string, string) (bool, error) {
	f.asked++
	return f.active, nil
}

type fakeProgress struct {
	marked int
	err    error
}

func (f *fakeProgress) MarkResourceCompleted(context.Context, string, string, *domain.AuditEntry) error {
	f.marked++
	return f.err
}

// ---- armado -------------------------------------------------------------

func publishedCourse() *domain.Course {
	return &domain.Course{ID: courseID, StableID: stableID, AuthorID: authorID, Status: domain.CourseStatusPublished}
}

func newService(t *testing.T, repo *fakeQuizRepo, course *domain.Course, access quiz.ContentAccess, progress quiz.ProgressRecorder) *quiz.Service {
	t.Helper()
	res := (&fakeLookups{}).resource()
	return quiz.NewService(quiz.Deps{
		Quizzes:   repo,
		Resources: &resourceLookup{res: res},
		Units:     unitLookup{},
		Modules:   moduleLookup{},
		Courses:   &courseLookup{course: course},
		Access:    access,
		Progress:  progress,
	}, nil)
}

func student() quiz.Actor { return quiz.Actor{ID: studentID, Role: domain.RoleStudent} }
func author() quiz.Actor  { return quiz.Actor{ID: authorID, Role: domain.RoleProfessor} }

func repoWithQuiz() *fakeQuizRepo {
	return &fakeQuizRepo{quiz: &domain.Quiz{
		ID: quizID, ResourceID: resourceID, Title: "Evaluación", PassingScore: 70, MaxAttempts: 3,
	}}
}

func oneAnswer() []domain.Answer {
	return []domain.Answer{{QuestionID: "q1", SelectedOptionID: "o1"}}
}

// ---- quien ve la clave --------------------------------------------------

// La diferencia entre el autor y el estudiante no es un filtro sobre la misma
// lectura: son dos lecturas distintas, y solo una carga is_correct.
func TestGetUsesTheAuthorReadOnlyForTheAuthor(t *testing.T) {
	repo := repoWithQuiz()
	service := newService(t, repo, publishedCourse(), &fakeAccess{active: true}, nil)

	if _, asAuthor, err := service.Get(context.Background(), author(), resourceID); err != nil || !asAuthor {
		t.Fatalf("el autor obtuvo asAuthor=%v, err=%v", asAuthor, err)
	}
	if repo.forAuthorCalls != 1 || repo.forStudentCalls != 0 {
		t.Errorf("el autor provocó %d lecturas con clave y %d sin clave", repo.forAuthorCalls, repo.forStudentCalls)
	}

	repo = repoWithQuiz()
	service = newService(t, repo, publishedCourse(), &fakeAccess{active: true}, nil)
	if _, asAuthor, err := service.Get(context.Background(), student(), resourceID); err != nil || asAuthor {
		t.Fatalf("el estudiante obtuvo asAuthor=%v, err=%v", asAuthor, err)
	}
	if repo.forStudentCalls != 1 || repo.forAuthorCalls != 0 {
		t.Errorf("el estudiante provocó %d lecturas sin clave y %d con clave", repo.forStudentCalls, repo.forAuthorCalls)
	}
}

func TestGetRequiresAnActiveEnrollment(t *testing.T) {
	repo := repoWithQuiz()
	service := newService(t, repo, publishedCourse(), &fakeAccess{active: false}, nil)

	_, _, err := service.Get(context.Background(), student(), resourceID)

	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("Get error = %v, se esperaba ErrForbidden", err)
	}
	if repo.forStudentCalls != 0 {
		t.Error("se leyó el cuestionario para un estudiante sin inscripción")
	}
}

// ---- presentar ----------------------------------------------------------

func TestSubmitRequiresAnActiveEnrollment(t *testing.T) {
	repo := repoWithQuiz()
	access := &fakeAccess{active: false}
	service := newService(t, repo, publishedCourse(), access, nil)

	_, err := service.Submit(context.Background(), student(), quizID, oneAnswer())

	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("Submit error = %v, se esperaba ErrForbidden", err)
	}
	if repo.graded != 0 {
		t.Error("se calificó un envío de alguien sin inscripción activa")
	}
}

// El autor no se evalúa a sí mismo: puede leer su curso sin inscribirse, pero
// una calificación suya entraría en las estadísticas de su propio curso.
func TestSubmitRefusesTheAuthor(t *testing.T) {
	repo := repoWithQuiz()
	service := newService(t, repo, publishedCourse(), &fakeAccess{active: false}, nil)

	_, err := service.Submit(context.Background(), author(), quizID, oneAnswer())

	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("Submit error = %v, se esperaba ErrForbidden", err)
	}
	if repo.graded != 0 {
		t.Error("se calificó un envío del autor")
	}
}

func TestSubmitRefusesAnUnpublishedCourse(t *testing.T) {
	for _, status := range []domain.CourseStatus{domain.CourseStatusDraft, domain.CourseStatusUnpublished} {
		t.Run(string(status), func(t *testing.T) {
			course := publishedCourse()
			course.Status = status
			repo := repoWithQuiz()
			access := &fakeAccess{active: true}
			service := newService(t, repo, course, access, nil)

			_, err := service.Submit(context.Background(), student(), quizID, oneAnswer())

			if !errors.Is(err, domain.ErrConflict) {
				t.Fatalf("Submit error = %v, se esperaba ErrConflict", err)
			}
			if access.asked != 0 {
				t.Error("se consultó la inscripción de un curso que no está publicado")
			}
		})
	}
}

// Un servicio sin comprobación de acceso rechaza todo, en lugar de repartir
// cuestionarios.
func TestSubmitFailsClosedWithoutAccessChecker(t *testing.T) {
	repo := repoWithQuiz()
	service := quiz.NewService(quiz.Deps{
		Quizzes:   repo,
		Resources: &resourceLookup{res: (&fakeLookups{}).resource()},
		Units:     unitLookup{},
		Modules:   moduleLookup{},
		Courses:   &courseLookup{course: publishedCourse()},
	}, nil)

	_, err := service.Submit(context.Background(), student(), quizID, oneAnswer())

	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("Submit error = %v, se esperaba ErrForbidden", err)
	}
	if repo.graded != 0 {
		t.Error("se calificó sin comprobación de acceso cableada")
	}
}

func TestSubmitRejectsMalformedAnswers(t *testing.T) {
	casos := map[string][]domain.Answer{
		"sin respuestas":    {},
		"sin pregunta":      {{SelectedOptionID: "o1"}},
		"sin opción":        {{QuestionID: "q1"}},
		"pregunta repetida": {{QuestionID: "q1", SelectedOptionID: "o1"}, {QuestionID: "q1", SelectedOptionID: "o2"}},
	}
	for nombre, answers := range casos {
		t.Run(nombre, func(t *testing.T) {
			repo := repoWithQuiz()
			service := newService(t, repo, publishedCourse(), &fakeAccess{active: true}, nil)

			_, err := service.Submit(context.Background(), student(), quizID, answers)

			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("Submit error = %v, se esperaba ErrInvalidInput", err)
			}
			if repo.graded != 0 {
				t.Error("se llegó a calificar un envío mal formado")
			}
		})
	}
}

// ---- aprobar completa el recurso ---------------------------------------

func TestSubmitMarksTheResourceCompletedOnPass(t *testing.T) {
	repo := repoWithQuiz()
	progress := &fakeProgress{}
	service := newService(t, repo, publishedCourse(), &fakeAccess{active: true}, progress)

	if _, err := service.Submit(context.Background(), student(), quizID, oneAnswer()); err != nil {
		t.Fatalf("Submit returned an error: %v", err)
	}
	if progress.marked != 1 {
		t.Errorf("se registró el recurso como completado %d veces, se esperaba 1", progress.marked)
	}
}

func TestSubmitDoesNotMarkTheResourceOnFail(t *testing.T) {
	repo := repoWithQuiz()
	repo.grade = &domain.QuizGrade{
		Submission:        &domain.QuizSubmission{ID: "sub-2", Attempt: 1, Score: 20, Passed: false},
		AttemptsRemaining: 2,
	}
	progress := &fakeProgress{}
	service := newService(t, repo, publishedCourse(), &fakeAccess{active: true}, progress)

	if _, err := service.Submit(context.Background(), student(), quizID, oneAnswer()); err != nil {
		t.Fatalf("Submit returned an error: %v", err)
	}
	if progress.marked != 0 {
		t.Error("un intento reprobado marcó el recurso como completado")
	}
}

// Si el progreso falla despues de calificar, la calificación se conserva. Perder
// el progreso se arregla con un latido; perder un intento no se arregla.
func TestSubmitKeepsTheGradeWhenProgressFails(t *testing.T) {
	repo := repoWithQuiz()
	progress := &fakeProgress{err: errors.New("la base no responde")}
	service := newService(t, repo, publishedCourse(), &fakeAccess{active: true}, progress)

	grade, err := service.Submit(context.Background(), student(), quizID, oneAnswer())

	if err != nil {
		t.Fatalf("Submit devolvió error pese a que la calificación se registró: %v", err)
	}
	if grade == nil || !grade.Submission.Passed {
		t.Error("se perdió la calificación por un fallo posterior al calificar")
	}
}

// ---- autoría ------------------------------------------------------------

func TestAddQuestionDemandsExactlyOneCorrectOption(t *testing.T) {
	casos := map[string][]domain.QuestionOption{
		"ninguna correcta": {{Text: "a"}, {Text: "b"}},
		"dos correctas":    {{Text: "a", IsCorrect: true}, {Text: "b", IsCorrect: true}},
		"una sola opción":  {{Text: "a", IsCorrect: true}},
		"opción vacía":     {{Text: "a", IsCorrect: true}, {Text: "  "}},
	}
	for nombre, options := range casos {
		t.Run(nombre, func(t *testing.T) {
			repo := repoWithQuiz()
			service := newService(t, repo, publishedCourse(), &fakeAccess{active: true}, nil)

			_, err := service.AddQuestion(context.Background(), author(), quizID, "¿Pregunta?", options)

			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("AddQuestion error = %v, se esperaba ErrInvalidInput", err)
			}
			if repo.createdQ != nil {
				t.Error("se creó una pregunta que no evalúa nada")
			}
		})
	}
}

func TestAuthoringRefusesANonAuthor(t *testing.T) {
	repo := repoWithQuiz()
	service := newService(t, repo, publishedCourse(), &fakeAccess{active: true}, nil)

	_, err := service.AddQuestion(context.Background(), student(), quizID, "¿Pregunta?",
		[]domain.QuestionOption{{Text: "a", IsCorrect: true}, {Text: "b"}})

	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("AddQuestion error = %v, se esperaba ErrForbidden", err)
	}
}

// Un cuestionario solo cuelga de un recurso de tipo quiz: en un vídeo, ningún
// cliente sabría cómo presentarlo.
func TestCreateQuizRefusesANonQuizResource(t *testing.T) {
	repo := repoWithQuiz()
	res := (&fakeLookups{}).resource()
	res.Type = domain.ResourceTypeVideo
	service := quiz.NewService(quiz.Deps{
		Quizzes:   repo,
		Resources: &resourceLookup{res: res},
		Units:     unitLookup{},
		Modules:   moduleLookup{},
		Courses:   &courseLookup{course: publishedCourse()},
		Access:    &fakeAccess{active: true},
	}, nil)

	_, err := service.CreateQuiz(context.Background(), author(), resourceID, "Evaluación", 70, 3)

	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("CreateQuiz error = %v, se esperaba ErrInvalidInput", err)
	}
}
