# Evidencia C6 — Autoescalado de workers por señal de cola ([#209](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/209))

| Criterio de aceptación | |
| :--- | :---: |
| La señal es de cola, no de CPU | ✅ §1 — `outstanding`, y la CPU descartada con datos de A7 |
| La señal se publica desde fuera de los grupos elásticos | ✅ §2 — la VM de la cola, y es la única forma que funciona |
| El grupo escala entre 1 y 3 instancias por la señal | ✅ §4 — `single_instance_assignment = 2` |
| No se retira un consumidor con trabajo en curso | ⚠️ §5 — acotado, **no** garantizado hasta C7 |
| **Bloquea el cierre:** la métrica visible y la decisión tomada sobre ella | ⏳ §6 — requiere `apply` y carga real |
| Anotado en `NOTAS_TECNICAS.md` | ✅ nota 42; cierra el aviso de la nota 4 y corrige B4 |

**Estado: escrito y validado; sin desplegar.** `terraform validate` y `terraform
fmt` pasan, `go build`, `go vet` y `gofmt` limpios, `bash -n` sobre los dos
guiones.

---

## 1. La señal: `outstanding`, y por qué no la CPU

El enunciado admite «profundidad o antigüedad de la cola, **trabajos pendientes
por consumidor** u otra señal que represente la demanda de procesamiento». La CPU
no está en esa lista, y aquí tampoco serviría.

### La CPU falla por la razón **contraria** a la de C2

| | Por qué la CPU no sirve |
| :--- | :--- |
| **API (C2)** | no se mueve: 0,18 puntos por petición/s. A 500× el caudal, 3,7 % → 22,4 % |
| **Worker (C6)** | está siempre arriba: A7 midió **96,31 % de CPU media durante casi dos horas** |

Un worker que transcodifica está saturado mientras tenga **un** trabajo. La CPU no
distingue «uno en curso» de «cien esperando»: con un objetivo de CPU el grupo
pediría el máximo de réplicas por un solo vídeo y las mantendría hasta vaciar la
cola. Misma conclusión que en C2, causas opuestas.

`load_balancing_utilization`, la señal que sí usa C2, aquí no existe: a los
workers no les llega ningún balanceador y el grupo no tiene `named_port`.

### `outstanding` = `pending` + `active`

El exportador publica cuatro series. Tres describen la cola; una gobierna el
escalado:

| Métrica | Para qué |
| :--- | :--- |
| `.../queue/pending` | observación: trabajo que nadie ha tocado |
| `.../queue/active` | observación: trabajo en curso |
| `.../queue/oldest_pending_age_seconds` | observación: antigüedad del más viejo |
| **`.../queue/outstanding`** | **la que lee el autoescalador** |

La suma se calcula **en el exportador** y no en la consulta del autoescalador,
porque el autoescalador solo sabe sumar series del **mismo** tipo de métrica.

#### Con `pending` a secas el grupo oscila, y es aritmética

```
pending=4 → pide 2 réplicas → la nueva toma 2 → pending=2 → pide 1
          → retira → el trabajo en vuelo se reencola → pending=4 → …
```

Un trabajo sale de `pending` en el instante en que **alguien lo empieza**, así que
`pending` baja porque hay más capacidad trabajando — y el autoescalador lee esa
bajada como que sobra. `outstanding` solo baja cuando un trabajo **termina**, así
que durante un vaciado decrece de forma monótona.

## 2. Dónde se publica, y por qué no podía ser en el worker

La métrica existía desde A2: `internal/worker/queue_metrics.go` ya publica
`pending`, `active` y la antigüedad desde dentro de cada worker. **Eso vale para
observar y no vale para autoescalar.** Tres impedimentos, cada uno suficiente:

| | |
| :--- | :--- |
| **El autoescalador SUMA** las series que devuelve su consulta | La profundidad es una magnitud **global**. Con tres workers publicándola, ve el triple del trabajo que hay y escala por su propio reflejo |
| La serie desaparece en una rotación | Justo cuando la situación está cambiando |
| Con `gce_instance` **no se puede filtrar** | Ver abajo |

### El tercero obligó a cambiar de plan a mitad del issue

El camino obvio era un exportador de Prometheus raspado por el agente de
operaciones, como ya hacen la API y el worker. **No funciona**, y la
documentación de GCE lo dice en dos frases que juntas cierran la puerta:

