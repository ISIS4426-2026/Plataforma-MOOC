package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/admin"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
)

// AdminHandler exposes administrative account management over /api/v1/admin.
//
// Every route it registers sits behind RequireAuth and RequireAdmin, so these
// handlers can assume an administrator is present in the context.
type AdminHandler struct {
	service *admin.Service
	logger  *slog.Logger
}

func NewAdminHandler(service *admin.Service, logger *slog.Logger) *AdminHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &AdminHandler{service: service, logger: logger}
}

type changeRoleRequest struct {
	Role string `json:"role"`
}

type changeStatusRequest struct {
	Status string `json:"status"`
}

type userListResponse struct {
	Items      []userResponse   `json:"items"`
	Pagination CursorPagination `json:"pagination"`
}

// ListUsers handles GET /api/v1/admin/users.
func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	filter := domain.UserFilter{Cursor: r.URL.Query().Get("cursor")}

	if raw := r.URL.Query().Get("role"); raw != "" {
		role := domain.Role(raw)
		filter.Role = &role
	}
	if raw := r.URL.Query().Get("status"); raw != "" {
		status := domain.UserStatus(raw)
		filter.Status = &status
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

	page, err := h.service.ListUsers(r.Context(), filter)
	if err != nil {
		h.respondAdminError(w, r, err)
		return
	}

	items := make([]userResponse, 0, len(page.Users))
	for _, user := range page.Users {
		items = append(items, newUserResponse(user))
	}

	response := userListResponse{
		Items:      items,
		Pagination: CursorPagination{HasMore: page.HasMore},
	}
	if page.NextCursor != "" {
		cursor := page.NextCursor
		response.Pagination.NextCursor = &cursor
	}

	RespondWithJSON(w, http.StatusOK, response)
}

// ChangeRole handles PATCH /api/v1/admin/users/{user_id}/role.
func (h *AdminHandler) ChangeRole(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}

	targetID := r.PathValue("userID")
	if targetID == "" {
		RespondWithError(w, http.StatusBadRequest, "invalid_input", "Falta el identificador de usuario.", nil)
		return
	}

	var req changeRoleRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	updated, err := h.service.ChangeRole(r.Context(), actor, targetID, domain.Role(req.Role), changeOptions(r))
	if err != nil {
		h.respondAdminError(w, r, err)
		return
	}

	RespondWithJSON(w, http.StatusOK, map[string]any{"user": newUserResponse(updated)})
}

// ChangeStatus handles PATCH /api/v1/admin/users/{user_id}/status.
func (h *AdminHandler) ChangeStatus(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}

	targetID := r.PathValue("userID")
	if targetID == "" {
		RespondWithError(w, http.StatusBadRequest, "invalid_input", "Falta el identificador de usuario.", nil)
		return
	}

	var req changeStatusRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	updated, err := h.service.ChangeStatus(r.Context(), actor, targetID, domain.UserStatus(req.Status), changeOptions(r))
	if err != nil {
		h.respondAdminError(w, r, err)
		return
	}

	respondWithUser(w, updated)
}

// GetUser handles GET /api/v1/admin/users/{user_id}.
//
// It carries the ETag a client needs before attempting a conditional write, and
// honours If-None-Match so a poll that finds nothing new costs a 304 instead of
// a full body.
func (h *AdminHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	targetID := r.PathValue("userID")
	if targetID == "" {
		RespondWithError(w, http.StatusBadRequest, "invalid_input", "Falta el identificador de usuario.", nil)
		return
	}

	user, err := h.service.GetUser(r.Context(), targetID)
	if err != nil {
		h.respondAdminError(w, r, err)
		return
	}

	RespondWithETag(w, r, user.ETag(), map[string]any{"user": newUserResponse(user)})
}

// respondWithUser writes the updated resource together with its new ETag, so a
// client can chain another conditional write without a second round trip.
func respondWithUser(w http.ResponseWriter, user *domain.User) {
	w.Header().Set(HeaderETag, user.ETag())
	RespondWithJSON(w, http.StatusOK, map[string]any{"user": newUserResponse(user)})
}

// changeOptions reads the precondition a caller attached to the request.
//
// A missing If-Match means "apply regardless", which keeps the header optional:
// requiring it would break every client that does not implement optimistic
// concurrency. Sending "*" means "the resource must exist", which it does by
// the time the change is attempted, so it imposes no version constraint.
func changeOptions(r *http.Request) domain.ChangeOptions {
	ifMatch := strings.TrimSpace(r.Header.Get(HeaderIfMatch))
	if ifMatch == "" || ifMatch == "*" {
		return domain.ChangeOptions{}
	}
	return domain.ChangeOptions{ExpectedETag: ifMatch}
}

// actor builds the audit actor from the authenticated request.
func (h *AdminHandler) actor(w http.ResponseWriter, r *http.Request) (admin.Actor, bool) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		RespondWithError(w, http.StatusUnauthorized, "unauthorized", "Se requiere autenticación.", nil)
		return admin.Actor{}, false
	}

	return admin.Actor{
		ID:        user.ID,
		IPAddress: clientIP(r),
		UserAgent: r.UserAgent(),
	}, true
}

// respondAdminError maps domain errors onto the uniform error contract.
func (h *AdminHandler) respondAdminError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrPreconditionFailed):
		// 412 rather than 409: the request was well formed and permitted, and
		// what failed is the condition the caller attached to it.
		RespondWithError(w, http.StatusPreconditionFailed, "precondition_failed",
			"El recurso cambió desde la versión que tienes. Vuelve a leerlo y reintenta.", nil)
	case errors.Is(err, domain.ErrLastAdminProtected):
		// 409 rather than 403: the caller has the permission, the platform
		// state is what forbids the change.
		RespondWithError(w, http.StatusConflict, "last_admin_protected",
			"No es posible dejar la plataforma sin administradores activos.", nil)
	case errors.Is(err, domain.ErrInvalidInput):
		RespondWithError(w, http.StatusBadRequest, "invalid_input",
			"Los datos enviados no son válidos.", nil)
	case errors.Is(err, domain.ErrNotFound):
		RespondWithError(w, http.StatusNotFound, "not_found",
			"El usuario solicitado no existe.", nil)
	case errors.Is(err, domain.ErrForbidden):
		RespondWithError(w, http.StatusForbidden, "forbidden",
			"No tienes permiso para realizar esta operación.", nil)
	default:
		h.logger.ErrorContext(r.Context(), "unhandled error in admin handler",
			slog.String("request_id", w.Header().Get("X-Request-ID")),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("error", err.Error()),
		)
		RespondWithError(w, http.StatusInternalServerError, "internal_error",
			"Ocurrió un error inesperado.", nil)
	}
}
