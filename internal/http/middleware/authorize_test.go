package middleware_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/http/handler"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/http/middleware"
)

// requestAs builds a request whose context already carries an authenticated
// user, the way RequireAuth would leave it.
func requestAs(role domain.Role) *http.Request {
	user := &domain.User{
		ID:     "11111111-1111-1111-1111-111111111111",
		Email:  "quien-sea@example.test",
		Role:   role,
		Status: domain.UserStatusActive,
	}
	session := &domain.Session{ID: "22222222-2222-2222-2222-222222222222", UserID: user.ID}

	r := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/x/status", nil)
	return r.WithContext(handler.WithAuthenticated(r.Context(), user, session))
}

func adminOnlyHandler(hits *int) http.Handler {
	return middleware.RequireAdmin(discardLogger())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*hits++
		w.WriteHeader(http.StatusOK)
	}))
}

// The acceptance criterion of issue #12: a non-administrator attempting these
// operations receives 403.
func TestRequireAdminRejectsOtherRoles(t *testing.T) {
	for _, role := range []domain.Role{domain.RoleStudent, domain.RoleProfessor} {
		t.Run(string(role), func(t *testing.T) {
			hits := 0
			w := httptest.NewRecorder()

			adminOnlyHandler(&hits).ServeHTTP(w, requestAs(role))

			if w.Code != http.StatusForbidden {
				t.Errorf("expected 403 for role %q, got %d", role, w.Code)
			}
			if hits != 0 {
				t.Error("the handler ran despite the role being rejected")
			}

			var body struct {
				Code string `json:"code"`
			}
			if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
				t.Fatalf("expected a uniform JSON error: %v", err)
			}
			if body.Code != "forbidden" {
				t.Errorf("expected code 'forbidden', got %q", body.Code)
			}
		})
	}
}

func TestRequireAdminAllowsAdministrators(t *testing.T) {
	hits := 0
	w := httptest.NewRecorder()

	adminOnlyHandler(&hits).ServeHTTP(w, requestAs(domain.RoleAdmin))

	if w.Code != http.StatusOK {
		t.Errorf("expected an administrator to pass, got %d", w.Code)
	}
	if hits != 1 {
		t.Error("expected the handler to run")
	}
}

// A refusal is worth reviewing later, so it must leave a trace.
func TestRequireAdminLogsTheRefusal(t *testing.T) {
	var buf bytes.Buffer
	handler := middleware.RequireAdmin(newBufferedLogger(&buf))(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	handler.ServeHTTP(httptest.NewRecorder(), requestAs(domain.RoleStudent))

	logged := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("role not permitted")) {
		t.Errorf("expected the refusal to be logged, got %q", logged)
	}
	if !bytes.Contains(buf.Bytes(), []byte(string(domain.RoleStudent))) {
		t.Error("expected the log to record which role was refused")
	}
}

// Without an authenticated user the answer is 401, not 403: 403 must keep
// meaning "you are known and not allowed" rather than "you are not known".
func TestRequireAdminAnswers401WhenNoUserIsPresent(t *testing.T) {
	var buf bytes.Buffer
	hits := 0
	handler := middleware.RequireAdmin(newBufferedLogger(&buf))(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++ }))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/x/status", nil))

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
	if hits != 0 {
		t.Error("the handler ran on an unauthenticated request")
	}
	// Reaching this branch means the chain was assembled wrong, which is a
	// defect worth surfacing rather than silently answering 401.
	if !bytes.Contains(buf.Bytes(), []byte("unauthenticated request")) {
		t.Error("expected the misconfiguration to be logged at error level")
	}
}

// RequireRole accepts several roles, for the endpoints that professors and
// administrators share.
func TestRequireRoleAcceptsAnyOfTheGivenRoles(t *testing.T) {
	handler := middleware.RequireRole(discardLogger(), domain.RoleAdmin, domain.RoleProfessor)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

	for role, want := range map[domain.Role]int{
		domain.RoleAdmin:     http.StatusOK,
		domain.RoleProfessor: http.StatusOK,
		domain.RoleStudent:   http.StatusForbidden,
	} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, requestAs(role))

		if w.Code != want {
			t.Errorf("role %q: expected %d, got %d", role, want, w.Code)
		}
	}
}
