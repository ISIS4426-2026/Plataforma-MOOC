package http

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/auth"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/config"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/http/handler"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/http/middleware"
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
	DB   handler.Pinger
	Auth *auth.Service
}

func NewServer(cfg *config.Config, deps Deps, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}

	mux := http.NewServeMux()
	authHandler := handler.NewAuthHandler(deps.Auth, logger)

	// requireAuth guards the endpoints that act on behalf of a signed-in user.
	// It is applied per route rather than globally so the public endpoints stay
	// reachable without a session.
	requireAuth := middleware.RequireAuth(deps.Auth)

	// Register API v1 routes
	mux.Handle("GET /api/v1/health", handler.NewHealthHandler(deps.DB))

	// The contract and its browsable rendering are served by the API itself, so
	// the documentation page and the endpoints share an origin and "Try it out"
	// works without a CORS policy.
	mux.HandleFunc("GET /api/docs", handler.NewDocsHandler())
	mux.HandleFunc("GET "+handler.SpecPath, handler.NewOpenAPISpecHandler())

	// Public authentication endpoints
	mux.HandleFunc("POST /api/v1/auth/register", authHandler.Register)
	mux.HandleFunc("GET /api/v1/auth/verify", authHandler.Verify)
	mux.HandleFunc("POST /api/v1/auth/verify/resend", authHandler.ResendVerification)
	mux.HandleFunc("POST /api/v1/auth/login", authHandler.Login)
	mux.HandleFunc("POST /api/v1/auth/password/forgot", authHandler.ForgotPassword)
	mux.HandleFunc("POST /api/v1/auth/password/reset", authHandler.ResetPassword)

	// Logout authenticates by the token it is about to revoke, so it validates
	// the credential itself instead of going through requireAuth.
	mux.HandleFunc("POST /api/v1/auth/logout", authHandler.Logout)

	// Session management for the signed-in user
	mux.Handle("GET /api/v1/auth/sessions",
		requireAuth(http.HandlerFunc(authHandler.ListSessions)))
	mux.Handle("DELETE /api/v1/auth/sessions/{sessionID}",
		requireAuth(http.HandlerFunc(authHandler.RevokeSession)))

	// Ordering matters: RequestID runs first so the correlation id is available
	// to everything below it, and Recoverer sits closest to the handlers so a
	// recovered panic is still counted by the access log.
	root := middleware.Chain(mux,
		middleware.RequestID(),
		middleware.RequestLogger(logger),
		middleware.Recoverer(logger),
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
