package worker

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/config"
)

// credencialesFalsas deja a google.DefaultClient construir un cliente sin red: el token se pide al usarlo.
func credencialesFalsas(t *testing.T) {
	t.Helper()
	ruta := filepath.Join(t.TempDir(), "adc.json")
	json := `{"type":"authorized_user","client_id":"id","client_secret":"secreto","refresh_token":"token"}`
	if err := os.WriteFile(ruta, []byte(json), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", ruta)
}

func TestNewMediaQueuePubSubConTopicValido(t *testing.T) {
	credencialesFalsas(t)
	cfg := &config.Config{QueueBackend: "pubsub", PubSubTopic: "projects/p/topics/t"}

	cola, cerrar, err := NewMediaQueue(context.Background(), cfg, slog.Default())
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if cola == nil || cerrar == nil {
		t.Fatal("se esperaba una cola y su funcion de cierre")
	}
	cerrar()
}

func TestNewMediaQueuePubSubRechazaUnTopicIncompleto(t *testing.T) {
	cfg := &config.Config{QueueBackend: "pubsub", PubSubTopic: "solo-el-nombre"}
	if _, _, err := NewMediaQueue(context.Background(), cfg, slog.Default()); err == nil {
		t.Fatal("un topic sin proyecto debia rechazarse")
	}
}

func TestNewMediaQueueAsynqPorOmision(t *testing.T) {
	cfg := &config.Config{RedisURL: "localhost:6379"} // asynq no conecta hasta que se encola
	cola, cerrar, err := NewMediaQueue(context.Background(), cfg, slog.Default())
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if cola == nil {
		t.Fatal("se esperaba una cola")
	}
	cerrar()
}

func TestNewMediaQueueAsynqRechazaUnaURLRota(t *testing.T) {
	cfg := &config.Config{RedisURL: "redis://%zz"}
	if _, _, err := NewMediaQueue(context.Background(), cfg, slog.Default()); err == nil {
		t.Fatal("una URL de Redis invalida debia rechazarse")
	}
}
