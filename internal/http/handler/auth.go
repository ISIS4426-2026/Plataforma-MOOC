package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/auth"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
)

// maxAuthBodyBytes caps request bodies on the authentication endpoints. They
// carry a handful of short fields, so anything larger is either a mistake or an
// attempt to make the server buffer arbitrary input.
const maxAuthBodyBytes = 16 << 10 // 16 KiB

// AuthHandler exposes public registration and email verification over
// /api/v1/auth.
type AuthHandler struct {
	service *auth.Service
}

func NewAuthHandler(service *auth.Service) *AuthHandler {
	return &AuthHandler{service: service}
}

// userResponse is the public projection of a user.
//
// It is an explicit type rather than domain.User so a field added to the domain
// model can never reach a response by accident.
type userResponse struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	FullName string `json:"full_name"`
	Role     string `json:"role"`
	Status   string `json:"status"`
}

func newUserResponse(user *domain.User) userResponse {
	return userResponse{
		ID:       user.ID,
		Email:    user.Email,
		FullName: user.FullName,
		Role:     string(user.Role),
		Status:   string(user.Status),
	}
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
}

type resendVerificationRequest struct {
	Email string `json:"email"`
}

// Register handles POST /api/v1/auth/register.
//
// The request carries no role: this endpoint only ever creates students, and a
// role field in the body is rejected rather than honoured.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	user, err := h.service.Register(r.Context(), auth.RegisterInput{
		Email:    req.Email,
		Password: req.Password,
		FullName: req.FullName,
	})
	if err != nil {
		respondAuthError(w, err)
		return
	}

	// 201 with the created account; the activation link travels by email only.
	RespondWithJSON(w, http.StatusCreated, map[string]any{
		"user":    newUserResponse(user),
		"message": "Cuenta creada. Revisa tu correo para activarla.",
	})
}

// Verify handles GET /api/v1/auth/verify and activates the account.
func (h *AuthHandler) Verify(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")

	user, err := h.service.VerifyEmail(r.Context(), token)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			// Unknown, already used and expired tokens are indistinguishable.
			RespondWithError(w, http.StatusBadRequest, "invalid_token",
				"El enlace de verificación no es válido o ya fue utilizado.", nil)
			return
		}
		respondAuthError(w, err)
		return
	}

	RespondWithJSON(w, http.StatusOK, map[string]any{
		"user":    newUserResponse(user),
		"message": "Cuenta verificada correctamente.",
	})
}

// ResendVerification handles POST /api/v1/auth/verify/resend.
//
// It always answers 202 with the same message, whether or not the address is
// registered, so the endpoint cannot be used to enumerate accounts.
func (h *AuthHandler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	var req resendVerificationRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if err := h.service.ResendVerification(r.Context(), req.Email); err != nil {
		respondAuthError(w, err)
		return
	}

	RespondWithJSON(w, http.StatusAccepted, map[string]any{
		"message": "Si la cuenta existe y está pendiente de verificación, enviamos un nuevo enlace.",
	})
}

// decodeJSON reads a JSON body with a size limit and rejects unknown fields,
// reporting failures itself. It returns false when the response is already
// written.
func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthBodyBytes)

	decoder := json.NewDecoder(r.Body)
	// Unknown fields are rejected so a request that tries to smuggle in, say, a
	// role is refused outright rather than silently ignored.
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		RespondWithError(w, http.StatusBadRequest, "invalid_body",
			"El cuerpo de la petición no es JSON válido.", nil)
		return false
	}

	return true
}

// respondAuthError maps domain errors onto the uniform error contract.
//
// Messages are deliberately generic: nothing here reveals whether an email is
// registered, and no error carries the submitted password.
func respondAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		RespondWithError(w, http.StatusBadRequest, "invalid_input",
			"Los datos enviados no son válidos.", nil)
	case errors.Is(err, domain.ErrConflict):
		RespondWithError(w, http.StatusConflict, "email_already_registered",
			"No fue posible completar el registro con ese correo.", nil)
	case errors.Is(err, domain.ErrForbidden):
		RespondWithError(w, http.StatusForbidden, "forbidden",
			"No tienes permiso para realizar esta operación.", nil)
	case errors.Is(err, domain.ErrNotFound):
		RespondWithError(w, http.StatusNotFound, "not_found",
			"El recurso solicitado no existe.", nil)
	default:
		RespondWithError(w, http.StatusInternalServerError, "internal_error",
			"Ocurrió un error inesperado.", nil)
	}
}
