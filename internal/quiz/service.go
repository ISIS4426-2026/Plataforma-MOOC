// Package quiz implementa la evaluacion de un curso: autoria de cuestionarios,
// presentacion y calificacion.
//
// El enunciado fija tres condiciones de aceptacion para este flujo, y las tres
// se sostienen mas abajo que este paquete: «la clave correcta nunca llega al
// cliente, el envio definitivo es idempotente y la calificacion se calcula en
// el servidor». Aqui viven las reglas de quien puede hacer que; las garantias
// las imponen el repositorio y la base.
package quiz

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
)

// ResourceLookup es la porcion del repositorio de recursos que este servicio
// lee: necesita saber a que curso pertenece un recurso y de que tipo es.
type ResourceLookup interface {
	GetByID(ctx context.Context, id string) (*domain.Resource, error)
}

// UnitLookup y ModuleLookup permiten subir del recurso al curso. Se declaran
// aqui, estrechas, en lugar de depender de los repositorios completos.
type UnitLookup interface {
	GetByID(ctx context.Context, id string) (*domain.Unit, error)
}

type ModuleLookup interface {
	GetByID(ctx context.Context, id string) (*domain.Module, error)
}

type CourseLookup interface {
	GetByID(ctx context.Context, id string) (*domain.Course, error)
}

// ContentAccess decide si alguien puede ver el contenido de un curso. La
// implementa enrollment.Service, igual que para el paquete structure: la regla
// pertenece a inscripciones, su aplicacion pertenece aqui.
type ContentAccess interface {
	IsActiveIn(ctx context.Context, studentID, courseStableID string) (bool, error)
}

// ProgressRecorder marca un recurso como completado.
//
// Se declara con esta forma, y no con la del repositorio de progreso, para que
// este paquete no tenga que saber nada de latidos ni de tiempos de permanencia:
// aprobar un quiz completa su recurso, y eso es todo lo que necesita expresar.
type ProgressRecorder interface {
	MarkResourceCompleted(ctx context.Context, studentID, resourceID string, entry *domain.AuditEntry) error
}

type Deps struct {
	Quizzes   domain.QuizRepository
	Resources ResourceLookup
	Units     UnitLookup
	Modules   ModuleLookup
	Courses   CourseLookup

	// Access cierra el lado de lectura y el de presentacion. Un valor nulo
	// rechaza todo en lugar de permitirlo: un servicio mal cableado debe fallar
	// de forma visible, no repartir cuestionarios.
	Access ContentAccess

	// Progress es opcional. Si esta, aprobar un quiz completa su recurso.
	Progress ProgressRecorder
}

type Actor struct {
	ID        string
	Role      domain.Role
	IPAddress string
	UserAgent string
}

type Service struct {
	deps   Deps
	logger *slog.Logger
}

func NewService(deps Deps, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{deps: deps, logger: logger}
}

// ---- autoria -----------------------------------------------------------

// CreateQuiz cuelga un cuestionario de un recurso de tipo quiz.
func (s *Service) CreateQuiz(ctx context.Context, actor Actor, resourceID, title string, passingScore, maxAttempts int) (*domain.Quiz, error) {
	if strings.TrimSpace(title) == "" {
		return nil, fmt.Errorf("a quiz needs a title: %w", domain.ErrInvalidInput)
	}
	if passingScore < 0 || passingScore > 100 {
		return nil, fmt.Errorf("passing score must be between 0 and 100: %w", domain.ErrInvalidInput)
	}
	if maxAttempts < 1 {
		return nil, fmt.Errorf("a quiz must allow at least one attempt: %w", domain.ErrInvalidInput)
	}

	resource, course, err := s.locate(ctx, resourceID)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeAuthor(actor, course); err != nil {
		return nil, err
	}

	// El recurso tiene que ser de tipo quiz. Colgar un cuestionario de un video
	// dejaria contenido que ningun cliente sabria como presentar, y el tipo del
	// recurso es justamente lo que se lo dice.
	if resource.Type != domain.ResourceTypeQuiz {
		return nil, fmt.Errorf("resource %s is of type %s, not quiz: %w", resourceID, resource.Type, domain.ErrInvalidInput)
	}

	return s.deps.Quizzes.Create(ctx, &domain.Quiz{
		ResourceID: resourceID, Title: title, PassingScore: passingScore, MaxAttempts: maxAttempts,
	}, newEntry(actor, "resource:"+resourceID))
}

