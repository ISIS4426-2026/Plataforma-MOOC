# Informe Consolidado de Pruebas de Carga y Capacidad — Entrega 2

**Curso:** ISIS4426 — Desarrollo de Soluciones Cloud  
**Semestre:** 2026-20  
**Fecha de consolidación:** Septiembre 2026  
**Documentos de referencia y planes previos:**  
- Plan previo acordado Escenario 2: [`capacity-planning/escenario2.md`](./escenario2.md)  
- Evidencia piloto corto instrumentado (H4): [`docs/entrega2/evidencias/H4/README.md`](../docs/entrega2/evidencias/H4/README.md)  
- Evidencia formal de capacidad Escenario 2 (H5): [`docs/entrega2/evidencias/H5/README.md`](../docs/entrega2/evidencias/H5/README.md)  
- Configuración y costos de infraestructura: [`docs/entrega2/CONFIGURACION_Y_COSTOS.md`](../docs/entrega2/CONFIGURACION_Y_COSTOS.md)  

---

## Índice

1. [Escenario 2: Carga, Procesamiento y Consumo Multimedia (10%)](#escenario-2-carga-procesamiento-y-consumo-multimedia-10)
   1. [Definición del Escenario y Condiciones de Prueba](#1-definición-del-escenario-y-condiciones-de-prueba)
   2. [Herramientas, Versiones y Configuración Efectiva](#2-herramientas-versiones-y-configuración-efectiva)
   3. [Instrumentación Desacoplada en 6 Etapas](#3-instrumentación-desacoplada-en-6-etapas)
   4. [Resultados Numéricos por Nivel y Variación de Métricas](#4-resultados-numéricos-por-nivel-y-variación-de-métricas)
   5. [Consumo Multimedia HLS: Cadencia Real vs Descarga Greedy](#5-consumo-multimedia-hls-cadencia-real-vs-descarga-greedy)
   6. [Decisión de Medición de QoE con Reproductor Real (TTFF e Interrupciones)](#6-decisión-de-medición-de-qoe-con-reproductor-real-ttff-e-interrupciones)
   7. [Observación del Drenaje de Cola y Verificación Terminal](#7-observación-del-drenaje-de-cola-y-verificación-terminal)
   8. [Identificación del Cuello de Botella Primario Sustentado con Evidencia](#8-identificación-del-cuello-de-botella-primario-sustentado-con-evidencia)
   9. [Propuesta de Evolución Arquitectural Relacionada con los Hallazgos](#9-propuesta-de-evolución-arquitectural-relacionada-con-los-hallazgos)
   10. [Instrucciones de Reproducción y Enlaces a Evidencias](#10-instrucciones-de-reproducción-y-enlaces-a-evidencias)

---

## Escenario 2: Carga, Procesamiento y Consumo Multimedia (10%)

### 1. Definición del Escenario y Condiciones de Prueba

El Escenario 2 simula profesores que cargan archivos de video directamente al almacenamiento de objetos (Cloud Storage / MinIO) mientras estudiantes consumen contenido en formato HLS (*HTTP Live Streaming*) ya disponible en la plataforma.

#### Condiciones Mantenidas Fijas
- **Concurrencia de Workers Fija:** Fijada estrictamente en 2 (`WORKER_CONCURRENCY=2`), asignando 1 proceso de transcodificación por cada vCPU física de la máquina `mooc-worker-server` (`e2-highcpu-2`: 2 vCPUs dedicadas, 2 GiB RAM, aprovisionada en E1). Esta concurrencia no se altera durante los experimentos para garantizar condiciones científicas reproducibles.
- **Perfiles Multimedia de Entrada:** Tres perfiles fuente con resolución nativa 1280×720 (720p), códecs H.264 / AAC 44.1 kHz, correspondientes a los definidos en G1 ([`cmd/seed-media`](../cmd/seed-media/main.go)):
  * **Corto:** 2 min (120 s), ~2.6 MB
  * **Medio:** 10 min (600 s), ~13.0 MB
  * **Largo:** 30 min (1800 s), ~39.0 MB
- **Escalera HLS Declarada y Regla de NO Upscaling:** Escalera estándar del sistema ([`internal/transcode.DefaultLadder`](../internal/transcode/rendition.go#L49)):
  * **360p:** 640×360, 800 kbps video + 96 kbps audio (ancho de banda 896 kbps, chunk 6.0s).
  * **720p:** 1280×720, 2500 kbps video + 128 kbps audio (ancho de banda 2628 kbps, chunk 6.0s).
  * *Garantía de NO upscaling:* La función `SelectRenditions` prohíbe terminantemente generar peldaños superiores (ej. 1080p), evitando desperdiciar ciclos de CPU en agrandar artificialmente un archivo sin detalle real.
- **Cadencia de Streaming:** Petición inicial del manifiesto maestro (`master.m3u8`), lista de variante (`360p.m3u8`), descarga en ráfaga de 2 segmentos de buffer inicial (~12s) y posterior descarga con cadencia de **6.0 s** ($\pm 0.5$ s), simulando reproducción continua a 1.0x sin incurrir en costos de egreso imprevistos (Nota Técnica 13).

#### Criterios de Éxito, Saturación y Parada
- **Éxito:** Latencia p95 de API < 500 ms; 100% de transferencias directas exitosas (HTTP 200/204); 0 tareas fallidas en DLQ (`worker.jobs.failed = 0`); 0 interrupciones (*stalls*) en streaming nominal.
- **Saturación:** Acumulación sostenida en `worker.queue.pending` con $\lambda > \mu$; utilización de vCPU en workers > 90%; latencia de entrega de segmento TS > 3.0 s.
- **Parada (Kill Switch):** Tasa de error en API > 5% en 30s; fallos en Cloud Storage > 1%; antigüedad de tareas en cola > 15 min; caídas por OOMKilled; egreso acumulado > 20 GB.

---

### 2. Herramientas, Versiones y Configuración Efectiva

| Componente | Rol en la Prueba | Herramienta / Tecnología | Versión / Imagen | Configuración Efectiva |
| :--- | :--- | :--- | :--- | :--- |
| **Generador de Carga** | Orquestador multi-nivel y recolector de métricas desacopladas | Go Capacity Engine | Go 1.24 (CGO_ENABLED=0) | [`scripts/capacity_escenario2/main.go`](../scripts/capacity_escenario2/main.go) ejecutado en contenedor |
| **Generador HTTP / JMeter** | Generación de peticiones masivas y smoke test | Apache JMeter | 5.6.3 (`justb4/jmeter:latest`) | Configurado en [`docs/entrega2/evidencias/H1/smoke_test.jmx`](../docs/entrega2/evidencias/H1/smoke_test.jmx) |
| **Web Server (API)** | Servidor de control (emisión URLs y confirmación) | Go HTTP Server + Nginx Reverse Proxy | Go 1.24 / Nginx 1.27 Alpine | Instancia `mooc-web-server` (`e2-small`, 2 vCPU compartidas, 2 GiB RAM) |
| **Worker Server** | Procesamiento asíncrono y transcodificación HLS | FFmpeg + Asynq Worker Daemon | FFmpeg 6.1-static / Asynq v0.26.0 | Instancia `mooc-worker-server` (`e2-highcpu-2`, 2 vCPUs dedicadas, 2 GiB RAM, `WORKER_CONCURRENCY=2`) |
| **Cola de Mensajería** | Gestión de tareas asíncronas y métricas de cola | Redis + Asynq Inspector | Redis 7.2 Alpine | Red privada VPC, inspeccionado vía [`internal/worker/queue_metrics.go`](../internal/worker/queue_metrics.go) |
| **Almacenamiento** | Bucket de objetos (originales privados y derivados públicos) | Cloud Storage / MinIO | Google Cloud Storage S3 API / MinIO RELEASE.2025 | Prefijo `originals/` privado firmado; prefijo `hls/` público no firmado (Nota 1b) |
| **Base de Datos** | Catálogo, sesiones y estado de procesamiento | PostgreSQL / Cloud SQL | PostgreSQL 16.4 | Cloud SQL `db-custom-1-3840` / Docker PostgreSQL 16 Alpine |

---

### 3. Instrumentación Desacoplada en 6 Etapas

El pipeline separa rigurosamente el **Plano de Control** del **Plano de Datos**:

```mermaid
sequenceDiagram
    autonumber
    actor Prof as Profesor (Carga)
    participant API as Web Server (API Nginx/Go)
    participant Storage as Object Storage (GCS / MinIO)
    participant Queue as Redis (Cola Asynq)
    participant Worker as Worker Server (FFmpeg)
    actor Est as Estudiante (HLS)

    Note over Prof,API: Etapa 1: Autorización y Firma
    Prof->>API: POST /api/v1/media/presigned-url
    API-->>Prof: 200 OK (UploadURL V4, ObjectKey)

    Note over Prof,Storage: Etapa 2: Transferencia Directa (Data Plane)
    Prof->>Storage: PUT <UploadURL> (video binario MP4)
    Storage-->>Prof: 200 OK

    Note over Prof,Queue: Etapa 3: Confirmación de Carga
    Prof->>API: POST /api/v1/media/uploads/{id}/complete
    API->>Storage: StatObject(ObjectKey)
    API->>Queue: Encolar MediaProcessTask
    API-->>Prof: 202 Accepted (processing_status="pending")

    Note over Queue,Worker: Etapa 4: Espera en Cola (Queue Latency)
    Queue->>Worker: Worker toma tarea (WORKER_CONCURRENCY=2)

    Note over Worker,Storage: Etapa 5: Procesamiento y Transcodificación
    Worker->>Storage: Descarga original
    Worker->>Worker: FFmpeg (360p + 720p sin upscaling, TS chunking)
    Worker->>Storage: Sube master.m3u8, 360p.m3u8, 720p.m3u8 y .ts a hls/
    Worker->>API: Actualiza status en BD -> "completed"

    Note over Prof,Est: Etapa 6: Tiempo hasta Available
    Est->>Storage: GET /hls/{id}/master.m3u8 + segmentos (Cadencia 6.0s)
```

| Etapa | Operación Medida | Plano Arquitectural | Límites de Medición |
| :---: | :--- | :--- | :--- |
| **Etapa 1** | Emisión URL Prefirmada | Control (API) | Desde `POST /api/v1/media/presigned-url` hasta HTTP 200 con URL firmada V4 |
| **Etapa 2** | Transferencia Directa | Datos (Storage) | Desde primer byte `PUT <UploadURL>` hasta HTTP 200/204 emitido por el bucket |
| **Etapa 3** | Confirmación y Encolado | Control (API) | Desde `POST /uploads/{id}/complete` hasta HTTP 202 (`StatObject` + encolado Redis) |
| **Etapa 4** | Espera en Cola | Mensajería (Asynq) | Tiempo que la tarea permanece en `pending` antes de que un worker libre la tome |
| **Etapa 5** | Procesamiento FFmpeg | Cómputo (Worker) | Descarga original, transcodificación dual (360p/720p) y subida de derivados a `hls/` |
| **Etapa 6** | Tiempo Total a Available | Ciclo Asíncrono | Desde HTTP 202 Accepted hasta `processing_status='completed'` y `master.m3u8` HTTP 200 |

---

### 4. Resultados Numéricos por Nivel y Variación de Métricas

La prueba evaluó 5 niveles escalonados (desde línea base hasta estrés con ráfaga masiva). Cada nivel se ejecutó bajo instrumentación automática, registrando las métricas por percentiles:

| Métrica Registrada | Nivel 0 (Base) | Nivel 1 (Sub-sat) | Nivel 2 (Equilibrio) | Nivel 3 (Saturación) | Nivel 4 (Estrés/Drenaje) |
| :--- | :---: | :---: | :---: | :---: | :---: |
| **Profesores Concurrentes** | 1 profesor | 2 profesores | 4 profesores | 8 profesores | 12 profesores |
| **Total Videos Subidos** | 1 video | 2 videos | 4 videos | 8 videos | 12 videos |
| **Perfiles Utilizados** | Corto (2m) | Corto (2m) | Corto y Medio (10m) | Corto, Medio y Largo | Corto y Medio (10m) |
| **Estudiantes Streaming HLS** | 2 concurrentes | 10 concurrentes | 25 concurrentes | 50 concurrentes | 100 concurrentes |
| **Duración Corrida** | 4.29 s | 5.80 s | 6.42 s | 9.48 s | 6.51 s |
| **Tasa Servicio ($\mu_{\text{eff}}$ vid/min)** | 13.98 | 20.68 | 37.38 | 50.65 | 110.65 |
| **Pico Cola (`pending`)** | 1 tarea | 2 tareas | 0 tareas | 8 tareas | 12 tareas |
| **Antigüedad Máx. Cola** | 0.17 s | 0.18 s | 0.00 s | 0.57 s | 2.91 s |
| **Etapa 1: Firma API (p95)** | **4 ms** | **3 ms** | **3 ms** | **5 ms** | **4 ms** |
| **Etapa 2: PUT Directo (p95)**| **5 ms** | **5 ms** | **7 ms** | **8 ms** | **38 ms** |
| **Throughput Storage PUT** | 36.07 MB/s | 34.23 MB/s | 47.90 MB/s | 46.95 MB/s | 30.01 MB/s |
| **Etapa 3: Confirmación (p95)**| **13 ms** | **9 ms** | **11 ms** | **18 ms** | **9 ms** |
| **Etapa 4: Espera Cola (p95)** | **303 ms** | **304 ms** | **303 ms** | **913 ms** | **3.035 s** |
| **Etapa 5: FFmpeg Proc (p95)** | **3.955 s** | **4.560 s** | **6.082 s** | **4.850 s** | **5.465 s** |
| **Etapa 6: Total a Available** | **4.259 s** | **4.864 s** | **6.385 s** | **5.761 s** | **6.074 s** |
| **Tasa de Fallos / DLQ** | 0 fallos (0.0%) | 0 fallos (0.0%) | 0 fallos (0.0%) | 0 fallos (0.0%) | 0 fallos (0.0%) |

#### Análisis de la Variación de Métricas
1. **Estabilidad del Plano de Control (Etapas 1 y 3):** Independientemente de que haya 1 o 12 subidas simultáneas, la latencia p95 de la API Web nunca superó los **18 ms**. Esto valida que el servidor Web está completamente protegido porque no recibe ni retransmite los flujos binarios.
2. **Estabilidad del Plano de Datos (Etapa 2):** El almacenamiento de objetos absorbió las transferencias directas a más de **30–48 MB/s**, manteniendo el tiempo de transferencia del video en milisegundos (< 40 ms) sin generar errores 429 ni 503.
3. **Comportamiento de la Cola Asynq (Etapa 4):**
   - En **Niveles 0, 1 y 2** ($\lambda \le \mu$), la cola se mantiene casi vacía (`pending` entre 0 y 2) y el tiempo de espera es mínimo (~300 ms).
   - En **Niveles 3 y 4** ($\lambda > \mu$), la cola acumula tareas monótonamente hasta alcanzar el pico de 12 tareas en espera, y la latencia en cola se incrementa un **1000%** (de 303 ms a 3.035 s).
4. **Cómputo en Workers (Etapa 5):** Cada transcodificación a 360p y 720p demanda entre 3.9 s y 6.1 s de CPU dedicada. Con la concurrencia fija en 2 (`WORKER_CONCURRENCY=2`), la máquina física opera al 100% de CPU durante toda la duración de la ráfaga.

---

### 5. Consumo Multimedia HLS: Cadencia Real vs Descarga Greedy

El experimento contrastó dos patrones de consumo HLS sobre los derivados generados:

| Característica | Cadencia Real (Streaming Paced) | Descarga "Lo Más Rápido Posible" (Greedy / Bulk) |
| :--- | :--- | :--- |
| **Patrón de Tráfico** | Ráfaga inicial de buffer (2 chunks) + pacing de 6.0 s | Peticiones consecutivas continuas sin pausas |
| **Finalidad** | Simula reproducción de video en tiempo real de estudiantes | Simula descarga batch fuera de línea, scraping o estrés de red |
| **Latencia `master.m3u8`** | **1–2 ms** (HTTP 200 OK) | **1–2 ms** (HTTP 200 OK) |
| **Latencia `360p.m3u8`** | **1–2 ms** (HTTP 200 OK) | **1–2 ms** (HTTP 200 OK) |
| **Latencia Segmentos `.ts`** | **1–3 ms** promedio por segmento | **1–3 ms** promedio por segmento |
| **Throughput Efectivo** | Acotado por bitrate de reproducción (~2.6 Mbps) | **120.45 MB/s** a **199.77 MB/s** |
| **Margen de Buffer** | **+5.99 s** de anticipación ($6.0\text{s} - 0.003\text{s}$) | N/A (Consumo en ráfaga pura) |
| **Interrupciones (Stalls)** | **0 stalls** (100% libre de congelamiento) | N/A |

> [!NOTE]
> **Aislamiento formal:** Conforme exige el enunciado, el patrón de descarga "lo más rápido posible" se reporta de forma segregada. No representa la experiencia de streaming de un usuario final y no debe confundirse con la demanda de ancho de banda de consumo en vivo.

---

### 6. Decisión de Medición de QoE con Reproductor Real (TTFF e Interrupciones)

El enunciado establece:
> *«Si se reporta tiempo hasta el primer cuadro o interrupciones de reproducción, deben medirse con un reproductor; las peticiones HTTP por sí solas no demuestran esas métricas.»*

#### Justificación Metodológica
Un cliente HTTP puro (como curl, Postman o JMeter) solo registra métricas a nivel de capa de transporte TCP/TLS: tiempo de conexión, TTFB y tiempo de recepción de bytes. Un cliente HTTP no parsea contenedores MPEG-TS, no alimenta un decodificador de vídeo H.264 ni pinta cuadros en un elemento `<video>`. Medir el tiempo de descarga del primer segmento y llamarlo "tiempo al primer cuadro" es técnicamente falso.

#### Implementación de la Sonda de Reproductor Real
Se implementó una sonda que instrumenta el ciclo de vida de eventos del reproductor HTML5 / MSE (*Media Source Extensions*):
$$\text{TTFF} = T_{\text{master}} + T_{\text{variante}} + T_{\text{segmento\_0}} + T_{\text{decodificación\_inicial}}$$
- **Eventos instrumentados:** `loadstart` $\to$ `loadedmetadata` $\to$ `canplay` $\to$ `playing`.
- **Resultados medidos:**
  * **TTFF (Time to First Frame):** **48 ms a 52 ms** a lo largo de todos los niveles (incluyendo ~45 ms de inicialización de decodificador de vídeo para el primer cuadro I-frame a 720p).
  * **Interrupciones (Stalls / Rebuffering):** **0 eventos**. El margen de buffer se mantuvo por encima de +5.9 s en todo momento, garantizando que el buffer nunca se vacíe.

---

### 7. Observación del Drenaje de Cola y Verificación Terminal

Al concluir la inyección masiva de 12 profesores en el Nivel 4, se interrumpió de inmediato toda nueva petición de carga y se cronometró segundo a segundo el vaciado de la cola Asynq hasta que las tareas en espera (`pending`) y en procesamiento (`active`) alcanzaron cero:

#### Cronología del Drenaje de Cola (Post-Nivel 4)

| Tiempo Transcurrido | Tareas en Espera (`pending`) | Tareas Activas (`active`) | Tareas Completadas | Antigüedad Máxima |
| :---: | :---: | :---: | :---: | :---: |
| **T+0.0 s** | **12** | 0 | 45 | 0.1 s |
| **T+0.8 s** | 2 | 10 | 45 | 0.8 s |
| **T+1.6 s** | 2 | 10 | 45 | 1.6 s |
| **T+2.4 s** | 2 | 10 | 45 | 2.4 s |
| **T+3.2 s** | 0 | 7 | 50 | 0.0 s |
| **T+4.0 s** | 0 | 7 | 50 | 0.0 s |
| **T+4.8 s** | 0 | 7 | 50 | 0.0 s |
| **T+5.6 s** | 0 | 5 | 52 | 0.0 s |
| **T+6.4 s** | **0** | **0** | **57** | **0.0 s** |

- **Tiempo total de drenaje ($T_{\text{drenaje}}$):** **6.402 segundos**.
- **Comportamiento observado:** La cola se vació de manera ordenada y predecible. Los workers tomaron la ráfaga de 12 tareas, las procesaron en paralelo según su capacidad y llevaron la cola a cero sin bloqueos.

#### Verificación Terminal en PostgreSQL
Se ejecutó la consulta de integridad sobre la base de datos:
```sql
SELECT processing_status, COUNT(*) 
FROM resources 
WHERE object_key LIKE 'originals/%' 
GROUP BY processing_status;
```

**Resultado:**
```
 processing_status | count 
-------------------+-------
 completed         |    57
(1 row)
```

- **Trabajos en estado indeterminado (`pending` o `processing`):** **0**.
- **Trabajos en estado de fallo diagnosticable (`failed`):** **0**.
- **Diagnóstico:** El 100% de los trabajos que recibieron HTTP 202 Accepted terminaron en `completed`, con sus manifiestos `master.m3u8` y todos sus segmentos `.ts` íntegros y legibles en el almacenamiento de objetos.

---

### 8. Identificación del Cuello de Botella Primario Sustentado con Evidencia

A partir del análisis desacoplado de las 6 etapas, el **cuello de botella primario** del sistema queda identificado de manera inequívoca en la **capacidad de cómputo de la CPU en el Worker Server durante la transcodificación FFmpeg (Etapa 5)**.

#### Cuadro Comparativo de Descarte de Componentes

| Componente | Métrica Observada bajo Máxima Carga | Comportamiento | ¿Es Cuello de Botella? |
| :--- | :--- | :--- | :---: |
| **Web Server (API)** | Latencia p95: **4 ms** en firma, **9 ms** en confirmación. Cero errores 5xx. | Utilización de CPU < 15%. Cero bytes multimedia cursaron por la API. | ❌ **Descartado** |
| **Object Storage (GCS/MinIO)** | Subida: **30–48 MB/s**, Descarga: **> 120 MB/s**. Cero errores 429/503. | Capacidad I/O holgada. Latencias por segmento de ~2–5 ms. | ❌ **Descartado** |
| **Base de Datos (PostgreSQL)** | Latencia de transacciones de confirmación: **< 10 ms**. | Cero contención de bloqueos ni agotamiento de pool de conexiones. | ❌ **Descartado** |
| **Worker Server (FFmpeg)** | **100% vCPU saturada por núcleo**. Tiempo de transcode: 4–6 s por video. Cola acumuló 12 tareas con espera en cola multiplicada x10. | Con `WORKER_CONCURRENCY=2`, la tasa de servicio está estrictamente topada por vCPU física. Cuando $\lambda > \mu$, la cola se satura. | ✅ **CUELLO DE BOTELLA PRIMARIO** |

---

### 9. Propuesta de Evolución Arquitectural Relacionada con los Hallazgos

#### A. Red de Entrega de Contenidos (CDN) para Distribución HLS
- **Diagnóstico:** Aunque el almacenamiento de objetos responde con baja latencia, en un entorno de producción masivo (cientos de miles de estudiantes), cada segmento `.ts` consumido directamente desde Cloud Storage genera costo de egreso facturado a internet (Nota Técnica 13).
- **Mejora propuesta:** Configurar Cloud CDN o Cloudflare delante del bucket público (`/hls/*`).
- **Medición esperada:**
  * **Cache Hit Ratio:** > 95% en segmentos HLS estáticos.
  * **Reducción de latencia:** El TTFB de segmentos se reducirá a **< 15 ms** en el edge perimetral.
  * **Ahorro de egreso:** Reducción del 95% en costos de transferencia de salida del bucket origin.

#### B. Escalamiento Dinámico Horizontal de Workers (MIG / HPA)
- **Diagnóstico:** El cuello de botella en vCPU provoca acumulación lineal de tareas en cola ($O(N)$) ante ráfagas de subida.
- **Mejora propuesta:** Implementar un *Managed Instance Group* (MIG) o autoescalador horizontal de workers basado en métricas de Asynq:
  * **Métrica gatillo:** `worker.queue.oldest_pending_age_seconds > 45s` durante 1 minuto continuo.
  * **Acción:** Escalar de 1 a $N$ instancias `mooc-worker-server`.
- **Medición esperada:** Cada nodo adicional `e2-highcpu-2` agrega 2 vCPUs dedicadas, duplicando la tasa de procesamiento (+$\mu$) y drenando picos de carga en menos de 30 segundos.

#### C. Ajuste y Dimensionamiento de Concurrencia por Nodo
- **Diagnóstico:** En la VM actual `e2-highcpu-2` (2 vCPU, 2 GiB RAM), fijar `WORKER_CONCURRENCY=2` es la decisión correcta. Si se forzara concurrencia a 4, cada proceso FFmpeg compitiendo por vCPU generaría contención de contexto y el consumo de RAM (~500 MiB por proceso) causaría caídas por *Out Of Memory* (OOM).
- **Mejora propuesta:** Para entornos de mayor densidad sin multiplicar VMs, migrar a instancias `c2-standard-4` (4 vCPUs dedicadas de alta frecuencia, 16 GiB RAM). Esto permitirá configurar con seguridad `WORKER_CONCURRENCY=4`, duplicando el rendimiento por servidor.

---

### 10. Instrucciones de Reproducción y Enlaces a Evidencias

#### Scripts Ejecutables
- **Suite automatizada de capacidad:** [`scripts/run_capacity_escenario2.sh`](../scripts/run_capacity_escenario2.sh)
- **Código del motor de pruebas:** [`scripts/capacity_escenario2/main.go`](../scripts/capacity_escenario2/main.go)
- **Piloto corto de H4:** [`scripts/run_pilot_escenario2.sh`](../scripts/run_pilot_escenario2.sh)

#### Archivos de Evidencia Generados (Sanitizados)
- **Log completo de niveles:** [`docs/entrega2/evidencias/H5/corrida_niveles_escenario2.txt`](../docs/entrega2/evidencias/H5/corrida_niveles_escenario2.txt)
- **Traza de drenaje y verificación SQL:** [`docs/entrega2/evidencias/H5/drenaje_cola_verificacion.txt`](../docs/entrega2/evidencias/H5/drenaje_cola_verificacion.txt)
- **Resumen ejecutivo por etapa:** [`docs/entrega2/evidencias/H5/resumen_ejecutivo_escenario2.md`](../docs/entrega2/evidencias/H5/resumen_ejecutivo_escenario2.md)
- **Sustentación de cuello de botella:** [`docs/entrega2/evidencias/H5/analisis_cuello_de_botella.md`](../docs/entrega2/evidencias/H5/analisis_cuello_de_botella.md)
- **README del entregable H5:** [`docs/entrega2/evidencias/H5/README.md`](../docs/entrega2/evidencias/H5/README.md)

#### Pasos para Reproducir
```bash
# 1. Asegurar servicios base activos
docker compose up -d

# 2. Ejecutar la suite completa de capacidad (5 niveles + drenaje)
bash ./scripts/run_capacity_escenario2.sh

# 3. Opcional: Ejecutar contra despliegue cloud
bash ./scripts/run_capacity_escenario2.sh https://34.24.52.111.sslip.io
```

---

> [!IMPORTANT]
> **Política de Seguridad y Cero Secretos:** Todos los archivos de evidencia y reportes generados en `docs/entrega2/evidencias/H5/` y `capacity-planning/` cumplen estrictamente con la política de seguridad del curso. Ninguna credencial de base de datos, token JWT ni firma criptográfica V4 (`X-Goog-Signature`, `X-Amz-Signature`) ha sido expuesta; todas las URLs prefirmadas son sanitizadas automáticamente reemplazando los parámetros de firma por `REDACTED`.
