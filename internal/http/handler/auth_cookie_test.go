package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSessionCookieHasSecureBrowserAttributes(t *testing.T) {
	expiresAt := time.Date(2026, time.September, 27, 12, 0, 0, 0, time.UTC)
	h := NewAuthHandlerWithSameSite(nil, http.SameSiteLaxMode, nil)
	cookie := h.sessionCookie("session-token", expiresAt)
	response := httptest.NewRecorder()
	http.SetCookie(response, cookie)
	serialized := response.Header().Get("Set-Cookie")

	if cookie.Name != SessionCookieName || cookie.Value != "session-token" {
		t.Errorf("unexpected session cookie identity: %#v", cookie)
	}
	if cookie.Path != "/" || cookie.Domain != "" {
		t.Errorf("__Host- cookie must use Path=/ and omit Domain: %#v", cookie)
	}
	if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("session cookie is missing secure browser attributes: %#v", cookie)
	}
	if !cookie.Expires.Equal(expiresAt) {
		t.Errorf("cookie expiry %v does not match session expiry %v", cookie.Expires, expiresAt)
	}
	for _, attribute := range []string{"Secure", "HttpOnly", "SameSite=Lax", "Path=/"} {
		if !strings.Contains(serialized, attribute) {
			t.Errorf("serialized cookie lacks %q: %s", attribute, serialized)
		}
	}
	if strings.Contains(serialized, "Domain=") {
		t.Errorf("serialized __Host- cookie must not set Domain: %s", serialized)
	}
}

func TestExpiredSessionCookieDeletesTheBrowserCookie(t *testing.T) {
	h := NewAuthHandlerWithSameSite(nil, http.SameSiteLaxMode, nil)
	cookie := h.expiredSessionCookie()
	if cookie.Name != SessionCookieName || cookie.Path != "/" || cookie.MaxAge >= 0 {
		t.Errorf("session cookie is not configured to expire: %#v", cookie)
	}
	if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("expired cookie is missing secure browser attributes: %#v", cookie)
	}
}

// ---- A4: la cookie entre sitios distintos -------------------------------
//
// `lax` es el valor por omision y el preferible: con el frontend detras del
// mismo balanceador, todo es del mismo sitio. Pero si D1 lo aloja aparte, la
// cookie tiene que poder viajar entre origenes o la sesion no existira para el
// frontend, por mucho que el login haya devuelto 200.

func TestSameSiteFromConfig(t *testing.T) {
	for valor, quiero := range map[string]http.SameSite{
		"":       http.SameSiteLaxMode,
		"lax":    http.SameSiteLaxMode,
		"LAX":    http.SameSiteLaxMode,
		" none":  http.SameSiteNoneMode,
		"strict": http.SameSiteStrictMode,
	} {
		got, ok := SameSiteFromConfig(valor)
		if !ok || got != quiero {
			t.Errorf("SameSiteFromConfig(%q) = %v, %v; se esperaba %v, true", valor, got, ok, quiero)
		}
	}

	// Un valor que no se reconoce cae en el seguro y lo dice, para que el
	// servidor pueda avisar en vez de dejar un frontend que no se autentica
	// sin que nadie sepa por que.
	got, ok := SameSiteFromConfig("sometimes")
	if ok || got != http.SameSiteLaxMode {
		t.Errorf("un valor desconocido debe caer en lax y reportarlo; dio %v, %v", got, ok)
	}
}

func TestCookieEntreSitiosConservaLasDemasGarantias(t *testing.T) {
	h := NewAuthHandlerWithSameSite(nil, http.SameSiteNoneMode, nil)
	cookie := h.sessionCookie("session-token", time.Now().Add(time.Hour))

	if cookie.SameSite != http.SameSiteNoneMode {
		t.Fatalf("SameSite = %v, se esperaba None", cookie.SameSite)
	}
	// SameSite=None sin Secure lo rechaza el navegador, y un __Host- sin
	// Path=/ o con Domain deja de ser __Host-.
	if !cookie.Secure || !cookie.HttpOnly || cookie.Path != "/" || cookie.Domain != "" {
		t.Errorf("la cookie entre sitios perdio alguna garantia: %#v", cookie)
	}
}

// La cookie de borrado tiene que llevar los mismos atributos que la que
// sustituye: con un SameSite distinto el navegador la trata como otra cookie y
// la de verdad sobrevive al cierre de sesion.
func TestLaCookieDeBorradoCoincideConLaDeSesion(t *testing.T) {
	for _, modo := range []http.SameSite{http.SameSiteLaxMode, http.SameSiteNoneMode, http.SameSiteStrictMode} {
		h := NewAuthHandlerWithSameSite(nil, modo, nil)
		sesion := h.sessionCookie("t", time.Now().Add(time.Hour))
		borrado := h.expiredSessionCookie()

		if borrado.SameSite != sesion.SameSite || borrado.Path != sesion.Path ||
			borrado.Secure != sesion.Secure || borrado.Domain != sesion.Domain {
			t.Errorf("modo %v: la cookie de borrado no coincide con la de sesion; sesion=%#v borrado=%#v",
				modo, sesion, borrado)
		}
	}
}
