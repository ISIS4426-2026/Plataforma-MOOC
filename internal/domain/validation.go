package domain

import "strings"

// FieldError names one thing wrong with a submitted field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationErrors is every problem found with an attempted write, not just
// the first.
//
// Issue #20 asks for this directly: "un intento de publicar con datos
// incompletos devuelve la lista completa de errores, no solo el primero"
// (segmento 2 de la sección 10.2). Returning on the first failure would save
// the caller nothing -- they would fix it, resubmit, and hit the next one,
// one round trip per problem, on a form a professor may be filling out for
// the first time.
type ValidationErrors []FieldError

func (v ValidationErrors) Error() string {
	messages := make([]string, len(v))
	for i, fe := range v {
		messages[i] = fe.Field + ": " + fe.Message
	}
	return "validation failed: " + strings.Join(messages, "; ")
}
