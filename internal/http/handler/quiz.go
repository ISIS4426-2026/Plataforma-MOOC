package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/quiz"
)

// QuizHandler expone la autoria de cuestionarios y su presentacion (issue #112).
//
// Las respuestas se construyen aqui con tipos explicitos en lugar de serializar
// el dominio, y eso importa para una cosa concreta: la respuesta que ve un
// estudiante no tiene ningun campo donde pudiera caber la clave correcta. No
// depende de recordar una etiqueta `json:"-"`.
type QuizHandler struct {
	service *quiz.Service
	logger  *slog.Logger
}

func NewQuizHandler(service *quiz.Service, logger *slog.Logger) *QuizHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &QuizHandler{service: service, logger: logger}
}

// ---- cuerpos de peticion ----------------------------------------------

type createQuizRequest struct {
	Title        string `json:"title"`
	PassingScore int    `json:"passing_score"`
	MaxAttempts  int    `json:"max_attempts"`
}

type addQuestionRequest struct {
	Prompt  string `json:"prompt"`
	Options []struct {
		Text      string `json:"text"`
		IsCorrect bool   `json:"is_correct"`
	} `json:"options"`
}

type submitQuizRequest struct {
	Answers []struct {
		QuestionID       string `json:"question_id"`
		SelectedOptionID string `json:"selected_option_id"`
	} `json:"answers"`
}

// ---- cuerpos de respuesta ---------------------------------------------

// studentOption es lo que ve quien va a contestar: el texto y nada mas.
type studentOption struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Position int    `json:"position"`
}

// authorOption añade cual es la correcta. Es un tipo distinto, no un campo
// opcional, para que la vista del estudiante no tenga donde alojarla.
type authorOption struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	Position  int    `json:"position"`
	IsCorrect bool   `json:"is_correct"`
}

type studentQuestion struct {
	ID       string          `json:"id"`
	Prompt   string          `json:"prompt"`
	Position int             `json:"position"`
	Options  []studentOption `json:"options"`
}

type authorQuestion struct {
	ID       string         `json:"id"`
	Prompt   string         `json:"prompt"`
	Position int            `json:"position"`
	Options  []authorOption `json:"options"`
}

type quizResponse struct {
	ID           string `json:"id"`
	ResourceID   string `json:"resource_id"`
	Title        string `json:"title"`
	PassingScore int    `json:"passing_score"`
	MaxAttempts  int    `json:"max_attempts"`

	// Solo uno de los dos se rellena, segun quien pregunte.
	Questions       []studentQuestion `json:"questions,omitempty"`
	QuestionsAuthor []authorQuestion  `json:"questions_with_key,omitempty"`
}

type submissionResponse struct {
	SubmissionID      string `json:"submission_id"`
	Score             int    `json:"score"`
	Passed            bool   `json:"passed"`
	Attempt           int    `json:"attempt"`
	AttemptsRemaining int    `json:"attempts_remaining"`
	SubmittedAt       string `json:"submitted_at"`
}

// ---- autoria -----------------------------------------------------------

// CreateQuiz cuelga un cuestionario de un recurso de tipo quiz.
func (h *QuizHandler) CreateQuiz(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	var body createQuizRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		RespondWithError(w, http.StatusBadRequest, "invalid_input", "El cuerpo de la solicitud no es un JSON válido.", nil)
		return
	}
	// Valores por defecto sensatos: el esquema ya define 70 y 3, y repetirlos
	// aqui evita que omitir el campo se interprete como cero.
	if body.PassingScore == 0 {
		body.PassingScore = 70
	}
	if body.MaxAttempts == 0 {
		body.MaxAttempts = 3
	}

	created, err := h.service.CreateQuiz(r.Context(), actor, r.PathValue("resourceID"), body.Title, body.PassingScore, body.MaxAttempts)
	if err != nil {
		h.respondQuizError(w, r, err)
		return
	}
	RespondWithJSON(w, http.StatusCreated, newAuthorQuiz(created))
}

// AddQuestion agrega una pregunta con sus opciones.
func (h *QuizHandler) AddQuestion(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	var body addQuestionRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		RespondWithError(w, http.StatusBadRequest, "invalid_input", "El cuerpo de la solicitud no es un JSON válido.", nil)
		return
	}

	options := make([]domain.QuestionOption, 0, len(body.Options))
	for _, option := range body.Options {
		options = append(options, domain.QuestionOption{Text: option.Text, IsCorrect: option.IsCorrect})
	}

	created, err := h.service.AddQuestion(r.Context(), actor, r.PathValue("quizID"), body.Prompt, options)
	if err != nil {
		h.respondQuizError(w, r, err)
		return
	}
	RespondWithJSON(w, http.StatusCreated, newAuthorQuestion(created))
}

// ---- lectura -----------------------------------------------------------

// Get devuelve el cuestionario de un recurso, con o sin clave segun quien pida.
func (h *QuizHandler) Get(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	found, asAuthor, err := h.service.Get(r.Context(), actor, r.PathValue("resourceID"))
	if err != nil {
		h.respondQuizError(w, r, err)
		return
	}
	if asAuthor {
		RespondWithJSON(w, http.StatusOK, newAuthorQuiz(found))
		return
	}
	RespondWithJSON(w, http.StatusOK, newStudentQuiz(found))
}

