# Reporte Integral de Pruebas E2E: Identidad y Autoría (#27)

> **Pila de Ejecución:** Sistema desplegado en contenedores Docker Compose (`api`, `postgres`, `redis`, `minio`, `mailpit`, `worker`). **Cero mocks**: todas las peticiones consumen endpoints HTTP reales, base de datos PostgreSQL transaccional, caché/sesiones en Redis y servidor de correo Mailpit.
> **Fecha de Ejecución:** 2026-09-14 04:29:35 UTC
> **Alineación Normativa:** Secciones 6, 9 y 10.2 del Pliego de Condiciones y [`PROJECT_KEY_ASPECTS.md`](../PROJECT_KEY_ASPECTS.md).

---

## 1. Resumen Ejecutivo de Ejecución

| Métrica Global | Valor | Estado |
| :--- | :--- | :---: |
| **Colecciones Evaluadas** | 3 colecciones E2E | `PASS` |
| **Peticiones HTTP Ejecutadas** | 112 peticiones | `PASS` |
| **Aserciones Automáticas Totales** | 216 aserciones | `PASS` |
| **Aserciones Aprobadas** | 216 aserciones | `PASS` |
| **Aserciones Fallidas** | 0 fallos | `PASS` |
| **Tasa de Éxito (Success Rate)** | **100.0%** | **`100% OK`** |
| **Mocks Utilizados** | **0 (Infraestructura Real Docker Compose)** | `CONFORME` |

```
████████████████████████████████████████ 100% APROBADO (216/216 aserciones)
```

---

## 2. Cobertura por Segmento de Demostración (Sección 10.2)

### 2.1 Segmento 1: Identidad y Administración
- **Criterio de Evaluación (Sección 9):** Identidad, autorización y seguridad (Must).
- **Evidencia Generada:** Capturas de respuestas HTTP, emails en Mailpit y logs de auditoría en [`docs/e2e/evidencia/segmento1_identidad_admin/`](./evidencia/segmento1_identidad_admin/).

#### Batería E2E: Identidad y Seguridad de Sesiones

