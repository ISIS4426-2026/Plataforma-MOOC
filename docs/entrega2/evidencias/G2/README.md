# Evidencia G2 — Colecciones Postman/newman apuntadas a cloud y carga de semilla (issue #128)

**Única tarea de su ola: todo el trabajo posterior (G3, H2) pasa por aquí.**

## Estado

| | |
| :--- | :--- |
| Entorno Postman para cloud | ✅ [`mooc_cloud.postman_environment.json`](../../../postman/mooc_cloud.postman_environment.json) |
| Colecciones extendidas (inscripciones, quizzes, progreso, insignias, media) | ✅ Ya existían de A5/A6/#109/#111/#112/#113 — nada que agregar aquí |
| Manejo de CSRF y cookies sobre HTTPS en los scripts | ✅ Bug real encontrado y corregido — ver abajo |
| Runner de newman reproducible con reporte exportable | ✅ [`scripts/test_postman_cloud.sh`](../../../../scripts/test_postman_cloud.sh) → `docs/entrega2/evidencias/G2/reportes/*.txt` |
| Corrida completa contra cloud sin fallos inesperados | ⚠️ 6/7 colecciones en verde contra la nube real. La 7ª (identidad) falla en su primer paso por una dependencia externa a este issue — ver "Lo que queda fuera" |
| Carga de datos semilla desde el conjunto de G1 | 🛑 **Bloqueado** — ver "Lo que queda fuera" |

## El hallazgo: CSRF rechazaba clientes Bearer con cookie residual

D3 (recién mergeado) agrega una cookie de sesión `Secure` (`__Host-mooc_session`)
en cada login, además del token que ya se devolvía en el cuerpo. Las siete
colecciones de esta suite autentican exclusivamente con `Authorization:
Bearer <token>` — nunca leen ni escriben esa cookie a propósito —, pero
Postman/newman mantiene su propio cookie jar como lo haría un navegador real:
guarda el `Set-Cookie` del login y lo reenvía automáticamente en cada
petición siguiente de la misma corrida.

El middleware de CSRF (`internal/http/middleware/csrf.go`) hace exactamente
lo que se diseñó para hacer: una petición mutante que lleva la cookie de
sesión y no declara un `Origin` permitido se rechaza con `403
csrf_origin_rejected`, **aunque también lleve un Bearer válido**. Localmente
nunca se notó porque `CSRF_ALLOWED_ORIGINS` vive vacío en desarrollo; contra
la nube, donde D3 sí lo configura (`https://34.24.52.111.sslip.io`), toda
colección que hace login y luego una petición mutante lo dispara — se
verificó primero en `collection_admin` y se corrigió en las siete por
igual, ya que las siete comparten el mismo patrón (login seguido de
peticiones que cambian estado).

No es un bug del servidor — es exactamente la protección que D3 construyó, y
la prueba manual de D3 (`rechazo_csrf_origen_no_permitido.txt`) ya la
verificó con `curl` punto a punto. Lo que reveló esta prueba es un
comportamiento que una herramienta con cookie jar automático (Postman, un
navegador, `requests.Session()`, `axios` con `withCredentials`) puede
disparar sin querer cuando se usa como cliente Bearer puro.

**La corrección va en las colecciones, no en el servidor.** Las siete ahora
llevan un script de pre-petición a nivel de colección que limpia el cookie
jar antes de cada petición:

```javascript
pm.cookies.jar().clear(pm.environment.get('baseUrl'), function () {});
```

Verificado antes y después: `collection_admin` contra la nube pasaba de 23
peticiones/0 fallos, sin la corrección, a rechazar con `403
csrf_origin_rejected` cada petición mutante desde el primer cambio de rol
("06 Cambiar rol de estudiante a profesor") en adelante — y vuelve a 23/0 con
ella.

## Corrida real contra la nube

Seis colecciones, contra `https://34.24.52.111.sslip.io` (proyecto GCP
`plataforma-mooc-entrega2`), 2026-09-27:

