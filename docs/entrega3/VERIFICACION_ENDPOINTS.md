# Verificación de endpoints contra la arquitectura desplegada

**29 de octubre de 2026.** Commit `1f50d952`, entorno completo encendido: grupo
de API detrás del balanceador, grupo de workers, cola, base administrada.

El objetivo es que el frontend pueda construir contra esta API sin encontrarse
sorpresas. No es una prueba de carga ni de latencia: es «¿existe esta llamada,
autoriza a quien debe y responde lo que dice el contrato?».

---

## Resultado

| | Peticiones | Aserciones | Fallos |
| :--- | ---: | ---: | ---: |
| `collection_admin` | 23 | 46 | 0 |
| `collection_authoring` | 30 | 60 | 0 |
| `collection_enrollments` | 21 | 36 | 0 |
| `collection_progress` | 39 | 64 | 0 |
| `collection_quizzes` | 28 | 56 | 0 |
| `collection_media` | 29 | 52 | 0 |
| **Newman, total** | **170** | **314** | **0** |
| `verificar_endpoints_nube.sh` | 20 comprobaciones | — | **0** |

**Las 57 rutas que registra `internal/http/server.go` están verificadas contra
el entorno desplegado.**

Cómo reproducirlo:

```bash
bash ./scripts/test_postman_cloud.sh                       # las seis colecciones
BASE_URL="$(cd infra/terraform && terraform output -raw lb_url)" \
  bash ./scripts/verificar_endpoints_nube.sh               # las 13 rutas restantes
```

Los dos no se pueden lanzar pegados: el limitador de login son **10 intentos por
minuto y por IP**, no por cuenta.

---

## Lo que hay que saber para escribir el cliente

### 1. Un `POST` sin cuerpo necesita `Content-Length: 0`

Es lo más importante de este documento. El balanceador de Google responde **411
Length Required** a un `POST` sin `Content-Length`, **antes de llegar a nuestro
servidor**. Afecta a todas las llamadas sin cuerpo:

```
POST /api/v1/courses/{id}/publish
POST /api/v1/courses/{id}/unpublish
POST /api/v1/courses/{id}/enrollments
POST /api/v1/quizzes/{id}/attempts
POST /api/v1/auth/logout
POST /api/v1/auth/verify/resend
```

Un navegador lo manda solo —`fetch(url, {method: "POST"})` emite
`Content-Length: 0`— así que un frontend web **no lo sufre**. Lo sufren `curl`,
los clientes de servidor y cualquier proxy intermedio que lo omita. Medido:

| Lo que se manda | Respuesta |
| :--- | :--- |
| sin `Content-Length` | `411`, de Google, no de la API |
| `Content-Length: 0` | llega a la API |
| cuerpo `{}` | llega a la API |

### 2. La forma de un error es plana

```json
{ "code": "unauthorized", "message": "Se requiere una sesión válida." }
```

**No** está envuelta en `error`. Hay un `details` opcional para los fallos de
validación, con `field` y `message` por entrada. Coincide con el esquema
`ErrorResponse` del contrato.

### 3. No hay alta directa de profesores

El contrato declaraba `POST /users/professors` —«Admin-only endpoint to
provision professor accounts»— **sin que existiera ni ruta ni handler**. Un
cliente generado del contrato lo habría llamado y recibido un 404. **Se borró
del contrato el 29 de octubre de 2026**, junto con su esquema
`CreateProfessorRequest`.

La forma real de tener un profesor es registrar la cuenta y promoverla:

```
PATCH /api/v1/admin/users/{userID}/role
```

Con eso, el contrato y el código **ya no divergen en ninguna ruta**. Las dos
rutas que el servidor sirve y el contrato no declara —`GET /api/docs` y
`GET /api/v1/metrics`— son de infraestructura y no tienen por qué estar ahí.

### 4. `GET /api/v1/metrics` devuelve 403 **a propósito**

`nginx.mig.conf` la deniega: servirla publicaría el inventario de rutas, los
códigos de respuesta y el volumen de tráfico. El agente de observabilidad raspa
el 8080 del contenedor y no pasa por el proxy. **Un 200 ahí sería el fallo.**

Las sondas que sí son públicas: `/api/v1/health`, `/api/v1/live`,
`/api/v1/ready`, y `/api/docs`.

### 5. Publicar un curso exige un recurso obligatorio y visible

```json
{"code":"validation_failed",
 "message":"El curso no cumple los requisitos de publicación.",
 "details":[{"field":"approval_criteria",
             "message":"El curso debe tener al menos un recurso obligatorio visible"}]}
```

Sin publicar no hay inscripción, y sin inscripción no hay intentos de quiz. Es
el orden que tiene que seguir cualquier pantalla de autoría.

### 6. Los tipos de recurso válidos

`text`, `image`, `video`, `audio`, `pdf`, `presentation`, `downloadable`,
`iframe`, `external_link`, `quiz` — de `internal/domain/course.go`. **`document`
no es uno de ellos**, aunque lo parezca.

### 7. El límite de login es por IP

10 intentos por minuto **y por IP**, no por cuenta. Un entorno de desarrollo
donde varias personas salen por la misma IP pública los comparte, y una suite de
pruebas que haga muchos logins seguidos agota el cupo. El síntoma es un `429` que
no tiene nada que ver con las credenciales.

---

## Lo que se arregló durante la verificación

| | |
| :--- | :--- |
| `mooc_cloud.postman_environment.json` apuntaba a `34.24.52.111.sslip.io` | la IP de la Entrega 2. Quien corriera la suite «contra la nube» apuntaba a una dirección que ya no existe |
| Dos aserciones de `collection_media` leían `response.json().error.code` | la API nunca ha devuelto esa forma; son las dos únicas de las siete colecciones que lo hacían |
| No existía cobertura de 13 rutas | `scripts/verificar_endpoints_nube.sh` |

### Las 13 rutas que no cubría ninguna colección

Entre ellas, lo que un frontend usa el primer día:

| | Verificado |
| :--- | :--- |
| `PUT` / `DELETE /api/v1/modules/{id}` | 200 / 204 |
| `PUT` / `DELETE /api/v1/units/{id}` | 200 / 204 |
| `PUT /api/v1/resources/{id}` | 200 |
| `POST /api/v1/quizzes/{id}/attempts` | 200 |
| `GET` / `PUT /api/v1/quizzes/{id}/attempts/current` | 200 |
| `GET /api/v1/admin/audit-logs` | 200 |
| `GET /api/v1/health`, `/live`, `/ready` | 200 |
| `GET /api/v1/metrics` | 403 (correcto, §4) |
| `GET /api/docs` | 200 |

Además se comprueba que un **estudiante no puede editar** la estructura
(`PUT /api/v1/modules/{id}` → **403**): que una ruta exista no dice nada sobre a
quién autoriza, y ese es el fallo que un frontend no detecta solo.

---

Este archivo no contiene credenciales, llaves ni secretos.