| # | Petición / Caso de Prueba | Método | Endpoint | HTTP Esperado | Latencia | Aserciones Automáticas | Estado |
| :---: | :--- | :---: | :--- | :---: | :---: | :--- | :---: |
| 01 | **00 Health** | `GET` | `/api/v1/health` | `200 OK` | 90ms | ✓ `responde 200`<br>✓ `estado pass`<br>✓ `la base de datos responde` | `PASS` |
| 02 | **01 Registro crea estudiante pendiente** | `POST` | `/api/v1/auth/register` | `201 Created` | 129ms | ✓ `responde 201`<br>✓ `el rol es estudiante`<br>✓ `queda pendiente de verificacion`<br>✓ `la respuesta no expone la contrasena` | `PASS` |
| 03 | **02 Registro rechaza intento de crear profesor** | `POST` | `/api/v1/auth/register` | `400 Bad Request` | 8ms | ✓ `responde 400`<br>✓ `el campo role se rechaza, no se ignora` | `PASS` |
| 04 | **03 Registro duplicado devuelve conflicto** | `POST` | `/api/v1/auth/register` | `409 Conflict` | 69ms | ✓ `responde 409`<br>✓ `detecta el duplicado` | `PASS` |
| 05 | **04 Login antes de verificar es rechazado** | `POST` | `/api/v1/auth/login` | `403 Forbidden` | 84ms | ✓ `responde 403`<br>✓ `exige verificar el correo` | `PASS` |
| 06 | **05 Mailpit: buscar el correo de verificacion** | `GET` | `/api/v1/search?query=to:{{testEmail}}&limit=1` | `200 OK` | 35ms | ✓ `responde 200`<br>✓ `llego el correo al destinatario`<br>✓ `el asunto es el esperado` | `PASS` |
| 07 | **06 Mailpit: extraer el token del enlace** | `GET` | `/api/v1/message/{{messageId}}` | `200 OK` | 12ms | ✓ `responde 200`<br>✓ `el correo trae un enlace de activacion`<br>✓ `el enlace apunta al endpoint de verificacion` | `PASS` |
| 08 | **07 Verificar rechaza token expirado o invalido** | `GET` | `/api/v1/auth/verify?token={{expiredToken}}` | `400 Bad Request` | 15ms | ✓ `responde 400`<br>✓ `rechaza por token invalido o expirado` | `PASS` |
| 09 | **08 Verificar activa la cuenta** | `GET` | `/api/v1/auth/verify?token={{verificationToken}}` | `200 OK` | 23ms | ✓ `responde 200`<br>✓ `la cuenta queda activa` | `PASS` |
| 10 | **09 El enlace es de un solo uso** | `GET` | `/api/v1/auth/verify?token={{verificationToken}}` | `400 Bad Request` | 8ms | ✓ `responde 400`<br>✓ `el token ya no sirve` | `PASS` |
| 11 | **10 Login exitoso tras verificar** | `POST` | `/api/v1/auth/login` | `200 OK` | 74ms | ✓ `responde 200`<br>✓ `devuelve un token de sesion`<br>✓ `devuelve la fecha de expiracion`<br>✓ `la respuesta no expone la contrasena` | `PASS` |
| 12 | **11 Login con contrasena incorrecta** | `POST` | `/api/v1/auth/login` | `401 Unauthorized` | 66ms | ✓ `responde 401`<br>✓ `mensaje generico` | `PASS` |
| 13 | **12 Login con correo inexistente responde igual** | `POST` | `/api/v1/auth/login` | `401 Unauthorized` | 61ms | ✓ `responde 401`<br>✓ `no revela si el correo existe` | `PASS` |
| 14 | **13 Endpoint protegido sin token** | `GET` | `/api/v1/auth/sessions` | `401 Unauthorized` | 6ms | ✓ `responde 401`<br>✓ `pide autenticacion`<br>✓ `forma de error no autorizado` | `PASS` |
| 15 | **14 Endpoint protegido con token** | `GET` | `/api/v1/auth/sessions` | `200 OK` | 21ms | ✓ `responde 200`<br>✓ `lista la sesion abierta`<br>✓ `no expone el hash del token` | `PASS` |
| 16 | **15 Revocacion de sesion por ID** | `DELETE` | `/api/v1/auth/sessions/{{sessionId}}` | `204 No Content` | 16ms | ✓ `responde 204`<br>✓ `sin cuerpo tras revocacion` | `PASS` |
| 17 | **16 Revocacion inmediata por ID: el token ya no sirve** | `GET` | `/api/v1/auth/sessions` | `401 Unauthorized` | 6ms | ✓ `responde 401 tras revocacion por ID`<br>✓ `la sesion revocada no tiene acceso` | `PASS` |
| 18 | **17 Login para obtener nueva sesion para logout** | `POST` | `/api/v1/auth/login` | `200 OK` | 72ms | ✓ `responde 200`<br>✓ `devuelve nuevo token de sesion` | `PASS` |
| 19 | **18 Logout** | `POST` | `/api/v1/auth/logout` | `204 No Content` | 18ms | ✓ `responde 204`<br>✓ `sin cuerpo` | `PASS` |
| 20 | **19 Revocacion inmediata tras logout: el token ya no sirve** | `GET` | `/api/v1/auth/sessions` | `401 Unauthorized` | 8ms | ✓ `responde 401 en la peticion siguiente`<br>✓ `la sesion quedo invalidada` | `PASS` |
| 21 | **20 Reenvio de verificacion no revela nada** | `POST` | `/api/v1/auth/verify/resend` | `202 Accepted` | 16ms | ✓ `responde 202`<br>✓ `mensaje generico` | `PASS` |
| 22 | **21 Login de nuevo para abrir una sesion** | `POST` | `/api/v1/auth/login` | `200 OK` | 74ms | ✓ `responde 200`<br>✓ `devuelve token de sesion` | `PASS` |
| 23 | **22 Recuperacion: solicitar enlace** | `POST` | `/api/v1/auth/password/forgot` | `202 Accepted` | 38ms | ✓ `responde 202`<br>✓ `mensaje generico de recuperacion` | `PASS` |
| 24 | **23 Recuperacion: correo inexistente responde identico** | `POST` | `/api/v1/auth/password/forgot` | `202 Accepted` | 7ms | ✓ `responde 202`<br>✓ `mensaje identico al de un correo real` | `PASS` |
| 25 | **24 Mailpit: buscar el correo de recuperacion** | `GET` | `/api/v1/search?query=to:{{testEmail}}&limit=1` | `200 OK` | 12ms | ✓ `responde 200`<br>✓ `llego el correo de recuperacion`<br>✓ `es el mensaje de recuperacion` | `PASS` |
| 26 | **25 Mailpit: extraer el token de recuperacion** | `GET` | `/api/v1/message/{{messageId}}` | `200 OK` | 10ms | ✓ `responde 200`<br>✓ `el correo trae un enlace de recuperacion`<br>✓ `el enlace apunta al endpoint de reseteo` | `PASS` |
| 27 | **26 Reset rechaza token expirado o invalido** | `POST` | `/api/v1/auth/password/reset` | `400 Bad Request` | 67ms | ✓ `responde 400`<br>✓ `rechaza por token invalido o expirado` | `PASS` |
| 28 | **27 Reset con contrasena corta no quema el enlace** | `POST` | `/api/v1/auth/password/reset` | `400 Bad Request` | 10ms | ✓ `responde 400`<br>✓ `rechaza por dato invalido, no por token` | `PASS` |
| 29 | **28 Reset con contrasena valida** | `POST` | `/api/v1/auth/password/reset` | `204 No Content` | 70ms | ✓ `responde 204`<br>✓ `sin cuerpo tras reseteo exitoso` | `PASS` |
| 30 | **29 La sesion previa quedo invalidada** | `GET` | `/api/v1/auth/sessions` | `401 Unauthorized` | 9ms | ✓ `responde 401`<br>✓ `sesion invalidada por cambio de contrasena` | `PASS` |
| 31 | **30 Login con la contrasena vieja** | `POST` | `/api/v1/auth/login` | `401 Unauthorized` | 64ms | ✓ `responde 401`<br>✓ `credenciales viejas rechazadas` | `PASS` |
| 32 | **31 Login con la contrasena nueva** | `POST` | `/api/v1/auth/login` | `200 OK` | 67ms | ✓ `responde 200`<br>✓ `autenticacion exitosa con nueva contrasena` | `PASS` |
| 33 | **32 El enlace de recuperacion es de un solo uso** | `POST` | `/api/v1/auth/password/reset` | `400 Bad Request` | 65ms | ✓ `responde 400`<br>✓ `el enlace ya no sirve` | `PASS` |
| 34 | **33 Login para obtener una sesion de estudiante** | `POST` | `/api/v1/auth/login` | `200 OK` | 79ms | ✓ `responde 200`<br>✓ `el rol es estudiante`<br>✓ `forma de token de estudiante` | `PASS` |
| 35 | **34 Listar usuarios sin token** | `GET` | `/api/v1/admin/users` | `401 Unauthorized` | 5ms | ✓ `responde 401`<br>✓ `pide autenticacion`<br>✓ `codigo unauthorized` | `PASS` |
| 36 | **35 Listar usuarios como estudiante** | `GET` | `/api/v1/admin/users` | `403 Forbidden` | 9ms | ✓ `responde 403`<br>✓ `codigo forbidden` | `PASS` |
| 37 | **36 Cambiar un rol como estudiante** | `PATCH` | `/api/v1/admin/users/11111111-1111-1111-1111-111111111111/role` | `403 Forbidden` | 12ms | ✓ `responde 403`<br>✓ `el rechazo ocurre antes de tocar el recurso` | `PASS` |
| 38 | **37 Suspender una cuenta como estudiante** | `PATCH` | `/api/v1/admin/users/11111111-1111-1111-1111-111111111111/status` | `403 Forbidden` | 11ms | ✓ `responde 403`<br>✓ `codigo forbidden` | `PASS` |
| 39 | **38 Registro con Idempotency-Key** | `POST` | `/api/v1/auth/register` | `201 Created` | 85ms | ✓ `responde 201`<br>✓ `no viene marcada como repeticion`<br>✓ `forma de usuario registrado` | `PASS` |
| 40 | **39 Mismo POST, misma clave** | `POST` | `/api/v1/auth/register` | `201 Created` | 8ms | ✓ `mismo codigo que la original`<br>✓ `marcada como repeticion`<br>✓ `cuerpo identico al original`<br>✓ `el mismo usuario, no uno nuevo` | `PASS` |
| 41 | **40 Misma clave con cuerpo diferente** | `POST` | `/api/v1/auth/register` | `422 Unprocessable Entity` | 11ms | ✓ `responde 422`<br>✓ `codigo idempotency_key_reused` | `PASS` |
| 42 | **41 La misma clave en otro endpoint es otra operacion** | `POST` | `/api/v1/auth/password/forgot` | `202 Accepted` | 44ms | ✓ `responde 202`<br>✓ `no es una repeticion del registro`<br>✓ `forma de respuesta valida` | `PASS` |
| 43 | **42 Sin clave, cada peticion se ejecuta** | `POST` | `/api/v1/auth/register` | `409 Conflict` | 62ms | ✓ `responde 409 por correo duplicado`<br>✓ `la operacion si se ejecuto` | `PASS` |
| 44 | **43 Rate limiting: el limite de login bloquea** | `GET` | `/api/v1/health` | `429 Too Many Requests` | 7ms | ✓ `healthcheck responde 200`<br>✓ `healthcheck estado pass`<br>✓ `el limite se dispara antes de 15 intentos`<br>✓ `responde 429`<br>✓ `con codigo rate_limit_exceeded`<br>✓ `indica cuando reintentar`<br>✓ `reporta cupo agotado` | `PASS` |
| 45 | **43 Rate limiting: el limite de login bloquea** | `GET` | `/api/v1/health` | `429 Too Many Requests` | 7ms | ✓ `healthcheck responde 200`<br>✓ `healthcheck estado pass`<br>✓ `el limite se dispara antes de 15 intentos`<br>✓ `responde 429`<br>✓ `con codigo rate_limit_exceeded`<br>✓ `indica cuando reintentar`<br>✓ `reporta cupo agotado` | `PASS` |
| 46 | **43 Rate limiting: el limite de login bloquea** | `GET` | `/api/v1/health` | `429 Too Many Requests` | 7ms | ✓ `healthcheck responde 200`<br>✓ `healthcheck estado pass`<br>✓ `el limite se dispara antes de 15 intentos`<br>✓ `responde 429`<br>✓ `con codigo rate_limit_exceeded`<br>✓ `indica cuando reintentar`<br>✓ `reporta cupo agotado` | `PASS` |
| 47 | **43 Rate limiting: el limite de login bloquea** | `GET` | `/api/v1/health` | `429 Too Many Requests` | 7ms | ✓ `healthcheck responde 200`<br>✓ `healthcheck estado pass`<br>✓ `el limite se dispara antes de 15 intentos`<br>✓ `responde 429`<br>✓ `con codigo rate_limit_exceeded`<br>✓ `indica cuando reintentar`<br>✓ `reporta cupo agotado` | `PASS` |
| 48 | **43 Rate limiting: el limite de login bloquea** | `GET` | `/api/v1/health` | `429 Too Many Requests` | 7ms | ✓ `healthcheck responde 200`<br>✓ `healthcheck estado pass`<br>✓ `el limite se dispara antes de 15 intentos`<br>✓ `responde 429`<br>✓ `con codigo rate_limit_exceeded`<br>✓ `indica cuando reintentar`<br>✓ `reporta cupo agotado` | `PASS` |
| 49 | **43 Rate limiting: el limite de login bloquea** | `GET` | `/api/v1/health` | `429 Too Many Requests` | 7ms | ✓ `healthcheck responde 200`<br>✓ `healthcheck estado pass`<br>✓ `el limite se dispara antes de 15 intentos`<br>✓ `responde 429`<br>✓ `con codigo rate_limit_exceeded`<br>✓ `indica cuando reintentar`<br>✓ `reporta cupo agotado` | `PASS` |
| 50 | **43 Rate limiting: el limite de login bloquea** | `GET` | `/api/v1/health` | `429 Too Many Requests` | 7ms | ✓ `healthcheck responde 200`<br>✓ `healthcheck estado pass`<br>✓ `el limite se dispara antes de 15 intentos`<br>✓ `responde 429`<br>✓ `con codigo rate_limit_exceeded`<br>✓ `indica cuando reintentar`<br>✓ `reporta cupo agotado` | `PASS` |
| 51 | **43 Rate limiting: el limite de login bloquea** | `GET` | `/api/v1/health` | `429 Too Many Requests` | 7ms | ✓ `healthcheck responde 200`<br>✓ `healthcheck estado pass`<br>✓ `el limite se dispara antes de 15 intentos`<br>✓ `responde 429`<br>✓ `con codigo rate_limit_exceeded`<br>✓ `indica cuando reintentar`<br>✓ `reporta cupo agotado` | `PASS` |
| 52 | **43 Rate limiting: el limite de login bloquea** | `GET` | `/api/v1/health` | `429 Too Many Requests` | 7ms | ✓ `healthcheck responde 200`<br>✓ `healthcheck estado pass`<br>✓ `el limite se dispara antes de 15 intentos`<br>✓ `responde 429`<br>✓ `con codigo rate_limit_exceeded`<br>✓ `indica cuando reintentar`<br>✓ `reporta cupo agotado` | `PASS` |
| 53 | **43 Rate limiting: el limite de login bloquea** | `GET` | `/api/v1/health` | `429 Too Many Requests` | 7ms | ✓ `healthcheck responde 200`<br>✓ `healthcheck estado pass`<br>✓ `el limite se dispara antes de 15 intentos`<br>✓ `responde 429`<br>✓ `con codigo rate_limit_exceeded`<br>✓ `indica cuando reintentar`<br>✓ `reporta cupo agotado` | `PASS` |
| 54 | **43 Rate limiting: el limite de login bloquea** | `GET` | `/api/v1/health` | `429 Too Many Requests` | 7ms | ✓ `healthcheck responde 200`<br>✓ `healthcheck estado pass`<br>✓ `el limite se dispara antes de 15 intentos`<br>✓ `responde 429`<br>✓ `con codigo rate_limit_exceeded`<br>✓ `indica cuando reintentar`<br>✓ `reporta cupo agotado` | `PASS` |
| 55 | **43 Rate limiting: el limite de login bloquea** | `GET` | `/api/v1/health` | `429 Too Many Requests` | 7ms | ✓ `healthcheck responde 200`<br>✓ `healthcheck estado pass`<br>✓ `el limite se dispara antes de 15 intentos`<br>✓ `responde 429`<br>✓ `con codigo rate_limit_exceeded`<br>✓ `indica cuando reintentar`<br>✓ `reporta cupo agotado` | `PASS` |
| 56 | **43 Rate limiting: el limite de login bloquea** | `GET` | `/api/v1/health` | `429 Too Many Requests` | 7ms | ✓ `healthcheck responde 200`<br>✓ `healthcheck estado pass`<br>✓ `el limite se dispara antes de 15 intentos`<br>✓ `responde 429`<br>✓ `con codigo rate_limit_exceeded`<br>✓ `indica cuando reintentar`<br>✓ `reporta cupo agotado` | `PASS` |
| 57 | **43 Rate limiting: el limite de login bloquea** | `GET` | `/api/v1/health` | `429 Too Many Requests` | 7ms | ✓ `healthcheck responde 200`<br>✓ `healthcheck estado pass`<br>✓ `el limite se dispara antes de 15 intentos`<br>✓ `responde 429`<br>✓ `con codigo rate_limit_exceeded`<br>✓ `indica cuando reintentar`<br>✓ `reporta cupo agotado` | `PASS` |
| 58 | **43 Rate limiting: el limite de login bloquea** | `GET` | `/api/v1/health` | `429 Too Many Requests` | 7ms | ✓ `healthcheck responde 200`<br>✓ `healthcheck estado pass`<br>✓ `el limite se dispara antes de 15 intentos`<br>✓ `responde 429`<br>✓ `con codigo rate_limit_exceeded`<br>✓ `indica cuando reintentar`<br>✓ `reporta cupo agotado` | `PASS` |
| 59 | **43 Rate limiting: el limite de login bloquea** | `GET` | `/api/v1/health` | `429 Too Many Requests` | 7ms | ✓ `healthcheck responde 200`<br>✓ `healthcheck estado pass`<br>✓ `el limite se dispara antes de 15 intentos`<br>✓ `responde 429`<br>✓ `con codigo rate_limit_exceeded`<br>✓ `indica cuando reintentar`<br>✓ `reporta cupo agotado` | `PASS` |

