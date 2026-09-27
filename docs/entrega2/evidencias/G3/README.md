# Evidencia G3 — E2E de flujos críticos en cloud (issue #129)

Corrida del **2026-09-27** contra `https://34.24.52.111.sslip.io`, proyecto GCP
`plataforma-mooc-entrega2`. Reproducible con un comando:

```bash
go run ./scripts/e2e_cloud      # o: make test-e2e-cloud
```

**61 pasos verificados · 59 en verde · 2 fallos · 1 paso declarado fuera de
alcance.**

Los dos fallos que quedan son el mismo defecto abierto (#167) y no están en el
código de la aplicación. La verificación encontró **cuatro defectos en total**:
tres ya corregidos y verificados en esta misma corrida, uno abierto.

| | |
| :--- | :--- |
| Tabla completa paso a paso | [`resultados.md`](./resultados.md) — la genera el runner, no se edita a mano |
| Registro íntegro de la corrida | [`corrida_e2e_cloud.txt`](./corrida_e2e_cloud.txt) |
| Defecto abierto | [#167](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/167) — el prefijo `hls/` no admite lectura sin firma |
| Defectos corregidos | [#166](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/166) y sus dos derivados; notas 19, 20 y 21 de [`NOTAS_TECNICAS.md`](../../NOTAS_TECNICAS.md) |

---

## Qué verifica, y por qué no es la suite de G2 otra vez

G2 corre las siete colecciones de Postman contra la nube, cada una ejercitando
una familia de endpoints **por separado y sobre datos sembrados**. Eso deja una
pregunta sin responder: que cada familia pase por su cuenta no demuestra que las
cinco piezas del despliegue estén de acuerdo entre sí.

Esto es el recorrido continuo: **un curso que no existía, construido, con su
video subido por URL firmada y transcodificado por el worker de la otra VM,
publicado, inscrito, calificado y certificado**, releyendo el estado del
despliegue después de cada respuesta exitosa. Un solo recorrido atraviesa la VM
web, la base administrada, el bucket, la cola de la VM de workers y el proveedor
SMTP — y ese cruce es lo que ninguna colección aislada puede afirmar.

El criterio de aceptación *«cada respuesta HTTP exitosa acompañada de la
validación del estado esperado»* es el eje del diseño: ningún paso se conforma
con el código de estado. Un `202` de carga confirmada se acompaña del
`processing_status` y del `object_key` que quedaron guardados; un `200` de
publicación, del `status` y la `version` releídos; un `200` de latido, del
numerador y el denominador del progreso.

---

## Flujo → resultado → evidencia

| Flujo | Pasos | Resultado | Lo que quedó demostrado |
| :--- | :---: | :---: | :--- |
| **0. Entorno** | 1 | ✅ | La API responde y alcanza Cloud SQL (`status=pass`, `database=up`) |
| **1. Registro, verificación de correo y login** | 8 + 1 ℹ️ | ✅ | Registro real con entrega SMTP aceptada; la cuenta sin verificar no entra; token inválido rechazado sin distinguir la causa; las cuatro sesiones sembradas se emiten con su rol |
| **2. Roles, propiedad y accesos denegados** | 8 | ✅ | Estudiante no crea cursos (403); catálogo público sin el borrador; contenido 401 sin sesión y 403 sin inscripción; autor y administrador sí leen; **otro profesor no edita un curso ajeno** — la propiedad manda sobre el rol |
| **3. Autoría y publicación** | 15 | ✅ | El curso vacío acumula los dos errores de validación y no se publica; posiciones base 0 en módulo, unidad y recursos; publicación con criterio de aprobación; inmutabilidad `409` con el título intacto; el curso aparece en el catálogo |
| **4. Carga multimedia, procesamiento y consumo** | 12 | ⚠️ 10/12 | Firma V4, carga directa al bucket sin pasar por la API, confirmación idempotente, original legible con firma e **inaccesible sin ella**. El worker de la otra VM transcodifica a HLS en **10 s**. Sigue rojo el consumo público del manifiesto (#167) |
| **5. Inscripción, quiz idempotente y progreso** | 12 | ✅ | Inscripción activa; el estudiante no ve la clave de respuestas; envío calificado 100/aprobado; **reenvío con la misma `Idempotency-Key` devuelve el mismo `submission_id` y un solo envío en el historial**; progreso 0 → 1/3 → 2/3 → 3/3 con insignia; el latido repetido no infla el avance; el autor no acumula progreso |
| **6. Insignia y verificación pública** | 5 | ✅ | El dueño lee su insignia con `ETag` y `304` condicional; la de otra persona responde `404`, no `403`; **la verificación pública sin sesión confirma la insignia nombrando el curso y sin identificar al estudiante**; código inexistente `404` |

### El tramo de correo, y hasta dónde llega solo

El token de activación **viaja únicamente por correo**, así que la pata que lo
consume no se puede automatizar desde aquí sin leer un buzón. Lo que sí queda
demostrado, y no es poco: `Register` propaga el fallo de envío en lugar de
tragárselo, de modo que **un `201` en el registro es un correo que el relevo SMTP
aceptó**. Es exactamente el paso donde G2 se encontró un `500` porque F1 no
estaba desplegado. El ciclo completo con el token real —activación, recuperación
de contraseña y rechazo de la contraseña anterior— está verificado a mano en
[`../F1/README.md`](../F1/README.md).

---

## Los cuatro defectos que encontró

Ninguno estaba en la lógica de negocio. Tres eran de despliegue y configuración,
encadenados de tal forma que cada uno tapaba al siguiente; el cuarto es una
decisión de infraestructura que resultó no ser aplicable.

**Los tres primeros están corregidos y verificados en esta corrida.** Que se
encadenaran es lo interesante: mientras `REDIS_URL` apuntaba al Redis local, la
API encolaba en el vacío y **nada fallaba visiblemente**, así que ni el worker
ausente ni su arranque imposible ni el permiso del volumen se podían ver. Cada
arreglo destapó el siguiente.

### #166 — La API encolaba en un Redis que ningún worker lee

Una carga de video se confirmaba con `202`, el original quedaba guardado y el
recurso se quedaba en `pending` para siempre. **Sin un solo error en los logs**:
el mensaje se entregaba a un Redis perfectamente sano, el del propio Web Server.

`scripts/prepare_web_env.sh` fijaba `REDIS_URL='redis:6379'`; el worker de E1
consume de la cola del Worker Server. La dirección correcta la escribía el
`prepare_env.sh` **no versionado** de D2 y se perdió al versionarlo.

**Corregido**: el script exige `QUEUE_PRIVATE_IP`, sin valor por omisión, porque
una dirección ausente tiene que detener el despliegue en lugar de elegir el Redis
equivocado en silencio. Nota 19.

### El Worker Server estaba vacío

Al apuntar la cola a la VM correcta apareció lo que estaba tapado: `connection
refused`, no timeout — el paquete llegaba y nadie escuchaba. **Ni contenedores ni
unidad de systemd**: el stack de E1 nunca se redesplegó tras recrear las VMs, y
su `.env` artesanal se fue con ellas.

**Corregido de raíz**: [`scripts/prepare_worker_env.sh`](../../../../scripts/prepare_worker_env.sh),
hermano del de la VM web, más la unidad `mooc-worker.service` y el procedimiento
en [`ADMINISTRACION.md`](../../../../infra/terraform/ADMINISTRACION.md). La
configuración de esa VM deja de ser artesanal.

### El worker no podía arrancar en producción

Ya desplegado, entró en bucle de reinicio exigiendo `APP_BASE_URL`,
`CSRF_ALLOWED_ORIGINS` y las cuatro variables de SMTP. **No usa ninguna**, y una
era imposible de satisfacer: `mail.tf` le niega `smtp-password` a propósito. La
única forma de arrancarlo era darle una credencial que la infraestructura decidió
que no debía tener.

**Corregido**: `Validate()` sigue siendo la de la API; `ValidateWorker()` omite la
superficie HTTP y el correo y conserva entero lo compartido —contraseña de la
base, `sslmode`, almacenamiento y pools—. Tres pruebas lo fijan, incluida una que
comprueba que esa misma configuración **sigue siendo inválida para la API**.
Nota 20.

### El volumen de trabajo no era escribible

Con el worker arriba, cada tarea moría en `mkdir ...: permission denied` y se
reintentaba hasta archivarse. La imagen corre como el usuario `app` y prepara
`/tmp/mooc-media` a su nombre; el Compose montaba un bind mount del host que
Docker crea como `root`. Las dos mitades correctas por separado, incompatibles
juntas.

**Corregido**: un volumen con nombre montado sobre la ruta que la imagen ya
declara, que hereda su propiedad al inicializarse. Ni `chmod 777` ni correr como
root. Nota 21.

### #167 — El prefijo `hls/` no es legible sin firma, y con acceso uniforme no puede serlo

**El único que queda abierto**, y el único que no es de despliegue.

La nota 1b decidió servir `hls/` sin firma y dejar `originals/` privado, y encargó
a C3 replicarlo en el bucket administrado. `storage.tf` no lo hace, y **no puede
como está planteado**: IAM no admite condiciones en enlaces concedidos a
`allUsers`, así que con acceso uniforme a nivel de bucket o es público el bucket
entero —lo que esa misma decisión descarta— o no hay prefijo público.

La medición que lo separa de «el worker no escribió nada» es pedir un objeto que
no existe: un prefijo público responde `404`, uno privado `403`.

```
GET /hls/no-existe-a-proposito/master.m3u8   → 403   (404 si fuera público)
GET /originals/no-existe/x.mp4               → 403   (correcto, es privado)
```

No se corrige aquí: las tres salidas reales están evaluadas en #167 y la que no
degrada la postura de seguridad —un bucket aparte para los derivados— es un
cambio de C3/C4, no de una verificación.

## Dos cosas que el runner hace a propósito

**No guarda un cookie jar.** El login también emite una cookie de sesión
`Secure`, y un cliente que la almacene y la reenvíe convierte cada mutación
posterior en un rechazo de CSRF salvo que además declare un `Origin` permitido.
Es el hallazgo que G2 tuvo que corregir en las siete colecciones de Postman. Un
cliente Bearer puro no debe llevar esa cookie, y este no la lleva nunca.

**Espera el límite de login en lugar de reportarlo como fallo.** El limitador
está acotado por IP y no por cuenta, así que dos corridas seguidas pueden toparse
con un `429` que no dice nada del flujo bajo prueba. Se espera una vez y se
reintenta, como hace el runner de G2.

## Nunca

Credenciales, llaves ni secretos. Los tokens de sesión y las contraseñas no se
imprimen, y de una URL firmada se registra **solo la ruta**: la query string es
la firma. Lo que aparece en los archivos de esta carpeta es método, ruta, código
de estado y el estado verificado.