// AddQuestion agrega una pregunta con sus opciones.
func (s *Service) AddQuestion(ctx context.Context, actor Actor, quizID, prompt string, options []domain.QuestionOption) (*domain.Question, error) {
	if strings.TrimSpace(prompt) == "" {
		return nil, fmt.Errorf("a question needs a prompt: %w", domain.ErrInvalidInput)
	}
	// Dos opciones es el minimo para que elegir signifique algo, y al menos una
	// correcta es lo que hace la pregunta calificable. Sin esas dos condiciones
	// la pregunta existiria pero no evaluaria nada.
	if len(options) < 2 {
		return nil, fmt.Errorf("a question needs at least two options: %w", domain.ErrInvalidInput)
	}
	correct := 0
	for _, option := range options {
		if strings.TrimSpace(option.Text) == "" {
			return nil, fmt.Errorf("an option needs text: %w", domain.ErrInvalidInput)
		}
		if option.IsCorrect {
			correct++
		}
	}
	if correct != 1 {
		return nil, fmt.Errorf("a question needs exactly one correct option, got %d: %w", correct, domain.ErrInvalidInput)
	}

	course, err := s.courseOfQuiz(ctx, quizID)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeAuthor(actor, course); err != nil {
		return nil, err
	}

	return s.deps.Quizzes.AddQuestion(ctx, quizID, &domain.Question{Prompt: prompt, Options: options},
		newEntry(actor, "quiz:"+quizID))
}

// ---- lectura -----------------------------------------------------------

// Get devuelve el cuestionario de un recurso.
//
// La vista depende de quien pregunta, y la diferencia no es cosmetica: al autor
// se le carga la clave de respuestas desde la base y al estudiante no se le
// carga. Son dos consultas distintas, no un filtrado posterior.
func (s *Service) Get(ctx context.Context, actor Actor, resourceID string) (*domain.Quiz, bool, error) {
	_, course, err := s.locate(ctx, resourceID)
	if err != nil {
		return nil, false, err
	}

	quiz, err := s.deps.Quizzes.GetByResourceID(ctx, resourceID)
	if err != nil {
		return nil, false, err
	}

	if s.isAuthor(actor, course) {
		full, err := s.deps.Quizzes.GetForAuthor(ctx, quiz.ID)
		return full, true, err
	}

	// Un estudiante necesita inscripcion activa, igual que para cualquier otro
	// contenido del curso.
	if err := s.authorizeStudent(ctx, actor, course); err != nil {
		return nil, false, err
	}
	student, err := s.deps.Quizzes.GetForStudent(ctx, quiz.ID)
	return student, false, err
}

// Submissions lista los intentos del propio estudiante.
func (s *Service) Submissions(ctx context.Context, actor Actor, quizID string) ([]*domain.QuizSubmission, error) {
	if _, err := s.courseOfQuiz(ctx, quizID); err != nil {
		return nil, err
	}
	return s.deps.Quizzes.ListSubmissions(ctx, actor.ID, quizID)
}

// ---- presentacion ------------------------------------------------------

