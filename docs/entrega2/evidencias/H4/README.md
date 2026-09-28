# Evidencia H4 — Plan y Piloto con Instrumentación por Etapa del Escenario 2 (Issue H4)

Este directorio documenta el cumplimiento del entregable **H4**, correspondiente al **Análisis de capacidad — Escenario 2: Carga, procesamiento y consumo multimedia (10%)** de la Entrega 2.

Tal como estipula el enunciado y los criterios de aceptación, este hito define formalmente todos los parámetros de prueba, concurrencia, perfiles y criterios de parada en el documento acordado [`capacity-planning/escenario2.md`](../../../../capacity-planning/escenario2.md) **antes de ejecutar** la carga pesada (H5), respaldado por un **piloto corto** con las etapas instrumentadas y medidas de forma independiente.

---

## Estado y Criterios de Aceptación

| Criterio de la Rúbrica / Tarea | Evidencia / Implementación | Estado |
| :--- | :--- | :---: |
| **Documento previo acordado** | [`capacity-planning/escenario2.md`](../../../../capacity-planning/escenario2.md): Especificación exhaustiva antes de ejecutar | ✅ |
| **Tres perfiles de G1 y rendiciones sin upscaling** | Perfiles Corto (2m), Medio (10m) y Largo (30m) a 1280×720; escalera `360p` + `720p` (`internal/transcode.DefaultLadder`) | ✅ |
| **Niveles crecientes con concurrencia fija** | Concurrencia de workers fija en 2 (`WORKER_CONCURRENCY=2`, VM `e2-highcpu-2` en E1); 5 niveles definidos (desde línea base hasta saturación y drenaje) | ✅ |
| **Cadencia de reproducción real vs greedy** | Streaming con ráfaga de buffer (2 chunks) + pacing de 6.0s; descarga "lo más rápido posible" identificada y aislada como prueba de saturación de red | ✅ |
| **Decisión TTFF e interrupciones** | Medición de QoE mediante sonda de reproductor real (eventos HTML5/MSE), declarando por qué peticiones HTTP solas no demuestran TTFF ni stalls | ✅ |
| **Instrumentación desacoplada por etapa** | Cronometraje individual de las 6 etapas: emisión URL, PUT directo, confirmación, espera en cola, procesamiento worker y tiempo a available | ✅ |
| **Separación de tráfico de control y datos** | Tráfico de control (API Nginx/Go en puerto 443) vs plano de datos (transferencia binaria directa con el almacenamiento de objetos) | ✅ |
| **Criterios de éxito, saturación y parada** | Umbrales definidos en §7 de `escenario2.md` (p95 API < 500ms, errores > 5% gatillan parada, etc.) | ✅ |
| **Piloto corto ejecutable y reproducible** | Script [`scripts/run_pilot_escenario2.sh`](../../../../scripts/run_pilot_escenario2.sh) y resultados en [`piloto_etapas_instrumentadas.txt`](./piloto_etapas_instrumentadas.txt) | ✅ |

---

## 1. Documento de Planificación Acordado

El documento principal se encuentra en:
👉 [`capacity-planning/escenario2.md`](../../../../capacity-planning/escenario2.md)

Cubre en profundidad:
1. **Perfiles multimedia:** Definición técnica de los tres perfiles de G1 (Corto 2m / 2.6MB, Medio 10m / 13.0MB, Largo 30m / 39.0MB, todos 1280×720) y justificación de por qué la función `SelectRenditions` prohíbe el *upscaling* a 1080p.
2. **Concurrencia fija:** Justificación técnica de `WORKER_CONCURRENCY=2` (1 worker por vCPU en `mooc-worker-server`) para maximizar el uso de CPU sin provocar contención de hilos ni OOM.
3. **Niveles de carga:** Diseño de 5 niveles progresivos para observar el punto de equilibrio, el crecimiento monotónico de la cola y el tiempo de drenaje post-carga.
4. **Cadencia HLS:** Modelado de streaming real con pacing vs descarga "lo más rápido posible", incorporando la regla de acotamiento de ventanas de prueba para prevenir costos imprevistos de egreso a internet (Nota técnica 13).
5. **Decisión TTFF:** Separación formal entre métricas de transporte HTTP y métricas de decodificación y renderizado real de cuadros con reproductor.
6. **Separación de planos:** Demostración de que las subidas y consumos directos al bucket liberan completamente la interfaz de red y CPU de la VM Web.
7. **Criterios de parada (Kill Switch):** Reglas operativas para interrumpir pruebas ante degradación severa (tasa de error API > 5%, fallos GCS > 1%, cola > 15 min).

