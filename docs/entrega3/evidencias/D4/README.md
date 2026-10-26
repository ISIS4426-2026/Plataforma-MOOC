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

## Pendiente

1. Repetir el script contra el balanceador y guardar la salida junto a la de local.
2. Capturas de los estados (la de «No tienes permiso» se vio en pantalla pero no se guardó al disco).

## Nunca

Credenciales, llaves ni secretos. La contraseña entra por variable de entorno y no se imprime.
