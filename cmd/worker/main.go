package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/config"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/worker"
)

func main() {
	enqueueFlag := flag.Bool("enqueue-test", false, "Enqueue a test ping job to Redis and exit")
	msgFlag := flag.String("message", "Test ping job from CLI", "Custom message for test ping job")
	flag.BoolVar(enqueueFlag, "enqueue", false, "Alias for -enqueue-test")
	flag.Parse()

	cfg := config.Load()

	if *enqueueFlag {
		if err := enqueueTestJob(cfg, *msgFlag); err != nil {
			log.Fatalf("Failed to enqueue test job: %v\n", err)
		}
		return
	}

	log.Printf("Starting Plataforma MOOC Background Worker [Env: %s]\n", cfg.Environment)
	engine, err := worker.NewWorkerEngine(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize worker engine: %v\n", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := engine.Start(ctx); err != nil {
		log.Fatalf("Worker engine encountered fatal error: %v\n", err)
	}
}

func enqueueTestJob(cfg *config.Config, msg string) error {
	client, err := worker.NewClient(cfg.RedisURL)
	if err != nil {
		return fmt.Errorf("failed to create worker client: %w", err)
	}
	defer client.Close()

	info, err := client.EnqueueTestPing(context.Background(), msg)
	if err != nil {
		return fmt.Errorf("failed to enqueue test ping job: %w", err)
	}

	log.Printf("[CLI] Successfully enqueued test job [ID: %s, Type: %s, Queue: %s]\n", info.ID, info.Type, info.Queue)
	return nil
}
