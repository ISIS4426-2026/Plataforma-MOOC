package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/course"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
)

// CourseHandler exposes course authoring over /api/v1/courses.
//
// List and Get are public, matching the catalog: a visitor browses courses
// before deciding to register. Create and Update sit behind requireAuth and
// RequireRole in server.go, so these two can assume an authenticated
// professor or administrator is present in the context.
type CourseHandler struct {
	service *course.Service
	logger  *slog.Logger
}

func NewCourseHandler(service *course.Service, logger *slog.Logger) *CourseHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &CourseHandler{service: service, logger: logger}
}

type createCourseRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type updateCourseRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type courseResponse struct {
	ID          string `json:"id"`
	StableID    string `json:"stable_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Version     int    `json:"version"`
	Status      string `json:"status"`
}

func newCourseResponse(c *domain.Course) courseResponse {
	return courseResponse{
		ID:          c.ID,
		StableID:    c.StableID,
		Title:       c.Title,
		Description: c.Description,
		Version:     c.Version,
		Status:      string(c.Status),
	}
}

type courseListResponse struct {
	Items      []courseResponse `json:"items"`
	Pagination CursorPagination `json:"pagination"`
}

// Create handles POST /api/v1/courses.
func (h *CourseHandler) Create(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}

	var req createCourseRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	created, err := h.service.Create(r.Context(), actor, req.Title, req.Description)
	if err != nil {
		h.respondCourseError(w, r, err)
		return
	}

	RespondWithJSON(w, http.StatusCreated, newCourseResponse(created))
}

// Get handles GET /api/v1/courses/{course_id}.
func (h *CourseHandler) Get(w http.ResponseWriter, r *http.Request) {
	courseID := r.PathValue("courseID")
	if courseID == "" {
		RespondWithError(w, http.StatusBadRequest, "invalid_input", "Falta el identificador del curso.", nil)
		return
	}

	found, err := h.service.Get(r.Context(), courseID)
	if err != nil {
		h.respondCourseError(w, r, err)
		return
	}

	RespondWithETag(w, r, found.ETag(), newCourseResponse(found))
}

// List handles GET /api/v1/courses.
func (h *CourseHandler) List(w http.ResponseWriter, r *http.Request) {
	filter := domain.CourseFilter{
		Search: r.URL.Query().Get("search"),
		Cursor: r.URL.Query().Get("cursor"),
	}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, "invalid_input",
				"El parámetro limit debe ser un número.", nil)
			return
		}
		filter.Limit = limit
	}

	page, err := h.service.ListPublished(r.Context(), filter)
	if err != nil {
		h.respondCourseError(w, r, err)
		return
	}

	items := make([]courseResponse, 0, len(page.Courses))
	for _, c := range page.Courses {
		items = append(items, newCourseResponse(c))
	}

	response := courseListResponse{
		Items:      items,
		Pagination: CursorPagination{HasMore: page.HasMore},
	}
	if page.NextCursor != "" {
		cursor := page.NextCursor
		response.Pagination.NextCursor = &cursor
	}

	RespondWithJSON(w, http.StatusOK, response)
}

// Update handles PUT /api/v1/courses/{course_id}.
func (h *CourseHandler) Update(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}

	courseID := r.PathValue("courseID")
	if courseID == "" {
		RespondWithError(w, http.StatusBadRequest, "invalid_input", "Falta el identificador del curso.", nil)
		return
	}

	var req updateCourseRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	updated, err := h.service.Update(r.Context(), actor, courseID, req.Title, req.Description, changeOptions(r))
	if err != nil {
		h.respondCourseError(w, r, err)
		return
	}

	w.Header().Set(HeaderETag, updated.ETag())
	RespondWithJSON(w, http.StatusOK, newCourseResponse(updated))
}

// actor builds the authoring actor from the authenticated request. It mirrors
// AdminHandler.actor; the two are not shared because they build values of
// different package-local types (admin.Actor vs course.Actor).
func (h *CourseHandler) actor(w http.ResponseWriter, r *http.Request) (course.Actor, bool) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		RespondWithError(w, http.StatusUnauthorized, "unauthorized", "Se requiere autenticación.", nil)
		return course.Actor{}, false
	}

	return course.Actor{
		ID:        user.ID,
		Role:      user.Role,
		IPAddress: clientIP(r),
		UserAgent: r.UserAgent(),
	}, true
}

func (h *CourseHandler) respondCourseError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrPreconditionFailed):
		RespondWithError(w, http.StatusPreconditionFailed, "precondition_failed",
			"El curso cambió desde la versión que tienes. Vuelve a leerlo y reintenta.", nil)
	case errors.Is(err, domain.ErrCourseImmutable):
		RespondWithError(w, http.StatusConflict, "course_immutable",
			"Una versión publicada es inmutable. Despublica el curso antes de editarlo.", nil)
	case errors.Is(err, domain.ErrInvalidInput):
		RespondWithError(w, http.StatusBadRequest, "invalid_input",
			"Los datos enviados no son válidos.", nil)
	case errors.Is(err, domain.ErrNotFound):
		RespondWithError(w, http.StatusNotFound, "not_found",
			"El curso solicitado no existe.", nil)
	case errors.Is(err, domain.ErrForbidden):
		RespondWithError(w, http.StatusForbidden, "forbidden",
			"No tienes permiso para realizar esta operación.", nil)
	default:
		h.logger.ErrorContext(r.Context(), "unhandled error in course handler",
			slog.String("request_id", w.Header().Get("X-Request-ID")),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("error", err.Error()),
		)
		RespondWithError(w, http.StatusInternalServerError, "internal_error",
			"Ocurrió un error inesperado.", nil)
	}
}
