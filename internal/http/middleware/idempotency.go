package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/http/handler"
)

// maxIdempotentBodyBytes bounds what is buffered to compute the fingerprint and
// to hand back to the handler. It matches the cap the authentication endpoints
// already apply to their bodies.
const maxIdempotentBodyBytes = 16 << 10

// HeaderIdempotencyKey is the request header that opts into replay protection.
const HeaderIdempotencyKey = "Idempotency-Key"

// maxIdempotencyKeyLength bounds the client-supplied key, which ends up in a
// Redis key and in logs.
const maxIdempotencyKeyLength = 255

// Idempotency makes a write endpoint safe to retry.
//
// A request carrying an Idempotency-Key is executed once; any later request with
// the same key gets the recorded response, without the operation running again.
// Without a key the request passes straight through, so the protection is
// something a client opts into rather than something imposed on every caller.
//
// # What the key is scoped to
//
// The stored key combines the method, the path, the authenticated user and the
// client-supplied value. A key is therefore meaningful only for the same
// operation by the same caller: reusing "abc123" on a different endpoint, or by
// a different user, is a different key and not a replay. Without that scoping
// one client's key could collide with another's and return them somebody else's
// response.
//
// # Reusing a key with a different body
//
// That is a client defect, not a retry, and it answers 422. Returning the first
// response would hide the mistake and make the client believe a request it never
// successfully sent had been applied.
//
// # Retrying while the original is still running
//
// That answers 409. The first request has not produced an answer to replay yet,
// and running the operation concurrently is exactly what the key exists to
// prevent.
func Idempotency(store domain.IdempotencyStore, ttl time.Duration, logger *slog.Logger) Middleware {
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientKey := r.Header.Get(HeaderIdempotencyKey)
			if clientKey == "" {
				next.ServeHTTP(w, r)
				return
			}

			if len(clientKey) > maxIdempotencyKeyLength {
				handler.RespondWithError(w, http.StatusBadRequest, "invalid_idempotency_key",
					"La cabecera Idempotency-Key excede la longitud permitida.", nil)
				return
			}

			body, err := readAndRestoreBody(r)
			if err != nil {
				handler.RespondWithError(w, http.StatusBadRequest, "invalid_body",
					"No fue posible leer el cuerpo de la petición.", nil)
				return
			}

			scopedKey := scopeKey(r, clientKey)
			fingerprint := fingerprintRequest(r, body)

			recorded, inProgress, err := store.Reserve(r.Context(), scopedKey, fingerprint, ttl)
			if err != nil {
				// Unlike the rate limiter, this fails closed. Letting the request
				// through would run an operation the client asked to be run at
				// most once, and a duplicated side effect is worse than a
				// retryable error.
				logger.ErrorContext(r.Context(), "idempotency store unavailable, refusing the request",
					slog.String("request_id", RequestIDFrom(r.Context())),
					slog.String("path", r.URL.Path),
					slog.String("error", err.Error()),
				)
				handler.RespondWithError(w, http.StatusServiceUnavailable, "idempotency_unavailable",
					"No fue posible garantizar la idempotencia de la solicitud. Reintenta más tarde.", nil)
				return
			}

			if inProgress {
				handler.RespondWithError(w, http.StatusConflict, "idempotency_in_progress",
					"Una solicitud con esta Idempotency-Key aún se está procesando.", nil)
				return
			}

			if recorded != nil {
				if recorded.RequestFingerprint != fingerprint {
					handler.RespondWithError(w, http.StatusUnprocessableEntity, "idempotency_key_reused",
						"La Idempotency-Key ya se usó con un cuerpo diferente.", nil)
					return
				}
				replay(w, recorded)
				return
			}

			capture := &capturingWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(capture, r)

			// Only a definitive outcome is worth replaying. A 5xx is a failure
			// the client should be able to retry for real, so the claim is
			// released instead of freezing the error for the whole window.
			if capture.status >= http.StatusInternalServerError {
				if err := store.Release(r.Context(), scopedKey); err != nil {
					logger.ErrorContext(r.Context(), "failed to release an idempotency claim",
						slog.String("request_id", RequestIDFrom(r.Context())),
						slog.String("error", err.Error()),
					)
				}
				return
			}

			response := &domain.RecordedResponse{
				Status:             capture.status,
				Body:               capture.body.Bytes(),
				ContentType:        capture.Header().Get("Content-Type"),
				RequestFingerprint: fingerprint,
			}
			if err := store.Store(r.Context(), scopedKey, response, ttl); err != nil {
				// The operation already ran and the client is getting its
				// answer; failing now would report an error for work that
				// succeeded. The cost is that a retry would run again, which is
				// why this is logged at error level.
				logger.ErrorContext(r.Context(), "failed to record an idempotent response",
					slog.String("request_id", RequestIDFrom(r.Context())),
					slog.String("error", err.Error()),
				)
			}
		})
	}
}

// replay writes a stored response back to the client.
//
// The header marks the answer as a replay, so a client comparing two responses
// can tell that the second one did not run the operation again.
func replay(w http.ResponseWriter, recorded *domain.RecordedResponse) {
	if recorded.ContentType != "" {
		w.Header().Set("Content-Type", recorded.ContentType)
	}
	w.Header().Set("Idempotent-Replay", "true")
	w.WriteHeader(recorded.Status)
	_, _ = w.Write(recorded.Body)
}

// readAndRestoreBody buffers the body so it can be fingerprinted, then puts it
// back for the handler, which has not read it yet.
func readAndRestoreBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxIdempotentBodyBytes))
	if err != nil {
		return nil, err
	}
	_ = r.Body.Close()

	r.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

// scopeKey binds the client key to the operation and the caller.
func scopeKey(r *http.Request, clientKey string) string {
	actor := "anonymous"
	if user, ok := handler.UserFromContext(r.Context()); ok {
		actor = user.ID
	}

	sum := sha256.Sum256([]byte(r.Method + "\n" + r.URL.Path + "\n" + actor + "\n" + clientKey))
	return hex.EncodeToString(sum[:])
}

// fingerprintRequest digests the payload the key was used with.
func fingerprintRequest(r *http.Request, body []byte) string {
	sum := sha256.Sum256(append([]byte(r.URL.RawQuery+"\n"), body...))
	return hex.EncodeToString(sum[:])
}

// capturingWriter records the response while still writing it to the client, so
// a retry can be answered without running the handler again.
type capturingWriter struct {
	http.ResponseWriter
	status      int
	body        bytes.Buffer
	wroteHeader bool
}

func (c *capturingWriter) WriteHeader(status int) {
	if c.wroteHeader {
		return
	}
	c.status = status
	c.wroteHeader = true
	c.ResponseWriter.WriteHeader(status)
}

func (c *capturingWriter) Write(b []byte) (int, error) {
	if !c.wroteHeader {
		c.wroteHeader = true
	}
	c.body.Write(b)
	return c.ResponseWriter.Write(b)
}