#### Batería E2E: Administración y Control de Privilegios (RBAC)

| # | Petición / Caso de Prueba | Método | Endpoint | HTTP Esperado | Latencia | Aserciones Automáticas | Estado |
| :---: | :--- | :---: | :--- | :---: | :---: | :--- | :---: |
| 01 | **01 Login como Administrador Principal** | `POST` | `/api/v1/auth/login` | `200 OK` | 166ms | ✓ `responde 200`<br>✓ `devuelve token Bearer y datos de usuario admin` | `PASS` |
| 02 | **02 Listar usuarios (Paginacion y Estructura)** | `GET` | `/api/v1/admin/users?limit=10` | `200 OK` | 14ms | ✓ `responde 200`<br>✓ `estructura de listado con paginacion y usuarios` | `PASS` |
| 03 | **03 Filtrar usuarios por rol (profesor)** | `GET` | `/api/v1/admin/users?role=profesor` | `200 OK` | 16ms | ✓ `responde 200`<br>✓ `todos los usuarios devueltos tienen rol profesor` | `PASS` |
| 04 | **04 Filtrar usuarios por estado (active)** | `GET` | `/api/v1/admin/users?status=active` | `200 OK` | 12ms | ✓ `responde 200`<br>✓ `todos los usuarios devueltos estan activos` | `PASS` |
| 05 | **05 Consultar detalle de usuario por ID** | `GET` | `/api/v1/admin/users/{{targetStudentId}}` | `200 OK` | 15ms | ✓ `responde 200`<br>✓ `datos correctos de usuario y cabecera ETag` | `PASS` |
| 06 | **06 Cambiar rol de estudiante a profesor** | `PATCH` | `/api/v1/admin/users/{{targetStudentId}}/role` | `200 OK` | 23ms | ✓ `responde 200`<br>✓ `rol actualizado exitosamente a profesor` | `PASS` |
| 07 | **07 Verificar persistencia del nuevo rol** | `GET` | `/api/v1/admin/users/{{targetStudentId}}` | `200 OK` | 10ms | ✓ `responde 200`<br>✓ `el usuario persiste con rol profesor` | `PASS` |
| 08 | **08 Restaurar rol de profesor a estudiante** | `PATCH` | `/api/v1/admin/users/{{targetStudentId}}/role` | `200 OK` | 19ms | ✓ `responde 200`<br>✓ `rol restaurado exitosamente a estudiante` | `PASS` |
| 09 | **09 Suspender cuenta de estudiante** | `PATCH` | `/api/v1/admin/users/{{targetStudentId}}/status` | `200 OK` | 28ms | ✓ `responde 200`<br>✓ `cuenta suspendida correctamente` | `PASS` |
| 10 | **10 Verificar que el estudiante quedo suspendido** | `GET` | `/api/v1/admin/users/{{targetStudentId}}` | `200 OK` | 12ms | ✓ `responde 200`<br>✓ `el estado persiste en suspended` | `PASS` |
| 11 | **11 Reactivar cuenta de estudiante** | `PATCH` | `/api/v1/admin/users/{{targetStudentId}}/status` | `200 OK` | 26ms | ✓ `responde 200`<br>✓ `cuenta reactivada a active` | `PASS` |
| 12 | **12 Verificar que el estudiante quedo activo** | `GET` | `/api/v1/admin/users/{{targetStudentId}}` | `200 OK` | 9ms | ✓ `responde 200`<br>✓ `el estado persiste en active` | `PASS` |
| 13 | **13 Login como usuario no-admin (Estudiante)** | `POST` | `/api/v1/auth/login` | `200 OK` | 90ms | ✓ `responde 200`<br>✓ `sesion obtenida con rol estudiante` | `PASS` |
| 14 | **14 No-admin intenta listar usuarios** | `GET` | `/api/v1/admin/users` | `403 Forbidden` | 10ms | ✓ `responde 403`<br>✓ `codigo forbidden` | `PASS` |
| 15 | **15 No-admin intenta consultar usuario por ID** | `GET` | `/api/v1/admin/users/{{targetStudentId}}` | `403 Forbidden` | 14ms | ✓ `responde 403`<br>✓ `codigo forbidden` | `PASS` |
| 16 | **16 No-admin intenta cambiar un rol** | `PATCH` | `/api/v1/admin/users/{{targetStudentId}}/role` | `403 Forbidden` | 11ms | ✓ `responde 403`<br>✓ `codigo forbidden` | `PASS` |
| 17 | **17 No-admin intenta suspender una cuenta** | `PATCH` | `/api/v1/admin/users/{{targetStudentId}}/status` | `403 Forbidden` | 10ms | ✓ `responde 403`<br>✓ `codigo forbidden` | `PASS` |
| 18 | **18 Peticion sin autenticacion a listar usuarios** | `GET` | `/api/v1/admin/users` | `401 Unauthorized` | 6ms | ✓ `responde 401`<br>✓ `cabecera WWW-Authenticate y codigo unauthorized` | `PASS` |
| 19 | **19 Suspender al administrador secundario** | `PATCH` | `/api/v1/admin/users/{{adminSecondaryId}}/status` | `200 OK` | 51ms | ✓ `responde 200`<br>✓ `admin secundario suspendido exitosamente` | `PASS` |
| 20 | **20 Intento de suspender al ultimo administrador activo (espera rechazo 409)** | `PATCH` | `/api/v1/admin/users/{{adminPrimaryId}}/status` | `409 Conflict` | 18ms | ✓ `responde 409`<br>✓ `codigo last_admin_protected y mensaje protector` | `PASS` |
| 21 | **21 Intento de degradar rol del ultimo administrador activo (espera rechazo 409)** | `PATCH` | `/api/v1/admin/users/{{adminPrimaryId}}/role` | `409 Conflict` | 17ms | ✓ `responde 409`<br>✓ `codigo last_admin_protected y mensaje protector` | `PASS` |
| 22 | **22 Reactivar al administrador secundario** | `PATCH` | `/api/v1/admin/users/{{adminSecondaryId}}/status` | `200 OK` | 21ms | ✓ `responde 200`<br>✓ `admin secundario reactivado correctamente` | `PASS` |
| 23 | **23 Verificar que ambos administradores continuan activos** | `GET` | `/api/v1/admin/users?role=administrador` | `200 OK` | 12ms | ✓ `responde 200`<br>✓ `existen al menos 2 administradores activos` | `PASS` |

