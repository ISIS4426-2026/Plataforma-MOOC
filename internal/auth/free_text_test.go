package auth_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/auth"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
)

// registerWithName drives a registration with a given full name and returns the
// error, if any.
func registerWithName(t *testing.T, h *harness, name string) error {
	t.Helper()

	_, err := h.service.Register(context.Background(), auth.RegisterInput{
		Email:    testEmail,
		Password: testPassword,
		FullName: name,
	})
	return err
}

// Free text that carries markup is refused rather than stored. Output encoding
// is the primary defence, but nothing legitimate needs angle brackets in a name.
func TestRegisterRejectsMarkupInFreeText(t *testing.T) {
	payloads := map[string]string{
		"script tag":  `<script>alert(1)</script>`,
		"img onerror": `<img src=x onerror=alert(1)>`,
		"bare open":   `Juan <Perez`,
		"bare close":  `Juan Perez>`,
	}

	for name, payload := range payloads {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t, auth.Config{})

			if err := registerWithName(t, h, payload); !errors.Is(err, domain.ErrInvalidInput) {
				t.Errorf("expected domain.ErrInvalidInput for %q, got %v", payload, err)
			}
		})
	}
}

// Control characters are used to hide content from a reviewer or to split a
// record in a log or a CSV export.
func TestRegisterRejectsControlCharactersInFreeText(t *testing.T) {
	payloads := map[string]string{
		"newline":         "Juan\nPerez",
		"carriage return": "Juan\rPerez",
		"tab":             "Juan\tPerez",
		"null byte":       "Juan\x00Perez",
		"escape":          "Juan\x1b[31mPerez",
	}

	for name, payload := range payloads {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t, auth.Config{})

			if err := registerWithName(t, h, payload); !errors.Is(err, domain.ErrInvalidInput) {
				t.Errorf("expected domain.ErrInvalidInput for %q, got %v", payload, err)
			}
		})
	}
}

// A single field must not be a place to park a payload.
func TestRegisterRejectsOverlongFreeText(t *testing.T) {
	h := newHarness(t, auth.Config{})

	if err := registerWithName(t, h, strings.Repeat("a", 201)); !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("expected domain.ErrInvalidInput, got %v", err)
	}
}

// The filter must not get in the way of real names.
func TestRegisterAcceptsLegitimateNames(t *testing.T) {
	names := []string{
		"María José Gutiérrez",
		"Fredy Alexander Chaparro Castro",
		"Ana O'Brien",
		"Jean-Luc Dupont",
		"Nguyễn Thị Ngọc",
		"李 明",
	}

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t, auth.Config{})

			if err := registerWithName(t, h, name); err != nil {
				t.Errorf("a legitimate name was rejected: %v", err)
			}
		})
	}
}

// Surrounding whitespace is trimmed rather than refused, since it is a typing
// slip and not an attack.
func TestRegisterTrimsSurroundingWhitespace(t *testing.T) {
	h := newHarness(t, auth.Config{})

	user, err := h.service.Register(context.Background(), auth.RegisterInput{
		Email:    testEmail,
		Password: testPassword,
		FullName: "   Ana Gómez   ",
	})
	if err != nil {
		t.Fatalf("Register returned an error: %v", err)
	}

	if user.FullName != "Ana Gómez" {
		t.Errorf("expected the name trimmed, got %q", user.FullName)
	}
}

// The primary defence: whatever reaches a response is HTML-escaped by
// encoding/json, so a value that survived storage still cannot break out of an
// HTML context that embeds the document.
func TestJSONEncodingEscapesHTMLSensitiveCharacters(t *testing.T) {
	payload := map[string]string{"full_name": `<script>alert("x")</script> & more`}

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	rendered := string(encoded)
	for _, forbidden := range []string{"<", ">", "&"} {
		if strings.Contains(rendered, forbidden) {
			t.Errorf("expected %q to be escaped, got %s", forbidden, rendered)
		}
	}

	// And the value still round-trips unchanged for a legitimate consumer.
	var decoded map[string]string
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if decoded["full_name"] != payload["full_name"] {
		t.Error("escaping must not alter the value once decoded")
	}
}
