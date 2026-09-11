package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/structure"
)

// StructureHandler exposes Module, Unit and Resource CRUD (issue #19) over
// /api/v1. Listing is public, matching how a course's modules are already
// exposed; every write requires a professor or administrator, enforced by
// middleware.RequireRole in server.go the same way as course authoring.
type StructureHandler struct {
	service *structure.Service
	logger  *slog.Logger
}

func NewStructureHandler(service *structure.Service, logger *slog.Logger) *StructureHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &StructureHandler{service: service, logger: logger}
}

type moduleResponse struct {
	ID       string `json:"id"`
	StableID string `json:"stable_id"`
	CourseID string `json:"course_id"`
	Title    string `json:"title"`
	Position int    `json:"position"`
}

func newModuleResponse(m *domain.Module) moduleResponse {
	return moduleResponse{ID: m.ID, StableID: m.StableID, CourseID: m.CourseID, Title: m.Title, Position: m.Position}
}

type unitResponse struct {
	ID       string `json:"id"`
	StableID string `json:"stable_id"`
	ModuleID string `json:"module_id"`
	Title    string `json:"title"`
	Position int    `json:"position"`
}

func newUnitResponse(u *domain.Unit) unitResponse {
	return unitResponse{ID: u.ID, StableID: u.StableID, ModuleID: u.ModuleID, Title: u.Title, Position: u.Position}
}

type resourceResponse struct {
	ID               string `json:"id"`
	StableID         string `json:"stable_id"`
	UnitID           string `json:"unit_id"`
	Title            string `json:"title"`
	Type             string `json:"type"`
	Position         int    `json:"position"`
	IsVisible        bool   `json:"is_visible"`
	IsMandatory      bool   `json:"is_mandatory"`
	AllowDownload    bool   `json:"allow_download"`
	ContentText      string `json:"content_text,omitempty"`
	ObjectKey        string `json:"object_key,omitempty"`
	ProcessingStatus string `json:"processing_status,omitempty"`
}

func newResourceResponse(r *domain.Resource) resourceResponse {
	return resourceResponse{
		ID: r.ID, StableID: r.StableID, UnitID: r.UnitID, Title: r.Title, Type: string(r.Type), Position: r.Position,
		IsVisible: r.IsVisible, IsMandatory: r.IsMandatory, AllowDownload: r.AllowDownload,
		ContentText: r.ContentText, ObjectKey: r.ObjectKey, ProcessingStatus: r.ProcessingStatus,
	}
}

// ---- Modules --------------------------------------------------------------

type createModuleRequest struct {
	Title string `json:"title"`
}

type updateModuleRequest struct {
	Title string `json:"title"`
}

// CreateModule handles POST /api/v1/courses/{courseID}/modules.
func (h *StructureHandler) CreateModule(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	courseID := r.PathValue("courseID")
	var req createModuleRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	created, err := h.service.CreateModule(r.Context(), actor, courseID, req.Title)
	if err != nil {
		h.respondStructureError(w, r, err)
		return
	}
	RespondWithJSON(w, http.StatusCreated, newModuleResponse(created))
}

// ListModules handles GET /api/v1/courses/{courseID}/modules.
func (h *StructureHandler) ListModules(w http.ResponseWriter, r *http.Request) {
	modules, err := h.service.ListModules(r.Context(), r.PathValue("courseID"))
	if err != nil {
		h.respondStructureError(w, r, err)
		return
	}
	items := make([]moduleResponse, 0, len(modules))
	for _, m := range modules {
		items = append(items, newModuleResponse(m))
	}
	RespondWithJSON(w, http.StatusOK, map[string]any{"items": items})
}

// UpdateModule handles PUT /api/v1/modules/{moduleID}.
func (h *StructureHandler) UpdateModule(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	var req updateModuleRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	updated, err := h.service.UpdateModule(r.Context(), actor, r.PathValue("moduleID"), req.Title)
	if err != nil {
		h.respondStructureError(w, r, err)
		return
	}
	RespondWithJSON(w, http.StatusOK, newModuleResponse(updated))
}