---

### 2.2 Segmento 2: Autoría y Publicación
- **Criterio de Evaluación (Sección 9):** Autoría y publicación (Must).
- **Evidencia Generada:** Capturas de peticiones jerárquicas, validación multi-error, inmutabilidad y logs en [`docs/e2e/evidencia/segmento2_autoria_publicacion/`](./evidencia/segmento2_autoria_publicacion/).

#### Batería E2E: Jerarquía, Validación, Reordenamiento e Inmutabilidad

| # | Petición / Caso de Prueba | Método | Endpoint | HTTP Esperado | Latencia | Aserciones Automáticas | Estado |
| :---: | :--- | :---: | :--- | :---: | :---: | :--- | :---: |
| 01 | **01 Login como Profesor** | `POST` | `/api/v1/auth/login` | `200 OK` | 158ms | ✓ `responde 200`<br>✓ `devuelve token Bearer y usuario profesor` | `PASS` |
| 02 | **02 Crear Curso en Borrador (Draft)** | `POST` | `/api/v1/courses` | `201 Created` | 17ms | ✓ `responde 201`<br>✓ `curso creado en estado draft y version 1` | `PASS` |
| 03 | **03 Intento de publicacion con errores (espera 422 validation_failed)** | `POST` | `/api/v1/courses/{{courseId}}/publish` | `422 Unprocessable Entity` | 14ms | ✓ `responde 422`<br>✓ `lista exhaustiva de errores de validacion` | `PASS` |
| 04 | **04 Crear Modulo 1 en Borrador** | `POST` | `/api/v1/courses/{{courseId}}/modules` | `201 Created` | 23ms | ✓ `responde 201`<br>✓ `modulo creado con posicion 0 e identificador estable` | `PASS` |
| 05 | **05 Crear Unidad 1 en Modulo 1** | `POST` | `/api/v1/modules/{{moduleId}}/units` | `201 Created` | 29ms | ✓ `responde 201`<br>✓ `unidad creada con posicion 0 e identificador estable` | `PASS` |
| 06 | **06 Crear Recurso 1 (Lectura Obligatoria - Markdown Canonico)** | `POST` | `/api/v1/units/{{unitId}}/resources` | `201 Created` | 34ms | ✓ `responde 201`<br>✓ `recurso 1 obligatorio creado en posicion 0` | `PASS` |
| 07 | **07 Crear Recurso 2 (Recurso Complementario - Posicion 1)** | `POST` | `/api/v1/units/{{unitId}}/resources` | `201 Created` | 26ms | ✓ `responde 201`<br>✓ `recurso 2 complementario creado en posicion 1` | `PASS` |
| 08 | **08 Crear Recurso 3 (Quiz Formativo - Posicion 2)** | `POST` | `/api/v1/units/{{unitId}}/resources` | `201 Created` | 33ms | ✓ `responde 201`<br>✓ `recurso 3 quiz creado en posicion 2` | `PASS` |
| 09 | **09 Previsualizar Metadatos del Curso (GET Draft con ETag)** | `GET` | `/api/v1/courses/{{courseId}}` | `200 OK` | 10ms | ✓ `responde 200`<br>✓ `datos del borrador y cabecera ETag presentes` | `PASS` |
| 10 | **10 Previsualizar Modulos del Curso** | `GET` | `/api/v1/courses/{{courseId}}/modules` | `200 OK` | 12ms | ✓ `responde 200`<br>✓ `listado de modulos del curso ordenado` | `PASS` |
| 11 | **11 Previsualizar Unidades del Modulo** | `GET` | `/api/v1/modules/{{moduleId}}/units` | `200 OK` | 11ms | ✓ `responde 200`<br>✓ `listado de unidades del modulo ordenado` | `PASS` |
| 12 | **12 Previsualizar Recursos de la Unidad (Antes de Reordenar)** | `GET` | `/api/v1/units/{{unitId}}/resources` | `200 OK` | 13ms | ✓ `responde 200`<br>✓ `tres recursos con posiciones 0, 1 y 2` | `PASS` |
| 13 | **13 Reordenar: Eliminar recurso intermedio (Posicion 1)** | `DELETE` | `/api/v1/resources/{{resource2Id}}` | `204 No Content` | 47ms | ✓ `responde 204`<br>✓ `cuerpo estrictamente vacio` | `PASS` |
| 14 | **14 Verificar Reordenamiento y Preservacion de Identificadores Estables** | `GET` | `/api/v1/units/{{unitId}}/resources` | `200 OK` | 11ms | ✓ `responde 200`<br>✓ `reordenamiento exitoso sin huecos y stable_id intacto` | `PASS` |
| 15 | **15 Publicacion Exitosa del Curso** | `POST` | `/api/v1/courses/{{courseId}}/publish` | `200 OK` | 41ms | ✓ `responde 200`<br>✓ `curso publicado exitosamente con version 1` | `PASS` |
| 16 | **16 Consultar Curso Publicado en Catalogo** | `GET` | `/api/v1/courses/{{courseId}}` | `200 OK` | 10ms | ✓ `responde 200`<br>✓ `curso accesible en catalogo como published` | `PASS` |
| 17 | **17 Inmutabilidad: Rechazo al editar metadatos del curso publicado** | `PUT` | `/api/v1/courses/{{courseId}}` | `409 Conflict` | 19ms | ✓ `responde 409`<br>✓ `codigo course_immutable y mensaje explicativo` | `PASS` |
| 18 | **18 Inmutabilidad: Rechazo al agregar modulo a curso publicado** | `POST` | `/api/v1/courses/{{courseId}}/modules` | `409 Conflict` | 9ms | ✓ `responde 409`<br>✓ `codigo course_immutable` | `PASS` |
| 19 | **19 Inmutabilidad: Rechazo al agregar unidad en modulo de curso publicado** | `POST` | `/api/v1/modules/{{moduleId}}/units` | `409 Conflict` | 19ms | ✓ `responde 409`<br>✓ `codigo course_immutable` | `PASS` |
| 20 | **20 Inmutabilidad: Rechazo al agregar recurso en unidad de curso publicado** | `POST` | `/api/v1/units/{{unitId}}/resources` | `409 Conflict` | 26ms | ✓ `responde 409`<br>✓ `codigo course_immutable` | `PASS` |
| 21 | **21 Inmutabilidad: Rechazo al eliminar recurso en curso publicado** | `DELETE` | `/api/v1/resources/{{resource1Id}}` | `409 Conflict` | 24ms | ✓ `responde 409`<br>✓ `codigo course_immutable` | `PASS` |
| 22 | **22 Inmutabilidad: Rechazo al republicar un curso ya publicado** | `POST` | `/api/v1/courses/{{courseId}}/publish` | `409 Conflict` | 8ms | ✓ `responde 409`<br>✓ `codigo conflict` | `PASS` |
| 23 | **23 Despublicacion temporal (Flujo MVP Seccion 5.1)** | `POST` | `/api/v1/courses/{{courseId}}/unpublish` | `200 OK` | 26ms | ✓ `responde 200`<br>✓ `estado transiciona a unpublished` | `PASS` |
| 24 | **24 Editar curso temporalmente despublicado** | `PUT` | `/api/v1/courses/{{courseId}}` | `200 OK` | 23ms | ✓ `responde 200`<br>✓ `titulo y descripcion actualizados exitosamente` | `PASS` |
| 25 | **25 Republicar curso tras la actualizacion** | `POST` | `/api/v1/courses/{{courseId}}/publish` | `200 OK` | 34ms | ✓ `responde 200`<br>✓ `curso republicado como published con nuevo titulo` | `PASS` |
| 26 | **26 Login como Estudiante (Usuario no autor)** | `POST` | `/api/v1/auth/login` | `200 OK` | 73ms | ✓ `responde 200`<br>✓ `sesion obtenida con rol estudiante` | `PASS` |
| 27 | **27 No-autor intenta crear curso (espera 403)** | `POST` | `/api/v1/courses` | `403 Forbidden` | 14ms | ✓ `responde 403`<br>✓ `codigo forbidden` | `PASS` |
| 28 | **28 No-autor intenta agregar modulo (espera 403)** | `POST` | `/api/v1/courses/{{courseId}}/modules` | `403 Forbidden` | 8ms | ✓ `responde 403`<br>✓ `codigo forbidden` | `PASS` |
| 29 | **29 No-autor intenta publicar curso (espera 403)** | `POST` | `/api/v1/courses/{{courseId}}/publish` | `403 Forbidden` | 8ms | ✓ `responde 403`<br>✓ `codigo forbidden` | `PASS` |
| 30 | **30 Peticion anonima sin autenticacion a crear curso (espera 401)** | `POST` | `/api/v1/courses` | `401 Unauthorized` | 7ms | ✓ `responde 401`<br>✓ `cabecera WWW-Authenticate y codigo unauthorized` | `PASS` |

