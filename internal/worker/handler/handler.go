package handler

import (
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/worker/task"
	"github.com/hibiken/asynq"
)

// RegisterRoutes registers all task handlers onto the provided ServeMux.
func RegisterRoutes(mux *asynq.ServeMux) {
	mux.HandleFunc(task.TypeTestPing, HandleTestPingTask)
	mux.HandleFunc(task.TypeMediaProcess, HandleMediaProcessTask)
}
