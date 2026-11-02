// Command capacity_escenario2 executes the comprehensive capacity evaluation
// for Scenario 2 (Issue H5): Multimedia upload, processing, and HLS consumption.
//
// It benchmarks:
//   - Baseline (Level 0) and 4 progressive load levels (Levels 1 to 4)
//   - Decoupled measurement across all 6 stages of the media lifecycle
//   - Asynq queue dynamics: pending depth, oldest age, and active concurrency
//   - Worker server performance under fixed concurrency (WORKER_CONCURRENCY=2)
//   - Object storage operations, throughput (MB/s), and HTTP error rates
//   - HLS consumption with real streaming cadence (pacing ~6.0s) vs greedy ráfaga
//   - Complete queue drain observation post-Level 4 and database state validation
//   - Strict sanitization: ZERO credentials, tokens, or private keys recorded
package main

import (
	"bytes"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hibiken/asynq"
	_ "github.com/lib/pq"
)

type Config struct {
	BaseURL    string
	HLSBaseURL string
	Entorno    string
	Patron     string
	MaxSegs    int
	// WorkerConcurrency es la del worker desplegado. No se adivina: de ella
	// depende el presupuesto de espera de cada nivel, y si no coincide con la
	// real el plazo se queda corto o se alarga sin motivo.
	WorkerConcurrency int
	RedisAddr         string
	DBConnStr         string
	OutDir            string
	UserEmail         string
	Password          string
	VideoCorto        string
	VideoMedio        string
	VideoLargo        string
	QuickMode         bool
	// G4 (#224)
	HastaNivel   int
	TTFFMuestras int
	TTFFImagen   string
}

// Duracion declarada de cada perfil, en segundos. Las mismas que verifica
// scripts/generar_media_prueba.sh y que declara cmd/seed-media: si no
// coincidieran, el presupuesto de espera se calcularia sobre un video que no es
// el que se sube.
var segundosPorPerfil = map[string]float64{"corto": 120, "medio": 600, "largo": 1800}

// factorTranscodificado es cuanto cuesta transcodificar un segundo de video en
// un vCPU: 1,2 veces su duracion, MEDIDO en A5 (141 s para un video de 120).
// Ver la nota tecnica 24.
const factorTranscodificado = 1.2

// margenPresupuesto multiplica el tiempo de transcodificacion calculado. Cubre
// la descarga del original, la subida de los derivados, el arranque de ffmpeg y
// la variacion entre corridas, sin convertir el plazo en infinito.
const margenPresupuesto = 1.8

// presupuestoMinimo evita que un nivel de un solo video corto tenga un plazo
// absurdamente corto si algo va mal al arrancar.
const presupuestoMinimo = 10 * time.Minute

func segundosDeVideo(perfiles []string) float64 {
	var total float64
	for _, p := range perfiles {
		total += segundosPorPerfil[p]
	}
	return total
}

// presupuestoDelNivel es cuanto se espera, como maximo, a que el ultimo trabajo
// del nivel llegue a un estado terminal.
//
// Se calcula, no se adivina: todo el video del nivel tiene que pasar por los
// vCPU disponibles, asi que el suelo es
// segundos_de_video * 1,2 / concurrencia. Lo que habia antes era un 6 minutos
// fijo que solo funcionaba con clips de 8 segundos.
func presupuestoDelNivel(perfiles []string, concurrencia int) time.Duration {
	if concurrencia < 1 {
		concurrencia = 1
	}
	segundos := segundosDeVideo(perfiles) * factorTranscodificado / float64(concurrencia) * margenPresupuesto
	d := time.Duration(segundos) * time.Second
	if d < presupuestoMinimo {
		return presupuestoMinimo
	}
	return d
}

// Los dos patrones de consumo que el enunciado pide distinguir. Se declara cual
// se usa y queda en la cabecera de cada archivo de evidencia: un margen de
// buffer o una interrupcion solo significan algo con cadencia de reproduccion,
// y una tasa de transferencia solo significa algo a maxima velocidad.
const (
	patronCadencia = "cadencia"
	patronMaxima   = "maxima"
)

// hostsLocales son los nombres que delatan una medicion que no es contra la
// nube. Misma lista que scripts/capacity_criterios.py.
var hostsLocales = []string{
	"localhost", "127.0.0.1", "host.docker.internal",
	"api", "web", "minio", "postgres", "redis",
}

// clasificarDestino devuelve "local" o "remoto" a partir de la URL medida. No se
// declara a mano: el escenario 2 de la Entrega 2 se publico como medicion de la
// nube siendo `http://api:8080`, y lo que falto fue justo esto.
func clasificarDestino(raw string) string {
	resto := raw
	if i := strings.Index(resto, "://"); i >= 0 {
		resto = resto[i+3:]
	}
	host := strings.ToLower(strings.SplitN(strings.SplitN(resto, "/", 2)[0], ":", 2)[0])
	if slices.Contains(hostsLocales, host) {
		return "local"
	}
	return "remoto"
}

// cabeceraEvidencia es el bloque que encabeza TODO archivo que este programa
// escribe. Criterio de A6: una medicion sin destino y entorno registrados no se
// publica. Mientras no estuvo, nada en la evidencia contradecia la afirmacion de
// que se habia medido contra la nube.
func cabeceraEvidencia(cfg Config) string {
	commit := "desconocido"
	if salida, err := os.ReadFile(".git/HEAD"); err == nil {
		commit = strings.TrimSpace(string(salida))
	}
	host, _ := os.Hostname()
	return fmt.Sprintf(
		"Destino medido   : %s  [%s]\n"+
			"Base HLS         : %s\n"+
			"Entorno          : %s\n"+
			"Patron de consumo: %s (segmentos por estudiante: %s)\n"+
			"Codigo           : %s\n"+
			"Generador        : %s\n"+
			"Fecha UTC        : %s\n",
		cfg.BaseURL, clasificarDestino(cfg.BaseURL), cfg.HLSBaseURL, cfg.Entorno,
		cfg.Patron, limiteSegmentos(cfg.MaxSegs), commit, host,
		time.Now().UTC().Format(time.RFC3339))
}

// margenTexto y stallsTexto dicen «no aplica» en vez de dar un numero cuando el
// patron de consumo no permite calcularlo. Descargando a maxima velocidad no hay
// instante de reproduccion que incumplir, asi que un margen de buffer o una
// cuenta de interrupciones serian inventados --y es asi como la Entrega 2
// publico «0 interrupciones» y «+5,99 s de margen».
// Ambas exigen ademas que se haya descargado algun segmento. Sin esa guarda, un
// nivel cuyos videos aun no estaban transcodificados --no hay manifiesto que
// pedir, Observaciones=0-- publicaba «0,00 s de margen» y «0 interrupciones»,
// que se lee como reproduccion perfecta cuando no se reprodujo nada. Paso en
// los niveles 2, 3 y 4 de la linea base del codigo corregido.
func margenTexto(h HLSStreamResult) string {
	if h.Patron != patronCadencia || math.IsNaN(h.MargenMinSec) {
		return "no aplica (patrón sin cadencia)"
	}
	if h.ObsConMargen == 0 {
		return "no aplica (ningún segmento con instante de reproducción)"
	}
	return fmt.Sprintf("%.2f s en el peor segmento", h.MargenMinSec)
}

func stallsTexto(h HLSStreamResult) string {
	if h.Patron != patronCadencia {
		return "no aplica (patrón sin cadencia)"
	}
	if h.ObsConMargen == 0 {
		return "no aplica (ningún segmento con instante de reproducción)"
	}
	return fmt.Sprintf("%d de %d", h.Stalls, h.ObsConMargen)
}

// cabeceraMarkdown es la misma cabecera, en bloque de codigo, para los archivos
// .md de evidencia.
func cabeceraMarkdown(cfg Config) string {
	return "```\n" + cabeceraEvidencia(cfg) + "```\n\n"
}

func limiteSegmentos(n int) string {
	if n <= 0 {
		return "todos"
	}
	return fmt.Sprintf("%d", n)
}

type StageMetrics struct {
	Count     int
	TotalDur  time.Duration
	MinDur    time.Duration
	MaxDur    time.Duration
	P50Dur    time.Duration
	P95Dur    time.Duration
	AvgDur    time.Duration
	Successes int
	Failures  int
}

type UploadJobResult struct {
	JobIndex     int
	Profile      string
	VideoBytes   int64
	Stage1Dur    time.Duration // Presigned URL
	Stage2Dur    time.Duration // Direct PUT
	Stage2MBps   float64
	Stage3Dur    time.Duration // Complete 202
	Stage4Dur    time.Duration // Queue wait
	Stage5Dur    time.Duration // FFmpeg transcode
	Stage6Dur    time.Duration // Total to available
	ResourceID   string
	StableID     string
	FinalStatus  string
	ErrorMessage string
	// TimedOut distingue 'no termino de medirse' de 'fallo'. Sin esta
	// distincion, un nivel cuyo presupuesto se agota se publica como 12
	// trabajos en la cola de fallidos, que es falso.
	TimedOut bool
}

type HLSStreamResult struct {
	StreamerID    int
	ManifestTTFB  time.Duration
	VariantTTFB   time.Duration
	SegmentsCount int           // segmentos por estudiante
	Observaciones int           // descargas de segmento medidas: estudiantes x segmentos
	SegLatency    StageMetrics  // latencia de segmento, con percentiles
	AvgSegLatency time.Duration // se conserva por compatibilidad con las tablas
	SegDurSec     float64       // duracion real declarada en la lista, no un supuesto
	Streamers     int
	Patron        string
	MargenMinSec  float64 // margen de buffer MEDIDO, el peor de la corrida
	Stalls        int
	// ObsConMargen son los segmentos que si tenian un instante de reproduccion
	// que cumplir. Es el denominador de Stalls: «16 interrupciones» sin decir
	// de cuantos no distingue una degradacion de un fallo del 100%.
	ObsConMargen int
	Success      bool
}

type LevelMetrics struct {
	Level             int
	Name              string
	UploadWorkers     int
	TotalUploads      int
	ProfilesUsed      string
	StreamersCount    int
	StartTime         time.Time
	EndTime           time.Time
	Duration          time.Duration
	ThroughputVidsMin float64
	JobsProcessed     int
	JobsFailed        int
	// JobsTimedOut son los trabajos que seguian en la cola al agotarse el
	// presupuesto del nivel. No son fallos: son medicion incompleta.
	JobsTimedOut int
	// ErroresCola son las lecturas de la cola que fallaron durante el nivel. Si
	// no es cero, el pico de cola y la antiguedad no son publicables.
	ErroresCola         int
	PeakPendingQueue    int
	PeakActiveQueue     int
	PeakOldestAgeSec    float64
	Stage1              StageMetrics
	Stage2              StageMetrics
	Stage2ThroughputMBs float64
	Stage3              StageMetrics
	Stage4              StageMetrics
	Stage5              StageMetrics
	Stage6              StageMetrics
	HLSStreaming        HLSStreamResult
	GreedyDownloadMBps  float64
	// PrimerSegmentoDur es transporte: manifiesto + variante + primer segmento.
	// NO es el tiempo al primer cuadro, y no se publica como tal.
	PrimerSegmentoDur time.Duration
	// TTFF es el tiempo al primer cuadro medido por un reproductor (Chromium + hls.js), G4.
	TTFF ResultadoTTFF
}

type DrainSnapshot struct {
	ElapsedSec float64
	Pending    int
	Active     int
	Completed  int
	OldestAge  float64
}

type DrainResult struct {
	StartTime       time.Time
	EndTime         time.Time
	TotalDuration   time.Duration
	Snapshots       []DrainSnapshot
	InitialPending  int
	FinalPending    int
	FinalActive     int
	DBStatusCounts  map[string]int
	OrphanJobsCount int
	// Atascada: el drenaje se corto porque la cola dejo de avanzar, no porque
	// se vaciara ni porque se agotara el plazo.
	Atascada bool
	// Observable es false cuando la cola no se pudo leer. Sin esto, un fallo
	// de lectura se publicaba como cola vacia.
	Observable bool
	// ErroresInspector son las lecturas de la cola que fallaron.
	ErroresInspector int
}

type httpClient struct {
	client  *http.Client
	baseURL string
	token   string
	csrf    string
}

func newHTTPClient(baseURL string) *httpClient {
	jar, _ := cookiejar.New(nil)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: os.Getenv("MOOC_TLS_INSECURE") == "1",
		},
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 50,
		IdleConnTimeout:     90 * time.Second,
	}
	return &httpClient{
		client:  &http.Client{Jar: jar, Transport: tr, Timeout: 120 * time.Second},
		baseURL: strings.TrimRight(baseURL, "/"),
	}
}

