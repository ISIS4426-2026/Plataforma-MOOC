package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/enrollment"
)

// EnrollmentHandler exposes enrolling, withdrawing and the listings around them
// (issue #111).
//
// Every route acts on the caller's own enrollment, except the roster, which is
// the author's view of who is taking their course.
type EnrollmentHandler struct {
	service *enrollment.Service
	logger  *slog.Logger
}

func NewEnrollmentHandler(service *enrollment.Service, logger *slog.Logger) *EnrollmentHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &EnrollmentHandler{service: service, logger: logger}
}

type enrollmentResponse struct {
	ID             string `json:"id"`
	StudentID      string `json:"student_id"`
	CourseStableID string `json:"course_stable_id"`
	Status         string `json:"status"`
	EnrolledAt     string `json:"enrolled_at"`
	WithdrawnAt    string `json:"withdrawn_at,omitempty"`
}

func newEnrollmentResponse(e *domain.Enrollment) enrollmentResponse {
	out := enrollmentResponse{
		ID: e.ID, StudentID: e.StudentID, CourseStableID: e.CourseStableID,
		Status: string(e.Status), EnrolledAt: e.EnrolledAt.UTC().Format(time.RFC3339),
	}
	if e.WithdrawnAt != nil {
		out.WithdrawnAt = e.WithdrawnAt.UTC().Format(time.RFC3339)
	}
	return out
}

func newEnrollmentList(list []*domain.Enrollment) []enrollmentResponse {
	// A non-nil empty slice so an empty roster serialises as [], not null.
	out := make([]enrollmentResponse, 0, len(list))
	for _, e := range list {
		out = append(out, newEnrollmentResponse(e))
	}
	return out
}

// Enroll signs the caller up for a course.
//
// It answers 200, not 201, because it is idempotent: a repeat does not create a
// second enrollment, and reporting "created" on a request that created nothing
// would be a lie the client may act on.
func (h *EnrollmentHandler) Enroll(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	created, err := h.service.Enroll(r.Context(), actor, r.PathValue("courseID"))
	if err != nil {
		h.respondEnrollmentError(w, r, err)
		return
	}
	RespondWithJSON(w, http.StatusOK, newEnrollmentResponse(created))
}

// Withdraw takes the caller off a course, keeping their progress.
func (h *EnrollmentHandler) Withdraw(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	updated, err := h.service.Withdraw(r.Context(), actor, r.PathValue("courseID"))
	if err != nil {
		h.respondEnrollmentError(w, r, err)
		return
	}
	RespondWithJSON(w, http.StatusOK, newEnrollmentResponse(updated))
}

// Mine lists the caller's enrollments.
func (h *EnrollmentHandler) Mine(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	list, err := h.service.Mine(r.Context(), actor)
	if err != nil {
		h.respondEnrollmentError(w, r, err)
		return
	}
	RespondWithJSON(w, http.StatusOK, map[string]any{"items": newEnrollmentList(list)})
}

// Status reports whether the caller is enrolled in a course.
func (h *EnrollmentHandler) Status(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	current, err := h.service.Status(r.Context(), actor, r.PathValue("courseID"))
	if err != nil {
		h.respondEnrollmentError(w, r, err)
		return
	}
	if current == nil {
		// Never enrolled. Not a 404: the course exists, the caller simply has no
		// enrollment in it, and a client asking "am I enrolled" deserves an
		// answer rather than an error.
		RespondWithJSON(w, http.StatusOK, map[string]any{"enrolled": false})
		return
	}
	RespondWithJSON(w, http.StatusOK, map[string]any{
		"enrolled":   current.Active(),
		"enrollment": newEnrollmentResponse(current),
	})
}

// Roster lists who is taking a course. Author and administrators only.
func (h *EnrollmentHandler) Roster(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	list, err := h.service.Roster(r.Context(), actor, r.PathValue("courseID"))
	if err != nil {
		h.respondEnrollmentError(w, r, err)
		return
	}
	RespondWithJSON(w, http.StatusOK, map[string]any{"items": newEnrollmentList(list)})
}

// ---- shared -----------------------------------------------------------

func (h *EnrollmentHandler) actor(w http.ResponseWriter, r *http.Request) (enrollment.Actor, bool) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		RespondWithError(w, http.StatusUnauthorized, "unauthorized", "Se requiere autenticación.", nil)
		return enrollment.Actor{}, false
	}
	return enrollment.Actor{ID: user.ID, Role: user.Role, IPAddress: clientIP(r), UserAgent: r.UserAgent()}, true
}

func (h *EnrollmentHandler) respondEnrollmentError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrConflict):
		RespondWithError(w, http.StatusConflict, "course_not_published",
			"Solo es posible inscribirse en un curso publicado.", nil)
	case errors.Is(err, domain.ErrNotFound):
		RespondWithError(w, http.StatusNotFound, "not_found", "El curso solicitado no existe.", nil)
	case errors.Is(err, domain.ErrForbidden):
		RespondWithError(w, http.StatusForbidden, "forbidden", "No tienes permiso para realizar esta operación.", nil)
	case errors.Is(err, domain.ErrInvalidInput):
		RespondWithError(w, http.StatusBadRequest, "invalid_input", "Los datos enviados no son válidos.", nil)
	default:
		h.logger.ErrorContext(r.Context(), "unhandled error in enrollment handler",
			slog.String("request_id", w.Header().Get("X-Request-ID")),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("error", err.Error()),
		)
		RespondWithError(w, http.StatusInternalServerError, "internal_error", "Ocurrió un error inesperado.", nil)
	}
}
