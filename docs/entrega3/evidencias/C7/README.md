# Evidencia C7 — Retiro seguro de workers: drenaje, reintentos e idempotencia ([#210](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/210))

| | |
| :--- | :--- |
| **Bloque** | C — Grupos elásticos y balanceador |
| **Depende de** | C5 (#208), C6 (#209) |
| **Desbloquea** | E1, G6 |
| **Estado** | **cerrado**: drenaje medido y capturado el 29 de octubre de 2026 |

> **El mecanismo ya existía y estaba mal configurado.** C7 no construye un
> drenaje: lo ata. `asynq` ya deja de aceptar trabajos nuevos en cuanto recibe
> la señal, ya devuelve a la cola lo que no termina y ya lo hace sin gastar un
> reintento. Lo que faltaba era decirle cuánto esperar, y que la marca de «esto
> ya se hizo» fuera visible desde las otras instancias del grupo.

---

## 1. Lo que estaba roto

### El plazo de drenaje eran 8 segundos, no 120

`mooc-worker.service` le concede al contenedor 120 s para parar
(`down --timeout 120`) y a la unidad 180 s (`TimeoutStopSec`). B3 los puso ahí a
propósito y dejó escrito en la unidad que «la política de verdad la decide C7».

Pero `asynq` v0.26.0 tiene `defaultShutdownTimeout = 8 * time.Second` y
`NewWorkerEngine` **nunca fijaba `ShutdownTimeout`**. El resultado es la peor
combinación posible: la capa de sistema espera dos minutos a un proceso que se
rindió a los ocho segundos. Los 112 s restantes no se usaban para nada.

### La idempotencia de producción era por proceso

`cmd/worker/main.go` no pasaba `WithIdempotencyStore`, así que el motor caía al
`NewMemoryIdempotencyStore` por omisión — un mapa en memoria, dentro de un
proceso. El `RedisIdempotencyStore` existía completo y sin usar desde el issue
que lo escribió.

Con un worker suelto casi no se notaba. Desde C5 el worker es un grupo de 1 a 3
instancias, y **el trabajo que un drenaje devuelve a la cola lo recoge otra
instancia**, que no ve ninguna marca. Es exactamente el escenario que C7 tiene
que cubrir, y el middleware no lo cubría.

> **Matiz, porque importa para no exagerar el hallazgo.** Los efectos duplicados
> sí estaban cubiertos, por otro sitio: `mediaproc.go` comprueba contra la base
> que el recurso no esté ya `completed` sobre el mismo objeto, y el prefijo de
> los derivados sale de `storage.HLSPrefix(resource.StableID)`, que es
> determinista — un reintento **sobrescribe** los mismos objetos, no crea un
> segundo juego. El middleware en memoria era una red de seguridad que no estaba
> atada a nada, no la única defensa.

---

## 2. Lo que ya funcionaba, verificado en el código de asynq

Vale la pena dejarlo escrito, porque la tentación con un issue así es construir
un drenaje propio sobre uno que ya está:

| Momento | Qué hace asynq |
| :--- | :--- |
| Llega la señal | `processor.stop()` deja de tomar tareas nuevas **de inmediato**, independientemente del plazo |
| Vence el plazo | `close(p.abort)` → `requeue()`, que devuelve la tarea a pendiente **sin consumir un reintento** |
| El proceso muere de golpe | el *lease* de 30 s vence y el `recoverer` la recupera |
| El manejador | `ffmpeg` y `ffprobe` usan `exec.CommandContext`, así que la cancelación llega al proceso hijo |

La distinción entre `requeue` y `handleFailedMessage` es la que sostiene el
criterio del enunciado. Una instancia retirada por el autoescalador **no ha
fallado en nada**: si cada retirada consumiera uno de los tres intentos, tres
reducciones seguidas mandarían a la cola de muertos un trabajo perfectamente
sano. Está cubierto por
[`internal/worker/drain_test.go`](../../../../internal/worker/drain_test.go).

---

## 3. Lo que C7 cambia

| Dónde | Qué |
| :--- | :--- |
| `internal/config` | `WORKER_DRAIN_TIMEOUT`, por omisión **90 s**, validado |
| `internal/worker/worker.go` | `WithShutdownTimeout`, y dos trazas que delimitan el drenaje |
| `internal/worker/idempotency.go` | `NewRedisIdempotencyStoreFromURL`, el almacén compartido |
| `cmd/worker/main.go` | ata las dos cosas |
| `internal/worker/middleware.go` | una tarea interrumpida deja de registrarse como `failed` |
| `infra/` | `mooc-worker-drain-timeout` → `.env` → contenedor |

### Por qué 90 segundos

Es el **menor de tres techos**, no un número elegido por gusto:

1. `mooc-worker.service` le da al contenedor 120 s. Pasarse de ahí es pedir que
   docker mate el proceso a mitad del drenaje.
2. Una instancia que el grupo borra solo dispone de la ventana corta de apagado
   del invitado. **Medida el 29 de octubre: al menos 102 s** (§6.3). La ventana
   larga sería `graceful_shutdown` — ver §5.
3. El perfil corto de A7 son **144 s** de transcodificado.

El (3) parece decir que el plazo no sirve para nada: 90 s no salvan ni un
trabajo corto completo, y mucho menos los 1 711 s del perfil largo. **Sirve para
lo que el enunciado pide**, que es «termina ese trabajo **o lo recupera**, sin
perderlo y sin duplicar efectos»:

- lo que estaba por acabar, acaba;
- lo que no, vuelve a la cola sin gastar un reintento;
- lo que recoja otra instancia no repite efectos, porque la marca ahora es
  compartida y porque `mediaproc` no reprocesa un recurso ya `completed`.

Subirlo por encima del techo (2) no compra nada: lo que no cabe en la ventana
del invitado lo recupera igual el *lease* de 30 s.

### Una tarea interrumpida ya no dice `failed`

Al cancelarse el contexto, el manejador devuelve `context.Canceled` y el
registro decía `estado=failed` sobre un trabajo sano que iba a volver a la cola.
Como **la evidencia que bloquea este issue es precisamente el registro del
drenaje**, esa línea convertía la prueba en su contrario. Ahora sale como
`estado=interrumpido` con el motivo; lo que devuelve el manejador no cambia.

> **Corrección medida el 29 de octubre: esa línea no aparece en un drenaje real,
> y antes de correrlo este documento decía que sí.** Al abortar, asynq devuelve
> la tarea a la cola y el proceso sale; la gorutina del manejador sigue dentro de
> ffmpeg y **el proceso termina antes de que ella vuelva**, así que el middleware
> nunca llega a registrar nada. En la prueba unitaria sí sale, porque allí el
> manejador vuelve en cuanto se cancela el contexto.
>
> El cambio sigue valiendo para el caso en que el manejador sí vuelve —el
> vencimiento del *lease*, o un trabajo que no es un transcodificado— pero no
> forma parte del registro esperado de un drenaje en producción.

### Cómo se lee un drenaje en el registro

Drenaje real del 29 de octubre de 2026, con un vídeo del perfil medio en vuelo
—600 s de vídeo, unos 448 s de transcodificado— y el plazo de 90 s en
producción. Completo en
[`drenaje_29oct2026.txt`](drenaje_29oct2026.txt):

```
19:26:45.003Z  SIGTERM (systemctl stop mooc-worker)
19:26:45.355Z  [Worker Engine] drenaje iniciado: no se aceptan trabajos nuevos  plazo=1m30s
19:26:45.908Z  Waiting for all workers to finish...
19:28:15.910Z  Quitting worker. task id=media-originals/108b17d2-.../1f2aa8d5-....mp4
19:28:15.912Z  Pushed task id=... back to queue
19:28:15.913Z  All workers have finished
19:28:15.916Z  [Worker Engine] drenaje terminado  duración_ms=90560
19:28:16.489Z  systemctl stop devuelve
```

**90,56 s**, dentro de los 120 s del `down --timeout` y de los 180 s de
`TimeoutStopSec`. El plazo se agotó entero, que es lo esperado con un trabajo de
448 s.

Las dos líneas de `[Worker Engine]` son nuestras y delimitan el drenaje entero;
la diferencia de marcas dice cuánto duró de verdad. Sin ellas habría que
deducirlo de los mensajes internos de asynq, que no dicen con qué plazo se
configuró el servidor — y el síntoma de este issue era exactamente creer que el
plazo era uno cuando era otro.

---

## 4. Los límites que el issue pide documentar

### Concurrencia por instancia

| | |
| :--- | :--- |
| Por instancia | **2** (`WORKER_CONCURRENCY`, = `mooc-worker-concurrency`) |
| Instancias | 1 a 3 (C5/C6) |
| **Transcodificados simultáneos, techo** | **6** |

El 2 es 1 por vCPU del perfil `e2-highcpu-2`. A7 midió que subirlo dentro de la
misma VM compra un 10 % y no el doble, porque ffmpeg ya reparte entre los
núcleos; la palanca es más instancias, que es C6. Y el 2 es el mismo número que
`single_instance_assignment` del autoescalador: si alguien cambia la
concurrencia, ese valor cambia con ella o el grupo deja de pedir el tamaño con
el que todo el trabajo está atendido.

### Archivos temporales

Los originales y las renditions viven en `/tmp/mooc-media`, que es el volumen
con nombre `mooc_worker_scratch` — no el disco raíz. Tres capas de limpieza, y
ninguna sobra:

1. Cada tarea crea su propio `mooc-media-*` y lo borra en un `defer`. Es lo que
   impide que un worker que procesa cientos de trabajos llene el disco.
2. En una tarea interrumpida ese `defer` **sí corre**: el manejador vuelve al
   cancelarse el contexto.
3. `mooc-worker.service` vacía el volumen entero en `ExecStartPre`, para lo que
   quede de un proceso que murió de golpe.

En una retirada del autoescalador la discusión es académica: la instancia se
borra y el disco se va con ella. Importa en un reinicio del servicio sobre una
instancia que sigue viva.

El tamaño está acotado por `MEDIA_MAX_ORIGINAL_MB` (2 048 por omisión), que es la
guarda del worker y es distinta del tope de subida: una URL prefirmada no se
puede limitar en el bucket, y la VM solo tiene 30 GiB.

### Puntos únicos de falla que **siguen** en pie

Esto es lo que C7 documenta y no resuelve, y conviene que esté escrito sin
adornos:

| Qué | Por qué es un punto único | Qué pasa si cae |
| :--- | :--- | :--- |
| **Redis** (`mooc-queue-server`) | una sola VM, sin réplica; es la única máquina fuera de los grupos elásticos | no hay cola: nada se encola, nada se consume, y el autoescalador de C6 se queda sin señal y congela el tamaño del grupo |
| **La base** (`mooc-db-1`) | una sola instancia de Cloud SQL, sin alta disponibilidad | el worker no puede marcar estados; los trabajos fallan y reintentan |
| El almacén de idempotencia | comparte ese mismo Redis | se pierde la marca, pero **no** la protección: la defensa de verdad es el estado `completed` en la base |

Que la idempotencia comparta el Redis de la cola es deliberado: es una
dependencia que ya existe —si ese Redis no está, no hay nada que procesar— y
evita añadir un segundo punto único. Las claves van con el prefijo
`idempotency:worker:`, así que no se confunden con las de asynq.

El AOF de Redis sobrevive a un reinicio de la VM, y por eso la nota 42 insistió
en que la cola pasara a `metadata["startup-script"]`: cambiar su arranque ya no
cuesta una recreación.

---

## 5. La alternativa que no se tomó: `graceful_shutdown`

Compute Engine permite un apagado ordenado configurable **de 1 segundo a 6
horas**, y la restricción documentada para grupos administrados es que no puedes
*saltártelo* ni *terminarlo antes* cuando el autoescalador borra una instancia —
no que no se pueda usar. Con eso, un worker retirado terminaría su
transcodificado completo, incluso el perfil largo de 1 711 s, y el criterio
pasaría de «o lo recupera» a «lo termina» siempre.

**No se usa, y la razón es concreta:** el provider está fijado en `google 8.4.0`
y su esquema **no expone `graceful_shutdown`** ni en
`google_compute_instance_template` ni en `google_compute_instance` — comprobado
con `terraform providers schema -json`. Subir el provider a mitad de entrega
cambia el plan de los 68 recursos del estado compartido del equipo, y el
criterio del enunciado admite explícitamente la recuperación.

Queda anotado para quien retome el proyecto: es la mejora de mayor efecto por
menos código de todo el bloque C, y lo único que necesita es una versión de
provider más nueva.

---

## 6. La sesión del 29 de octubre de 2026

Commit `1f50d952`. El worker arrancó declarando `"drenaje":"1m30s"`, así que la
imagen desplegada es la de C7 — importa, porque con la anterior el drenaje
habría durado 8 s, la captura habría salido «bien» y nadie lo habría notado.

| Criterio del issue | Estado |
| :--- | :--- |
| Apagado ordenado atado al ciclo de vida | **cumplido** — 90,56 s medidos, §3 |
| Reencolado seguro de asynq | **cumplido** — `pending=1`, `retry=0`, `archived=0` |
| Reintentos sin efectos duplicados | **cumplido** — §6.2 |
| El plazo configurado y justificado | **cumplido** — y ahora medido, §6.3 |
| **El drenaje capturado mientras ocurre** | **cumplido** — [`drenaje_29oct2026.txt`](drenaje_29oct2026.txt) |
| Límites y puntos únicos de falla documentados | **cumplido** — §4 |

### 6.1 El reencolado, leído en Redis y no en el código

Justo después del drenaje:

| | |
| :--- | :--- |
| `pending` | **1** |
| `active` | 0 |
| `retry` | **0** |
| `archived` | 0 |

Esto distingue los dos caminos sin tener que creerse nada. Una tarea que **sí**
consume reintento no vuelve a `pending`: va al zset `retry`, con espera
exponencial. Que esté en `pending`, disponible de inmediato, es la firma de
`requeue()`. Los dos contadores bastan.

Al rearrancar el servicio, el trabajo se rehízo entero —`started` a las
19:29:39, `completed` a las 19:37:07, **448 s**— y el recurso llegó a
`completed` con 171 objetos bajo su prefijo HLS: un solo juego de derivados.

Nota de calibración para E1 y G6: 448 s para un vídeo de 600 s son **0,75× la
duración**, no el 1,2× que midió A7. La diferencia es la escalera de
renditions y la instancia, no un error de una de las dos medidas; quien use
cualquiera de los dos números tiene que decir con cuál se quedó.

### 6.2 Sin efectos duplicados: qué se probó y qué no

Se reconfirmó **la misma** subida sobre el recurso ya `completed`:

| | |
| :--- | :--- |
| Respuesta | `202` con el recurso tal cual, `processing_status: completed` |
| Trabajo encolado | **ninguno** — nada en el registro del worker |
| Derivados | **171 antes, 171 después** |

Lo honesto es decir **qué capa paró el duplicado**, porque no fue la que se
esperaba: lo paró la **API**, en `media.Service.ConfirmUpload`, que ante una
confirmación repetida de una subida ya procesada responde con el recurso sin
encolar nada (*upload already confirmed and processed; nothing to do*). La
guarda del worker —`mediaproc` saltándose un recurso ya `completed`— no llegó a
ejercitarse en vivo; la cubren sus pruebas unitarias.

Son tres capas y las tres siguen en pie: la API no reencola, el worker no
reprocesa, y el prefijo determinista hace que un rehecho sobrescriba en vez de
duplicar. Esta sesión demuestra la primera y la tercera.

### 6.3 La retirada por el grupo, y el número que faltaba

El §3 del runbook retira la instancia con `delete-instances`, que es el camino
del autoescalador sin esperar los 900 s de `scale_in_control`. Detalle completo
en [`retirada_por_el_grupo_29oct2026.txt`](retirada_por_el_grupo_29oct2026.txt).

**El registro de esa retirada no se puede leer**, y conviene saber por qué antes
de buscarlo: el worker escribe a stdout, docker lo guarda con el driver
`json-file` y eso **no se envía a Cloud Logging**; y las líneas de systemd
posteriores al borrado nunca se despacharon, porque la máquina desapareció
antes.

Se deduce, y de forma inequívoca, por dónde acaba la tarea:

| Camino | Destino | Reintento |
| :--- | :--- | :--- |
| Drenaje (`requeue`) | `pending`, inmediato | **no** consume |
| *Lease* vencido (30 s) | `retry`, con espera | sí consume |

La tarea apareció en **`pending` con `retry=0`** a las 19:46:18, 102 s después
de la solicitud de borrado. Si la instancia se hubiera apagado de golpe, el
*lease* habría vencido sobre las 19:45:06 y estaría en `retry` con un intento
gastado.

Y el instante cuadra con el plazo: `19:46:18 − 90 s ≈ 19:44:48`, que son unos
12 s después del borrado — el apagado ACPI y el orden de systemd.

**Conclusión: la retirada por el grupo sí ejecuta el drenaje, y la instancia
sobrevivió al menos 102 s** desde la solicitud. Los 90 s caben dentro de esa
ventana. Subirlo a los 120 s que permite la unidad exigiría una ventana de
~132 s, que esta sesión **no** demuestra: 90 s es el valor que está medido y el
que se queda.

---

Este archivo no contiene credenciales, llaves ni secretos.
