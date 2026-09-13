# Colecciones de Postman — Identidad (#24), Administración (#25) y Autoría de Cursos (#26)

Este documento describe la suite completa de pruebas automatizadas en **Postman / Newman** para los subsistemas de:
1. **Identidad, Autenticación y Control de Acceso** (Issue #24).
2. **Operaciones de Administración y Protección de Roles** (Issue #25).
3. **Autoría de Cursos, Jerarquía, Inmutabilidad y Publicación** (Issue #26).

La suite cumple estrictamente con los criterios de evaluación de la **Sección 9**, los flujos críticos de la **Sección 10.2** del pliego de condiciones y los estándares de diseño y seguridad de [`PROJECT_KEY_ASPECTS.md`](../../PROJECT_KEY_ASPECTS.md).

---

## 1. Estructura y Componentes

La carpeta `docs/postman/` contiene los siguientes artefactos:

| Archivo | Descripción |
|---|---|
| [`collection_api.postman_collection.json`](./collection_api.postman_collection.json) | Colección v2.1 de Postman para **Identidad y Seguridad** (Issue #24) con 44 peticiones organizadas secuencialmente, pre-request scripts y 110 aserciones automatizadas. |
| [`collection_admin.postman_collection.json`](./collection_admin.postman_collection.json) | Colección v2.1 de Postman para **Administración** (Issue #25) con 23 peticiones organizadas secuencialmente, tests de RBAC, casos borde de último administrador y 46 aserciones automatizadas. |
| [`collection_authoring.postman_collection.json`](./collection_authoring.postman_collection.json) | Colección v2.1 de Postman para **Autoría de Cursos** (Issue #26) con 30 peticiones organizadas secuencialmente, ciclo de vida completo de borrador a publicado, validación exhaustiva de publicación, inmutabilidad, reordenamiento con preservación de `stable_id` y 60 aserciones automatizadas. |
| [`mooc_local.postman_environment.json`](./mooc_local.postman_environment.json) | Entorno parametrizado para ejecuciones desde Postman Desktop en la máquina host (`http://localhost:8080` y `http://localhost:8025`). |
| [`mooc_docker.postman_environment.json`](./mooc_docker.postman_environment.json) | Entorno parametrizado para ejecuciones desatendidas en la red de Docker Compose (`http://api:8080` y `http://mailpit:8025`). |
| [`README.md`](./README.md) | Documentación técnica integral, matrices de peticiones/aserciones y guía de ejecución. |

**Total de la suite**: **97 peticiones HTTP** y **216 aserciones automatizadas** con **0 fallos**.

---

## 2. Variables de Entorno y Parametrización

Ninguna URL, credencial, identificador UUID ni token de sesión se encuentra hardcodeado en las peticiones. Todos los valores se parametrizan a través de los entornos (`mooc_local` / `mooc_docker`) o se extraen y asignan dinámicamente en tiempo de ejecución:

### 2.1 Variables Globales y de Entorno Base
| Variable | Ámbito | Propósito / Valor por Defecto |
|---|---|---|
| `baseUrl` | Entorno | URL base de la API REST (`http://localhost:8080` en local, `http://api:8080` en Docker). |
| `mailpitUrl` | Entorno | URL base de la API REST de Mailpit (`http://localhost:8025` en local, `http://mailpit:8025` en Docker). |
| `testPassword` | Entorno | Contraseña unificada de prueba para flujos de registro e identidad (`Password123!`). |
| `newPassword` | Entorno | Contraseña para validación de reseteo de clave (`NewPassword123!`). |
| `expiredToken` | Entorno | Token expirado/inválido para pruebas negativas (`expired-token-sample-00000000`). |

### 2.2 Variables Determinísticas para Administración (#25) y Autoría (#26)
Procedentes de los seeds sintéticos determinísticos ([`synthetic_data.sql`](../../scripts/seeds/synthetic_data.sql)):
| Variable | Ámbito | Valor por Defecto | Propósito |
|---|---|---|---|
| `adminEmail` | Entorno | `admin@plataforma-mooc.test` | Correo del Administrador Principal pre-sembrado. |
| `adminPassword` | Entorno | `Password123!` | Contraseña del Administrador Principal. |
| `adminPrimaryId` | Entorno | `a0000000-0000-0000-0000-000000000001` | UUID del Administrador Principal para prueba de último admin. |
| `adminSecondaryEmail` | Entorno | `admin.secundario@plataforma-mooc.test` | Correo del Administrador Secundario pre-sembrado. |
| `adminSecondaryId` | Entorno | `a0000000-0000-0000-0000-000000000002` | UUID del Administrador Secundario para suspender/reactivar. |
| `professorEmail` | Entorno | `profesor1@plataforma-mooc.test` | Correo de profesor pre-sembrado para flujos de autoría. |
| `professorPassword` | Entorno | `Password123!` | Contraseña del profesor. |
| `professorId` | Entorno | `b0000000-0000-0000-0000-000000000001` | UUID del profesor autor. |
| `studentEmail` | Entorno | `estudiante1@plataforma-mooc.test` | Correo de usuario no-admin (rol `estudiante`) pre-sembrado. |
| `studentPassword` | Entorno | `Password123!` | Contraseña de usuario no-admin. |
| `targetStudentId` | Entorno | `c0000000-0000-0000-0000-000000000001` | UUID del estudiante objetivo para cambios de rol y estado. |
| `seededPublishedCourseId` | Entorno | `d0000000-0000-0000-0000-000000000001` | UUID del curso publicado sembrado en la base de datos. |

### 2.3 Variables Dinámicas (Asignadas durante la Ejecución)
| Variable | Colección | Propósito |
|---|---|---|
| `testEmail` | Identidad | Correo determinístico-único por corrida (`postman-<timestamp>@example.test`) generado en el paso 01. |
| `verificationToken` | Identidad | Token de activación extraído del cuerpo del correo vía API REST de Mailpit. |
| `sessionToken` | Identidad | Token Bearer emitido tras inicio de sesión exitoso. |
| `adminToken` | Administración | Token Bearer del Administrador Principal emitido en el login administrativo. |
| `professorToken` | Autoría | Token Bearer del Profesor emitido en el login de autoría. |
| `studentToken` | Admón / Autoría | Token Bearer del Estudiante para verificar rechazos RBAC `403 Forbidden`. |
| `courseId` | Autoría | UUID del curso generado dinámicamente en el paso 02. |
| `courseStableId` | Autoría | Identificador estable del curso (`stable_id`) asignado en la creación. |
| `moduleId`, `moduleStableId` | Autoría | UUID e identificador estable del módulo creado. |
| `unitId`, `unitStableId` | Autoría | UUID e identificador estable de la unidad creada. |
| `resource1Id`, `resource1StableId` | Autoría | Identificadores del recurso 1 (lectura obligatoria). |
| `resource2Id`, `resource2StableId` | Autoría | Identificadores del recurso 2 (complementario, para prueba de reordenamiento). |
| `resource3Id`, `resource3StableId` | Autoría | Identificadores del recurso 3 (quiz formativo, para verificar preservación de `stable_id`). |
| `courseETag` | Autoría | Cabecera `ETag` capturada durante la previsualización para validación de concurrencia. |

---

## 3. Cobertura de Criterios de Aceptación

### 3.1 Subsistema de Identidad, Autenticación y Seguridad (#24)
- **Flujos Funcionales:** Registro público (`pending_verification`), activación por enlace Mailpit (`active`), login seguro (`token` Bearer con expiración), revocación de sesión por ID (204 -> 401), logout de sesión activa (204 -> 401) y restablecimiento de clave vía correo con revocación masiva de sesiones.
- **Casos Negativos Obligatorios:** Credenciales erróneas (401), correo no registrado (401 anti-enumeración), token expirado/consumido (400 `invalid_token`), intento de registrar profesor por endpoint público (400 `invalid_body`), clave corta (400 `invalid_input`), reuso de clave de idempotencia con payload dispar (422), y rate limit de login (429 con cabecera `Retry-After`).

### 3.2 Subsistema de Administración y Control de Acceso (#25)
- **Flujos Funcionales:** Listado con paginación (`limit`, `has_more`), filtrado por `role` (`profesor`), filtrado por `status` (`active`), consulta detallada por ID con `ETag`, cambio de rol (`estudiante` -> `profesor` -> `estudiante`) y suspensión/reactivación de cuentas.
- **Casos Negativos RBAC:** Usuarios no-admin (rol `estudiante`) son rechazados uniformemente con `403 Forbidden` (`code: forbidden`) en todas las operaciones administrativas; peticiones anónimas reciben `401 Unauthorized` con cabecera `WWW-Authenticate: Bearer`.
- **Caso Borde:** Protección del último administrador activo: tras suspender al administrador secundario, cualquier intento de suspender o degradar el rol del único administrador restante es rechazado con `409 Conflict` (`code: last_admin_protected`). La reactivación limpia del administrador secundario restaura el estado determinístico original.

### 3.3 Subsistema de Autoría de Cursos (#26)

#### A. Jerarquía de 4 Niveles, Previsualización y Publicación (Criterio 1)
1. **Creación de Curso en Borrador:** `POST /api/v1/courses` crea un curso con `version: 1` y `status: "draft"`, asignando un `stable_id` persistente y registrando la traza de auditoría.
2. **Validación Exhaustiva de Publicación (Multi-Error Aggregate):** `POST /api/v1/courses/{courseId}/publish` ejecutado sobre un curso sin estructura devuelve `422 Unprocessable Entity` con código `validation_failed` y una lista agregada de errores en `details` que contiene simultáneamente:
   - `structure`: Ausencia de módulos/unidades/recursos visibles.
   - `approval_criteria`: Falta de recurso obligatorio visible que defina el criterio de finalización.
3. **Estructuración Jerárquica:**
   - **Módulo:** `POST /api/v1/courses/{courseId}/modules` crea el módulo en `position: 0` con `stable_id`.
   - **Unidad:** `POST /api/v1/modules/{moduleId}/units` crea la unidad bajo el módulo en `position: 0` con `stable_id`.
   - **Recursos Diversos:** `POST /api/v1/units/{unitId}/resources` crea recursos con diferentes tipos y configuraciones:
     - Recurso 1: Tipo `text` (Extended Markdown), `is_visible: true`, `is_mandatory: true` (posición 0).
     - Recurso 2: Tipo `text` complementario, `is_mandatory: false`, `allow_download: true` (posición 1).
     - Recurso 3: Tipo `quiz`, `is_mandatory: true` (posición 2).
4. **Previsualización de la Estructura Completa:**
   - Previsualización del curso: `GET /api/v1/courses/{courseId}` devuelve metadatos, estado `draft`, versión 1 y cabecera HTTP `ETag`.
   - Previsualización de módulos: `GET /api/v1/courses/{courseId}/modules` verifica la lista ordenada.
   - Previsualización de unidades: `GET /api/v1/modules/{moduleId}/units` verifica la lista ordenada.
   - Previsualización de recursos: `GET /api/v1/units/{unitId}/resources` verifica la lista de 3 recursos en posiciones 0, 1 y 2.
5. **Publicación Exitosa:** `POST /api/v1/courses/{courseId}/publish` valida que la estructura satisface todos los requisitos y transiciona el curso a `status: "published"` con `version: 1`. El curso queda visible de inmediato en el catálogo público (`GET /api/v1/courses/{courseId}`).

#### B. Inmutabilidad Estricta de la Versión Publicada (Criterio 2)
Una vez que un curso alcanza el estado `published`, cualquier intento de mutación directa sobre el curso o su estructura es terminantemente bloqueado:
1. **Edición de metadatos:** `PUT /api/v1/courses/{courseId}` -> Rechazado con `409 Conflict` (`code: course_immutable`).
2. **Adición de módulo:** `POST /api/v1/courses/{courseId}/modules` -> Rechazado con `409 Conflict` (`code: course_immutable`).
3. **Adición de unidad:** `POST /api/v1/modules/{moduleId}/units` -> Rechazado con `409 Conflict` (`code: course_immutable`).
4. **Adición de recurso:** `POST /api/v1/units/{unitId}/resources` -> Rechazado con `409 Conflict` (`code: course_immutable`).
5. **Eliminación de recurso:** `DELETE /api/v1/resources/{resourceId}` -> Rechazado con `409 Conflict` (`code: course_immutable`).
6. **Republicación duplicada:** `POST /api/v1/courses/{courseId}/publish` -> Rechazado con `409 Conflict` (`code: conflict`).
7. **Flujo de Despublicación Temporal (MVP Sección 5.1):**
   - El profesor ejecuta `POST /api/v1/courses/{courseId}/unpublish` -> Transiciona a `status: "unpublished"`.
   - Con el curso despublicado, `PUT /api/v1/courses/{courseId}` permite actualizar título y descripción exitosamente (200 OK).
   - El profesor vuelve a publicar con `POST /api/v1/courses/{courseId}/publish` -> Vuelve a `status: "published"`.

#### C. Preservación de Identificadores Estables tras Reordenamiento (Criterio 3)
1. Antes del reordenamiento, la unidad cuenta con tres recursos en posiciones:
   - Recurso 1: Posición 0, `stable_id: resource1StableId`.
   - Recurso 2: Posición 1, `stable_id: resource2StableId`.
   - Recurso 3: Posición 2, `stable_id: resource3StableId`.
2. Se ejecuta `DELETE /api/v1/resources/{resource2Id}` para eliminar el recurso intermedio (posición 1).
3. La base de datos cierra automáticamente el hueco posicional (renumerando a los hermanos con posición mayor).
4. Se consulta `GET /api/v1/units/{unitId}/resources` y se comprueba:
   - La lista ahora tiene longitud 2.
   - El recurso 1 permanece en posición 0 con su `stable_id` intacto.
   - El recurso 3 se ha desplazado de la posición 2 a la **posición 1**.
   - El `id` y el **`stable_id` del recurso 3 permanecen idénticos**, garantizando que el progreso de los estudiantes registrado contra ese identificador estable no se desalinea ni se corrompe.

#### D. Control de Acceso RBAC y Seguridad
1. Un usuario con rol `estudiante` intentando crear un curso (`POST /api/v1/courses`) -> `403 Forbidden` (`code: forbidden`).
2. Un estudiante intentando agregar módulos (`POST /api/v1/courses/{id}/modules`) -> `403 Forbidden` (`code: forbidden`).
3. Un estudiante intentando publicar un curso (`POST /api/v1/courses/{id}/publish`) -> `403 Forbidden` (`code: forbidden`).
4. Una petición sin cabecera de autenticación a endpoints de autoría -> `401 Unauthorized` con cabecera `WWW-Authenticate: Bearer`.

---

## 4. Matriz de Peticiones y Aserciones Automáticas

### 4.1 Colección de Autoría de Cursos (#26) — 30 Peticiones / 60 Aserciones

| # | Petición | Método y Ruta | HTTP | Aserción de Forma de Respuesta | Naturaleza |
|---|---|---|:---:|---|---|
| **01** | Login como Profesor | `POST /api/v1/auth/login` | 200 | `token` Bearer obtenido, `user.role == "profesor"` | Positiva |
| **02** | Crear Curso en Borrador (Draft) | `POST /api/v1/courses` | 201 | `status == "draft"`, `version == 1`, `stable_id` UUID no vacío | Positiva |
| **03** | Intento de publicación con errores | `POST /api/v1/courses/{courseId}/publish` | 422 | `code == "validation_failed"`, lista de errores con `structure` y `approval_criteria` | Negativa (422) |
| **04** | Crear Módulo 1 en Borrador | `POST /api/v1/courses/{courseId}/modules` | 201 | `position == 0`, `stable_id` no vacío, `course_id` coincide | Positiva |
| **05** | Crear Unidad 1 en Módulo 1 | `POST /api/v1/modules/{moduleId}/units` | 201 | `position == 0`, `stable_id` no vacío, `module_id` coincide | Positiva |
| **06** | Crear Recurso 1 (Lectura Obligatoria) | `POST /api/v1/units/{unitId}/resources` | 201 | `position == 0`, `is_mandatory == true`, `type == "text"` | Positiva |
| **07** | Crear Recurso 2 (Complementario - Pos 1) | `POST /api/v1/units/{unitId}/resources` | 201 | `position == 1`, `is_mandatory == false`, `type == "text"` | Positiva |
| **08** | Crear Recurso 3 (Quiz Formativo - Pos 2) | `POST /api/v1/units/{unitId}/resources` | 201 | `position == 2`, `is_mandatory == true`, `type == "quiz"` | Positiva |
| **09** | Previsualizar Metadatos del Curso (ETag) | `GET /api/v1/courses/{courseId}` | 200 | `status == "draft"`, `version == 1`, cabecera `ETag` presente | Previsualización |
| **10** | Previsualizar Módulos del Curso | `GET /api/v1/courses/{courseId}/modules` | 200 | `items.length >= 1`, módulo en posición 0 | Previsualización |
| **11** | Previsualizar Unidades del Módulo | `GET /api/v1/modules/{moduleId}/units` | 200 | `items.length >= 1`, unidad en posición 0 | Previsualización |
| **12** | Previsualizar Recursos de la Unidad | `GET /api/v1/units/{unitId}/resources` | 200 | `items.length == 3`, posiciones 0, 1 y 2 ordenadas | Previsualización |
| **13** | Reordenar: Eliminar recurso intermedio | `DELETE /api/v1/resources/{resource2Id}` | 204 | Cuerpo estrictamente vacío | Reordenamiento |
| **14** | Verificar Reordenamiento y `stable_id` | `GET /api/v1/units/{unitId}/resources` | 200 | `items.length == 2`, Recurso 3 ahora en posición 1 con su `stable_id` e `id` idénticos | Invariante Borde |
| **15** | Publicación Exitosa del Curso | `POST /api/v1/courses/{courseId}/publish` | 200 | `status == "published"`, `version == 1`, `ETag` actualizado | Positiva |
| **16** | Consultar Curso Publicado en Catálogo | `GET /api/v1/courses/{courseId}` | 200 | `status == "published"`, accesible públicamente | Positiva |
| **17** | Inmutabilidad: Rechazo al editar metadatos | `PUT /api/v1/courses/{courseId}` | 409 | `code == "course_immutable"`, mensaje explicativo | Inmutabilidad |
| **18** | Inmutabilidad: Rechazo al agregar módulo | `POST /api/v1/courses/{courseId}/modules` | 409 | `code == "course_immutable"` | Inmutabilidad |
| **19** | Inmutabilidad: Rechazo al agregar unidad | `POST /api/v1/modules/{moduleId}/units` | 409 | `code == "course_immutable"` | Inmutabilidad |
| **20** | Inmutabilidad: Rechazo al agregar recurso | `POST /api/v1/units/{unitId}/resources` | 409 | `code == "course_immutable"` | Inmutabilidad |
| **21** | Inmutabilidad: Rechazo al eliminar recurso | `DELETE /api/v1/resources/{resource1Id}` | 409 | `code == "course_immutable"` | Inmutabilidad |
| **22** | Inmutabilidad: Rechazo republicar curso | `POST /api/v1/courses/{courseId}/publish` | 409 | `code == "conflict"` | Inmutabilidad |
| **23** | Despublicación temporal (MVP 5.1) | `POST /api/v1/courses/{courseId}/unpublish` | 200 | `status == "unpublished"` | Positiva |
| **24** | Editar curso temporalmente despublicado | `PUT /api/v1/courses/{courseId}` | 200 | Título y descripción actualizados exitosamente | Positiva |
| **25** | Republicar curso tras actualización | `POST /api/v1/courses/{courseId}/publish` | 200 | `status == "published"`, nuevo título persistido | Positiva |
| **26** | Login como Estudiante (No-autor) | `POST /api/v1/auth/login` | 200 | `studentToken` emitido, `user.role == "estudiante"` | Positiva |
| **27** | No-autor intenta crear curso | `POST /api/v1/courses` | 403 | `code == "forbidden"` | Negativa RBAC |
| **28** | No-autor intenta agregar módulo | `POST /api/v1/courses/{courseId}/modules` | 403 | `code == "forbidden"` | Negativa RBAC |
| **29** | No-autor intenta publicar curso | `POST /api/v1/courses/{courseId}/publish` | 403 | `code == "forbidden"` | Negativa RBAC |
| **30** | Petición anónima sin autenticación | `POST /api/v1/courses` | 401 | Cabecera `WWW-Authenticate: Bearer`, `code == "unauthorized"` | Negativa RBAC |

---

## 5. Instrucciones de Ejecución

### Opción 1: Ejecución Automatizada con Makefile / Docker (Recomendada para CI y Evaluación)

Asegúrate de tener la infraestructura levantada con los datos sintéticos sembrados:
```bash
docker compose up -d
make seed
```

#### Ejecutar todas las suites secuencialmente (Identidad + Administración + Autoría):
```bash
make test-postman
```

#### Ejecutar individualmente la suite de Autoría de Cursos (#26):
```bash
make test-postman-authoring
```

O directamente mediante Docker con la imagen oficial de Newman:
```bash
docker run --rm --network plataforma-mooc_default   -v "$(pwd)/docs/postman":/etc/newman postman/newman:alpine   run /etc/newman/collection_authoring.postman_collection.json   -e /etc/newman/mooc_docker.postman_environment.json
```

**Resultado esperado para Autoría de Cursos (#26):**
- 30 peticiones HTTP ejecutadas.
- 30 scripts de prueba evaluados.
- **60 aserciones superadas con 0 fallos**.

---

### Opción 2: Ejecución en Postman Desktop

1. Abre **Postman Desktop**.
2. Haz clic en **Import** y selecciona los archivos correspondientes:
   - Colección: `docs/postman/collection_authoring.postman_collection.json` (y las demás colecciones deseadas).
   - Entorno: `docs/postman/mooc_local.postman_environment.json`.
3. En la esquina superior derecha, selecciona el entorno activo: **Plataforma MOOC - Local**.
4. Abre la colección **Plataforma MOOC - Autoria de Cursos** y entra a la pestaña **Runner** (o haz clic en el botón *Run Collection*).
5. Selecciona ejecutar todas las peticiones en el orden original.
6. Haz clic en **Run Plataforma MOOC - Autoria de Cursos**.
