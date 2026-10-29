# Evidencia C5 — Plantilla de instancia y grupo administrado de workers (1-3) ([#208](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/208))

| Criterio de aceptación | |
| :--- | :---: |
| Una instancia nueva del grupo empieza a consumir y procesar trabajos sin intervención | ⏳ §6 — requiere `apply` |
| La concurrencia por instancia está documentada y es la misma en todas | ✅ §3 — sale de la plantilla |
| El grupo escala instancias, no solo procesos dentro de una máquina | ✅ §3 — medido en A7 |
| El grupo nunca baja de 1 ni sube de 3 | ✅ §4 — el techo lo impone C6 |
| **Bloquea el cierre:** una instancia nueva consumiendo y terminando su primer trabajo | ⏳ §6 |
| Anotado en `NOTAS_TECNICAS.md` | ✅ notas 41, y corrección de la 31 |

**Estado: escrito y validado; sin desplegar.** `terraform validate` pasa, `go
build` y `go vet` limpios.

---

## 1. Lo que C5 no podía copiar de C1

El issue pide autohealing con un criterio de salud del worker: **«proceso vivo y
conectado a la cola compartida»**. Eso no se resuelve replicando el grupo de la
API, porque **un worker no atiende peticiones**.

Las opciones que no sirven, y por qué:

| Sonda | Por qué no basta |
| :--- | :--- |
| TCP al 9090 | el puerto acepta conexiones con el proceso vivo y la cola caída |
| HTTP a `/metrics` | igual: responde 200 sin saber nada de la cola |

Las dos dan por sana una instancia que **vive y no consume**: ocupa una máquina
sin hacer trabajo, que es exactamente lo que el grupo tendría que reparar.

### La sonda que sí

C5 añade al worker las dos sondas que la API tenía desde A2
([`internal/worker/readiness.go`](../../../../internal/worker/readiness.go)):

| Ruta | Qué responde |
| :--- | :--- |
| `/healthz` | el proceso vive. No prueba dependencias |
| `/readyz` | **PING sobre la misma conexión con la que consume** |

Que sea la misma conexión no es un detalle: comprobar una distinta de la que
trabaja puede dar sano mientras el consumo está roto.

**El autohealing mira `/readyz`.** Un worker que vive sin alcanzar la cola es
precisamente el caso que hay que reparar.

### La contrapartida, declarada

**Si la cola cae, todas las instancias del grupo se declaran no sanas a la vez**
y el grupo intentará recrearlas. No arregla nada —el problema está fuera— pero
tampoco empeora el servicio, porque sin cola no hay trabajo que hacer.

Es una consecuencia más de que la cola sea un punto único de fallo, declarado
desde B1 (nota 30). Quien quiera evitarlo tiene que hacer la cola redundante, no
cambiar la sonda: una sonda que ignore la cola devuelve el problema del principio.

## 2. Una contradicción con B2, y cómo se resuelve

**B2 dejó escrito: «No hay ninguna regla con destino `mooc-worker-node`. Nadie
les habla.»** Era cierto entonces, y el autohealing lo deja de serlo: la
comprobación de salud llega desde los rangos de Google y tiene que alcanzar el
9090.

B2 también decía «ninguna regla para el 9090 de métricas, porque el agente de
operaciones raspa `localhost`». Eso **sigue siendo cierto para las métricas**; lo
que se abre es la **sonda**, que comparte puerto porque el worker no tiene otro
servidor HTTP.

```hcl
resource "google_compute_firewall" "allow_worker_health" {
  allow { protocol = "tcp"  ports = ["9090"] }
  source_ranges = ["130.211.0.0/22", "35.191.0.0/16"]
  target_tags   = ["mooc-worker-node"]
}
```

Acotada a los dos rangos de comprobación de salud y solo a la etiqueta del grupo:
**ni el balanceador ni el resto de la subred entran por ahí**. Corregido en la
nota 31, que era donde estaba la afirmación.

## 3. Concurrencia: la misma en todas, y por qué escalar instancias

`mooc-worker-concurrency = 2` viaja en la metadata de la plantilla
(`local.mooc_metadata_worker`, B3). Como **todas las instancias del grupo salen
de la misma plantilla**, el valor es idéntico en todas por construcción: no hay
nada que sincronizar ni que vigilar.

El 2 es un proceso de transcodificación por vCPU de `e2-highcpu-2`.

### «Escalar instancias, no procesos», con el número de A7

Es un criterio de aceptación, y A7 lo midió en lugar de suponerlo:

| | trabajos/min |
| :--- | ---: |
| `WORKER_CONCURRENCY=1` | 0,639 |
| `WORKER_CONCURRENCY=2` | 0,703 |
| **Ganancia** | **+10,1 %** |

Doblar la concurrencia **dentro** de una VM de dos vCPU compra un 10 %, no el
doble, porque ffmpeg ya reparte el trabajo entre los núcleos disponibles: subir
la concurrencia reparte los mismos núcleos entre más procesos.

Y A7 midió además la saturación: el worker estuvo al **96,31 % de CPU media**
durante casi dos horas mientras el servidor web no pasaba del 2,67 %. **Es la
máquina la que se agota**, así que la única palanca real es más máquinas — que
es lo que hace C6.

## 4. La plantilla y el grupo

