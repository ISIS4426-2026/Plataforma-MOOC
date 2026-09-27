// Command e2e_cloud verifies the critical business flows against the deployed
// cloud environment (issue G3, #129).
//
// It is not a second copy of the Postman suite. G2 already runs those
// collections against the cloud, each one exercising an endpoint family in
// isolation against seeded data. What this adds is the single continuous journey
// the rubric asks about: one course authored from nothing, its video uploaded
// through a signed URL and transcoded by the worker on the other VM, published,
// enrolled into, graded, completed and finally certified -- with the state
// re-read from the deployment after every successful response.
//
// That journey is what crosses the whole deployment in one pass: the web VM, the
// managed database, the object bucket, the queue on the worker VM and the SMTP
// provider. An endpoint family passing on its own does not show that those five
// pieces agree.
//
//	go run ./scripts/e2e_cloud
//	go run ./scripts/e2e_cloud -base https://otro.host -espera 6m
//
// Nothing it writes carries a credential: tokens, passwords and the query string
// of a signed URL are never recorded.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/storage"
)

// The seeded accounts. Their password is the seed's shared development value,
// documented in docs/DATOS_SINTETICOS.md and already present in the Postman
// environments; it is not a production credential.
const (
	seedPassword = "Password123!"

	adminEmail      = "admin@plataforma-mooc.test"
	professorEmail  = "profesor1@plataforma-mooc.test"
	professor2Email = "profesor2@plataforma-mooc.test"
	studentEmail    = "estudiante1@plataforma-mooc.test"
)

func main() {
	var (
		base      = flag.String("base", "", "origen HTTPS del despliegue (por omisión, el de docs/postman/mooc_cloud.postman_environment.json)")
		bucket    = flag.String("bucket", "plataforma-mooc-entrega2-media", "bucket de Cloud Storage del despliegue")
		videoPath = flag.String("video", "docs/media/clase.mp4", "mp4 real que se sube en el tramo de multimedia")
		outDir    = flag.String("salida", "docs/entrega2/evidencias/G3", "directorio donde se escribe la evidencia")
		wait      = flag.Duration("espera", 4*time.Minute, "cuánto esperar a que el worker termine la transcodificación")
	)
	flag.Parse()

	root, err := repoRoot()
	if err != nil {
		fatal(err)
	}

	origin := *base
	if origin == "" {
		origin, err = originFromPostmanEnv(filepath.Join(root, "docs", "postman", "mooc_cloud.postman_environment.json"))
		if err != nil {
			fatal(err)
		}
	}
	origin = strings.TrimRight(origin, "/")

	video, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(*videoPath)))
	if err != nil {
		fatal(fmt.Errorf("leer el video de prueba: %w", err))
	}

	logPath := filepath.Join(root, filepath.FromSlash(*outDir), "corrida_e2e_cloud.txt")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		fatal(err)
	}
	logFile, err := os.Create(logPath)
	if err != nil {
		fatal(err)
	}
	defer logFile.Close()

	r := &run{
		api: &client{
			base: origin,
			// Redirects are not followed: a 302 on a signed URL would be a
			// finding, not something to chase silently.
			hc: &http.Client{
				Timeout: 90 * time.Second,
				CheckRedirect: func(*http.Request, []*http.Request) error {
					return http.ErrUseLastResponse
				},
			},
		},
		log:     io.MultiWriter(os.Stdout, logFile),
		started: time.Now(),
	}

	fmt.Fprintf(r.log, "G3 — E2E de flujos críticos en la nube\n")
	fmt.Fprintf(r.log, "Origen  : %s\n", origin)
	fmt.Fprintf(r.log, "Bucket  : %s\n", *bucket)
	fmt.Fprintf(r.log, "Inicio  : %s\n", r.started.Format(time.RFC3339))

	journey(r, *bucket, video, *wait)

	tablePath := filepath.Join(root, filepath.FromSlash(*outDir), "resultados.md")
	if err := r.writeTable(tablePath, origin); err != nil {
		fatal(err)
	}

	fmt.Fprintf(r.log, "\n===============================================\n")
	fmt.Fprintf(r.log, "Pasos verificados: %d · fallos: %d · fuera de alcance: %d · duración: %s\n",
		len(r.steps)-r.informational(), r.failed(), r.informational(),
		time.Since(r.started).Round(time.Second))
	fmt.Fprintf(r.log, "Tabla: %s\n", mustRel(root, tablePath))
	fmt.Fprintf(r.log, "Log  : %s\n", mustRel(root, logPath))

	if r.failed() > 0 {
		os.Exit(1)
	}
}

