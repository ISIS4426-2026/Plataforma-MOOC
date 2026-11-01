# Evidencia E2 — Consistencia de operaciones académicas y envío duplicado sin doble calificación ([#217](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/217))

| | |
| :--- | :--- |
| **Bloque** | E — Integridad y estado compartido |
| **Rúbrica** | Integridad y operación |
| **Responsable** | `@CrispisCas9` (verificación y cierre hechos por `@tmichelldiaz`) |
| **Depende de** | C4 (#207), D2 (#212) |
| **Desbloquea** | G3, I1 |
| **Guion de verificación** | `go run ./scripts/e2e_cloud -escenario consistencia -min-instancias 2` ([`scripts/e2e_cloud/consistencia.go`](../../../../scripts/e2e_cloud/consistencia.go)) |

---

## 0. Resultado

La prueba corre contra la nube con **dos réplicas de API** y mira, después de cada operación repetida, el estado que quedó (historial, censo, avance y auditoría) leído por réplicas distintas; no se conforma con el `200`.

**Corrida antes de la corrección** — 2026-11-01, 09:12 UTC, [`antes_de_la_correccion/`](./antes_de_la_correccion/corrida_consistencia.txt) · [`resultados.md`](./antes_de_la_correccion/resultados.md): 44 pasos, **2 fallos**, los dos del mismo defecto:

| Qué se hizo | Qué debía quedar | Qué quedó |
| :--- | :--- | :--- |
| 8 inscripciones simultáneas de un mismo estudiante, sin clave | 1 fila y 1 evento `enrollment.created` | 1 fila, pero **8** eventos en la auditoría (y otros 8, sin cambio, tras reintentar) |

Todo lo demás pasó. Está en la sección 1.

**Corrida después de la corrección:** se añade en cuanto la corrección esté desplegada (ver 2).

---

## 1. Qué se probó y qué dio

Las 7 áreas, con la réplica que atendió cada petición en el log (`inst=`):

| Área | Prueba | Resultado (antes de la corrección) |
| :--- | :--- | :--- |
| **Lectura tras escritura** | Se publica el curso y se lee 12 veces el detalle y el catálogo público, repartidas entre las dos réplicas | 0 lecturas obsoletas en las dos |
| **Envío duplicado, misma clave** | 6 `POST /submissions` simultáneos con la misma `Idempotency-Key`; los que llegan mientras corre el original reciben `409 idempotency_in_progress` y, al reintentar, la respuesta grabada | 1 original + 5 repeticiones (`Idempotent-Replay: true`), un solo `submission_id`, intento 1, nota 100 |
| **Envío duplicado, otra réplica** | Se reenvía la misma clave hasta que la atiende una réplica distinta de la que calificó | mismo `submission_id`, mismo intento, misma nota; historial = 1 envío; auditoría = 1 `quiz.submitted` |
| **Clave reutilizada con otro cuerpo** | Misma clave, solo la primera respuesta | `422 idempotency_key_reused`: no se reinterpreta |
| **Envío duplicado, sin clave** | 6 `POST /submissions` simultáneos de otro estudiante, sin `Idempotency-Key` | 1 aceptado (intento 1) y 5 `409 quiz_conflict`: ninguno repitió número de intento; historial = 1 y auditoría = 1. Enviando después uno a uno sin clave se gastan los otros dos intentos y el siguiente es `409` con el historial en 3 de 3 |
| **Inscripción repetida** | 8 simultáneas sin clave; 8 con la misma clave de otro estudiante; reinscripción secuencial | 1 fila por estudiante y el censo en 2 filas. Con clave: 1 evento. **Sin clave: 8 eventos (el defecto)** |
| **Progreso repetido** | 16 latidos simultáneos (8 por lectura) que, el último de ellos, completan el curso | 16×`200`; avance 3/3 (no más); 1 `progress.course_completed`; **1** insignia; mismo avance en las dos réplicas |

Dos filas que conviene leer despacio:

- **Sin clave no hay «duplicado» que detectar.** Un segundo `POST` sin `Idempotency-Key` es, para el servidor, un segundo intento (el quiz del ejemplo tiene 3). Lo que la base garantiza —y se midió— es que dos simultáneos **nunca repiten número de intento** (restricción única) y que el historial y la auditoría coinciden con lo aceptado. La interfaz del estudiante manda siempre la clave. Nota 52.
- **Las respuestas del envío duplicado** están en el log, tal como pide el criterio: busca `6 envíos simultáneos de estudiante1` en [`corrida_consistencia.txt`](./antes_de_la_correccion/corrida_consistencia.txt) (cinco `409 idempotency_in_progress` y un `200` con la calificación) y el paso «El reenvío en otra réplica devuelve la misma calificación» (original en una réplica, repetición en la otra). El estado final está en los pasos siguientes: historial, auditoría y progreso.

---

## 2. El defecto y su corrección

`EnrollmentRepository.Enroll` decide entre `enrollment.created` y `enrollment.reactivated` leyendo antes la fila con `FOR UPDATE`. En la primera inscripción no hay fila que bloquear, así que las ocho solicitudes leían «no existía», una insertaba y las otras esperaban en el índice único para actualizar lo mismo, y **las ocho escribían el evento de alta**. La tabla de inscripciones quedaba bien; el rastro de auditoría contaba ocho altas de una sola.

Corrección ([`internal/postgres/enrollment_repository.go`](../../../../internal/postgres/enrollment_repository.go)): un candado de transacción por (estudiante, curso), `pg_advisory_xact_lock`, al empezar. Quien espera relee la fila ya confirmada y no registra nada.

Prueba ([`enrollment_repository_test.go`](../../../../internal/postgres/enrollment_repository_test.go), necesita `DATABASE_URL`): 12 estudiantes × 16 llamadas simultáneas cada uno. **Sin la corrección falla** (`enrollment rows = 1 (want 1), audit entries = 2 (want 1)`), con ella pasa. Se ejecutó en local contra PostgreSQL 16 junto con los paquetes `postgres`, `enrollment`, `quiz` y `progress`.

---

## 3. Cachés locales

Se revisó el código (no hay forma de medir una caché que no existe) y se comprobó con lecturas repetidas entre réplicas:

| Dónde podría haber una caché | Qué hay | Política |
| :--- | :--- | :--- |
| Memoria de la API | Ningún mapa ni `sync.Map` guarda cursos, inscripciones, intentos ni progreso: cada lectura va a PostgreSQL | Sin caché |
| Sesión | Redis compartido (`internal/cache/session_cache.go`), con respaldo en PostgreSQL | Compartida; la revocación es inmediata (E1, nota 3) |
| Clave de idempotencia | Redis compartido (`internal/cache/idempotency_store.go`); si Redis no responde, **rechaza** con 503 en vez de ejecutar dos veces | Compartida, falla cerrada |
| Almacén de idempotencia del worker | En memoria, **solo** si Redis no responde al arrancar el worker | Excepción documentada en `cmd/worker/main.go`; no sirve datos de negocio |
| HTTP | La API no envía `Cache-Control` y el balanceador no tiene CDN. El detalle del curso lleva `ETag` calculado de la fila: cualquier réplica lo reconoce (304 en las dos). **El catálogo no envía `ETag`** aunque el contrato lo diga (nota 52) | Sin caché intermedia |

---

## 4. Cómo repetirlo

Necesita dos réplicas de API activas (el paso 0 se detiene si el balanceador no reparte entre ellas) y las cuentas sembradas. Crea un curso nuevo cada vez; no toca datos existentes.

```bash
gcloud compute instance-groups managed update-autoscaling mooc-api-group --zone=us-east1-b --min-num-replicas=2
go run ./scripts/e2e_cloud -escenario consistencia -min-instancias 2
gcloud compute instance-groups managed update-autoscaling mooc-api-group --zone=us-east1-b --min-num-replicas=1   # valor de Terraform
```

Escribe `corrida_consistencia.txt` y `resultados.md` en esta carpeta (`-salida` para cambiarla). Gasta 4 inicios de sesión de la cuota por IP (el arnés espera 65 s si recibe 429).

---

## 5. Criterios de aceptación

| Criterio | Dónde |
| :--- | :--- |
| Las comprobaciones de estado compartido que aplican a la API, con evidencia | Sección 1; log y tabla de cada corrida |
| El envío duplicado, con ambas respuestas y el estado final | Sección 1, filas «misma clave» y «otra réplica» |
| Ninguna caché local contradice la consistencia; si hay, su política | Sección 3 |
| Lo que el equipo debe saber, en `NOTAS_TECNICAS.md` | [Nota 52](../../NOTAS_TECNICAS.md#52) |
