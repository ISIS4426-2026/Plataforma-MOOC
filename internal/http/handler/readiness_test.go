package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/http/handler"
)

type pingerFalso struct{ err error }

func (p pingerFalso) PingContext(context.Context) error { return p.err }

func listo() *atomic.Bool {
	b := &atomic.Bool{}
	b.Store(true)
	return b
}

// Liveness no puede depender de nada. Si dependiera de la base, un parpadeo de
// Cloud SQL haria fallar la comprobacion de las tres replicas a la vez y el
// grupo administrado las recrearia todas: un problema transitorio de una
// dependencia compartida convertido en una caida completa.
func TestLivenessNoDependeDeLaBase(t *testing.T) {
	w := httptest.NewRecorder()
	handler.NewLivenessHandler()(w, httptest.NewRequest(http.MethodGet, "/api/v1/live", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, se esperaba 200 aunque la base no responda", w.Code)
	}
}

func TestReadinessPasaConLaBaseArriba(t *testing.T) {
	w := httptest.NewRecorder()
	handler.NewReadinessHandler(pingerFalso{}, listo())(
		w, httptest.NewRequest(http.MethodGet, "/api/v1/ready", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, se esperaba 200", w.Code)
	}
	var body handler.Readiness
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("respuesta ilegible: %v", err)
	}
	if body.Status != "pass" {
		t.Errorf("status = %q, se esperaba pass", body.Status)
	}
}

// 503 es lo que saca a la instancia de la rotacion del balanceador. Un 200 con
// la base caida le reparte errores a los estudiantes.
func TestReadinessFallaConLaBaseCaida(t *testing.T) {
	w := httptest.NewRecorder()
	handler.NewReadinessHandler(pingerFalso{err: errors.New("sin conexión")}, listo())(
		w, httptest.NewRequest(http.MethodGet, "/api/v1/ready", nil))

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, se esperaba 503", w.Code)
	}
}

// La ventana en la que el proceso vive pero todavia no termino de montarse.
// Durante ella la base puede contestar perfectamente y aun asi esta replica no
// debe recibir trafico.
func TestReadinessFallaAntesDeEstarListo(t *testing.T) {
	w := httptest.NewRecorder()
	handler.NewReadinessHandler(pingerFalso{}, &atomic.Bool{})(
		w, httptest.NewRequest(http.MethodGet, "/api/v1/ready", nil))

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, se esperaba 503 mientras la instancia se monta", w.Code)
	}
}

// El criterio de aceptacion del issue: la respuesta no revela dependencias,
// versiones ni topologia. Quien consulta estas rutas es una sonda automatica
// que solo mira el codigo de estado.
func TestLasSondasNoRevelanNada(t *testing.T) {
	prohibido := []string{
		"database", "postgres", "redis", "sql", "bucket", "storage",
		"version", "host", "addr", "port", "instance",
	}

	for ruta, h := range map[string]http.HandlerFunc{
		"/api/v1/live":  handler.NewLivenessHandler(),
		"/api/v1/ready": handler.NewReadinessHandler(pingerFalso{}, listo()),
	} {
		for _, caida := range []bool{false, true} {
			sonda := h
			if caida {
				sonda = handler.NewReadinessHandler(
					pingerFalso{err: errors.New("se cayó la base de datos en el host interno")}, listo())
			}

			w := httptest.NewRecorder()
			sonda(w, httptest.NewRequest(http.MethodGet, ruta, nil))

			cuerpo := strings.ToLower(w.Body.String())
			for _, palabra := range prohibido {
				if strings.Contains(cuerpo, palabra) {
					t.Errorf("%s (caída=%v) filtró %q en %q", ruta, caida, palabra, w.Body.String())
				}
			}
		}
	}
}
