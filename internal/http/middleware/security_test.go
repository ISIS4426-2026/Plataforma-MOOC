package middleware_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/http/middleware"
)

// discardLogger silences a middleware whose logging is not what the test is
// about.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// --- Rate limiting (issue #13) ---------------------------------------------

// countingLimiter is an in-memory fixed-window limiter with the same semantics
// as the Redis one, so the middleware can be exercised without a server.
type countingLimiter struct {
	mu     sync.Mutex
	counts map[string]int
	err    error
}

func newCountingLimiter() *countingLimiter {
	return &countingLimiter{counts: make(map[string]int)}
}

func (l *countingLimiter) Allow(_ context.Context, key string, limit int, window time.Duration) (domain.RateLimitResult, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.err != nil {
		return domain.RateLimitResult{}, l.err
	}

	l.counts[key]++
	count := l.counts[key]

	return domain.RateLimitResult{
		Allowed:    count <= limit,
		Limit:      limit,
		Remaining:  max(limit-count, 0),
		RetryAfter: window,
	}, nil
}

func okHandlerFor(hits *int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*hits++
		w.WriteHeader(http.StatusOK)
	})
}

func policy(limit int) middleware.RateLimitPolicy {
	return middleware.RateLimitPolicy{Name: "prueba", Limit: limit, Window: time.Minute}
}

// The acceptance criterion: exceeding the threshold must answer 429.
func TestRateLimitBlocksOnceTheThresholdIsExceeded(t *testing.T) {
	const limit = 3

	hits := 0
	handler := middleware.RateLimit(newCountingLimiter(), policy(limit), discardLogger())(okHandlerFor(&hits))

	// The first `limit` requests pass.
	for i := 1; i <= limit; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("request %d should have passed, got %d", i, w.Code)
		}
	}

	// The next one is refused.
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil))

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 once the limit is exceeded, got %d", w.Code)
	}
	if hits != limit {
		t.Errorf("the handler ran %d times, expected %d: a blocked request reached it", hits, limit)
	}

	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("expected a uniform JSON error: %v", err)
	}
	if body.Code != "rate_limit_exceeded" {
		t.Errorf("expected code 'rate_limit_exceeded', got %q", body.Code)
	}
}

// A rejected caller must be told when to come back.
func TestRateLimitAnnouncesWhenToRetry(t *testing.T) {
	handler := middleware.RateLimit(newCountingLimiter(), policy(1), discardLogger())(okHandlerFor(new(int)))

	for range 2 {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil))

		if w.Code == http.StatusTooManyRequests {
			retryAfter := w.Header().Get(middleware.HeaderRetryAfter)
			if retryAfter == "" {
				t.Fatal("a 429 must carry Retry-After")
			}
			if seconds, err := strconv.Atoi(retryAfter); err != nil || seconds <= 0 {
				t.Errorf("Retry-After must be a positive number of seconds, got %q", retryAfter)
			}
			return
		}
	}

	t.Fatal("expected the second request to be rejected")
}

// The quota is reported on every response, not only on rejections, so a client
// can slow down before being blocked.
func TestRateLimitReportsRemainingQuota(t *testing.T) {
	handler := middleware.RateLimit(newCountingLimiter(), policy(5), discardLogger())(okHandlerFor(new(int)))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil))

	if got := w.Header().Get(middleware.HeaderRateLimit); got != "5" {
		t.Errorf("expected RateLimit-Limit 5, got %q", got)
	}
	if got := w.Header().Get(middleware.HeaderRateLimitRemaining); got != "4" {
		t.Errorf("expected RateLimit-Remaining 4 after one request, got %q", got)
	}
}

// Quotas are per client address: one caller exhausting its share must not lock
// everybody else out.
func TestRateLimitIsPerClientAddress(t *testing.T) {
	handler := middleware.RateLimit(newCountingLimiter(), policy(1), discardLogger())(okHandlerFor(new(int)))

	primero := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	primero.RemoteAddr = "10.0.0.1:1234"
	for range 2 {
		handler.ServeHTTP(httptest.NewRecorder(), primero)
	}

	otro := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	otro.RemoteAddr = "10.0.0.2:1234"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, otro)

	if w.Code != http.StatusOK {
		t.Errorf("a different address must have its own quota, got %d", w.Code)
	}
}

// Forwarding headers are attacker-controlled: honouring them would hand out a
// fresh quota per request and defeat the limit.
func TestRateLimitIgnoresForwardingHeaders(t *testing.T) {
	handler := middleware.RateLimit(newCountingLimiter(), policy(1), discardLogger())(okHandlerFor(new(int)))

	send := func(forwarded string) int {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		r.RemoteAddr = "10.0.0.1:1234"
		r.Header.Set("X-Forwarded-For", forwarded)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w.Code
	}

	if code := send("1.1.1.1"); code != http.StatusOK {
		t.Fatalf("the first request should pass, got %d", code)
	}
	if code := send("2.2.2.2"); code != http.StatusTooManyRequests {
		t.Errorf("changing X-Forwarded-For must not reset the quota, got %d", code)
	}
}