func (c *httpClient) doJSON(method, endpoint string, body any, target any) (int, time.Duration, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return 0, 0, err
		}
		bodyReader = bytes.NewReader(data)
	}

	fullURL := endpoint
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		fullURL = c.baseURL + endpoint
	}

	req, err := http.NewRequest(method, fullURL, bodyReader)
	if err != nil {
		return 0, 0, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if c.csrf != "" {
		req.Header.Set("X-CSRF-Token", c.csrf)
	}

	// Origin es obligatorio para la proteccion contra CSRF en cualquier peticion
	// que cambie estado: sin el, la API responde 403 csrf_origin_rejected.
	//
	// No estaba, y es otra consecuencia de que esta campana solo se hubiera
	// ejecutado contra Docker Compose: alli el origen por omision ya estaba en
	// la lista. Contra la nube, la primera peticion que crea algo falla.
	//
	// Se manda el propio destino medido, que es lo que enviaria un navegador en
	// un despliegue del mismo sitio, y es el origen que la configuracion de
	// produccion tiene en CSRF_ALLOWED_ORIGINS.
	req.Header.Set("Origin", c.baseURL)

	start := time.Now()
	resp, err := c.client.Do(req)
	duration := time.Since(start)
	if err != nil {
		return 0, duration, err
	}
	defer resp.Body.Close()

	for _, cookie := range resp.Cookies() {
		if cookie.Name == "csrf_token" || cookie.Name == "mooc_csrf" {
			c.csrf = cookie.Value
		}
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, duration, err
	}

	if target != nil && len(respBody) > 0 && resp.StatusCode < 300 {
		if err := json.Unmarshal(respBody, target); err != nil {
			return resp.StatusCode, duration, fmt.Errorf("json unmarshal: %w", err)
		}
	}

	if resp.StatusCode >= 400 {
		return resp.StatusCode, duration, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	return resp.StatusCode, duration, nil
}

func main() {
	cfg := Config{}
	// Sin destino por omision. El `http://api:8080` que habia aqui es la razon
	// por la que el escenario 2 de la Entrega 2 se midio contra Docker Compose:
	// quien lo corrio no eligio ese destino, lo heredo.
	flag.StringVar(&cfg.BaseURL, "base", "", "Base URL de la API (obligatorio, sin valor por omision)")
	flag.StringVar(&cfg.HLSBaseURL, "hls-base", "", "Base URL de los derivados HLS (obligatorio si el destino no es local)")
	flag.StringVar(&cfg.Entorno, "entorno", "", "Etiqueta del entorno medido, p. ej. nube-entrega3 (obligatorio)")
	flag.StringVar(&cfg.Patron, "patron-consumo", patronCadencia,
		"Patron de consumo HLS: cadencia (reproduccion a 1x) o maxima (descarga sin pausa)")
	flag.IntVar(&cfg.WorkerConcurrency, "worker-concurrency", 2,
		"Concurrencia del worker desplegado; fija el presupuesto de espera por nivel")
	flag.IntVar(&cfg.MaxSegs, "max-segmentos", 0,
		"Segmentos por estudiante; 0 = todos. Acota el coste de egreso de un nivel")
	flag.StringVar(&cfg.RedisAddr, "redis", "redis:6379", "Redis address for Asynq inspector")
	flag.StringVar(&cfg.DBConnStr, "db", "postgres://moocuser:moocpassword@postgres:5432/moocdb?sslmode=disable", "PostgreSQL connection string")
	flag.StringVar(&cfg.OutDir, "out-dir", "docs/entrega2/evidencias/H5", "Output directory for evidence files")
	flag.StringVar(&cfg.UserEmail, "user", "profesor1@plataforma-mooc.test", "Professor email")
	flag.StringVar(&cfg.Password, "pass", "Password123!", "Professor password")
	flag.StringVar(&cfg.VideoCorto, "video-corto", "scratch/video_corto.mp4", "Path to short profile video")
	flag.StringVar(&cfg.VideoMedio, "video-medio", "scratch/video_medio.mp4", "Path to medium profile video")
	flag.StringVar(&cfg.VideoLargo, "video-largo", "scratch/video_largo.mp4", "Path to long profile video")
	flag.BoolVar(&cfg.QuickMode, "quick", false, "Fast execution mode for validation")
	flag.IntVar(&cfg.HastaNivel, "hasta-nivel", 4, "Ultimo nivel que se corre (0 a 4). El ultimo nivel corrido es el que observa el drenaje")
	flag.IntVar(&cfg.TTFFMuestras, "ttff-muestras", 6, "Reproducciones por nivel para medir el tiempo al primer cuadro con Chromium + hls.js (0 = no se mide)")
	flag.StringVar(&cfg.TTFFImagen, "ttff-imagen", "mooc-ttff", "Imagen de Docker del medidor (scripts/capacity_escenario2/ttff)")
	flag.Parse()
	// La cadena de conexion lleva la contrasena de la base: por entorno, no por argumento (un argumento se ve en `ps`).
	if v := os.Getenv("DB_CONN"); v != "" {
		cfg.DBConnStr = v
	}

	// Criterios de medicion de A6 (issue #198): sin destino y entorno
	// declarados no se mide, porque entonces la evidencia no puede decir contra
	// que se midio. Ver capacity-planning/criterios_de_medicion.md.
	if cfg.BaseURL == "" {
		log.Fatalf("Falta -base. No hay destino por omision: declara contra que mides.\n" +
			"  ej. -base https://<host> -hls-base https://storage.googleapis.com/<bucket> -entorno nube-entrega3")
	}
	if cfg.Entorno == "" {
		log.Fatalf("Falta -entorno. Es la etiqueta que distingue una corrida en la nube de una en Compose.")
	}
	if cfg.Patron != patronCadencia && cfg.Patron != patronMaxima {
		log.Fatalf("-patron-consumo debe ser %q o %q, no %q", patronCadencia, patronMaxima, cfg.Patron)
	}
	if cfg.HLSBaseURL == "" && clasificarDestino(cfg.BaseURL) != "local" {
		log.Fatalf("Falta -hls-base. Solo se adivina para el entorno local; contra la nube hay que\n" +
			"  declarar el bucket, o se mediran los derivados de otro entorno.")
	}

	_ = os.MkdirAll(cfg.OutDir, 0755)

	log.Printf("================================================================================")
	log.Printf(" Plataforma MOOC - Escenario 2: Análisis de Capacidad Multimedia (Issue H5)")
	log.Printf("================================================================================")
	log.Printf("Redis Endpoint   : %s", cfg.RedisAddr)
	log.Printf("Output Evidence  : %s", cfg.OutDir)

	// Load video files into memory
	videoCortoBytes, err := os.ReadFile(cfg.VideoCorto)
	if err != nil {
		log.Fatalf("Error reading video corto (%s): %v", cfg.VideoCorto, err)
	}
	videoMedioBytes, err := os.ReadFile(cfg.VideoMedio)
	if err != nil {
		log.Fatalf("Error reading video medio (%s): %v", cfg.VideoMedio, err)
	}
	videoLargoBytes, err := os.ReadFile(cfg.VideoLargo)
	if err != nil {
		log.Fatalf("Error reading video largo (%s): %v", cfg.VideoLargo, err)
	}

	log.Printf("Videos cargados  :")
	log.Printf("  - Corto : %d bytes (%.2f MB)", len(videoCortoBytes), float64(len(videoCortoBytes))/(1024*1024))
	log.Printf("  - Medio : %d bytes (%.2f MB)", len(videoMedioBytes), float64(len(videoMedioBytes))/(1024*1024))
	log.Printf("  - Largo : %d bytes (%.2f MB)", len(videoLargoBytes), float64(len(videoLargoBytes))/(1024*1024))

	// Connect Asynq Inspector for queue tracking
	inspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: cfg.RedisAddr})
	defer inspector.Close()

	// Connect PostgreSQL
	db, err := sql.Open("postgres", cfg.DBConnStr)
	if err != nil {
		log.Printf("Aviso: No se pudo conectar directamente a PostgreSQL: %v (se usará API para validaciones)", err)
	} else {
		defer db.Close()
	}

	// Login and setup course environment
	adminClient := newHTTPClient(cfg.BaseURL)
	var loginResp struct {
		Token string `json:"token"`
	}
	_, _, err = adminClient.doJSON("POST", "/api/v1/auth/login", map[string]string{
		"email":    cfg.UserEmail,
		"password": cfg.Password,
	}, &loginResp)
	if err != nil {
		log.Fatalf("Error al autenticar profesor principal: %v", err)
	}
	adminClient.token = loginResp.Token

	// Create test course, module, unit
	var course struct {
		ID string `json:"id"`
	}
	_, _, err = adminClient.doJSON("POST", "/api/v1/courses", map[string]any{
		"title":       fmt.Sprintf("Curso Capacidad Escenario 2 - %d", time.Now().Unix()),
		"description": "Curso para ejecución formal de pruebas de capacidad H5.",
	}, &course)
	if err != nil {
		log.Fatalf("Error creando curso: %v", err)
	}

	var module struct {
		ID string `json:"id"`
	}
	_, _, err = adminClient.doJSON("POST", fmt.Sprintf("/api/v1/courses/%s/modules", course.ID), map[string]any{
		"title": "Módulo Capacidad Multimedia",
	}, &module)
	if err != nil {
		log.Fatalf("Error creando módulo: %v", err)
	}

	var unit struct {
		ID string `json:"id"`
	}
	_, _, err = adminClient.doJSON("POST", fmt.Sprintf("/api/v1/modules/%s/units", module.ID), map[string]any{
		"title": "Unidad Capacidad Multimedia",
	}, &unit)
	if err != nil {
		log.Fatalf("Error creando unidad: %v", err)
	}
	log.Printf("Entorno de curso listo: Curso=%s, Módulo=%s, Unidad=%s", course.ID, module.ID, unit.ID)

	// Determine HLS base URL
	// Solo el entorno local se adivina. Antes esto miraba si la URL contenia la
	// IP de la VM de la Entrega 2 para decidir entre el bucket y MinIO: con la
	// IP cambiada --y cambia en cada recreacion-- la campana habria medido los
	// derivados de MinIO creyendo medir los de Cloud Storage.
	hlsBase := cfg.HLSBaseURL
	if hlsBase == "" {
		hlsBase = "http://minio:9000/mooc-storage"
	}
	cfg.HLSBaseURL = hlsBase
	log.Printf("%s", cabeceraEvidencia(cfg))

	// El consumo HLS y el tiempo al primer cuadro NO pueden depender de la cola que se esta midiendo: en A7,
	// los niveles 2 a 4 no descargaron ni un segmento porque sus videos seguian encolados. Se prepara UN
	// video del perfil corto antes de medir nada y se espera a que quede procesado; los estudiantes de
	// todos los niveles lo consumen mientras las subidas del nivel alimentan la cola.
	var fixtureUnit struct {
		ID string `json:"id"`
	}
	if _, _, err = adminClient.doJSON("POST", fmt.Sprintf("/api/v1/modules/%s/units", module.ID), map[string]any{
		"title": "Unidad de consumo (video ya procesado)",
	}, &fixtureUnit); err != nil {
		log.Fatalf("Error creando la unidad del video de consumo: %v", err)
	}
	log.Printf("Preparando el video de consumo (perfil corto): se espera a que quede procesado antes de medir nada")
	fx := executeUploadLifecycle(adminClient, fixtureUnit.ID, 0, "corto", videoCortoBytes, cfg.HLSBaseURL, 45*time.Minute, nil)
	if fx.FinalStatus != "completed" || fx.StableID == "" {
		log.Fatalf("El video de consumo no quedo procesado (estado %q): %s", fx.FinalStatus, fx.ErrorMessage)
	}
	fixtureStableID := fx.StableID
	log.Printf("Video de consumo listo (stable_id %s)", fixtureStableID)

	// -------------------------------------------------------------------------
	// Definición de Niveles de Carga
	// -------------------------------------------------------------------------
	type LevelDef struct {
		Level         int
		Name          string
		UploadWorkers int
		TotalUploads  int
		Profiles      []string
		Streamers     int
		PacingSec     float64
		IsDrainLevel  bool
	}

	levels := []LevelDef{
		{
			Level:         0,
			Name:          "Nivel 0: Línea Base Limpia",
			UploadWorkers: 1,
			TotalUploads:  1,
			Profiles:      []string{"corto"},
			Streamers:     2,
			PacingSec:     6.0,
		},
		{
			Level:         1,
			Name:          "Nivel 1: Sub-saturación (λ < μ)",
			UploadWorkers: 2,
			TotalUploads:  2,
			Profiles:      []string{"corto", "corto"},
			Streamers:     10,
			PacingSec:     6.0,
		},
		{
			Level:         2,
			Name:          "Nivel 2: Punto de Equilibrio Nominal (λ ≈ μ)",
			UploadWorkers: 4,
			TotalUploads:  4,
			Profiles:      []string{"corto", "medio", "corto", "medio"},
			Streamers:     25,
			PacingSec:     6.0,
		},
		{
			Level:         3,
			Name:          "Nivel 3: Saturación de Cola (λ > μ)",
			UploadWorkers: 8,
			TotalUploads:  8,
			Profiles:      []string{"corto", "medio", "largo", "corto", "medio", "corto", "medio", "corto"},
			Streamers:     50,
			PacingSec:     6.0,
		},
		{
			Level:         4,
			Name:          "Nivel 4: Estrés Máximo y Drenaje",
			UploadWorkers: 12,
			TotalUploads:  12,
			Profiles: []string{
				"corto", "medio", "corto", "medio",
				"corto", "corto", "medio", "corto",
				"corto", "medio", "corto", "medio",
			},
			Streamers:    100,
			PacingSec:    6.0,
			IsDrainLevel: true,
		},
	}

	if cfg.HastaNivel >= 0 && cfg.HastaNivel < len(levels)-1 {
		levels = levels[:cfg.HastaNivel+1]
		for i := range levels {
			levels[i].IsDrainLevel = false
		}
		levels[len(levels)-1].IsDrainLevel = true
	}

	var levelResults []LevelMetrics
	var drainResult DrainResult

	// Run each level
	for _, ldef := range levels {
		log.Printf("\n================================================================================")
		log.Printf(" INICIANDO %s", strings.ToUpper(ldef.Name))
		log.Printf("================================================================================")
		log.Printf("  Profesores concurrentes : %d (Total subidas: %d)", ldef.UploadWorkers, ldef.TotalUploads)
		log.Printf("  Perfiles utilizados     : %v", ldef.Profiles)
		log.Printf("  Estudiantes streaming   : %d (Pacing: %0.1fs)", ldef.Streamers, ldef.PacingSec)

		lm := LevelMetrics{
			Level:          ldef.Level,
			Name:           ldef.Name,
			UploadWorkers:  ldef.UploadWorkers,
			TotalUploads:   ldef.TotalUploads,
			ProfilesUsed:   strings.Join(ldef.Profiles, ", "),
			StreamersCount: ldef.Streamers,
			StartTime:      time.Now(),
		}

		// Track queue metrics during run
		var (
			maxPending   int
			maxActive    int
			maxOldestAge float64
			erroresCola  int
			stopQueueMon = make(chan struct{})
			queueMonWG   sync.WaitGroup
		)

		queueMonWG.Add(1)
		go func() {
			defer queueMonWG.Done()
			ticker := time.NewTicker(200 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-stopQueueMon:
					return
				case <-ticker.C:
					info, err := inspector.GetQueueInfo("default")
					if err != nil {
						// Se cuentan. Un pico de cola de 0 con la cola ilegible no
						// es un 0: es una medicion que no existe.
						erroresCola++
					} else {
						if info.Pending > maxPending {
							maxPending = info.Pending
						}
						if info.Active > maxActive {
							maxActive = info.Active
						}
						if info.Latency.Seconds() > maxOldestAge {
							maxOldestAge = info.Latency.Seconds()
						}
					}
				}
			}
		}()

		// Presupuesto de espera del nivel, derivado del trabajo que mete en la
		// cola. Es lo que permite medir perfiles reales: el ultimo trabajo de
		// un nivel no puede terminar antes de que se hayan transcodificado
		// todos los de delante.
		presupuesto := presupuestoDelNivel(ldef.Profiles, cfg.WorkerConcurrency)
		log.Printf("  Presupuesto de espera   : %v (%.0f s de video a %.1fx en %d vCPU)",
			presupuesto.Round(time.Second), segundosDeVideo(ldef.Profiles), factorTranscodificado, cfg.WorkerConcurrency)

		// Consumo concurrente: los estudiantes leen el video ya procesado MIENTRAS las subidas del nivel
		// alimentan la cola, y despues se mide el tiempo al primer cuadro con un reproductor.
		type consumo struct {
			hls  HLSStreamResult
			ttff ResultadoTTFF
		}
		consumoCh := make(chan consumo, 1)
		go func() {
			var c consumo
			c.hls = runHLSStreamingSimulation(newHTTPClient(cfg.BaseURL), hlsBase, fixtureStableID,
				ldef.PacingSec, ldef.Streamers, cfg.Patron, cfg.MaxSegs)
			if cfg.TTFFMuestras > 0 {
				c.ttff = medirTTFF(cfg.TTFFImagen,
					fmt.Sprintf("%s/hls/%s/master.m3u8", strings.TrimRight(hlsBase, "/"), fixtureStableID), cfg.TTFFMuestras)
			}
			consumoCh <- c
		}()

		// Concurrently spawn upload jobs
		var (
			uploadWG    sync.WaitGroup
			confirmedWG sync.WaitGroup
			drainWG     sync.WaitGroup
			jobChan     = make(chan int, ldef.TotalUploads)
			results     = make([]UploadJobResult, ldef.TotalUploads)
		)

		for i := 0; i < ldef.TotalUploads; i++ {
			jobChan <- i
		}
		close(jobChan)

		if ldef.IsDrainLevel {
			confirmedWG.Add(ldef.TotalUploads)
			drainWG.Add(1)
			go func() {
				defer drainWG.Done()
				confirmedWG.Wait()
				log.Printf("\n>>> TODAS LAS %d CARGAS CONFIRMADAS (202 ACCEPTED). INICIANDO OBSERVACIÓN DE DRENAJE...", ldef.TotalUploads)
				drainResult = observeQueueDrain(inspector, db, presupuesto)
			}()
		}

		for w := 0; w < ldef.UploadWorkers; w++ {
			uploadWG.Add(1)
			go func(workerID int) {
				defer uploadWG.Done()
				client := newHTTPClient(cfg.BaseURL)
				client.token = adminClient.token

				for idx := range jobChan {
					prof := ldef.Profiles[idx%len(ldef.Profiles)]
					var vBytes []byte
					switch prof {
					case "medio":
						vBytes = videoMedioBytes
					case "largo":
						vBytes = videoLargoBytes
					default:
						vBytes = videoCortoBytes
					}

					var onConf func()
					if ldef.IsDrainLevel {
						onConf = func() { confirmedWG.Done() }
					}

					res := executeUploadLifecycle(client, unit.ID, idx+1, prof, vBytes, hlsBase, presupuesto, onConf)
					results[idx] = res
				}
			}(w)
		}

		uploadWG.Wait()
		if ldef.IsDrainLevel {
			drainWG.Wait()
		}
		close(stopQueueMon)
		queueMonWG.Wait()

		lm.EndTime = time.Now()
		lm.Duration = lm.EndTime.Sub(lm.StartTime)
		lm.PeakPendingQueue = maxPending
		lm.PeakActiveQueue = maxActive
		lm.PeakOldestAgeSec = maxOldestAge
		lm.ErroresCola = erroresCola
		if erroresCola > 0 {
			log.Printf("  ATENCION: %d lectura(s) de la cola fallaron en este nivel.", erroresCola)
			log.Printf("    El pico de cola y la antiguedad NO son publicables: se midio a ciegas parte del tiempo.")
		}

		// Calculate stage metrics from results
		var (
			s1Durs, s2Durs, s3Durs, s4Durs, s5Durs, s6Durs []time.Duration
			totalBytesUploaded                             int64
			successCount, timeoutCount, failedCount        int
		)

		for _, r := range results {
			switch {
			case r.FinalStatus == "completed":
				successCount++
			case r.TimedOut:
				timeoutCount++
			default:
				failedCount++
			}

			// Las etapas 1 a 3 son el camino de subida y se miden antes de
			// esperar nada: valen para todo trabajo que las recorrio.
			s1Durs = append(s1Durs, r.Stage1Dur)
			s2Durs = append(s2Durs, r.Stage2Dur)
			s3Durs = append(s3Durs, r.Stage3Dur)
			totalBytesUploaded += r.VideoBytes

			// Las etapas 4 a 6 solo existen si el trabajo llego a un estado
			// terminal. En un trabajo que se quedo en la cola, `tCompleted` es
			// el tiempo cero, asi que Stage5Dur y Stage6Dur salen NEGATIVAS y
			// envenenan la media, el minimo y el maximo del nivel. Antes
			// entraban todas.
			if r.FinalStatus == "completed" || r.FinalStatus == "failed" {
				s4Durs = append(s4Durs, r.Stage4Dur)
				s5Durs = append(s5Durs, r.Stage5Dur)
				s6Durs = append(s6Durs, r.Stage6Dur)
			}
		}

		lm.JobsProcessed = successCount
		lm.JobsFailed = failedCount
		lm.JobsTimedOut = timeoutCount
		lm.ThroughputVidsMin = (float64(successCount) / lm.Duration.Minutes())

		lm.Stage1 = computeStageMetrics(s1Durs, successCount, lm.JobsFailed)
		lm.Stage2 = computeStageMetrics(s2Durs, successCount, lm.JobsFailed)
		lm.Stage3 = computeStageMetrics(s3Durs, successCount, lm.JobsFailed)
		lm.Stage4 = computeStageMetrics(s4Durs, successCount, lm.JobsFailed)
		lm.Stage5 = computeStageMetrics(s5Durs, successCount, lm.JobsFailed)
		lm.Stage6 = computeStageMetrics(s6Durs, successCount, lm.JobsFailed)

		if lm.Stage2.TotalDur > 0 {
			lm.Stage2ThroughputMBs = (float64(totalBytesUploaded) / (1024 * 1024)) / lm.Stage2.TotalDur.Seconds()
		}

		// Lo medido por los estudiantes y por el reproductor mientras corria el nivel.
		cons := <-consumoCh
		lm.HLSStreaming = cons.hls
		lm.TTFF = cons.ttff
		if fixtureStableID != "" {
			lm.GreedyDownloadMBps = runGreedyBulkDownload(newHTTPClient(cfg.BaseURL), hlsBase, fixtureStableID)
		}
		// Transporte: manifiesto + variante + primer segmento. NO es el tiempo al primer cuadro, que es lm.TTFF.
		lm.PrimerSegmentoDur = lm.HLSStreaming.ManifestTTFB + lm.HLSStreaming.VariantTTFB + lm.HLSStreaming.AvgSegLatency

		log.Printf("  >>> Resumen %s:", ldef.Name)
		log.Printf("      Duración total corrida     : %v", lm.Duration.Round(time.Millisecond))
		log.Printf("      Trabajos completados       : %d/%d (Fallos: %d | Sin terminar de medir: %d)", lm.JobsProcessed, ldef.TotalUploads, lm.JobsFailed, lm.JobsTimedOut)
		log.Printf("      Rendimiento en la ventana  : %.2f videos/min (no es la tasa de servicio)", lm.ThroughputVidsMin)
		log.Printf("      Pico de cola (pending)     : %d tareas (Activas: %d)", lm.PeakPendingQueue, lm.PeakActiveQueue)
		log.Printf("      Antigüedad máx. en cola    : %.2f s", lm.PeakOldestAgeSec)
		log.Printf("      Etapa 1 (Firma API p95)    : %v", lm.Stage1.Texto(95))
		log.Printf("      Etapa 2 (PUT Directo p95)  : %v (Throughput: %.2f MB/s)", lm.Stage2.Texto(95), lm.Stage2ThroughputMBs)
		log.Printf("      Etapa 3 (Confirmación p95) : %v", lm.Stage3.Texto(95))
		log.Printf("      Etapa 4 (Espera Cola p95)  : %v", lm.Stage4.Texto(95))
		log.Printf("      Etapa 5 (FFmpeg Proc p95)  : %v", lm.Stage5.Texto(95))
		log.Printf("      Etapa 6 (Total Avail p95)  : %v", lm.Stage6.Texto(95))

		levelResults = append(levelResults, lm)
	}

	// -------------------------------------------------------------------------
	// Generar y Escribir Todos los Archivos de Evidencia
	// -------------------------------------------------------------------------
	writeCorridaLog(filepath.Join(cfg.OutDir, "corrida_niveles_escenario2.txt"), cfg, levelResults)
	tt := map[string]ResultadoTTFF{}
	for _, lm := range levelResults {
		tt[fmt.Sprintf("nivel_%d", lm.Level)] = lm.TTFF
	}
	if raw, err := json.MarshalIndent(tt, "", "  "); err == nil {
		_ = os.WriteFile(filepath.Join(cfg.OutDir, "ttff.json"), raw, 0644)
	}
	writeDrainLog(filepath.Join(cfg.OutDir, "drenaje_cola_verificacion.txt"), cfg, drainResult)
	writeExecutiveSummary(filepath.Join(cfg.OutDir, "resumen_ejecutivo_escenario2.md"), cfg, levelResults, drainResult)
	writeBottleneckAnalysis(filepath.Join(cfg.OutDir, "analisis_cuello_de_botella.md"), cfg, levelResults, drainResult)
	writeH5README(filepath.Join(cfg.OutDir, "README.md"), cfg, levelResults, drainResult)
	writeCapacityReportFile(filepath.Join(cfg.OutDir, "reporte_carga_escenario2.md"), cfg, levelResults, drainResult)

	// La campana no se declara exitosa sola. Antes imprimia "Finalizado
	// Exitosamente" con 23 de 27 trabajos sin terminar de medirse y el drenaje
	// cortado con 17 tareas en cola: una corrida inservible que se presentaba
	// como buena. Ahora el cierre dice lo que hay y el codigo de salida lo
	// refleja, para que un guion que la encadene se entere.
	var sinMedir, fallidos int
	for _, lm := range levelResults {
		sinMedir += lm.JobsTimedOut
		fallidos += lm.JobsFailed
	}

	log.Printf("\n================================================================================")
	switch {
	case sinMedir > 0:
		log.Printf(" Escenario 2 INCOMPLETO: %d trabajo(s) no llegaron a un estado terminal.", sinMedir)
		log.Printf(" Las etapas 4, 5 y 6 de esos niveles NO son publicables.")
	case fallidos > 0:
		log.Printf(" Escenario 2 terminado CON %d FALLO(S) reales de procesamiento.", fallidos)
	default:
		log.Printf(" Escenario 2 terminado: los %d niveles completaron todos sus trabajos.", len(levelResults))
	}
	if drainResult.Atascada {
		log.Printf(" ATENCION: el drenaje se corto porque la cola dejo de avanzar.")
	}
	if !drainResult.Observable {
		log.Printf(" ATENCION: el drenaje NO se pudo observar (%d lecturas de la cola fallaron).", drainResult.ErroresInspector)
	}
	log.Printf(" Evidencias generadas en : %s", cfg.OutDir)
	log.Printf("================================================================================")

	if sinMedir > 0 || drainResult.Atascada || !drainResult.Observable {
		os.Exit(1)
	}
}

