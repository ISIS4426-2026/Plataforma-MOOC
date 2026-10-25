package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/lib/pq"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
)

// This file backs issue #112.
//
// Tres cosas ocurren aqui y no mas arriba, y las tres por la misma razon: son
// garantias que no pueden depender de que ninguna capa superior se comporte
// bien.
//
//  1. La calificacion se calcula leyendo la clave correcta dentro de la
//     transaccion. Ni el servicio ni el handler la ven.
//  2. La lectura del estudiante no selecciona is_correct. No la oculta: no la
//     trae.
//  3. El limite de intentos lo garantiza una restriccion de unicidad, no un
//     conteo previo. Contar y despues insertar deja una ventana entre ambas
//     cosas, y dos envios simultaneos caben dentro de esa ventana.

type QuizRepository struct{ db *sql.DB }

var _ domain.QuizRepository = (*QuizRepository)(nil)

func NewQuizRepository(db *sql.DB) *QuizRepository {
	return &QuizRepository{db: db}
}

// Create cuelga un quiz de un recurso.
func (r *QuizRepository) Create(ctx context.Context, quiz *domain.Quiz, entry *domain.AuditEntry) (*domain.Quiz, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Bloquea el recurso y su curso, igual que cualquier otra escritura de
	// autoria: crear un quiz sobre un curso publicado es modificar contenido
	// publicado, y la inmutabilidad se aplica igual.
	if err := lockResourceAndCourse(ctx, tx, quiz.ResourceID); err != nil {
		return nil, err
	}

	const query = `
		INSERT INTO quizzes (resource_id, title, passing_score, max_attempts)
		VALUES ($1, $2, $3, $4)
		RETURNING id, resource_id, title, passing_score, max_attempts`

	created := &domain.Quiz{}
	err = tx.QueryRowContext(ctx, query, quiz.ResourceID, quiz.Title, quiz.PassingScore, quiz.MaxAttempts).Scan(
		&created.ID, &created.ResourceID, &created.Title, &created.PassingScore, &created.MaxAttempts,
	)
	if isUniqueViolation(err) {
		return nil, fmt.Errorf("resource %s already has a quiz: %w", quiz.ResourceID, domain.ErrConflict)
	}
	if err != nil {
		return nil, fmt.Errorf("create quiz: %w", err)
	}

	if entry != nil {
		entry.Action = domain.AuditActionQuizCreated
		if err := insertAuditEntry(ctx, tx, entry); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit quiz: %w", err)
	}
	return created, nil
}

// AddQuestion agrega una pregunta con sus opciones, todo en una transaccion.
//
// Una pregunta sin opciones no se puede contestar, asi que insertarla sola y
// dejar las opciones para otra llamada permitiria que un fallo a mitad dejara
// el quiz en un estado que ningun estudiante puede completar.
func (r *QuizRepository) AddQuestion(ctx context.Context, quizID string, question *domain.Question, entry *domain.AuditEntry) (*domain.Question, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	resourceID, err := quizResourceID(ctx, tx, quizID)
	if err != nil {
		return nil, err
	}
	if err := lockResourceAndCourse(ctx, tx, resourceID); err != nil {
		return nil, err
	}

	// La posicion es base 0 y se asigna contando hermanas, igual que en modulos,
	// unidades y recursos. El bloqueo del recurso de arriba es lo que hace que
	// dos inserciones simultaneas no calculen la misma.
	var position int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM quiz_questions WHERE quiz_id = $1`, quizID,
	).Scan(&position); err != nil {
		return nil, fmt.Errorf("count questions: %w", err)
	}

	created := &domain.Question{QuizID: quizID, Prompt: question.Prompt, Position: position}
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO quiz_questions (quiz_id, prompt, position) VALUES ($1, $2, $3) RETURNING id`,
		quizID, question.Prompt, position,
	).Scan(&created.ID); err != nil {
		return nil, fmt.Errorf("insert question: %w", err)
	}

	// La posicion de cada opcion es el orden en que el autor las envio. Es lo
	// que hace que la pregunta se presente siempre igual.
	for i, option := range question.Options {
		var optionID string
		if err := tx.QueryRowContext(ctx,
			`INSERT INTO quiz_options (question_id, text, is_correct, position) VALUES ($1, $2, $3, $4) RETURNING id`,
			created.ID, option.Text, option.IsCorrect, i,
		).Scan(&optionID); err != nil {
			return nil, fmt.Errorf("insert option: %w", err)
		}
		created.Options = append(created.Options, domain.QuestionOption{
			ID: optionID, Text: option.Text, Position: i, IsCorrect: option.IsCorrect,
		})
	}

	if entry != nil {
		entry.Action = domain.AuditActionQuizQuestionAdded
		if err := insertAuditEntry(ctx, tx, entry); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit question: %w", err)
	}
	return created, nil
}