| | |
| :--- | :--- |
| Tipo de máquina | `e2-highcpu-2`, el mismo que la VM fija que sustituye, para que G4 pueda comparar contra A7 sin que el tipo sea otra variable |
| Disco | 30 GiB; lo que lo llena no es un original sino los restos de tareas interrumpidas, y de eso se encarga la limpieza del arranque (B3) |
| IP externa | **ninguna**: un worker no recibe tráfico de nadie |
| Cuenta de servicio | `sa-worker-server`, que **no** puede firmar URLs ni leer el secreto de SMTP |
| Etiquetas | `mooc-worker-node`, `allow-iap-ssh`. **Sin `named_port`**: a los workers no les llega ningún balanceador |
| Zonal | por lo mismo que C1: Cloud SQL y la cola son de una sola zona |
| `target_size` | 1, con `ignore_changes`. El rango 1–3 lo impone **C6** |

### El retardo inicial es mayor que el de la API, y es medido

`initial_delay_sec = 420` frente a los 300 de la API. El arranque del worker hace
lo mismo que el de la API —B3 midió 62–66 s— **más** bajar una imagen que lleva
FFmpeg dentro, que es sensiblemente mayor.

Sigue valiendo la asimetría de C1: por debajo del arranque real el autohealer
mata instancias que se están montando. **Hay que calibrarlo** con
`/var/lib/mooc/arranque.json` del rol worker en cuanto exista una medida — y esa
medida es parte de lo que C5 demuestra al desplegarse.

### Rotar un worker no es rotar una réplica de la API

`max_unavailable_fixed = 0` levanta la nueva antes de retirar la vieja, igual que
en C1. Pero **no espera a que la vieja termine su tarea**, y una tarea de verdad
tarda minutos: A7 midió 1 711 s en el perfil largo.

Que eso no sea pérdida de datos lo garantiza la idempotencia —asynq reencola y el
worker repite sin duplicar derivados— pero es trabajo tirado. El drenaje
ordenado, apagar el consumo de tareas nuevas **antes** de parar, es **C7**. Hasta
que C7 cierre, una rotación tira como mucho una tarea en vuelo por instancia.

## 5. La VM fija del worker se retira

Por un motivo distinto del que llevó a C3 a retirar la web: la superficie pública
del worker ya se la quitó B2.

Aquí el motivo es que **un consumidor fijo fuera del grupo también saca tareas de
la cola compartida**, así que una medición del escalado (G6) no podría atribuir
el trabajo al grupo. Y «el grupo escala instancias» es un criterio de C5 que con
una VM suelta consumiendo en paralelo no se puede demostrar.

Con esto, `compute.tf` se queda **sin instancias**: las dos VMs fijas de la
Entrega 2 quedan sustituidas por los dos grupos administrados.

## 6. Qué falta, y el procedimiento exacto

> **El procedimiento completo está en
> [`../C6/RUNBOOK.md`](../C6/RUNBOOK.md).** C5 y C6 piden la misma sesión
> —entorno desplegado más trabajo real en la cola— así que se cierran juntas, y
> C5 va primero porque es el caso barato: un grupo cuyas instancias no consumen
> no va a autoescalar nada. Lo de abajo es el detalle de la parte de C5.

| Criterio | Qué necesita |
| :--- | :--- |
| Una instancia nueva consume y procesa sin intervención | `apply` **y un trabajo real en la cola** |
| **La captura** (bloquea el cierre) | lo mismo |

### Hay que publicar imagen

C5 toca `cmd/worker/main.go` e `internal/worker/readiness.go`, así que **sí**
entra en la regla de la nota 34. Sin publicar, el plan se niega.

### Cómo se encola un trabajo real

El worker no se puede probar sin trabajo, y no vale inventarse una tarea con
`redis-cli`: asynq las codifica, y una hecha a mano fallaría al decodificar —
probaría el decodificador, no el consumo.

El camino corto ya existe: **`scripts/a7_verificar_e2e.sh`** hace el flujo normal
de subida —login, URL prefirmada, PUT al bucket, `complete`— y espera a que el
recurso llegue a `completed`. Sube un PDF y una imagen, que **también pasan por el
worker** como trabajos de cola (nota 16), y son mucho más baratos que un vídeo.

```bash
BASE_URL="$(cd infra/terraform && terraform output -raw lb_url)" \
  bash ./scripts/a7_verificar_e2e.sh
```

Necesita cuentas, así que antes hay que sembrar la base — y el guion que las
carga apunta por omisión a `mooc-web-server`, que **se retiró en C3**. El runbook
tiene la línea con el salto correcto.

> **Esto vale para C5 y no vale para C6.** Un PDF y un PNG pasan por el escaneo en
> segundos, y el guion espera a que cada uno llegue a `completed` antes de seguir:
> la cola nunca acumula. C6 necesita que la cola **crezca**, y para eso usa
> `scripts/c6_encolar_carga.sh` con vídeos.

### Qué capturar

1. La consola del grupo `mooc-worker-group` con su instancia creada por el grupo.
2. El registro del worker consumiendo: `job media:process started` y
   `job media:process completed` con su `duración_ms`.
3. El recurso llegando a `completed` por la API.

Lo que la captura tiene que dejar claro es que **nadie tocó la instancia**: la
creó el grupo, arrancó sola y empezó a consumir.

---

Este archivo no contiene credenciales, llaves ni secretos.