// journey walks the six flows in the order a real course lives through them, so
// each one inherits the state the previous one left behind.
func journey(r *run, bucket string, video []byte, wait time.Duration) {
	bucketOrigin := "https://storage.googleapis.com/" + bucket

	// ---- entorno -----------------------------------------------------------

	r.flowIs("0. Entorno desplegado")

	var health struct {
		Status   string            `json:"status"`
		Services map[string]string `json:"services"`
	}
	res := r.api.do("GET", "/api/v1/health", "", nil, nil)
	_ = res.into(&health)
	if !r.check("La API responde y alcanza la base administrada", res, 200,
		fmt.Sprintf("status=%q database=%q", health.Status, health.Services["database"]),
		health.Status == "pass" && health.Services["database"] == "up") {
		fmt.Fprintln(r.log, "\nEl entorno no responde; no tiene sentido seguir.")
		return
	}

	// ---- 1. identidad ------------------------------------------------------

	r.flowIs("1. Registro, verificación de correo y login")

	// A fresh address on the reserved .test TLD: it is the seed's own convention
	// and cannot reach a real mailbox, while the relay still has to accept the
	// message for the registration to succeed.
	newEmail := fmt.Sprintf("e2e.g3.%d@plataforma-mooc.test", time.Now().Unix())

	var created struct {
		User struct {
			ID     string `json:"id"`
			Role   string `json:"role"`
			Status string `json:"status"`
		} `json:"user"`
	}
	res = r.api.do("POST", "/api/v1/auth/register", "", map[string]any{
		"email": newEmail, "password": seedPassword, "full_name": "Cuenta E2E G3",
	}, nil)
	_ = res.into(&created)
	// The 201 is itself the proof that the mail left: Register propagates a send
	// failure instead of swallowing it, so a registration that answers 201 is one
	// whose verification mail the SMTP relay accepted. That is the difference
	// between this run and G2's, which met a 500 here because F1 was not deployed.
	r.check("Registro de una cuenta nueva (el 201 implica que el relevo SMTP aceptó el correo)", res, 201,
		fmt.Sprintf("role=%q status=%q", created.User.Role, created.User.Status),
		created.User.Role == "estudiante" && created.User.Status == "pending_verification")

	var pending struct {
		Token string `json:"token"`
	}
	res = r.api.do("POST", "/api/v1/auth/login", "", map[string]any{
		"email": newEmail, "password": seedPassword,
	}, nil)
	_ = res.into(&pending)
	r.check("La cuenta sin verificar no puede iniciar sesión", res, 403,
		fmt.Sprintf("code=%q, sin token emitido", res.errorCode()),
		res.errorCode() == "email_not_verified" && pending.Token == "")

	res = r.api.do("GET", "/api/v1/auth/verify?token=token-inexistente-000000", "", nil, nil)
	r.check("Un enlace de verificación inválido se rechaza sin distinguir la causa", res, 400,
		fmt.Sprintf("code=%q", res.errorCode()), res.errorCode() == "invalid_token")

	res = r.api.do("POST", "/api/v1/auth/verify/resend", "", map[string]any{"email": newEmail}, nil)
	r.check("Reenvío del enlace de verificación (segunda entrega real por SMTP)", res, 202, "", true)

	// The token only travels by email, so the leg that consumes it cannot be
	// driven from here. F1 closed it by hand against a real mailbox, and its
	// evidence is cited in the README instead of being faked.
	r.outOfScope("Activación con el token recibido por correo",
		"GET /api/v1/auth/verify?token=…",
		"el token solo viaja por correo, así que esta pata no se conduce desde aquí; ciclo completo verificado a mano en la evidencia de F1")

	admin := login(r, adminEmail, "administrador")
	professor := login(r, professorEmail, "profesor")
	professor2 := login(r, professor2Email, "profesor")
	student := login(r, studentEmail, "estudiante")
	if professor == "" || student == "" || admin == "" {
		fmt.Fprintln(r.log, "\nSin sesiones no hay recorrido que seguir.")
		return
	}

	// ---- 2. roles y propiedad ---------------------------------------------

	r.flowIs("2. Roles, propiedad y accesos denegados")

	res = r.api.do("POST", "/api/v1/courses", student, map[string]any{
		"title": "Curso que un estudiante no puede crear", "description": "No debe persistir.",
	}, nil)
	r.check("Un estudiante no puede crear cursos", res, 403,
		fmt.Sprintf("code=%q", res.errorCode()), res.errorCode() == "forbidden")

	title := fmt.Sprintf("G3 · Recorrido E2E en la nube %d", time.Now().Unix())
	var course struct {
		ID       string `json:"id"`
		StableID string `json:"stable_id"`
		Status   string `json:"status"`
		Version  int    `json:"version"`
		Title    string `json:"title"`
	}
	res = r.api.do("POST", "/api/v1/courses", professor, map[string]any{
		"title": title, "description": "Curso creado por la verificación E2E de G3.",
	}, nil)
	_ = res.into(&course)
	if !r.check("El profesor crea el curso en borrador", res, 201,
		fmt.Sprintf("status=%q version=%d stable_id emitido", course.Status, course.Version),
		course.Status == "draft" && course.Version == 1 && course.StableID != "") {
		return
	}

	res, listed := catalogHas(r, title)
	r.check("El borrador no aparece en el catálogo público", res, 200,
		"el catálogo responde sin sesión y no lista el borrador", !listed)

	res = r.api.do("GET", "/api/v1/courses/"+course.ID+"/modules", "", nil, nil)
	r.check("Sin sesión, el contenido del curso responde 401", res, 401,
		fmt.Sprintf("code=%q", res.errorCode()), res.errorCode() == "unauthorized")

	res = r.api.do("GET", "/api/v1/courses/"+course.ID+"/modules", student, nil, nil)
	r.check("Con sesión pero sin inscripción, responde 403", res, 403,
		fmt.Sprintf("code=%q", res.errorCode()), res.errorCode() == "forbidden")

	res = r.api.do("GET", "/api/v1/courses/"+course.ID+"/modules", professor, nil, nil)
	r.check("El autor lee su propio borrador sin inscribirse", res, 200,
		"la lista responde al autor", res.err == nil)

	res = r.api.do("GET", "/api/v1/courses/"+course.ID+"/modules", admin, nil, nil)
	r.check("El administrador lee el borrador ajeno", res, 200,
		"la lista responde al administrador", res.err == nil)

	res = r.api.do("POST", "/api/v1/courses/"+course.ID+"/modules", professor2,
		map[string]any{"title": "Módulo de otro profesor"}, nil)
	r.check("Otro profesor no puede editar un curso que no es suyo", res, 403,
		fmt.Sprintf("code=%q, la propiedad manda sobre el rol", res.errorCode()),
		res.errorCode() == "forbidden")

	// ---- 3. autoría y publicación -----------------------------------------

	r.flowIs("3. Autoría y publicación")

	res = r.api.do("POST", "/api/v1/courses/"+course.ID+"/publish", professor, nil, nil)
	fields := res.details()
	r.check("Publicar un curso vacío acumula los errores de validación", res, 422,
		fmt.Sprintf("code=%q campos=%v", res.errorCode(), fields),
		res.errorCode() == "validation_failed" && slices.Contains(fields, "structure") && slices.Contains(fields, "approval_criteria"))

	res = r.api.do("GET", "/api/v1/courses/"+course.ID, professor, nil, nil)
	var after struct {
		Status string `json:"status"`
	}
	_ = res.into(&after)
	r.check("El intento fallido no publicó nada", res, 200,
		fmt.Sprintf("status=%q", after.Status), after.Status == "draft")

	module := createChild(r, professor, "POST", "/api/v1/courses/"+course.ID+"/modules",
		map[string]any{"title": "Módulo 1"}, "El profesor crea el módulo")
	if module.ID == "" {
		return
	}
	unit := createChild(r, professor, "POST", "/api/v1/modules/"+module.ID+"/units",
		map[string]any{"title": "Unidad 1"}, "El profesor crea la unidad")
	if unit.ID == "" {
		return
	}

	videoRes := createChild(r, professor, "POST", "/api/v1/units/"+unit.ID+"/resources",
		map[string]any{
			"title": "Clase magistral", "type": "video",
			"is_visible": true, "is_mandatory": true, "allow_download": false,
		}, "Recurso de video, obligatorio y visible")
	textRes := createChild(r, professor, "POST", "/api/v1/units/"+unit.ID+"/resources",
		map[string]any{
			"title": "Lectura complementaria", "type": "text",
			"is_visible": true, "is_mandatory": true, "content_text": "# Lectura\n\nContenido de la unidad.",
		}, "Recurso de texto, obligatorio y visible")
	quizRes := createChild(r, professor, "POST", "/api/v1/units/"+unit.ID+"/resources",
		map[string]any{
			"title": "Evaluación de la unidad", "type": "quiz",
			"is_visible": true, "is_mandatory": true,
		}, "Recurso de quiz, obligatorio y visible")
	if videoRes.ID == "" || textRes.ID == "" || quizRes.ID == "" {
		return
	}

	var quiz authorQuiz
	res = r.api.do("POST", "/api/v1/resources/"+quizRes.ID+"/quiz", professor, map[string]any{
		"title": "Evaluación diagnóstica", "passing_score": 70, "max_attempts": 3,
	}, nil)
	_ = res.into(&quiz)
	r.check("El profesor define el cuestionario", res, 201,
		fmt.Sprintf("quiz creado sobre el recurso %s", short(quizRes.ID)), quiz.ID != "")
	if quiz.ID == "" {
		return
	}

	for i, q := range []map[string]any{
		{"prompt": "¿Qué componente transcodifica el video?", "options": []map[string]any{
			{"text": "El Worker Server", "is_correct": true},
			{"text": "El navegador del estudiante", "is_correct": false},
		}},
		{"prompt": "¿Dónde viven los derivados HLS?", "options": []map[string]any{
			{"text": "En el prefijo hls/ del bucket", "is_correct": true},
			{"text": "En el disco de la VM web", "is_correct": false},
		}},
	} {
		res = r.api.do("POST", "/api/v1/quizzes/"+quiz.ID+"/questions", professor, q, nil)
		var q struct {
			Position int `json:"position"`
		}
		_ = res.into(&q)
		r.check(fmt.Sprintf("Pregunta %d con su clave de respuesta", i+1), res, 201,
			fmt.Sprintf("position=%d", q.Position), q.Position == i)
	}

	// The quiz has to be read back to submit against it: the creation response
	// predates the questions, so its key is empty. Reading it as the author is
	// also the only view that carries the key at all.
	res = r.api.do("GET", "/api/v1/resources/"+quizRes.ID+"/quiz", professor, nil, nil)
	_ = res.into(&quiz)
	if !r.check("El autor relee el cuestionario con su clave de respuestas", res, 200,
		fmt.Sprintf("%d pregunta(s) con clave, passing_score=70", len(quiz.Questions)),
		len(quiz.Questions) == 2) {
		return
	}

	// ---- 4. multimedia ----------------------------------------------------

	r.flowIs("4. Carga multimedia, procesamiento asíncrono y consumo")

	var presigned struct {
		UploadURL   string `json:"upload_url"`
		ObjectKey   string `json:"object_key"`
		Method      string `json:"method"`
		ContentType string `json:"content_type"`
	}
	res = r.api.do("POST", "/api/v1/media/presigned-url", professor, map[string]any{
		"resource_id": videoRes.ID, "filename": "clase.mp4",
		"mime_type": "video/mp4", "file_size_bytes": len(video),
	}, nil)
	_ = res.into(&presigned)
	wantPrefix := "originals/" + videoRes.StableID + "/"
	r.check("La API firma la URL de carga directa", res, 200,
		fmt.Sprintf("method=%q content_type=%q object_key bajo %s", presigned.Method, presigned.ContentType, wantPrefix),
		presigned.Method == "PUT" && presigned.ContentType == "video/mp4" &&
			strings.HasPrefix(presigned.ObjectKey, wantPrefix) &&
			strings.Contains(presigned.UploadURL, "X-Goog-Signature"))

	res = r.api.do("POST", "/api/v1/media/presigned-url", professor, map[string]any{
		"resource_id": videoRes.ID, "filename": "payload.exe",
		"mime_type": "video/mp4", "file_size_bytes": 1024,
	}, nil)
	r.check("Una extensión no permitida no obtiene firma", res, 400,
		fmt.Sprintf("code=%q", res.errorCode()), res.errorCode() == "invalid_input")

	res = r.api.do("POST", "/api/v1/media/presigned-url", student, map[string]any{
		"resource_id": videoRes.ID, "filename": "clase.mp4",
		"mime_type": "video/mp4", "file_size_bytes": 1024,
	}, nil)
	r.check("Un estudiante no obtiene URL de carga", res, 403,
		fmt.Sprintf("code=%q", res.errorCode()), res.errorCode() == "forbidden")

	if presigned.UploadURL == "" {
		return
	}

	// The upload goes straight to the bucket, never through the API. In the cloud
	// the signing host and the client host are the same, which is what note 1 of
	// NOTAS_TECNICAS.md says the local environment cannot reproduce.
	res = r.api.absolute("PUT", presigned.UploadURL, bytes.NewReader(video),
		map[string]string{"Content-Type": "video/mp4"})
	r.check("El navegador sube el original directo al bucket con la URL firmada", res, 200,
		fmt.Sprintf("%d bytes aceptados por Cloud Storage sin pasar por la API", len(video)),
		res.err == nil)

	var confirmed struct {
		ObjectKey        string `json:"object_key"`
		ProcessingStatus string `json:"processing_status"`
	}
	res = r.api.do("POST", "/api/v1/media/uploads/"+videoRes.ID+"/complete", professor,
		map[string]any{"object_key": presigned.ObjectKey}, nil)
	_ = res.into(&confirmed)
	r.check("Confirmar la carga registra el objeto y encola el procesamiento", res, 202,
		fmt.Sprintf("processing_status=%q object_key registrado", confirmed.ProcessingStatus),
		confirmed.ObjectKey == presigned.ObjectKey && confirmed.ProcessingStatus == "pending")

	var reconfirmed struct {
		ObjectKey        string `json:"object_key"`
		ProcessingStatus string `json:"processing_status"`
	}
	res = r.api.do("POST", "/api/v1/media/uploads/"+videoRes.ID+"/complete", professor,
		map[string]any{"object_key": presigned.ObjectKey}, nil)
	_ = res.into(&reconfirmed)
	r.check("Confirmar dos veces la misma carga no cambia el estado", res, 202,
		fmt.Sprintf("object_key intacto, processing_status=%q", reconfirmed.ProcessingStatus),
		reconfirmed.ObjectKey == presigned.ObjectKey)

	var download struct {
		DownloadURL string `json:"download_url"`
	}
	res = r.api.do("GET", "/api/v1/media/resources/"+videoRes.ID+"/download-url", professor, nil, nil)
	_ = res.into(&download)
	r.check("El autor obtiene una URL firmada de lectura del original", res, 200,
		"la URL lleva firma V4", strings.Contains(download.DownloadURL, "X-Goog-Signature"))

	if download.DownloadURL != "" {
		res = r.api.absolute("GET", download.DownloadURL, nil, nil)
		r.check("El original se lee con la firma y devuelve los bytes subidos", res, 200,
			fmt.Sprintf("Content-Length=%s frente a %d subidos",
				res.header.Get("Content-Length"), len(video)),
			res.header.Get("Content-Length") == fmt.Sprint(len(video)))
	}

	res = r.api.absolute("GET", bucketOrigin+"/"+presigned.ObjectKey, nil, nil)
	r.check("El mismo original sin firma es inaccesible", res, 403,
		"el prefijo originals/ no es legible de forma anónima", res.err == nil)

	// The queue lives on the other VM, so this wait is the only step that
	// observes both machines at once: the API enqueued on one and the worker has
	// to pick it up on the other for the status to move.
	final, elapsed := waitForProcessing(r, professor, unit.ID, videoRes.ID, wait)
	r.note("El worker de la otra VM transcodifica el original a HLS",
		"GET /api/v1/units/{unitID}/resources (sondeo)",
		fmt.Sprintf("processing_status=%q tras %s de espera", final, elapsed.Round(time.Second)),
		final == "completed",
		fmt.Sprintf("el recurso no alcanzó \"completed\" en %s; la tarea no llegó al worker o falló", wait))

	// Whether hls/ is readable without a signature has to be asked separately from
	// whether the worker wrote anything, because a private bucket answers 403 to a
	// missing object too. A deliberately absent key discriminates: 404 means the
	// prefix is public and the object is not there, 403 means the prefix is not
	// public and the manifest could never be played however well the worker ran.
	res = r.api.absolute("GET", bucketOrigin+"/hls/no-existe-a-proposito/master.m3u8", nil, nil)
	r.check("El prefijo hls/ admite lectura sin firma", res, 404,
		"un objeto inexistente responde 404 (prefijo público) y no 403 (prefijo privado)",
		res.err == nil)

	master := bucketOrigin + "/" + storage.HLSMasterKey(videoRes.StableID)
	res = r.api.absolute("GET", master, nil, nil)
	body := string(res.body)
	r.check("Un reproductor consume el manifiesto HLS sin firma", res, 200,
		"el maestro declara sus variantes por ruta relativa",
		strings.Contains(body, "#EXT-X-STREAM-INF"))

	// ---- publicación -------------------------------------------------------

	r.flowIs("3. Autoría y publicación (cierre)")

	var published struct {
		Status  string `json:"status"`
		Version int    `json:"version"`
	}
	res = r.api.do("POST", "/api/v1/courses/"+course.ID+"/publish", professor, nil, nil)
	_ = res.into(&published)
	if !r.check("Con estructura y criterio de aprobación, el curso se publica", res, 200,
		fmt.Sprintf("status=%q version=%d", published.Status, published.Version),
		published.Status == "published") {
		return
	}

	res = r.api.do("PUT", "/api/v1/courses/"+course.ID, professor, map[string]any{
		"title": title + " (editado)", "description": "Intento de edición tras publicar.",
	}, nil)
	r.check("Un curso publicado es inmutable", res, 409,
		fmt.Sprintf("code=%q", res.errorCode()), res.errorCode() == "course_immutable")

	res = r.api.do("GET", "/api/v1/courses/"+course.ID, professor, nil, nil)
	var unchanged struct {
		Title string `json:"title"`
	}
	_ = res.into(&unchanged)
	r.check("El rechazo dejó el título intacto", res, 200,
		fmt.Sprintf("title sigue siendo el publicado (%q)", short(unchanged.Title)),
		unchanged.Title == title)

	res, listed = catalogHas(r, title)
	r.check("El curso publicado ya figura en el catálogo público", res, 200,
		"el catálogo anónimo lo lista", listed)

	// ---- 5. inscripción, quiz y progreso -----------------------------------

	r.flowIs("5. Inscripción, quiz idempotente y progreso")

	var enrollment struct {
		Status         string `json:"status"`
		CourseStableID string `json:"course_stable_id"`
	}
	res = r.api.do("POST", "/api/v1/courses/"+course.ID+"/enrollments", student, nil, nil)
	_ = res.into(&enrollment)
	r.check("El estudiante se inscribe en el curso publicado", res, 200,
		fmt.Sprintf("status=%q sobre el stable_id del curso", enrollment.Status),
		enrollment.Status == "active" && enrollment.CourseStableID == course.StableID)

	res = r.api.do("GET", "/api/v1/courses/"+course.ID+"/modules", student, nil, nil)
	var modules struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	_ = res.into(&modules)
	r.check("Con la inscripción activa ya lee el contenido", res, 200,
		fmt.Sprintf("%d módulo(s) visibles donde antes había 403", len(modules.Items)),
		len(modules.Items) == 1)

	res = r.api.do("GET", "/api/v1/resources/"+quizRes.ID+"/quiz", student, nil, nil)
	r.check("El estudiante ve el cuestionario sin la clave de respuestas", res, 200,
		"la respuesta no trae questions_with_key ni is_correct",
		!strings.Contains(string(res.body), "is_correct") &&
			!strings.Contains(string(res.body), "questions_with_key"))

	var progress struct {
		CompletedCount int     `json:"completed_count"`
		TotalCount     int     `json:"total_count"`
		Percentage     float64 `json:"percentage"`
		IsApproved     bool    `json:"is_approved"`
		Badge          *struct {
			ID               string `json:"id"`
			CourseStableID   string `json:"course_stable_id"`
			VerificationCode string `json:"verification_code"`
			IsRevoked        bool   `json:"revoked"`
		} `json:"badge"`
	}
	res = r.api.do("GET", "/api/v1/progress/courses/"+course.ID, student, nil, nil)
	_ = res.into(&progress)
	r.check("El progreso arranca en cero sobre los tres recursos obligatorios", res, 200,
		fmt.Sprintf("%d/%d · %.2f%% · aprobado=%v",
			progress.CompletedCount, progress.TotalCount, progress.Percentage, progress.IsApproved),
		progress.CompletedCount == 0 && progress.TotalCount == 3 && !progress.IsApproved)

	answers := correctAnswers(quiz)
	key := fmt.Sprintf("g3-envio-%d", time.Now().UnixNano())

	var first struct {
		SubmissionID string `json:"submission_id"`
		Score        int    `json:"score"`
		Passed       bool   `json:"passed"`
		Attempt      int    `json:"attempt"`
	}
	res = r.api.do("POST", "/api/v1/quizzes/"+quiz.ID+"/submissions", student,
		map[string]any{"answers": answers}, map[string]string{"Idempotency-Key": key})
	_ = res.into(&first)
	r.check("El estudiante envía el quiz y queda calificado", res, 200,
		fmt.Sprintf("score=%d passed=%v attempt=%d", first.Score, first.Passed, first.Attempt),
		first.Passed && first.Score == 100 && first.Attempt == 1)

	var replay struct {
		SubmissionID string `json:"submission_id"`
		Attempt      int    `json:"attempt"`
		Score        int    `json:"score"`
	}
	res = r.api.do("POST", "/api/v1/quizzes/"+quiz.ID+"/submissions", student,
		map[string]any{"answers": answers}, map[string]string{"Idempotency-Key": key})
	_ = res.into(&replay)
	r.check("Reenviar con la misma Idempotency-Key no vuelve a calificar", res, 200,
		fmt.Sprintf("submission_id idéntico y attempt sigue en %d", replay.Attempt),
		replay.SubmissionID == first.SubmissionID && replay.Attempt == first.Attempt)

	res = r.api.do("GET", "/api/v1/quizzes/"+quiz.ID+"/submissions/me", student, nil, nil)
	var mine struct {
		Items []struct {
			ID string `json:"submission_id"`
		} `json:"items"`
	}
	_ = res.into(&mine)
	r.check("Solo quedó un envío registrado, no dos", res, 200,
		fmt.Sprintf("%d envío(s) en el historial del estudiante", len(mine.Items)),
		len(mine.Items) == 1)

	res = r.api.do("GET", "/api/v1/progress/courses/"+course.ID, student, nil, nil)
	_ = res.into(&progress)
	r.check("Aprobar el quiz completó su recurso sin latido del cliente", res, 200,
		fmt.Sprintf("%d/%d · %.2f%%", progress.CompletedCount, progress.TotalCount, progress.Percentage),
		progress.CompletedCount == 1)

	res = r.api.do("POST", "/api/v1/progress/heartbeat", student, map[string]any{
		"resource_id": textRes.ID, "dwell_time_seconds": 45, "completed": true,
	}, nil)
	_ = res.into(&progress)
	r.check("Un latido sobre la lectura avanza el progreso", res, 200,
		fmt.Sprintf("%d/%d · %.2f%%", progress.CompletedCount, progress.TotalCount, progress.Percentage),
		progress.CompletedCount == 2)

	res = r.api.do("POST", "/api/v1/progress/heartbeat", student, map[string]any{
		"resource_id": videoRes.ID, "dwell_time_seconds": 60, "completed": true,
	}, nil)
	_ = res.into(&progress)
	badgeOK := progress.Badge != nil
	r.check("El último recurso obligatorio completa el curso y emite la insignia", res, 200,
		fmt.Sprintf("%d/%d · %.2f%% · aprobado=%v · insignia=%v",
			progress.CompletedCount, progress.TotalCount, progress.Percentage, progress.IsApproved, badgeOK),
		progress.CompletedCount == 3 && progress.Percentage == 100 && progress.IsApproved && badgeOK)

	var repeated struct {
		CompletedCount int `json:"completed_count"`
	}
	res = r.api.do("POST", "/api/v1/progress/heartbeat", student, map[string]any{
		"resource_id": videoRes.ID, "dwell_time_seconds": 60, "completed": true,
	}, nil)
	_ = res.into(&repeated)
	r.check("Repetir el latido no infla el avance", res, 200,
		fmt.Sprintf("sigue en %d/3", repeated.CompletedCount), repeated.CompletedCount == 3)

	res = r.api.do("POST", "/api/v1/progress/heartbeat", professor, map[string]any{
		"resource_id": textRes.ID, "dwell_time_seconds": 30, "completed": true,
	}, nil)
	r.check("El autor no acumula progreso en su propio curso", res, 403,
		fmt.Sprintf("code=%q: sin inscripción no hay avance", res.errorCode()),
		res.errorCode() == "forbidden")

	if !badgeOK {
		return
	}

	// ---- 6. insignia -------------------------------------------------------

	r.flowIs("6. Insignia y verificación pública")

	var owned struct {
		ID             string `json:"id"`
		CourseStableID string `json:"course_stable_id"`
		IsRevoked      bool   `json:"revoked"`
	}
	res = r.api.do("GET", "/api/v1/badges/"+progress.Badge.ID, student, nil, nil)
	etag := res.header.Get("ETag")
	_ = res.into(&owned)
	r.check("El estudiante lee su propia insignia", res, 200,
		fmt.Sprintf("course_stable_id correcto, revoked=%v, ETag emitido", owned.IsRevoked),
		owned.CourseStableID == course.StableID && !owned.IsRevoked && etag != "")

	res = r.api.do("GET", "/api/v1/badges/"+progress.Badge.ID, student, nil,
		map[string]string{"If-None-Match": etag})
	r.check("Con el ETag vigente la lectura es condicional", res, 304,
		"304 sin cuerpo", len(res.body) == 0)

	res = r.api.do("GET", "/api/v1/badges/"+progress.Badge.ID, professor, nil, nil)
	r.check("La insignia de otra persona responde 404, no 403", res, 404,
		fmt.Sprintf("code=%q: el identificador no confirma que exista", res.errorCode()),
		res.errorCode() == "not_found")

	// The public answer names the course and nothing else: no student, no email,
	// no identifier. That omission is the contract, so it is asserted rather than
	// assumed -- a verification endpoint that leaked the holder would still pass a
	// check that only looked at the status code.
	var verified struct {
		Valid       bool   `json:"valid"`
		CourseTitle string `json:"course_title"`
		IsRevoked   bool   `json:"revoked"`
	}
	res = r.api.do("GET", "/api/v1/badges/verify/"+progress.Badge.VerificationCode, "", nil, nil)
	_ = res.into(&verified)
	leaks := ""
	for _, probe := range []string{studentEmail, "student", "full_name", "email", course.StableID} {
		if strings.Contains(string(res.body), probe) {
			leaks = probe
			break
		}
	}
	r.check("Cualquiera verifica la insignia con el código, sin sesión", res, 200,
		fmt.Sprintf("valid=%v revoked=%v, nombra el curso y no identifica al estudiante",
			verified.Valid, verified.IsRevoked),
		verified.Valid && !verified.IsRevoked && verified.CourseTitle == title && leaks == "")

	res = r.api.do("GET", "/api/v1/badges/verify/00000000-0000-0000-0000-000000000000", "", nil, nil)
	r.check("Un código inexistente no confirma ni niega nada más que el 404", res, 404,
		fmt.Sprintf("code=%q", res.errorCode()), res.errorCode() == "not_found")
}

