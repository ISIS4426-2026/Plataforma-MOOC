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
	"cmp"
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
	student2Email   = "estudiante2@plataforma-mooc.test"
	student2ID      = "c0000000-0000-0000-0000-000000000002"
)

func main() {
	var (
		base         = flag.String("base", "", "origen HTTPS del despliegue (por omisión, el de docs/postman/mooc_cloud.postman_environment.json)")
		bucket       = flag.String("bucket", "plataforma-mooc-entrega2-media", "bucket privado de originales, documentos y miniaturas")
		hlsBucket    = flag.String("bucket-hls", "plataforma-mooc-entrega2-hls", "bucket de derivados HLS, de lectura pública (#167)")
		videoPath    = flag.String("video", "docs/media/clase.mp4", "mp4 real que se sube en el tramo de multimedia")
		docPath      = flag.String("documento", "docs/media/guia.pdf", "pdf real que se sube en el tramo de documentos")
		imgPath      = flag.String("imagen", "docs/media/diagrama.png", "imagen real que se sube en el tramo de documentos")
		// La evidencia de la Entrega 2 esta congelada: es el registro que se
		// califico. Una corrida de hoy tiene mas pasos que aquella, asi que
		// escribe en la entrega en curso y no sobre docs/entrega2/evidencias/G3.
		outDir        = flag.String("salida", "docs/entrega3/evidencias/correcciones-entrega1", "directorio donde se escribe la evidencia")
		wait          = flag.Duration("espera", 4*time.Minute, "cuánto esperar a que el worker termine la transcodificación")
		minInstancias = flag.Int("min-instancias", 1, "mínimo de instancias distintas (X-Instance-ID) que deben atender una misma sesión (2 para E1/C4)")
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

	doc, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(*docPath)))
	if err != nil {
		fatal(fmt.Errorf("leer el documento de prueba: %w", err))
	}

	img, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(*imgPath)))
	if err != nil {
		fatal(fmt.Errorf("leer la imagen de prueba: %w", err))
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

	fmt.Fprintf(r.log, "G3 / E1 — E2E de flujos críticos e integridad en la nube\n")
	fmt.Fprintf(r.log, "Origen  : %s\n", origin)
	fmt.Fprintf(r.log, "Bucket  : %s\n", *bucket)
	fmt.Fprintf(r.log, "Réplicas: mín. %d exigida(s)\n", *minInstancias)
	fmt.Fprintf(r.log, "Inicio  : %s\n", r.started.Format(time.RFC3339))

	journey(r, *bucket, *hlsBucket, video, doc, img, *wait, *minInstancias)

	tablePath := filepath.Join(root, filepath.FromSlash(*outDir), "resultados.md")
	if err := r.writeTable(tablePath, origin); err != nil {
		fatal(err)
	}

	fmt.Fprintf(r.log, "\n===============================================\n")
	if stats := r.instanceCounts(); len(stats) > 0 {
		var parts []string
		for _, st := range stats {
			parts = append(parts, fmt.Sprintf("%s:%d", st.ID, st.Count))
		}
		fmt.Fprintf(r.log, "Instancias observadas (X-Instance-ID): %s\n", strings.Join(parts, " · "))
	}
	fmt.Fprintf(r.log, "Pasos verificados: %d · fallos: %d · fuera de alcance: %d · duración: %s\n",
		len(r.steps)-r.informational(), r.failed(), r.informational(),
		time.Since(r.started).Round(time.Second))
	fmt.Fprintf(r.log, "Tabla: %s\n", mustRel(root, tablePath))
	fmt.Fprintf(r.log, "Log  : %s\n", mustRel(root, logPath))

	if r.failed() > 0 {
		os.Exit(1)
	}
}

