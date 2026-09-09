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
