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
	cookie := sessionCookie("session-token", expiresAt)
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
	cookie := expiredSessionCookie()
	if cookie.Name != SessionCookieName || cookie.Path != "/" || cookie.MaxAge >= 0 {
		t.Errorf("session cookie is not configured to expire: %#v", cookie)
	}
	if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("expired cookie is missing secure browser attributes: %#v", cookie)
	}
}