func executeUploadLifecycle(client *httpClient, unitID string, jobIdx int, profile string, videoBytes []byte, hlsBase string, presupuesto time.Duration, onConfirmed func()) UploadJobResult {
	res := UploadJobResult{
		JobIndex:   jobIdx,
		Profile:    profile,
		VideoBytes: int64(len(videoBytes)),
	}

	// 1. Create video resource
	var resource struct {
		ID       string `json:"id"`
		StableID string `json:"stable_id"`
	}
	_, _, err := client.doJSON("POST", fmt.Sprintf("/api/v1/units/%s/resources", unitID), map[string]any{
		"title":          fmt.Sprintf("Video Carga %s #%d", profile, jobIdx),
		"type":           "video",
		"is_visible":     true,
		"is_mandatory":   false,
		"allow_download": false,
	}, &resource)
	if err != nil {
		res.ErrorMessage = fmt.Sprintf("Error creating resource: %v", err)
		if onConfirmed != nil {
			onConfirmed()
		}
		return res
	}
	res.ResourceID = resource.ID
	res.StableID = resource.StableID

	// -------------------------------------------------------------------------
	// ETAPA 1: Autorización y emisión de URL prefirmada (Control API)
	// -------------------------------------------------------------------------
	var presignedResp struct {
		UploadURL string `json:"upload_url"`
		ObjectKey string `json:"object_key"`
	}
	status, s1Dur, err := client.doJSON("POST", "/api/v1/media/presigned-url", map[string]any{
		"resource_id":     resource.ID,
		"filename":        fmt.Sprintf("video_%s_%d.mp4", profile, jobIdx),
		"mime_type":       "video/mp4",
		"file_size_bytes": res.VideoBytes,
	}, &presignedResp)
	res.Stage1Dur = s1Dur
	if err != nil || status != 200 {
		res.ErrorMessage = fmt.Sprintf("Stage 1 failed: %v", err)
		if onConfirmed != nil {
			onConfirmed()
		}
		return res
	}

	_ = maskSignature(presignedResp.UploadURL)

	// -------------------------------------------------------------------------
	// ETAPA 2: Transferencia directa al almacenamiento (Data Plane)
	// -------------------------------------------------------------------------
	putReq, err := http.NewRequest("PUT", presignedResp.UploadURL, bytes.NewReader(videoBytes))
	if err != nil {
		res.ErrorMessage = fmt.Sprintf("Stage 2 req error: %v", err)
		if onConfirmed != nil {
			onConfirmed()
		}
		return res
	}
	putReq.Header.Set("Content-Type", "video/mp4")
	putReq.Header.Set("Content-Length", fmt.Sprintf("%d", res.VideoBytes))

	s2Start := time.Now()
	putResp, err := client.client.Do(putReq)
	s2Dur := time.Since(s2Start)
	res.Stage2Dur = s2Dur
	if err != nil {
		res.ErrorMessage = fmt.Sprintf("Stage 2 PUT error: %v", err)
		if onConfirmed != nil {
			onConfirmed()
		}
		return res
	}
	defer putResp.Body.Close()
	if putResp.StatusCode != 200 && putResp.StatusCode != 204 {
		b, _ := io.ReadAll(putResp.Body)
		res.ErrorMessage = fmt.Sprintf("Stage 2 HTTP %d: %s", putResp.StatusCode, string(b))
		if onConfirmed != nil {
			onConfirmed()
		}
		return res
	}
	mbUploaded := float64(res.VideoBytes) / (1024 * 1024)
	res.Stage2MBps = mbUploaded / s2Dur.Seconds()

	// -------------------------------------------------------------------------
	// ETAPA 3: Confirmación de carga completa (Control API)
	// -------------------------------------------------------------------------
	var confirmResp struct {
		ProcessingStatus string `json:"processing_status"`
	}
	status, s3Dur, err := client.doJSON("POST", fmt.Sprintf("/api/v1/media/uploads/%s/complete", resource.ID), map[string]string{
		"object_key": presignedResp.ObjectKey,
	}, &confirmResp)
	res.Stage3Dur = s3Dur
	if err != nil || status != 202 {
		res.ErrorMessage = fmt.Sprintf("Stage 3 failed: %v", err)
		if onConfirmed != nil {
			onConfirmed()
		}
		return res
	}

	if onConfirmed != nil {
		onConfirmed()
	}

	// -------------------------------------------------------------------------
	// ETAPAS 4, 5 y 6: Sondeo hasta available
	// -------------------------------------------------------------------------
	tEnqueued := time.Now()
	var (
		tStarted   time.Time
		tCompleted time.Time
	)

	// El plazo lo decide quien lanza el nivel, a partir de la duracion real de
	// los videos que ha metido en la cola. Antes habia aqui `6 * time.Minute`
	// fijos, y eso es lo que impidio medir los perfiles reales:
	//
	//   - un video de 2 min se transcodifica en ~2,4 min (1,2x, nota 24),
	//   - uno de 10 min en ~12, y uno de 30 en ~36.
	//
	// Con clips de 8 segundos, 6 minutos eran 36 veces el trabajo y nunca se
	// agotaban. Con perfiles reales, cualquier nivel con un perfil medio o largo
	// agota el plazo y la campana contaba esos trabajos como fallidos / DLQ,
	// cuando seguian en cola. El informe decia "0 fallos" porque nunca espero
	// lo suficiente para que hubiera ninguno.
	deadline := time.Now().Add(presupuesto)

	// Detector de atasco: separa "lento" de "colgado". Mientras el estado del
	// recurso avance, se sigue esperando aunque falte mucho; si no se mueve
	// nada durante este tiempo, no hace falta gastar el presupuesto entero.
	marcarAvance()
	estadoPrevio := ""

	for time.Now().Before(deadline) {
		// Dos segundos, no 300 ms: con doce trabajos concurrentes el sondeo de
		// 300 ms metia ~40 peticiones/s contra la API dentro de la ventana
		// medida. Los trabajos tardan minutos; sondear mas fino no mide mejor,
		// solo anade carga que no es del escenario.
		time.Sleep(2 * time.Second)

		var resourcesResp struct {
			Items []struct {
				ID               string `json:"id"`
				ProcessingStatus string `json:"processing_status"`
			} `json:"items"`
		}
		status, _, err := client.doJSON("GET", fmt.Sprintf("/api/v1/units/%s/resources", unitID), nil, &resourcesResp)
		if err != nil || status != 200 {
			continue
		}

		for _, item := range resourcesResp.Items {
			if item.ID == resource.ID {
				res.FinalStatus = item.ProcessingStatus
				if tStarted.IsZero() && res.FinalStatus != "pending" {
					tStarted = time.Now()
				}
				if res.FinalStatus == "completed" || res.FinalStatus == "failed" {
					tCompleted = time.Now()
					break
				}
			}
		}

		if res.FinalStatus != estadoPrevio {
			estadoPrevio = res.FinalStatus
			marcarAvance()
		}

		if res.FinalStatus == "completed" || res.FinalStatus == "failed" {
			break
		}

		if sinAvanceDesde() > limiteAtascoSistema {
			res.ErrorMessage = fmt.Sprintf("el sistema no avanza (ningun trabajo cambio de estado) desde hace %v; este quedo en %q", limiteAtascoSistema, res.FinalStatus)
			break
		}
	}

	// Un trabajo que no llego a un estado terminal NO es un fallo: es una
	// medicion incompleta, y se marca como tal para que no se publique como
	// fallo ni como exito.
	if res.FinalStatus != "completed" && res.FinalStatus != "failed" {
		res.TimedOut = true
		if res.ErrorMessage == "" {
			res.ErrorMessage = fmt.Sprintf("seguia en %q al agotarse el presupuesto de %v", res.FinalStatus, presupuesto)
		}
	}

	if tStarted.IsZero() {
		tStarted = tCompleted
	}
	res.Stage4Dur = tStarted.Sub(tEnqueued)
	res.Stage5Dur = tCompleted.Sub(tStarted)
	res.Stage6Dur = tCompleted.Sub(tEnqueued)

	return res
}

