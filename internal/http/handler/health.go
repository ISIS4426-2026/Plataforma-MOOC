package handler

import (
	"encoding/json"
	"net/http"
	"time"
)

type HealthResponse struct {
	Status    string            `json:"status"`
	Timestamp string            `json:"timestamp"`
	Services  map[string]string `json:"services"`
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	resp := HealthResponse{
		Status:    "pass",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Services: map[string]string{
			"database": "up",
			"redis":    "up",
		},
	}

	_ = json.NewEncoder(w).Encode(resp)
}
