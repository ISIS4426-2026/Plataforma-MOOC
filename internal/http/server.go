package http

import (
	"fmt"
	"net/http"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/config"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/http/handler"
)

type Server struct {
	cfg    *config.Config
	router *http.ServeMux
}

func NewServer(cfg *config.Config) *Server {
	mux := http.NewServeMux()

	// Register API v1 routes
	mux.HandleFunc("GET /api/v1/health", handler.HealthHandler)

	return &Server{
		cfg:    cfg,
		router: mux,
	}
}

func (s *Server) Start() error {
	addr := fmt.Sprintf(":%s", s.cfg.Port)
	return http.ListenAndServe(addr, s.router)
}

func (s *Server) Handler() http.Handler {
	return s.router
}