// journey walks the flows in the order a real course lives through them, so
// each one inherits the state the previous one left behind.
func journey(r *run, bucket, hlsBucket string, video, doc, img []byte, wait time.Duration, minInstancias int) {
	bucketOrigin := "https://storage.googleapis.com/" + bucket
	// Los derivados viven en su propio bucket, que es el unico publico (#167).
	hlsOrigin := "https://storage.googleapis.com/" + hlsBucket

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

	// Las dos sondas que miraran el balanceador y el grupo administrado. Se
	// comprueban aqui, antes que nada, porque de ellas depende que una replica
	// reciba trafico y que no la recreen por error.
	res = r.api.do("GET", "/api/v1/live", "", nil, nil)
	r.check("La sonda de liveness responde sin consultar dependencias", res, 200,
		"200 mientras el proceso sirva, para que un parpadeo de la base no recree las réplicas",
		res.err == nil)

	res = r.api.do("GET", "/api/v1/ready", "", nil, nil)
	r.check("La sonda de readiness declara que la réplica puede atender", res, 200,
		"200 con la base alcanzable", res.err == nil)

	// El criterio de aceptación de A2: la sonda no revela dependencias,
	// versiones ni topología. Es la respuesta que cualquiera puede pedir al
	// balanceador sin autenticarse.
	cuerpoSonda := strings.ToLower(string(res.body))
	filtra := ""
	for _, palabra := range []string{"database", "postgres", "redis", "bucket", "version", "host", "addr"} {
		if strings.Contains(cuerpoSonda, palabra) {
			filtra = palabra
		}
	}
	r.check("La sonda pública no revela dependencias ni topología", res, 200,
		fmt.Sprintf("cuerpo=%s", strings.TrimSpace(string(res.body))), filtra == "")

	// Sin este identificador no hay forma de demostrar que el balanceador
	// reparte entre réplicas, que es lo que piden C4 y el escenario 3A.
	instancia := res.header.Get("X-Instance-ID")
	r.check("Cada respuesta dice qué réplica la atendió", res, 200,
		fmt.Sprintf("X-Instance-ID=%q", instancia),
		instancia != "")

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
	videoCompleteInst := res.instanceID()
	videoCompleteReq := res.requestID()
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

	// Whether the derivatives are readable without a signature has to be asked
	// separately from whether the worker wrote anything, because a private bucket
	// answers 403 to a missing object too. A deliberately absent key
	// discriminates: 404 means the bucket is public and the object is not there,
	// 403 means it is not public and the manifest could never be played however
	// well the worker ran.
	res = r.api.absolute("GET", hlsOrigin+"/hls/no-existe-a-proposito/master.m3u8", nil, nil)
	r.check("El bucket de derivados admite lectura sin firma", res, 404,
		"un objeto inexistente responde 404 (bucket público) y no 403 (bucket privado)",
		res.err == nil)

	master := hlsOrigin + "/" + storage.HLSMasterKey(videoRes.StableID)
	res = r.api.absolute("GET", master, nil, nil)
	body := string(res.body)
	hlsOK := res.err == nil && res.status == 200 && strings.Contains(body, "#EXT-X-STREAM-INF")
	r.check("Un reproductor consume el manifiesto HLS sin firma", res, 200,
		"el maestro declara sus variantes por ruta relativa",
		strings.Contains(body, "#EXT-X-STREAM-INF"))

	r.recordJob(mediaJob{
		ResourceID:  videoRes.ID,
		StableID:    videoRes.StableID,
		Kind:        "video",
		ObjectKey:   presigned.ObjectKey,
		Instance:    videoCompleteInst,
		RequestID:   videoCompleteReq,
		FinalStatus: final,
		Elapsed:     elapsed,
		Derivatives: fmt.Sprintf("HLS %s (200=%v)", storage.HLSMasterKey(videoRes.StableID), hlsOK),
		Reconciled:  final == "completed" && hlsOK,
	})

	// ---- documentos e imágenes ---------------------------------------------
	//
	// Este tramo no existía, y esa ausencia es la que dejó pasar el defecto que
	// encontró la revisión de la Entrega 1: el worker enrutaba cualquier recurso
	// a la canalización HLS, así que a ffprobe se le pedía encontrar un flujo de
	// vídeo dentro de un PDF. Respondía «file carries no video or audio stream»,
	// que es un fallo permanente, y el recurso quedaba en failed para siempre.
	//
	// Toda la cobertura previa subía sólo mp4, así que el camino de documentos
	// nunca se ejercitó. Estas comprobaciones son las que faltaban.

	r.flowIs("4b. Documentos e imágenes sin transcodificación")

	for _, d := range []struct {
		nombre    string
		tipo      string
		fichero   string
		mime      string
		bytes     []byte
		prefijo   string
		etiqueta  string
		descargar bool
	}{
		{"Guía de la unidad", "pdf", "guia.pdf", "application/pdf", doc, "documents/", "PDF", true},
		{"Diagrama de la arquitectura", "image", "diagrama.png", "image/png", img, "thumbnails/", "imagen", false},
	} {
		recurso := createChild(r, professor, "POST", "/api/v1/units/"+unit.ID+"/resources",
			map[string]any{
				"title": d.nombre, "type": d.tipo,
				"is_visible": true, "is_mandatory": false, "allow_download": d.descargar,
			}, fmt.Sprintf("Recurso de %s, visible y no obligatorio", d.etiqueta))
		if recurso.ID == "" {
			continue
		}

		var firma struct {
			UploadURL   string `json:"upload_url"`
			ObjectKey   string `json:"object_key"`
			ContentType string `json:"content_type"`
		}
		res = r.api.do("POST", "/api/v1/media/presigned-url", professor, map[string]any{
			"resource_id": recurso.ID, "filename": d.fichero,
			"mime_type": d.mime, "file_size_bytes": len(d.bytes),
		}, nil)
		_ = res.into(&firma)
		quiero := d.prefijo + recurso.StableID + "/"
		if !r.check(fmt.Sprintf("La API firma la carga del %s bajo %s", d.etiqueta, d.prefijo), res, 200,
			fmt.Sprintf("content_type=%q object_key bajo %s", firma.ContentType, quiero),
			firma.ContentType == d.mime && strings.HasPrefix(firma.ObjectKey, quiero)) {
			continue
		}

		res = r.api.absolute("PUT", firma.UploadURL, bytes.NewReader(d.bytes),
			map[string]string{"Content-Type": d.mime})
		r.check(fmt.Sprintf("El %s sube directo al bucket con la URL firmada", d.etiqueta), res, 200,
			fmt.Sprintf("%d bytes aceptados por Cloud Storage", len(d.bytes)), res.err == nil)

		var confirmado struct {
			ProcessingStatus string `json:"processing_status"`
		}
		res = r.api.do("POST", "/api/v1/media/uploads/"+recurso.ID+"/complete", professor,
			map[string]any{"object_key": firma.ObjectKey}, nil)
		_ = res.into(&confirmado)
		docCompleteInst := res.instanceID()
		docCompleteReq := res.requestID()
		r.check(fmt.Sprintf("Confirmar la carga del %s encola su procesamiento", d.etiqueta), res, 202,
			fmt.Sprintf("processing_status=%q", confirmado.ProcessingStatus),
			confirmado.ProcessingStatus == "pending")

		// El corazón de la corrección. Antes de ella este sondeo habría acabado
		// en "failed" pasara el tiempo que pasara.
		estado, tardo := waitForProcessing(r, professor, unit.ID, recurso.ID, wait)
		r.check(fmt.Sprintf("El %s queda disponible sin pasar por el transcodificador", d.etiqueta),
			res, 202,
			fmt.Sprintf("processing_status=%q tras %s", estado, tardo.Round(time.Second)),
			estado == "completed")

		var descarga struct {
			DownloadURL string `json:"download_url"`
		}
		res = r.api.do("GET", "/api/v1/media/resources/"+recurso.ID+"/download-url", professor, nil, nil)
		_ = res.into(&descarga)
		leidoOK := false
		if r.check(fmt.Sprintf("El autor obtiene URL firmada de lectura del %s", d.etiqueta), res, 200,
			"la URL lleva firma V4", strings.Contains(descarga.DownloadURL, "X-Goog-Signature")) &&
			descarga.DownloadURL != "" {

			res = r.api.absolute("GET", descarga.DownloadURL, nil, nil)
			leidoOK = res.err == nil && res.status == 200 && res.header.Get("Content-Length") == fmt.Sprint(len(d.bytes))
			r.check(fmt.Sprintf("El %s se lee con la firma y devuelve los bytes subidos", d.etiqueta),
				res, 200,
				fmt.Sprintf("Content-Length=%s frente a %d subidos",
					res.header.Get("Content-Length"), len(d.bytes)),
				leidoOK)
		}

		res = r.api.absolute("GET", bucketOrigin+"/"+firma.ObjectKey, nil, nil)
		r.check(fmt.Sprintf("El mismo %s sin firma es inaccesible", d.etiqueta), res, 403,
			fmt.Sprintf("el prefijo %s no es legible de forma anónima", d.prefijo), res.err == nil)

		r.recordJob(mediaJob{
			ResourceID:  recurso.ID,
			StableID:    recurso.StableID,
			Kind:        d.tipo,
			ObjectKey:   firma.ObjectKey,
			Instance:    docCompleteInst,
			RequestID:   docCompleteReq,
			FinalStatus: estado,
			Elapsed:     tardo,
			Derivatives: fmt.Sprintf("objeto verificado (%d B, lectura=%v)", len(d.bytes), leidoOK),
			Reconciled:  estado == "completed" && leidoOK,
		})
	}

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

	// ---- intento congelado y guardado parcial ------------------------------
	//
	// La revisión de la Entrega 1 marcó que no existía ninguno de los dos, y
	// que «el envío no congela el cuestionario, así que editar el quiz altera
	// intentos pasados». Estos pasos recorren el flujo nuevo; el envío de más
	// abajo sigue funcionando sin pasar por él, que es lo que prueba que la
	// corrección no rompió el contrato anterior.

	var abierto struct {
		Attempt int `json:"attempt"`
		Expired bool
		Quiz    struct {
			Questions []struct {
				ID      string `json:"id"`
				Options []struct {
					ID        string `json:"id"`
					IsCorrect bool   `json:"is_correct"`
				} `json:"options"`
			} `json:"questions"`
		} `json:"quiz"`
		Answers []map[string]string `json:"answers"`
	}
	res = r.api.do("POST", "/api/v1/quizzes/"+quiz.ID+"/attempts", student, nil, nil)
	_ = res.into(&abierto)
	r.check("El estudiante abre el intento y recibe el cuestionario congelado", res, 200,
		fmt.Sprintf("attempt=%d con %d pregunta(s), sin respuestas previas",
			abierto.Attempt, len(abierto.Quiz.Questions)),
		abierto.Attempt == 1 && len(abierto.Quiz.Questions) == 2 && len(abierto.Answers) == 0)

	// El cuestionario congelado llega sin la clave, igual que la lectura normal
	// del estudiante: el snapshot la conserva en la base, no en la respuesta.
	clave := false
	for _, q := range abierto.Quiz.Questions {
		for _, o := range q.Options {
			if o.IsCorrect {
				clave = true
			}
		}
	}
	r.check("El intento congelado no lleva la clave de respuestas", res, 200,
		"ninguna opción viene marcada como correcta", !clave)

	res = r.api.do("POST", "/api/v1/quizzes/"+quiz.ID+"/attempts", student, nil, nil)
	var reabierto struct {
		Attempt int `json:"attempt"`
	}
	_ = res.into(&reabierto)
	r.check("Abrir el intento dos veces no consume dos intentos", res, 200,
		fmt.Sprintf("attempt=%d en ambas llamadas", reabierto.Attempt),
		reabierto.Attempt == abierto.Attempt)

	res = r.api.do("PUT", "/api/v1/quizzes/"+quiz.ID+"/attempts/current", student,
		map[string]any{"answers": answers[:1]}, nil)
	r.check("El estudiante guarda una respuesta parcial sin calificar nada", res, 200,
		"1 de 2 respuestas guardadas", res.err == nil)

	var recuperado struct {
		Answers []map[string]string `json:"answers"`
	}
	res = r.api.do("GET", "/api/v1/quizzes/"+quiz.ID+"/attempts/current", student, nil, nil)
	_ = res.into(&recuperado)
	r.check("El avance sobrevive a cerrar el navegador", res, 200,
		fmt.Sprintf("%d respuesta(s) recuperadas del intento en curso", len(recuperado.Answers)),
		len(recuperado.Answers) == 1)

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

	// ---- 7. continuidad de sesion, revocacion y suspension (E1) ------------

	r.flowIs("7. Continuidad de sesión, revocación y suspensión entre réplicas (E1)")

	// 7.1 Peticiones sucesivas de una misma sesion atendidas por instancias
	// distintas funcionan sin reautenticar.
	studentInsts, lastStudentRes, allStudentOK := probeSessionAcrossInstances(r, student, minInstancias, 30)
	r.check("Peticiones sucesivas de la misma sesión atendidas por instancias distintas sin reautenticar",
		lastStudentRes, 200,
		fmt.Sprintf("%d instancia(s) distinta(s) atendieron la sesión: %s (mín. exigido=%d)",
			len(studentInsts), formatInstanceMap(studentInsts), minInstancias),
		allStudentOK && len(studentInsts) >= minInstancias)

	// 7.2 Logout en una instancia invalida la sesion en todas las demas.
	// Reutilizamos la sesion de professor2 ya emitida en el flujo 1 (ahorra
	// presupuesto del limitador de login de 10 intentos/min/IP). Primero
	// calentamos la sesion en las replicas activas para demostrar que no se
	// queda cacheada en ninguna tras el logout.
	warmProf2Insts, warmProf2Res, allWarmProf2OK := probeSessionAcrossInstances(r, professor2, minInstancias, 30)
	r.check("La sesión de profesor2 opera y calienta caché en las réplicas antes del logout",
		warmProf2Res, 200,
		fmt.Sprintf("200 en %d instancia(s): %s", len(warmProf2Insts), formatInstanceMap(warmProf2Insts)),
		allWarmProf2OK && len(warmProf2Insts) >= minInstancias)

	res = r.api.do("POST", "/api/v1/auth/logout", professor2, nil, nil)
	logoutInst := res.instanceID()
	r.check("Logout revoca la sesión en la instancia que lo recibe y en la caché compartida",
		res, 204,
		fmt.Sprintf("204 procesado por X-Instance-ID=%q", logoutInst),
		true)

	postLogoutInsts, postLogoutRes, allLogout401, totalLogoutProbes := probeRevokedAcrossInstances(r, professor2, warmProf2Insts, minInstancias, 30)
	r.check("El logout invalida la sesión de inmediato en todas las instancias en la siguiente petición",
		postLogoutRes, 401,
		fmt.Sprintf("401 (code=%q) en el 100%% de %d peticiones repartidas en: %s",
			postLogoutRes.errorCode(), totalLogoutProbes, formatInstanceMap(postLogoutInsts)),
		allLogout401 && len(postLogoutInsts) >= minInstancias)

	// 7.3 Suspender un usuario corta sus sesiones en todas las instancias.
	// Usamos estudiante2 (cuenta sembrada que no participa en los flujos 1-6)
	// y la reactivamos al terminar para que la corrida sea idempotente.
	res = r.api.do("PATCH", "/api/v1/admin/users/"+student2ID+"/status", admin,
		map[string]any{"status": "active"}, nil)
	r.check("El administrador asegura que estudiante2 está activo antes de la prueba",
		res, 200, "status=\"active\"", true)

	student2 := login(r, student2Email, "estudiante")
	if student2 != "" {
		warmSuspInsts, warmSuspRes, allWarmSuspOK := probeSessionAcrossInstances(r, student2, minInstancias, 30)
		r.check("La sesión de estudiante2 es atendida por las réplicas antes de suspender",
			warmSuspRes, 200,
			fmt.Sprintf("200 en %d instancia(s): %s", len(warmSuspInsts), formatInstanceMap(warmSuspInsts)),
			allWarmSuspOK && len(warmSuspInsts) >= minInstancias)

		var suspended struct {
			User struct {
				Status string `json:"status"`
			} `json:"user"`
		}
		res = r.api.do("PATCH", "/api/v1/admin/users/"+student2ID+"/status", admin,
			map[string]any{"status": "suspended"}, nil)
		_ = res.into(&suspended)
		suspendInst := res.instanceID()
		r.check("El administrador suspende la cuenta de estudiante2",
			res, 200,
			fmt.Sprintf("status=%q procesado por X-Instance-ID=%q", suspended.User.Status, suspendInst),
			suspended.User.Status == "suspended")

		postSuspInsts, postSuspRes, allSusp401, totalSuspProbes := probeRevokedAcrossInstances(r, student2, warmSuspInsts, minInstancias, 30)
		r.check("Suspender al usuario corta sus sesiones en todas las instancias en la siguiente petición",
			postSuspRes, 401,
			fmt.Sprintf("401 (code=%q) en el 100%% de %d peticiones repartidas en: %s",
				postSuspRes.errorCode(), totalSuspProbes, formatInstanceMap(postSuspInsts)),
			allSusp401 && len(postSuspInsts) >= minInstancias)

		var reactivated struct {
			User struct {
				Status string `json:"status"`
			} `json:"user"`
		}
		res = r.api.do("PATCH", "/api/v1/admin/users/"+student2ID+"/status", admin,
			map[string]any{"status": "active"}, nil)
		_ = res.into(&reactivated)
		r.check("El administrador reactiva estudiante2 para dejar la base idempotente",
			res, 200,
			fmt.Sprintf("status=%q restaurado", reactivated.User.Status),
			reactivated.User.Status == "active")

		res = r.api.do("GET", "/api/v1/auth/sessions", student2, nil, nil)
		r.check("La sesión previa a la suspensión sigue revocada incluso tras reactivar la cuenta",
			res, 401,
			fmt.Sprintf("code=%q en X-Instance-ID=%q", res.errorCode(), res.instanceID()),
			res.errorCode() == "unauthorized")
	}

	// ---- 8. reconciliacion de trabajos aceptados (E1) ----------------------

	r.flowIs("8. Reconciliación de trabajos multimedia aceptados (E1)")

	reconciliados := 0
	var resumenJobs []string
	for _, j := range r.jobs {
		if j.Reconciled {
			reconciliados++
		}
		resumenJobs = append(resumenJobs, fmt.Sprintf("%s:%s(%s)", j.Kind, j.FinalStatus, short(j.ResourceID)))
	}
	r.note("Todos los trabajos aceptados alcanzaron su estado final sin pérdidas ni duplicados",
		"reconciliación POST /complete (202) -> worker -> PostgreSQL + GCS",
		fmt.Sprintf("%d/%d trabajos reconciliados en completed [%s]",
			reconciliados, len(r.jobs), strings.Join(resumenJobs, ", ")),
		reconciliados == len(r.jobs) && len(r.jobs) >= 3,
		"algún trabajo aceptado no alcanzó completed o sus derivados no cuadraron")
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

// probeSessionAcrossInstances sends consecutive authenticated requests with the
// same Bearer token until at least minInstances distinct X-Instance-ID headers
// have responded with HTTP 200 (or maxAttempts is reached).
func probeSessionAcrossInstances(r *run, token string, minInstances, maxAttempts int) (map[string]int, *response, bool) {
	seen := make(map[string]int)
	var lastRes *response
	allOK := true
	for i := 1; i <= maxAttempts; i++ {
		res := r.api.do("GET", "/api/v1/auth/sessions", token, nil, nil)
		lastRes = res
		inst := res.instanceID()
		hasInst := inst != "" && inst != "—"
		if !hasInst {
			inst = "(sin cabecera)"
			allOK = false
		} else {
			seen[inst]++
		}
		if res.status != http.StatusOK {
			allOK = false
			r.check(fmt.Sprintf("Sondeo de continuidad #%d en %s", i, inst),
				res, http.StatusOK, "fallo inesperado de continuidad", false)
			break
		}
		if hasInst && seen[inst] == 1 {
			r.check(fmt.Sprintf("Sesión activa atendida por instancia %s (intento #%d)", inst, i),
				res, http.StatusOK,
				fmt.Sprintf("X-Instance-ID=%q X-Request-ID=%q", inst, res.requestID()),
				hasInst)
		}
		if len(seen) >= minInstances && i >= minInstances*2 {
			break
		}
	}
	return seen, lastRes, allOK
}

// probeRevokedAcrossInstances verifies that a revoked or suspended token receives
// HTTP 401 across at least minInstances distinct API instances (and covers all
// warmInsts when possible).
func probeRevokedAcrossInstances(r *run, token string, warmInsts map[string]int, minInstances, maxAttempts int) (map[string]int, *response, bool, int) {
	seen := make(map[string]int)
	var lastRes *response
	all401 := true
	total := 0
	for i := 1; i <= maxAttempts; i++ {
		res := r.api.do("GET", "/api/v1/auth/sessions", token, nil, nil)
		lastRes = res
		total++
		inst := res.instanceID()
		hasInst := inst != "" && inst != "—"
		if !hasInst {
			inst = "(sin cabecera)"
			all401 = false
		} else {
			seen[inst]++
		}
		if res.status != http.StatusUnauthorized || res.errorCode() != "unauthorized" {
			all401 = false
			r.check(fmt.Sprintf("Sondeo post-revocación #%d en %s", i, inst),
				res, http.StatusUnauthorized,
				fmt.Sprintf("code=%q (se esperaba 401 unauthorized)", res.errorCode()),
				false)
			break
		}
		if hasInst && seen[inst] == 1 {
			r.check(fmt.Sprintf("Rechazo inmediato (401) en instancia %s (intento #%d)", inst, i),
				res, http.StatusUnauthorized,
				fmt.Sprintf("code=%q X-Instance-ID=%q", res.errorCode(), inst),
				hasInst)
		}
		coveredWarm := true
		for k := range warmInsts {
			if seen[k] == 0 {
				coveredWarm = false
				break
			}
		}
		if len(seen) >= minInstances && coveredWarm && i >= minInstances*2 {
			break
		}
	}
	return seen, lastRes, all401, total
}

func formatInstanceMap(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, cmp.Compare)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, m[k]))
	}
	return strings.Join(parts, ", ")
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
