package worker

import (
	"fmt"
	"log/slog"
	"os"
)

// SlogAsynqAdapter adapts *slog.Logger to satisfy the asynq.Logger interface.
type SlogAsynqAdapter struct {
	logger *slog.Logger
}

// NewSlogAsynqAdapter creates a new SlogAsynqAdapter.
func NewSlogAsynqAdapter(logger *slog.Logger) *SlogAsynqAdapter {
	if logger == nil {
		logger = slog.Default()
	}
	return &SlogAsynqAdapter{logger: logger}
}

func (a *SlogAsynqAdapter) Debug(args ...interface{}) {
	a.logger.Debug(fmt.Sprint(args...))
}

func (a *SlogAsynqAdapter) Info(args ...interface{}) {
	a.logger.Info(fmt.Sprint(args...))
}

func (a *SlogAsynqAdapter) Warn(args ...interface{}) {
	a.logger.Warn(fmt.Sprint(args...))
}

func (a *SlogAsynqAdapter) Error(args ...interface{}) {
	a.logger.Error(fmt.Sprint(args...))
}

func (a *SlogAsynqAdapter) Fatal(args ...interface{}) {
	a.logger.Error(fmt.Sprint(args...))
	os.Exit(1)
}
