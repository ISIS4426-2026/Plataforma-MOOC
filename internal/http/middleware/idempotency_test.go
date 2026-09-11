package middleware_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/http/middleware"
)

// memoryIdempotencyStore mirrors the Redis implementation closely enough to
// exercise the middleware: the claim is atomic, and a key holds either a claim
// in flight or a finished response.
type memoryIdempotencyStore struct {
	mu      sync.Mutex
	records map[string]*record
	err     error
}

type record struct {
	inProgress bool
	response   *domain.RecordedResponse
}

func newMemoryIdempotencyStore() *memoryIdempotencyStore {
	return &memoryIdempotencyStore{records: make(map[string]*record)}
}

func (s *memoryIdempotencyStore) Reserve(_ context.Context, key, fingerprint string, _ time.Duration) (*domain.RecordedResponse, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.err != nil {
		return nil, false, s.err
	}

	existing, taken := s.records[key]
	if !taken {
		s.records[key] = &record{inProgress: true}
		return nil, false, nil
	}
	if existing.inProgress {
		return nil, true, nil
	}
	return existing.response, false, nil
}

func (s *memoryIdempotencyStore) Store(_ context.Context, key string, response *domain.RecordedResponse, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.records[key] = &record{response: response}
	return nil
}

func (s *memoryIdempotencyStore) Release(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.records, key)
	return nil
}

// countingCreateHandler stands in for an endpoint with a visible side effect.
type countingCreateHandler struct {
	mu      sync.Mutex
	effects int
	status  int
}