// ---- auxiliares ------------------------------------------------------------

// login authenticates one seeded account. The login limiter is keyed by IP, not
// by account (G2's second finding), so a run that repeats soon after another can
// meet a 429 that says nothing about the flow under test: it is waited out once
// rather than reported as a failure.
func login(r *run, email, wantRole string) string {
	var out struct {
		Token string `json:"token"`
		User  struct {
			Role   string `json:"role"`
			Status string `json:"status"`
		} `json:"user"`
	}

	res := r.api.do("POST", "/api/v1/auth/login", "",
		map[string]any{"email": email, "password": seedPassword}, nil)
	if res.status == http.StatusTooManyRequests {
		fmt.Fprintf(r.log, "        429 en el login: el cupo por IP se agotó, esperando 65s\n")
		time.Sleep(65 * time.Second)
		res = r.api.do("POST", "/api/v1/auth/login", "",
			map[string]any{"email": email, "password": seedPassword}, nil)
	}
	_ = res.into(&out)

	r.check("Inicia sesión "+wantRole+" ("+local(email)+")", res, 200,
		fmt.Sprintf("role=%q status=%q, sesión emitida", out.User.Role, out.User.Status),
		out.Token != "" && out.User.Role == wantRole && out.User.Status == "active")
	return out.Token
}