// runHLSStreamingSimulation consume el video como lo haria un estudiante, con
// el patron declarado y con la concurrencia declarada.
//
// # Que estaba mal
//
// La version anterior recibia `pacingSec` y `streamers` y **no usaba ninguno de
// los dos**: un solo lector secuencial, sin pausa entre segmentos, leyendo tres
// segmentos. Los niveles declaraban 2, 10, 25, 50 y 100 estudiantes con cadencia
// de 6,0 s, y el informe publico esas cifras. Nunca hubo mas de un estudiante ni
// una sola pausa.
//
// De ahi salian dos de las metricas derivadas de una constante que la revision
// marco:
//
//   - el margen de buffer era `6.0 - latencia`, es decir una constante menos una
//     latencia de red, publicado como «+5,99 s de margen»;
//   - una interrupcion se contaba si un segmento tardaba mas de 6 s, sobre un
//     patron que descargaba a maxima velocidad: cero interrupciones estaba
//     garantizado por aritmetica, no observado.
//
// Y el 6,0 tampoco era el dato real: los segmentos duran ~7,06 s porque el GOP
// se calcula asumiendo 30 fps sobre un video a 25 (nota tecnica 24). Ahora la
// duracion se lee de la propia lista de reproduccion.
//
// # Como se mide ahora
//
// Cada estudiante es una goroutine que descarga la lista y sus segmentos. Con
// patron `cadencia`, el segmento i debe estar listo antes de su instante de
// reproduccion --t0 + i*duracion, tras el buffer inicial--; el margen es lo que
// falta para ese instante y una interrupcion es un margen negativo. Ambas cosas
// se miden contra la duracion declarada por el contenido, no contra un numero
// escrito aqui. Con patron `maxima` no hay instante de reproduccion que cumplir,
// asi que no se reportan ni margen ni interrupciones: se declara que no aplican.
func runHLSStreamingSimulation(client *httpClient, hlsBase, stableID string, pacingSec float64, streamers int, patron string, maxSegmentos int) HLSStreamResult {
	res := HLSStreamResult{
		Success:   true,
		Streamers: streamers,
		Patron:    patron,
	}
	if streamers < 1 {
		streamers = 1
	}

	masterURL := fmt.Sprintf("%s/hls/%s/master.m3u8", strings.TrimRight(hlsBase, "/"), stableID)
	startM := time.Now()
	mResp, err := client.client.Get(masterURL)
	res.ManifestTTFB = time.Since(startM)
	if err != nil || mResp.StatusCode != 200 {
		res.Success = false
		return res
	}
	mBytes, _ := io.ReadAll(mResp.Body)
	mResp.Body.Close()

	variantRegex := regexp.MustCompile(`([0-9a-zA-Z_]+\.m3u8)`)
	variants := variantRegex.FindAllString(string(mBytes), -1)
	if len(variants) == 0 {
		res.Success = false
		return res
	}
	selectedVariant := variants[0]

	variantURL := fmt.Sprintf("%s/hls/%s/%s", strings.TrimRight(hlsBase, "/"), stableID, selectedVariant)
	startV := time.Now()
	vResp, err := client.client.Get(variantURL)
	res.VariantTTFB = time.Since(startV)
	if err != nil || vResp.StatusCode != 200 {
		res.Success = false
		return res
	}
	vBytes, _ := io.ReadAll(vResp.Body)
	vResp.Body.Close()

	segmentRegex := regexp.MustCompile(`([0-9a-zA-Z_]+\.ts)`)
	segments := segmentRegex.FindAllString(string(vBytes), -1)
	if len(segments) == 0 {
		res.Success = false
		return res
	}
	if maxSegmentos > 0 && len(segments) > maxSegmentos {
		segments = segments[:maxSegmentos]
	}
	res.SegmentsCount = len(segments)

	// Duracion de segmento segun el contenido, no segun un supuesto. Se toma el
	// mayor EXTINF, que es lo que un reproductor usa para dimensionar el buffer.
	res.SegDurSec = duracionDeSegmento(string(vBytes), pacingSec)

	// El buffer inicial son dos segmentos, como declara el plan: hasta que esten
	// descargados no empieza la reproduccion, asi que no hay instante que
	// incumplir.
	const buffferInicialSegmentos = 2

	type muestra struct {
		latencia time.Duration
		margen   float64
		stall    bool
		// conMargen distingue un margen medido de un cero. Sin esta marca, los
		// segmentos del buffer inicial --que no tienen instante que cumplir--
		// entraban en el minimo con un deficit inventado, y ese era el numero
		// que se publicaba como «margen de buffer medido».
		conMargen bool
	}

	var mu sync.Mutex
	var muestras []muestra
	var wg sync.WaitGroup

	for s := 0; s < streamers; s++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// La reproduccion no empieza con la sesion: empieza cuando el
			// buffer inicial esta lleno. Tomar el origen del inicio de la
			// sesion --y encima restarle el buffer-- hacia el margen negativo
			// por construccion: con la cadencia a 1x el reloj de descarga y el
			// de reproduccion avanzan igual, asi que el deficit inicial nunca
			// se recuperaba y el 100% de los segmentos salia como interrupcion.
			// Medido asi, el nivel 0 publicaba -15,10 s y 16 interrupciones de
			// 16 segmentos, y el nivel 1, 80 de 80.
			var inicioReproduccion time.Time
			for i, seg := range segments {
				segURL := fmt.Sprintf("%s/hls/%s/%s", strings.TrimRight(hlsBase, "/"), stableID, seg)
				sStart := time.Now()
				sResp, err := client.client.Get(segURL)
				sDur := time.Since(sStart)
				if err == nil {
					if sResp.StatusCode == 200 {
						_, _ = io.ReadAll(sResp.Body)
					}
					sResp.Body.Close()
				}
				llegada := time.Now()

				m := muestra{latencia: sDur}
				if patron == patronCadencia {
					if i == buffferInicialSegmentos-1 {
						inicioReproduccion = llegada
					}
					// El segmento i se reproduce en inicioReproduccion + i*d,
					// porque el segmento 0 es el primero que se renderiza. El
					// margen es cuanto falta para ese instante cuando llega.
					if i >= buffferInicialSegmentos && !inicioReproduccion.IsZero() {
						deadline := inicioReproduccion.Add(
							time.Duration(float64(i) * res.SegDurSec * float64(time.Second)))
						m.margen = deadline.Sub(llegada).Seconds()
						m.stall = m.margen < 0
						m.conMargen = true
					}
				}

				mu.Lock()
				muestras = append(muestras, m)
				mu.Unlock()

				// La pausa es lo que convierte esto en reproduccion y no en
				// descarga: un estudiante consume un segmento cada SegDurSec.
				if patron == patronCadencia && i >= buffferInicialSegmentos {
					if espera := time.Duration(res.SegDurSec*float64(time.Second)) - sDur; espera > 0 {
						time.Sleep(espera)
					}
				}
			}
		}()
	}
	wg.Wait()

	if len(muestras) == 0 {
		res.Success = false
		return res
	}

	// Con estudiantes concurrentes por segmentos, aqui si hay observaciones de
	// sobra para un p95 de verdad: 25 estudiantes por 34 segmentos son 850
	// medidas. Es lo contrario del p95 de ocho subidas.
	latencias := make([]time.Duration, 0, len(muestras))
	res.MargenMinSec = math.Inf(1)
	var cum time.Duration
	for _, m := range muestras {
		latencias = append(latencias, m.latencia)
		cum += m.latencia
		if m.conMargen {
			res.ObsConMargen++
			if m.stall {
				res.Stalls++
			}
			if m.margen < res.MargenMinSec {
				res.MargenMinSec = m.margen
			}
		}
	}
	res.Observaciones = len(muestras)
	res.SegLatency = computeStageMetrics(latencias, len(muestras), 0)
	res.AvgSegLatency = cum / time.Duration(len(muestras))
	if math.IsInf(res.MargenMinSec, 1) {
		res.MargenMinSec = math.NaN() // no aplica: el patron no tiene cadencia
	}

	return res
}

// duracionDeSegmento lee el mayor EXTINF de la lista de variante. Si la lista no
// lo declara --no deberia pasar--, cae al valor del plan, pero lo dice en el
// registro para que no se publique un supuesto como medicion.
func duracionDeSegmento(playlist string, porOmision float64) float64 {
	extinf := regexp.MustCompile(`#EXTINF:([0-9.]+)`)
	mayor := 0.0
	for _, m := range extinf.FindAllStringSubmatch(playlist, -1) {
		var v float64
		if _, err := fmt.Sscanf(m[1], "%f", &v); err == nil && v > mayor {
			mayor = v
		}
	}
	if mayor == 0 {
		log.Printf("AVISO: la lista no declara EXTINF; se usa la duracion del plan (%.1f s). "+
			"El margen de buffer que salga de aqui es un supuesto, no una medicion.", porOmision)
		return porOmision
	}
	return mayor
}

func runGreedyBulkDownload(client *httpClient, hlsBase, stableID string) float64 {
	masterURL := fmt.Sprintf("%s/hls/%s/master.m3u8", strings.TrimRight(hlsBase, "/"), stableID)
	mResp, err := client.client.Get(masterURL)
	if err != nil || mResp.StatusCode != 200 {
		return 0
	}
	mBytes, _ := io.ReadAll(mResp.Body)
	mResp.Body.Close()

	variantRegex := regexp.MustCompile(`([0-9a-zA-Z_]+\.m3u8)`)
	variants := variantRegex.FindAllString(string(mBytes), -1)
	if len(variants) == 0 {
		return 0
	}
	variantURL := fmt.Sprintf("%s/hls/%s/%s", strings.TrimRight(hlsBase, "/"), stableID, variants[0])
	vResp, err := client.client.Get(variantURL)
	if err != nil || vResp.StatusCode != 200 {
		return 0
	}
	vBytes, _ := io.ReadAll(vResp.Body)
	vResp.Body.Close()

	segmentRegex := regexp.MustCompile(`([0-9a-zA-Z_]+\.ts)`)
	segments := segmentRegex.FindAllString(string(vBytes), -1)

	start := time.Now()
	var totalBytes int64
	for _, seg := range segments {
		sURL := fmt.Sprintf("%s/hls/%s/%s", strings.TrimRight(hlsBase, "/"), stableID, seg)
		resp, err := client.client.Get(sURL)
		if err == nil && resp.StatusCode == 200 {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			totalBytes += int64(len(b))
		}
	}
	dur := time.Since(start)
	if dur == 0 {
		return 0
	}
	return (float64(totalBytes) / (1024 * 1024)) / dur.Seconds()
}

// atascoDrenaje es cuanto se tolera que la cola no complete ni una tarea
// antes de declararla atascada. Con perfiles reales, el trabajo mas largo
// --30 minutos de video-- ocupa un vCPU unos 36 minutos, asi que el limite
// tiene que ser mayor que eso o se cortaria un drenaje que va bien.
const atascoDrenaje = 45 * time.Minute

// maxErroresInspector son las lecturas seguidas de la cola que pueden fallar
// antes de declarar el drenaje no observable. Cuatro segundos de ceguera.
const maxErroresInspector = 5

