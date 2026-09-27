# Evidencia E1 — Worker Server, Cola en Red Privada y Procesamiento Asíncrono (Issue #125)

Este directorio documenta el aprovisionamiento, despliegue y validación del **Worker Server** y su intermediario de cola (**Redis/Asynq**) en una máquina virtual dedicada dentro de la red privada, cumpliendo con la rúbrica de **Despliegue e integración (50%)** de la Entrega 2.

Todas las evidencias corresponden a ejecuciones reales sobre la infraestructura en Google Cloud Platform (`mooc-worker-server` y `mooc-web-server`) y han sido rigurosamente sanitizadas para excluir cualquier tipo de contraseña, token de sesión, llave privada o secreto.

---

## Criterios de Aceptación y Archivos de Evidencia

| Archivo | Criterio del Enunciado | Estado |
| :--- | :--- | :---: |
| [`terraform_plan.txt`](./terraform_plan.txt) | **VM con perfil B1 declarada en Terraform**: Instancia `mooc-worker-server` (`e2-highcpu-2`, 30 GiB `pd-balanced`, zona `us-east1-b`) con cuenta de servicio diferenciada `sa-worker-server` y etiquetas de firewall `worker-server` y `allow-iap-ssh`. | ✅ Cumplido |
| [`aislamiento_red_privada.txt`](./aislamiento_red_privada.txt) | **La cola es inalcanzable desde fuera de la red privada**: El puerto 6379 (Redis) es accesible exclusivamente desde la subred interna (`10.0.1.0/24`) por el Web Server (`10.0.1.2`), y totalmente inaccesible (paquetes descartados por el firewall) desde el Internet público (`0.0.0.0/0`). | ✅ Cumplido |
| [`concurrencia_y_disco_temporal.txt`](./concurrencia_y_disco_temporal.txt) | **Fijar y registrar la concurrencia de workers y el uso de disco temporal**: Concurrencia fijada en 2 (`WORKER_CONCURRENCY=2`, 1 por vCPU del perfil B1 para prevenir OOM en 2 GiB de RAM) y almacenamiento temporal en `/tmp/mooc-worker` acotado a 2.048 MB con limpieza inmediata automática tras transcodificación. | ✅ Cumplido |
| [`procesamiento_video_e2e.txt`](./procesamiento_video_e2e.txt) | **Un archivo cargado llega a available con derivados en el bucket**: Carga directa vía URL prefirmada V4 a Cloud Storage, encolado asíncrono, transcodificación HLS en el worker (manifiesto `master.m3u8` y variantes), derivados en `hls/` y actualización transaccional a estado `completed` (`available`). | ✅ Cumplido |
| [`idempotencia_entrega_duplicada.txt`](./idempotencia_entrega_duplicada.txt) | **Entrega duplicada no genera salidas repetidas**: Reenvío de la misma tarea interceptado por el middleware de idempotencia y el estado del procesador; cero re-escrituras en el bucket y cero mutaciones adicionales en la base relacional. | ✅ Cumplido |
| [`fallo_reintento_backoff_dlq.txt`](./fallo_reintento_backoff_dlq.txt) | **Un fallo inyectado reintenta con backoff y termina en cola de fallidos diagnosticable**: 3 reintentos automáticos con retraso exponencial (2s, 4s, 8s), emisión de alerta estructurada `DLQ_JOB_FAILED` y depósito en cola `asynq:archived` (DLQ) con información diagnóstica completa. | ✅ Cumplido |

---

## 1. Topología del Procesamiento Asíncrono en la Nube

```
                   INTERNET (0.0.0.0/0)
                            │
               HTTP / HTTPS │ :80, :443 (Regla mooc-allow-web-ingress)
                            ▼
    ┌──────────────────────────────────────────────────┐
    │ VM: mooc-web-server (34.24.52.111 / 10.0.1.2)    │
    │  - proxy (Nginx 1.27)                            │
    │  - api (Go API modular)                          │
    └───────────────────────┬──────────────────────────┘
                            │
                            │ Red Privada VPC (mooc-subnet: 10.0.1.0/24)
                            │ Tráfico interno TCP :6379 permitido por mooc-allow-internal
                            │ (Inalcanzable desde Internet: bloqueado por firewall)
                            ▼
    ┌──────────────────────────────────────────────────┐
    │ VM: mooc-worker-server (34.148.188.42 / 10.0.1.3)│
    │                                                  │
    │  ┌──────────────────────┐  ┌──────────────────┐  │
    │  │  redis (cola asynq)  │  │  worker (Go)     │  │
    │  │  redis:7-alpine      │  │  FFmpeg          │  │
    │  │  Puerto host :6379   │  │  Concurrencia: 2 │  │
    │  └──────────────────────┘  └────────┬─────────┘  │
    └─────────────────────────────────────┼────────────┘
                                          │
                   ┌──────────────────────┴──────────────────────┐
                   │ Private Peering                             │ Google Private API
                   │ PostgreSQL 10.171.240.3                     │ HTTPS
                   ▼                                             ▼
        ┌──────────────────────┐                     ┌──────────────────────┐
        │ Cloud SQL (mooc-db-1)│                     │ Cloud Storage Bucket │
        │ Estado: completed    │                     │ originals/ -> hls/   │
        └──────────────────────┘                     └──────────────────────┘
```

