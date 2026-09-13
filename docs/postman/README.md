# Colecciones de Postman — Identidad y Seguridad

Este documento describe la suite de pruebas automatizadas en **Postman / Newman** para el subsistema de **Identidad, Autenticación y Control de Acceso** de la Plataforma MOOC (Issue #24).

La suite cumple estrictamente con los criterios de evaluación de la **Sección 9**, los flujos críticos de la **Sección 10.2** del pliego de condiciones y los estándares de diseño y seguridad de [`PROJECT_KEY_ASPECTS.md`](../../PROJECT_KEY_ASPECTS.md).

---

## 1. Estructura y Componentes

La carpeta `docs/postman/` contiene los siguientes artefactos:

| Archivo | Descripción |
|---|---|
| [`collection_api.postman_collection.json`](./collection_api.postman_collection.json) | Colección v2.1 de Postman con 44 peticiones organizadas secuencialmente, con pre-request scripts y tests automáticos. |
| [`mooc_local.postman_environment.json`](./mooc_local.postman_environment.json) | Entorno parametrizado para ejecuciones desde Postman Desktop en el host (`http://localhost:8080` y `http://localhost:8025`). |
| [`mooc_docker.postman_environment.json`](./mooc_docker.postman_environment.json) | Entorno parametrizado para ejecuciones automáticas en red Docker Compose (`http://api:8080` y `http://mailpit:8025`). |
| [`README.md`](./README.md) | Documentación técnica detallada, mapa de aserciones y guía de ejecución. |

---

## 2. Variables de Entorno y Parametrización

Ninguna URL, token de sesión ni identificador se encuentra hardcodeado. Todas las peticiones utilizan variables de entorno o variables de colección que se pueblan dinámicamente en tiempo de ejecución:

| Variable | Tipo | Propósito / Valor por Defecto |
|---|---|---|
| `baseUrl` | Entorno | URL base de la API REST (`http://localhost:8080` en local, `http://api:8080` en Docker). |
| `mailpitUrl` | Entorno | URL base de la API REST de Mailpit (`http://localhost:8025` en local, `http://mailpit:8025` en Docker). |
| `testPassword` | Entorno | Contraseña unificada de prueba para flujos de registro y login inicial (`Password123!`). |
| `newPassword` | Entorno | Nueva contraseña para validación de recuperación y cambio de credenciales (`NewPassword123!`). |
| `expiredToken` | Entorno | Token expirado/inválido para pruebas negativas de verificación y reseteo (`expired-token-sample-00000000`). |
| `testEmail` | Dinámica | Correo determinístico-único por corrida (`postman-<timestamp>@example.test`) generado en el paso 01. |
| `verificationToken` | Dinámica | Token de activación extraído del cuerpo del correo vía API REST de Mailpit (regex `token=([A-Za-z0-9_-%]+)`). |
| `messageId` | Dinámica | ID del mensaje en Mailpit obtenido tras consultar `GET /api/v1/search?query=to:<correo>`. |
| `sessionToken` | Dinámica | Token Bearer emitido tras inicio de sesión exitoso. |
| `sessionId` | Dinámica | UUID de la sesión activa obtenido al listar `GET /api/v1/auth/sessions`. |
| `resetToken` | Dinámica | Token de recuperación de contraseña extraído de Mailpit. |
| `mensajeGenerico` | Dinámica | Mensaje genérico de recuperación (202 Accepted) para verificar respuestas idénticas contra cuentas inexistentes. |
| `studentToken` | Dinámica | Token de estudiante autenticado utilizado para verificar restricciones 403 en endpoints administrativos. |
| `idemEmail` | Dinámica | Correo único generado para las pruebas del encabezado `Idempotency-Key`. |
| `idemKey` | Dinámica | Clave UUID/aleatoria de idempotencia para verificar deduplicación y reintento. |
| `idemBody` | Dinámica | Cuerpo grabado de la primera petición para verificar respuesta idéntica en reenvío. |
| `idemUserId` | Dinámica | ID del usuario creado en la primera petición idempotente. |

---

## 3. Cobertura de Criterios de Aceptación

### A. Flujos Funcionales de Identidad (Criterio 1)
1. **Registro:** `POST /api/v1/auth/register` crea usuario en rol `estudiante` y estado `pending_verification`.
2. **Verificación de correo:** Consulta automática de Mailpit vía REST, extracción de token y activación con `GET /api/v1/auth/verify?token=...` cambiando estado a `active`.
3. **Login:** `POST /api/v1/auth/login` valida credenciales y entrega token Bearer junto con su expiración (`expires_at`), sin filtrar contraseñas ni hashes en el cuerpo.
4. **Revocación de sesión por ID:** `DELETE /api/v1/auth/sessions/{sessionId}` revoca específicamente la sesión por su identificador único (204 No Content), invalidando de inmediato su uso posterior (401 Unauthorized).
5. **Logout:** `POST /api/v1/auth/logout` revoca la sesión del llamante (204 No Content) con invalidación inmediata (401 Unauthorized).
6. **Recuperación de contraseña:** `POST /api/v1/auth/password/forgot` solicita enlace; Mailpit entrega el correo; `POST /api/v1/auth/password/reset` actualiza las credenciales, invalidando de inmediato todas las sesiones activas del usuario.

### B. Casos Negativos Obligatorios (Criterio 2)
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

## 4. Matriz de Peticiones y Aserciones Automáticas

Cada una de las 44 peticiones cuenta con al menos una aserción de **código de estado HTTP** y una aserción sobre la **forma / estructura de la respuesta**:

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

## 5. Instrucciones de Ejecución

### Opción 1: Ejecución Automatizada con Makefile / Docker (Recomendada para CI y Evaluación)

Asegúrate de que la infraestructura esté levantada:
```bash
docker compose up -d
```

Ejecuta la suite con Newman dentro de la red Docker:
```bash
make test-postman
```

O directamente mediante el comando Docker equivalente:
```bash
docker run --rm --network plataforma-mooc_default \
  -v "$(pwd)/docs/postman":/etc/newman postman/newman:alpine \
  run /etc/newman/collection_api.postman_collection.json \
  -e /etc/newman/mooc_docker.postman_environment.json
```

**Resultado esperado:**
- 59 peticiones HTTP ejecutadas (incluye el bucle de rate limit).
- 44 scripts de prueba evaluados.
- 110 aserciones superadas con **0 fallos**.

### Opción 2: Ejecución Manual en Postman Desktop

1. Abre **Postman Desktop**.
2. Haz clic en **Import** y selecciona los archivos:
   - `docs/postman/collection_api.postman_collection.json`
   - `docs/postman/mooc_local.postman_environment.json`
3. En la esquina superior derecha, selecciona el entorno activo: **Plataforma MOOC - Local**.
4. Abre la colección **Plataforma MOOC - Identidad** y selecciona la pestaña **Runner** (o botón *Run Collection*).
5. Selecciona ejecutar todas las peticiones en el orden original.
6. Haz clic en **Run Plataforma MOOC - Identidad**.

> [!NOTE]
> La colección genera un correo único en cada corrida (`postman-<timestamp>@example.test`) e interactúa de forma transparente con la API REST de Mailpit para consultar y extraer tokens de verificación y recuperación sin intervención humana.

> [!WARNING]
> La última carpeta **Seguridad (#13)** agota intencionalmente la cuota de intentos de login (10 peticiones/minuto) para verificar el código `429 Too Many Requests`. Si se desea ejecutar la colección de forma inmediata consecutiva sin esperar el transcurso de un minuto, limpie las llaves de rate limiting en Redis:
> ```bash
> docker compose exec -T redis redis-cli EVAL "for _,k in ipairs(redis.call('keys','ratelimit:*')) do redis.call('del',k) end" 0
> ```
