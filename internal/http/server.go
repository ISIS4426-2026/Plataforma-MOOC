package http

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/admin"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/auth"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/config"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/course"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/http/handler"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/http/middleware"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/structure"
)

// Timeouts guard against slow or idle clients holding server resources. Without
// them a stalled connection is kept open indefinitely, which is a trivial way to
// exhaust a stateless instance that is meant to scale horizontally.
const (
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 120 * time.Second
)

type Server struct {
	cfg     *config.Config
	logger  *slog.Logger
	handler http.Handler
	http    *http.Server
}

// NewServer wires the routing table and the middleware chain shared by every
// endpoint.
//
// Dependencies are injected rather than constructed here so that the HTTP layer
// stays decoupled from persistence, as required by the architecture rules, and
// so tests can supply their own doubles.
// Deps are the collaborators the HTTP layer needs. Grouping them keeps the
// constructor signature stable as more modules register routes.
type Deps struct {
	DB               handler.Pinger
	Auth             *auth.Service
	Admin            *admin.Service
	Course           *course.Service
	Structure        *structure.Service
	Audit            domain.AuditRepository
	RateLimiter      domain.RateLimiter
	IdempotencyStore domain.IdempotencyStore
}

func NewServer(cfg *config.Config, deps Deps, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}

	mux := http.NewServeMux()
	authHandler := handler.NewAuthHandler(deps.Auth, logger)
	adminHandler := handler.NewAdminHandler(deps.Admin, logger)
	courseHandler := handler.NewCourseHandler(deps.Course, logger)
	structureHandler := handler.NewStructureHandler(deps.Structure, logger)
	auditHandler := handler.NewAuditHandler(deps.Audit, logger)

	// requireAuth guards the endpoints that act on behalf of a signed-in user.
	// It is applied per route rather than globally so the public endpoints stay
	// reachable without a session.
	requireAuth := middleware.RequireAuth(deps.Auth)

	// Administrative routes compose both: RequireAuth establishes who the caller
	// is, and RequireAdmin decides whether that identity may proceed. Order
	// matters, since the role check reads the user the first one publishes.
	requireAdmin := func(h http.HandlerFunc) http.Handler {
		return requireAuth(middleware.RequireAdmin(logger)(h))
	}

	// Course authoring (issue #17) is open to professors and administrators;
	// RequireRole is what already anticipates more than one accepted role.
	requireAuthor := func(h http.HandlerFunc) http.Handler {
		return requireAuth(middleware.RequireRole(logger, domain.RoleAdmin, domain.RoleProfessor)(h))
	}

	// Replay protection for the writes where running twice would be visible:
	// a second account, a second email, a second audit entry. It is opt-in, so
	// a client that sends no Idempotency-Key is unaffected.
	idempotent := middleware.Idempotency(deps.IdempotencyStore, cfg.IdempotencyTTL, logger)

	// Rate limits are applied per endpoint group rather than globally: the
	// thresholds that make sense for credential guessing would be absurd for
	// ordinary reads, and exhausting the login quota must not lock a user out
	// of password recovery.
	limitLogin := middleware.RateLimit(deps.RateLimiter, middleware.RateLimitPolicy{
		Name:   "login",
		Limit:  cfg.RateLimitLoginAttempts,
		Window: cfg.RateLimitLoginWindow,
	}, logger)
	limitRegister := middleware.RateLimit(deps.RateLimiter, middleware.RateLimitPolicy{
		Name:   "register",
		Limit:  cfg.RateLimitRegisterAttempts,
		Window: cfg.RateLimitRegisterWindow,
	}, logger)
	limitRecovery := middleware.RateLimit(deps.RateLimiter, middleware.RateLimitPolicy{
		Name:   "recovery",
		Limit:  cfg.RateLimitRecoveryAttempts,
		Window: cfg.RateLimitRecoveryWindow,
	}, logger)

	// Register API v1 routes
	mux.Handle("GET /api/v1/health", handler.NewHealthHandler(deps.DB))

	// The contract and its browsable rendering are served by the API itself, so
	// the documentation page and the endpoints share an origin and "Try it out"
	// works without a CORS policy.
	mux.HandleFunc("GET /api/docs", handler.NewDocsHandler())
	mux.HandleFunc("GET "+handler.SpecPath, handler.NewOpenAPISpecHandler())

	// Public authentication endpoints. These are the ones exposed to credential
	// guessing and mailbox flooding, so each carries a rate limit.
	mux.Handle("POST /api/v1/auth/register",
		limitRegister(idempotent(http.HandlerFunc(authHandler.Register))))
	// El token tiene 256 bits de entropia, asi que adivinarlo es inviable; el
	// limite existe para que golpear el endpoint con tokens basura no salga
	// gratis en consultas a la base de datos.
	mux.Handle("GET /api/v1/auth/verify",
		limitRecovery(http.HandlerFunc(authHandler.Verify)))
	mux.Handle("POST /api/v1/auth/verify/resend",
		limitRecovery(http.HandlerFunc(authHandler.ResendVerification)))
	mux.Handle("POST /api/v1/auth/login",
		limitLogin(http.HandlerFunc(authHandler.Login)))
	mux.Handle("POST /api/v1/auth/password/forgot",
		limitRecovery(idempotent(http.HandlerFunc(authHandler.ForgotPassword))))
	mux.Handle("POST /api/v1/auth/password/reset",
		limitRecovery(http.HandlerFunc(authHandler.ResetPassword)))

	// Logout authenticates by the token it is about to revoke, so it validates
	// the credential itself instead of going through requireAuth.
	mux.HandleFunc("POST /api/v1/auth/logout", authHandler.Logout)

	// Session management for the signed-in user
	mux.Handle("GET /api/v1/auth/sessions",
		requireAuth(http.HandlerFunc(authHandler.ListSessions)))
	mux.Handle("DELETE /api/v1/auth/sessions/{sessionID}",
		requireAuth(http.HandlerFunc(authHandler.RevokeSession)))

	// Administrative account management (issue #12). Every route is restricted
	// to administrators; a signed-in user with another role gets 403.
	mux.Handle("GET /api/v1/admin/users", requireAdmin(adminHandler.ListUsers))
	mux.Handle("GET /api/v1/admin/users/{userID}", requireAdmin(adminHandler.GetUser))

	// The administrative writes are both idempotent and conditional: a replay
	// returns the recorded answer, and a stale If-Match is refused. Idempotency
	// sits inside the role check so the scoped key is bound to a caller that has
	// already been authenticated.
	mux.Handle("PATCH /api/v1/admin/users/{userID}/role",
		requireAuth(middleware.RequireAdmin(logger)(idempotent(http.HandlerFunc(adminHandler.ChangeRole)))))
	mux.Handle("PATCH /api/v1/admin/users/{userID}/status",
		requireAuth(middleware.RequireAdmin(logger)(idempotent(http.HandlerFunc(adminHandler.ChangeStatus)))))

	// Course authoring (issue #17). Listing and reading a single course are
	// public, matching the catalog; creating and editing a draft require a
	// professor or administrator and carry Idempotency-Key like the other
	// mutating admin endpoints.
	mux.HandleFunc("GET /api/v1/courses", courseHandler.List)
	mux.HandleFunc("GET /api/v1/courses/{courseID}", courseHandler.Get)
	mux.Handle("POST /api/v1/courses",
		requireAuthor(idempotent(http.HandlerFunc(courseHandler.Create)).ServeHTTP))
	mux.Handle("PUT /api/v1/courses/{courseID}",
		requireAuthor(idempotent(http.HandlerFunc(courseHandler.Update)).ServeHTTP))

	// Publish and unpublish (issue #20). Publish validates the whole
	// structure, so it is idempotent-key protected like the other writes;
	// unpublish is the MVP's way back to editable (section 5.1) and carries
	// the same protection since running it twice must not be visible.
	mux.Handle("POST /api/v1/courses/{courseID}/publish",
		requireAuthor(idempotent(http.HandlerFunc(courseHandler.Publish)).ServeHTTP))
	mux.Handle("POST /api/v1/courses/{courseID}/unpublish",
		requireAuthor(idempotent(http.HandlerFunc(courseHandler.Unpublish)).ServeHTTP))

	// Module, Unit and Resource CRUD (issue #19), same public-read /
	// professor-or-admin-write split as course authoring. Listing lives
	// under its parent's path (a module's units, a unit's resources);
	// writes to an existing item address it directly by id, since by then
	// its place in the hierarchy is already fixed.
	mux.HandleFunc("GET /api/v1/courses/{courseID}/modules", structureHandler.ListModules)
	mux.Handle("POST /api/v1/courses/{courseID}/modules",
		requireAuthor(idempotent(http.HandlerFunc(structureHandler.CreateModule)).ServeHTTP))
	mux.Handle("PUT /api/v1/modules/{moduleID}",
		requireAuthor(idempotent(http.HandlerFunc(structureHandler.UpdateModule)).ServeHTTP))
	mux.Handle("DELETE /api/v1/modules/{moduleID}", requireAuthor(structureHandler.DeleteModule))

	mux.HandleFunc("GET /api/v1/modules/{moduleID}/units", structureHandler.ListUnits)
	mux.Handle("POST /api/v1/modules/{moduleID}/units",
		requireAuthor(idempotent(http.HandlerFunc(structureHandler.CreateUnit)).ServeHTTP))
	mux.Handle("PUT /api/v1/units/{unitID}",
		requireAuthor(idempotent(http.HandlerFunc(structureHandler.UpdateUnit)).ServeHTTP))
	mux.Handle("DELETE /api/v1/units/{unitID}", requireAuthor(structureHandler.DeleteUnit))

	mux.HandleFunc("GET /api/v1/units/{unitID}/resources", structureHandler.ListResources)
	mux.Handle("POST /api/v1/units/{unitID}/resources",
		requireAuthor(idempotent(http.HandlerFunc(structureHandler.CreateResource)).ServeHTTP))
	mux.Handle("PUT /api/v1/resources/{resourceID}",
		requireAuthor(idempotent(http.HandlerFunc(structureHandler.UpdateResource)).ServeHTTP))
	mux.Handle("DELETE /api/v1/resources/{resourceID}", requireAuthor(structureHandler.DeleteResource))

	// Read side of the audit trail (issue #18): administrators only, no
	// Idempotency-Key since it is a GET with no side effect to replay.
	mux.Handle("GET /api/v1/admin/audit-logs", requireAdmin(auditHandler.List))

	// Ordering matters. RequestID runs first so the correlation id is available
	// to everything below it. RequestLogger comes next so every response is
	// recorded, including the ones CSRF rejects. Recoverer sits below the logger
	// so a recovered panic is still counted, and CSRF is innermost so a forged
	// request is refused before it reaches a handler.
	root := middleware.Chain(mux,
		middleware.RequestID(),
		middleware.RequestLogger(logger),
		middleware.Recoverer(logger),
		middleware.CSRF(middleware.CSRFConfig{AllowedOrigins: cfg.CSRFAllowedOrigins}, logger),
	)

	return &Server{
		cfg:     cfg,
		logger:  logger,
		handler: root,
		http: &http.Server{
			Addr:              fmt.Sprintf(":%s", cfg.Port),
			Handler:           root,
			ReadHeaderTimeout: readHeaderTimeout,
			ReadTimeout:       readTimeout,
			WriteTimeout:      writeTimeout,
			IdleTimeout:       idleTimeout,
		},
	}
}

// Start blocks serving requests. It returns nil on a graceful shutdown, since
// http.ErrServerClosed is the expected outcome of calling Shutdown and not a
// failure the caller should report.
func (s *Server) Start() error {
	s.logger.Info("api server listening",
		slog.String("addr", s.http.Addr),
		slog.String("env", s.cfg.Environment),
	)

	if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Shutdown stops accepting connections and waits for in-flight requests to
// finish, up to the deadline carried by ctx.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}

// Handler exposes the fully decorated handler so tests can exercise the real
// middleware chain instead of a bare mux.
func (s *Server) Handler() http.Handler {
	return s.handler
}
