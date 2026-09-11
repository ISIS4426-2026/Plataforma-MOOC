package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
)

// AuditHandler exposes the read side of the audit trail over
// /api/v1/admin/audit-logs (issue #18).
//
// It talks to domain.AuditRepository directly rather than through a service:
// there is no business rule between an administrator's query and the rows it
// returns, only parsing and pagination, so a service layer here would just
// forward calls.
type AuditHandler struct {
	repo   domain.AuditRepository
	logger *slog.Logger
}

func NewAuditHandler(repo domain.AuditRepository, logger *slog.Logger) *AuditHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &AuditHandler{repo: repo, logger: logger}
}

type auditEntryResponse struct {
	ID             string         `json:"id"`
	ActorID        *string        `json:"actor_id,omitempty"`
	Action         string         `json:"action"`
	TargetResource string         `json:"target_resource"`
	Details        map[string]any `json:"details,omitempty"`
	IPAddress      string         `json:"ip_address,omitempty"`
	UserAgent      string         `json:"user_agent,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
}

func newAuditEntryResponse(e *domain.AuditEntry) auditEntryResponse {
	return auditEntryResponse{
		ID:             e.ID,
		ActorID:        e.ActorID,
		Action:         string(e.Action),
		TargetResource: e.TargetResource,
		Details:        e.Details,
		IPAddress:      e.IPAddress,
		UserAgent:      e.UserAgent,
		CreatedAt:      e.CreatedAt,
	}
}

type auditLogListResponse struct {
	Items      []auditEntryResponse `json:"items"`
	Pagination CursorPagination     `json:"pagination"`
}

// List handles GET /api/v1/admin/audit-logs.
func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	filter := domain.AuditFilter{
		ActorID:        query.Get("actor_id"),
		ActionPrefix:   query.Get("action_prefix"),
		TargetResource: query.Get("target_resource"),
		Cursor:         query.Get("cursor"),
	}

	if raw := query.Get("from"); raw != "" {
		from, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, "invalid_input",
				"El parámetro from debe ser una fecha en formato RFC3339.", nil)
			return
		}
		filter.From = &from
	}
	if raw := query.Get("to"); raw != "" {
		to, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, "invalid_input",
				"El parámetro to debe ser una fecha en formato RFC3339.", nil)
			return
		}
		filter.To = &to
	}
	if raw := query.Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, "invalid_input",
				"El parámetro limit debe ser un número.", nil)
			return
		}
		filter.Limit = limit
	}

	page, err := h.repo.List(r.Context(), filter)
	if err != nil {
		h.respondAuditError(w, r, err)
		return
	}

	items := make([]auditEntryResponse, 0, len(page.Entries))
	for _, entry := range page.Entries {
		items = append(items, newAuditEntryResponse(entry))
	}

	response := auditLogListResponse{
		Items:      items,
		Pagination: CursorPagination{HasMore: page.HasMore},
	}
	if page.NextCursor != "" {
		cursor := page.NextCursor
		response.Pagination.NextCursor = &cursor
	}

	RespondWithJSON(w, http.StatusOK, response)
}

func (h *AuditHandler) respondAuditError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		RespondWithError(w, http.StatusBadRequest, "invalid_input",
			"Los parámetros de búsqueda no son válidos.", nil)
	default:
		h.logger.ErrorContext(r.Context(), "unhandled error in audit handler",
			slog.String("request_id", w.Header().Get("X-Request-ID")),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("error", err.Error()),
		)
		RespondWithError(w, http.StatusInternalServerError, "internal_error",
			"Ocurrió un error inesperado.", nil)
	}
}