1. **Desacoplamiento de Cargas:** El Web Server queda completamente liberado de la transcodificación de video (FFmpeg) y de alojar la cola de mensajes en memoria. La API modular encola las tareas directamente en la instancia privada de Redis del Worker Server a través de la VPC (`10.0.1.3:6379`).
2. **Aislamiento Perimetral (B3):**
   * La regla de firewall `mooc-allow-web-ingress` aplica **únicamente** a instancias con etiqueta `web-server`. Al carecer de esta etiqueta, `mooc-worker-server` no expone ningún puerto al Internet público.
   * La regla `mooc-allow-internal` habilita la comunicación interna entre las instancias de la subred `10.0.1.0/24`.
   * El puerto 6379 es inaccesible desde el exterior (los intentos de conexión desde Internet terminan en *timeout*).
3. **Mínimo Privilegio (IAM):**
   * La VM `mooc-worker-server` tiene adjunta la cuenta `sa-worker-server@plataforma-mooc-entrega2.iam.gserviceaccount.com`.
   * Permisos concedidos: Lectura en `originals/` para descargar videos fuente; escritura acotada por condición CEL exclusivamente a `hls/*` (`roles/storage.objectUser`); lectura del secreto `db-password` en Secret Manager; y conectividad como cliente a Cloud SQL (`roles/cloudsql.client`).
   * No dispone de permisos para firmar URLs ni para escribir fuera del prefijo de derivados.

---

## 2. Concurrencia y Acotamiento de Recursos

El enunciado exige justificar y registrar la concurrencia y el uso de almacenamiento temporal:

| Parámetro | Valor Efectivo | Justificación Técnica |
| :--- | :--- | :--- |
| **Concurrencia** (`WORKER_CONCURRENCY`) | `2` | El perfil B1 (`e2-highcpu-2`) cuenta con 2 vCPU dedicadas y 2 GiB de RAM. La transcodificación de video HLS con FFmpeg satura 1 vCPU por flujo y demanda entre 400 y 600 MiB de RAM. Fijar la concurrencia en 2 permite saturar la CPU de forma óptima sin generar contención de contexto y manteniendo el consumo de memoria en ~1.2 GiB, lo que previene fallos por falta de memoria (OOM). |
| **Directorio de Trabajo** (`MEDIA_WORK_DIR`) | `/tmp/mooc-worker` | Montado como volumen persistente en el host dentro del disco `pd-balanced` de 30 GiB. Cada proceso aísla su espacio mediante `os.MkdirTemp` y garantiza su liberación mediante `defer os.RemoveAll(workDir)`. En reposo, el directorio ocupa 0 bytes. |
| **Tope Máximo de Video** (`MEDIA_MAX_ORIGINAL_MB`) | `2048` (2 GiB) | Barrera de protección en el worker que descarta cualquier archivo mayor a 2 GiB antes de procesarlo, evitando que un video sobredimensionado llene el disco de 30 GiB. |

---

## 3. Demostración de Idempotencia y Resiliencia (DLQ)

1. **Idempotencia ante Entrega Duplicada:**
   * La clave de idempotencia se deriva determinísticamente del archivo cargado (`media-originals/<resource_id>/<filename>`).
   * Al recibir una entrega duplicada, el middleware de idempotencia (`IdempotencyMiddleware`) y la validación de estado en `MediaProcessor` verifican que el objeto ya fue procesado y su estado en base de datos es `completed`.
   * La tarea duplicada se completa en 4 ms sin re-ejecutar FFmpeg, sin sobreescribir los segmentos en el bucket y sin registrar entradas redundantes en la auditoría.
2. **Reintentos con Backoff Exponencial y Cola de Fallidos (DLQ):**
   * Ante fallos transitorios, Asynq aplica la función `DefaultExponentialBackoff` definida en `internal/worker/backoff.go`:
     $$\text{delay}(n) = 2 \times 2^n \quad (\text{para } n \in \{0, 1, 2\})$$
     * Reintento 1 ($n=0$): 2 segundos.
     * Reintento 2 ($n=1$): 4 segundos.
     * Reintento 3 ($n=2$): 8 segundos.
   * Al alcanzar el límite de 3 reintentos (`retried >= maxRetry`), el manejador de errores de Asynq emite la alerta estructurada `[ALERT] Job moved to Dead-Letter Queue (DLQ)` con código `DLQ_JOB_FAILED`.
   * La tarea es archivada en Redis en el conjunto ordenado `asynq:archived`, conservando su identificador, carga útil original y el mensaje de diagnóstico para su inspección y recuperación.

---

## 4. Guía de Reproducción Rápida

### A. Verificar conectividad interna y rechazo perimetral de la cola
```bash
# 1. Desde el Web Server (debe responder PONG):
gcloud compute ssh mooc-web-server --zone=us-east1-b --project=plataforma-mooc-entrega2 \
  --tunnel-through-iap --command="nc -z -v -w 3 10.0.1.3 6379"

# 2. Desde Internet (debe fallar por timeout):
nc -z -v -w 5 34.148.188.42 6379
```

### B. Verificar estado y contenedores del Worker Server
```bash
gcloud compute ssh mooc-worker-server --zone=us-east1-b --project=plataforma-mooc-entrega2 \
  --tunnel-through-iap --command="cd ~/mooc && docker compose -f docker-compose.worker.yml ps"
```

### C. Inspeccionar logs del worker y registro de concurrencia
```bash
gcloud compute ssh mooc-worker-server --zone=us-east1-b --project=plataforma-mooc-entrega2 \
  --tunnel-through-iap --command="docker logs mooc-worker | head -n 25"
```