// GetForAuthor devuelve el quiz con la clave de respuestas.
func (r *QuizRepository) GetForAuthor(ctx context.Context, quizID string) (*domain.Quiz, error) {
	return r.get(ctx, quizID, true)
}

// GetForStudent devuelve el quiz sin la clave.
func (r *QuizRepository) GetForStudent(ctx context.Context, quizID string) (*domain.Quiz, error) {
	return r.get(ctx, quizID, false)
}

func (r *QuizRepository) get(ctx context.Context, quizID string, withKey bool) (*domain.Quiz, error) {
	quiz := &domain.Quiz{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, resource_id, title, passing_score, max_attempts FROM quizzes WHERE id = $1`, quizID,
	).Scan(&quiz.ID, &quiz.ResourceID, &quiz.Title, &quiz.PassingScore, &quiz.MaxAttempts)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get quiz: %w", err)
	}

	// Las dos consultas difieren en una columna, y esa es toda la proteccion:
	// para el estudiante, is_correct no se selecciona. No hay nada que filtrar
	// mas arriba porque no se cargo.
	query := `
		SELECT q.id, q.prompt, q.position, o.id, o.text, o.position, FALSE
		FROM quiz_questions q
		LEFT JOIN quiz_options o ON o.question_id = q.id
		WHERE q.quiz_id = $1
		ORDER BY q.position, o.position`
	if withKey {
		query = `
		SELECT q.id, q.prompt, q.position, o.id, o.text, o.position, o.is_correct
		FROM quiz_questions q
		LEFT JOIN quiz_options o ON o.question_id = q.id
		WHERE q.quiz_id = $1
		ORDER BY q.position, o.position`
	}

	rows, err := r.db.QueryContext(ctx, query, quizID)
	if err != nil {
		return nil, fmt.Errorf("list questions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	byID := map[string]int{}
	for rows.Next() {
		var (
			questionID, prompt string
			position           int
			optionID, text     sql.NullString
			optionPosition     sql.NullInt64
			isCorrect          sql.NullBool
		)
		if err := rows.Scan(&questionID, &prompt, &position, &optionID, &text, &optionPosition, &isCorrect); err != nil {
			return nil, fmt.Errorf("scan question: %w", err)
		}
		idx, seen := byID[questionID]
		if !seen {
			quiz.Questions = append(quiz.Questions, domain.Question{
				ID: questionID, QuizID: quizID, Prompt: prompt, Position: position,
				Options: []domain.QuestionOption{},
			})
			idx = len(quiz.Questions) - 1
			byID[questionID] = idx
		}
		// El LEFT JOIN deja la opcion nula cuando una pregunta aun no tiene
		// ninguna. Es un quiz a medio escribir, no un error de lectura.
		if optionID.Valid {
			quiz.Questions[idx].Options = append(quiz.Questions[idx].Options, domain.QuestionOption{
				ID: optionID.String, Text: text.String,
				Position: int(optionPosition.Int64), IsCorrect: isCorrect.Bool,
			})
		}
	}
	return quiz, rows.Err()
}

// GetByResourceID resuelve el quiz de un recurso, sin preguntas.
func (r *QuizRepository) GetByResourceID(ctx context.Context, resourceID string) (*domain.Quiz, error) {
	quiz := &domain.Quiz{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, resource_id, title, passing_score, max_attempts FROM quizzes WHERE resource_id = $1`, resourceID,
	).Scan(&quiz.ID, &quiz.ResourceID, &quiz.Title, &quiz.PassingScore, &quiz.MaxAttempts)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get quiz by resource: %w", err)
	}
	return quiz, nil
}

