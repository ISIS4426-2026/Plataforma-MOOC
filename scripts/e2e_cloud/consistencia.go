package main

// E2 (#217): consistencia de las operaciones academicas cuando el trafico se
// reparte entre replicas, y envio duplicado sin doble calificacion.
//
// El recorrido de G3/E1 prueba que cada cosa funciona una vez. Esto prueba lo que
// ese recorrido no toca: la misma operacion repetida, y repetida a la vez, contra
// varias replicas, mirando despues el estado que quedo y no solo la respuesta.
// Una respuesta 200 no demuestra que no se haya calificado dos veces; la cuenta de
// envios, el registro de auditoria y el progreso si.

import (
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
)

// burst lanza n peticiones en el mismo instante: todas esperan una senal y salen
// juntas, que es lo mas cerca que un cliente llega de dos clics casi simultaneos
// atendidos por replicas distintas.
func burst(n int, send func(i int) *response) []*response {
	out := make([]*response, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			out[i] = send(i)
		}(i)
	}
	close(start)
	wg.Wait()
	return out
}

func replayed(res *response) bool {
	return res != nil && res.header != nil && res.header.Get("Idempotent-Replay") == "true"
}

func instancesOf(resps []*response) map[string]int {
	seen := map[string]int{}
	for _, res := range resps {
		if id := res.instanceID(); id != "—" {
			seen[id]++
		}
	}
	return seen
}

// logBurst deja en el log una linea por respuesta: es la prueba de "ambas
// respuestas" que pide el criterio de aceptacion, con la replica que atendio cada
// una.
func logBurst(r *run, titulo string, resps []*response) {
	fmt.Fprintf(r.log, "        %s\n", titulo)
	for i, res := range resps {
		cuerpo := strings.Join(strings.Fields(string(res.body)), " ")
		if len(cuerpo) > 150 {
			cuerpo = cuerpo[:147] + "..."
		}
		marca := ""
		if replayed(res) {
			marca = " Idempotent-Replay=true"
		}
		fmt.Fprintf(r.log, "          #%d %d inst=%s%s %s\n", i+1, res.status, res.instanceID(), marca, cuerpo)
	}
}

// readAcross repite una lectura hasta que al menos min replicas distintas la han
// atendido, y exige que todas devuelvan el estado nuevo. Es la prueba de lectura
// tras escritura: si alguna replica guardara el estado viejo en memoria, aparece
// aqui como una lectura obsoleta.
func readAcross(r *run, name, path, token string, min int, expect string, fresh func(*response) bool) bool {
	seen := map[string]int{}
	stale, total := 0, 0
	var last *response
	for total < 80 && (total < 12 || len(seen) < min) {
		res := r.api.do("GET", path, token, nil, nil)
		last = res
		total++
		if res.err != nil || res.status != http.StatusOK || !fresh(res) {
			stale++
		}
		if id := res.instanceID(); id != "—" {
			seen[id]++
		}
	}
	state := fmt.Sprintf("%d lectura(s) en %d réplica(s) [%s], %d obsoleta(s) o fallida(s) — %s",
		total, len(seen), formatInstanceMap(seen), stale, expect)
	return r.check(name, last, http.StatusOK, state, stale == 0 && len(seen) >= min)
}

// auditCount cuenta las entradas de auditoria que cumplen el filtro. La auditoria
// se escribe en la misma transaccion que el cambio, asi que una entrada de mas es
// un efecto de negocio duplicado.
func auditCount(r *run, admin string, q url.Values) (int, *response) {
	q.Set("limit", "100")
	res := r.api.do("GET", "/api/v1/admin/audit-logs?"+q.Encode(), admin, nil, nil)
	var page struct {
		Items []struct {
			Action string `json:"action"`
		} `json:"items"`
	}
	_ = res.into(&page)
	return len(page.Items), res
}

