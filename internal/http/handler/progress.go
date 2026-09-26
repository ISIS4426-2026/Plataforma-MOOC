package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/progress"
)

// ProgressHandler exposes reporting progress, reading it, and verifying a badge
// (issue #113).
//
// Every route but verification acts on the caller's own record. Verification is
// the one public endpoint on the platform that answers about a named person, and
// it does so only to whoever holds the badge's code.
type ProgressHandler struct {
	service *progress.Service
	logger  *slog.Logger

	// appBaseURL is the platform's public origin, used to build the link a
	// student hands to whoever wants to check their badge. Building the link is a
	// presentation concern, so it lives here and not in the service.
	appBaseURL string
}

func NewProgressHandler(service *progress.Service, appBaseURL string, logger *slog.Logger) *ProgressHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &ProgressHandler{service: service, appBaseURL: appBaseURL, logger: logger}
}

type heartbeatRequest struct {
	ResourceID       string `json:"resource_id"`
	DwellTimeSeconds int    `json:"dwell_time_seconds"`
	Completed        bool   `json:"completed"`
}

// The field names follow api/openapi.yaml, which documented this response before
// it was implemented. percentage and completed_resource_stable_ids are its
// names; course_stable_id corrects course_id, since progress keys on the stable
// id -- as the spec's own array name already said.
type progressResponse struct {
	CourseStableID  string   `json:"course_stable_id"`
	CompletedStable []string `json:"completed_resource_stable_ids"`
	CompletedCount  int      `json:"completed_count"`
	TotalCount      int      `json:"total_count"`
	Percentage      float64  `json:"percentage"`
	IsApproved      bool     `json:"is_approved"`
	UpdatedAt       string   `json:"updated_at,omitempty"`
	Badge           *badge   `json:"badge,omitempty"`
}

// badge is the student's own view of their credential: the code they can share,
// the link that resolves it, and the date it was issued.
//
// The storage key is deliberately absent. It names an object the student has no
// business addressing, and no image is rendered at it yet anyway.
type badge struct {
	ID               string `json:"id"`
	CourseStableID   string `json:"course_stable_id,omitempty"`
	VerificationCode string `json:"verification_code"`
	VerificationURL  string `json:"verification_url,omitempty"`
	IssuedAt         string `json:"issued_at"`
	IsRevoked        bool   `json:"revoked"`
}

// newBadge is the one place a badge becomes JSON, so the standalone badge
// resource and the one nested in a progress response cannot drift apart.
func (h *ProgressHandler) newBadge(b *domain.Badge, withCourse bool) *badge {
	out := &badge{
		ID:               b.ID,
		VerificationCode: b.VerificationCode,
		VerificationURL:  domain.BadgeVerificationURL(h.appBaseURL, b.VerificationCode),
		IssuedAt:         b.IssuedAt.UTC().Format(time.RFC3339),
		IsRevoked:        b.IsRevoked,
	}
	// Inside a progress response the course is already the subject, so repeating
	// it would be noise; on its own the badge has to say what it certifies.
	if withCourse {
		out.CourseStableID = b.CourseStableID
	}
	return out
}

func (h *ProgressHandler) newProgressResponse(p *domain.StudentProgress, b *domain.Badge) progressResponse {
	out := progressResponse{
		CourseStableID: p.CourseStableID,
		// A non-nil empty slice so a student who has completed nothing gets [],
		// not null.
		CompletedStable: p.CompletedResources,
		CompletedCount:  p.CompletedCount,
		TotalCount:      p.TotalCount,
		Percentage:      p.PercentCompleted,
		IsApproved:      p.IsApproved,
	}
	if out.CompletedStable == nil {
		out.CompletedStable = []string{}
	}
	if !p.UpdatedAt.IsZero() {
		out.UpdatedAt = p.UpdatedAt.UTC().Format(time.RFC3339)
	}
	if b != nil {
		out.Badge = h.newBadge(b, false)
	}
	return out
}