// Grade califica y registra un intento.
func (r *QuizRepository) Grade(
	ctx context.Context, quizID, studentID string, answers []domain.Answer, entry *domain.AuditEntry,
) (*domain.QuizGrade, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var passingScore, maxAttempts int
	err = tx.QueryRowContext(ctx,
		`SELECT passing_score, max_attempts FROM quizzes WHERE id = $1`, quizID,
	).Scan(&passingScore, &maxAttempts)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read quiz policy: %w", err)
	}

	var used int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM quiz_submissions WHERE quiz_id = $1 AND student_id = $2`, quizID, studentID,
	).Scan(&used); err != nil {
		return nil, fmt.Errorf("count attempts: %w", err)
	}
	if used >= maxAttempts {
		return nil, fmt.Errorf("student %s used all %d attempts: %w", studentID, maxAttempts, domain.ErrConflict)
	}

	// El cuestionario contra el que se califica queda congelado pase lo que
	// pase, haya intento abierto o no.
	//
	// Con intento abierto se usa el que se congelo al iniciarlo, que es lo que
	// el estudiante vio. Sin el --el envio directo, que es como funcionaba
	// todo hasta ahora y como sigue funcionando el arnes E2E-- se congela el
	// estado de este instante. En los dos casos, editar el quiz despues deja de
	// alterar la nota ya registrada, que es el defecto que marco la revision de
	// la Entrega 1.
	var (
		snapshot []byte
		expired  bool
	)
	err = tx.QueryRowContext(ctx,
		`SELECT snapshot, (expires_at IS NOT NULL AND expires_at <= NOW())
		 FROM quiz_attempt_drafts WHERE quiz_id = $1 AND student_id = $2`,
		quizID, studentID,
	).Scan(&snapshot, &expired)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if err := tx.QueryRowContext(ctx,
			`SELECT `+snapshotExpr+` FROM quizzes z WHERE z.id = $1`, quizID,
		).Scan(&snapshot); err != nil {
			return nil, fmt.Errorf("freeze quiz at submission: %w", err)
		}
	case err != nil:
		return nil, fmt.Errorf("read attempt draft: %w", err)
	case expired:
		return nil, fmt.Errorf("attempt for quiz %s expired: %w", quizID, domain.ErrConflict)
	}

	score, err := gradeAgainstSnapshot(ctx, tx, snapshot, answers)
	if err != nil {
		return nil, err
	}
	passed := score >= passingScore

	encoded, err := json.Marshal(answers)
	if err != nil {
		return nil, fmt.Errorf("encode answers: %w", err)
	}

	submission := &domain.QuizSubmission{
		QuizID: quizID, StudentID: studentID, Answers: answers,
		Attempt: used + 1, Score: score, Passed: passed,
	}
	err = tx.QueryRowContext(ctx, `
		INSERT INTO quiz_submissions (quiz_id, student_id, answers, score, passed, attempt, quiz_snapshot)
		VALUES ($1, $2, $3::jsonb, $4, $5, $6, $7::jsonb)
		RETURNING id, submitted_at`,
		quizID, studentID, encoded, score, passed, submission.Attempt, snapshot,
	).Scan(&submission.ID, &submission.SubmittedAt)
	if isUniqueViolation(err) {
		// Otro envio del mismo estudiante gano la carrera y se quedo con este
		// numero de intento. Es el caso que la restriccion existe para cubrir:
		// contar y despues insertar deja una ventana, y aqui dos peticiones
		// cayeron dentro de ella.
		return nil, fmt.Errorf("concurrent submission for attempt %d: %w", submission.Attempt, domain.ErrConflict)
	}
	if err != nil {
		return nil, fmt.Errorf("record submission: %w", err)
	}

	// El intento ya se consumio: su borrador no debe sobrevivir al envio, o la
	// siguiente lectura ofreceria un intento en curso que ya esta calificado.
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM quiz_attempt_drafts WHERE quiz_id = $1 AND student_id = $2`,
		quizID, studentID); err != nil {
		return nil, fmt.Errorf("close attempt draft: %w", err)
	}

	if entry != nil {
		entry.Action = domain.AuditActionQuizSubmitted
		if err := insertAuditEntry(ctx, tx, entry); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit submission: %w", err)
	}

	return &domain.QuizGrade{
		Submission:        submission,
		AttemptsUsed:      submission.Attempt,
		AttemptsRemaining: maxAttempts - submission.Attempt,
	}, nil
}

