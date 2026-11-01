# Evidencia D1 — Base del frontend: stack, build versionado, despliegue, sesión y CORS (issue #211)

Código en [`frontend/`](../../../../frontend/). Decisión y trampas en la
[nota 46](../../NOTAS_TECNICAS.md#46) de `docs/entrega3/NOTAS_TECNICAS.md`.

## Estado frente a los criterios del issue

| Criterio de aceptación | Estado | Evidencia |
| :--- | :---: | :--- |
| El frontend desplegado consume la API **a través del balanceador** con sesión por cookie | ✅ | [`verificacion_nube_desplegado.txt`](./verificacion_nube_desplegado.txt): `https://136.68.55.254.sslip.io`, sesión sostenida con la cookie HttpOnly (§7) |
| La versión del build es visible y corresponde a un commit | ✅ | Pie de página y `/version.json` = `f6aa76e5988c` en la nube ([captura](./capturas/2026-11-01_nube-login-version-visible.png)) |
| CORS y CSRF configurados y documentados, con un origen ajeno rechazado | ✅ | Local: [`verificacion_origen_csrf_cors_local.txt`](./verificacion_origen_csrf_cors_local.txt). Nube: origen ajeno → 403 `csrf_origin_rejected` ([§7](./verificacion_nube_desplegado.txt)) |
| El bundle no contiene claves, credenciales ni URLs internas | ✅ | [`bundle_check.txt`](./bundle_check.txt) (§5) |
| Lo que el equipo deba saber, anotado en `NOTAS_TECNICAS.md` | ✅ | Nota 27, con la columna de a qué issues afecta |

| Tarea del issue | Estado |
| :--- | :---: |
| Arrancar sin esperar a la infraestructura (Docker Compose local) | ✅ |
| Coordinar el origen con A4 (CSRF/CORS) | ✅ decidido, documentado y **confirmado por Fredy** (2026-10-26) |
| Elegir el framework y justificarlo | ✅ React + Vite + TypeScript (§1) |
| Decidir y documentar el alojamiento y su efecto en CORS, cookies y CSRF | ✅ nginx en el mismo origen (§1) |
| Cliente HTTP con sesión por cookie, origen permitido y manejo global de 401, 403 y 5xx | ✅ (§2, pruebas unitarias) |
| Versión identificable del build, visible y registrada | ✅ (§3) |
| Configuración por variables de build, sin secretos en el bundle | ✅ (§5) |
| **Registrar la URL HTTPS pública del frontend y su conexión con la API** | ✅ **`https://136.68.55.254.sslip.io`** (mismo origen: la interfaz y `/api/` salen por el balanceador) |

## 1. Stack y alojamiento

| | |
| :--- | :--- |
| Framework | **React 19 + Vite 8 + TypeScript** (router: `react-router`). Elegido por ser el estándar más documentado, compilar a archivos estáticos sin servidor de Node en producción y tener un ecosistema de pruebas (Vitest) que ya corre en la CI. La API es solo REST JSON y el contrato (`api/openapi.yaml`) ya existe |
| Alojamiento | **El frontend se sirve en el mismo origen que la API**, con nginx (`Dockerfile.web` + `nginx.web.conf`) que sirve el build y reenvía `/api/` |
| Por qué | La cookie de sesión es `SameSite=Lax`: solo viaja si la página y la API comparten origen. Mismo origen significa **sin CORS, sin `SameSite=None` y con una sola entrada en `CSRF_ALLOWED_ORIGINS`** (nota 22). Un bucket aparte exigiría las tres cosas a la vez, y con `None` la protección CSRF descansa entera en la lista de orígenes |
| Efecto en la versión | La imagen del frontend viaja con la de la plantilla de instancias: todas las réplicas de un mismo grupo sirven la misma versión |
| Alternativa descartada | Bucket detrás del balanceador con regla de ruta: posible, pero depende de la configuración de C3 y desacopla la versión del frontend de la de la API |

## 2. Sesión por cookie y cliente HTTP (probado en el navegador, contra la API local)

Entorno: `docker compose --profile frontend up` (frontend en `http://localhost:8081`, mismo origen que la API).

| Prueba en el navegador | Resultado |
| :--- | :--- |
| Sin sesión, abrir `/` | Redirige a `/login` |
| Login con contraseña incorrecta | «Correo o contraseña incorrectos.» (el 401 del login **no** se trata como sesión vencida) |
| Login correcto | Entra; muestra «Sesión válida: el servidor reconoce tu cookie» (consulta autenticada `GET /me/enrollments`) y la réplica de la API que respondió (`X-Instance-ID`) |
| Recargar la página | La sesión se mantiene |
| `document.cookie` desde la consola | **Vacío**: la cookie `__Host-mooc_session` es `HttpOnly` y el JavaScript no puede leerla |
| `sessionStorage` / `localStorage` | Solo el usuario (sin token). **Ningún token en ningún almacén** |
| La sesión se elimina en el servidor y se recarga | Vuelve a `/login` con el aviso «Tu sesión venció» ([captura](./capturas/2026-10-26_sesion-vencida-aviso-global.png)) |
| «Cerrar sesión» | Vuelve a `/login`; una petición posterior con la cookie del navegador devuelve **401** (la invalidó el servidor, no solo la interfaz) |
| La API se recrea mientras hay sesión | La cookie sigue siendo válida (sesiones en el almacén compartido, A4) |

Capturas: [`inicio-sesion-valida-version-visible`](./capturas/2026-10-26_inicio-sesion-valida-version-visible.png),
[`sesion-vencida-aviso-global`](./capturas/2026-10-26_sesion-vencida-aviso-global.png).

**El cliente nunca lee ni guarda el token del login** (la respuesta lo trae, pero se ignora): la sesión
es solo la cookie. [`pruebas_unitarias_cliente.txt`](./pruebas_unitarias_cliente.txt) — 9 pruebas pasan:
mismo origen y sin `Authorization`; `Idempotency-Key` y cuerpo JSON; 401 fuera del login → manejador
global; 401 del login → no; 403 con su código de negocio (p. ej. `csrf_origin_rejected`); 5xx aunque
el cuerpo no sea JSON; sin respuesta de red → `status 0`; 204 sin cuerpo; claves de idempotencia únicas.

## 3. Versión del build

- Visible en el pie de cada pantalla («Versión `<commit>` · compilada …») y en `/version.json`
  ([`version_build_local.json`](./version_build_local.json)).
- El commit llega por `VITE_APP_COMMIT` (CI: `github.sha`; imagen: `--build-arg APP_COMMIT`); en local se
  lee de git. **Un build con cambios sin commitear se marca `-sin-commit`**, para que la versión visible no
  afirme más de lo que es.
- Comprobado en las dos direcciones: con cambios sin commitear el build dice `<sha>-sin-commit`; desde el
  commit limpio `650bdec28268` dice solo el SHA ([`version_build_local.json`](./version_build_local.json)).
- ⚠️ Las capturas de esta carpeta muestran `3affa11394ea-sin-commit`: se tomaron antes de que existiera el
  commit de D1. Para la evidencia en cloud, el build debe salir de un commit limpio.

## 4. CORS y CSRF con un origen ajeno

[`verificacion_origen_csrf_cors_local.txt`](./verificacion_origen_csrf_cors_local.txt), generado con
`scripts/verify_frontend_origin.sh` (reutilizable contra el balanceador: `BASE_URL=https://<balanceador>`):

| Comprobación | Resultado |
| :--- | :---: |
| Login con `Origin` = el del frontend | 200 |
| Escritura con la cookie y `Origin` **ajeno** | **403** `csrf_origin_rejected` |
| Escritura con la cookie y **sin** `Origin` | **403** |
| Preflight desde origen ajeno | Sin `Access-Control-Allow-Origin`, y nunca `*` |
| Escritura con la cookie y el origen del frontend | 204 (pasa) |

Configuración que lo sostiene: `CSRF_ALLOWED_ORIGINS` = el origen público exacto (esquema y puerto, sin
barra final). CORS no añade ninguna cabecera con el mismo origen. Cabeceras del frontend: `X-Frame-Options:
DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy` y una política de contenido que solo permite
scripts y estilos propios y el origen del almacenamiento de objetos para archivos.

## 5. El bundle no lleva secretos ni direcciones internas

[`bundle_check.txt`](./bundle_check.txt): `frontend/scripts/check-bundle.mjs`, 13 reglas (llaves privadas,
claves de Google/AWS/GitHub/Slack, JWT, cadenas de conexión con credenciales, IPs privadas, `localhost` con
puerto o loopback, servicios internos de Compose, DNS interno de GCP) → **sin hallazgos** en los 4 archivos
del build. Se verificó que el revisor **detecta** lo que debe, con un archivo de prueba que contiene cada
tipo (salida 1). El revisor corre **dentro de `docker build`** y en la CI: una imagen con un secreto no se
construye. Configuración pública solo por `VITE_*` (origen de la API opcional, vacío = mismo origen).

Dos falsos positivos de la librería de rutas se resolvieron afinando la regla, no silenciándola (nota 46).


## 6. El frontend dentro del grupo de instancias (B3 / C1)

Hasta C1 el nginx de las instancias solo reenviaba a la API: **el frontend no estaba en ninguna instancia**.
Cambios (rama `feat/211-d1-frontend-en-grupo`, para revisión de Fredy):

| Archivo | Cambio |
| :--- | :--- |
| `nginx.web.conf` | Mismas cabeceras de reenvío que `nginx.mig.conf` (`X-Forwarded-Proto` con respaldo al esquema real y `X-Forwarded-For` con la cadena). Con `$scheme` fijo, detrás del balanceador la API vería `http` y marcaría mal las cookies |
| `docker-compose.prod.yml` | `proxy.image` = `${PROXY_IMAGE:-nginx:1.27-alpine}`: por omisión no cambia nada (VM fija de la Entrega 2) |
| `scripts/publish_images.sh` | Publica también `web:<commit>` junto a `api` y `worker` |
| `scripts/prepare_web_env.sh` | Acepta `./nginx.web.conf` y escribe `PROXY_IMAGE` con el mismo `IMAGE_TAG`: la versión de la interfaz y la de la API no se separan |
| `infra/terraform/api_group.tf` | `mooc-nginx-config = ./nginx.web.conf` en la plantilla del grupo (aún sin aplicar) |

Comprobado: la plantilla de nginx se renderiza y `nginx -t` pasa; `docker compose config` resuelve el proxy con la imagen
`web`; `terraform validate` pasa; la comprobación de salud (`/api/v1/ready`) sigue cayendo dentro de `/api/`.
**Sin comprobar en la nube:** requiere publicar la imagen y aplicar C1.

## 7. En la nube (2026-10-31)

- **URL pública del frontend y de la API:** `https://136.68.55.254.sslip.io` (el frontend se sirve en el mismo origen que la API, a través del balanceador).
- Versión desplegada `f6aa76e5988c` (commit de `main`), visible en el pie y en `/version.json`.
- Cabeceras de seguridad, `/api/v1/metrics` denegada (403) y origen ajeno rechazado (403) comprobados contra el balanceador.
- Sesión por cookie sostenida: `document.cookie` vacío (HttpOnly), solo el usuario en `sessionStorage`, sin token.
- Detalle completo: [`verificacion_nube_desplegado.txt`](./verificacion_nube_desplegado.txt) y, antes del despliegue, la prueba contra la API con el proxy de desarrollo: [`verificacion_nube_dev_proxy.txt`](./verificacion_nube_dev_proxy.txt).
- Despliegue: ver la nota 46 y `scripts/publish_images.sh` (publica `api`, `worker` y `web`).

## Pendiente

Nada de este issue. (El `GET /me` que mencionaba la nota 46 sigue sin existir: la interfaz guarda el usuario del login sin token.)

## Nunca

Credenciales, llaves ni secretos. La contraseña de prueba de las cuentas se pasa por variable de entorno y
no aparece en ningún archivo de esta carpeta.