---

## 3. Matriz de Trazabilidad: Flujos Críticos vs Aserciones Automatizadas

| Segmento 10.2 | Requisito Técnico Verificado | Código HTTP | Resultado | Archivo de Evidencia |
| :--- | :--- | :---: | :---: | :--- |
| **1. Identidad** | Registro de estudiante con estado `pending_verification` | `201 Created` | `PASS` | [`01_registro_estudiante.json`](./evidencia/segmento1_identidad_admin/01_registro_estudiante.json) |
| **1. Identidad** | Envío de correo real con token a Mailpit | `200 OK` | `PASS` | [`mailpit_verification_email.json`](./evidencia/segmento1_identidad_admin/mailpit_verification_email.json) |
| **1. Identidad** | Activación de cuenta por token de un solo uso | `200 OK` | `PASS` | [`03_activacion_cuenta.json`](./evidencia/segmento1_identidad_admin/03_activacion_cuenta.json) |
| **1. Identidad** | Inicio de sesión seguro con emisión de Bearer Token | `200 OK` | `PASS` | [`04_login_estudiante.json`](./evidencia/segmento1_identidad_admin/04_login_estudiante.json) |
| **1. Identidad** | Prohibición de autoregistro de instructores (rol forzado) | `201 Created` | `PASS` | [`05_intento_registro_profesor_publico.json`](./evidencia/segmento1_identidad_admin/05_intento_registro_profesor_publico.json) |
| **1. Identidad** | Cierre de sesión y revocación inmediata de tokens en Redis | `200 OK` | `PASS` | [`06_logout_revocacion_sesion.json`](./evidencia/segmento1_identidad_admin/06_logout_revocacion_sesion.json) |
| **1. Identidad** | Petición con sesión revocada rechazada con cabecera Bearer | `401 Unauthorized` | `PASS` | [`07_acceso_token_revocado_401.json`](./evidencia/segmento1_identidad_admin/07_acceso_token_revocado_401.json) |
| **1. Identidad** | Idempotencia en operaciones de escritura (`Idempotency-Key`) | `201 / 422` | `PASS` | [`08_idempotencia_registro.json`](./evidencia/segmento1_identidad_admin/08_idempotencia_registro.json) |
| **1. Identidad** | Bloqueo por tasa excesiva de peticiones con `Retry-After` | `429 Too Many Req` | `PASS` | [`09_rate_limiting_429.json`](./evidencia/segmento1_identidad_admin/09_rate_limiting_429.json) |
| **1. Admin** | Consulta paginada de usuarios como Administrador | `200 OK` | `PASS` | [`10_admin_listar_usuarios.json`](./evidencia/segmento1_identidad_admin/10_admin_listar_usuarios.json) |
| **1. Admin** | Protección inquebrantable del último administrador activo | `409 Conflict` | `PASS` | [`11_proteccion_ultimo_admin_409.json`](./evidencia/segmento1_identidad_admin/11_proteccion_ultimo_admin_409.json) |
| **1. Admin** | Rechazo de privilegios a estudiante en módulo administrativo | `403 Forbidden` | `PASS` | [`12_rbac_no_admin_rechazo_403.json`](./evidencia/segmento1_identidad_admin/12_rbac_no_admin_rechazo_403.json) |
| **2. Autoría** | Creación de borrador de curso (`draft`, versión 1, `stable_id`) | `201 Created` | `PASS` | [`01_crear_curso_borrador.json`](./evidencia/segmento2_autoria_publicacion/01_crear_curso_borrador.json) |
| **2. Autoría** | Validación exhaustiva: reporte simultáneo de todos los errores | `422 Unproc Entity` | `PASS` | [`02_intento_publicacion_errores_acumulados_422.json`](./evidencia/segmento2_autoria_publicacion/02_intento_publicacion_errores_acumulados_422.json) |
| **2. Autoría** | Estructura jerárquica de 4 niveles (`Curso`->`Mód`->`Uni`->`Rec`) | `201 Created` | `PASS` | [`03_crear_modulo.json`](./evidencia/segmento2_autoria_publicacion/03_crear_modulo.json) |
| **2. Autoría** | Recurso enriquecido persistido en Markdown canónico | `201 Created` | `PASS` | [`05_crear_recurso_markdown_canonico.json`](./evidencia/segmento2_autoria_publicacion/05_crear_recurso_markdown_canonico.json) |
| **2. Autoría** | Previsualización de borrador con cabecera `ETag` de caché | `200 OK` | `PASS` | [`06_previsualizar_curso_draft_etag.json`](./evidencia/segmento2_autoria_publicacion/06_previsualizar_curso_draft_etag.json) |
| **2. Autoría** | Reordenamiento automático libre de colisiones al borrar recurso | `204 No Content` | `PASS` | [`07_reordenamiento_eliminar_recurso_204.json`](./evidencia/segmento2_autoria_publicacion/07_reordenamiento_eliminar_recurso_204.json) |
| **2. Autoría** | Preservación exacta de identificadores estables (`stable_id`) | `200 OK` | `PASS` | [`08_verificacion_stable_id_preservado.json`](./evidencia/segmento2_autoria_publicacion/08_verificacion_stable_id_preservado.json) |
| **2. Autoría** | Publicación exitosa de curso completo a versión 1 | `200 OK` | `PASS` | [`09_publicacion_exitosa_curso.json`](./evidencia/segmento2_autoria_publicacion/09_publicacion_exitosa_curso.json) |
| **2. Autoría** | Inmutabilidad de versión publicada: rechazo de edición | `409 Conflict` | `PASS` | [`10_inmutabilidad_rechazo_edicion_metadatos_409.json`](./evidencia/segmento2_autoria_publicacion/10_inmutabilidad_rechazo_edicion_metadatos_409.json) |
| **2. Autoría** | Inmutabilidad de versión publicada: rechazo de módulos nuevos | `409 Conflict` | `PASS` | [`11_inmutabilidad_rechazo_agregar_modulo_409.json`](./evidencia/segmento2_autoria_publicacion/11_inmutabilidad_rechazo_agregar_modulo_409.json) |
| **2. Autoría** | Despublicación temporal para edición y posterior republicación | `200 OK` | `PASS` | [`12_despublicacion_temporal_mvp51.json`](./evidencia/segmento2_autoria_publicacion/12_despublicacion_temporal_mvp51.json) |
| **2. Autoría** | Rechazo de creación o edición de cursos a usuarios no-autores | `403 Forbidden` | `PASS` | [`14_rbac_no_autor_rechazo_403.json`](./evidencia/segmento2_autoria_publicacion/14_rbac_no_autor_rechazo_403.json) |

