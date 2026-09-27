package domain

import (
	"context"
	"time"
)

// QuestionOption is one of the choices of a question.
//
// IsCorrect never reaches a client. El enunciado lo pide como criterio de
// aceptacion -- «la clave correcta nunca llega al cliente»-- y por eso la
// proteccion no es solo esta etiqueta: el repositorio tiene una lectura para el
// estudiante que ni siquiera selecciona la columna. Un fallo de serializacion
// no puede filtrar lo que nunca se cargo.
type QuestionOption struct {
	ID   string `json:"id"`
	Text string `json:"text"`

	// Position fija el orden en que se presentan. Sin ella no habria ninguno:
	// created_at es la hora de inicio de la transaccion, asi que todas las
	// opciones insertadas juntas empatan y el desempate caeria en un UUID.
	Position  int  `json:"position"`
	IsCorrect bool `json:"-"`
}

type Question struct {
	ID       string           `json:"id"`
	QuizID   string           `json:"quiz_id"`
	Prompt   string           `json:"prompt"`
	Position int              `json:"position"`
	Options  []QuestionOption `json:"options"`
}

type Quiz struct {
	ID         string `json:"id"`
	ResourceID string `json:"resource_id"`
	Title      string `json:"title"`

	// PassingScore es el porcentaje a partir del cual se aprueba.
	PassingScore int `json:"passing_score"`

	// MaxAttempts es la politica de intentos del quiz. Por quiz y no global,
	// porque es una decision del autor igual que la nota de aprobacion.
	MaxAttempts int `json:"max_attempts"`

	Questions []Question `json:"questions,omitempty"`
}

// Answer es la respuesta a una pregunta.
//
// Una lista y no un mapa, siguiendo el contrato que api/openapi.yaml ya
// documentaba. Ademas una lista conserva el orden y hace visible una pregunta
// respondida dos veces, que con un mapa se perderia en silencio.
type Answer struct {
	QuestionID       string `json:"question_id"`
	SelectedOptionID string `json:"selected_option_id"`
}

type QuizSubmission struct {
	ID        string   `json:"id"`
	QuizID    string   `json:"quiz_id"`
	StudentID string   `json:"student_id"`
	Answers   []Answer `json:"answers"`

	// Attempt es el numero de intento, empezando en 1. Se guarda en lugar de
	// deducirse al leer porque es lo que la restriccion de unicidad usa para
	// impedir que dos envios simultaneos consuman el mismo intento.
	Attempt int `json:"attempt"`

	// Score es el porcentaje de aciertos, 0 a 100, calculado siempre en el
	// servidor. El enunciado rechaza cualquier calificacion que venga del
	// cliente.
	Score  int  `json:"score"`
	Passed bool `json:"passed"`

	SubmittedAt time.Time `json:"submitted_at"`
}

// QuizGrade es lo que el estudiante recibe tras presentar: su resultado y
// cuantos intentos le quedan. No incluye que respuesta era la correcta.
type QuizGrade struct {
	Submission        *QuizSubmission
	AttemptsUsed      int
	AttemptsRemaining int
}

type QuizRepository interface {
	// Create crea el quiz de un recurso. Falla con ErrConflict si el recurso ya
	// tiene uno.
	Create(ctx context.Context, quiz *Quiz, entry *AuditEntry) (*Quiz, error)

	// AddQuestion agrega una pregunta con sus opciones al final del quiz. Las
	// opciones se insertan en la misma transaccion: una pregunta sin opciones no
	// es contestable, y dejarla a medias seria peor que no crearla.
	AddQuestion(ctx context.Context, quizID string, question *Question, entry *AuditEntry) (*Question, error)

	// GetForAuthor devuelve el quiz completo, con la marca de cual opcion es
	// correcta. Solo el autor y los administradores llegan aqui.
	GetForAuthor(ctx context.Context, quizID string) (*Quiz, error)

	// GetForStudent devuelve el quiz sin la clave de respuestas.
	//
	// No es la misma consulta filtrando despues: es una consulta que **no
	// selecciona** is_correct. Esa es la diferencia entre ocultar un dato y no
	// tenerlo, y es la que hace que un error en la capa HTTP no pueda filtrarlo.
	GetForStudent(ctx context.Context, quizID string) (*Quiz, error)

	// GetByResourceID resuelve el quiz de un recurso, sin preguntas.
	GetByResourceID(ctx context.Context, resourceID string) (*Quiz, error)

	// Grade califica un envio y lo registra como un intento nuevo.
	//
	// La calificacion ocurre dentro de la transaccion, leyendo la clave correcta
	// de la base: ni el cliente ni las capas superiores la ven en ningun momento.
	//
	// Devuelve ErrConflict cuando el estudiante agoto sus intentos, tanto si lo
	// detecta al contar como si lo detecta la restriccion de unicidad al
	// insertar -- que es lo que ocurre cuando dos envios llegan a la vez.
	Grade(ctx context.Context, quizID, studentID string, answers []Answer, entry *AuditEntry) (*QuizGrade, error)

	// ListSubmissions devuelve los intentos de un estudiante, del mas reciente
	// al mas antiguo.
	ListSubmissions(ctx context.Context, studentID, quizID string) ([]*QuizSubmission, error)
}