func consistency(r *run, minInstancias int) {
	// ---- 0. entorno --------------------------------------------------------

	r.flowIs("0. Réplicas disponibles")

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

	vistas := map[string]int{}
	for i := 0; i < 60 && (i < 12 || len(vistas) < minInstancias); i++ {
		probe := r.api.do("GET", "/api/v1/live", "", nil, nil)
		res = probe
		if id := probe.instanceID(); id != "—" {
			vistas[id]++
		}
	}
	if !r.check("El balanceador reparte entre las réplicas exigidas", res, 200,
		fmt.Sprintf("%d réplica(s) distintas respondieron [%s]; se exigen %d", len(vistas), formatInstanceMap(vistas), minInstancias),
		len(vistas) >= minInstancias) {
		fmt.Fprintln(r.log, "\nSin las réplicas exigidas la prueba no demostraría nada; se detiene.")
		return
	}

	admin := login(r, adminEmail, "administrador")
	professor := login(r, professorEmail, "profesor")
	student1, student1ID := loginID(r, studentEmail, "estudiante")
	student2, student2ID := loginID(r, student2Email, "estudiante")
	if admin == "" || professor == "" || student1 == "" || student2 == "" {
		fmt.Fprintln(r.log, "\nSin sesiones no hay prueba que hacer.")
		return
	}

	// ---- 1. preparacion ----------------------------------------------------

	r.flowIs("1. Curso de prueba con un cuestionario y dos lecturas obligatorias")

	title := fmt.Sprintf("E2 · Consistencia entre réplicas %d", time.Now().Unix())
	var course struct {
		ID       string `json:"id"`
		StableID string `json:"stable_id"`
		Status   string `json:"status"`
	}
	res = r.api.do("POST", "/api/v1/courses", professor, map[string]any{
		"title": title, "description": "Curso creado por la verificación E2 de consistencia.",
	}, nil)
	_ = res.into(&course)
	if !r.check("El profesor crea el curso en borrador", res, 201,
		fmt.Sprintf("status=%q", course.Status), course.Status == "draft" && course.StableID != "") {
		return
	}

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
	var textos [2]node
	for i := range textos {
		textos[i] = createChild(r, professor, "POST", "/api/v1/units/"+unit.ID+"/resources",
			map[string]any{
				"title": fmt.Sprintf("Lectura %d", i+1), "type": "text",
				"is_visible": true, "is_mandatory": true, "content_text": "# Lectura\n\nContenido de la unidad.",
			}, fmt.Sprintf("Recurso de texto %d, obligatorio y visible", i+1))
	}
	quizRes := createChild(r, professor, "POST", "/api/v1/units/"+unit.ID+"/resources",
		map[string]any{"title": "Evaluación", "type": "quiz", "is_visible": true, "is_mandatory": true},
		"Recurso de quiz, obligatorio y visible")
	if textos[0].ID == "" || textos[1].ID == "" || quizRes.ID == "" {
		return
	}

	var quiz authorQuiz
	res = r.api.do("POST", "/api/v1/resources/"+quizRes.ID+"/quiz", professor, map[string]any{
		"title": "Evaluación de consistencia", "passing_score": 70, "max_attempts": 3,
	}, nil)
	_ = res.into(&quiz)
	if !r.check("El profesor define el cuestionario con 3 intentos", res, 201,
		fmt.Sprintf("quiz creado sobre el recurso %s", short(quizRes.ID)), quiz.ID != "") {
		return
	}
	for i, q := range []map[string]any{
		{"prompt": "¿Dónde se guarda la clave de idempotencia?", "options": []map[string]any{
			{"text": "En Redis, compartido por todas las réplicas", "is_correct": true},
			{"text": "En la memoria de cada réplica", "is_correct": false},
		}},
		{"prompt": "¿Qué impide dos envíos con el mismo número de intento?", "options": []map[string]any{
			{"text": "Una restricción única en la base", "is_correct": true},
			{"text": "Un candado en la memoria de la API", "is_correct": false},
		}},
	} {
		res = r.api.do("POST", "/api/v1/quizzes/"+quiz.ID+"/questions", professor, q, nil)
		r.check(fmt.Sprintf("Pregunta %d con su clave de respuesta", i+1), res, 201, "", res.err == nil)
	}
	res = r.api.do("GET", "/api/v1/resources/"+quizRes.ID+"/quiz", professor, nil, nil)
	_ = res.into(&quiz)
	if !r.check("El autor relee el cuestionario con su clave", res, 200,
		fmt.Sprintf("%d pregunta(s) con clave", len(quiz.Questions)), len(quiz.Questions) == 2) {
		return
	}
	answers := correctAnswers(quiz)

	res = r.api.do("POST", "/api/v1/courses/"+course.ID+"/publish", professor, nil, nil)
	var published struct {
		Status string `json:"status"`
	}
	_ = res.into(&published)
	if !r.check("El curso se publica", res, 200, fmt.Sprintf("status=%q", published.Status),
		published.Status == "published") {
		return
	}

	// ---- 2. lectura tras escritura -----------------------------------------

	r.flowIs("2. Lectura tras escritura: la publicación se ve igual en todas las réplicas")

	readAcross(r, "El detalle del curso figura como publicado en cada réplica",
		"/api/v1/courses/"+course.ID, professor, minInstancias, "ninguna réplica sirve el borrador anterior",
		func(res *response) bool {
			var c struct {
				Status string `json:"status"`
			}
			return res.into(&c) == nil && c.Status == "published"
		})

	readAcross(r, "El catálogo público lo lista en cada réplica",
		"/api/v1/courses?limit=50&search="+url.QueryEscape(title), "", minInstancias, "el catálogo anónimo ya incluye el curso",
		func(res *response) bool { return strings.Contains(string(res.body), course.StableID) })

	// ---- 3. inscripcion ----------------------------------------------------

	r.flowIs("3. Inscripción repetida y simultánea: una fila y un evento")

	enrollPath := "/api/v1/courses/" + course.ID + "/enrollments"
	rosterRows := func(studentID string) (int, int, *response) {
		res := r.api.do("GET", enrollPath, professor, nil, nil)
		var roster struct {
			Items []struct {
				StudentID string `json:"student_id"`
				Status    string `json:"status"`
			} `json:"items"`
		}
		_ = res.into(&roster)
		mine := 0
		for _, it := range roster.Items {
			if it.StudentID == studentID {
				mine++
			}
		}
		return mine, len(roster.Items), res
	}
	enrollEvents := func(studentID string) (int, *response) {
		return auditCount(r, admin, url.Values{
			"target_resource": {"course:" + course.StableID},
			"actor_id":        {studentID},
			"action_prefix":   {"enrollment."},
		})
	}

	// Sin Idempotency-Key a proposito: es el caso sin la defensa del middleware,
	// el de la base sola. Dos clics, o un reintento del cliente, que llegan a la
	// vez a replicas distintas.
	resps := burst(8, func(int) *response { return r.api.do("POST", enrollPath, student1, nil, nil) })
	logBurst(r, "8 inscripciones simultáneas de estudiante1, sin Idempotency-Key", resps)
	enrollIDs := map[string]bool{}
	todas200 := true
	for _, res := range resps {
		var e struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		}
		if res.status != http.StatusOK || res.into(&e) != nil || e.Status != "active" {
			todas200 = false
		}
		enrollIDs[e.ID] = true
	}
	r.check("Ocho inscripciones simultáneas devuelven la misma inscripción activa", resps[0], 200,
		fmt.Sprintf("las 8 respondieron 200 active con %d identificador(es) distinto(s); réplicas [%s]",
			len(enrollIDs), formatInstanceMap(instancesOf(resps))),
		todas200 && len(enrollIDs) == 1)

	mine, total, res := rosterRows(student1ID)
	r.check("El censo del curso tiene una sola fila de estudiante1", res, 200,
		fmt.Sprintf("%d fila(s) de estudiante1 en un censo de %d", mine, total), mine == 1 && total == 1)

	nEvents, res := enrollEvents(student1ID)
	r.check("La auditoría registra una sola inscripción, no ocho", res, 200,
		fmt.Sprintf("%d entrada(s) enrollment.* de estudiante1 sobre el curso", nEvents), nEvents == 1)

	res = r.api.do("POST", enrollPath, student1, nil, nil)
	nEvents, _ = enrollEvents(student1ID)
	r.check("Inscribirse otra vez, ya inscrito, no agrega ningún evento", res, 200,
		fmt.Sprintf("%d entrada(s) tras el reintento secuencial", nEvents), nEvents == 1)

	readAcross(r, "La inscripción activa se lee igual en cada réplica",
		enrollPath+"/me", student1, minInstancias, "status=active en todas",
		func(res *response) bool {
			var e struct {
				Enrolled   bool `json:"enrolled"`
				Enrollment struct {
					Status string `json:"status"`
				} `json:"enrollment"`
			}
			return res.into(&e) == nil && e.Enrolled && e.Enrollment.Status == "active"
		})

	// Con clave: una sola ejecucion, y el resto o repite la respuesta o espera.
	keyEnroll := fmt.Sprintf("e2-inscripcion-%d", time.Now().UnixNano())
	resps = burst(8, func(int) *response {
		return r.api.do("POST", enrollPath, student2, nil, map[string]string{"Idempotency-Key": keyEnroll})
	})
	logBurst(r, "8 inscripciones simultáneas de estudiante2, con la misma Idempotency-Key", resps)
	ok := true
	enrollIDs = map[string]bool{}
	var resuelta *response
	enEspera := 0
	for _, res := range resps {
		if res.status == http.StatusConflict && res.errorCode() == "idempotency_in_progress" {
			enEspera++
			// El original aun no termina: es la respuesta correcta, y el reintento
			// de abajo debe devolver la grabada.
			for try := 0; try < 10 && res.status == http.StatusConflict; try++ {
				time.Sleep(300 * time.Millisecond)
				res = r.api.do("POST", enrollPath, student2, nil, map[string]string{"Idempotency-Key": keyEnroll})
			}
		}
		var e struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		}
		if res.status != http.StatusOK || res.into(&e) != nil || e.Status != "active" {
			ok = false
		}
		enrollIDs[e.ID] = true
		if resuelta == nil || replayed(res) {
			resuelta = res
		}
	}
	r.check("Con la misma clave, ocho solicitudes simultáneas dejan una inscripción", resuelta, 200,
		fmt.Sprintf("%d llegaron mientras la original corría (409 idempotency_in_progress) y al reintentar recibieron la grabada; todas terminaron en 200 active con %d identificador(es)", enEspera, len(enrollIDs)),
		ok && len(enrollIDs) == 1)

	mine, total, res = rosterRows(student2ID)
	r.check("El censo suma una fila por estudiante, dos en total", res, 200,
		fmt.Sprintf("%d fila(s) de estudiante2 en un censo de %d", mine, total), mine == 1 && total == 2)
	nEvents, res = enrollEvents(student2ID)
	r.check("La auditoría registra una sola inscripción de estudiante2", res, 200,
		fmt.Sprintf("%d entrada(s) enrollment.*", nEvents), nEvents == 1)

	// ---- 4. envio duplicado del quiz ---------------------------------------

	r.flowIs("4. Envío duplicado del cuestionario: una calificación, un intento, un evento")

	submitPath := "/api/v1/quizzes/" + quiz.ID + "/submissions"
	submissionEvents := func(studentID string) (int, *response) {
		return auditCount(r, admin, url.Values{
			"target_resource": {"quiz:" + quiz.ID},
			"actor_id":        {studentID},
			"action_prefix":   {"quiz.submitted"},
		})
	}
	history := func(token string) (int, *response) {
		res := r.api.do("GET", submitPath+"/me", token, nil, nil)
		var list struct {
			Items []struct {
				ID      string `json:"submission_id"`
				Attempt int    `json:"attempt"`
			} `json:"items"`
		}
		_ = res.into(&list)
		return len(list.Items), res
	}
	type grade struct {
		SubmissionID string `json:"submission_id"`
		Score        int    `json:"score"`
		Passed       bool   `json:"passed"`
		Attempt      int    `json:"attempt"`
	}

	// Estudiante 1: la misma clave, seis veces a la vez.
	key := fmt.Sprintf("e2-envio-%d", time.Now().UnixNano())
	send1 := func() *response {
		return r.api.do("POST", submitPath, student1, map[string]any{"answers": answers}, map[string]string{"Idempotency-Key": key})
	}
	resps = burst(6, func(int) *response { return send1() })
	logBurst(r, "6 envíos simultáneos de estudiante1 con la misma Idempotency-Key", resps)
	originals, okAll := 0, true
	ids := map[string]bool{}
	var primera *response
	for _, res := range resps {
		for try := 0; try < 10 && res.status == http.StatusConflict && res.errorCode() == "idempotency_in_progress"; try++ {
			time.Sleep(300 * time.Millisecond)
			res = send1()
		}
		var g grade
		if res.status != http.StatusOK || res.into(&g) != nil || g.Attempt != 1 || g.Score != 100 || !g.Passed {
			okAll = false
		}
		ids[g.SubmissionID] = true
		if !replayed(res) {
			originals++
			primera = res
		}
	}
	if primera == nil {
		primera = resps[0]
	}
	r.check("Seis envíos simultáneos con la misma clave se califican una sola vez", primera, 200,
		fmt.Sprintf("1 original y %d repetición(es) con Idempotent-Replay; %d submission_id distinto(s); intento 1 en todas", len(resps)-originals, len(ids)),
		okAll && originals == 1 && len(ids) == 1)

	// Reenvio posterior hasta que lo atienda otra replica: la clave vive en Redis,
	// no en la memoria de la que califico.
	var otra *response
	for try := 0; try < 40; try++ {
		cand := send1()
		if cand.status == http.StatusOK && replayed(cand) && cand.instanceID() != primera.instanceID() {
			otra = cand
			break
		}
	}
	if otra == nil {
		otra = send1()
	}
	var gOrig, gRep grade
	_ = primera.into(&gOrig)
	_ = otra.into(&gRep)
	r.check("El reenvío en otra réplica devuelve la misma calificación", otra, 200,
		fmt.Sprintf("original en %s y repetición en %s: mismo submission_id (%s), mismo intento (%d), misma nota (%d)",
			primera.instanceID(), otra.instanceID(), short(gRep.SubmissionID), gRep.Attempt, gRep.Score),
		replayed(otra) && otra.instanceID() != primera.instanceID() &&
			gRep.SubmissionID == gOrig.SubmissionID && gRep.Attempt == gOrig.Attempt && gRep.Score == gOrig.Score)

	nSubs, res := history(student1)
	r.check("El historial del estudiante tiene un solo envío", res, 200,
		fmt.Sprintf("%d envío(s) registrado(s)", nSubs), nSubs == 1)
	nEvents, res = submissionEvents(student1ID)
	r.check("La auditoría registra un solo quiz.submitted", res, 200,
		fmt.Sprintf("%d entrada(s) quiz.submitted de estudiante1", nEvents), nEvents == 1)

	readAcross(r, "El historial se lee igual en cada réplica",
		submitPath+"/me", student1, minInstancias, "un envío en todas",
		func(res *response) bool {
			var list struct {
				Items []struct{} `json:"items"`
			}
			return res.into(&list) == nil && len(list.Items) == 1
		})

	// Reutilizar la clave con otro cuerpo no es un reintento.
	res = r.api.do("POST", submitPath, student1, map[string]any{"answers": answers[:1]},
		map[string]string{"Idempotency-Key": key})
	r.check("La misma clave con otro cuerpo se rechaza, no se reinterpreta", res, 422,
		fmt.Sprintf("code=%q", res.errorCode()), res.errorCode() == "idempotency_key_reused")

	// Estudiante 2: sin clave. Aqui no hay middleware que proteja; lo que sostiene
	// la cuenta es la restriccion unica (quiz, estudiante, intento).
	resps = burst(6, func(int) *response {
		return r.api.do("POST", submitPath, student2, map[string]any{"answers": answers}, nil)
	})
	logBurst(r, "6 envíos simultáneos de estudiante2, SIN Idempotency-Key", resps)
	exitos, intentos, inesperados := 0, map[int]bool{}, 0
	var aceptada *response
	for _, res := range resps {
		var g grade
		switch {
		case res.status == http.StatusOK && res.into(&g) == nil:
			exitos++
			if aceptada == nil {
				aceptada = res
			}
			intentos[g.Attempt] = true
		case res.status == http.StatusConflict:
			// Perdio la carrera por el numero de intento: es la respuesta correcta.
		default:
			inesperados++
		}
	}
	if aceptada == nil {
		aceptada = resps[0]
	}
	r.check("Sin clave, los envíos simultáneos nunca repiten un número de intento", aceptada, 200,
		fmt.Sprintf("%d aceptado(s) con intentos distintos %v, el resto 409; %d respuesta(s) inesperada(s); réplicas [%s]",
			exitos, sortedKeys(intentos), inesperados, formatInstanceMap(instancesOf(resps))),
		exitos >= 1 && exitos <= 3 && len(intentos) == exitos && inesperados == 0)

	nSubs, res = history(student2)
	nEvents, _ = submissionEvents(student2ID)
	r.check("Lo registrado coincide con lo aceptado: ni más ni menos", res, 200,
		fmt.Sprintf("%d aceptado(s) → %d envío(s) en el historial y %d en la auditoría", exitos, nSubs, nEvents),
		nSubs == exitos && nEvents == exitos)

	// Agotar el cupo: ningun duplicado puede pasar de max_attempts.
	var cierre *response
	for try := 0; try < 5; try++ {
		cierre = r.api.do("POST", submitPath, student2, map[string]any{"answers": answers}, nil)
		if cierre.status != http.StatusOK {
			break
		}
	}
	nSubs, _ = history(student2)
	r.check("Agotado el cupo, el envío siguiente se rechaza", cierre, 409,
		fmt.Sprintf("code=%q; el historial se queda en %d de 3 intentos", cierre.errorCode(), nSubs),
		nSubs == 3)

	// ---- 5. progreso -------------------------------------------------------

	r.flowIs("5. Progreso repetido y simultáneo: sin inflar el avance y con una sola insignia")

	progressPath := "/api/v1/progress/courses/" + course.ID
	type progressView struct {
		CompletedCount int     `json:"completed_count"`
		TotalCount     int     `json:"total_count"`
		Percentage     float64 `json:"percentage"`
		IsApproved     bool    `json:"is_approved"`
		Badge          *struct {
			ID string `json:"id"`
		} `json:"badge"`
	}
	var pv progressView
	res = r.api.do("GET", progressPath, student1, nil, nil)
	_ = res.into(&pv)
	r.check("Aprobar el quiz completó un recurso, uno solo", res, 200,
		fmt.Sprintf("%d/%d · %.2f%%", pv.CompletedCount, pv.TotalCount, pv.Percentage),
		pv.CompletedCount == 1 && pv.TotalCount == 3)

	// Dieciseis latidos a la vez sobre las dos lecturas que faltan: el ultimo que
	// completa el curso emite la insignia, y esa emision es lo que no debe duplicarse.
	resps = burst(16, func(i int) *response {
		return r.api.do("POST", "/api/v1/progress/heartbeat", student1, map[string]any{
			"resource_id": textos[i%2].ID, "dwell_time_seconds": 30, "completed": true,
		}, nil)
	})
	todas200 = true
	for _, res := range resps {
		if res.status != http.StatusOK {
			todas200 = false
		}
	}
	fmt.Fprintf(r.log, "        16 latidos simultáneos (8 por lectura) de estudiante1; réplicas [%s]\n", formatInstanceMap(instancesOf(resps)))
	r.check("Dieciséis latidos simultáneos se aceptan sin error", resps[0], 200,
		fmt.Sprintf("estados: %s", statusSummary(resps)), todas200)

	res = r.api.do("GET", progressPath, student1, nil, nil)
	pv = progressView{}
	_ = res.into(&pv)
	r.check("El avance quedó en 3 de 3, no más", res, 200,
		fmt.Sprintf("%d/%d · %.2f%% · aprobado=%v", pv.CompletedCount, pv.TotalCount, pv.Percentage, pv.IsApproved),
		pv.CompletedCount == 3 && pv.TotalCount == 3 && pv.Percentage == 100 && pv.IsApproved && pv.Badge != nil)

	// La emision de la insignia se audita sobre la propia insignia (badge:<id>),
	// no sobre el curso, asi que se cuenta por actor desde el inicio de la corrida.
	nBadges, res := auditCount(r, admin, url.Values{
		"actor_id":      {student1ID},
		"action_prefix": {"badge."},
		"from":          {r.started.UTC().Format(time.RFC3339)},
	})
	r.check("Se emitió una sola insignia", res, 200,
		fmt.Sprintf("%d entrada(s) badge.issued de estudiante1 desde el inicio de la corrida", nBadges), nBadges == 1)
	courseTarget := url.Values{"target_resource": {"course:" + course.StableID}, "actor_id": {student1ID}}
	courseTarget.Set("action_prefix", "progress.")
	nDone, res := auditCount(r, admin, courseTarget)
	r.check("El curso se completó una sola vez", res, 200,
		fmt.Sprintf("%d entrada(s) progress.course_completed", nDone), nDone == 1)

	readAcross(r, "El avance final se lee igual en cada réplica",
		progressPath, student1, minInstancias, "3/3 y la insignia en todas",
		func(res *response) bool {
			var p progressView
			return res.into(&p) == nil && p.CompletedCount == 3 && p.Badge != nil
		})

	// ---- 6. caches ---------------------------------------------------------

	r.flowIs("6. Cachés locales: el validador del curso es el mismo en toda réplica")

	// El detalle del curso es lo que lleva ETag (el catalogo no lo envia). El
	// validador se calcula de la fila, no de la memoria: cualquier replica lo debe
	// reconocer.
	detail := "/api/v1/courses/" + course.ID
	res = r.api.do("GET", detail, professor, nil, nil)
	etag := res.header.Get("ETag")
	r.check("El detalle del curso responde con un ETag", res, 200, fmt.Sprintf("ETag presente: %v", etag != ""), etag != "")
	if etag != "" {
		seen := map[string]int{}
		no304 := 0
		var last *response
		for i := 0; i < 60 && (i < 12 || len(seen) < minInstancias); i++ {
			last = r.api.do("GET", detail, professor, nil, map[string]string{"If-None-Match": etag})
			if last.status != http.StatusNotModified {
				no304++
			}
			if id := last.instanceID(); id != "—" {
				seen[id]++
			}
		}
		r.check("Cualquier réplica reconoce el ETag y responde 304", last, http.StatusNotModified,
			fmt.Sprintf("%d réplica(s) [%s]; %d respuesta(s) distinta(s) de 304", len(seen), formatInstanceMap(seen), no304),
			no304 == 0 && len(seen) >= minInstancias)
	}
}

func sortedKeys(m map[int]bool) []int {
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func statusSummary(resps []*response) string {
	counts := map[int]int{}
	for _, res := range resps {
		counts[res.status]++
	}
	parts := []string{}
	keys := make([]int, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%d×%d", counts[k], k))
	}
	return strings.Join(parts, " ")
}