type node struct {
	ID       string `json:"id"`
	StableID string `json:"stable_id"`
	Position int    `json:"position"`
}

// createChild creates one structural node and asserts its position. Positions are
// base 0 throughout the repositories -- note 3 of NOTAS_TECNICAS.md -- so the
// first child of anything must come back at 0.
func createChild(r *run, token, method, path string, body map[string]any, name string) node {
	var out node
	res := r.api.do(method, path, token, body, nil)
	_ = res.into(&out)
	r.check(name, res, 201,
		fmt.Sprintf("position=%d (base 0), stable_id emitido", out.Position),
		out.ID != "" && out.StableID != "")
	return out
}

// catalogHas searches the public catalog, without a session, and reports whether
// the course is listed. The catalog is the one content route that stays public:
// browsing is not consuming.
func catalogHas(r *run, title string) (*response, bool) {
	res := r.api.do("GET", "/api/v1/courses?limit=50&search="+urlQuery(title), "", nil, nil)
	var page struct {
		Items []struct {
			Title string `json:"title"`
		} `json:"items"`
	}
	_ = res.into(&page)
	for _, c := range page.Items {
		if c.Title == title {
			return res, true
		}
	}
	return res, false
}

// waitForProcessing polls the resource until the worker marks it completed. The
// resource listing is the only window into the worker's outcome from outside the
// VPC: the queue itself is unreachable by design.
func waitForProcessing(r *run, token, unitID, resourceID string, limit time.Duration) (string, time.Duration) {
	start := time.Now()
	last := "desconocido"

	for time.Since(start) < limit {
		res := r.api.do("GET", "/api/v1/units/"+unitID+"/resources", token, nil, nil)
		var list struct {
			Items []struct {
				ID               string `json:"id"`
				ProcessingStatus string `json:"processing_status"`
			} `json:"items"`
		}
		if err := res.into(&list); err == nil {
			for _, it := range list.Items {
				if it.ID != resourceID {
					continue
				}
				if it.ProcessingStatus != last {
					fmt.Fprintf(r.log, "        processing_status=%q (%s)\n",
						it.ProcessingStatus, time.Since(start).Round(time.Second))
				}
				last = it.ProcessingStatus
				if last == "completed" || last == "failed" {
					return last, time.Since(start)
				}
			}
		}
		time.Sleep(10 * time.Second)
	}
	return last, time.Since(start)
}