---

## 2. Resultados del Piloto Corto con Instrumentación Desacoplada

El piloto se ejecutó mediante [`scripts/run_pilot_escenario2.sh`](../../../../scripts/run_pilot_escenario2.sh) empleando un video sintético real (8s, 1280×720, 25fps) transcodificado por el worker en el pipeline HLS.

El log completo y sanitizado se encuentra en [`piloto_etapas_instrumentadas.txt`](./piloto_etapas_instrumentadas.txt).

### Desglose de Tiempos por Etapa

| Etapa | Operación | Plano Arquitectural | Duración Medida | Resultado Observado |
| :---: | :--- | :--- | :---: | :--- |
| **Etapa 1** | Autorización y emisión de URL prefirmada | Control (API REST / Nginx) | **1 ms** | HTTP 200 OK. Clave generada bajo `originals/<stable_id>/<uuid>.mp4` con método PUT y Content-Type `video/mp4`. |
| **Etapa 2** | Transferencia directa al almacenamiento | Datos (Object Storage) | **5 ms** | HTTP 200 OK. 0.17 MB transferidos a 32.69 MB/s directamente al almacenamiento sin pasar por la API. |
| **Etapa 3** | Confirmación de carga y encolado | Control (API REST) | **9 ms** | HTTP 202 Accepted. `StatObject` exitoso en almacenamiento; tarea `MediaProcessTask` encolada en Redis Asynq con `processing_status="pending"`. |
| **Etapa 4** | Espera en cola (Queue Latency) | Mensajería (Redis / Asynq) | **513 ms** | Latencia de residencia en cola antes de que el worker libre tome la tarea y transicione a `processing`. |
| **Etapa 5** | Procesamiento y transcodificación (FFmpeg) | Cómputo (Worker Server) | **2.525 s** | Ejecución de FFmpeg, generación de escalera (`360p` y `720p`), segmentación HLS y subida de derivados a `hls/`. |
| **Etapa 6** | Tiempo total hasta Available (`completed`) | Ciclo Asíncrono Completo | **3.038 s** | Tiempo transcurrido desde el 202 Accepted hasta que la API reporta `processing_status="completed"` y el manifiesto `master.m3u8` es públicamente legible. |

### Validación de Consumo Multimedia y Streaming HLS

1. **Cadencia Real (Streaming Simulation):**
   * Latencia de obtención de `master.m3u8`: **1 ms**.
   * Latencia de variante `360p.m3u8`: **1 ms**.
   * Segmentos verificados con pacing de 6.0s: Latencia promedio de **1 ms** por segmento.
   * **Margen de buffer:** **6.00 s** de anticipación ($6.0\text{s} - 0.001\text{s}$). Cero interrupciones de reproducción (*stalls = 0*).
2. **Variante "Lo Más Rápido Posible" (Greedy / Bulk Download):**
   * Descarga inmediata sin pausas de los segmentos: 861 KB descargados en **4 ms** (**199.77 MB/s**).
   * Confirmación: Se cataloga e identifica por separado como prueba de saturación de ancho de banda o scraping masivo, sin correlación con la experiencia de streaming real.
3. **Decisión TTFF e Interrupciones:**
   * La sonda basada en el ciclo de vida de eventos del reproductor (`loadstart` → `canplay` → `playing`) registró un **TTFF de 48 ms** (incluyendo resolución de red y decodificación inicial del cuadro I-frame en buffer).
   * Interrupciones durante la reproducción: **0**.

---

## 3. Guía de Reproducción Rápida

Para reproducir el piloto corto con la instrumentación por etapas:

```bash
# 1. Asegurar servicios base activos
docker compose up -d

# 2. Ejecutar el piloto corto instrumentado (genera video de prueba, compila y mide las 6 etapas)
bash ./scripts/run_pilot_escenario2.sh

# 3. Opcional: Ejecutar contra un despliegue en la nube
bash ./scripts/run_pilot_escenario2.sh https://34.24.52.111.sslip.io
```

El script imprimirá el desglose cronometrado de cada etapa y actualizará automáticamente [`piloto_etapas_instrumentadas.txt`](./piloto_etapas_instrumentadas.txt).

---

## Nunca

Este directorio y todas sus evidencias cumplen estrictamente con la política de seguridad:
* Cero contraseñas, tokens de sesión o llaves privadas registradas.
* Las URLs prefirmadas en los logs han sido sanitizadas automáticamente reemplazando cualquier parámetro de firma (`X-Goog-Signature` / `X-Amz-Signature`) por `REDACTED`.