// gradeAgainstSnapshot calcula el porcentaje de aciertos contra el cuestionario
// congelado, recorriendo el JSONB dentro de la misma transaccion.
//
// Sigue valiendo lo que decia la version que leia las tablas vivas: la clave no
// sale de esta funcion ni en el valor de retorno ni en el error. Lo unico que
// se devuelve es un numero.
//
// Calificar contra el snapshot y no contra las tablas es lo que hace
// reproducible la nota. Tambien cierra de paso el agujero que la version
// anterior cerraba con un JOIN: una opcion correcta de otro quiz no puede sumar
// punto, porque sencillamente no esta en este snapshot.
func gradeAgainstSnapshot(ctx context.Context, tx *sql.Tx, snapshot []byte, answers []domain.Answer) (int, error) {
	selected := make([]string, 0, len(answers))
	for _, a := range answers {
		selected = append(selected, a.SelectedOptionID)
	}

	var total, correct int
	err := tx.QueryRowContext(ctx, `
		SELECT
			jsonb_array_length($1::jsonb -> 'questions'),
			COALESCE((
				SELECT COUNT(DISTINCT q ->> 'id')
				FROM jsonb_array_elements($1::jsonb -> 'questions') AS q,
				     jsonb_array_elements(q -> 'options') AS o
				WHERE (o ->> 'is_correct')::boolean
				  AND (o ->> 'id') = ANY($2::text[])
			), 0)`,
		snapshot, pq.Array(selected),
	).Scan(&total, &correct)
	if err != nil {
		return 0, fmt.Errorf("grade answers: %w", err)
	}

	if total == 0 {
		// Un quiz sin preguntas no se aprueba por omision. Calificar con 100 lo
		// convertiria en un aprobado automatico, que es lo contrario de evaluar.
		return 0, nil
	}

	// Redondeo al entero mas cercano, que es lo que admite la columna. Con 3
	// preguntas y 2 aciertos son 67, no 66.
	return (correct*200 + total) / (total * 2), nil
}