func (h *countingCreateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	h.effects++
	current := h.effects
	h.mu.Unlock()

	status := h.status
	if status == 0 {
		status = http.StatusCreated
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"created":` + strconv.Itoa(current) + `}`))
}

func idempotentHandler(t *testing.T, store domain.IdempotencyStore, next http.Handler) http.Handler {
	t.Helper()
	return middleware.Idempotency(store, time.Hour, discardLogger())(next)
}

func postWithKey(key, body string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if key != "" {
		r.Header.Set(middleware.HeaderIdempotencyKey, key)
	}
	return r
}

// The acceptance criterion: the same POST with the same key, sent twice,
// produces a single persisted effect.
func TestSameKeyTwiceProducesOneEffect(t *testing.T) {
	backend := &countingCreateHandler{}
	handler := idempotentHandler(t, newMemoryIdempotencyStore(), backend)

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, postWithKey("abc123", `{"email":"a@b.test"}`))

	second := httptest.NewRecorder()
	handler.ServeHTTP(second, postWithKey("abc123", `{"email":"a@b.test"}`))

	if backend.effects != 1 {
		t.Errorf("the operation ran %d times, expected exactly 1", backend.effects)
	}
	if first.Code != second.Code {
		t.Errorf("the replay answered %d, the original %d", second.Code, first.Code)
	}
	if first.Body.String() != second.Body.String() {
		t.Errorf("the replay returned a different body:\n  first : %s\n  second: %s",
			first.Body.String(), second.Body.String())
	}
	if second.Header().Get("Idempotent-Replay") != "true" {
		t.Error("a replayed response must say so, otherwise a client cannot tell it apart")
	}
	if first.Header().Get("Idempotent-Replay") != "" {
		t.Error("the original response must not be marked as a replay")
	}
}

// Without a key the middleware stays out of the way, so replay protection is
// something a client opts into.
func TestWithoutAKeyEveryRequestRuns(t *testing.T) {
	backend := &countingCreateHandler{}
	handler := idempotentHandler(t, newMemoryIdempotencyStore(), backend)

	for range 3 {
		handler.ServeHTTP(httptest.NewRecorder(), postWithKey("", `{"email":"a@b.test"}`))
	}

	if backend.effects != 3 {
		t.Errorf("expected 3 executions without a key, got %d", backend.effects)
	}
}

// Different keys are different operations.
func TestDifferentKeysRunSeparately(t *testing.T) {
	backend := &countingCreateHandler{}
	handler := idempotentHandler(t, newMemoryIdempotencyStore(), backend)

	handler.ServeHTTP(httptest.NewRecorder(), postWithKey("key-1", `{"email":"a@b.test"}`))
	handler.ServeHTTP(httptest.NewRecorder(), postWithKey("key-2", `{"email":"a@b.test"}`))

	if backend.effects != 2 {
		t.Errorf("expected 2 executions for 2 keys, got %d", backend.effects)
	}
}

// Reusing a key with a different payload is a client defect, and answering with
// the first response would hide it.
func TestReusingAKeyWithADifferentBodyIsRejected(t *testing.T) {
	backend := &countingCreateHandler{}
	handler := idempotentHandler(t, newMemoryIdempotencyStore(), backend)

	handler.ServeHTTP(httptest.NewRecorder(), postWithKey("abc123", `{"email":"a@b.test"}`))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, postWithKey("abc123", `{"email":"OTRO@b.test"}`))

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
	if backend.effects != 1 {
		t.Errorf("the operation ran %d times, expected 1", backend.effects)
	}
}

// A retry arriving while the original is still running has no answer to replay
// yet, and running the operation alongside it is what the key exists to prevent.
func TestRetryWhileTheOriginalIsRunningIsRefused(t *testing.T) {
	store := newMemoryIdempotencyStore()

	release := make(chan struct{})
	started := make(chan struct{})
	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusCreated)
	})

	handler := idempotentHandler(t, store, backend)

	go handler.ServeHTTP(httptest.NewRecorder(), postWithKey("abc123", `{}`))
	<-started

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, postWithKey("abc123", `{}`))

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409 while the original is in flight, got %d", w.Code)
	}

	close(release)
}

// A server failure must stay retryable: freezing a 500 for the whole window
// would turn a transient fault into a permanent one for that key.
func TestAServerErrorIsNotRecorded(t *testing.T) {
	backend := &countingCreateHandler{status: http.StatusInternalServerError}
	handler := idempotentHandler(t, newMemoryIdempotencyStore(), backend)

	handler.ServeHTTP(httptest.NewRecorder(), postWithKey("abc123", `{}`))

	backend.status = http.StatusCreated
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, postWithKey("abc123", `{}`))

	if w.Code != http.StatusCreated {
		t.Errorf("expected the retry to run again and succeed, got %d", w.Code)
	}
	if backend.effects != 2 {
		t.Errorf("expected the failed request to be retryable, ran %d times", backend.effects)
	}
}

// A client error is a definitive answer and is worth replaying: retrying a
// malformed request would just fail the same way.
func TestAClientErrorIsRecorded(t *testing.T) {
	backend := &countingCreateHandler{status: http.StatusBadRequest}
	handler := idempotentHandler(t, newMemoryIdempotencyStore(), backend)

	handler.ServeHTTP(httptest.NewRecorder(), postWithKey("abc123", `{}`))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, postWithKey("abc123", `{}`))

	if backend.effects != 1 {
		t.Errorf("expected the 400 to be replayed, the operation ran %d times", backend.effects)
	}
	if w.Header().Get("Idempotent-Replay") != "true" {
		t.Error("expected the second answer to be marked as a replay")
	}
}

// Unlike the rate limiter, this fails closed: letting the request through would
// run an operation the client asked to be run at most once.
func TestAStoreOutageRefusesTheRequest(t *testing.T) {
	store := newMemoryIdempotencyStore()
	store.err = errors.New("redis unavailable")

	var buf bytes.Buffer
	backend := &countingCreateHandler{}
	handler := middleware.Idempotency(store, time.Hour, newBufferedLogger(&buf))(backend)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, postWithKey("abc123", `{}`))

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
	if backend.effects != 0 {
		t.Error("the operation ran despite the guarantee being unavailable")
	}
	if !bytes.Contains(buf.Bytes(), []byte("idempotency store unavailable")) {
		t.Error("the outage must be logged")
	}
}

// An over-long key would end up in a Redis key and in logs.
func TestAnOverlongKeyIsRejected(t *testing.T) {
	backend := &countingCreateHandler{}
	handler := idempotentHandler(t, newMemoryIdempotencyStore(), backend)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, postWithKey(strings.Repeat("k", 256), `{}`))

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	if backend.effects != 0 {
		t.Error("the operation ran despite the key being rejected")
	}
}

// The same key on a different endpoint is a different operation; otherwise one
// call could be answered with another's response.
func TestTheKeyIsScopedToTheEndpoint(t *testing.T) {
	backend := &countingCreateHandler{}
	handler := idempotentHandler(t, newMemoryIdempotencyStore(), backend)

	first := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{}`))
	first.Header.Set(middleware.HeaderIdempotencyKey, "abc123")
	handler.ServeHTTP(httptest.NewRecorder(), first)

	second := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password/forgot", strings.NewReader(`{}`))
	second.Header.Set(middleware.HeaderIdempotencyKey, "abc123")
	handler.ServeHTTP(httptest.NewRecorder(), second)

	if backend.effects != 2 {
		t.Errorf("a key must not carry across endpoints, the operation ran %d times", backend.effects)
	}
}

// The handler must still be able to read the body the middleware buffered to
// compute the fingerprint.
func TestTheBodyStillReachesTheHandler(t *testing.T) {
	const payload = `{"email":"a@b.test"}`

	var seen string
	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(r.Body)
		seen = buf.String()
		w.WriteHeader(http.StatusCreated)
	})

	handler := idempotentHandler(t, newMemoryIdempotencyStore(), backend)
	handler.ServeHTTP(httptest.NewRecorder(), postWithKey("abc123", payload))

	if seen != payload {
		t.Errorf("the handler received %q, expected %q", seen, payload)
	}
}