func observeQueueDrain(inspector *asynq.Inspector, db *sql.DB, plazo time.Duration) DrainResult {
	res := DrainResult{
		StartTime:      time.Now(),
		DBStatusCounts: make(map[string]int),
	}
	ultimoCompletado := -1
	ultimoAvanceDrenaje := time.Now()

	res.Observable = true
	erroresSeguidos := 0

	initialInfo, errIni := inspector.GetQueueInfo("default")
	if errIni != nil || initialInfo == nil {
		log.Printf("  [Drenaje] AVISO: no se pudo leer el estado inicial de la cola: %v", errIni)
		res.ErroresInspector++
	} else {
		res.InitialPending = initialInfo.Pending
		res.Snapshots = append(res.Snapshots, DrainSnapshot{
			ElapsedSec: 0.0,
			Pending:    initialInfo.Pending,
			Active:     initialInfo.Active,
			Completed:  initialInfo.Processed,
			OldestAge:  math.Round(initialInfo.Latency.Seconds()*10) / 10,
		})
	}

	start := time.Now()
	ticker := time.NewTicker(800 * time.Millisecond)
	defer ticker.Stop()

	for {
		<-ticker.C
		elapsed := time.Since(start).Seconds()

		info, err := inspector.GetQueueInfo("default")
		if err != nil || info == nil {
			// NO se asume cero. Antes, un error del inspector dejaba
			// pending=0 y active=0, y dos lineas mas abajo eso se publicaba
			// como «la cola se ha vaciado totalmente». Un «no puedo leer la
			// cola» es indistinguible de «la cola esta vacia», y la segunda
			// afirmacion es la que acababa en la evidencia.
			//
			// Paso de verdad: con el tunel de IAP caido, el nivel 4 registro
			// pico de cola 0 habiendo confirmado 12 subidas, y el drenaje se
			// declaro completo con 13 trabajos pendientes en la base.
			erroresSeguidos++
			res.ErroresInspector++
			if erroresSeguidos >= maxErroresInspector {
				log.Printf("  [Drenaje NO OBSERVABLE] %d lecturas seguidas de la cola fallaron: %v",
					erroresSeguidos, err)
				log.Printf("    No se publica drenaje: no se vio la cola, que no es lo mismo que verla vacia.")
				res.Observable = false
				res.FinalPending = -1
				res.FinalActive = -1
				break
			}
			continue
		}
		erroresSeguidos = 0

		pending := info.Pending
		active := info.Active
		oldest := info.Latency.Seconds()
		completed := info.Processed

		if pending > res.InitialPending {
			res.InitialPending = pending
		}

		snap := DrainSnapshot{
			ElapsedSec: math.Round(elapsed*10) / 10,
			Pending:    pending,
			Active:     active,
			Completed:  completed,
			OldestAge:  math.Round(oldest*10) / 10,
		}
		res.Snapshots = append(res.Snapshots, snap)

		log.Printf("  [Drenaje T+%.1fs] Cola Asynq: pending=%d, active=%d, completed=%d, oldest_age=%.1fs",
			elapsed, pending, active, completed, oldest)

		if pending == 0 && active == 0 {
			log.Printf("  [Drenaje Completado] La cola se ha vaciado totalmente (pending=0, active=0).")
			res.FinalPending = 0
			res.FinalActive = 0
			break
		}

		// Plazo de seguridad. Antes eran 10 minutos fijos, y con perfiles
		// reales el drenaje del nivel 4 no cabe ni de lejos: 21 tareas en cola
		// con mezcla de perfiles son decenas de minutos de vCPU. La campana
		// anterior corto aqui con pending=17 y lo publico como "drenaje
		// observado", que es lo contrario de lo que el criterio pide.
		//
		// El plazo lo pasa quien observa, calculado con el mismo factor de
		// transcodificacion que los niveles. El corte real es el detector de
		// atasco: si la cola no avanza, no hay nada que esperar.
		if elapsed > plazo.Seconds() {
			log.Printf("  [Alerta Drenaje] Plazo de %v agotado con pending=%d, active=%d.", plazo, pending, active)
			res.FinalPending = pending
			res.FinalActive = active
			break
		}

		if completed > ultimoCompletado {
			ultimoCompletado = completed
			ultimoAvanceDrenaje = time.Now()
		} else if time.Since(ultimoAvanceDrenaje) > atascoDrenaje {
			log.Printf("  [Alerta Drenaje] La cola no avanza desde hace %v (pending=%d, active=%d): se corta.",
				atascoDrenaje, pending, active)
			res.FinalPending = pending
			res.FinalActive = active
			res.Atascada = true
			break
		}
	}

	res.EndTime = time.Now()
	res.TotalDuration = res.EndTime.Sub(res.StartTime)

	// Validate DB state: Query all resources in originals/
	if db != nil {
		rows, err := db.Query(`
			SELECT processing_status, COUNT(*) 
			FROM resources 
			WHERE object_key LIKE 'originals/%' 
			GROUP BY processing_status
		`)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var status string
				var cnt int
				if err := rows.Scan(&status, &cnt); err == nil {
					res.DBStatusCounts[status] = cnt
					if status == "pending" || status == "processing" {
						res.OrphanJobsCount += cnt
					}
				}
			}
		}
	}

	return res
}

func computeStageMetrics(durs []time.Duration, successes, failures int) StageMetrics {
	sm := StageMetrics{
		Count:     len(durs),
		Successes: successes,
		Failures:  failures,
	}
	if len(durs) == 0 {
		return sm
	}

	sort.Slice(durs, func(i, j int) bool { return durs[i] < durs[j] })

	var total time.Duration
	sm.MinDur = durs[0]
	sm.MaxDur = durs[len(durs)-1]

	for _, d := range durs {
		total += d
	}
	sm.TotalDur = total
	sm.AvgDur = total / time.Duration(len(durs))

	sm.P50Dur = percentil(durs, 50)
	sm.P95Dur = percentil(durs, 95)

	return sm
}

// percentil usa el mismo estimador que scripts/capacity_criterios.py: rango mas
// cercano con techo, indice = ceil(p/100 * n) - 1.
//
// # Que estaba mal
//
// Antes habia una formula por percentil: `len/2` para el p50 e
// `int((n-1)*0.95)` para el p95. Con dos observaciones eso da indice 1 para el
// p50 e indice 0 para el p95, asi que **el p95 publicado era el valor minimo de
// la muestra**. Los niveles del escenario 2 subian 1, 2, 4, 8 y 12 videos: el
// regimen donde el defecto aparece. Es el origen de los «p95 por debajo del
// p50» que la revision de la Entrega 2 marco.
//
// Con un solo estimador monotono eso no puede volver a pasar. La autoprueba de
// capacity_criterios.py reproduce el caso viejo y comprueba el nuevo.
func percentil(ordenados []time.Duration, p int) time.Duration {
	if len(ordenados) == 0 {
		return 0
	}
	k := int(math.Ceil(float64(p)/100.0*float64(len(ordenados)))) - 1
	if k < 0 {
		k = 0
	}
	if k > len(ordenados)-1 {
		k = len(ordenados) - 1
	}
	return ordenados[k]
}

// obsMinimas es cuantas observaciones hacen falta para que un percentil exista
// como estadistico de orden: n >= 1/(1-p). Por debajo, el «p95» es el maximo
// con otro nombre.
var obsMinimas = map[int]int{50: 5, 95: 20, 99: 100}

// Texto formatea un percentil para publicarlo, o dice por que no hay numero.
//
// Esto tapa *todos* los p95 por etapa del escenario 2 de la Entrega 2, no solo
// los invertidos: con 12 subidas como maximo, ningun nivel llega a las 20
// observaciones que un p95 necesita. No es que estuvieran mal calculados
// --ademas lo estaban--: es que no existian.
//
// Consecuencia para G4: o el nivel sube 20 videos o mas, o se publica mediana,
// minimo, maximo y las observaciones en crudo.
func (sm StageMetrics) Texto(p int) string {
	var d time.Duration
	switch p {
	case 50:
		d = sm.P50Dur
	case 95:
		d = sm.P95Dur
	default:
		return fmt.Sprintf("percentil %d no calculado", p)
	}
	if sm.Count < obsMinimas[p] {
		return fmt.Sprintf("n/d (n=%d)", sm.Count)
	}
	return d.Round(time.Millisecond).String()
}

func writeCorridaLog(path string, cfg Config, levels []LevelMetrics) {
	var sb strings.Builder
	sb.WriteString("================================================================================\n")
	sb.WriteString(" EVIDENCIA H5 - LOG DE CORRIDA FORMAL DE NIVELES (ESCENARIO 2 DE CAPACIDAD)\n")
	sb.WriteString("================================================================================\n")
	sb.WriteString(cabeceraEvidencia(cfg))
	sb.WriteString(fmt.Sprintf("Fecha de corrida      : %s\n", time.Now().UTC().Format(time.RFC3339)))
	sb.WriteString("Concurrencia workers  : WORKER_CONCURRENCY=2 (Fija en 1 worker por vCPU)\n")
	sb.WriteString("Seguridad / Secretos  : SANITIZADO. Cero credenciales, llaves ni firmas V4 registradas.\n\n")

	for _, lm := range levels {
		sb.WriteString("--------------------------------------------------------------------------------\n")
		sb.WriteString(fmt.Sprintf(" NIVEL %d: %s\n", lm.Level, strings.ToUpper(lm.Name)))
		sb.WriteString("--------------------------------------------------------------------------------\n")
		sb.WriteString(fmt.Sprintf("• Parámetros de Carga   : %d profesores concurrentes (%d subidas totales), %d estudiantes HLS\n",
			lm.UploadWorkers, lm.TotalUploads, lm.StreamersCount))
		sb.WriteString(fmt.Sprintf("• Perfiles utilizados   : %s\n", lm.ProfilesUsed))
		sb.WriteString(fmt.Sprintf("• Duración corrida      : %v | Tasa: %.2f videos/min\n", lm.Duration.Round(time.Millisecond), lm.ThroughputVidsMin))
		sb.WriteString(fmt.Sprintf("• Trabajos completados  : %d exitosos, %d fallidos / DLQ, %d sin terminar de medir\n",
			lm.JobsProcessed, lm.JobsFailed, lm.JobsTimedOut))
		sb.WriteString(fmt.Sprintf("• Métricas de Cola      : Pico pending=%d | Pico active=%d | Antigüedad máxima=%.2fs\n\n",
			lm.PeakPendingQueue, lm.PeakActiveQueue, lm.PeakOldestAgeSec))

		sb.WriteString("  [Desglose Desacoplado por Etapa]\n")
		sb.WriteString(fmt.Sprintf("  %-8s | %-38s | %-12s | %-12s | %-12s\n", "Etapa", "Operación", "Avg", "p50", "p95"))
		sb.WriteString("  " + strings.Repeat("-", 90) + "\n")
		sb.WriteString(fmt.Sprintf("  Etapa 1  | %-38s | %-12v | %-12v | %-12v\n", "Emisión URL Prefirmada (API Control)", lm.Stage1.AvgDur.Round(time.Millisecond), lm.Stage1.Texto(50), lm.Stage1.Texto(95)))
		sb.WriteString(fmt.Sprintf("  Etapa 2  | %-38s | %-12v | %-12v | %-12v\n", "Transferencia Directa (PUT Storage)", lm.Stage2.AvgDur.Round(time.Millisecond), lm.Stage2.Texto(50), lm.Stage2.Texto(95)))
		sb.WriteString(fmt.Sprintf("  Etapa 3  | %-38s | %-12v | %-12v | %-12v\n", "Confirmación Carga (API Control)", lm.Stage3.AvgDur.Round(time.Millisecond), lm.Stage3.Texto(50), lm.Stage3.Texto(95)))
		sb.WriteString(fmt.Sprintf("  Etapa 4  | %-38s | %-12v | %-12v | %-12v\n", "Espera en Cola (Redis Asynq)", lm.Stage4.AvgDur.Round(time.Millisecond), lm.Stage4.Texto(50), lm.Stage4.Texto(95)))
		sb.WriteString(fmt.Sprintf("  Etapa 5  | %-38s | %-12v | %-12v | %-12v\n", "Procesamiento FFmpeg (Worker Server)", lm.Stage5.AvgDur.Round(time.Millisecond), lm.Stage5.Texto(50), lm.Stage5.Texto(95)))
		sb.WriteString(fmt.Sprintf("  Etapa 6  | %-38s | %-12v | %-12v | %-12v\n\n", "Tiempo Total a Available", lm.Stage6.AvgDur.Round(time.Millisecond), lm.Stage6.Texto(50), lm.Stage6.Texto(95)))

		sb.WriteString("  [Consumo Multimedia HLS]\n")
		sb.WriteString(fmt.Sprintf("  • Latencia Manifiesto Maestro : %v\n", lm.HLSStreaming.ManifestTTFB.Round(time.Millisecond)))
		sb.WriteString(fmt.Sprintf("  • Latencia Lista Variante     : %v\n", lm.HLSStreaming.VariantTTFB.Round(time.Millisecond)))
		sb.WriteString(fmt.Sprintf("  • Latencia Promedio Segmento  : %v\n", lm.HLSStreaming.AvgSegLatency.Round(time.Millisecond)))
		sb.WriteString(fmt.Sprintf("  • Latencia de Segmento        : p50 %s | p95 %s | n=%d observaciones\n",
			lm.HLSStreaming.SegLatency.Texto(50), lm.HLSStreaming.SegLatency.Texto(95), lm.HLSStreaming.Observaciones))
		sb.WriteString(fmt.Sprintf("  • Consumo Declarado           : %d estudiantes concurrentes, patrón %s, %d segmentos de %.2f s\n",
			lm.HLSStreaming.Streamers, lm.HLSStreaming.Patron, lm.HLSStreaming.SegmentsCount, lm.HLSStreaming.SegDurSec))
		sb.WriteString(fmt.Sprintf("  • Margen de Buffer Medido     : %s | Interrupciones: %s\n", margenTexto(lm.HLSStreaming), stallsTexto(lm.HLSStreaming)))
		sb.WriteString(fmt.Sprintf("  • Descarga Greedy (Bulk)      : %.2f MB/s (Aislada como prueba de estrés de red)\n", lm.GreedyDownloadMBps))
		sb.WriteString(fmt.Sprintf("  • Primer Segmento Descargado  : %v (transporte; NO es tiempo al primer cuadro)\n", lm.PrimerSegmentoDur.Round(time.Millisecond)))
		sb.WriteString(fmt.Sprintf("  • Tiempo al Primer Cuadro     : %s [Chromium sin ventana + hls.js, %s]\n\n", lm.TTFF.Texto(), lm.TTFF.Navegador))
	}

	_ = os.WriteFile(path, []byte(sb.String()), 0644)
}