// DeleteModule handles DELETE /api/v1/modules/{moduleID}.
func (h *StructureHandler) DeleteModule(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	if err := h.service.DeleteModule(r.Context(), actor, r.PathValue("moduleID")); err != nil {
		h.respondStructureError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Units ----------------------------------------------------------------

type createUnitRequest struct {
	Title string `json:"title"`
}

type updateUnitRequest struct {
	Title string `json:"title"`
}

// CreateUnit handles POST /api/v1/modules/{moduleID}/units.
func (h *StructureHandler) CreateUnit(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	var req createUnitRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	created, err := h.service.CreateUnit(r.Context(), actor, r.PathValue("moduleID"), req.Title)
	if err != nil {
		h.respondStructureError(w, r, err)
		return
	}
	RespondWithJSON(w, http.StatusCreated, newUnitResponse(created))
}

// ListUnits handles GET /api/v1/modules/{moduleID}/units.
func (h *StructureHandler) ListUnits(w http.ResponseWriter, r *http.Request) {
	units, err := h.service.ListUnits(r.Context(), r.PathValue("moduleID"))
	if err != nil {
		h.respondStructureError(w, r, err)
		return
	}
	items := make([]unitResponse, 0, len(units))
	for _, u := range units {
		items = append(items, newUnitResponse(u))
	}
	RespondWithJSON(w, http.StatusOK, map[string]any{"items": items})
}

// UpdateUnit handles PUT /api/v1/units/{unitID}.
func (h *StructureHandler) UpdateUnit(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	var req updateUnitRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	updated, err := h.service.UpdateUnit(r.Context(), actor, r.PathValue("unitID"), req.Title)
	if err != nil {
		h.respondStructureError(w, r, err)
		return
	}
	RespondWithJSON(w, http.StatusOK, newUnitResponse(updated))
}

// DeleteUnit handles DELETE /api/v1/units/{unitID}.
func (h *StructureHandler) DeleteUnit(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	if err := h.service.DeleteUnit(r.Context(), actor, r.PathValue("unitID")); err != nil {
		h.respondStructureError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Resources --------------------------------------------------------------

type createResourceRequest struct {
	Title         string `json:"title"`
	Type          string `json:"type"`
	IsVisible     *bool  `json:"is_visible"`
	IsMandatory   *bool  `json:"is_mandatory"`
	AllowDownload bool   `json:"allow_download"`
	ContentText   string `json:"content_text"`
}

type updateResourceRequest struct {
	Title         string `json:"title"`
	IsVisible     bool   `json:"is_visible"`
	IsMandatory   bool   `json:"is_mandatory"`
	AllowDownload bool   `json:"allow_download"`
}

// CreateResource handles POST /api/v1/units/{unitID}/resources.
func (h *StructureHandler) CreateResource(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	var req createResourceRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	// A resource is visible and required to pass unless the caller explicitly
	// says otherwise: the pointer distinguishes "omitted" from "false".
	isVisible := true
	if req.IsVisible != nil {
		isVisible = *req.IsVisible
	}
	isMandatory := true
	if req.IsMandatory != nil {
		isMandatory = *req.IsMandatory
	}

	created, err := h.service.CreateResource(r.Context(), actor, r.PathValue("unitID"), structure.NewResourceInput{
		Title: req.Title, Type: domain.ResourceType(req.Type),
		IsVisible: isVisible, IsMandatory: isMandatory, AllowDownload: req.AllowDownload,
		ContentText: req.ContentText,
	})
	if err != nil {
		h.respondStructureError(w, r, err)
		return
	}
	RespondWithJSON(w, http.StatusCreated, newResourceResponse(created))
}

// ListResources handles GET /api/v1/units/{unitID}/resources.
func (h *StructureHandler) ListResources(w http.ResponseWriter, r *http.Request) {
	resources, err := h.service.ListResources(r.Context(), r.PathValue("unitID"))
	if err != nil {
		h.respondStructureError(w, r, err)
		return
	}
	items := make([]resourceResponse, 0, len(resources))
	for _, res := range resources {
		items = append(items, newResourceResponse(res))
	}
	RespondWithJSON(w, http.StatusOK, map[string]any{"items": items})
}

// UpdateResource handles PUT /api/v1/resources/{resourceID}.
func (h *StructureHandler) UpdateResource(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	var req updateResourceRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	updated, err := h.service.UpdateResource(r.Context(), actor, r.PathValue("resourceID"), domain.ResourceUpdate{
		Title: req.Title, IsVisible: req.IsVisible, IsMandatory: req.IsMandatory, AllowDownload: req.AllowDownload,
	})
	if err != nil {
		h.respondStructureError(w, r, err)
		return
	}
	RespondWithJSON(w, http.StatusOK, newResourceResponse(updated))
}

// DeleteResource handles DELETE /api/v1/resources/{resourceID}.
func (h *StructureHandler) DeleteResource(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	if err := h.service.DeleteResource(r.Context(), actor, r.PathValue("resourceID")); err != nil {
		h.respondStructureError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- shared -----------------------------------------------------------

// actor mirrors CourseHandler.actor; not shared because the two build values
// of different package-local Actor types.
func (h *StructureHandler) actor(w http.ResponseWriter, r *http.Request) (structure.Actor, bool) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		RespondWithError(w, http.StatusUnauthorized, "unauthorized", "Se requiere autenticación.", nil)
		return structure.Actor{}, false
	}
	return structure.Actor{ID: user.ID, Role: user.Role, IPAddress: clientIP(r), UserAgent: r.UserAgent()}, true
}

func (h *StructureHandler) respondStructureError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrCourseImmutable):
		RespondWithError(w, http.StatusConflict, "course_immutable",
			"Una versión publicada es inmutable. Despublica el curso antes de editar su estructura.", nil)
	case errors.Is(err, domain.ErrInvalidInput):
		RespondWithError(w, http.StatusBadRequest, "invalid_input", "Los datos enviados no son válidos.", nil)
	case errors.Is(err, domain.ErrNotFound):
		RespondWithError(w, http.StatusNotFound, "not_found", "El elemento solicitado no existe.", nil)
	case errors.Is(err, domain.ErrForbidden):
		RespondWithError(w, http.StatusForbidden, "forbidden", "No tienes permiso para realizar esta operación.", nil)
	default:
		h.logger.ErrorContext(r.Context(), "unhandled error in structure handler",
			slog.String("request_id", w.Header().Get("X-Request-ID")),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("error", err.Error()),
		)
		RespondWithError(w, http.StatusInternalServerError, "internal_error", "Ocurrió un error inesperado.", nil)
	}
}
