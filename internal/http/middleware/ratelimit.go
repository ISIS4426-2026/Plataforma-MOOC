package middleware

import (
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/http/handler"
)

// Rate limit response headers, following the IETF draft naming that Swagger and
// most clients understand.
const (
	HeaderRateLimit          = "RateLimit-Limit"
	HeaderRateLimitRemaining = "RateLimit-Remaining"
	HeaderRateLimitReset     = "RateLimit-Reset"
	HeaderRetryAfter         = "Retry-After"
)

// RateLimitPolicy is the quota applied to one group of endpoints.
//
// Name is part of the counter key, so two endpoints sharing a policy value but
// given different names get independent quotas: exhausting the login limit must
// not lock a user out of password recovery.
type RateLimitPolicy struct {
	Name   string
	Limit  int
	Window time.Duration
}

// RateLimit throttles requests per client address and policy.
//
// The counter is keyed by policy and peer address. Forwarding headers are
// ignored on purpose: any client can set X-Forwarded-For, so trusting it would
// let an attacker mint a fresh quota per request and defeat the limit entirely.
// Behind a real proxy this needs a configured list of trusted hops.
//
// When the limiter itself fails the request is allowed through and the failure
// is logged. That is a deliberate trade: a Redis outage should degrade
// protection rather than lock every user out of logging in. It also means an
// outage temporarily removes the throttle, which is why the error is logged at
// error level.
func RateLimit(limiter domain.RateLimiter, policy RateLimitPolicy, logger *slog.Logger) Middleware {
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := policy.Name + ":" + peerAddress(r)

			result, err := limiter.Allow(r.Context(), key, policy.Limit, policy.Window)
			if err != nil {
				logger.ErrorContext(r.Context(), "rate limiter unavailable, allowing request",
					slog.String("request_id", RequestIDFrom(r.Context())),
					slog.String("policy", policy.Name),
					slog.String("error", err.Error()),
				)
				next.ServeHTTP(w, r)
				return
			}

			resetSeconds := int(result.RetryAfter.Seconds() + 0.999)
			w.Header().Set(HeaderRateLimit, strconv.Itoa(result.Limit))
			w.Header().Set(HeaderRateLimitRemaining, strconv.Itoa(result.Remaining))
			w.Header().Set(HeaderRateLimitReset, strconv.Itoa(resetSeconds))

			if !result.Allowed {
				w.Header().Set(HeaderRetryAfter, strconv.Itoa(resetSeconds))
				logger.WarnContext(r.Context(), "rate limit exceeded",
					slog.String("request_id", RequestIDFrom(r.Context())),
					slog.String("policy", policy.Name),
					slog.String("path", r.URL.Path),
				)
				handler.RespondWithError(w, http.StatusTooManyRequests, "rate_limit_exceeded",
					"Demasiadas solicitudes. Intenta de nuevo más tarde.", nil)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// peerAddress returns the address of the connection, without its port.
func peerAddress(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