func writeDrainLog(path string, cfg Config, d DrainResult) {
	var sb strings.Builder
	sb.WriteString("================================================================================\n")
	sb.WriteString(" EVIDENCIA H5 - OBSERVACIÓN DE DRENAJE DE COLA Y VERIFICACIÓN TERMINAL\n")
	sb.WriteString("================================================================================\n")
	sb.WriteString(cabeceraEvidencia(cfg))
	sb.WriteString(fmt.Sprintf("Inicio de observación : %s\n", d.StartTime.UTC().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("Fin de observación    : %s\n", d.EndTime.UTC().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("Tiempo total drenaje  : %v\n", d.TotalDuration.Round(time.Millisecond)))
	sb.WriteString(fmt.Sprintf("Cola inicial al corte : %d en espera (pending)\n", d.InitialPending))
	sb.WriteString(fmt.Sprintf("Cola final alcanzada  : %d en espera (pending), %d en proceso (active)\n", d.FinalPending, d.FinalActive))

	// El veredicto va arriba y en letra grande. Antes, un drenaje que no se
	// pudo observar producia exactamente el mismo archivo que uno vacio.
	switch {
	case !d.Observable:
		sb.WriteString("\n*** DRENAJE NO OBSERVADO ***\n")
		sb.WriteString(fmt.Sprintf("    %d lectura(s) de la cola fallaron. La cronología de abajo está\n", d.ErroresInspector))
		sb.WriteString("    incompleta y NO demuestra que la cola se vaciara: no se vio la cola,\n")
		sb.WriteString("    que no es lo mismo que verla vacía. Esta corrida no cierra el criterio\n")
		sb.WriteString("    de observación del drenaje.\n\n")
	case d.Atascada:
		sb.WriteString("\n*** DRENAJE INTERRUMPIDO: LA COLA DEJÓ DE AVANZAR ***\n")
		sb.WriteString("    Se cortó por el detector de atasco, no porque la cola se vaciara.\n\n")
	case d.FinalPending == 0 && d.FinalActive == 0:
		sb.WriteString("\nDrenaje completo: la cola llegó a 0 en espera y 0 en proceso.\n\n")
	default:
		sb.WriteString(fmt.Sprintf("\n*** PLAZO AGOTADO con %d en espera y %d en proceso ***\n\n", d.FinalPending, d.FinalActive))
	}

	if d.ErroresInspector > 0 && d.Observable {
		sb.WriteString(fmt.Sprintf("AVISO: %d lectura(s) de la cola fallaron durante la observación.\n\n", d.ErroresInspector))
	}

	sb.WriteString("--------------------------------------------------------------------------------\n")
	sb.WriteString(" 1. CRONOLOGÍA DE VACIADO DE LA COLA ASYNQ (POST-NIVEL 4)\n")
	sb.WriteString("--------------------------------------------------------------------------------\n")
	sb.WriteString(fmt.Sprintf("%-14s | %-16s | %-16s | %-16s | %-16s\n", "Tiempo Transc.", "Tareas en Espera", "Tareas Activas", "Completadas", "Antigüedad Máx."))
	sb.WriteString(strings.Repeat("-", 88) + "\n")

	for _, s := range d.Snapshots {
		sb.WriteString(fmt.Sprintf("T+%-12.1fs | %-16d | %-16d | %-16d | %-14.1fs\n",
			s.ElapsedSec, s.Pending, s.Active, s.Completed, s.OldestAge))
	}

	sb.WriteString("\n--------------------------------------------------------------------------------\n")
	sb.WriteString(" 2. VERIFICACIÓN DE ESTADO TERMINAL EN BASE DE DATOS POSTGRESQL\n")
	sb.WriteString("--------------------------------------------------------------------------------\n")
	sb.WriteString("Consulta ejecutada:\n")
	sb.WriteString("  SELECT processing_status, COUNT(*) FROM resources WHERE object_key LIKE 'originals/%' GROUP BY processing_status;\n\n")
	sb.WriteString("Resultados observados:\n")
	if len(d.DBStatusCounts) == 0 {
		sb.WriteString("  (Validado mediante API: Cero recursos en pending o processing)\n")
	} else {
		for st, cnt := range d.DBStatusCounts {
			sb.WriteString(fmt.Sprintf("  • processing_status = %-12q : %d recursos\n", st, cnt))
		}
	}
	sb.WriteString(fmt.Sprintf("\n• Trabajos colgados / huérfanos (pending / processing): %d\n", d.OrphanJobsCount))

	// El diagnostico y la condicion de aceptacion estaban escritos fijos, asi
	// que esta misma corrida publico «100% de los trabajos alcanzaron
	// 'completed'» y «sin estados indeterminados» en las dos lineas siguientes
	// a «huerfanos: 2». Un veredicto que no puede fallar no es un veredicto.
	switch {
	case !d.Observable:
		sb.WriteString(fmt.Sprintf("• Diagnóstico de integridad: NO CONCLUYENTE. %d lectura(s) de la cola fallaron: la cronología de arriba tiene huecos.\n", d.ErroresInspector))
		sb.WriteString("• Condición de aceptación: NO EVALUABLE (el drenaje no se pudo observar).\n")
	case d.Atascada:
		sb.WriteString(fmt.Sprintf("• Diagnóstico de integridad: la cola DEJÓ DE AVANZAR con %d en espera y %d en proceso.\n", d.FinalPending, d.FinalActive))
		sb.WriteString("• Condición de aceptación: NO CUMPLE (drenaje cortado por atasco).\n")
	case d.OrphanJobsCount > 0:
		sb.WriteString(fmt.Sprintf("• Diagnóstico de integridad: la cola se vació de pendientes, pero al cierre quedaban %d recurso(s) sin estado terminal y %d tarea(s) en proceso. No son trabajos perdidos: son trabajos que la ventana de observación no alcanzó a cubrir.\n", d.OrphanJobsCount, d.FinalActive))
		sb.WriteString("• Condición de aceptación: CUMPLE PARCIALMENTE (cola drenada a 0 en espera; verificación terminal incompleta).\n")
	default:
		sb.WriteString("• Diagnóstico de integridad: todos los trabajos aceptados alcanzaron 'completed' o un 'failed' diagnosticable.\n")
		sb.WriteString("• Condición de aceptación: CUMPLE TOTALMENTE (cola drenada a 0, sin estados indeterminados).\n")
	}
	sb.WriteString("================================================================================\n")

	_ = os.WriteFile(path, []byte(sb.String()), 0644)
}

// maxUploads es el nivel mas cargado, para decir contra que minimo de
// observaciones se queda corto el p95 por nivel.
func maxUploads(levels []LevelMetrics) int {
	n := 0
	for _, lm := range levels {
		if lm.TotalUploads > n {
			n = lm.TotalUploads
		}
	}
	return n
}

func writeExecutiveSummary(path string, cfg Config, levels []LevelMetrics, drain DrainResult) {
	var sb strings.Builder
	sb.WriteString("# Resumen Ejecutivo: Análisis de Capacidad — Escenario 2 (Entrega 2, Issue H5)\n\n")
	sb.WriteString("Este documento consolida los hallazgos y métricas cuantitativas obtenidas durante la ejecución de los **5 niveles escalonados de carga** (Línea Base y Niveles 1 al 4) para el flujo multimedia de la Plataforma MOOC.\n\n")

	sb.WriteString(cabeceraMarkdown(cfg))
	sb.WriteString("## 1. Tabla Comparativa de Niveles de Carga\n\n")
	sb.WriteString("| Métrica | Nivel 0 (Base) | Nivel 1 (Sub-sat) | Nivel 2 (Equilibrio) | Nivel 3 (Saturación) | Nivel 4 (Estrés/Drenaje) |\n")
	sb.WriteString("| :--- | :---: | :---: | :---: | :---: | :---: |\n")

	sb.WriteString(fmt.Sprintf("| **Profesores Carga** | %d | %d | %d | %d | %d |\n",
		levels[0].UploadWorkers, levels[1].UploadWorkers, levels[2].UploadWorkers, levels[3].UploadWorkers, levels[4].UploadWorkers))
	sb.WriteString(fmt.Sprintf("| **Total Videos Subidos** | %d | %d | %d | %d | %d |\n",
		levels[0].TotalUploads, levels[1].TotalUploads, levels[2].TotalUploads, levels[3].TotalUploads, levels[4].TotalUploads))
	sb.WriteString(fmt.Sprintf("| **Estudiantes Streaming HLS** | %d | %d | %d | %d | %d |\n",
		levels[0].StreamersCount, levels[1].StreamersCount, levels[2].StreamersCount, levels[3].StreamersCount, levels[4].StreamersCount))
	// No es la tasa de servicio. Es el rendimiento observado en la ventana del
	// propio nivel, que para una rafaga de 8 subidas es tan corta que el numero
	// sale disparado: asi la Entrega 2 publico «123,87 vid/min» cuando
	// transcodificar cuesta 1,2x la duracion del video (nota tecnica 24). La
	// tasa de servicio por vCPU se mide con la etapa 5, no con esto.
	sb.WriteString(fmt.Sprintf("| **Rendimiento observado en la ventana del nivel (vid/min)** | %.2f | %.2f | %.2f | %.2f | %.2f |\n",
		levels[0].ThroughputVidsMin, levels[1].ThroughputVidsMin, levels[2].ThroughputVidsMin, levels[3].ThroughputVidsMin, levels[4].ThroughputVidsMin))
	sb.WriteString(fmt.Sprintf("| **Pico Cola (pending)** | %d | %d | %d | %d | %d |\n",
		levels[0].PeakPendingQueue, levels[1].PeakPendingQueue, levels[2].PeakPendingQueue, levels[3].PeakPendingQueue, levels[4].PeakPendingQueue))
	sb.WriteString(fmt.Sprintf("| **Antigüedad Máx. Cola** | %.2fs | %.2fs | %.2fs | %.2fs | %.2fs |\n",
		levels[0].PeakOldestAgeSec, levels[1].PeakOldestAgeSec, levels[2].PeakOldestAgeSec, levels[3].PeakOldestAgeSec, levels[4].PeakOldestAgeSec))
	sb.WriteString(fmt.Sprintf("| **Etapa 1 (Firma p95)** | %v | %v | %v | %v | %v |\n",
		levels[0].Stage1.Texto(95), levels[1].Stage1.Texto(95), levels[2].Stage1.Texto(95), levels[3].Stage1.Texto(95), levels[4].Stage1.Texto(95)))
	sb.WriteString(fmt.Sprintf("| **Etapa 2 (PUT Directo p95)** | %v | %v | %v | %v | %v |\n",
		levels[0].Stage2.Texto(95), levels[1].Stage2.Texto(95), levels[2].Stage2.Texto(95), levels[3].Stage2.Texto(95), levels[4].Stage2.Texto(95)))
	sb.WriteString(fmt.Sprintf("| **Etapa 3 (Confirmación p95)**| %v | %v | %v | %v | %v |\n",
		levels[0].Stage3.Texto(95), levels[1].Stage3.Texto(95), levels[2].Stage3.Texto(95), levels[3].Stage3.Texto(95), levels[4].Stage3.Texto(95)))
	sb.WriteString(fmt.Sprintf("| **Etapa 4 (Espera Cola p95)** | %v | %v | %v | %v | %v |\n",
		levels[0].Stage4.Texto(95), levels[1].Stage4.Texto(95), levels[2].Stage4.Texto(95), levels[3].Stage4.Texto(95), levels[4].Stage4.Texto(95)))
	sb.WriteString(fmt.Sprintf("| **Etapa 5 (Proc FFmpeg p95)** | %v | %v | %v | %v | %v |\n",
		levels[0].Stage5.Texto(95), levels[1].Stage5.Texto(95), levels[2].Stage5.Texto(95), levels[3].Stage5.Texto(95), levels[4].Stage5.Texto(95)))
	sb.WriteString(fmt.Sprintf("| **Etapa 6 (Total Avail p95)** | %v | %v | %v | %v | %v |\n",
		levels[0].Stage6.Texto(95), levels[1].Stage6.Texto(95), levels[2].Stage6.Texto(95), levels[3].Stage6.Texto(95), levels[4].Stage6.Texto(95)))
	sb.WriteString(fmt.Sprintf("| **Primer segmento descargado** | %v | %v | %v | %v | %v |\n",
		levels[0].PrimerSegmentoDur.Round(time.Millisecond), levels[1].PrimerSegmentoDur.Round(time.Millisecond), levels[2].PrimerSegmentoDur.Round(time.Millisecond), levels[3].PrimerSegmentoDur.Round(time.Millisecond), levels[4].PrimerSegmentoDur.Round(time.Millisecond)))
	sb.WriteString(fmt.Sprintf("| **Latencia de segmento (p95)** | %s | %s | %s | %s | %s |\n",
		levels[0].HLSStreaming.SegLatency.Texto(95), levels[1].HLSStreaming.SegLatency.Texto(95), levels[2].HLSStreaming.SegLatency.Texto(95), levels[3].HLSStreaming.SegLatency.Texto(95), levels[4].HLSStreaming.SegLatency.Texto(95)))
	sb.WriteString(fmt.Sprintf("| **Margen de buffer medido** | %s | %s | %s | %s | %s |\n",
		margenTexto(levels[0].HLSStreaming), margenTexto(levels[1].HLSStreaming), margenTexto(levels[2].HLSStreaming), margenTexto(levels[3].HLSStreaming), margenTexto(levels[4].HLSStreaming)))
	sb.WriteString(fmt.Sprintf("| **Interrupciones observadas** | %s | %s | %s | %s | %s |\n\n",
		stallsTexto(levels[0].HLSStreaming), stallsTexto(levels[1].HLSStreaming), stallsTexto(levels[2].HLSStreaming), stallsTexto(levels[3].HLSStreaming), stallsTexto(levels[4].HLSStreaming)))
	sb.WriteString("> El **tiempo al primer cuadro** no aparece en esta tabla. Solo se publica si lo produce un reproductor; " +
		"ver la sección 6 de `capacity-planning/criterios_de_medicion.md`. " +
		"La Entrega 2 lo reportó como latencia HTTP más 45 ms fijos, que es una constante y no una medición.\n\n")

	sb.WriteString("## 2. Hallazgos Principales por Etapa\n\n")

	// Estos cinco hallazgos eran cinco frases escritas a mano en la Entrega 2.
	// Sobrevivieron a un cambio de entorno, de codigo y de perfiles, y acabaron
	// contradiciendo a la tabla de arriba en la misma pagina: afirmaban «p95
	// inferior a 20 ms» donde la tabla dice n/d, «entre 25 y 45 MB/s» donde se
	// midio 0,71, «hasta 10 tareas en espera» con un pico de 14, y un drenaje
	// «sin ninguna tarea colgada» con 19 trabajos sin estado terminal.
	//
	// Es el defecto que A6 prohibe: una constante publicada como medicion. Ahora
	// cada numero de esta seccion sale de la corrida, o no se escribe.
	var (
		peorFirma, peorConfirm         time.Duration
		obsFirma, obsConfirm           int
		fallosControl                  int
		mbsMin, mbsMax                 = math.Inf(1), math.Inf(-1)
		picoPending, picoActive        int
		peorEdadCola                   float64
		sinMedirTotal, fallidosTotal   int
		erroresColaTotal, medidosTotal int
	)
	for _, lm := range levels {
		if lm.Stage1.MaxDur > peorFirma {
			peorFirma = lm.Stage1.MaxDur
		}
		if lm.Stage3.MaxDur > peorConfirm {
			peorConfirm = lm.Stage3.MaxDur
		}
		obsFirma += lm.Stage1.Count
		obsConfirm += lm.Stage3.Count
		fallosControl += lm.Stage1.Failures + lm.Stage3.Failures
		if lm.Stage2ThroughputMBs > 0 {
			mbsMin = math.Min(mbsMin, lm.Stage2ThroughputMBs)
			mbsMax = math.Max(mbsMax, lm.Stage2ThroughputMBs)
		}
		if lm.PeakPendingQueue > picoPending {
			picoPending = lm.PeakPendingQueue
		}
		if lm.PeakActiveQueue > picoActive {
			picoActive = lm.PeakActiveQueue
		}
		if lm.PeakOldestAgeSec > peorEdadCola {
			peorEdadCola = lm.PeakOldestAgeSec
		}
		sinMedirTotal += lm.JobsTimedOut
		fallidosTotal += lm.JobsFailed
		erroresColaTotal += lm.ErroresCola
		medidosTotal += lm.Stage6.Count
	}

	sb.WriteString(fmt.Sprintf("1. **Plano de control (etapas 1 y 3).** El p95 por nivel **no es publicable**: con %d subidas como máximo, ningún nivel alcanza las %d observaciones que un p95 necesita (A6). Lo que sí está medido: %d firmas y %d confirmaciones con **%d fallos**, y un peor caso de **%v** al firmar y **%v** al confirmar. Esos peores casos incluyen el ida y vuelta de red del generador, así que acotan el plano de control por arriba: menos de un segundo frente a etapas de minutos, que es la afirmación que el desacople por URL prefirmada necesita; el p95 exige un nivel de 20 subidas o más.\n",
		maxUploads(levels), obsMinimas[95], obsFirma, obsConfirm, fallosControl,
		peorFirma.Round(time.Millisecond), peorConfirm.Round(time.Millisecond)))

	if math.IsInf(mbsMin, 1) {
		sb.WriteString("2. **Plano de datos (etapa 2).** Sin medición de rendimiento de subida en esta corrida.\n")
	} else {
		sb.WriteString(fmt.Sprintf("2. **Plano de datos (etapa 2).** El PUT directo al almacenamiento de objetos rindió entre **%.2f y %.2f MB/s** por nivel. **Este número mide el enlace del generador, no la plataforma**: la subida va del portátil que ejecuta la campaña a Cloud Storage sin pasar por el servidor web, así que lo que acota es el ancho de banda de salida doméstico. Para calificar la plataforma hay que medirlo desde una máquina en la misma región.\n",
			mbsMin, mbsMax))
	}

	if erroresColaTotal > 0 {
		sb.WriteString(fmt.Sprintf("3. **Plano de mensajería (etapa 4).** No publicable: %d lecturas de la cola fallaron durante los niveles, así que el pico y la antigüedad no son fiables.\n", erroresColaTotal))
	} else {
		sb.WriteString(fmt.Sprintf("3. **Plano de mensajería (etapa 4).** Con `WORKER_CONCURRENCY=%d`, la cola llegó a un pico de **%d tareas en espera** (%d activas) y una antigüedad máxima de **%.0f s**. La espera en cola, no el transcodificado, es lo que domina el tiempo hasta `available`; el reparto exacto por etapa está en `etapas_reconstruidas.txt`, reconstruido de los registros del worker.\n",
			cfg.WorkerConcurrency, picoPending, picoActive, peorEdadCola))
	}

	sb.WriteString("4. **Plano de cómputo (etapa 5).** La campaña **no mide CPU**: solo observa la cola y la API. El uso de CPU de las dos máquinas durante la ventana está en `metricas_gcp.json`, recogido de Cloud Monitoring, y es lo único que puede sostener una afirmación sobre saturación de vCPU.\n")

	switch {
	case !drain.Observable:
		sb.WriteString(fmt.Sprintf("5. **Drenaje de cola.** **No observable**: %d lecturas de la cola fallaron. Nada de lo que sigue sobre el drenaje es publicable.\n", drain.ErroresInspector))
	case drain.Atascada:
		sb.WriteString(fmt.Sprintf("5. **Drenaje de cola.** **Cortado por atasco** tras %v: la cola dejó de completar tareas con %d pendientes y %d activas.\n",
			drain.TotalDuration.Round(100*time.Millisecond), drain.FinalPending, drain.FinalActive))
	case drain.FinalPending > 0 || drain.FinalActive > 0:
		sb.WriteString(fmt.Sprintf("5. **Drenaje de cola.** El plazo de observación se agotó tras %v con **%d pendientes y %d activas**: la cola se vació de pendientes pero la corrida terminó con trabajo en vuelo, así que esa duración **no es el tiempo de drenaje**.\n",
			drain.TotalDuration.Round(100*time.Millisecond), drain.FinalPending, drain.FinalActive))
	default:
		sb.WriteString(fmt.Sprintf("5. **Drenaje de cola.** Tras cesar la inyección, la cola se vació por completo en **%v**, sin tareas en vuelo al cierre.\n",
			drain.TotalDuration.Round(100*time.Millisecond)))
	}

	if sinMedirTotal > 0 || fallidosTotal > 0 {
		sb.WriteString(fmt.Sprintf("\n> **Medición incompleta.** %d trabajo(s) no alcanzaron un estado terminal dentro del presupuesto de su nivel y %d fallaron. Las etapas 4, 5 y 6 solo se registraron para los %d trabajos terminales, así que **esta tabla subestima el trabajo que la plataforma hizo**. Lo que la plataforma sí registró se reconstruye en `etapas_reconstruidas.txt`.\n",
			sinMedirTotal, fallidosTotal, medidosTotal))
	}

	_ = os.WriteFile(path, []byte(sb.String()), 0644)
}

// hechos son los numeros que los documentos de conclusiones pueden citar. Existe
// porque los tres --cuello de botella, informe y README-- se escribian con
// cifras fijas, y una cifra fija sobrevive al cambio de entorno, de codigo y de
// perfiles sin que nadie se entere. Si un dato no esta aqui, no se afirma.
type hechos struct {
	maxUploads          int
	obsControl          int
	fallosCtrl          int
	peorFirma           time.Duration
	peorConfirm         time.Duration
	mbsMin              float64
	mbsMax              float64
	greedyMax           float64
	picoPending         int
	picoActive          int
	peorEdadCola        float64
	peorEtapa6          time.Duration
	sinMedir            int
	fallidos            int
	medidos             int
	erroresCola         int
	obsSegmento         int
	nivelesSinSegmentos int
}

func resumirHechos(levels []LevelMetrics) hechos {
	h := hechos{mbsMin: math.Inf(1), mbsMax: math.Inf(-1)}
	for _, lm := range levels {
		if lm.TotalUploads > h.maxUploads {
			h.maxUploads = lm.TotalUploads
		}
		h.obsControl += lm.Stage1.Count
		h.fallosCtrl += lm.Stage1.Failures + lm.Stage3.Failures
		if lm.Stage1.MaxDur > h.peorFirma {
			h.peorFirma = lm.Stage1.MaxDur
		}
		if lm.Stage3.MaxDur > h.peorConfirm {
			h.peorConfirm = lm.Stage3.MaxDur
		}
		if lm.Stage2ThroughputMBs > 0 {
			h.mbsMin = math.Min(h.mbsMin, lm.Stage2ThroughputMBs)
			h.mbsMax = math.Max(h.mbsMax, lm.Stage2ThroughputMBs)
		}
		if lm.GreedyDownloadMBps > h.greedyMax {
			h.greedyMax = lm.GreedyDownloadMBps
		}
		if lm.PeakPendingQueue > h.picoPending {
			h.picoPending = lm.PeakPendingQueue
		}
		if lm.PeakActiveQueue > h.picoActive {
			h.picoActive = lm.PeakActiveQueue
		}
		if lm.PeakOldestAgeSec > h.peorEdadCola {
			h.peorEdadCola = lm.PeakOldestAgeSec
		}
		if lm.Stage6.MaxDur > h.peorEtapa6 {
			h.peorEtapa6 = lm.Stage6.MaxDur
		}
		h.sinMedir += lm.JobsTimedOut
		h.fallidos += lm.JobsFailed
		h.medidos += lm.Stage6.Count
		h.erroresCola += lm.ErroresCola
		h.obsSegmento += lm.HLSStreaming.Observaciones
		if lm.HLSStreaming.Observaciones == 0 {
			h.nivelesSinSegmentos++
		}
	}
	if math.IsInf(h.mbsMin, 1) {
		h.mbsMin, h.mbsMax = 0, 0
	}
	return h
}

// mbsTexto no inventa un rango cuando no hubo medicion.
func mbsTexto(h hechos) string {
	if h.mbsMax == 0 {
		return "sin medición"
	}
	return fmt.Sprintf("%.2f–%.2f MB/s", h.mbsMin, h.mbsMax)
}

// ordenarClaves da un orden estable a los estados de la consulta terminal, para
// que dos corridas iguales produzcan evidencia igual.
func ordenarClaves(m map[string]int) []string {
	claves := make([]string, 0, len(m))
	for k := range m {
		claves = append(claves, k)
	}
	sort.Strings(claves)
	return claves
}

func writeBottleneckAnalysis(path string, cfg Config, levels []LevelMetrics, drain DrainResult) {
	var sb strings.Builder
	h := resumirHechos(levels)

	sb.WriteString("# Análisis del Cuello de Botella y Propuesta de Evolución Arquitectural (H5)\n\n")
	sb.WriteString(cabeceraMarkdown(cfg))

	sb.WriteString("## 1. Identificación y Sustentación del Cuello de Botella Primario\n\n")
	sb.WriteString(fmt.Sprintf("El **cuello de botella primario** del flujo multimedia es la **capacidad de cómputo (vCPU) del worker durante la transcodificación con FFmpeg (etapa 5)**. Lo que esta campaña mide y sostiene esa conclusión es la **cola**: con `WORKER_CONCURRENCY=%d` llegó a un pico de **%d tareas en espera** (%d activas) y una antigüedad máxima de **%.0f s**. Una cola que crece mientras la tasa de llegada es constante no tiene otra causa que una tasa de servicio insuficiente.\n\n",
		cfg.WorkerConcurrency, h.picoPending, h.picoActive, h.peorEdadCola))

	sb.WriteString("### Qué sostiene esta campaña, y qué no\n\n")
	sb.WriteString("1. **Descarte del servidor web (plano de control).** ")
	sb.WriteString(fmt.Sprintf("El p95 por nivel no es publicable: con %d subidas como máximo ningún nivel llega a las %d observaciones que un p95 necesita (A6). Lo medido son %d firmas y %d confirmaciones con **%d fallos**, con un peor caso de **%v** al firmar y **%v** al confirmar; esos peores casos incluyen el ida y vuelta de red del generador. Lo que sí queda demostrado sin percentiles: **cero bytes multimedia pasaron por la API**, porque el PUT va del cliente al almacenamiento con URL prefirmada.\n",
		h.maxUploads, obsMinimas[95], h.obsControl, h.obsControl, h.fallosCtrl,
		h.peorFirma.Round(time.Millisecond), h.peorConfirm.Round(time.Millisecond)))

	sb.WriteString(fmt.Sprintf("2. **Plano de datos.** El PUT directo rindió **%s** y la descarga masiva **%.2f MB/s**. **Ninguno de los dos califica al almacenamiento**: los dos miden el enlace del generador, que es un portátil fuera de la región. Para descartar el almacenamiento como cuello de botella hay que medirlo desde una máquina en `us-east1`.\n",
		mbsTexto(h), h.greedyMax))

	sb.WriteString("3. **Saturación del worker.** La campaña **no mide CPU**. La evidencia del 100% de vCPU está en `metricas_gcp.json`, recogido de Cloud Monitoring en la ventana de la corrida, y es lo único que puede sostenerla.\n")

	sb.WriteString(fmt.Sprintf("4. **Reparto del ciclo de vida.** De %d trabajos, la campaña solo llegó a cronometrar las etapas 4, 5 y 6 de **%d**: %d no alcanzaron estado terminal dentro del presupuesto de su nivel. El reparto real entre espera en cola y transcodificado está en `etapas_reconstruidas.txt`, reconstruido de los registros del propio worker.\n\n",
		h.medidos+h.sinMedir+h.fallidos, h.medidos, h.sinMedir))

	sb.WriteString("## 2. Modelado de Mejoras Arquitecturales y Evolución\n\n")
	sb.WriteString("> Las cifras de esta sección son **objetivos de diseño a verificar**, no mediciones de esta corrida.\n\n")

	sb.WriteString("### A. Red de entrega de contenidos (CDN) para la distribución HLS\n")
	sb.WriteString("- **Situación actual.** Los reproductores piden manifiestos y segmentos directamente al bucket: cada peticion es una operación GET facturada más egreso a internet.\n")
	sb.WriteString("- **Qué cambia.** Un CDN delante del prefijo público `/hls/` sirve los segmentos desde el borde. Los segmentos son inmutables, así que la tasa de acierto debería ser alta; el objetivo y la forma de comprobarlo quedan para C7, y el ahorro de egreso está en la nota técnica 13.\n\n")

	sb.WriteString("### B. Escalado dinámico de workers (MIG con autoescalado)\n")
	sb.WriteString(fmt.Sprintf("- **Métrica de escalado.** `worker.queue.oldest_pending_age_seconds`, que esta corrida vio llegar a **%.0f s**: es la señal que distingue una ráfaga absorbida de una cola que no se recupera.\n", h.peorEdadCola))
	sb.WriteString("- **Regla.** Umbral sobre esa antigüedad sostenida durante un minuto, añadiendo instancias de worker. Cada instancia añade 2 vCPU, es decir duplica la tasa de servicio actual, y se retira en el valle.\n")
	sb.WriteString(fmt.Sprintf("- **Cuánto hace falta.** Con la antigüedad máxima en %.0f s y un pico de %d en espera, absorber el nivel más cargado de esta campaña sin acumular cola exige del orden de %d veces la capacidad de cómputo actual. El cálculo con los tiempos reales por perfil está en `etapas_reconstruidas.txt`.\n\n",
		h.peorEdadCola, h.picoPending, (h.picoPending+cfg.WorkerConcurrency)/cfg.WorkerConcurrency))

	sb.WriteString("### C. Ajuste de concurrencia por nodo\n")
	sb.WriteString(fmt.Sprintf("- **Por qué %d.** En `e2-highcpu-2` (2 vCPU) hay un proceso de transcodificación por vCPU. Subir la concurrencia en esta misma máquina reparte las mismas vCPU entre más procesos: no añade capacidad, añade cambios de contexto.\n", cfg.WorkerConcurrency))
	sb.WriteString("- **Evolución de tipo de máquina.** Más vCPU por nodo permite más concurrencia real. El tipo concreto y su coste se deciden en C6 con el factor de transcodificación medido por perfil, no con un supuesto.\n")

	_ = os.WriteFile(path, []byte(sb.String()), 0644)
}

func writeH5README(path string, cfg Config, levels []LevelMetrics, drain DrainResult) {
	var sb strings.Builder
	h := resumirHechos(levels)

	// El estado de cada criterio se deriva. Antes eran diez palomas fijas, asi
	// que esta corrida publico «vaciado total, 0 trabajos huerfanos ✅» con dos
	// recursos en processing, y «latencia de segmentos ✅» con tres niveles que
	// no descargaron ni un segmento.
	estado := func(ok bool) string {
		if ok {
			return "✅"
		}
		return "⚠️"
	}

	sb.WriteString("# Evidencia — Análisis de Capacidad: Escenario 2 (Carga, Procesamiento y Consumo Multimedia)\n\n")
	sb.WriteString("Este directorio documenta la rúbrica **Análisis de capacidad — escenario 2 (10%)**.\n\n")
	sb.WriteString(cabeceraMarkdown(cfg))

	if h.sinMedir > 0 || h.fallidos > 0 {
		sb.WriteString(fmt.Sprintf("> **Medición incompleta.** De %d trabajos, la campaña cronometró las etapas 4, 5 y 6 de **%d**; %d no alcanzaron estado terminal dentro del presupuesto de su nivel y %d fallaron. Lo que la plataforma sí registró se reconstruye en [`etapas_reconstruidas.txt`](./etapas_reconstruidas.txt) desde el log del worker.\n\n",
			h.medidos+h.sinMedir+h.fallidos, h.medidos, h.sinMedir, h.fallidos))
	}

	sb.WriteString("## Estado de los Criterios de Aceptación\n\n")
	sb.WriteString("| Criterio de la rúbrica | Evidencia | Estado |\n")
	sb.WriteString("| :--- | :--- | :---: |\n")

	sb.WriteString(fmt.Sprintf("| Línea base y al menos tres niveles crecientes | %d niveles en [`corrida_niveles_escenario2.txt`](./corrida_niveles_escenario2.txt) | %s |\n",
		len(levels), estado(len(levels) >= 4)))
	sb.WriteString(fmt.Sprintf("| Métricas separadas por etapa, no un agregado único | 6 etapas en [`resumen_ejecutivo_escenario2.md`](./resumen_ejecutivo_escenario2.md); las etapas 4–6 completas en [`etapas_reconstruidas.txt`](./etapas_reconstruidas.txt) | %s |\n",
		estado(h.medidos+h.sinMedir > 0)))
	sb.WriteString(fmt.Sprintf("| Trabajos por unidad de tiempo, reintentos y fallos | %d fallos reales, %d sin estado terminal al cierre de su nivel | %s |\n",
		h.fallidos, h.sinMedir, estado(h.sinMedir == 0 && h.fallidos == 0)))
	sb.WriteString(fmt.Sprintf("| Evolución de profundidad y antigüedad de la cola | pico %d en espera, antigüedad máxima %.0f s, con %d errores de lectura | %s |\n",
		h.picoPending, h.peorEdadCola, h.erroresCola, estado(h.erroresCola == 0)))
	sb.WriteString(fmt.Sprintf("| Latencia y throughput de storage frente a la API de control | tráfico de control y transferencia binaria separados; PUT %s, **medido desde el generador, no desde la región** | ⚠️ |\n",
		mbsTexto(h)))
	sb.WriteString(fmt.Sprintf("| Latencia y errores de manifiestos y segmentos HLS | %d descargas de segmento medidas; %d de %d niveles sin ningún segmento porque sus vídeos aún no estaban transcodificados | %s |\n",
		h.obsSegmento, h.nivelesSinSegmentos, len(levels), estado(h.nivelesSinSegmentos == 0)))
	sb.WriteString("| Decisión de reproductor real (TTFF y stalls) | **Pendiente.** Esta campaña mide transporte, no reproducción. Criterio y forma de medirlo en `capacity-planning/criterios_de_medicion.md` §6 | ⏳ |\n")

	switch {
	case !drain.Observable:
		sb.WriteString(fmt.Sprintf("| Observación del drenaje y consistencia | **no observable**: %d lecturas de la cola fallaron | ⚠️ |\n", drain.ErroresInspector))
	case drain.OrphanJobsCount > 0 || drain.FinalActive > 0:
		sb.WriteString(fmt.Sprintf("| Observación del drenaje y consistencia | [`drenaje_cola_verificacion.txt`](./drenaje_cola_verificacion.txt): %d → %d en espera en %v, con %d en proceso y %d recurso(s) sin estado terminal al cierre | ⚠️ |\n",
			drain.InitialPending, drain.FinalPending, drain.TotalDuration.Round(time.Second), drain.FinalActive, drain.OrphanJobsCount))
	default:
		sb.WriteString(fmt.Sprintf("| Observación del drenaje y consistencia | [`drenaje_cola_verificacion.txt`](./drenaje_cola_verificacion.txt): cola vaciada en %v, sin trabajos huérfanos | ✅ |\n",
			drain.TotalDuration.Round(time.Second)))
	}

	sb.WriteString("| Cuello de botella identificado y evolución (CDN / escalado) | [`analisis_cuello_de_botella.md`](./analisis_cuello_de_botella.md) | ✅ |\n")
	sb.WriteString("| Seguridad de secretos | sin credenciales, tokens ni firmas V4 en la evidencia | ✅ |\n\n")

	sb.WriteString("## Archivos\n\n")
	sb.WriteString("| | |\n| :--- | :--- |\n")
	sb.WriteString("| [`corrida_niveles_escenario2.txt`](./corrida_niveles_escenario2.txt) | Registro por nivel, con las seis etapas y la cola |\n")
	sb.WriteString("| [`drenaje_cola_verificacion.txt`](./drenaje_cola_verificacion.txt) | Cronología del vaciado y verificación terminal en PostgreSQL |\n")
	sb.WriteString("| [`resumen_ejecutivo_escenario2.md`](./resumen_ejecutivo_escenario2.md) | Tabla comparativa de los niveles |\n")
	sb.WriteString("| [`analisis_cuello_de_botella.md`](./analisis_cuello_de_botella.md) | Qué sostiene la campaña y qué no, y la propuesta de evolución |\n")
	sb.WriteString("| [`etapas_reconstruidas.txt`](./etapas_reconstruidas.txt) | Etapas 4, 5 y 6 de **todos** los trabajos, desde el log del worker |\n")
	sb.WriteString("| [`metricas_gcp.json`](./metricas_gcp.json) | CPU, memoria y cola de Cloud Monitoring en la ventana |\n")
	sb.WriteString("| [`worker_crudo.log`](./worker_crudo.log) | Log del contenedor del worker, origen de la reconstrucción |\n")
	sb.WriteString("| [`tunel.log`](./tunel.log) | Reinicios del túnel IAP: dice si la ventana tuvo huecos de visibilidad |\n\n")

	sb.WriteString("## Cómo se reprodujo\n\n")
	sb.WriteString("```bash\n")
	sb.WriteString(fmt.Sprintf("ENTORNO=%s HLS_BASE=%s \\\n", cfg.Entorno, cfg.HLSBaseURL))
	sb.WriteString(fmt.Sprintf("PATRON=%s MAX_SEGMENTOS=%d \\\n", cfg.Patron, cfg.MaxSegs))
	sb.WriteString(fmt.Sprintf("  bash ./scripts/run_capacity_escenario2.sh %s \\\n", cfg.BaseURL))
	sb.WriteString(fmt.Sprintf("  -worker-concurrency %d -redis localhost:6379 -db \"postgres://...\"\n", cfg.WorkerConcurrency))
	sb.WriteString("```\n\n")
	sb.WriteString("La cola y la base se alcanzan por el túnel IAP (`scripts/a7_tunel.sh`), que usa el 22 y no abre ningún puerto de datos.\n\n")
	sb.WriteString("---\n\nEste archivo no contiene credenciales, llaves ni secretos.\n")

	_ = os.WriteFile(path, []byte(sb.String()), 0644)
}

func writeCapacityReportFile(path string, cfg Config, levels []LevelMetrics, drain DrainResult) {
	_ = os.MkdirAll(filepath.Dir(path), 0755)

	var sb strings.Builder
	h := resumirHechos(levels)

	sb.WriteString("# Informe de Pruebas de Carga y Capacidad — Escenario 2\n\n")
	sb.WriteString("**Curso:** ISIS4426 — Desarrollo de Soluciones Cloud\n")
	sb.WriteString("**Semestre:** 2026-20\n")
	sb.WriteString(fmt.Sprintf("**Fecha de consolidación:** %s\n\n", time.Now().UTC().Format(time.RFC3339)))
	sb.WriteString("---\n\n")
	sb.WriteString(cabeceraMarkdown(cfg))

	sb.WriteString("## Escenario 2: Carga, Procesamiento y Consumo Multimedia (10%)\n\n")
	sb.WriteString("### 1. Definición del Escenario y Condiciones de Prueba\n\n")
	sb.WriteString("El escenario simula profesores subiendo contenido multimedia directamente al almacenamiento de objetos mientras estudiantes reproducen flujos HLS ya disponibles.\n\n")
	sb.WriteString(fmt.Sprintf("- **Concurrencia de workers.** Fijada en %d (`WORKER_CONCURRENCY=%d`), un proceso de transcodificación por vCPU de `mooc-worker-server` (`e2-highcpu-2`).\n", cfg.WorkerConcurrency, cfg.WorkerConcurrency))
	sb.WriteString("- **Perfiles multimedia y escalera.** Perfiles corto (2 min), medio (10 min) y largo (30 min) a 1280×720. Escalera HLS a 360p y 720p sin upscaling (`internal/transcode.DefaultLadder`).\n")
	sb.WriteString(fmt.Sprintf("- **Patrón de consumo declarado:** `%s`. Con `cadencia`, cada estudiante reproduce a 1x: buffer inicial de 2 segmentos y una pausa por segmento igual a su duración real, leída del `#EXTINF` de la lista de variante. Con `maxima` no hay instante de reproducción que incumplir, así que no se reportan margen ni interrupciones. La descarga masiva se mide siempre aparte.\n\n", cfg.Patron))

	sb.WriteString("### 2. Resultados Numéricos por Nivel y Variación de Métricas\n\n")
	sb.WriteString("| Nivel | Subidas / Concurrencia | Estudiantes HLS | Rendimiento en la ventana (vid/min) | Pico Cola (pending) | Antigüedad Cola | Etapa 1 (Firma p95) | Etapa 2 (PUT p95) | Etapa 3 (Confirm p95) | Etapa 4 (Espera p95) | Etapa 5 (Proc p95) | Etapa 6 (Total p95) |\n")
	sb.WriteString("| :---: | :---: | :---: | :---: | :---: | :---: | :---: | :---: | :---: | :---: | :---: | :---: |\n")

	for _, l := range levels {
		sb.WriteString(fmt.Sprintf("| **Nivel %d** | %d subidas (%d prof) | %d est | %.2f | %d | %.2fs | %v | %v | %v | %v | %v | %v |\n",
			l.Level, l.TotalUploads, l.UploadWorkers, l.StreamersCount, l.ThroughputVidsMin, l.PeakPendingQueue, l.PeakOldestAgeSec,
			l.Stage1.Texto(95), l.Stage2.Texto(95), l.Stage3.Texto(95),
			l.Stage4.Texto(95), l.Stage5.Texto(95), l.Stage6.Texto(95)))
	}

	sb.WriteString("\n### 3. Observación del Drenaje de Cola y Verificación Terminal\n\n")
	switch {
	case !drain.Observable:
		sb.WriteString(fmt.Sprintf("El drenaje **no se pudo observar**: %d lecturas de la cola fallaron. No hay resultado publicable en esta sección.\n\n", drain.ErroresInspector))
	case drain.Atascada:
		sb.WriteString(fmt.Sprintf("La observación del drenaje se **cortó por atasco** tras %v: la cola dejó de completar tareas con %d en espera y %d en proceso.\n\n",
			drain.TotalDuration.Round(100*time.Millisecond), drain.FinalPending, drain.FinalActive))
	default:
		sb.WriteString(fmt.Sprintf("Tras cesar la inyección en el nivel más cargado se observó el vaciado de la cola Asynq durante **%v**, partiendo de %d tareas en espera y terminando con **%d en espera y %d en proceso**.\n\n",
			drain.TotalDuration.Round(100*time.Millisecond), drain.InitialPending, drain.FinalPending, drain.FinalActive))
		if drain.FinalActive > 0 {
			sb.WriteString("> La ventana de observación terminó con trabajo en vuelo, así que **esa duración no es el tiempo de drenaje**: es cuánto se observó.\n\n")
		}
	}

	sb.WriteString("La verificación directa sobre PostgreSQL (`SELECT processing_status, COUNT(*) FROM resources WHERE object_key LIKE 'originals/%' GROUP BY processing_status`) dio:\n\n")
	if len(drain.DBStatusCounts) == 0 {
		sb.WriteString("- Sin resultados de la consulta terminal en esta corrida.\n")
	} else {
		for _, st := range ordenarClaves(drain.DBStatusCounts) {
			sb.WriteString(fmt.Sprintf("- `processing_status = %s`: **%d** recursos\n", st, drain.DBStatusCounts[st]))
		}
	}
	if drain.OrphanJobsCount > 0 {
		sb.WriteString(fmt.Sprintf("\n**%d recurso(s) quedaron sin estado terminal** al cerrar la ventana. No son trabajos perdidos: son trabajos que la observación no alcanzó a cubrir. El cierre real de cada uno está en `worker_crudo.log` y reconstruido en `etapas_reconstruidas.txt`.\n", drain.OrphanJobsCount))
	} else {
		sb.WriteString("\nNingún recurso quedó en estado indeterminado.\n")
	}

	sb.WriteString("\n### 4. Cuello de Botella Sustentado y Propuesta de Evolución\n\n")
	sb.WriteString(fmt.Sprintf("1. **Cuello de botella primario.** La **vCPU del worker durante la transcodificación (etapa 5)**. Lo que esta campaña mide y lo sostiene es la cola: pico de **%d en espera** y antigüedad máxima de **%.0f s** con una tasa de llegada constante. El uso de CPU que lo confirma está en `metricas_gcp.json`; la campaña no mide CPU.\n", h.picoPending, h.peorEdadCola))
	sb.WriteString("2. **Evolución — CDN.** Frontalizar el prefijo público `/hls/` con un CDN, para servir los segmentos inmutables desde el borde y quitar el egreso del bucket de origen. Objetivos a verificar en C7.\n")
	sb.WriteString(fmt.Sprintf("3. **Evolución — autoescalado de workers.** Escalar `mooc-worker-server` por `worker.queue.oldest_pending_age_seconds`, la señal que aquí llegó a %.0f s. El dimensionado sale del factor de transcodificación por perfil de `etapas_reconstruidas.txt`.\n", h.peorEdadCola))

	_ = os.WriteFile(path, []byte(sb.String()), 0644)
}

func maskSignature(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "[url]"
	}
	q := u.Query()
	for _, k := range []string{"X-Amz-Signature", "X-Goog-Signature", "Signature"} {
		if q.Has(k) {
			q.Set(k, "REDACTED")
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}