// Submissions lista los intentos del propio estudiante.
func (h *QuizHandler) Submissions(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	list, err := h.service.Submissions(r.Context(), actor, r.PathValue("quizID"))
	if err != nil {
		h.respondQuizError(w, r, err)
		return
	}
	items := make([]submissionResponse, 0, len(list))
	for _, s := range list {
		items = append(items, submissionResponse{
			SubmissionID: s.ID, Score: s.Score, Passed: s.Passed, Attempt: s.Attempt,
			SubmittedAt: s.SubmittedAt.UTC().Format(time.RFC3339),
		})
	}
	RespondWithJSON(w, http.StatusOK, map[string]any{"items": items})
}

// ---- presentacion ------------------------------------------------------

// Submit califica un intento.
func (h *QuizHandler) Submit(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	var body submitQuizRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		RespondWithError(w, http.StatusBadRequest, "invalid_input", "El cuerpo de la solicitud no es un JSON válido.", nil)
		return
	}

	answers := make([]domain.Answer, 0, len(body.Answers))
	for _, answer := range body.Answers {
		answers = append(answers, domain.Answer{QuestionID: answer.QuestionID, SelectedOptionID: answer.SelectedOptionID})
	}

	grade, err := h.service.Submit(r.Context(), actor, r.PathValue("quizID"), answers)
	if err != nil {
		h.respondQuizError(w, r, err)
		return
	}
	RespondWithJSON(w, http.StatusOK, submissionResponse{
		SubmissionID:      grade.Submission.ID,
		Score:             grade.Submission.Score,
		Passed:            grade.Submission.Passed,
		Attempt:           grade.Submission.Attempt,
		AttemptsRemaining: grade.AttemptsRemaining,
		SubmittedAt:       grade.Submission.SubmittedAt.UTC().Format(time.RFC3339),
	})
}

// ---- conversiones ------------------------------------------------------

func newStudentQuiz(q *domain.Quiz) quizResponse {
	out := quizResponse{
		ID: q.ID, ResourceID: q.ResourceID, Title: q.Title,
		PassingScore: q.PassingScore, MaxAttempts: q.MaxAttempts,
		Questions: make([]studentQuestion, 0, len(q.Questions)),
	}
	for _, question := range q.Questions {
		options := make([]studentOption, 0, len(question.Options))
		for _, option := range question.Options {
			options = append(options, studentOption{ID: option.ID, Text: option.Text, Position: option.Position})
		}
		out.Questions = append(out.Questions, studentQuestion{
			ID: question.ID, Prompt: question.Prompt, Position: question.Position, Options: options,
		})
	}
	return out
}

func newAuthorQuiz(q *domain.Quiz) quizResponse {
	out := quizResponse{
		ID: q.ID, ResourceID: q.ResourceID, Title: q.Title,
		PassingScore: q.PassingScore, MaxAttempts: q.MaxAttempts,
	}
	if len(q.Questions) == 0 {
		return out
	}
	out.QuestionsAuthor = make([]authorQuestion, 0, len(q.Questions))
	for _, question := range q.Questions {
		out.QuestionsAuthor = append(out.QuestionsAuthor, newAuthorQuestion(&question))
	}
	return out
}

func newAuthorQuestion(q *domain.Question) authorQuestion {
	options := make([]authorOption, 0, len(q.Options))
	for _, option := range q.Options {
		options = append(options, authorOption{ID: option.ID, Text: option.Text, Position: option.Position, IsCorrect: option.IsCorrect})
	}
	return authorQuestion{ID: q.ID, Prompt: q.Prompt, Position: q.Position, Options: options}
}

// ---- compartido --------------------------------------------------------

func (h *QuizHandler) actor(w http.ResponseWriter, r *http.Request) (quiz.Actor, bool) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		RespondWithError(w, http.StatusUnauthorized, "unauthorized", "Se requiere autenticación.", nil)
		return quiz.Actor{}, false
	}
	return quiz.Actor{ID: user.ID, Role: user.Role, IPAddress: clientIP(r), UserAgent: r.UserAgent()}, true
}

func (h *QuizHandler) respondQuizError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrCourseImmutable):
		RespondWithError(w, http.StatusConflict, "course_immutable",
			"No es posible modificar la evaluación de un curso publicado.", nil)
	case errors.Is(err, domain.ErrConflict):
		RespondWithError(w, http.StatusConflict, "quiz_conflict",
			"No quedan intentos disponibles, o el recurso ya tiene un cuestionario.", nil)
	case errors.Is(err, domain.ErrNotFound):
		RespondWithError(w, http.StatusNotFound, "not_found", "El recurso solicitado no existe.", nil)
	case errors.Is(err, domain.ErrForbidden):
		RespondWithError(w, http.StatusForbidden, "forbidden",
			"No tienes permiso para realizar esta operación.", nil)
	case errors.Is(err, domain.ErrInvalidInput):
		RespondWithError(w, http.StatusBadRequest, "invalid_input", "Los datos enviados no son válidos.", nil)
	default:
		h.logger.ErrorContext(r.Context(), "unhandled error in quiz handler",
			slog.String("request_id", w.Header().Get("X-Request-ID")),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("error", err.Error()),
		)
		RespondWithError(w, http.StatusInternalServerError, "internal_error", "Ocurrió un error inesperado.", nil)
	}
}