func (r *QuizRepository) ListSubmissions(ctx context.Context, studentID, quizID string) ([]*domain.QuizSubmission, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, quiz_id, student_id, answers, score, passed, attempt, submitted_at
		FROM quiz_submissions
		WHERE student_id = $1 AND quiz_id = $2
		ORDER BY attempt DESC`, studentID, quizID)
	if err != nil {
		return nil, fmt.Errorf("list submissions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []*domain.QuizSubmission
	for rows.Next() {
		s := &domain.QuizSubmission{}
		var encoded []byte
		if err := rows.Scan(&s.ID, &s.QuizID, &s.StudentID, &encoded, &s.Score, &s.Passed, &s.Attempt, &s.SubmittedAt); err != nil {
			return nil, fmt.Errorf("scan submission: %w", err)
		}
		if err := json.Unmarshal(encoded, &s.Answers); err != nil {
			return nil, fmt.Errorf("decode answers: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// quizResourceID resuelve a que recurso pertenece un quiz, para poder aplicar
// la inmutabilidad del curso a las escrituras de autoria.
func quizResourceID(ctx context.Context, q querier, quizID string) (string, error) {
	var resourceID string
	err := q.QueryRowContext(ctx, `SELECT resource_id FROM quizzes WHERE id = $1`, quizID).Scan(&resourceID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", domain.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("resolve quiz resource: %w", err)
	}
	return resourceID, nil
}

// isUniqueViolation reconoce el codigo 23505 de PostgreSQL.
func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505"
}

// ---- intentos en curso --------------------------------------------------
//
// El cuestionario congelado se arma **en SQL**, con jsonb_build_object, y no
// leyendolo a Go para serializarlo despues. La razon es la misma que da la
// cabecera de este fichero: la clave de respuestas vive dentro del snapshot y
// no tiene por que pasar por la memoria del proceso para llegar a su columna.

const snapshotExpr = `
	jsonb_build_object(
		'quiz_id', z.id,
		'title', z.title,
		'passing_score', z.passing_score,
		'questions', COALESCE((
			SELECT jsonb_agg(jsonb_build_object(
				'id', q.id,
				'prompt', q.prompt,
				'position', q.position,
				'options', COALESCE((
					SELECT jsonb_agg(jsonb_build_object(
						'id', o.id, 'text', o.text,
						'position', o.position, 'is_correct', o.is_correct
					) ORDER BY o.position)
					FROM quiz_options o WHERE o.question_id = q.id
				), '[]'::jsonb)
			) ORDER BY q.position)
			FROM quiz_questions q WHERE q.quiz_id = z.id
		), '[]'::jsonb)
	)`

// snapshotRow es la forma del JSONB congelado. Vive aqui y no en domain porque
// es un detalle de como se guarda, no del dominio.
type snapshotRow struct {
	QuizID       string `json:"quiz_id"`
	Title        string `json:"title"`
	PassingScore int    `json:"passing_score"`
	Questions    []struct {
		ID       string `json:"id"`
		Prompt   string `json:"prompt"`
		Position int    `json:"position"`
		Options  []struct {
			ID       string `json:"id"`
			Text     string `json:"text"`
			Position int    `json:"position"`
			// is_correct esta en el JSON, pero no se declara aqui: lo que no se
			// decodifica no se puede filtrar por descuido mas arriba.
		} `json:"options"`
	} `json:"questions"`
}

// toQuiz construye el quiz que ve el estudiante. IsCorrect queda en false
// siempre, porque el campo ni siquiera se lee del JSON.
func (s snapshotRow) toQuiz() *domain.Quiz {
	quiz := &domain.Quiz{ID: s.QuizID, Title: s.Title, PassingScore: s.PassingScore}
	for _, q := range s.Questions {
		question := domain.Question{ID: q.ID, QuizID: s.QuizID, Prompt: q.Prompt, Position: q.Position}
		for _, o := range q.Options {
			question.Options = append(question.Options, domain.QuestionOption{
				ID: o.ID, Text: o.Text, Position: o.Position,
			})
		}
		quiz.Questions = append(quiz.Questions, question)
	}
	return quiz
}

func scanAttempt(scan func(...any) error) (*domain.QuizAttempt, error) {
	var (
		attempt           domain.QuizAttempt
		rawSnap, rawAnsws []byte
		expires           sql.NullTime
	)
	if err := scan(&attempt.QuizID, &attempt.StudentID, &attempt.Attempt,
		&rawSnap, &rawAnsws, &attempt.StartedAt, &attempt.UpdatedAt, &expires); err != nil {
		return nil, err
	}
	var snap snapshotRow
	if err := json.Unmarshal(rawSnap, &snap); err != nil {
		return nil, fmt.Errorf("decode attempt snapshot: %w", err)
	}
	attempt.Quiz = snap.toQuiz()
	if err := json.Unmarshal(rawAnsws, &attempt.Answers); err != nil {
		return nil, fmt.Errorf("decode attempt answers: %w", err)
	}
	if attempt.Answers == nil {
		attempt.Answers = []domain.Answer{}
	}
	if expires.Valid {
		attempt.ExpiresAt = &expires.Time
	}
	return &attempt, nil
}

const attemptColumns = `quiz_id, student_id, attempt, snapshot, answers, started_at, updated_at, expires_at`

func (r *QuizRepository) StartAttempt(
	ctx context.Context, quizID, studentID string, entry *domain.AuditEntry,
) (*domain.QuizAttempt, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Un intento ya abierto se devuelve tal cual. Volver a congelar el
	// cuestionario a mitad de camino seria cambiarle las preguntas al
	// estudiante, que es justo lo que esto viene a impedir.
	existing, err := scanAttempt(tx.QueryRowContext(ctx,
		`SELECT `+attemptColumns+` FROM quiz_attempt_drafts
		 WHERE quiz_id = $1 AND student_id = $2`, quizID, studentID).Scan)
	if err == nil {
		return existing, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("read attempt draft: %w", err)
	}

	var maxAttempts int
	err = tx.QueryRowContext(ctx,
		`SELECT max_attempts FROM quizzes WHERE id = $1`, quizID).Scan(&maxAttempts)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read quiz policy: %w", err)
	}

	var used int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM quiz_submissions WHERE quiz_id = $1 AND student_id = $2`,
		quizID, studentID).Scan(&used); err != nil {
		return nil, fmt.Errorf("count attempts: %w", err)
	}
	if used >= maxAttempts {
		return nil, fmt.Errorf("student %s used all %d attempts: %w",
			studentID, maxAttempts, domain.ErrConflict)
	}

	created, err := scanAttempt(tx.QueryRowContext(ctx, `
		INSERT INTO quiz_attempt_drafts (quiz_id, student_id, attempt, snapshot, expires_at)
		SELECT z.id, $2, $3, `+snapshotExpr+`,
		       CASE WHEN z.time_limit_seconds IS NULL THEN NULL
		            ELSE NOW() + make_interval(secs => z.time_limit_seconds) END
		FROM quizzes z WHERE z.id = $1
		RETURNING `+attemptColumns,
		quizID, studentID, used+1).Scan)
	if isUniqueViolation(err) {
		// Dos peticiones de inicio a la vez: la que pierde la carrera informa
		// conflicto en lugar de abrir un segundo intento.
		return nil, fmt.Errorf("concurrent attempt start: %w", domain.ErrConflict)
	}
	if err != nil {
		return nil, fmt.Errorf("open attempt draft: %w", err)
	}

	if entry != nil {
		entry.Action = domain.AuditActionQuizSubmitted
		if err := insertAuditEntry(ctx, tx, entry); err != nil {
			return nil, err
		}
	}
	return created, tx.Commit()
}

