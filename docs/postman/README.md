# Colecciones de Postman — Identidad (#24) y Administración (#25)

Este documento describe la suite completa de pruebas automatizadas en **Postman / Newman** para los subsistemas de **Identidad, Autenticación y Control de Acceso** (Issue #24) y **Operaciones de Administración y Protección de Roles** (Issue #25) de la Plataforma MOOC.

La suite cumple estrictamente con los criterios de evaluación de la **Sección 9**, los flujos críticos de la **Sección 10.2** del pliego de condiciones y los estándares de diseño y seguridad de [`PROJECT_KEY_ASPECTS.md`](../../PROJECT_KEY_ASPECTS.md).

---

## 1. Estructura y Componentes

La carpeta `docs/postman/` contiene los siguientes artefactos:

| Archivo | Descripción |
|---|---|
| [`collection_api.postman_collection.json`](./collection_api.postman_collection.json) | Colección v2.1 de Postman para **Identidad y Seguridad** (Issue #24) con 44 peticiones organizadas secuencialmente, pre-request scripts y 110 aserciones automatizadas. |
| [`collection_admin.postman_collection.json`](./collection_admin.postman_collection.json) | Colección v2.1 de Postman para **Administración** (Issue #25) con 23 peticiones organizadas secuencialmente, tests de RBAC, casos borde de último administrador y 46 aserciones automatizadas. |
| [`mooc_local.postman_environment.json`](./mooc_local.postman_environment.json) | Entorno parametrizado para ejecuciones desde Postman Desktop en la máquina host (`http://localhost:8080` y `http://localhost:8025`). |
| [`mooc_docker.postman_environment.json`](./mooc_docker.postman_environment.json) | Entorno parametrizado para ejecuciones desatendidas en la red de Docker Compose (`http://api:8080` y `http://mailpit:8025`). |
| [`README.md`](./README.md) | Documentación técnica integral, matrices de peticiones/aserciones y guía de ejecución. |

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

### 2.2 Variables Determinísticas para Administración (#25)
Procedentes de los seeds sintéticos determinísticos ([`synthetic_data.sql`](../../scripts/seeds/synthetic_data.sql)):
| Variable | Ámbito | Valor por Defecto | Propósito |
|---|---|---|---|
| `adminEmail` | Entorno | `admin@plataforma-mooc.test` | Correo del Administrador Principal pre-sembrado. |
| `adminPassword` | Entorno | `Password123!` | Contraseña del Administrador Principal. |
| `adminPrimaryId` | Entorno | `a0000000-0000-0000-0000-000000000001` | UUID del Administrador Principal para prueba de último admin. |
| `adminSecondaryEmail` | Entorno | `admin.secundario@plataforma-mooc.test` | Correo del Administrador Secundario pre-sembrado. |
| `adminSecondaryId` | Entorno | `a0000000-0000-0000-0000-000000000002` | UUID del Administrador Secundario para suspender/reactivar. |
| `studentEmail` | Entorno | `estudiante1@plataforma-mooc.test` | Correo de usuario no-admin (rol `estudiante`) pre-sembrado. |
| `studentPassword` | Entorno | `Password123!` | Contraseña de usuario no-admin. |
| `targetStudentId` | Entorno | `c0000000-0000-0000-0000-000000000001` | UUID del estudiante objetivo para cambios de rol y estado. |

### 2.3 Variables Dinámicas (Asignadas durante la Ejecución)
| Variable | Colección | Propósito |
|---|---|---|
| `testEmail` | Identidad | Correo determinístico-único por corrida (`postman-<timestamp>@example.test`) generado en el paso 01. |
| `verificationToken` | Identidad | Token de activación extraído del cuerpo del correo vía API REST de Mailpit. |
| `messageId` | Identidad | ID del mensaje en Mailpit obtenido tras consultar `GET /api/v1/search?query=to:<correo>`. |
| `sessionToken` | Identidad | Token Bearer emitido tras inicio de sesión exitoso. |
| `sessionId` | Identidad | UUID de la sesión activa obtenido al listar `GET /api/v1/auth/sessions`. |
| `resetToken` | Identidad | Token de recuperación de contraseña extraído de Mailpit. |
| `mensajeGenerico` | Identidad | Mensaje de recuperación (202 Accepted) para verificar respuestas idénticas contra cuentas inexistentes. |
| `idemEmail`, `idemKey`, `idemBody`, `idemUserId` | Identidad | Variables para pruebas completas del encabezado `Idempotency-Key`. |
| `adminToken` | Administración | Token Bearer del Administrador Principal emitido en el login administrativo. |
| `studentToken` | Identidad / Admón | Token Bearer del Estudiante para verificar rechazos RBAC `403 Forbidden`. |

---

## 3. Cobertura de Criterios de Aceptación

### 3.1 Subsistema de Identidad, Autenticación y Seguridad (#24)

#### A. Flujos Funcionales de Identidad (Criterio 1)
1. **Registro:** `POST /api/v1/auth/register` crea usuario en rol `estudiante` y estado `pending_verification`.
2. **Verificación de correo:** Consulta automática de Mailpit vía REST, extracción de token y activación con `GET /api/v1/auth/verify?token=...` cambiando estado a `active`.
3. **Login:** `POST /api/v1/auth/login` valida credenciales y entrega token Bearer junto con su expiración (`expires_at`), sin filtrar contraseñas ni hashes en el cuerpo.
4. **Revocación de sesión por ID:** `DELETE /api/v1/auth/sessions/{sessionId}` revoca específicamente la sesión por su identificador único (204 No Content), invalidando de inmediato su uso posterior (401 Unauthorized).
5. **Logout:** `POST /api/v1/auth/logout` revoca la sesión del llamante (204 No Content) con invalidación inmediata (401 Unauthorized).
6. **Recuperación de contraseña:** `POST /api/v1/auth/password/forgot` solicita enlace; Mailpit entrega el correo; `POST /api/v1/auth/password/reset` actualiza las credenciales, invalidando de inmediato todas las sesiones activas del usuario.

#### B. Casos Negativos Obligatorios (Criterio 2)
1. **Credenciales inválidas:**
   - Contraseña errónea en login -> `401 Unauthorized` (`code: invalid_credentials`).
   - Correo no registrado en login -> `401 Unauthorized` (`code: invalid_credentials`), con mensaje idéntico al de contraseña errónea para evitar enumeración de usuarios.
   - Contraseña anterior tras un cambio exitoso -> `401 Unauthorized` (`code: invalid_credentials`).
2. **Token expirado / inválido:**
   - Verificación de correo con token expirado -> `400 Bad Request` (`code: invalid_token`).
   - Reutilización de token de verificación ya consumido -> `400 Bad Request` (`code: invalid_token`).
   - Reseteo de contraseña con token expirado -> `400 Bad Request` (`code: invalid_token`).
   - Reutilización de enlace de reseteo ya consumido -> `400 Bad Request` (`code: invalid_token`).
3. **Intento de crear profesor por registro público:**
   - Petición a `POST /api/v1/auth/register` con campo `"role": "profesor"` -> Rechazada con `400 Bad Request` (`code: invalid_body`), garantizando que la autogestión sólo permite estudiantes.
4. **Otros casos de frontera y seguridad:**
   - Registro duplicado con variaciones de mayúsculas/minúsculas -> `409 Conflict` (`code: email_already_registered`).
   - Intento de login antes de verificar correo -> `403 Forbidden` (`code: email_not_verified`).
   - Petición sin token a endpoint protegido -> `401 Unauthorized` con cabecera `WWW-Authenticate: Bearer`.
   - Recuperación con correo inexistente -> `202 Accepted` con mensaje idéntico al real (anti-enumeración).
   - Reseteo con contraseña corta (<8 caracteres) -> `400 Bad Request` (`code: invalid_input`) sin quemar el token de recuperación.
   - Intento de un estudiante de acceder a rutas administrativas -> `403 Forbidden` (`code: forbidden`).
   - Reutilización de `Idempotency-Key` con payload distinto -> `422 Unprocessable Entity` (`code: idempotency_key_reused`).
   - Agotamiento de tasa de login -> `429 Too Many Requests` (`code: rate_limit_exceeded`) con `Retry-After` y `RateLimit-Remaining: 0`.

---

### 3.2 Subsistema de Administración y Control de Acceso (#25)

#### A. Flujos Funcionales de Administración (Criterio 1)
1. **Listar usuarios y paginación:** `GET /api/v1/admin/users?limit=10` devuelve listado paginado con estructura uniforme (`items`, `pagination.limit`, `pagination.has_more`).
2. **Filtro por rol:** `GET /api/v1/admin/users?role=profesor` valida que todos los registros retornados posean rol `profesor`.
3. **Filtro por estado:** `GET /api/v1/admin/users?status=active` valida que todos los registros retornados se encuentren activos.
4. **Consulta detallada por ID:** `GET /api/v1/admin/users/{userID}` verifica la entrega del recurso completo, ausencia de campos sensibles y cabecera HTTP `ETag` para control de concurrencia optimista.
5. **Cambiar rol:** `PATCH /api/v1/admin/users/{userID}/role` cambia rol de `estudiante` a `profesor` con verificación de persistencia subsiguiente, y luego restaura limpiamente el rol original a `estudiante`.
6. **Suspender y reactivar cuentas:** `PATCH /api/v1/admin/users/{userID}/status` suspende la cuenta a `suspended`, verifica persistencia, y la reactiva inmediatamente a `active` verificando persistencia.

#### B. Casos Negativos de Control de Acceso RBAC (Criterio 2)
1. **No-admin intenta listar usuarios:** Un estudiante autenticado (`studentToken`) enviando `GET /api/v1/admin/users` es rechazado con `403 Forbidden` y sobre JSON `{"code": "forbidden"}`.
2. **No-admin intenta consultar usuario:** Petición `GET /api/v1/admin/users/{userID}` con `studentToken` es rechazada con `403 Forbidden` (`code: forbidden`).
3. **No-admin intenta cambiar rol:** Petición `PATCH /api/v1/admin/users/{userID}/role` con `studentToken` es rechazada con `403 Forbidden` (`code: forbidden`).
4. **No-admin intenta suspender cuenta:** Petición `PATCH /api/v1/admin/users/{userID}/status` con `studentToken` es rechazada con `403 Forbidden` (`code: forbidden`).
5. **Petición anónima (sin autenticación):** Petición sin cabecera `Authorization` a `GET /api/v1/admin/users` es rechazada con `401 Unauthorized`, código `unauthorized` y cabecera estándar `WWW-Authenticate: Bearer`.

#### C. Casos Borde: Protección del Último Administrador Activo (Criterio 3)
1. **Aislamiento controlado del último admin:** Mediante `PATCH /api/v1/admin/users/{adminSecondaryId}/status`, se suspende temporalmente al administrador secundario pre-sembrado, dejando en la base de datos exactamente un (1) administrador activo (`a0000000-0000-0000-0000-000000000001`).
2. **Intento de suspender al último administrador activo:** Petición a `PATCH /api/v1/admin/users/{adminPrimaryId}/status` con `status: "suspended"` es terminantemente rechazada por la capa de dominio con `409 Conflict` y código `"last_admin_protected"`, protegiendo la plataforma contra lockout irreversible.
3. **Intento de degradar al último administrador activo:** Petición a `PATCH /api/v1/admin/users/{adminPrimaryId}/role` con `role: "profesor"` o `role: "estudiante"` es igualmente rechazada con `409 Conflict` y código `"last_admin_protected"`.
4. **Restauración y limpieza limpia:** Se reactiva inmediatamente al administrador secundario con `PATCH /api/v1/admin/users/{adminSecondaryId}/status` (`status: "active"`), y se verifica con `GET /api/v1/admin/users?role=administrador` que ambos administradores permanecen activos, garantizando la repetibilidad e idempotencia de corridas subsiguientes.

---

## 4. Matrices de Peticiones y Aserciones Automáticas

### 4.1 Colección de Identidad y Seguridad (#24) — 44 Peticiones / 110 Aserciones

| # | Petición | Método y Ruta | HTTP | Aserción de Forma de Respuesta | Naturaleza |
|---|---|---|:---:|---|---|
| **00** | Health | `GET /api/v1/health` | 200 | `status == "pass"`, `services.database == "up"` | Positiva |
| **01** | Registro crea estudiante | `POST /api/v1/auth/register` | 201 | `user.role == "estudiante"`, `user.status == "pending_verification"`, sin contraseña | Positiva |
| **02** | Rechazo de rol profesor | `POST /api/v1/auth/register` | 400 | `code == "invalid_body"`, `message` presente | Negativa |
| **03** | Registro duplicado | `POST /api/v1/auth/register` | 409 | `code == "email_already_registered"` | Negativa |
| **04** | Login antes de verificar | `POST /api/v1/auth/login` | 403 | `code == "email_not_verified"` | Negativa |
| **05** | Mailpit: buscar correo activación | `GET /api/v1/search?query=...` | 200 | `messages.length >= 1`, `Subject` contiene "Confirma tu cuenta" | Auxiliar |
| **06** | Mailpit: extraer token activación | `GET /api/v1/message/{id}` | 200 | Regex coincide, enlace incluye `/api/v1/auth/verify?token=` | Auxiliar |
| **07** | Rechazo de token expirado (verif) | `GET /api/v1/auth/verify?token=...` | 400 | `code == "invalid_token"` | Negativa |
| **08** | Verificar activa cuenta | `GET /api/v1/auth/verify?token=...` | 200 | `user.status == "active"`, `message` presente | Positiva |
| **09** | Enlace de verificación un solo uso | `GET /api/v1/auth/verify?token=...` | 400 | `code == "invalid_token"` | Negativa |
| **10** | Login exitoso tras verificar | `POST /api/v1/auth/login` | 200 | `token` no vacío, `expires_at` cadena, sin contraseña | Positiva |
| **11** | Login contraseña errónea | `POST /api/v1/auth/login` | 401 | `code == "invalid_credentials"` | Negativa |
| **12** | Login correo inexistente | `POST /api/v1/auth/login` | 401 | `code == "invalid_credentials"`, idéntico a contraseña errónea | Negativa |
| **13** | Endpoint sesiones sin token | `GET /api/v1/auth/sessions` | 401 | Cabecera `WWW-Authenticate: Bearer`, `code == "unauthorized"` | Negativa |
| **14** | Endpoint sesiones con token | `GET /api/v1/auth/sessions` | 200 | `items` arreglo con longitud >= 1, sin `token_hash` | Positiva |
| **15** | Revocación de sesión por ID | `DELETE /api/v1/auth/sessions/{id}` | 204 | Cuerpo estrictamente vacío | Positiva |
| **16** | Verificación revocación inmediata (ID) | `GET /api/v1/auth/sessions` | 401 | `code == "unauthorized"` | Negativa |
| **17** | Login para sesión de logout | `POST /api/v1/auth/login` | 200 | `token` no vacío, `expires_at` cadena | Positiva |
| **18** | Logout de sesión | `POST /api/v1/auth/logout` | 204 | Cuerpo estrictamente vacío | Positiva |
| **19** | Verificación revocación inmediata (logout) | `GET /api/v1/auth/sessions` | 401 | `code == "unauthorized"` | Negativa |
| **20** | Reenvío de verificación silencioso | `POST /api/v1/auth/verify/resend` | 202 | `message` genérico ("Si la cuenta existe...") | Negativa |
| **21** | Login previo a recuperación | `POST /api/v1/auth/login` | 200 | `token` no vacío, `expires_at` cadena | Positiva |
| **22** | Solicitar enlace recuperación | `POST /api/v1/auth/password/forgot` | 202 | `message` genérico no vacío | Positiva |
| **23** | Recuperación correo inexistente | `POST /api/v1/auth/password/forgot` | 202 | `message` idéntico al de cuenta existente | Negativa |
| **24** | Mailpit: buscar correo reset | `GET /api/v1/search?query=...` | 200 | `messages.length >= 1`, `Subject` contiene "Restablece" | Auxiliar |
| **25** | Mailpit: extraer token reset | `GET /api/v1/message/{id}` | 200 | Regex coincide, enlace incluye `/password/reset?token=` | Auxiliar |
| **26** | Rechazo de token expirado (reset) | `POST /api/v1/auth/password/reset` | 400 | `code == "invalid_token"` | Negativa |
| **27** | Reset con contraseña corta | `POST /api/v1/auth/password/reset` | 400 | `code == "invalid_input"`, token no consumido | Negativa |
| **28** | Reset con contraseña válida | `POST /api/v1/auth/password/reset` | 204 | Cuerpo estrictamente vacío | Positiva |
| **29** | Sesión previa invalidada | `GET /api/v1/auth/sessions` | 401 | `code == "unauthorized"` | Negativa |
| **30** | Login con contraseña vieja | `POST /api/v1/auth/login` | 401 | `code == "invalid_credentials"` | Negativa |
| **31** | Login con contraseña nueva | `POST /api/v1/auth/login` | 200 | `token` no vacío, `expires_at` cadena | Positiva |
| **32** | Token reset de un solo uso | `POST /api/v1/auth/password/reset` | 400 | `code == "invalid_token"` | Negativa |
| **33** | Login sesión estudiante (Admin) | `POST /api/v1/auth/login` | 200 | `user.role == "estudiante"`, `token` emitido | Positiva |
| **34** | Admin: listar usuarios sin token | `GET /api/v1/admin/users` | 401 | Cabecera `WWW-Authenticate: Bearer`, `code == "unauthorized"` | Negativa |
| **35** | Admin: listar usuarios como estudiante | `GET /api/v1/admin/users` | 403 | `code == "forbidden"` | Negativa |
| **36** | Admin: cambiar rol como estudiante | `PATCH /api/v1/admin/users/{id}/role` | 403 | `code == "forbidden"` | Negativa |
| **37** | Admin: suspender cuenta como estudiante | `PATCH /api/v1/admin/users/{id}/status` | 403 | `code == "forbidden"` | Negativa |
| **38** | Idempotencia: primer registro | `POST /api/v1/auth/register` | 201 | Cabecera `Idempotent-Replay` ausente, `user.id` emitido | Positiva |
| **39** | Idempotencia: reenvío idéntico | `POST /api/v1/auth/register` | 201 | `Idempotent-Replay: true`, cuerpo y `user.id` idénticos | Positiva |
| **40** | Idempotencia: payload alterado | `POST /api/v1/auth/register` | 422 | `code == "idempotency_key_reused"` | Negativa |
| **41** | Idempotencia: misma clave otra ruta | `POST /api/v1/auth/password/forgot` | 202 | Cabecera `Idempotent-Replay` ausente, `message` válido | Positiva |
| **42** | Sin clave: petición duplicada | `POST /api/v1/auth/register` | 409 | `code == "email_already_registered"` | Negativa |
| **43** | Seguridad: Rate limit login | `GET /api/v1/health` + login loop | 429 | `code == "rate_limit_exceeded"`, `Retry-After > 0`, `RateLimit-Remaining == "0"` | Negativa |

---

### 4.2 Colección de Administración (#25) — 23 Peticiones / 46 Aserciones

| # | Petición | Método y Ruta | HTTP | Aserción de Forma de Respuesta | Naturaleza |
|---|---|---|:---:|---|---|
| **01** | Login como Administrador Principal | `POST /api/v1/auth/login` | 200 | `token` Bearer obtenido, `user.role == "administrador"`, `expires_at` | Positiva |
| **02** | Listar usuarios (Paginación y Estructura) | `GET /api/v1/admin/users?limit=10` | 200 | `items` es arreglo con longitud >= 1, `pagination.limit == 10`, `pagination.has_more` booleano | Positiva |
| **03** | Filtrar usuarios por rol (profesor) | `GET /api/v1/admin/users?role=profesor` | 200 | `items` contiene elementos y todos tienen `role == "profesor"` | Positiva |
| **04** | Filtrar usuarios por estado (active) | `GET /api/v1/admin/users?status=active` | 200 | `items` contiene elementos y todos tienen `status == "active"` | Positiva |
| **05** | Consultar detalle de usuario por ID | `GET /api/v1/admin/users/{targetStudentId}` | 200 | Cabecera `ETag` presente, `user.id == targetStudentId`, `user.email` coincide | Positiva |
| **06** | Cambiar rol de estudiante a profesor | `PATCH /api/v1/admin/users/{targetStudentId}/role` | 200 | `user.role == "profesor"`, `message` confirma cambio de rol | Positiva |
| **07** | Verificar persistencia del nuevo rol | `GET /api/v1/admin/users/{targetStudentId}` | 200 | `user.role == "profesor"`, datos íntegros | Positiva |
| **08** | Restaurar rol de profesor a estudiante | `PATCH /api/v1/admin/users/{targetStudentId}/role` | 200 | `user.role == "estudiante"`, `message` confirma restauración | Positiva |
| **09** | Suspender cuenta de estudiante | `PATCH /api/v1/admin/users/{targetStudentId}/status` | 200 | `user.status == "suspended"`, `message` confirma suspensión | Positiva |
| **10** | Verificar que el estudiante quedó suspendido | `GET /api/v1/admin/users/{targetStudentId}` | 200 | `user.status == "suspended"` persistido en base de datos | Positiva |
| **11** | Reactivar cuenta de estudiante | `PATCH /api/v1/admin/users/{targetStudentId}/status` | 200 | `user.status == "active"`, `message` confirma activación | Positiva |
| **12** | Verificar que el estudiante quedó activo | `GET /api/v1/admin/users/{targetStudentId}` | 200 | `user.status == "active"` persistido en base de datos | Positiva |
| **13** | Login como usuario no-admin (Estudiante) | `POST /api/v1/auth/login` | 200 | `studentToken` emitido, `user.role == "estudiante"` | Positiva |
| **14** | No-admin intenta listar usuarios | `GET /api/v1/admin/users` | 403 | `code == "forbidden"`, mensaje rechaza acceso por falta de permisos | Negativa |
| **15** | No-admin intenta consultar usuario por ID | `GET /api/v1/admin/users/{targetStudentId}` | 403 | `code == "forbidden"`, rechazo RBAC | Negativa |
| **16** | No-admin intenta cambiar un rol | `PATCH /api/v1/admin/users/{targetStudentId}/role` | 403 | `code == "forbidden"`, rechazo RBAC | Negativa |
| **17** | No-admin intenta suspender una cuenta | `PATCH /api/v1/admin/users/{targetStudentId}/status` | 403 | `code == "forbidden"`, rechazo RBAC | Negativa |
| **18** | Petición sin autenticación a listar usuarios | `GET /api/v1/admin/users` | 401 | Cabecera `WWW-Authenticate: Bearer`, `code == "unauthorized"` | Negativa |
| **19** | Suspender al administrador secundario | `PATCH /api/v1/admin/users/{adminSecondaryId}/status` | 200 | `user.status == "suspended"`, deja exactamente 1 admin activo | Positiva |
| **20** | Intento de suspender al último admin activo | `PATCH /api/v1/admin/users/{adminPrimaryId}/status` | 409 | `code == "last_admin_protected"`, mensaje protector explícito | Borde |
| **21** | Intento de degradar rol del último admin activo | `PATCH /api/v1/admin/users/{adminPrimaryId}/role` | 409 | `code == "last_admin_protected"`, mensaje protector explícito | Borde |
| **22** | Reactivar al administrador secundario | `PATCH /api/v1/admin/users/{adminSecondaryId}/status` | 200 | `user.status == "active"`, restaura 2 administradores activos | Positiva |
| **23** | Verificar que ambos administradores continúan activos | `GET /api/v1/admin/users?role=administrador` | 200 | `items.length >= 2`, todos con `status == "active"` | Positiva |

---

## 5. Instrucciones de Ejecución

### Opción 1: Ejecución Automatizada con Makefile / Docker (Recomendada para CI y Evaluación)

Asegúrate de que la infraestructura esté levantada con los datos sintéticos sembrados:
```bash
docker compose up -d
make seed
```

#### Ejecutar ambas suites secuencialmente (Identidad + Administración):
```bash
make test-postman
```

#### Ejecutar únicamente la suite de Administración (#25):
```bash
make test-postman-admin
```

O directamente mediante Docker con la imagen oficial de Newman:
```bash
docker run --rm --network plataforma-mooc_default   -v "$(pwd)/docs/postman":/etc/newman postman/newman:alpine   run /etc/newman/collection_admin.postman_collection.json   -e /etc/newman/mooc_docker.postman_environment.json
```

**Resultado esperado para Administración (#25):**
- 23 peticiones HTTP ejecutadas.
- 23 scripts de prueba evaluados.
- 46 aserciones superadas con **0 fallos**.

#### Ejecutar únicamente la suite de Identidad (#24):
```bash
make test-postman-identity
```

---

### Opción 2: Ejecución en Postman Desktop

1. Abre **Postman Desktop**.
2. Haz clic en **Import** y selecciona los archivos correspondientes:
   - Colección: `docs/postman/collection_admin.postman_collection.json` (y/o `collection_api.postman_collection.json`).
   - Entorno: `docs/postman/mooc_local.postman_environment.json`.
3. En la esquina superior derecha, selecciona el entorno activo: **Plataforma MOOC - Local**.
4. Abre la colección **Plataforma MOOC - Administracion** y entra a la pestaña **Runner** (o haz clic en el botón *Run Collection*).
5. Selecciona ejecutar todas las peticiones en el orden original.
6. Haz clic en **Run Plataforma MOOC - Administracion**.

> [!NOTE]
> La colección de administración preserva la idempotencia del sistema: tras suspender temporalmente al administrador secundario para forzar y certificar el rechazo `409 Conflict` (`last_admin_protected`), la petición 22 lo reactiva automáticamente, dejando la base de datos en su estado determinístico original.
