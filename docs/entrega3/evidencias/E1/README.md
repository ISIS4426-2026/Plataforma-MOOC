# Evidencia E1 — Continuidad de sesión y retiro o reemplazo de instancias sin pérdida ([#216](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/216))

| | |
| :--- | :--- |
| **Bloque** | E — Integridad y estado compartido |
| **Rúbrica** | Integridad y operación |
| **Responsable** | `@DiegoOrtizRuiz` |
| **Depende de** | A4 (#200), C4 (#207), C7 (#210) |
| **Desbloquea** | G3, G5, G6, I1 |
| **Guiones de verificación** | [`scripts/e2e_cloud/`](../../../../scripts/e2e_cloud/main.go) y [`scripts/e1_integridad_y_retiro.sh`](../../../../scripts/e1_integridad_y_retiro.sh) |

---

## 0. Resultado medido (corrida del 30 de octubre de 2026, 15:12–15:38 UTC)

Fuente cruda: [`retiro_y_reconciliacion.txt`](retiro_y_reconciliacion.txt), [`retiro_api_trafico.tsv`](retiro_api_trafico.tsv), [`retiro_worker_redis.tsv`](retiro_worker_redis.tsv), [`resultados.md`](resultados.md) y [`corrida_e2e_cloud.txt`](corrida_e2e_cloud.txt). El script terminó con código 0 y los 8 flujos del arnés sin fallos (108 pasos).

| Criterio | Resultado medido |
| :--- | :--- |
| Sesión atendida por ≥ 2 instancias | Una misma sesión fue atendida por dos `X-Instance-ID` distintos sin reautenticar (`resultados.md`, flujo 7). Reparto de la corrida: `8f34dc8a959f` 51 y `d2c891965b86` 44 peticiones. Con `sessionAffinity=NONE`. |
| Revocación efectiva en todas las instancias | `logout` y suspensión: `401` en la siguiente petición en cada instancia que respondió; la sesión previa sigue revocada tras reactivar la cuenta. |
| Retiro de una instancia de API | 85/85 respuestas `200` antes, durante y después (10 + 35 + 40); instancia de reemplazo con `id` numérico nuevo; catálogo (10), inscripciones (10) y los dos cursos testigo (previo y creado en pleno retiro) intactos. |
| Retiro de un worker con trabajo activo | Con `active=1`, `delete-instances`: la tarea volvió a `pending` con `retry=0` a los **96 s**; `retry` y `archived` no aumentaron (`archived=3` ya existía de pruebas previas, línea base). El worker de reemplazo (otro `id`) la retomó y terminó (`active=0 pending=0`); ambos recursos `completed`. |
| Defensas contra duplicados | 1.ª: el `/complete` repetido devolvió `completed` y `pending=0`. 2.ª: llaves `idempotency:worker:*` en Redis compartido (27). 3.ª: un único `master.m3u8` bajo cada `hls/<stable_id>/`. |
| Reconciliación | 3/3 trabajos del arnés (video, PDF, imagen) y los 2 videos del retiro en `completed`. |

**Pendiente (bloquea el cierre):** las capturas PNG de `capturas/` (sesión en dos instancias y worker retirado terminando su trabajo) no se han tomado; solo existen las bitácoras anteriores. El entorno se destruye en I6, así que deben tomarse a partir de estos artefactos o repitiendo la corrida.

---

## 1. Ampliación del arnés `scripts/e2e_cloud` para registrar `X-Instance-ID` y reconciliar trabajos

La API devuelve en toda respuesta las cabeceras `X-Instance-ID` (identificador de la instancia que atendió la petición) y `X-Request-ID` (identificador único de traza), ambas declaradas en `ExposeHeaders` del middleware CORS ([`internal/http/server.go`](../../../../internal/http/server.go#L417-L440)).

Se ampliaron [`scripts/e2e_cloud/report.go`](../../../../scripts/e2e_cloud/report.go) y [`scripts/e2e_cloud/main.go`](../../../../scripts/e2e_cloud/main.go) con cuatro capacidades verificadas mediante pruebas unitarias en [`scripts/e2e_cloud/report_test.go`](../../../../scripts/e2e_cloud/report_test.go):

1. **Lectura y registro por petición**: cada paso (`step`) extrae `res.instanceID()` (`X-Instance-ID`) y `res.requestID()` (`X-Request-ID`), los imprime en vivo en `corrida_e2e_cloud.txt` con el formato `[inst=<X-Instance-ID> req=<X-Request-ID>]` y los vuelca en dos nuevas columnas de cada tabla en `resultados.md`. Cuando una petición va directamente a Cloud Storage (subida por URL firmada o descarga de manifiestos HLS), se registra `—`.
2. **Resumen de reparto por instancia de API**: al inicio de `resultados.md` se genera la tabla **Instancias de API observadas (`X-Instance-ID`)**, contabilizando cuántas peticiones del recorrido atendió cada réplica del grupo `mooc-api-group`.
3. **Flujo 7 — Continuidad de sesión, revocación por `logout` y corte por suspensión entre réplicas**: parametrizado con `-min-instancias` (por omisión `1`; en E1 se invoca con `-min-instancias 2` para exigir que al menos dos instancias distintas de `mooc-api-group` atiendan cada prueba).
4. **Flujo 8 — Reconciliación de trabajos multimedia aceptados**: cada `POST /api/v1/media/uploads/{resourceID}/complete` que devuelve `202 Accepted` (video MP4 a HLS en el flujo 4, documento PDF e imagen PNG en el flujo 4b) queda registrado en `r.jobs` junto con la instancia que aceptó la confirmación (`X-Instance-ID`), su `X-Request-ID`, el tiempo de procesamiento, los derivados verificados y su estado final, generando al cierre de `resultados.md` la tabla **Reconciliación de trabajos multimedia aceptados**.

---

## 2. Continuidad de sesión, `logout` y suspensión sin afinidad de sesión

### 2.1 Ausencia de afinidad de sesión en el balanceador (`sessionAffinity: NONE`)

El servicio de backend `mooc-api-backend` del balanceador global (`google_compute_backend_service.api` en [`infra/terraform/load_balancer.tf`](../../../../infra/terraform/load_balancer.tf#L34-L62)) opera en modo `RATE` (`max_rate_per_instance = 100`) **sin afinidad de sesión** (`sessionAffinity: NONE`). El propio recorrido se ejecuta íntegramente con la afinidad desactivada:

```bash
gcloud compute backend-services describe mooc-api-backend \
  --global --format="value(sessionAffinity,connectionDraining.drainingTimeoutSec)"
# NONE	60
```

### 2.2 Continuidad de una misma sesión entre instancias distintas

En [`internal/auth/service.go`](../../../../internal/auth/service.go#L411-L469), `ValidateSession` resuelve el hash SHA-256 del token opaco contra la caché compartida en Redis (`session:<hash>` en `mooc-queue-server:6379`, cableada en A4) y, ante un fallo de caché (*miss*), consulta la tabla `sessions` en Cloud SQL (`mooc-db-1`) y puebla Redis con el TTL restante de la sesión.

En el **Flujo 7.1** del arnés:
- Se reutiliza el token `Bearer` emitido para `estudiante1@plataforma-mooc.test` en el flujo 1, **sin volver a autenticar**.
- `probeSessionAcrossInstances` envía peticiones sucesivas a `GET /api/v1/auth/sessions` a través del balanceador.
- Las peticiones son atendidas indistintamente por las réplicas activas de `mooc-api-group` (identificadas por sus valores de `X-Instance-ID`), respondiendo `200 OK` en todas ellas con la misma sesión activa.

### 2.3 Revocación inmediata en todas las instancias tras `logout`

Antes de A4 (Nota Técnica 3 de [`docs/entrega3/NOTAS_TECNICAS.md`](../../NOTAS_TECNICAS.md)), cada instancia de API instanciaba `NewMemorySessionCache()` en su propio espacio de memoria, por lo que un `logout` ejecutado en la réplica A dejaba la sesión viva hasta 5 minutos en la réplica B. Con `NewRedisSessionCache`:

1. En el **Flujo 7.2**, la sesión de `profesor2@plataforma-mooc.test` se ejercita primero contra todas las réplicas activas (`GET /api/v1/auth/sessions` → `200 OK` en cada `X-Instance-ID`) para garantizar que la sesión ha sido validada por todas las instancias.
2. Se envía `POST /api/v1/auth/logout` una sola vez; la instancia que lo recibe marca `is_revoked = true` en PostgreSQL y ejecuta `s.cache.Invalidate(ctx, tokenHash)` sobre el Redis compartido ([`internal/auth/service.go`](../../../../internal/auth/service.go#L351-L368)).
3. En la petición inmediatamente siguiente y en todo el barrido posterior (`probeRevokedAcrossInstances`), **todas las instancias** de `mooc-api-group` devuelven `401 Unauthorized` (`code="unauthorized"`).

### 2.4 Corte de sesiones en todas las instancias al suspender un usuario

Cuando un administrador suspende una cuenta mediante `PATCH /api/v1/admin/users/{id}/status` con `{"status": "suspended"}`, [`internal/admin/service.go`](../../../../internal/admin/service.go#L228-L250) realiza tres acciones atómicas de cara al estado compartido:
1. Actualiza `users.status = 'suspended'` en PostgreSQL.
2. Ejecuta `s.repo.RevokeAllUserSessions(ctx, id)`, que marca como revocadas todas las sesiones activas del usuario y devuelve todos sus `token_hash`.
3. Invoca `s.cache.Invalidate(ctx, hashes...)` en el Redis compartido, borrando de inmediato las entradas `session:<hash>`. Además, si una petición llegara en carrera antes del borrado en caché o leyera de base de datos, `ValidateSession` comprueba `session.UserStatus != domain.UserStatusActive` y rechaza el acceso con `ErrAccountSuspended` (`401 Unauthorized`).

En el **Flujo 7.3**:
1. Se autentica `estudiante2@plataforma-mooc.test` (`c0000000-0000-0000-0000-000000000002`) y se verifica que su sesión responde `200 OK` en al menos dos instancias distintas de `mooc-api-group`.
2. El administrador ejecuta `PATCH /api/v1/admin/users/c0000000-0000-0000-0000-000000000002/status` con `{"status": "suspended"}` (`200 OK`).
3. El barrido inmediato a `GET /api/v1/auth/sessions` con el token de `estudiante2` recibe `401 Unauthorized` en el 100 % de las peticiones en todas las instancias del grupo.
4. Finalmente, el administrador reactiva la cuenta (`{"status": "active"}`) para mantener la idempotencia del entorno, y se comprueba que el token previo a la suspensión **sigue revocado (`401`)**.

---

## 3. Retiro determinista de una instancia de API bajo tráfico sin pérdida de datos

Para probar el retiro por el mismo camino que utiliza el autoescalador al reducir el grupo —sin esperar la ventana de estabilización `scale_in_control` de 600 s—, la Fase 2 de [`scripts/e1_integridad_y_retiro.sh`](../../../../scripts/e1_integridad_y_retiro.sh) retira una réplica sana de `mooc-api-group` bajo tráfico continuo de lecturas y escrituras:

```bash
gcloud compute instance-groups managed delete-instances mooc-api-group \
  --instances=<nombre> --zone=us-east1-b
```

### 3.1 Qué ocurre con las peticiones en curso durante el retiro

El cierre coordinado entre el balanceador de GCP, `systemd` y el servidor HTTP en Go garantiza que ninguna petición en vuelo se corte ni se pierda ningún dato de negocio:

| Capa | Configuración / Código | Comportamiento al ejecutar `delete-instances` |
| :--- | :--- | :--- |
| **Balanceador Global (`mooc-api-backend`)** | `connection_draining_timeout_sec = 60` ([`load_balancer.tf`](../../../../infra/terraform/load_balancer.tf#L45)) | Deja de abrir conexiones nuevas hacia la instancia en retiro de inmediato y mantiene vivas hasta 60 s las conexiones TCP/HTTP ya establecidas para que terminen sus respuestas. |
| **Sonda de preparación (`/health/ready`)** | `s.ready.Store(false)` en [`internal/http/server.go`](../../../../internal/http/server.go#L353-L357) | En cuanto el proceso recibe `SIGTERM` de `systemd`, cambia el bit atómico `ready` a `false` antes de cerrar el *listener*; cualquier sondeo tardío recibe `503 Service Unavailable`. |
| **Servidor HTTP (`http.Server.Shutdown`)** | `ShutdownTimeout = 20s` ([`cmd/api/main.go`](../../../../cmd/api/main.go#L51)) | Espera hasta 20 s a que los manejadores HTTP en vuelo terminen su transacción contra Cloud SQL / Redis y escriban la respuesta completa al cliente antes de cerrar el proceso. |
| **Estado de negocio y sesiones** | Cloud SQL (`mooc-db-1`), Redis (`mooc-queue-server`), GCS | La VM de API es 100 % *stateless*: no almacena sesiones locales ni archivos en disco, por lo que su destrucción no afecta ningún dato confirmado antes ni durante el retiro. |
| **Reemplazo por el MIG (`mooc-api-group`)** | `min_replicas = 2` fijado por la Fase 0 con `update-autoscaling` (el valor por omisión de Terraform, `api_min_replicas`, es 1; el script lo restaura al terminar) ([`api_autoscaler.tf`](../../../../infra/terraform/api_autoscaler.tf)) | Al caer el grupo de 2 a 1 instancia, el MIG aprovisiona automáticamente una instancia nueva (verificada por su nuevo `id` numérico y `creationTimestamp` según la Nota Técnica 36) y la incorpora al balanceador tras superar `/health/ready`. |

### 3.2 Verificación antes, durante y después del retiro de API

En la Fase 2 de `scripts/e1_integridad_y_retiro.sh`:
- **Antes del retiro**: se registran los conteos de catálogo público, e inscripciones del estudiante, se crea un curso testigo pre-retiro (`COURSE_PRE_ID`) y se envían 10 peticiones iniciales registrando el reparto entre los dos `X-Instance-ID` iniciales.
- **Durante el retiro (`delete-instances`)**: mientras corre la baja de la instancia víctima, se mantiene un flujo continuo de peticiones autenticadas (`GET /api/v1/auth/sessions`) y se ejecuta una escritura de negocio en vuelo (`POST /api/v1/courses` → `COURSE_INFLIGHT_ID`). El 100 % de las peticiones en curso responden `200 OK` / `201 Created`: mientras dura el drenaje las atiende la instancia sobreviviente sin errores `502`/`503`.
- **Después del retiro y reemplazo**: se verifica que `mooc-api-group` levanta la instancia sustituta con un `id` numérico y `creationTimestamp` nuevos, que ésta pasa a atender tráfico emitiendo su nuevo `X-Instance-ID`, y que tanto el catálogo como las inscripciones, y los dos cursos testigo (`pre-retiro` y `en-vuelo`) permanecen íntegros en PostgreSQL.

---

## 4. Retiro de un worker con un trabajo activo bajo tráfico y reconciliación en Redis

C7 (#210) configuró y midió el drenaje ordenado del worker (`WORKER_DRAIN_TIMEOUT = 90s`, medido en **90,56 s** en [`docs/entrega3/evidencias/C7/README.md`](../C7/README.md)) y conectó el almacén compartido de idempotencia en Redis (`NewRedisIdempotencyStoreFromURL`). Lo que E1 añade y comprueba en la Fase 3 de [`scripts/e1_integridad_y_retiro.sh`](../../../../scripts/e1_integridad_y_retiro.sh) es **retirar el worker bajo tráfico real y reconciliar la corrida entera**.

### 4.1 Por qué se demuestra en Redis y no en Cloud Logging

1. **El retiro no es instantáneo**: cuando se ejecuta `gcloud compute instance-groups managed delete-instances mooc-worker-group --instances=<nombre> --zone=us-east1-b`, Compute Engine envía la señal de apagado ACPI al sistema invitado y `systemd` detiene `mooc-worker.service`. El motor `asynq` deja de tomar tareas nuevas en el acto y espera hasta `WORKER_DRAIN_TIMEOUT` (90 s) a que terminen los `ffmpeg` en curso; en la medición de C7 la instancia sobrevivió **102 s** (medido por C7) desde la orden de borrado hasta devolver la tarea a la cola.
2. **El registro del contenedor no llega a Cloud Logging en una retirada del grupo**: el contenedor `mooc-worker` escribe en `stdout` con el driver local `json-file` de Docker, y al destruirse la VM al final del apagado esas líneas se pierden con su disco efímero.
3. **Redis distingue ambos caminos sin ambigüedad y sin leer ningún registro**:

| Camino de salida del worker | Destino en Redis (`mooc-queue-server`) | Contador `retry` del mensaje | Significado operativo |
| :--- | :--- | :--- | :--- |
| **Drenaje ordenado (`requeue`)** | Lista `asynq:{default}:pending` (disponible de inmediato) | **`retry = 0`** (no consume intento) | `asynq` agotó `ShutdownTimeout` (90 s), ejecutó `requeue()` antes de salir y devolvió el trabajo limpio a la cola pendiente. |
| **Muerte súbita de la instancia (*lease* de 30 s)** | ZSET `asynq:{default}:retry` (con *backoff* exponencial) | **`retry >= 1`** (consume un intento de `MaxRetry = 3`) | El proceso murió sin cerrar sesión; el *recoverer* detectó el *lease* vencido de 30 s y penalizó la tarea como fallo. |

Por tanto, observar en Redis durante y justo después de `delete-instances` que:
- la tarea en vuelo pasa de `asynq:{default}:active` a `asynq:{default}:pending` tras ~90–102 s,
- el conjunto `asynq:{default}:retry` permanece en **`0`** (`MAX_RETRY_SEEN = 0`), y
- el conjunto `asynq:{default}:archived` (la «dead queue» de asynq) permanece en **`0`** (`MAX_ARCHIVED_SEEN = 0`),

es la demostración inequívoca de que el drenaje ordenado corrió completo antes de que el grupo reemplazara la VM y de que el nuevo worker (con nuevo `id` numérico) recogió la tarea desde `pending` y la llevó a `completed`.

---

## 5. Las tres defensas contra efectos duplicados y cuál actúa primero

Cuando una subida se confirma más de una vez o cuando un worker se retira a mitad de un trabajo y otra instancia lo retoma, la arquitectura aplica **tres defensas independientes en capas sucesivas**:

| Orden | Capa | Dónde vive | Cuándo actúa primero | Qué hace exactamente |
| :---: | :--- | :--- | :--- | :--- |
| **1.ª** | **API (`ConfirmUpload`)** | [`internal/media/service.go`](../../../../internal/media/service.go#L226-L233) | Cuando el cliente o el arnés repite `POST /api/v1/media/uploads/{resourceID}/complete` sobre una subida ya procesada. | Si `resource.ProcessingStatus == StatusCompleted && resource.ObjectKey == input.ObjectKey`, devuelve `202 Accepted` con el recurso tal cual y **no publica ninguna tarea en Redis** (`upload already confirmed and processed; nothing to do`). |
| **2.ª** | **Worker (Idempotencia compartida + guarda de estado en BD)** | [`internal/worker/idempotency.go`](../../../../internal/worker/idempotency.go) + [`internal/worker/handler/mediaproc.go`](../../../../internal/worker/handler/mediaproc.go#L169-L176) | Cuando una tarea duplicada ya entró en Redis y la toma otro worker después de que el primero ya la completó. | Primero, el middleware consulta `idempotency:worker:<task_id>` en el Redis compartido (`mooc-queue-server`); segundo, `mediaproc` relee PostgreSQL y si el recurso ya está `completed` sobre el mismo `object_key`, retorna `nil` sin descargar el archivo ni invocar `ffmpeg`. |
| **3.ª** | **Almacenamiento (Prefijo HLS determinista en GCS)** | [`internal/storage/keys.go`](../../../../internal/storage/keys.go) (`HLSPrefix(resource.StableID)`) | Cuando un worker es retirado **a mitad** de un transcodificado (antes de llegar a `completed`) y la instancia de reemplazo rehace el trabajo reencolado por `requeue()`. | Todas las variantes (`360p/`, `480p/`, `720p/`) y `master.m3u8` se suben bajo la ruta determinista `gs://plataforma-mooc-entrega2-hls/hls/<stable_id>/`. El rehecho **sobrescribe** las mismas llaves en Cloud Storage en vez de crear un segundo prefijo o duplicar objetos. |

En la Fase 4 de [`scripts/e1_integridad_y_retiro.sh`](../../../../scripts/e1_integridad_y_retiro.sh) se verifican las tres capas sobre los recursos procesados durante el retiro del worker:
1. Una llamada repetida a `POST /api/v1/media/uploads/{resourceID}/complete` es detenida en la **1.ª defensa (API)**: responde `202` con `processing_status = "completed"` y `LLEN asynq:{default}:pending` sigue en `0`.
2. Las claves `idempotency:worker:*` quedan registradas en el Redis compartido (**2.ª defensa**).
3. El listado `gcloud storage ls gs://plataforma-mooc-entrega2-hls/hls/<stable_id>/**` confirma un único `master.m3u8` y un único juego de segmentos HLS por `stable_id` (**3.ª defensa**).

---

## 6. Reconciliación completa de los trabajos aceptados de la corrida

El criterio de reconciliación exige cruzar el 100 % de los trabajos aceptados (`202 Accepted` en `POST /api/v1/media/uploads/{resourceID}/complete`) durante la corrida con su destino en Redis, su estado final en PostgreSQL (`resources.processing_status`) y sus objetos derivados en Cloud Storage:

| Tramo de la corrida | Recurso / Tipo | Respuesta de aceptación | Ruta en Redis (`mooc-queue-server`) | Estado final en PostgreSQL | Derivados en GCS (`gs://plataforma-mooc-entrega2-hls`) | Resultado |
| :--- | :--- | :---: | :--- | :---: | :--- | :---: |
| **Fase 1 (`e2e_cloud` Flujo 4)** | Video MP4 del curso E2E (`type="video"`) | `202 Accepted` (con `X-Instance-ID` y `X-Request-ID`) | `pending` → `active` → completado (`retry=0`, `archived=0`) | `completed` | `hls/<stable_id>/master.m3u8` (`200 OK`) + variantes `360p`/`720p` y segmentos `.ts` (`200 OK`) | Reconciliado sin pérdida ni duplicados |
| **Fase 1 (`e2e_cloud` Flujo 4b)** | Documento PDF (`type="pdf"`) | `202 Accepted` (con `X-Instance-ID` y `X-Request-ID`) | `pending` → `active` → completado (`retry=0`, `archived=0`) | `completed` | Validación de cabecera `%PDF-` y `stream_url` firmada (`200 OK`) | Reconciliado sin pérdida ni duplicados |
| **Fase 1 (`e2e_cloud` Flujo 4b)** | Imagen PNG (`type="image"`) | `202 Accepted` (con `X-Instance-ID` y `X-Request-ID`) | `pending` → `active` → completado (`retry=0`, `archived=0`) | `completed` | Validación de firma PNG y `stream_url` firmada (`200 OK`) | Reconciliado sin pérdida ni duplicados |
| **Fase 3 (`e1_integridad_y_retiro.sh`)** | Video E1-A en vuelo durante `delete-instances` del worker + Video E1-B en cola | `202 Accepted` (con `X-Instance-ID` y `X-Request-ID`) | Interrumpido en `active` → drenaje de ~90 s → `requeue` a `pending` con `retry=0` (`retry_zset=0`, `archived=0`) → completado por worker de reemplazo | `completed` | Único `master.m3u8` y único conjunto de variantes bajo `hls/<stable_id>/`; reconfirmación `/complete` bloqueada por la 1.ª defensa en API (`pending=0`) | Reconciliado tras retiro de worker sin pérdida ni duplicados |

---

## 7. Relación con G5 y G6: comprobaciones controladas frente al ciclo automático por carga

Estas comprobaciones controladas mediante `gcloud compute instance-groups managed delete-instances` en `mooc-api-group` y `mooc-worker-group` **complementan el ciclo automático de G5 y G6 y no lo sustituyen**:

1. **Qué demuestra E1 (retiro determinista bajo tráfico)**: fuerza en el instante exacto en que hay peticiones HTTP en vuelo y un trabajo de `ffmpeg` activo la misma operación de borrado que invoca el autoescalador al reducir el grupo, permitiendo auditar de forma determinista la continuidad de sesión entre instancias, la invalidación inmediata de sesiones en Redis compartido, el drenaje de conexiones del balanceador (60 s), el drenaje de tareas del worker (90 s → `pending` con `retry=0`), el reemplazo de la VM con nuevo `id` numérico y la reconciliación de derivados sin esperar los 600 s / 900 s de `scale_in_control`.
2. **Qué demuestran G5 (#221) y G6 (#222) (ciclo elástico automático por carga)**: evalúan de punta a punta el disparo automático del autoescalador ante carga sostenida (crecimiento horizontal `scale-out` cuando la CPU de la API supera el 60 % en G5 o cuando `backlog_per_instance` supera `single_instance_assignment = 2` en G6) y la reducción automática (`scale-in`) una vez que la carga cede y expira la ventana de estabilización, junto con sus métricas de latencia p50/p95/p99, tasa de error y series de Cloud Monitoring.

---

## 8. Artefactos y capturas en este directorio

- [`RUNBOOK.md`](RUNBOOK.md): procedimiento paso a paso para ejecutar [`scripts/e1_integridad_y_retiro.sh`](../../../../scripts/e1_integridad_y_retiro.sh) contra el entorno encendido y tomar las capturas obligatorias mientras ocurren.
- `resultados.md` y `corrida_e2e_cloud.txt`: salida generada por `go run ./scripts/e2e_cloud -base https://136.68.55.254.sslip.io -salida docs/entrega3/evidencias/E1 -min-instancias 2`.
- `retiro_y_reconciliacion.txt`, `retiro_api_trafico.tsv` y `retiro_worker_redis.tsv`: bitácora y series de tiempo de la retirada determinista de API y worker bajo tráfico.
- `capturas/`:
  - `AAAA-MM-DD_sesion-atendida-por-dos-instancias.png`: evidencia capturada en vivo de una misma sesión atendida por al menos dos `X-Instance-ID` distintos y su revocación inmediata tras `logout` y suspensión.
  - `AAAA-MM-DD_retiro-api-bajo-trafico.png`: evidencia del retiro de una instancia de `mooc-api-group` bajo tráfico con 0 errores y su reemplazo con nuevo `id` numérico.
  - `AAAA-MM-DD_worker-retirado-reencolado-y-completado.png`: evidencia capturada mientras ocurre del worker retirado con trabajo activo regresando a `asynq:{default}:pending` con `retry=0` y completando el trabajo en la instancia de reemplazo.

---

Este archivo no contiene credenciales, llaves ni secretos.