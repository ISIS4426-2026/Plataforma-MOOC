package push

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func respuesta(texto string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, texto) })
}

func TestNewMuxEnrutaCadaRuta(t *testing.T) {
	mux := NewMux(respuesta("push"), respuesta("vivo"), respuesta("metricas"))

	casos := []struct {
		metodo, ruta string
		esperado     int
		cuerpo       string
	}{
		{http.MethodPost, "/pubsub/push", http.StatusOK, "push"},
		{http.MethodGet, "/healthz", http.StatusOK, "vivo"},
		{http.MethodGet, "/readyz", http.StatusOK, "vivo"},
		{http.MethodGet, "/metrics", http.StatusOK, "metricas"},
		{http.MethodGet, "/pubsub/push", http.StatusMethodNotAllowed, ""},
		{http.MethodGet, "/otra", http.StatusNotFound, ""},
	}
	for _, c := range casos {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(c.metodo, c.ruta, nil))
		if rec.Code != c.esperado {
			t.Errorf("%s %s: codigo %d, se esperaba %d", c.metodo, c.ruta, rec.Code, c.esperado)
		}
		if c.cuerpo != "" && !strings.Contains(rec.Body.String(), c.cuerpo) {
			t.Errorf("%s %s: cuerpo %q no contiene %q", c.metodo, c.ruta, rec.Body.String(), c.cuerpo)
		}
	}
}

func TestServeAtiendeYSeCierraAlTerminarElContexto(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	hecho := make(chan error, 1)
	go func() { hecho <- Serve(ctx, ln, respuesta("hola"), time.Second) }()

	resp, err := http.Get("http://" + ln.Addr().String() + "/")
	if err != nil {
		t.Fatal(err)
	}
	cuerpo, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if string(cuerpo) != "hola" {
		t.Fatalf("cuerpo %q", cuerpo)
	}

	cancel()
	select {
	case err := <-hecho:
		if err != nil {
			t.Fatalf("un cierre pedido no es un error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Serve no volvio tras cancelar el contexto")
	}
}

func TestServeDevuelveElErrorSiElServidorSeCae(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_ = ln.Close() // un listener cerrado hace fallar a Serve de inmediato

	if err := Serve(context.Background(), ln, respuesta("x"), time.Second); err == nil {
		t.Fatal("se esperaba el error del servidor")
	}
}
