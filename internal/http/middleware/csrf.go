package middleware

import (
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/http/handler"
)

// CSRF protection for a token-authenticated API.
//
// # Why this is an origin check and not a synchroniser token
//
// Cross-site request forgery works because the browser attaches the victim's
// credentials to a request the attacker triggered. That requires credentials
// the browser sends automatically, which means cookies. This API authenticates
// with `Authorization: Bearer <token>`, a header no browser ever attaches on
// its own, so a forged cross-site request arrives unauthenticated and is
// rejected by the auth middleware before it can change anything.
//
// A double-submit cookie or synchroniser token would therefore protect nothing
// that is not already protected, while adding a token to mint, store, rotate
// and hand to the frontend. What does add value, and is what OWASP recommends
// for APIs, is verifying where the request claims to come from: browsers set
// Origin on every state-changing cross-site request and scripts cannot forge
// it. That is what this middleware enforces.
//
// If the frontend ever moves to cookie-based sessions, this check stops being
// sufficient on its own and a synchroniser token has to be added alongside it.
const (
	HeaderOrigin  = "Origin"
	HeaderReferer = "Referer"
)

// CSRFConfig lists the origins allowed to submit state-changing requests.
type CSRFConfig struct {
	// AllowedOrigins are matched exactly, scheme and port included. An empty
	// list disables the check, which is only appropriate while no browser
	// client exists yet.
	AllowedOrigins []string
}

// CSRF rejects state-changing requests that declare an untrusted origin.
//
// Safe methods are not checked: by definition they change nothing, and
// requiring an origin on them would break ordinary navigation and link
// previews.
//
// A request with neither Origin nor Referer is allowed through. Those headers
// are absent only for non-browser clients such as curl, Postman or another
// service, and those cannot be the vehicle of a forgery: CSRF needs a browser
// that holds the victim's credentials. Rejecting them would break the API for
// every legitimate non-browser caller while stopping no attack.
func CSRF(cfg CSRFConfig, logger *slog.Logger) Middleware {
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(cfg.AllowedOrigins) == 0 || isSafeMethod(r.Method) {
				next.ServeHTTP(w, r)
				return
			}

			declared, present := declaredOrigin(r)
			if !present {
				next.ServeHTTP(w, r)
				return
			}

			if !slices.Contains(cfg.AllowedOrigins, declared) {
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
