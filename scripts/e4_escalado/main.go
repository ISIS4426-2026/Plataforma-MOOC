// E4: verificacion breve del escalado de Cloud Run y Pub/Sub, con carga controlada.
//
//	go run ./scripts/e4_escalado -base https://mooc-frontend-<n>.us-east1.run.app -fase todas
//
// Dos pruebas, una por servicio que escala:
//
//	api       N usuarios concurrentes consultan el catalogo autenticados, por escalones. Lo que se observa es el numero
//	          de instancias del backend (cabecera X-Instance-ID, y despues la metrica de Cloud Run).
//	workers   una tanda de videos se sube a la vez y se confirma: cada confirmacion publica un mensaje en Pub/Sub. Lo
//	          que se observa son los mensajes pendientes en la suscripcion y las instancias de workers que los drenan.
//
// Es carga CONTROLADA, no una prueba de capacidad: los niveles estan fijados de antemano y se imprimen, para que la
// corrida sea repetible. Cada fase escribe un JSON con la ventana de tiempo (UTC) que scripts/e4_escalado/metricas.py usa
// para pedir las metricas a Cloud Monitoring.
//
// Nada de lo que escribe lleva una credencial: ni el token de sesion ni las URL firmadas (se quita la query).
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
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Cuenta sembrada de la plataforma: su contrasena es el valor de desarrollo compartido de las semillas, el mismo que usa
// scripts/e2e_cloud.
const seedPassword = "Password123!"

type api struct {
	base  string
	hc    *http.Client
	token string
}

func newAPI(base string) *api {
	tr := &http.Transport{MaxIdleConns: 1000, MaxIdleConnsPerHost: 1000, IdleConnTimeout: 90 * time.Second}
	return &api{base: strings.TrimRight(base, "/"), hc: &http.Client{Transport: tr, Timeout: 60 * time.Second}}
}

type reply struct {
	status   int
	body     []byte
	instance string
	took     time.Duration
	err      error
}