> Si el tipo de recurso es `gce_instance` (el predeterminado), la métrica debe
> incluir una serie temporal separada para cada instancia, y **no se puede
> filtrar por etiquetas de recurso**. Para cualquier otro tipo de recurso, la
> métrica se trata como del grupo, y sí se usa el filtrado por etiquetas.

El agente publica bajo el `gce_instance` **de la máquina donde corre**. En la VM
de la cola, que no es miembro del grupo, eso produce una serie que el
autoescalador no puede seleccionar: no es de una instancia del grupo y no admite
filtro.

Por eso `cmd/queue-exporter` **escribe directamente en Cloud Monitoring** con
`CreateTimeSeries`, sobre el recurso `generic_node`, que sí es filtrable. No es
una preferencia por la API sobre el agente: es la única forma de que el
autoescalador vea el dato.

### La VM de la cola es el sitio, y ya estaba previsto

La nota 4 lo dejó escrito desde B1: *«si el exportador de esa métrica vive en una
instancia del grupo, el autoscaler se queda ciego justo cuando reduce a una
instancia»*, y `sa-queue-server` llevaba `monitoring.metricWriter` desde B2
esperando este momento. Es el **único cómputo del despliegue** que no pertenece a
un grupo elástico.

El enunciado lo pide del otro lado: la cola no debe «depender su disponibilidad
de una máquina que el autoscaling pueda retirar», y la señal que gobierna el
autoscaling es parte de esa disponibilidad.

## 3. Dos cosas que B1 y B4 habían declarado y dejan de ser ciertas

### B4: «`sa-queue-server` no lee secretos y no baja imágenes»

La primera mitad sigue en pie. La segunda no: el exportador es un binario de la
imagen `worker` —comparte `internal/worker`— así que la cuenta de la cola recibe
`artifactregistry.reader`.

Lo que **no** se le concede, y es lo que impide que esto deshaga la separación de
B2: el rol es de **solo lectura** sobre el repositorio. La cola puede bajar la
imagen y no puede publicar ninguna, ni leer el secreto de SMTP, ni abrir la base,
ni firmar URLs. Corregido en `artifact_registry.tf`.

### B1: la VM de la cola usaba `metadata_startup_script`

`metadata_startup_script` es **ForceNew**: cambiar su contenido recrea la VM. Para
una plantilla de grupo eso es irrelevante —una plantilla es inmutable— pero esta
máquina **guarda estado**: el AOF con los trabajos encolados, que es la razón
entera de no usar Memorystore (B1).

C6 la mueve a `metadata["startup-script"]`, que se actualiza en sitio. Desde
entonces, aplicar un cambio del arranque es **reiniciar** la VM —que conserva el
disco, y con él el AOF— en vez de recrearla.

> **Este cambio cuesta una recreación, la última.** Mover el guion de un sitio al
> otro también es ForceNew, así que el `apply` que introduce C6 recrea la cola una
> vez y pierde lo que hubiera encolado. En un entorno aparcado no hay nada; si
> hubiera trabajos en vuelo, hay que esperar a que la cola se vacíe.

## 4. El autoescalador

| | |
| :--- | :--- |
| Métrica | `custom.googleapis.com/mooc/queue/outstanding` |
| Filtro | `resource.type = "generic_node" AND resource.labels.namespace = "mooc-cola"` |
| Reparto | `single_instance_assignment = 2` |
| Límites | 1–3, los del enunciado |
| `mode` | `ON`: escala en los dos sentidos, porque G6 mide el ciclo completo |
| `cooldown_period` | 420 s |
| Interruptor | `var.worker_autoscaling_enabled`, por omisión **`false`** |

### «Trabajos pendientes por consumidor», traducido

`single_instance_assignment` es el valor de la métrica que el autoescalador asigna
a cada instancia: `réplicas = outstanding / 2`, acotado entre 1 y 3.

| `outstanding` | réplicas |
| ---: | ---: |
| 2 | 1 |
| 4 | 2 |
| ≥ 6 | 3 |

El 2 no es redondo por casualidad: es `mooc-worker-concurrency`, los procesos de
transcodificación que una instancia atiende a la vez (B3, C5). Con ese valor, el
tamaño que pide el autoescalador es **el número de instancias con el que todo el
trabajo está siendo atendido y nada espera**.

Subirlo encola trabajo a propósito; bajarlo pide instancias que no tendrían con
qué llenar sus dos ranuras. **Si alguien cambia la concurrencia, este número
cambia con ella.**

