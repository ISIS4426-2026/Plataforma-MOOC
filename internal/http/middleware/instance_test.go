package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/http/middleware"
)

func TestInstanceIDSaleEnLaCabecera(t *testing.T) {
	id := middleware.NewInstanceID("mooc-api-grupo-7h3k")
	h := middleware.WithInstanceID(id)(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := w.Header().Get(middleware.HeaderInstanceID); got != id.Public {
		t.Fatalf("cabecera = %q, se esperaba %q", got, id.Public)
	}
}

// Sin esto no se puede demostrar que el balanceador reparte, que es lo que
// piden C4 y el escenario 3A.
func TestDosInstanciasSeDistinguen(t *testing.T) {
	una := middleware.NewInstanceID("mooc-api-grupo-aaaa")
	otra := middleware.NewInstanceID("mooc-api-grupo-bbbb")

	if una.Public == otra.Public {
		t.Fatal("dos replicas distintas comparten identificador; el reparto no seria demostrable")
	}
}

// El mismo proceso no puede cambiar de identidad entre peticiones, o agrupar
// por destino dejaria de significar nada.
func TestLaIdentidadEsEstable(t *testing.T) {
	primera := middleware.NewInstanceID("mooc-api-grupo-7h3k")
	segunda := middleware.NewInstanceID("mooc-api-grupo-7h3k")

	if primera.Public != segunda.Public || primera.Private != segunda.Private {
		t.Fatalf("la identidad cambio entre llamadas: %+v vs %+v", primera, segunda)
	}
}

// El nombre que GCE le da a una instancia de un grupo delata el nombre del
// grupo y el sufijo que lo numera. Eso es topologia interna, y el criterio de
// aceptacion pide que la respuesta no la revele.
func TestLaCabeceraNoDelataElNombreDeLaMaquina(t *testing.T) {
	nombre := "mooc-api-grupo-7h3k"
	id := middleware.NewInstanceID(nombre)

	if strings.Contains(id.Public, "mooc") || strings.Contains(id.Public, "7h3k") {
		t.Errorf("el identificador publico %q contiene parte del nombre %q", id.Public, nombre)
	}
	if id.Private != nombre {
		t.Errorf("el log perdio el nombre real: %q", id.Private)
	}
}

// Una replica sin identidad es justo la que no se puede rastrear.
func TestSiempreHayIdentidad(t *testing.T) {
	id := middleware.NewInstanceID("   ")
	if id.Public == "" || id.Private == "" {
		t.Fatalf("identidad vacia: %+v", id)
	}

	// Sin variable de entorno cae en el nombre de la maquina.
	host, err := os.Hostname()
	if err == nil && strings.TrimSpace(host) != "" && id.Private != strings.TrimSpace(host) {
		t.Errorf("Private = %q, se esperaba el nombre de la maquina %q", id.Private, host)
	}
}