// Submit califica un intento.
//
// La calificacion la hace el repositorio dentro de su transaccion, leyendo la
// clave correcta de la base. Este metodo nunca la ve, y por eso no puede
// filtrarla ni equivocarse al aplicarla.
func (s *Service) Submit(ctx context.Context, actor Actor, quizID string, answers []domain.Answer) (*domain.QuizGrade, error) {
	if len(answers) == 0 {
		return nil, fmt.Errorf("a submission needs at least one answer: %w", domain.ErrInvalidInput)
	}
	// Una pregunta respondida dos veces es un cliente roto, y aceptarlo
	// significaria calificar una de las dos en silencio.
	seen := make(map[string]bool, len(answers))
	for _, answer := range answers {
		if answer.QuestionID == "" || answer.SelectedOptionID == "" {
			return nil, fmt.Errorf("an answer needs both a question and an option: %w", domain.ErrInvalidInput)
		}
		if seen[answer.QuestionID] {
			return nil, fmt.Errorf("question %s answered twice: %w", answer.QuestionID, domain.ErrInvalidInput)
		}
		seen[answer.QuestionID] = true
	}

	quiz, course, err := s.quizAndCourse(ctx, quizID)
	if err != nil {
		return nil, err
	}

	// Presentar exige inscripcion activa. El autor queda fuera a proposito: no
	// se evalua a si mismo, y darle una calificacion lo pondria en las
	// estadisticas de su propio curso.
	if err := s.authorizeStudent(ctx, actor, course); err != nil {
		return nil, err
	}

	grade, err := s.deps.Quizzes.Grade(ctx, quizID, actor.ID, answers, newEntry(actor, "quiz:"+quizID))
	if err != nil {
		return nil, err
	}

	s.logger.InfoContext(ctx, "quiz submitted",
		slog.String("student_id", actor.ID),
		slog.String("quiz_id", quizID),
		slog.Int("attempt", grade.Submission.Attempt),
		slog.Int("score", grade.Submission.Score),
		slog.Bool("passed", grade.Submission.Passed),
	)

	// Aprobar completa el recurso. Que lo registre el servidor y no un latido
	// del cliente es lo coherente con el enunciado, que rechaza los avances
	// enviados por el cliente: aqui el servidor sabe de primera mano que el
	// estudiante termino el recurso, porque acaba de calificarlo.
	if grade.Submission.Passed && s.deps.Progress != nil {
		if err := s.deps.Progress.MarkResourceCompleted(ctx, actor.ID, quiz.ResourceID, newEntry(actor, "resource:"+quiz.ResourceID)); err != nil {
			// No se devuelve el error: la calificacion ya se registro y es
			// valida. Perder el progreso es recuperable con un latido; perder
			// un intento por un fallo posterior a la calificacion no lo es.
			s.logger.ErrorContext(ctx, "quiz passed but progress was not recorded",
				slog.String("student_id", actor.ID),
				slog.String("resource_id", quiz.ResourceID),
				slog.String("error", err.Error()),
			)
		}
	}

	return grade, nil
}

// ---- reglas compartidas ------------------------------------------------

func (s *Service) isAuthor(actor Actor, course *domain.Course) bool {
	return actor.Role == domain.RoleAdmin || (actor.ID != "" && course.AuthorID == actor.ID)
}

func (s *Service) authorizeAuthor(actor Actor, course *domain.Course) error {
	if !s.isAuthor(actor, course) {
		return domain.ErrForbidden
	}
	return nil
}

// authorizeStudent exige inscripcion activa sobre un curso publicado.
func (s *Service) authorizeStudent(ctx context.Context, actor Actor, course *domain.Course) error {
	if s.deps.Access == nil {
		return fmt.Errorf("quiz service has no access checker wired: %w", domain.ErrForbidden)
	}
	if course.Status != domain.CourseStatusPublished {
		return fmt.Errorf("course %s is %s, not published: %w", course.StableID, course.Status, domain.ErrConflict)
	}
	active, err := s.deps.Access.IsActiveIn(ctx, actor.ID, course.StableID)
	if err != nil {
		return err
	}
	if !active {
		return fmt.Errorf("student %s has no active enrollment in course %s: %w", actor.ID, course.StableID, domain.ErrForbidden)
	}
	return nil
}

// locate sube del recurso hasta su curso.
func (s *Service) locate(ctx context.Context, resourceID string) (*domain.Resource, *domain.Course, error) {
	resource, err := s.deps.Resources.GetByID(ctx, resourceID)
	if err != nil {
		return nil, nil, err
	}
	unit, err := s.deps.Units.GetByID(ctx, resource.UnitID)
	if err != nil {
		return nil, nil, err
	}
	module, err := s.deps.Modules.GetByID(ctx, unit.ModuleID)
	if err != nil {
		return nil, nil, err
	}
	course, err := s.deps.Courses.GetByID(ctx, module.CourseID)
	if err != nil {
		return nil, nil, err
	}
	return resource, course, nil
}

func (s *Service) quizAndCourse(ctx context.Context, quizID string) (*domain.Quiz, *domain.Course, error) {
	quiz, err := s.deps.Quizzes.GetForStudent(ctx, quizID)
	if err != nil {
		return nil, nil, err
	}
	_, course, err := s.locate(ctx, quiz.ResourceID)
	if err != nil {
		return nil, nil, err
	}
	return quiz, course, nil
}

func (s *Service) courseOfQuiz(ctx context.Context, quizID string) (*domain.Course, error) {
	_, course, err := s.quizAndCourse(ctx, quizID)
	if err != nil {
		return nil, err
	}
	return course, nil
}

func newEntry(actor Actor, target string) *domain.AuditEntry {
	actorID := actor.ID
	return &domain.AuditEntry{
		ActorID:        &actorID,
		TargetResource: target,
		IPAddress:      actor.IPAddress,
		UserAgent:      actor.UserAgent,
	}
}