func (r *QuizRepository) GetAttempt(ctx context.Context, quizID, studentID string) (*domain.QuizAttempt, error) {
	attempt, err := scanAttempt(r.db.QueryRowContext(ctx,
		`SELECT `+attemptColumns+` FROM quiz_attempt_drafts
		 WHERE quiz_id = $1 AND student_id = $2`, quizID, studentID).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get attempt draft: %w", err)
	}
	return attempt, nil
}

func (r *QuizRepository) SaveAttemptAnswers(
	ctx context.Context, quizID, studentID string, answers []domain.Answer,
) (*domain.QuizAttempt, error) {
	if answers == nil {
		answers = []domain.Answer{}
	}
	encoded, err := json.Marshal(answers)
	if err != nil {
		return nil, fmt.Errorf("encode answers: %w", err)
	}

	// El plazo se comprueba en el UPDATE y no antes: entre leer y escribir cabe
	// justo el instante en que el intento caduca.
	saved, err := scanAttempt(r.db.QueryRowContext(ctx, `
		UPDATE quiz_attempt_drafts
		SET answers = $3::jsonb, updated_at = NOW()
		WHERE quiz_id = $1 AND student_id = $2
		  AND (expires_at IS NULL OR expires_at > NOW())
		RETURNING `+attemptColumns,
		quizID, studentID, encoded).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		// O no hay borrador, o lo hay pero expiro. Distinguirlo importa: lo
		// primero es un error del cliente y lo segundo una regla de negocio.
		var exists bool
		if qerr := r.db.QueryRowContext(ctx,
			`SELECT TRUE FROM quiz_attempt_drafts WHERE quiz_id = $1 AND student_id = $2`,
			quizID, studentID).Scan(&exists); qerr == nil {
			return nil, fmt.Errorf("attempt for quiz %s expired: %w", quizID, domain.ErrConflict)
		}
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("save attempt answers: %w", err)
	}
	return saved, nil
}
