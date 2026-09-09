package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/auth"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
)

// maxAuthBodyBytes caps request bodies on the authentication endpoints. They
// carry a handful of short fields, so anything larger is either a mistake or an
// attempt to make the server buffer arbitrary input.
const maxAuthBodyBytes = 16 << 10 // 16 KiB

// AuthHandler exposes public registration, email verification and session
// management over /api/v1/auth.
type AuthHandler struct {
	service *auth.Service
	logger  *slog.Logger
}

func NewAuthHandler(service *auth.Service, logger *slog.Logger) *AuthHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &AuthHandler{service: service, logger: logger}
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

type sessionResponse struct {
	ID             string    `json:"id"`
	UserAgent      string    `json:"user_agent,omitempty"`
	IPAddress      string    `json:"ip_address,omitempty"`
	ExpiresAt      time.Time `json:"expires_at"`
	CreatedAt      time.Time `json:"created_at"`
	LastActivityAt time.Time `json:"last_activity_at"`
}

func newSessionResponse(session *domain.Session) sessionResponse {
	return sessionResponse{
		ID:             session.ID,
		UserAgent:      session.UserAgent,
		IPAddress:      session.IPAddress,
		ExpiresAt:      session.ExpiresAt,
		CreatedAt:      session.CreatedAt,
		LastActivityAt: session.LastActivityAt,
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

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type forgotPasswordRequest struct {
	Email string `json:"email"`
}

type resetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token     string       `json:"token"`
	ExpiresAt time.Time    `json:"expires_at"`
	User      userResponse `json:"user"`
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
		h.respondAuthError(w, r, err)
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
		h.respondAuthError(w, r, err)
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
		h.respondAuthError(w, r, err)
		return
	}

	RespondWithJSON(w, http.StatusAccepted, map[string]any{
		"message": "Si la cuenta existe y está pendiente de verificación, enviamos un nuevo enlace.",
	})
}

// Login handles POST /api/v1/auth/login and returns the session token.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	result, err := h.service.Login(r.Context(), auth.LoginInput{
		Email:     req.Email,
		Password:  req.Password,
		UserAgent: r.UserAgent(),
		IPAddress: clientIP(r),
	})
	if err != nil {
		h.respondAuthError(w, r, err)
		return
	}

	RespondWithJSON(w, http.StatusOK, loginResponse{
		Token:     result.Token,
		ExpiresAt: result.Session.ExpiresAt,
		User:      newUserResponse(result.User),
	})
}

// ForgotPassword handles POST /api/v1/auth/password/forgot.
//
// It always answers 202 with the same body, whether or not the address is
// registered, so the endpoint cannot be used to enumerate accounts.
func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req forgotPasswordRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if err := h.service.RequestPasswordReset(r.Context(), req.Email); err != nil {
		h.respondAuthError(w, r, err)
		return
	}

	RespondWithJSON(w, http.StatusAccepted, map[string]any{
		"message": "Si la cuenta existe, enviamos un enlace para restablecer la contraseña.",
	})
}

// ResetPassword handles POST /api/v1/auth/password/reset.
//
// On success every session of the account is already revoked, so any other
// device is logged out before this returns.
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req resetPasswordRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if err := h.service.ResetPassword(r.Context(), req.Token, req.Password); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			// Unknown, already used and expired tokens are indistinguishable.
			RespondWithError(w, http.StatusBadRequest, "invalid_token",
				"El enlace de recuperación no es válido o ya fue utilizado.", nil)
			return
		}
		h.respondAuthError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Logout handles POST /api/v1/auth/logout and revokes the presented session.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	token, ok := BearerToken(r)
	if !ok {
		RespondWithError(w, http.StatusUnauthorized, "unauthorized",
			"Se requiere un token de sesión.", nil)
		return
	}

	if err := h.service.Logout(r.Context(), token); err != nil {
		h.respondAuthError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListSessions handles GET /api/v1/auth/sessions for the authenticated user.
func (h *AuthHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		RespondWithError(w, http.StatusUnauthorized, "unauthorized", "Se requiere autenticación.", nil)
		return
	}

	sessions, err := h.service.ListSessions(r.Context(), user.ID)
	if err != nil {
		h.respondAuthError(w, r, err)
		return
	}

	items := make([]sessionResponse, 0, len(sessions))
	for _, session := range sessions {
		items = append(items, newSessionResponse(session))
	}

	RespondWithJSON(w, http.StatusOK, map[string]any{"items": items})
}

// RevokeSession handles DELETE /api/v1/auth/sessions/{sessionID}.
//
// This is the self-service half of the immediate revocation requirement: after
// it returns, a request carrying that session's token is rejected at once.
func (h *AuthHandler) RevokeSession(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		RespondWithError(w, http.StatusUnauthorized, "unauthorized", "Se requiere autenticación.", nil)
		return
	}

	sessionID := r.PathValue("sessionID")
	if sessionID == "" {
		RespondWithError(w, http.StatusBadRequest, "invalid_input", "Falta el identificador de sesión.", nil)
		return
	}

	if err := h.service.RevokeSession(r.Context(), user.ID, sessionID); err != nil {
		h.respondAuthError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
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
// registered, and no error carries the submitted password. An error that maps
// to no known case is logged before the generic 500, because a failure the
// operator cannot see is a failure they cannot fix.
func (h *AuthHandler) respondAuthError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		RespondWithError(w, http.StatusBadRequest, "invalid_input",
			"Los datos enviados no son válidos.", nil)
	case errors.Is(err, domain.ErrConflict):
		RespondWithError(w, http.StatusConflict, "email_already_registered",
			"No fue posible completar el registro con ese correo.", nil)
	case errors.Is(err, auth.ErrEmailNotVerified):
		RespondWithError(w, http.StatusForbidden, "email_not_verified",
			"Debes verificar tu correo antes de iniciar sesión.", nil)
	case errors.Is(err, auth.ErrAccountSuspended):
		RespondWithError(w, http.StatusForbidden, "account_suspended",
			"La cuenta está suspendida.", nil)
	case errors.Is(err, domain.ErrUnauthorized):
		RespondWithError(w, http.StatusUnauthorized, "invalid_credentials",
			"Credenciales inválidas.", nil)
	case errors.Is(err, domain.ErrForbidden):
		RespondWithError(w, http.StatusForbidden, "forbidden",
			"No tienes permiso para realizar esta operación.", nil)
	case errors.Is(err, domain.ErrNotFound):
		RespondWithError(w, http.StatusNotFound, "not_found",
			"El recurso solicitado no existe.", nil)
	default:
		// The correlation id is read back from the response header rather than
		// from the context: the middleware that owns that context key imports
		// this package, so reading it directly would be an import cycle.
		h.logger.ErrorContext(r.Context(), "unhandled error in auth handler",
			slog.String("request_id", w.Header().Get("X-Request-ID")),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("error", err.Error()),
		)
		RespondWithError(w, http.StatusInternalServerError, "internal_error",
			"Ocurrió un error inesperado.", nil)
	}
}

// BearerToken extracts the credential from the Authorization header.
func BearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return "", false
	}

	const prefix = "Bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}

	token := strings.TrimSpace(header[len(prefix):])
	if token == "" {
		return "", false
	}

	return token, true
}

// clientIP reports the peer address of the connection.
//
// Forwarding headers are deliberately ignored: any client can set
// X-Forwarded-For, so trusting it would let a caller forge the address written
// into the session record. A proxy-aware version needs a configured list of
// trusted proxies.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