// Heartbeat records that the caller spent time on a resource, and possibly
// finished it.
//
// It answers 200 with the recomputed progress rather than 201 or 204: a
// heartbeat's whole purpose is to learn where the student now stands, and the
// same report arriving twice leaves that answer unchanged.
func (h *ProgressHandler) Heartbeat(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}

	var body heartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		RespondWithError(w, http.StatusBadRequest, "invalid_input",
			"El cuerpo de la solicitud no es un JSON válido.", nil)
		return
	}

	updated, earned, err := h.service.Report(r.Context(), actor, domain.Heartbeat{
		ResourceID:       body.ResourceID,
		DwellTimeSeconds: body.DwellTimeSeconds,
		Completed:        body.Completed,
	})
	if err != nil {
		h.respondProgressError(w, r, err)
		return
	}
	RespondWithJSON(w, http.StatusOK, h.newProgressResponse(updated, earned))
}

// Course reports how far the caller has got in a course.
func (h *ProgressHandler) Course(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	current, earned, err := h.service.Course(r.Context(), actor, r.PathValue("courseID"))
	if err != nil {
		h.respondProgressError(w, r, err)
		return
	}
	RespondWithJSON(w, http.StatusOK, h.newProgressResponse(current, earned))
}

// GetBadge returns one of the caller's own badges, supporting conditional GET.
//
// It requires a session, although api/openapi.yaml did not say so when this
// endpoint was first documented: the response carries the verification code, and
// serving that to anyone who knows a badge id would turn the id into the
// credential and undo the point of having an unguessable code.
//
// A badge that is not the caller's answers 404 rather than 403, which is also
// what the spec's response list implies. See progress.Service.Badge for why.
func (h *ProgressHandler) GetBadge(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	earned, err := h.service.Badge(r.Context(), actor, r.PathValue("badge_id"))
	if err != nil {
		h.respondProgressError(w, r, err)
		return
	}
	RespondWithETag(w, r, earned.ETag(), h.newBadge(earned, true))
}

// VerifyBadge resolves a badge's public code. No session required: the code is
// the credential.
func (h *ProgressHandler) VerifyBadge(w http.ResponseWriter, r *http.Request) {
	credential, err := h.service.Verify(r.Context(), r.PathValue("verification_code"))
	if errors.Is(err, domain.ErrNotFound) {
		// Deliberately the same answer as a malformed code: an unauthenticated
		// endpoint should not help anyone tell "no such badge" from "not a badge
		// code at all".
		RespondWithError(w, http.StatusNotFound, "not_found",
			"No existe una insignia con ese código de verificación.", nil)
		return
	}
	if err != nil {
		h.respondProgressError(w, r, err)
		return
	}
	RespondWithJSON(w, http.StatusOK, map[string]any{
		// valid is the answer a verifier came for, and it is false for a revoked
		// badge even though the record exists.
		//
		// Nothing here identifies the student. The spec calls this endpoint
		// privacy-preserving verification, and the shape is the contract: a
		// verifier learns that the code corresponds to a genuine, unrevoked badge
		// for this course, issued on this date, and nothing about who holds it.
		"valid":        credential.Valid(),
		"course_title": credential.CourseTitle,
		"issued_at":    credential.IssuedAt.UTC().Format(time.RFC3339),
		"revoked":      credential.IsRevoked,
	})
}

// ---- shared -----------------------------------------------------------

func (h *ProgressHandler) actor(w http.ResponseWriter, r *http.Request) (progress.Actor, bool) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		RespondWithError(w, http.StatusUnauthorized, "unauthorized", "Se requiere autenticación.", nil)
		return progress.Actor{}, false
	}
	return progress.Actor{ID: user.ID, Role: user.Role, IPAddress: clientIP(r), UserAgent: r.UserAgent()}, true
}

func (h *ProgressHandler) respondProgressError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrConflict):
		RespondWithError(w, http.StatusConflict, "course_not_published",
			"Solo es posible registrar progreso en un curso publicado.", nil)
	case errors.Is(err, domain.ErrNotFound):
		RespondWithError(w, http.StatusNotFound, "not_found", "El recurso solicitado no existe.", nil)
	case errors.Is(err, domain.ErrForbidden):
		RespondWithError(w, http.StatusForbidden, "forbidden",
			"Se requiere una inscripción activa en el curso.", nil)
	case errors.Is(err, domain.ErrInvalidInput):
		RespondWithError(w, http.StatusBadRequest, "invalid_input", "Los datos enviados no son válidos.", nil)
	default:
		h.logger.ErrorContext(r.Context(), "unhandled error in progress handler",
			slog.String("request_id", w.Header().Get("X-Request-ID")),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("error", err.Error()),
		)
		RespondWithError(w, http.StatusInternalServerError, "internal_error", "Ocurrió un error inesperado.", nil)
	}
}