// A limiter outage degrades protection instead of locking every user out.
func TestRateLimitFailsOpenWhenTheLimiterIsUnavailable(t *testing.T) {
	limiter := newCountingLimiter()
	limiter.err = errors.New("redis unavailable")

	var buf bytes.Buffer
	hits := 0
	handler := middleware.RateLimit(limiter, policy(1), newBufferedLogger(&buf))(okHandlerFor(&hits))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil))

	if w.Code != http.StatusOK {
		t.Errorf("expected the request to be allowed through, got %d", w.Code)
	}
	if hits != 1 {
		t.Error("expected the handler to run")
	}
	if !bytes.Contains(buf.Bytes(), []byte("rate limiter unavailable")) {
		t.Error("an outage that removes the throttle must be logged")
	}
}

// --- CSRF (issue #13) -------------------------------------------------------

func csrfHandler(t *testing.T, origins []string) http.Handler {
	t.Helper()
	return middleware.CSRF(middleware.CSRFConfig{AllowedOrigins: origins}, discardLogger())(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
}

// The acceptance criterion: a state-changing request without valid CSRF
// protection is rejected.
func TestCSRFRejectsAnUntrustedOrigin(t *testing.T) {
	handler := csrfHandler(t, []string{"https://app.plataforma-mooc.test"})

	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	r.Header.Set(middleware.HeaderOrigin, "https://sitio-del-atacante.test")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for an untrusted origin, got %d", w.Code)
	}

	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("expected a uniform JSON error: %v", err)
	}
	if body.Code != "csrf_origin_rejected" {
		t.Errorf("expected code 'csrf_origin_rejected', got %q", body.Code)
	}
}

func TestCSRFAllowsATrustedOrigin(t *testing.T) {
	const trusted = "https://app.plataforma-mooc.test"
	handler := csrfHandler(t, []string{trusted})

	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	r.Header.Set(middleware.HeaderOrigin, trusted)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected a trusted origin to pass, got %d", w.Code)
	}
}

// Non-browser clients send no Origin and cannot be the vehicle of a forgery, so
// rejecting them would break the API without stopping an attack.
func TestCSRFAllowsRequestsWithoutAnOrigin(t *testing.T) {
	handler := csrfHandler(t, []string{"https://app.plataforma-mooc.test"})

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil))

	if w.Code != http.StatusOK {
		t.Errorf("expected a request with no Origin to pass, got %d", w.Code)
	}
}

// Safe methods change nothing, so requiring an origin on them would break
// ordinary navigation.
func TestCSRFDoesNotCheckSafeMethods(t *testing.T) {
	handler := csrfHandler(t, []string{"https://app.plataforma-mooc.test"})

	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		r := httptest.NewRequest(method, "/api/v1/auth/sessions", nil)
		r.Header.Set(middleware.HeaderOrigin, "https://sitio-del-atacante.test")

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("%s should not be checked, got %d", method, w.Code)
		}
	}
}

// Falling back to Referer covers the browsers that omit Origin on same-site
// posts.
func TestCSRFFallsBackToReferer(t *testing.T) {
	const trusted = "https://app.plataforma-mooc.test"
	handler := csrfHandler(t, []string{trusted})

	cases := map[string]struct {
		referer string
		want    int
	}{
		"trusted referer":   {trusted + "/login", http.StatusOK},
		"untrusted referer": {"https://sitio-del-atacante.test/x", http.StatusForbidden},
		"malformed referer": {"no-es-una-url", http.StatusForbidden},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
			r.Header.Set(middleware.HeaderReferer, tc.referer)

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)

			if w.Code != tc.want {
				t.Errorf("expected %d, got %d", tc.want, w.Code)
			}
		})
	}
}

// The literal "null" origin is what a sandboxed iframe or a redirected form
// sends; it must never match an allowlist entry.
func TestCSRFTreatsNullOriginAsAbsent(t *testing.T) {
	handler := csrfHandler(t, []string{"null"})

	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	r.Header.Set(middleware.HeaderOrigin, "null")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	// It falls through to the no-origin case rather than matching the entry.
	if w.Code != http.StatusOK {
		t.Errorf("expected the null origin to be treated as absent, got %d", w.Code)
	}
}

// With no allowlist configured the check stays out of the way, which is the
// right default while no browser client exists.
func TestCSRFIsDisabledWithoutAnAllowlist(t *testing.T) {
	handler := csrfHandler(t, nil)

	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	r.Header.Set(middleware.HeaderOrigin, "https://sitio-del-atacante.test")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected the check to be disabled, got %d", w.Code)
	}
}