### El mínimo es 1, y aquí la razón no es la de C2

En C2 el mínimo es 1 porque con 0 no hay nadie que responda la primera petición.
Aquí **escalar desde cero sí funcionaría**: la señal la escribe la VM de la cola,
que no pertenece al grupo, así que no hay ceguera a cero réplicas.

No se hace porque el arranque del worker son minutos —B3 midió 62–66 s más la
imagen con FFmpeg— y el primer trabajo pagaría el arranque entero mientras un
alumno espera.

### `cooldown_period = 420`, el mismo número que C5

Las dos cosas contestan la misma pregunta —cuánto tarda un worker nuevo en estar
consumiendo de verdad— y tener dos números distintos para una sola pregunta es
cómo se acaba con un autohealer matando lo que el autoescalador acaba de pedir.

Sigue siendo una **estimación**. Hay que calibrarlo con
`/var/lib/mooc/arranque.json` del rol worker en cuanto C5 deje una medida: por
debajo del arranque real, el autoescalador ve una cola que no baja —porque el
worker nuevo aún no consume— y pide otra réplica, que hará lo mismo.

## 5. La reducción está acotada, no es segura

`scale_in_control`: **una réplica cada 900 s**, así que de 3 a 1 tarda al menos
media hora. Deliberadamente más lento que los 600 s de la API, y por una razón
medida: retirar una réplica de la API tira como mucho las peticiones que el
drenaje de C3 no alcance (60 s); retirar un worker interrumpe un transcodificado,
y A7 midió **1 711 s** en el perfil largo.

Que eso no sea pérdida de datos lo garantiza la idempotencia —asynq reencola y el
worker repite sin duplicar derivados— pero es trabajo tirado, y el trabajo
reencolado **vuelve a `outstanding`**, así que una reducción agresiva se desanda a
sí misma.

