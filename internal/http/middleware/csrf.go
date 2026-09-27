package middleware

import (
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/http/handler"
)

// CSRF protection for requests authenticated by browser session cookies.
//
// # Why this is an origin check and not a synchroniser token
//
// Cross-site request forgery works because the browser attaches the victim's
// credentials to a request the attacker triggered. Session cookies are sent
// automatically, so mutating requests carrying one must declare an allowlisted
// Origin or Referer. Bearer tokens remain available to API clients and are not
// attached automatically by browsers.
//
// This middleware verifies the browser-supplied request origin against an
// exact allowlist. Requests with no cookie and no Origin/Referer remain
// compatible with non-browser Bearer clients.
const (
	HeaderOrigin  = "Origin"
	HeaderReferer = "Referer"
)

// CSRFConfig lists the origins allowed to submit state-changing requests.
type CSRFConfig struct {
	// AllowedOrigins are matched exactly, scheme and port included. Production
	// configuration requires a non-empty list.
	AllowedOrigins []string
}

// CSRF rejects state-changing requests that declare an untrusted origin.
//
// Safe methods are not checked: by definition they change nothing, and
// requiring an origin on them would break ordinary navigation and link
// previews.
//
// A request with neither Origin nor Referer is allowed through only when it
// does not carry the browser session cookie. Bearer credentials are explicit
// and are not attached automatically by browsers.
func CSRF(cfg CSRFConfig, logger *slog.Logger) Middleware {
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isSafeMethod(r.Method) {
				next.ServeHTTP(w, r)
				return
			}

			_, cookieErr := r.Cookie(handler.SessionCookieName)
			hasSessionCookie := cookieErr == nil
			declared, present := declaredOrigin(r)
			if !present {
				if !hasSessionCookie {
					next.ServeHTTP(w, r)
					return
				}
			} else if len(cfg.AllowedOrigins) == 0 && !hasSessionCookie {
				next.ServeHTTP(w, r)
				return
			}

			if !present || !slices.Contains(cfg.AllowedOrigins, declared) {
				logger.WarnContext(r.Context(), "rejected request from untrusted origin",
					slog.String("request_id", RequestIDFrom(r.Context())),
					slog.String("origin", declared),
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
				)
				handler.RespondWithError(w, http.StatusForbidden, "csrf_origin_rejected",
					"El origen de la solicitud no está permitido.", nil)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}

// declaredOrigin reports the origin the request claims, preferring the Origin
// header and falling back to the scheme and host of Referer.
//
// The fallback matters because a few browsers still omit Origin on same-site
// form posts; Referer carries the same information with a path attached, which
// is discarded here.
func declaredOrigin(r *http.Request) (string, bool) {
	if origin := strings.TrimSpace(r.Header.Get(HeaderOrigin)); origin != "" && origin != "null" {
		return origin, true
	}

	referer := strings.TrimSpace(r.Header.Get(HeaderReferer))
	if referer == "" {
		return "", false
	}

	parsed, err := url.Parse(referer)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		// A malformed Referer is treated as declaring an origin that cannot be
		// matched, rather than as no origin at all: otherwise sending garbage
		// would be a way to skip the check.
		return referer, true
	}

	return parsed.Scheme + "://" + parsed.Host, true
}