| Colección | Peticiones | Aserciones | Fallos |
| :--- | ---: | ---: | ---: |
| Administración (#25) | 23 | 46 | 0 |
| Autoría de Cursos (#26) | 30 | 60 | 0 |
| Inscripciones (#111) | 21 | 36 | 0 |
| Progreso e Insignias (#113) | 39 | 64 | 0 |
| Quizzes (#112) | 28 | 56 | 0 |
| Multimedia (#109) | 25 | 43 | 0 |
| **Total** | **166** | **305** | **0** |

La colección de Multimedia incluye una carga directa real contra
`storage.googleapis.com/plataforma-mooc-entrega2-media/...` con una URL
firmada V4 real, sin pasar por la API — confirma en vivo lo que la nota 1 de
`NOTAS_TECNICAS.md` predijo: en la nube el endpoint del bucket es el mismo
para la API y para el cliente, así que la firma resuelve desde cualquier
parte, sin el truco de host interno que hace falta en local.

Salida completa del reporter CLI de newman, una por colección, en
[`reportes/`](./reportes/). No se exporta el reporter JSON de newman: incluye
el cuerpo y las cabeceras completas de cada petición y respuesta, lo que
para esta corrida significa contraseñas de login y tokens Bearer reales
contra producción en texto plano — ver la nota de "Nunca" al final.

### Segundo hallazgo: el limitador de login está acotado por IP, no por cuenta

Al correr las seis colecciones una tras otra sin pausa, `collection_quizzes`
recibió `429 rate_limit_exceeded` en su primer login — dos veces seguidas,
de forma reproducible, no por mala suerte. La causa: `RateLimit`
(`internal/http/middleware/ratelimit.go`) deriva la clave del limitador de
`r.RemoteAddr`, no de la cuenta que intenta autenticarse. Corriendo desde una
sola IP (la de quien ejecuta el runner), los logins de **todas** las
colecciones anteriores —admin, autoría, inscripciones, progreso, cada una
con varios— comparten un mismo cupo de `RATE_LIMIT_LOGIN_ATTEMPTS` (10 por
`RATE_LIMIT_LOGIN_WINDOW`, 1 minuto por defecto), y para cuando le toca el
turno a quizzes ya no queda cupo.

No es un bug: es el limitador haciendo exactamente lo que se diseñó para
hacer, y por IP es la elección correcta contra fuerza bruta. Es una
interacción real entre esa elección y un runner que ejecuta muchas
colecciones seguidas desde el mismo origen — algo que los objetivos locales
`test-postman-*` del Makefile ya evitaban limpiando `ratelimit:*` en Redis
entre corridas, algo que no es posible hacer desde fuera de la VPC contra la
nube. `scripts/test_postman_cloud.sh` lo resuelve espaciando cada colección
65 segundos y reintentando una vez si aun así se topa con el límite — por
eso la corrida completa tarda varios minutos.

## Lo que queda fuera de este issue

**Identidad (#24) no puede correr limpia contra la nube todavía.** Su primer
paso, `POST /api/v1/auth/register`, responde `500 internal_error` en el
entorno real:

```bash
curl -X POST https://34.24.52.111.sslip.io/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"...", "password":"...", "full_name":"..."}'
# {"code":"internal_error","message":"Ocurrió un error inesperado."}
```

La causa más probable es F1 (sustituir Mailpit por SMTP en la nube):
`docker-compose.prod.yml` ya declara las variables `SMTP_*`, pero F1 todavía
no está mergeado a `main`, así que el registro —que necesita enviar el correo
de verificación— falla al intentar entregarlo. G2 depende de D3 y G1 (los
dos ya resueltos), no de F1 explícitamente, pero en la práctica la cobertura
completa de identidad sí lo necesita. El resto de la suite (166
peticiones/305 aserciones) usa exclusivamente cuentas ya sembradas y activas,
así que no depende de correo saliente y por eso pasa limpia. Cuando F1
aterrice, correr `bash ./scripts/test_postman_cloud.sh --incluir-identidad`
debería bastar para cerrar esto.

**La carga de datos semilla de G1 (200 estudiantes de carga + curso
multimedia de tres perfiles) no se pudo aplicar contra la base administrada
de la nube desde esta sesión.** Cloud SQL no tiene IP pública (nota 15 de
`NOTAS_TECNICAS.md`): cargar la semilla exige ejecutar
`scripts/migrate.sh`-equivalente desde dentro de la VPC, típicamente por SSH
a `mooc-web-server`. El intento de conectarse por SSH a esa VM fue bloqueado
automáticamente por el modo seguro de esta sesión (clasificado como acceso a
infraestructura de producción), y no se buscó una vía alterna a propósito.
**Queda pendiente que alguien con acceso directo cargue
`scripts/seeds/capacity_data.sql`** (y, si se quiere el curso multimedia
también en la nube, una variante de `cmd/seed-media` apuntada al bucket
real) **contra la instancia administrada.** El seed funcional
(`synthetic_data.sql`) ya estaba cargado de antes — por eso las seis
colecciones anteriores, que solo necesitan las cuentas y el curso publicado
de esa semilla, corrieron limpias sin ningún dato de G1.

## Nunca

Credenciales, llaves ni secretos. Los reportes en `reportes/*.txt` son
exactamente la salida del reporter `cli` de newman: método, URL, código de
estado, tamaño y las aserciones con su resultado — sin cuerpos ni cabeceras.
Se descartó a propósito el reporter `json` de newman: vuelca la petición y
la respuesta completas, y para una corrida contra producción real eso es la
contraseña de cada login y el token `Bearer` de sesión en texto plano. Ese
reporte se generó localmente para depurar, nunca se guardó ni se subió al
repositorio.
