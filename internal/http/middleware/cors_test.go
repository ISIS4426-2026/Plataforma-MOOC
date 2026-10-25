package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/http/middleware"
)

const origenFrontend = "https://frontend.ejemplo.test"

func conCORS(origenes ...string) http.Handler {
	return middleware.CORS(middleware.CORSConfig{AllowedOrigins: origenes})(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		}))
}

// Con el frontend detras del mismo balanceador no hay peticion entre origenes,
// y el middleware no debe anadir ni una cabecera. Es la configuracion
// recomendada, y tiene que seguir siendo la mas barata.
func TestSinOrigenesNoTocaNada(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/courses", nil)
	r.Header.Set("Origin", origenFrontend)

	conCORS().ServeHTTP(w, r)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("sin lista de origenes no debe salir Allow-Origin, salio %q", got)
	}
}

func TestDevuelveElOrigenExactoYAdmiteCredenciales(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/courses", nil)
	r.Header.Set("Origin", origenFrontend)

	conCORS(origenFrontend).ServeHTTP(w, r)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != origenFrontend {
		t.Errorf("Allow-Origin = %q, se esperaba el origen exacto", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Allow-Credentials = %q; sin esto la cookie de sesion no viaja", got)
	}
	// Sin Vary, una cache intermedia podria servirle a un origen la respuesta
	// que se marco para otro.
	if !strings.Contains(w.Header().Get("Vary"), "Origin") {
		t.Errorf("falta Vary: Origin, salio %q", w.Header().Get("Vary"))
	}
}

// El comodin es incompatible con Allow-Credentials segun la especificacion, y
// ademas anularia la lista que sostiene la proteccion contra CSRF.
func TestNuncaDevuelveComodin(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/courses", nil)
	r.Header.Set("Origin", origenFrontend)

	conCORS(origenFrontend).ServeHTTP(w, r)

	if w.Header().Get("Access-Control-Allow-Origin") == "*" {
		t.Error("Allow-Origin devolvio comodin con credenciales habilitadas")
	}
}

// Un origen ajeno no recibe las cabeceras, pero tampoco un error: bloquear la
// lectura es cosa del navegador. Devolver 403 convertiria en fallo del
// servidor algo que es una politica del cliente.
func TestUnOrigenAjenoNoRecibeLasCabeceras(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/courses", nil)
	r.Header.Set("Origin", "https://atacante.ejemplo.test")

	conCORS(origenFrontend).ServeHTTP(w, r)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("un origen fuera de la lista recibio Allow-Origin = %q", got)
	}
	if w.Code != http.StatusOK {
		t.Errorf("code = %d; el servidor no debe convertirlo en error", w.Code)
	}
}

func TestElPreflightSeContestaSinLlegarAlEnrutador(t *testing.T) {
	llego := false
	h := middleware.CORS(middleware.CORSConfig{AllowedOrigins: []string{origenFrontend}})(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) { llego = true }))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodOptions, "/api/v1/quizzes/x/submissions", nil)
	r.Header.Set("Origin", origenFrontend)
	r.Header.Set("Access-Control-Request-Method", http.MethodPost)
	r.Header.Set("Access-Control-Request-Headers", "Content-Type, Idempotency-Key")

	h.ServeHTTP(w, r)

	if llego {
		t.Error("el preflight llego al enrutador; no lleva cookie ni traza que leer")
	}
	if w.Code != http.StatusNoContent {
		t.Errorf("code = %d, se esperaba 204", w.Code)
	}
	permitidas := w.Header().Get("Access-Control-Allow-Headers")
	for _, necesaria := range []string{"Content-Type", "Idempotency-Key"} {
		if !strings.Contains(permitidas, necesaria) {
			t.Errorf("el preflight no admitio %q: %q", necesaria, permitidas)
		}
	}
	if !strings.Contains(w.Header().Get("Access-Control-Allow-Methods"), http.MethodPost) {
		t.Errorf("el preflight no admitio POST: %q", w.Header().Get("Access-Control-Allow-Methods"))
	}
}

// Devolver lo que el navegador pida sin filtrar seria anunciar que se acepta
// cualquier cabecera, que es lo mismo que no tener lista.
func TestElPreflightFiltraLasCabecerasPedidas(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodOptions, "/api/v1/courses", nil)
	r.Header.Set("Origin", origenFrontend)
	r.Header.Set("Access-Control-Request-Method", http.MethodPost)
	r.Header.Set("Access-Control-Request-Headers", "Content-Type, X-Inventada")

	conCORS(origenFrontend).ServeHTTP(w, r)

	permitidas := w.Header().Get("Access-Control-Allow-Headers")
	if strings.Contains(strings.ToLower(permitidas), "inventada") {
		t.Errorf("el preflight admitio una cabecera que la API no usa: %q", permitidas)
	}
}

// Sin esto, `fetch` no ve ninguna cabecera de respuesta salvo un punado de
// seguras: el frontend no podria correlacionar una peticion con su traza ni
// saber que replica le contesto, que es lo que C4 y G5 necesitan demostrar.
func TestExponeLaTrazaYLaReplica(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/courses", nil)
	r.Header.Set("Origin", origenFrontend)

	conCORS(origenFrontend).ServeHTTP(w, r)

	expuestas := w.Header().Get("Access-Control-Expose-Headers")
	for _, necesaria := range []string{middleware.HeaderRequestID, middleware.HeaderInstanceID} {
		if !strings.Contains(expuestas, necesaria) {
			t.Errorf("no se expuso %q: %q", necesaria, expuestas)
		}
	}
}

// Una peticion del mismo sitio no lleva Origin y no debe pagar nada.
func TestSinCabeceraOrigenPasaDeLargo(t *testing.T) {
	w := httptest.NewRecorder()
	conCORS(origenFrontend).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/courses", nil))

	if got := w.Header().Get("Vary"); strings.Contains(got, "Origin") {
		t.Errorf("una peticion sin Origin no necesita Vary: %q", got)
	}
	if w.Code != http.StatusOK {
		t.Errorf("code = %d", w.Code)
	}
}
