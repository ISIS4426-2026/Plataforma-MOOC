# Colecciones de Postman

## Identidad

`collection_api.postman_collection.json` cubre el flujo completo de registro con verificación de correo y login con sesiones revocables.

### Requisitos

```bash
docker compose up -d
```

La API debe responder en `http://localhost:8080` y Mailpit en `http://localhost:8025`.

### Uso

1. En Postman: **Import** → arrastrar el archivo `.json`.
2. Abrir la colección → pestaña **Runner** (o el botón *Run collection*).
3. Ejecutar **de arriba a abajo**, sin desordenar las peticiones.

Las peticiones están encadenadas: cada una guarda en variables de colección lo que necesita la siguiente. Correr una suelta en medio del flujo falla porque las variables vienen vacías.

No hace falta configurar nada: la colección genera un correo único en cada corrida (`postman-<timestamp>@example.test`), así que se puede repetir sin limpiar la base de datos.

### Cómo obtiene el token de verificación

Las peticiones 05 y 06 consultan la **API REST de Mailpit**, no la interfaz web:

- `GET /api/v1/search?query=to:<correo>` localiza el mensaje
- `GET /api/v1/message/<id>` devuelve el cuerpo, del que se extrae el token con una expresión regular

Por eso el flujo es automático de punta a punta y no hay que copiar el enlace a mano.

### Qué verifica

17 peticiones y 42 aserciones:

| # | Petición | Comprueba |
|---|---|---|
| 00 | Health | 200 y que la base de datos responde |
| 01 | Registro | 201, rol `estudiante`, estado `pending_verification` |
| 02 | Registro con `role` | **400** — el registro público no crea profesores |
| 03 | Registro duplicado | 409 |
| 04 | Login sin verificar | **403** `email_not_verified` |
| 05 | Mailpit: buscar correo | el mensaje llegó al destinatario |
| 06 | Mailpit: extraer token | el cuerpo trae un enlace de activación |
| 07 | Verificar | 200, estado `active` |
| 08 | Reusar el enlace | **400** — el token es de un solo uso |
| 09 | Login | 200 con token, sin exponer la contraseña |
| 10 | Contraseña incorrecta | 401 `invalid_credentials` |
| 11 | Correo inexistente | 401 **idéntico** al anterior |
| 12 | Sesiones sin token | 401 con `WWW-Authenticate: Bearer` |
| 13 | Sesiones con token | 200, sin exponer `token_hash` |
| 14 | Logout | 204 |
| 15 | Mismo token después | **401** — revocación inmediata |
| 16 | Reenvío a cuenta activa | 202 genérico |

### Ejecución automatizada

La colección también corre sin abrir Postman, con Newman:

```bash
docker run --rm --network plataforma-mooc_default \
  -v "$(pwd)/docs/postman":/etc/newman postman/newman:alpine \
  run /etc/newman/collection_api.postman_collection.json \
  --env-var baseUrl=http://api:8080 \
  --env-var mailpitUrl=http://mailpit:8025
```

Se conecta a la red de Compose y usa los nombres de servicio, así que no depende de los puertos publicados en el host. Útil para incorporarlo al pipeline de CI.

## Seguridad (#13)

La última carpeta de la colección, **Seguridad (#13)**, dispara el rate limiting: agota el cupo de login desde el propio script y comprueba que llega el `429` con `Retry-After` y `RateLimit-Remaining: 0`.

Va al final a propósito, porque deja el cupo de login gastado durante el resto de la ventana. Con los valores por defecto (**10 intentos por minuto**) eso significa que **volver a correr la colección dentro del mismo minuto fallará** en el paso de login. Espera un minuto, o sube `RATE_LIMIT_LOGIN_ATTEMPTS` en tu `.env`.

### Por qué el CSRF no está en la colección

La comprobación de origen solo actúa cuando hay una lista blanca configurada, y por defecto `CSRF_ALLOWED_ORIGINS` está vacía —lo correcto mientras no exista un frontend—. Una petición de Postman tampoco envía cabecera `Origin`, así que nunca sería rechazada.

Para comprobarlo a mano:

```bash
# Levantar la API con una lista blanca
CSRF_ALLOWED_ORIGINS=https://app.plataforma-mooc.test docker compose up -d api

# Origen no permitido -> 403
curl -i -X POST http://localhost:8080/api/v1/auth/login   -H "Origin: https://sitio-del-atacante.test"   -H "Content-Type: application/json"   -d '{"email":"a@b.test","password":"loquesea1234"}'

# Método seguro desde el mismo origen -> 200, no se comprueba
curl -i -H "Origin: https://sitio-del-atacante.test" http://localhost:8080/api/v1/health
```

La cobertura automatizada de CSRF vive en `internal/http/middleware/security_test.go`.

## Administración (#12)

La carpeta **Administracion (#12)** comprueba el criterio de aceptación del issue: **un usuario no administrador recibe `403`** en las operaciones administrativas, y una petición sin token recibe `401`.

Las rutas felices —listar cuentas, cambiar rol, suspender y reactivar— no están en la colección porque necesitan una **cuenta de administrador**, y esa solo puede crearse fuera de banda: el registro público únicamente crea estudiantes, a propósito.

Para probarlas a mano, arranca un administrador así:

```bash
# 1. Registrar y verificar una cuenta normal por la API
#    (o reutilizar la que crea la colección)

# 2. Promoverla, que es el bootstrap del primer administrador
docker compose exec -T postgres psql -U moocuser -d moocdb   -c "UPDATE users SET role='administrador' WHERE email='TU_CORREO';"

# 3. Iniciar sesión y usar ese token
curl -s -X POST http://localhost:8080/api/v1/auth/login   -H "Content-Type: application/json"   -d '{"email":"TU_CORREO","password":"TU_CONTRASENA"}'
```

Con ese token:

```bash
curl -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/v1/admin/users?limit=10"

curl -X PATCH "http://localhost:8080/api/v1/admin/users/$USER_ID/role"   -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json"   -d '{"role":"profesor"}'

curl -X PATCH "http://localhost:8080/api/v1/admin/users/$USER_ID/status"   -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json"   -d '{"status":"suspended"}'
```

Intentar suspender o degradar al **último administrador activo** responde `409 last_admin_protected`, y no deja entrada en el registro de auditoría: el cambio y su entrada se escriben en la misma transacción, así que una operación rechazada no deja rastro a medias.

La cobertura automatizada de estas rutas vive en `internal/postgres/admin_repository_test.go`, incluida una prueba de concurrencia que comprueba que dos suspensiones simultáneas no pueden dejar la plataforma sin administradores.

