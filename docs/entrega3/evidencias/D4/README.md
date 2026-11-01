# Evidencia D4 — Estados de la interfaz y autorización negativa (issue #214)

## 1. Autorización negativa: lo que responde el servidor

[`autorizacion_negativa_servidor_local.txt`](./autorizacion_negativa_servidor_local.txt), generado con
[`scripts/verify_authorization.sh`](../../../../scripts/verify_authorization.sh). Reutilizable contra el
balanceador:

```bash
BASE_URL=https://<balanceador> CAPACITY_PASSWORD=<contraseña de prueba> bash scripts/verify_authorization.sh
```

| Caso | Resultado |
| :--- | :---: |
| El dueño edita su curso, agrega un módulo y lista sus cursos (control positivo) | 200 / 201 / 200 |
| Profesor B edita, agrega módulo, publica, despublica o lista la estructura del curso de A | **403** `forbidden` |
| El curso de A no aparece en «mis cursos» del profesor B | ✅ |
| Estudiante crea curso, agrega módulo, publica, edita, o pide `/me/courses` | **403** `forbidden` |
| Sin sesión: `GET /me/courses`, `POST /courses` | **401** `unauthorized` |

Conclusión: ocultar un botón en la interfaz **no** es el control de acceso; aunque se salte la interfaz, el servidor
rechaza. La interfaz además lo refleja (menú por rol, y «No tienes permiso para editar este curso» al abrir el curso
de otro profesor), pero la garantía es la del servidor.

El script crea un curso borrador del profesor A (título «[verificacion D4] …») por cada corrida; la API no permite
borrar cursos. Aviso para la corrida en la nube: ejecutarlo **una sola vez** y avisar al equipo.

## 2. Estados de interfaz

| Estado | Dónde se ve |
| :--- | :--- |
| Cargando | `Spinner` con texto en cada listado y editor |
| Vacío | `EmptyState` con acción (p. ej. «Crear mi primer curso», catálogo sin cursos) |
| Error con reintento | `ErrorState`: mensaje del servidor, `X-Request-ID` y botón «Reintentar» |
| Sesión vencida | Aviso global y vuelta a `/login` (401 fuera del login) |
| Sin permiso (403) | Aviso con el código de negocio; en el editor, pantalla «No tienes permiso» |
| Error del servidor (5xx) | Aviso global; mensaje claro aunque el cuerpo no sea JSON |
| Sin respuesta de red | `status 0` con mensaje de conexión |
| Validación (422) | Lista de razones por campo (publicación, formularios) |

## 1b. En la nube, a través del balanceador (2026-11-01)

[`autorizacion_negativa_script_nube_2026-11-01.txt`](./autorizacion_negativa_script_nube_2026-11-01.txt): las mismas
16 comprobaciones contra `https://136.68.55.254.sslip.io` (versión `f6aa76e5988c`) → **todas pasaron**, incluido el
rechazo (403) de un profesor sobre el curso de otro profesor. Además, con la sesión de un estudiante en el navegador:
[`autorizacion_negativa_nube_2026-10-31.txt`](./autorizacion_negativa_nube_2026-10-31.txt).

## 2b. Capturas de los estados, en la nube (2026-10-31)

Tomadas con la sesión de un estudiante en `https://136.68.55.254.sslip.io` (versión `f6aa76e5988c`):

| Estado | Captura |
| :--- | :--- |
| Cargando | [`nube-estado-cargando`](./capturas/2026-10-31_nube-estado-cargando.png) |
| Vacío (búsqueda sin resultados) | [`nube-estado-vacio-busqueda-sin-resultados`](./capturas/2026-10-31_nube-estado-vacio-busqueda-sin-resultados.png) |
| Recurso o curso que no existe | [`nube-estado-curso-no-existe`](./capturas/2026-10-31_nube-estado-curso-no-existe.png) |
| Sin permiso (403) con el aviso global | [`nube-estado-sin-permiso-403`](./capturas/2026-10-31_nube-estado-sin-permiso-403.png) |
| Error con «Reintentar» | [`nube-estado-error-con-reintento`](./capturas/2026-10-31_nube-estado-error-con-reintento.png) |
| Éxito y procesamiento (profesor) | [D3](../D3/capturas/) y [D2](../D2/capturas/) |

El estado de error se **simuló** haciendo que el navegador no pueda conectar con `/api/v1/courses` (sin tocar la nube): aparece «No hay
conexión con el servidor» y «Reintentar»; al quitar el fallo y pulsarlo, el catálogo se carga (12 cursos). «Sesión vencida»: ver
[D1](../D1/capturas/2026-10-26_sesion-vencida-aviso-global.png) (captura local).

## Pendiente

Nada de este issue.

## Nunca

Credenciales, llaves ni secretos. La contraseña entra por variable de entorno y no se imprime.