func (a *api) do(method, path string, body any, hdr map[string]string) reply {
	var rd io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, a.base+path, rd)
	if err != nil {
		return reply{err: err}
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if a.token != "" {
		req.Header.Set("Authorization", "Bearer "+a.token)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	return a.send(req)
}

func (a *api) send(req *http.Request) reply {
	t := time.Now()
	resp, err := a.hc.Do(req)
	if err != nil {
		return reply{err: err, took: time.Since(t)}
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return reply{status: resp.StatusCode, body: b, instance: resp.Header.Get("X-Instance-ID"), took: time.Since(t), err: err}
}

func (a *api) login(email string) error {
	r := a.do("POST", "/api/v1/auth/login", map[string]any{"email": email, "password": seedPassword}, nil)
	if r.err != nil || r.status != 200 {
		return fmt.Errorf("login %s: estado %d, error %v", email, r.status, r.err)
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(r.body, &out); err != nil || out.Token == "" {
		return fmt.Errorf("login %s: sin token", email)
	}
	a.token = out.Token
	return nil
}

type window struct {
	Fase   string    `json:"fase"`
	Inicio time.Time `json:"inicio_utc"`
	Fin    time.Time `json:"fin_utc"`
}

func main() {
	var (
		base     = flag.String("base", "", "origen HTTPS del frontend desplegado (obligatorio)")
		fase     = flag.String("fase", "todas", "api, workers o todas")
		escalon  = flag.String("usuarios", "20,60,150", "usuarios concurrentes de cada escalon de la fase api")
		duracion = flag.Duration("duracion-escalon", 45*time.Second, "cuanto dura cada escalon de la fase api")
		trabajos = flag.Int("trabajos", 9, "videos que se suben a la vez en la fase workers")
		video    = flag.String("video", "scratch/video_corto.mp4", "video que se sube (120 s en el perfil corto de A7)")
		espera   = flag.Duration("espera", 15*time.Minute, "cuanto esperar a que los workers terminen todos los videos")
		salida   = flag.String("salida", "docs/entrega4/evidencias/escalado", "directorio de la evidencia")
		profesor = flag.String("profesor", "profesor1@plataforma-mooc.test", "cuenta sembrada con rol profesor")
	)
	flag.Parse()
	if *base == "" {
		fmt.Fprintln(os.Stderr, "falta -base: declara el destino, no hay valor por omision")
		os.Exit(2)
	}
	if err := os.MkdirAll(*salida, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	a := newAPI(*base)
	if err := a.login(*profesor); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if *fase == "api" || *fase == "todas" {
		var niveles []int
		for _, s := range strings.Split(*escalon, ",") {
			n, err := strconv.Atoi(strings.TrimSpace(s))
			if err != nil || n <= 0 {
				fmt.Fprintf(os.Stderr, "-usuarios invalido: %q\n", s)
				os.Exit(2)
			}
			niveles = append(niveles, n)
		}
		if err := faseAPI(a, niveles, *duracion, *salida); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	if *fase == "workers" || *fase == "todas" {
		if err := faseWorkers(a, *trabajos, *video, *espera, *salida); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}

// ---- fase api ---------------------------------------------------------------

type muestra struct {
	t        time.Time
	took     time.Duration
	status   int
	instance string
	err      bool
	motivo   string
}

func faseAPI(a *api, niveles []int, dur time.Duration, salida string) error {
	fmt.Printf("== Fase api: escalones de %v usuarios concurrentes, %s cada uno, GET /api/v1/courses?limit=20\n", niveles, dur)
	ventana := window{Fase: "api", Inicio: time.Now().UTC()}
	var md strings.Builder
	fmt.Fprintf(&md, "# Fase api: usuarios concurrentes contra el backend\n\nInicio (UTC): %s\n\n", ventana.Inicio.Format(time.RFC3339))
	fmt.Fprintf(&md, "| Escalon | Usuarios | Peticiones | Errores | Req/s | p50 | p95 | p99 | Instancias distintas vistas |\n|---|---:|---:|---:|---:|---:|---:|---:|---:|\n")

	todas := map[string]bool{}
	for i, users := range niveles {
		var mu sync.Mutex
		var all []muestra
		stop := time.Now().Add(dur)
		var wg sync.WaitGroup
		for u := 0; u < users; u++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				var local []muestra
				for time.Now().Before(stop) {
					r := a.do("GET", "/api/v1/courses?limit=20", nil, nil)
					m := muestra{t: time.Now(), took: r.took, status: r.status, instance: r.instance, err: r.err != nil || r.status != 200}
					if m.err {
						m.motivo = strconv.Itoa(r.status)
						if r.err != nil {
							m.motivo = "red/timeout"
						}
					}
					local = append(local, m)
				}
				mu.Lock()
				all = append(all, local...)
				mu.Unlock()
			}()
		}
		wg.Wait()

		inst := map[string]bool{}
		motivos := map[string]int{}
		var errs int
		lat := make([]time.Duration, 0, len(all))
		for _, m := range all {
			if m.err {
				errs++
				motivos[m.motivo]++
			}
			if m.instance != "" {
				inst[m.instance] = true
				todas[m.instance] = true
			}
			lat = append(lat, m.took)
		}
		sort.Slice(lat, func(x, y int) bool { return lat[x] < lat[y] })
		pc := func(p float64) time.Duration {
			if len(lat) == 0 {
				return 0
			}
			return lat[int(float64(len(lat)-1)*p)]
		}
		fmt.Printf("  escalon %d: %3d usuarios · %6d peticiones · %d errores · %.0f req/s · p50 %v p95 %v p99 %v · %d instancia(s)\n",
			i+1, users, len(all), errs, float64(len(all))/dur.Seconds(), pc(.5).Round(time.Millisecond), pc(.95).Round(time.Millisecond), pc(.99).Round(time.Millisecond), len(inst))
		if errs > 0 {
			fmt.Printf("    errores por motivo: %v\n", motivos)
		}
		fmt.Fprintf(&md, "| %d | %d | %d | %d | %.0f | %v | %v | %v | %d |\n", i+1, users, len(all), errs,
			float64(len(all))/dur.Seconds(), pc(.5).Round(time.Millisecond), pc(.95).Round(time.Millisecond), pc(.99).Round(time.Millisecond), len(inst))
	}
	ventana.Fin = time.Now().UTC()
	fmt.Fprintf(&md, "\nFin (UTC): %s · instancias distintas en toda la fase: %d\n", ventana.Fin.Format(time.RFC3339), len(todas))
	fmt.Printf("  instancias distintas del backend vistas en toda la fase: %d\n", len(todas))
	return escribir(salida, "fase_api", ventana, md.String())
}

// ---- fase workers -----------------------------------------------------------

type node struct {
	ID       string `json:"id"`
	StableID string `json:"stable_id"`
}

func (a *api) crear(path string, body map[string]any) (node, error) {
	var n node
	r := a.do("POST", path, body, nil)
	if r.err != nil || r.status != 201 {
		return n, fmt.Errorf("POST %s: estado %d %v %s", path, r.status, r.err, truncar(string(r.body), 160))
	}
	if err := json.Unmarshal(r.body, &n); err != nil || n.ID == "" {
		return n, fmt.Errorf("POST %s: respuesta sin id", path)
	}
	return n, nil
}

func faseWorkers(a *api, n int, videoPath string, espera time.Duration, salida string) error {
	raw, err := os.ReadFile(videoPath)
	if err != nil {
		return fmt.Errorf("leer el video: %w", err)
	}
	fmt.Printf("== Fase workers: %d videos de %s (%d bytes) subidos a la vez\n", n, filepath.Base(videoPath), len(raw))

	titulo := "Escalado E4 " + time.Now().UTC().Format("20060102-150405")
	curso, err := a.crear("/api/v1/courses", map[string]any{"title": titulo, "description": "Curso de la verificacion de escalado de la Entrega 4."})
	if err != nil {
		return err
	}
	mod, err := a.crear("/api/v1/courses/"+curso.ID+"/modules", map[string]any{"title": "Modulo 1"})
	if err != nil {
		return err
	}
	unit, err := a.crear("/api/v1/modules/"+mod.ID+"/units", map[string]any{"title": "Unidad 1"})
	if err != nil {
		return err
	}
	ids := make([]string, n)
	for i := range ids {
		rec, err := a.crear("/api/v1/units/"+unit.ID+"/resources", map[string]any{
			"title": fmt.Sprintf("Video %d", i+1), "type": "video", "is_visible": true, "is_mandatory": false, "allow_download": false,
		})
		if err != nil {
			return err
		}
		ids[i] = rec.ID
	}

	// Presign y subida directa al bucket de todos a la vez; luego las confirmaciones, tambien a la vez: cada una publica un
	// mensaje, asi que lo que se mide empieza cuando las N confirmaciones salen.
	type subida struct{ id, key string }
	subidas := make([]subida, n)
	var wg sync.WaitGroup
	var fallos int32
	for i, id := range ids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := a.do("POST", "/api/v1/media/presigned-url", map[string]any{
				"resource_id": id, "filename": "clase.mp4", "mime_type": "video/mp4", "file_size_bytes": len(raw)}, nil)
			var p struct {
				UploadURL string `json:"upload_url"`
				ObjectKey string `json:"object_key"`
			}
			if r.err != nil || r.status != 200 || json.Unmarshal(r.body, &p) != nil || p.UploadURL == "" {
				atomic.AddInt32(&fallos, 1)
				return
			}
			req, _ := http.NewRequest("PUT", p.UploadURL, bytes.NewReader(raw))
			req.Header.Set("Content-Type", "video/mp4")
			if up := a.send(req); up.err != nil || up.status != 200 {
				atomic.AddInt32(&fallos, 1)
				return
			}
			subidas[i] = subida{id: id, key: p.ObjectKey}
		}()
	}
	wg.Wait()
	if fallos > 0 {
		return fmt.Errorf("%d subidas fallaron antes de confirmar", fallos)
	}
	fmt.Printf("  %d originales subidos; confirmando todos a la vez\n", n)

	ventana := window{Fase: "workers", Inicio: time.Now().UTC()}
	t0 := time.Now()
	for _, s := range subidas {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := a.do("POST", "/api/v1/media/uploads/"+s.id+"/complete", map[string]any{"object_key": s.key}, nil)
			if r.err != nil || r.status != 202 {
				atomic.AddInt32(&fallos, 1)
			}
		}()
	}
	wg.Wait()
	tPub := time.Since(t0)
	if fallos > 0 {
		return fmt.Errorf("%d confirmaciones no fueron aceptadas", fallos)
	}
	fmt.Printf("  %d confirmaciones aceptadas (202) en %v: el backend responde sin esperar a los workers\n", n, tPub.Round(time.Millisecond))

	// Sondeo: cuando termina cada video.
	fin := make(map[string]time.Duration, n)
	estado := map[string]string{}
	var linea []string
	for time.Since(t0) < espera && len(fin) < n {
		r := a.do("GET", "/api/v1/units/"+unit.ID+"/resources", nil, nil)
		var list struct {
			Items []struct {
				ID     string `json:"id"`
				Status string `json:"processing_status"`
			} `json:"items"`
		}
		if r.err == nil && json.Unmarshal(r.body, &list) == nil {
			for _, it := range list.Items {
				estado[it.ID] = it.Status
				if (it.Status == "completed" || it.Status == "failed") && fin[it.ID] == 0 {
					fin[it.ID] = time.Since(t0)
				}
			}
		}
		linea = append(linea, fmt.Sprintf("%3.0fs: %d/%d terminados", time.Since(t0).Seconds(), len(fin), n))
		fmt.Printf("  %s\n", linea[len(linea)-1])
		time.Sleep(10 * time.Second)
	}
	ventana.Fin = time.Now().UTC()

	var tiempos []time.Duration
	failed := 0
	for id, d := range fin {
		tiempos = append(tiempos, d)
		if estado[id] == "failed" {
			failed++
		}
	}
	sort.Slice(tiempos, func(i, j int) bool { return tiempos[i] < tiempos[j] })
	var md strings.Builder
	fmt.Fprintf(&md, "# Fase workers: %d videos de 120 s publicados a la vez\n\n", n)
	fmt.Fprintf(&md, "- Ventana (UTC): %s a %s\n- Confirmaciones aceptadas (202) en %v\n- Terminaron %d de %d (%d fallidos)\n",
		ventana.Inicio.Format(time.RFC3339), ventana.Fin.Format(time.RFC3339), tPub.Round(time.Millisecond), len(fin), n, failed)
	if len(tiempos) > 0 {
		fmt.Fprintf(&md, "- Primer video listo a los %v; ultimo a los %v\n", tiempos[0].Round(time.Second), tiempos[len(tiempos)-1].Round(time.Second))
	}
	fmt.Fprintf(&md, "\n## Avance observado\n\n")
	for _, l := range linea {
		fmt.Fprintf(&md, "- %s\n", l)
	}
	fmt.Printf("  terminaron %d de %d (%d fallidos)\n", len(fin), n, failed)
	if len(fin) < n {
		_ = escribir(salida, "fase_workers", ventana, md.String())
		return fmt.Errorf("solo %d de %d videos terminaron en %v", len(fin), n, espera)
	}
	return escribir(salida, "fase_workers", ventana, md.String())
}

func escribir(dir, nombre string, w window, md string) error {
	j, _ := json.MarshalIndent(w, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, nombre+"_ventana.json"), j, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, nombre+".md"), []byte(md), 0o644)
}

func truncar(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
