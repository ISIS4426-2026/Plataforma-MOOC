package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/media"
)

// MediaHandler exposes the direct-upload flow over /api/v1.
//
// It is two calls, not one, because the object does not exist when the URL is
// issued: the client asks for authorization, transfers to the bucket itself,
// and then confirms. The confirmation is what records the key and queues the
// processing job, and it is also the boundary the capacity scenario measures
// separately from the transfer.
type MediaHandler struct {
	service *media.Service
	logger  *slog.Logger
}

func NewMediaHandler(service *media.Service, logger *slog.Logger) *MediaHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &MediaHandler{service: service, logger: logger}
}

type presignedURLRequest struct {
	ResourceID    string `json:"resource_id"`
	Filename      string `json:"filename"`
	MimeType      string `json:"mime_type"`
	FileSizeBytes int64  `json:"file_size_bytes"`
}

type presignedURLResponse struct {
	UploadURL   string `json:"upload_url"`
	ObjectKey   string `json:"object_key"`
	Method      string `json:"method"`
	ContentType string `json:"content_type"`
	ExpiresAt   string `json:"expires_at"`
}

type confirmUploadRequest struct {
	ObjectKey string `json:"object_key"`
}

type downloadURLResponse struct {
	DownloadURL string `json:"download_url"`
	ExpiresAt   string `json:"expires_at"`
}

// PresignedUploadURL authorizes a transfer and signs the URL for it.
func (h *MediaHandler) PresignedUploadURL(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	var req presignedURLRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	ticket, err := h.service.AuthorizeUpload(r.Context(), actor, media.UploadRequest{
		ResourceID: req.ResourceID,
		Filename:   req.Filename,
		MimeType:   req.MimeType,
		SizeBytes:  req.FileSizeBytes,
	})
	if err != nil {
		h.respondMediaError(w, r, err)
		return
	}

	RespondWithJSON(w, http.StatusOK, presignedURLResponse{
		UploadURL:   ticket.UploadURL,
		ObjectKey:   ticket.ObjectKey,
		Method:      ticket.Method,
		ContentType: ticket.ContentType,
		ExpiresAt:   ticket.ExpiresAt.Format(time.RFC3339),
	})
}

// ConfirmUpload verifies the object landed and hands it to the worker.
func (h *MediaHandler) ConfirmUpload(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	var req confirmUploadRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	updated, err := h.service.ConfirmUpload(r.Context(), actor, r.PathValue("resourceID"), req.ObjectKey)
	if err != nil {
		h.respondMediaError(w, r, err)
		return
	}
	RespondWithJSON(w, http.StatusAccepted, newResourceResponse(updated))
}

// DownloadURL issues a short-lived read URL for a resource's stored object.
func (h *MediaHandler) DownloadURL(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}

	url, expiresAt, err := h.service.DownloadURL(r.Context(), actor, r.PathValue("resourceID"))
	if err != nil {
		h.respondMediaError(w, r, err)
		return
	}
	RespondWithJSON(w, http.StatusOK, downloadURLResponse{
		DownloadURL: url,
		ExpiresAt:   expiresAt.Format(time.RFC3339),
	})
}

// ---- shared -----------------------------------------------------------

func (h *MediaHandler) actor(w http.ResponseWriter, r *http.Request) (media.Actor, bool) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		RespondWithError(w, http.StatusUnauthorized, "unauthorized", "Se requiere autenticación.", nil)
		return media.Actor{}, false
	}
	return media.Actor{ID: user.ID, Role: user.Role, IPAddress: clientIP(r), UserAgent: r.UserAgent()}, true
}

func (h *MediaHandler) respondMediaError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrObjectNotFound):
		// Not a 404: the resource exists, the upload just never landed. The
		// caller's next step is to retry the transfer, not to look elsewhere.
		RespondWithError(w, http.StatusConflict, "upload_not_found",
			"No se encontró el archivo en el almacenamiento. Vuelve a subirlo antes de confirmar.", nil)
	case errors.Is(err, domain.ErrCourseImmutable):
		RespondWithError(w, http.StatusConflict, "course_immutable",
			"Una versión publicada es inmutable. Despublica el curso antes de cambiar sus archivos.", nil)
	case errors.Is(err, domain.ErrInvalidInput):
		RespondWithError(w, http.StatusBadRequest, "invalid_input", "Los datos enviados no son válidos.", nil)
	case errors.Is(err, domain.ErrNotFound):
		RespondWithError(w, http.StatusNotFound, "not_found", "El elemento solicitado no existe.", nil)
	case errors.Is(err, domain.ErrForbidden):
		RespondWithError(w, http.StatusForbidden, "forbidden", "No tienes permiso para realizar esta operación.", nil)
	default:
		h.logger.ErrorContext(r.Context(), "unhandled error in media handler",
			slog.String("request_id", w.Header().Get("X-Request-ID")),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("error", err.Error()),
		)
		RespondWithError(w, http.StatusInternalServerError, "internal_error", "Ocurrió un error inesperado.", nil)
	}
}
