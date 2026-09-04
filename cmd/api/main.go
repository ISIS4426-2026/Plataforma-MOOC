package main

import (
	"log"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/config"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/http"
)

func main() {
	cfg := config.Load()

	log.Printf("Starting Plataforma MOOC API server on port %s [Env: %s]\n", cfg.Port, cfg.Environment)
	server := http.NewServer(cfg)

	if err := server.Start(); err != nil {
		log.Fatalf("API server encountered fatal error: %v\n", err)
	}
}