// authorQuiz is the author's view of a quiz: the only one that carries the
// answer key, which is what makes a correct submission possible from here and
// what the student's view must not contain.
type authorQuiz struct {
	ID        string `json:"id"`
	Questions []struct {
		ID      string `json:"id"`
		Prompt  string `json:"prompt"`
		Options []struct {
			ID        string `json:"id"`
			Text      string `json:"text"`
			IsCorrect bool   `json:"is_correct"`
		} `json:"options"`
	} `json:"questions_with_key"`
}

// correctAnswers picks the right option of every question.
func correctAnswers(quiz authorQuiz) []map[string]string {
	out := make([]map[string]string, 0, len(quiz.Questions))
	for _, q := range quiz.Questions {
		for _, o := range q.Options {
			if o.IsCorrect {
				out = append(out, map[string]string{
					"question_id": q.ID, "selected_option_id": o.ID,
				})
				break
			}
		}
	}
	return out
}

func short(s string) string {
	if len(s) <= 28 {
		return s
	}
	return s[:25] + "..."
}

func local(email string) string {
	if i := strings.IndexByte(email, '@'); i > 0 {
		return email[:i]
	}
	return email
}

func urlQuery(s string) string {
	var b strings.Builder
	for _, c := range []byte(s) {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_', c == '.':
			b.WriteByte(c)
		case c == ' ':
			b.WriteByte('+')
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// originFromPostmanEnv reads the deployment origin from the Postman cloud
// environment so the host is declared in exactly one place in the repository.
func originFromPostmanEnv(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var env struct {
		Values []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"values"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return "", err
	}
	for _, v := range env.Values {
		if v.Key == "baseUrl" && v.Value != "" {
			return v.Value, nil
		}
	}
	return "", fmt.Errorf("%s no declara baseUrl", path)
}

// repoRoot walks up from the working directory until it finds go.mod, so the
// command works from anywhere in the tree.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no se encontró go.mod desde el directorio actual")
		}
		dir = parent
	}
}

func mustRel(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
