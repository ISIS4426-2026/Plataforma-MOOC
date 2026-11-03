package worker

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/worker/task"
)

type procesadorNulo struct{}

func (procesadorNulo) ProcessPayload(context.Context, task.MediaProcessPayload) (bool, error) {
	return false, nil
}

func puertoLibre(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	return strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
}

func TestServePushAtiendeSaludYSeCierra(t *testing.T) {
	puerto := puertoLibre(t)
	ctx, cancel := context.WithCancel(context.Background())
	hecho := make(chan error, 1)
	metricas := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "m") })
	// Un REDIS_URL ilegible solo desactiva la idempotencia compartida; no impide servir.
	go func() { hecho <- ServePush(ctx, puerto, "redis://%zz", procesadorNulo{}, metricas, slog.Default()) }()

	var resp *http.Response
	var err error
	for i := 0; i < 50; i++ {
		resp, err = http.Get("http://127.0.0.1:" + puerto + "/healthz")
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("el worker no atendio: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz = %d", resp.StatusCode)
	}

	cancel()
	select {
	case err := <-hecho:
		if err != nil {
			t.Fatalf("un cierre pedido no es un error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("ServePush no volvio")
	}
}

func TestServePushFallaSiElPuertoEstaOcupado(t *testing.T) {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	puerto := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)

	if err := ServePush(context.Background(), puerto, "", procesadorNulo{}, http.NotFoundHandler(), slog.Default()); err == nil {
		t.Fatal("se esperaba error con el puerto ocupado")
	}
}
