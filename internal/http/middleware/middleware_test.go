package middleware_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/http/middleware"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
}

// newBufferedLogger returns a logger writing JSON records into buf so tests can
// assert on the fields actually emitted.
func newBufferedLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestChainAppliesMiddlewareOutermostFirst(t *testing.T) {
	var order []string

	record := func(name string) middleware.Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name)
				next.ServeHTTP(w, r)
			})
		}
	}

	handler := middleware.Chain(okHandler(), record("first"), record("second"), record("third"))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	got := strings.Join(order, ",")
	if got != "first,second,third" {
		t.Errorf("expected the first argument to run outermost, got order %q", got)
	}
}

func TestRequestIDGeneratesAndEchoesIdentifier(t *testing.T) {
	var seen string
	handler := middleware.RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = middleware.RequestIDFrom(r.Context())
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	if seen == "" {
		t.Fatal("expected a request id in the context")
	}
	if header := w.Header().Get(middleware.HeaderRequestID); header != seen {
		t.Errorf("expected response header %q to equal context id %q", header, seen)
	}
}

// Reusing an inbound identifier is what lets a single trace span the frontend,
// the API and the workers.
func TestRequestIDReusesValidInboundIdentifier(t *testing.T) {
	const inbound = "trace-abc-123"

	var seen string
	handler := middleware.RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = middleware.RequestIDFrom(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(middleware.HeaderRequestID, inbound)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if seen != inbound {
		t.Errorf("expected inbound id %q to be reused, got %q", inbound, seen)
	}
}

// A client-supplied identifier is reflected into logs and headers, so anything
// that is not a plausible id must be replaced rather than trusted.
func TestRequestIDRejectsMaliciousInboundIdentifier(t *testing.T) {
	malicious := []string{
		"abc\r\nSet-Cookie: admin=true",
		"id with spaces",
		strings.Repeat("a", 129),
	}

	for _, inbound := range malicious {
		var seen string
		handler := middleware.RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seen = middleware.RequestIDFrom(r.Context())
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set(middleware.HeaderRequestID, inbound)
		handler.ServeHTTP(httptest.NewRecorder(), req)

		if seen == inbound {
			t.Errorf("malicious id %q was accepted verbatim", inbound)
		}
		if seen == "" {
			t.Errorf("expected a generated replacement id for %q", inbound)
		}
	}
}

func TestRecovererConvertsPanicIntoUniformError(t *testing.T) {
	var buf bytes.Buffer
	panicking := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})

	handler := middleware.Recoverer(newBufferedLogger(&buf))(panicking)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", res.StatusCode)
	}

	var body struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("expected a uniform JSON error body: %v", err)
	}
	if body.Code != "internal_error" {
		t.Errorf("expected code 'internal_error', got %q", body.Code)
	}
	// The stack trace belongs in the log, never in the response.
	if strings.Contains(body.Message, "boom") {
		t.Error("panic detail leaked into the response body")
	}
	if !strings.Contains(buf.String(), "boom") {
		t.Error("expected the panic value to be logged")
	}
}

// Ordering guarantee: Recoverer sits inside RequestLogger, so a panicking
// request still produces an access log entry carrying status 500.
func TestRecoveredPanicIsStillLoggedByRequestLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := newBufferedLogger(&buf)

	panicking := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})

	handler := middleware.Chain(panicking,
		middleware.RequestID(),
		middleware.RequestLogger(logger),
		middleware.Recoverer(logger),
	)
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))

	var sawAccessLogWith500 bool
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue
		}
		if status, ok := record["status"].(float64); ok && int(status) == http.StatusInternalServerError {
			sawAccessLogWith500 = true
		}
	}

	if !sawAccessLogWith500 {
		t.Error("expected the access log to record the recovered panic as status 500")
	}
}

// Verification and password-recovery links carry single-use tokens in the query
// string. Logging the raw query would turn the access log into a credential
// store, so only the path may be recorded.
func TestRequestLoggerNeverRecordsTheQueryString(t *testing.T) {
	const secret = "single-use-token-value"

	var buf bytes.Buffer
	handler := middleware.Chain(okHandler(),
		middleware.RequestID(),
		middleware.RequestLogger(newBufferedLogger(&buf)),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/verify?token="+secret, nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	logged := buf.String()
	if strings.Contains(logged, secret) {
		t.Error("the access log leaked a token from the query string")
	}
	if !strings.Contains(logged, "/api/v1/auth/verify") {
		t.Errorf("expected the path to be logged, got %q", logged)
	}
}

func TestRequestLoggerRecordsStatusAndCorrelationID(t *testing.T) {
	var buf bytes.Buffer
	handler := middleware.Chain(okHandler(),
		middleware.RequestID(),
		middleware.RequestLogger(newBufferedLogger(&buf)),
	)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))

	var record map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &record); err != nil {
		t.Fatalf("expected a JSON log record: %v", err)
	}

	if status, ok := record["status"].(float64); !ok || int(status) != http.StatusOK {
		t.Errorf("expected status 200 in the log record, got %v", record["status"])
	}
	if record["request_id"] != w.Header().Get(middleware.HeaderRequestID) {
		t.Errorf("log record id %v does not match the response header", record["request_id"])
	}
	if record["method"] != http.MethodGet {
		t.Errorf("expected method GET, got %v", record["method"])
	}
}