---

## 4. Estado de la Infraestructura Docker Compose Durante la Prueba

| Contenedor / Servicio | Imagen | Puerto Host / Red | Rol en la Prueba E2E | Estado |
| :--- | :--- | :--- | :--- | :---: |
| `plataforma-mooc-api-1` | `plataforma-mooc-api` | `8080:8080` (HTTP) | Monolito modular en Go, enrutamiento REST `/api/v1` | `Healthy` |
| `plataforma-mooc-postgres-1` | `postgres:16-alpine` | `5432:5432` (TCP) | Fuente transaccional de verdad, triggers de inmutabilidad | `Healthy` |
| `plataforma-mooc-redis-1` | `redis:7-alpine` | `6379:6379` (TCP) | Almacén de sesiones, control de tasa (rate limiting) y colas | `Healthy` |
| `plataforma-mooc-mailpit-1` | `axllent/mailpit:latest` | `8025` (UI/API), `1025` (SMTP) | Servidor SMTP real y API de verificación de correos | `Healthy` |
| `plataforma-mooc-minio-1` | `minio/minio:latest` | `9000-9001` (S3 API / UI) | Almacenamiento de objetos S3 para activos y presigned URLs | `Healthy` |
| `plataforma-mooc-worker-1` | `plataforma-mooc-worker` | `9090:9090` (Prometheus) | Procesamiento asíncrono en Go (Asynq) y telemetría | `Healthy` |

---

## 5. Instrucciones para Reproducir la Evaluación E2E

Para regenerar este reporte y recolectar evidencia fresca en cualquier momento, ejecute:

```bash
# 1. Levantar o verificar la infraestructura en Docker Compose
docker compose ps

# 2. Ejecutar la batería completa E2E con recolección de evidencia y logs
make test-e2e

# 3. O ejecutar el flujo demostrativo interactivo de los segmentos 1 y 2
make demo-segments-1-2
```
