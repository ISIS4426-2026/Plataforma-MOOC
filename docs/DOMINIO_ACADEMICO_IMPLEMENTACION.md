# Dominio Académico, Auditoría y Observabilidad — Implementación

> **Autor:** tmichelldiaz
> **Issues cubiertos:** [#15](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/15) · [#16](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/16) · [#17](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/17) · [#18](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/18) · [#19](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/19) · [#20](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/20) · [#21](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/21)
> **Documentos relacionados:** [`DATABASE_DESIGN.md`](DATABASE_DESIGN.md) (esquema relacional y política de ordenamiento en detalle) · [`e2e/REPORTE_E2E_IDENTIDAD_Y_AUTORIA.md`](e2e/REPORTE_E2E_IDENTIDAD_Y_AUTORIA.md) (evidencia de pruebas end-to-end) · [`README.md`](../README.md) (guía de despliegue y observabilidad)

Este documento explica **qué se construyó y por qué**, a nivel de diseño, para la porción del backend correspondiente al dominio académico (autoría de cursos y su estructura interna), la auditoría inmutable y la observabilidad básica. `DATABASE_DESIGN.md` documenta el esquema; este documento documenta las decisiones de negocio y las reglas de concurrencia detrás del código en `internal/course`, `internal/structure`, `internal/domain` y `internal/observability`.

---

## Índice

1. [Resumen y alcance](#1-resumen-y-alcance)
2. [#15 — Modelo de datos del dominio académico](#2-15--modelo-de-datos-del-dominio-académico)
3. [#16 — Reglas de orden e identificadores estables](#3-16--reglas-de-orden-e-identificadores-estables)
4. [#17 — CRUD de Curso](#4-17--crud-de-curso)
5. [#18 — Auditoría inmutable](#5-18--auditoría-inmutable)
6. [#19 — CRUD de Módulo, Unidad y Recurso](#6-19--crud-de-módulo-unidad-y-recurso)
7. [#20 — Publicación de versión inmutable](#7-20--publicación-de-versión-inmutable)
8. [#21 — Observabilidad básica](#8-21--observabilidad-básica)
9. [Endpoints expuestos](#9-endpoints-expuestos)
10. [Cómo verificar cada pieza](#10-cómo-verificar-cada-pieza)

---

## 1. Resumen y alcance

El equipo dividió el backend en verticales por persona. Esta porción cubre el **dominio académico completo**: desde el modelo de datos hasta que un curso queda publicado e inmutable, más dos temas transversales que dependían de esa base — auditoría y observabilidad. En el diagrama de dependencias de la entrega, este bloque es el que conecta la infraestructura base (migraciones, autenticación) con lo que consume QA en sus pruebas de autoría.

```
#15 Modelo de datos
      │
      ▼
#16 Orden e IDs estables
      │
      ├──────────────┬──────────────┐
      ▼              ▼              ▼
#17 CRUD Curso   #18 Auditoría   #21 Observabilidad
      │
      ▼
#19 CRUD Módulo/Unidad/Recurso
      │
      ▼
#20 Publicación de versión inmutable
```

Todo el código nuevo vive en:

| Paquete | Responsabilidad |
| :--- | :--- |
| `internal/domain/course.go` | Entidades (`Course`, `Module`, `Unit`, `Resource`) y los puertos (`CourseRepository`, `ModuleRepository`, `UnitRepository`, `ResourceRepository`) |
| `internal/domain/ordering.go` | Algoritmo de reposicionamiento (`InsertAt`, `MoveTo`, `Reposition`) |
| `internal/domain/audit.go` | `AuditEntry`, `AuditFilter`, puerto `AuditRepository` |
| `internal/domain/validation.go` | `ValidationErrors` / `FieldError`, usados por la validación de publicación |
| `internal/course` | Servicio de autoría: crear/editar/publicar/despublicar un curso |
| `internal/structure` | Servicio de CRUD para Módulo, Unidad y Recurso |
| `internal/postgres/course_repository.go` | Persistencia de `Course` |
| `internal/postgres/structure_repository.go` | Persistencia de `Module`, `Unit`, `Resource` |
| `internal/postgres/audit_repository.go` | Persistencia y consulta del rastro de auditoría |
| `internal/observability` | Configuración de OpenTelemetry (trazas y métricas) |
| `internal/http/handler/{course,structure,audit}.go` | Adaptadores HTTP |

---

## 2. #15 — Modelo de datos del dominio académico

**Criterios de aceptación:** jerarquía Curso → Módulo → Unidad → Recurso; metadatos, estado y versión del curso; los 10 tipos de recurso controlados; texto persistido como Markdown extendido canónico y binarios solo por referencia; diagrama ER de referencia.

**Estado al tomar el issue:** el compañero que construyó las migraciones (`000001_init_schema.up.sql`) ya había dejado la jerarquía completa en SQL y el diagrama ER en `DATABASE_DESIGN.md`. El único vacío real era que el `enum` de Go (`internal/domain/course.go`) solo declaraba 6 de los 10 tipos de recurso que la base de datos ya permitía.

**Qué se hizo:** se completó `ResourceType` con los 4 tipos faltantes (`presentation`, `downloadable`, `iframe`, `external_link`), dejando el modelo de Go en paridad exacta con la restricción `CHECK` de la tabla `resources`.

---

## 3. #16 — Reglas de orden e identificadores estables

**Criterios de aceptación:** cada elemento tiene un identificador estable independiente de la versión; reordenar no debe alterar esos identificadores; documentar cómo se recalculan las posiciones sin colisiones.

**Qué se hizo:**

- **`internal/domain/ordering.go`** define el algoritmo puro (sin base de datos): `InsertAt` calcula el nuevo orden de una lista de `stable_id` al insertar un elemento en un índice dado; `MoveTo` lo hace para mover uno existente; `Reposition` convierte ese orden en un mapa `stable_id → posición` denso (`0..n-1`). Insertar siempre es el caso trivial de `InsertAt` (se agrega al final), por lo que las escrituras de `internal/structure` no necesitan invocar el algoritmo completo, solo `len(hermanos)`.
- **Garantía a nivel de base de datos:** migración `000003_academic_ordering.up.sql` agrega restricciones `UNIQUE (padre_id, position) DEFERRABLE INITIALLY DEFERRED` sobre `modules`, `units` y `resources`. `DEFERRABLE` es la pieza clave: permite que una transacción reescriba temporalmente varias posiciones (y por tanto haga colisionar dos filas a mitad de camino) sin que Postgres rechace la escritura, siempre que al final de la transacción no quede ninguna colisión real.
- **Prueba de la propiedad pedida por el issue** (que el progreso de un estudiante sobrevive a un reordenamiento): `TestReorderPreservesStableIdentityForProgress` en `internal/domain/ordering_test.go` construye una unidad con recursos, marca uno como completado por su `stable_id`, lo reordena, y verifica que ese `stable_id` sigue resolviendo al mismo recurso.

Detalle completo del algoritmo y la restricción `DEFERRABLE` en [`DATABASE_DESIGN.md`, sección 5](DATABASE_DESIGN.md#5-ordenamiento-e-identificadores-estables-issue-16).

---

## 4. #17 — CRUD de Curso

**Criterios de aceptación:** crear/editar/consultar curso con metadatos, estado y versión vigente; nace en borrador; solo profesores autorizados y administradores pueden crear/editar; editar una versión ya publicada no debe alterarla.

**Qué se hizo (`internal/course/service.go`, `internal/postgres/course_repository.go`):**

- `Create` valida rol (`profesor` o `administrador`) y campos obligatorios, y el curso nace en `CourseStatusDraft`.
- `Update` primero resuelve **propiedad**: un profesor solo edita sus propios cursos; un administrador edita cualquiera. Esa verificación ocurre en el servicio, no en el repositorio, porque el `author_id` no cambia bajo concurrencia — no hay una condición de carrera que cerrar ahí, a diferencia de la comprobación de inmutabilidad.
- **Inmutabilidad de una versión publicada:** el repositorio bloquea la fila (`SELECT ... FOR UPDATE`) y, dentro de esa misma transacción, revisa `status`. Si ya es `published`, la escritura se rechaza con `ErrCourseImmutable` — no hay forma de que una edición se cuele entre la verificación y la escritura porque ambas ocurren bajo el mismo candado de fila.
- Concurrencia optimista vía `ETag`/`If-Match`, siguiendo el mismo patrón que ya usaba `internal/admin` para usuarios.

---

## 5. #18 — Auditoría inmutable

**Criterios de aceptación:** toda acción administrativa y de autoría genera un registro; el registro es *append-only* (no se puede editar ni borrar); cada registro identifica quién, qué, cuándo y sobre qué entidad; un intento de modificar un registro existente debe fallar siempre.

**Punto de partida:** el compañero de administración de usuarios ya había dejado `AuditRepository.Record` y la tabla `audit_logs`, con el contrato de que todo repositorio que cambia estado escribe su cambio y la entrada de auditoría en la misma transacción (para que "toda acción queda auditada" no dependa de que dos escrituras separadas no fallen independientemente).

**Qué se hizo:**

- **Inmutabilidad real, no solo por convención:** migración `000004_audit_immutability.up.sql` agrega un *trigger* que rechaza cualquier `UPDATE` o `DELETE` sobre `audit_logs`, sin importar qué rol de base de datos lo intente. Se prefirió un trigger sobre revocar permisos del rol de aplicación porque el trigger protege la tabla en cualquier entorno, sin depender de con qué usuario se conecte la API en ese entorno particular.
- **Lado de lectura:** `AuditRepository.List` (nuevo), con filtro por actor, por prefijo de acción (aprovechando que las acciones ya se nombran `<recurso>.<hecho>`), por recurso objetivo y por rango de fechas, con paginación por cursor igual a la del resto de la API. Expuesto en `GET /api/v1/admin/audit-logs`, solo para administradores.
- Se extendieron `internal/course` e `internal/structure` para que cada creación, edición, borrado, publicación y despublicación registre su entrada de auditoría en la misma transacción que el cambio.

**Prueba del criterio central** ("cualquier intento de modificar un registro falla siempre"): `TestAuditLogsRejectsUpdate` y `TestAuditLogsRejectsDelete` en `internal/postgres/audit_repository_test.go` ejecutan un `UPDATE`/`DELETE` real contra `audit_logs` y confirman que Postgres los rechaza.

---

## 6. #19 — CRUD de Módulo, Unidad y Recurso

**Criterios de aceptación:** CRUD completo respetando la jerarquía; cada recurso define título, orden, visibilidad, posibilidad de descarga, obligatoriedad y estado de procesamiento; no se puede crear una unidad sin módulo padre ni un recurso sin unidad padre; toda escritura queda en auditoría.

**Un vacío encontrado al implementar:** el modelo de recurso no tenía "posibilidad de descarga" (independiente del tipo de recurso — un video puede ser solo para streaming o descargable). Se agregó la columna `allow_download` vía `000005_resource_downloadable.up.sql`.

**Qué se hizo (`internal/structure/service.go`, `internal/postgres/structure_repository.go`):**

- **Regla de integridad del issue, aplicada con bloqueo de fila:** cada escritura bloquea (`FOR UPDATE OF`) toda la cadena de padres hasta `courses`, en una sola consulta. Eso resuelve dos cosas a la vez: si el padre no existe, la fila no aparece y se responde `ErrNotFound`; y si existe, el mismo bloqueo evita que dos inserciones concurrentes bajo el mismo padre lean el mismo conteo de hermanos y les asignen la misma posición.
- **Nuevo elemento siempre al final** (`position = COUNT(hermanos)`), el caso trivial del algoritmo de `#16`.
- **Borrado cierra el hueco:** tras eliminar una fila, se actualiza `position = position - 1` en los hermanos posteriores, para no dejar huecos en la secuencia `0..n-1`.
- El mismo candado de fila que resuelve "padre debe existir" también revisa el estado del curso: si está publicado, cualquier escritura de estructura se rechaza igual que en `#17` — un curso publicado es inmutable en su totalidad, no solo en sus metadatos.

---

## 7. #20 — Publicación de versión inmutable

**Criterios de aceptación:** publicar solo es posible con metadatos completos, estructura mínima (al menos un módulo con una unidad con un recurso visible y disponible) y criterios de aprobación definidos; una vez publicada, la versión anterior deja de ser editable; un intento de publicar con datos incompletos devuelve **todos** los errores, no solo el primero.

**Decisión de diseño documentada explícitamente en el código:** el modelo no tiene un campo separado para "criterios de aprobación". Se interpretó como: *al menos un recurso, dentro del camino mínimo válido, está marcado como obligatorio* — porque la aprobación de un estudiante se calculará justamente a partir de qué recursos obligatorios completó, y cero recursos obligatorios significa que no hay nada que defina la aprobación.

**Qué se hizo (`internal/course/service.go`):**

- `Publish` recorre módulos → unidades → recursos y acumula **todos** los problemas encontrados (título vacío, descripción vacía, sin estructura mínima, sin criterio de aprobación) en `domain.ValidationErrors`, en vez de retornar al primer error. El handler HTTP traduce eso a `422` con la lista completa en el campo `details`.
- Al publicar exitosamente, el curso pasa a `CourseStatusPublished`; a partir de ahí, `#17` y `#19` ya rechazan cualquier escritura sobre él.
- **`Unpublish` (agregado, no pedido explícitamente por el issue):** sin una forma de despublicar, un curso publicado no tendría *ningún* camino de vuelta a editable. La sección 5.1 del enunciado dice explícitamente que en el MVP "la edición de un curso publicado exige despublicarlo temporalmente" — así que se agregó como el complemento natural de `Publish`, sin la mecánica completa de versiones nuevas (`borrador de actualización`), que el propio enunciado marca como alcance opcional (sección 5.2).

**Prueba de extremo a extremo, tal como la pide el issue:** `TestPublishRoundTripLeavesThePublishedVersionImmutable` en `internal/course/publish_test.go` — borrador incompleto falla con la lista de errores, se completa, se publica, y luego se confirma que un intento de editarlo devuelve `ErrCourseImmutable`.

---

## 8. #21 — Observabilidad básica

**Criterios de aceptación:** logs estructurados (JSON) con nivel, timestamp y request-id; métricas mínimas vía OpenTelemetry (latencia por endpoint, tasa de error, jobs procesados/fallidos); logs y trazas correlacionados por un identificador común; dado un request-id, se debe poder encontrar su log y su traza.

**Punto de partida:** los logs JSON estructurados con `request_id` ya existían (`slog` + `middleware.RequestLogger`). Faltaba la mitad de trazas/métricas y la correlación entre las tres señales.

**Qué se hizo:**

- **`internal/observability/observability.go`:** configura un `TracerProvider` (las trazas se exportan como JSON al mismo `stdout` que ya captura `docker compose logs`, así que no hace falta levantar un colector aparte para inspeccionarlas) y un `MeterProvider` respaldado por un registro Prometheus propio, expuesto en `GET /api/v1/metrics` (API) y `GET :9090/metrics` (worker, que no tenía servidor HTTP propio).
- **`internal/http/middleware/tracing.go`:** abre un *span* por request, entre `RequestID` y `RequestLogger` en la cadena de middlewares — ese orden es lo que permite que el `request_id` quede como atributo del span, y que el `trace_id` del span ya exista cuando `RequestLogger` arma la línea de log. Registra `http.server.request.duration` (latencia por endpoint) y `http.server.request.errors` (tasa de error en 5xx).
- **`internal/worker/metrics.go`:** `worker.jobs.processed` y `worker.jobs.failed`, por tipo de tarea — la mitad de "jobs procesados/fallidos" del criterio de aceptación.

**Prueba del criterio central**, escrita literalmente como el issue la describe: `TestRequestIDLogAndTraceAreCorrelated` en `internal/http/middleware/tracing_test.go` hace una petición, y verifica que la línea de log contiene el `request_id` de la respuesta y un `trace_id`, y que ese mismo `trace_id` identifica el *span* capturado — en cualquiera de los dos sentidos, un identificador lleva al otro.

Detalle de cómo correlacionar en logs reales de Docker en el [`README.md`, sección de Observabilidad](../README.md#observabilidad-trazas-y-métricas-prometheus).

---

## 9. Endpoints expuestos

| Método | Ruta | Rol requerido | Issue |
| :--- | :--- | :--- | :---: |
| `GET` | `/api/v1/courses` | Público | #17 |
| `POST` | `/api/v1/courses` | Profesor / Admin | #17 |
| `GET` | `/api/v1/courses/{course_id}` | Público | #17 |
| `PUT` | `/api/v1/courses/{course_id}` | Profesor / Admin (dueño) | #17 |
| `POST` | `/api/v1/courses/{course_id}/publish` | Profesor / Admin (dueño) | #20 |
| `POST` | `/api/v1/courses/{course_id}/unpublish` | Profesor / Admin (dueño) | #20 |
| `GET` / `POST` | `/api/v1/courses/{course_id}/modules` | Público / Profesor-Admin | #19 |
| `PUT` / `DELETE` | `/api/v1/modules/{module_id}` | Profesor / Admin (dueño) | #19 |
| `GET` / `POST` | `/api/v1/modules/{module_id}/units` | Público / Profesor-Admin | #19 |
| `PUT` / `DELETE` | `/api/v1/units/{unit_id}` | Profesor / Admin (dueño) | #19 |
| `GET` / `POST` | `/api/v1/units/{unit_id}/resources` | Público / Profesor-Admin | #19 |
| `PUT` / `DELETE` | `/api/v1/resources/{resource_id}` | Profesor / Admin (dueño) | #19 |
| `GET` | `/api/v1/admin/audit-logs` | Administrador | #18 |
| `GET` | `/api/v1/metrics` | Público (scraping) | #21 |

Contrato completo (parámetros, esquemas, respuestas de error) en [`api/openapi.yaml`](../api/openapi.yaml) y probable en Swagger UI (`/api/docs`) con el stack levantado.

---

## 10. Cómo verificar cada pieza

Pruebas unitarias (sin infraestructura, corren en segundos):

```bash
go test ./internal/domain/... ./internal/course/... ./internal/structure/... ./internal/observability/... ./internal/http/middleware/... ./internal/worker/...
```

Pruebas de integración contra PostgreSQL real (requieren `DATABASE_URL`; se omiten limpiamente si no está configurada):

```bash
export DATABASE_URL="postgres://moocuser:moocpassword@localhost:5432/moocdb?sslmode=disable"
go test ./migrations/... ./internal/postgres/...
```

Con el stack completo levantado (`make docker-up`), las colecciones Postman de autoría (`docs/postman/collection_authoring.postman_collection.json`) ejercitan el mismo código descrito aquí de punta a punta contra la API real; el detalle de esa cobertura está en [`docs/e2e/REPORTE_E2E_IDENTIDAD_Y_AUTORIA.md`](e2e/REPORTE_E2E_IDENTIDAD_Y_AUTORIA.md).

En total, esta porción del backend agrega **~64 pruebas** propias (unitarias + integración) sobre el dominio académico, la auditoría y la observabilidad, sin contar las aserciones de las colecciones Postman.