> **Esto no es un drenaje y no hay que leerlo como si lo fuera.** Acota la
> frecuencia del daño a un trabajo en vuelo por ventana; no espera a que el
> trabajo termine, y 900 s no cubren los 1 711 s del perfil largo ni pretenden.
> Apagar el consumo de tareas nuevas antes de parar es **C7 (#210)**.

## 6. La corrida del 29 de octubre de 2026

> **El procedimiento paso a paso está en [`RUNBOOK.md`](RUNBOOK.md)**, que cubre
> la sesión entera y cierra **C5 y C6 a la vez**: las dos piden lo mismo
> —despliegue más trabajo real en la cola— y lo que cada una demuestra es
> distinto. Lo de abajo es el resumen.

Commit `e602524`. 10 vídeos del perfil corto encolados por el flujo normal de
subida con `scripts/c6_encolar_carga.sh`. **Ciclo completo en 33 min**, sin que
nadie tocara el tamaño del grupo.

| Criterio | Estado |
| :--- | :--- |
| La métrica visible en Cloud Monitoring | **cumplido** — una serie, el filtro literal del autoescalador |
| La decisión del autoescalador tomada sobre ella | **cumplido** — 1 → 2 → 3 réplicas, pedidas por él |
| La reducción | **cumplida** — 3 → 2 → 1, una por ventana de 900 s |
| **Las capturas de la consola** | **hechas** — las cuatro, en [`capturas/`](capturas/) |

### La cronología, con los números medidos

| Hora UTC | T+ | Qué |
| :--- | ---: | :--- |
| 16:10:34 | 0 s | las 10 confirmaciones. `outstanding` pasa de 0 a 10 entre dos muestreos |
| 16:10:58 | 24 s | primer punto con `outstanding = 10` |
| **16:11:07** | **33 s** | **el autoescalador crea la segunda réplica** |
| 16:11:47 | 73 s | crea la tercera |
| 16:13:28 | 174 s | `active = 6`: las tres instancias con sus dos ranuras llenas |
| 16:18:58 | 504 s | `outstanding = 0`, cola vaciada |
| 16:27:32 | 1 018 s | 3 → 2 |
| 16:42:54 | 1 940 s | 2 → 1. **Ciclo completo** |

Entre las dos reducciones pasaron **922 s**, que es `time_window_sec = 900` con el
margen del muestreo: `scale_in_control` hizo exactamente lo que declara.

Los 33 segundos entre el primer punto de la métrica y la primera réplica son la
medida que importa: es lo que tarda el autoescalador en **ver** la señal y actuar.
El resto —hasta que la réplica consume— es arranque, y C5 lo midió en 102 s.

### La aritmética del enunciado, comprobada

`single_instance_assignment = 2` tenía que dar `replicas = outstanding / 2`
acotado entre 1 y 3. Con `outstanding = 10` el cociente es 5, así que el tope de 3
manda — y es lo que pidió.

Que el 2 fuera el número correcto lo confirma el punto de las 16:13:28:
**`active = 6`** con tres instancias, es decir las seis ranuras de
`mooc-worker-concurrency = 2` ocupadas a la vez y `pending` ya en 2. El tamaño que
pidió el autoescalador es exactamente aquel con el que todo el trabajo está siendo
atendido.

### `pending` habría oscilado, y los datos lo enseñan

El argumento de la §1 deja de ser teórico:

| Instante | `pending` | `active` | `outstanding` | Qué pasó |
| :--- | ---: | ---: | ---: | :--- |
| 16:10:58 | 8 | 2 | **10** | un worker, sus dos ranuras llenas |
| 16:12:58 | 6 | 4 | **10** | la segunda réplica empieza a consumir |
| 16:13:28 | 2 | 6 | **8** | las seis ranuras llenas; termina el primer vídeo |

Entre las 16:10:58 y las 16:12:58, `pending` cayó de 8 a 6 con `outstanding`
clavado en 10: **no terminó nada**, simplemente alguien se puso a trabajar. Medio
minuto después `pending` estaba en 2 y solo **un** trabajo de los diez había
terminado.

Un autoescalador leyendo `pending` habría visto 2 —capacidad de sobra— y habría
retirado lo que acababa de pedir, reencolando el trabajo en vuelo y volviendo a
subir `pending`. `outstanding` no bajó hasta que un vídeo terminó de verdad, y
desde ahí decreció de forma monótona hasta 0.

### Los datos crudos

- [`cronologia.txt`](cronologia.txt) — un sondeo cada 30 s: `outstanding`, lo que
  el autoescalador recomienda, el tamaño del grupo y el estado de cada instancia
- [`metricas_crudas_29oct2026.csv`](metricas_crudas_29oct2026.csv) — los 76 puntos
  de las cuatro métricas, exportados el mismo día (nota 10: **caducan**)

### Hay que publicar imagen

C6 toca `cmd/queue-exporter/` y `Dockerfile.worker`, así que entra en la regla de
la nota 34: **publicar antes de aplicar no es opcional**. Y aquí importa el doble,
porque un `pull` fallido en la VM de la cola no se ve como un grupo recreando
instancias, sino como una métrica que nunca aparece.

El `data` de `bootstrap.tf` ya convierte eso en un error del **plan**.

### Encender

```bash
terraform apply \
  -var db_activation_policy=ALWAYS \
  -var lb_enabled=true \
  -var api_autoscaling_enabled=true \
  -var worker_autoscaling_enabled=true
```

### El guion de la prueba comprueba primero lo que falla en silencio

```bash
bash ./scripts/c6_prueba_autoescalado.sh docs/entrega3/evidencias/C6/cronologia.txt
```

Su **fase 1** lee la serie con el filtro exacto del autoescalador y falla si no
devuelve nada. No es un adorno: **un filtro que no encuentra la serie no es un
error para GCE**. El autoescalador sin datos mantiene el tamaño actual, así que el
síntoma no es un fallo visible sino un grupo que no escala. La fase 1 convierte
eso en un error ruidoso, y la distinción que importa es:

| Lo que devuelve el filtro | Qué significa |
| :--- | :--- |
| `0` | la serie existe, la cola está vacía. **Correcto** |
| nada | el autoescalador está ciego. **Roto** |

La **fase 2** imprime una línea por sondeo con T+, `outstanding`, lo que el
autoescalador recomienda, el tamaño del grupo y el estado de cada instancia.

### La carga: el guion no la genera, y es a propósito

Encolar trabajos a mano con `redis-cli` no vale: asynq los codifica, y uno hecho a
mano fallaría al decodificar — probaría el decodificador, no el consumo. Tiene que
ser el flujo normal de subida: URL prefirmada, PUT al bucket, `complete`.

```bash
BASE_URL="$(cd infra/terraform && terraform output -raw lb_url)" \
  bash ./scripts/c6_encolar_carga.sh 10
```

### Por qué un guion propio y no `a7_verificar_e2e.sh`

Ése es el correcto para **C5**, y para C6 no sirve por dos razones:

| | |
| :--- | :--- |
| Sube un PDF y un PNG | solo pasan por el escaneo: segundos. La cola se vacía tan rápido como se llena |
| Espera a `completed` de cada uno | nunca hay dos trabajos en vuelo, así que `outstanding` no pasa de 2 |

`c6_encolar_carga.sh` sube **vídeos** del perfil corto —120 s, unos 144 s de
transcodificación cada uno, A7 midió 1,2× la duración— y sobre todo **sube los 10
objetos primero y confirma los 10 seguidos después**.

Ese orden es el único truco del guion, y no es cosmético: el trabajo se encola en
la **confirmación**, no en el PUT. Confirmando cada subida al hacerla, con un
enlace doméstico el primer vídeo estaría transcodificado antes de encolarse el
sexto. Subiendo todo primero, `outstanding` salta a 10 en segundos **sea cual sea
la velocidad de subida**.

Necesita cuentas sembradas, y el guion que las carga apunta por omisión a una VM
que **se retiró en C3**: el runbook tiene la línea con el salto correcto.

### Qué capturar

1. El Explorador de Métricas con `custom.googleapis.com/mooc/queue/outstanding`
   subiendo. **Hecha**:
   [`capturas/2026-10-29_outstanding-sube-y-baja.png`](capturas/2026-10-29_outstanding-sube-y-baja.png).
2. La consola del autoescalador mostrando su **tamaño recomendado** y el motivo.
   **Hecha**:
   [`capturas/2026-10-29_autoescalador-utilizacion.png`](capturas/2026-10-29_autoescalador-utilizacion.png),
   la gráfica *Autoscaler utilization* sobre
   `custom.googleapis.com/mooc/queue/outstanding` con `Capacity: 2` —que es
   `single_instance_assignment`—. La política aplicada, en texto, está en
   [`autoescalador_29oct2026.yaml`](autoescalador_29oct2026.yaml).
3. El grupo pasando de 1 a 2 o 3 instancias, con la hora. **Hecha**.
4. El grupo volviendo a 1 cuando la cola se vacía. **Hecha**.

Las dos últimas son la misma imagen,
[`capturas/2026-10-29_grupo-tamano-1-3-1.png`](capturas/2026-10-29_grupo-tamano-1-3-1.png):
la gráfica *Group size* del grupo traza a la vez el tamaño configurado (min 1,
max 3), el recomendado y las instancias reales, así que el ciclo entero —los dos
escalones de subida sobre las 12:11 EDT y los dos de bajada a las 12:27 y 12:42—
se lee en un solo cuadro, con el eje de tiempo a la vista.

Esa gráfica es además la respuesta al «la ampliación manual del grupo no acredita
autoscaling»: la línea de **tamaño recomendado** y la de **instancias** se mueven
juntas, y el mínimo y el máximo configurados siguen planos en 1 y 3. Nadie
redimensionó nada.

La consulta de la captura 1, que es el filtro del autoescalador traducido a
PromQL:

```promql
custom_googleapis_com:mooc_queue_outstanding{monitored_resource="generic_node",namespace="mooc-cola"}
```

El `monitored_resource` no es opcional: sin él la API responde que el tipo de
métrica admite 23 tipos de recurso y no sabe cuál quieres. Y **el `namespace` sí
importa para la evidencia**: sin esa etiqueta la consulta deja de ser la del
autoescalador.

Lo que las capturas tienen que dejar claro es que **nadie redimensionó el grupo**:
el enunciado dice que «la ampliación manual del grupo no acredita autoscaling».
Por eso el guion compara contra el `targetSize` inicial y anuncia el hito solo
cuando el cambio lo pidió el autoescalador.

### Y exportar los puntos crudos en el momento

La nota 10 aplica entera: esta métrica personalizada **caduca**, y desaparece del
todo cuando el proyecto pierde los recursos en I6. La cronología del guion se
guarda en este directorio en la misma sesión en que se mide, no después.

**Hecho el 29 de octubre**, el mismo día de la corrida: los 76 puntos de las
cuatro métricas están en
[`metricas_crudas_29oct2026.csv`](metricas_crudas_29oct2026.csv), con el recurso y
el filtro en la cabecera del archivo. A partir de ahí la gráfica se puede rehacer
sin Cloud Monitoring.

---

Este archivo no contiene credenciales, llaves ni secretos.
