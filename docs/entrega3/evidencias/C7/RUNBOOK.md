# Runbook — la captura que cierra C7 ([#210](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/210))

Lo que bloquea el cierre es **el drenaje de un worker que se retira con un
trabajo activo, visto desde el registro**. Hace falta el entorno encendido y un
trabajo lo bastante largo para que siga en curso cuando llegue la señal.

> **El entorno está apagado.** Encenderlo es el §3 del
> [runbook de C5/C6](../C6/RUNBOOK.md), con sus cinco interruptores. Todo lo de
> aquí asume ese paso hecho y la base sembrada.

---

## 0. Las dos cosas que conviene saber antes

**El trabajo tiene que ser largo de verdad.** Un PDF se escanea en segundos y el
drenaje no tendría nada que drenar. Hay que usar el **perfil medio** —600 s de
vídeo, unos 720 s de transcodificado— para tener holgura de sobra:

```bash
bash ./scripts/generar_media_prueba.sh medio       # verifica campo por campo
```

**Hay dos retiradas distintas y las dos valen la pena.** La §2 es la que da la
captura limpia; la §3 es la que mide el número que C7 justifica.

---

## 1. Encolar un trabajo largo

```bash
VIDEO=scratch/video_medio.mp4 \
BASE_URL="$(cd infra/terraform && terraform output -raw lb_url)" \
  bash ./scripts/c6_encolar_carga.sh 1
```

Uno solo: lo que se quiere es **un** trabajo activo e identificable, no una cola.
Anota el identificador del recurso que imprime.

Confirma que está en curso antes de seguir:

```bash
W="$(gcloud compute instance-groups managed list-instances mooc-worker-group \
    --zone=us-east1-b --project=plataforma-mooc-entrega2 --format='value(name)' | head -1)"

gcloud compute ssh "$W" --zone=us-east1-b --tunnel-through-iap --command='
  sudo docker logs --tail 5 $(sudo docker ps -q --filter name=mooc-worker)'
```

Tiene que verse `job media:process started`. **No sigas sin esa línea**: un
drenaje sobre una cola vacía no prueba nada y es indistinguible de uno que
funciona.

## 2. El drenaje, parando el servicio · *la captura que cierra el issue*

Dentro de la instancia, con el trabajo en curso:

```bash
gcloud compute ssh "$W" --zone=us-east1-b --tunnel-through-iap --command='
  date -u +%H:%M:%SZ
  sudo systemctl stop mooc-worker
  date -u +%H:%M:%SZ
  sudo docker logs --tail 40 $(sudo docker ps -aq --filter name=mooc-worker) 2>&1 | tail -40'
```

Lo que tiene que aparecer, **en este orden**:

```
[Worker Engine] drenaje iniciado: no se aceptan trabajos nuevos   plazo=1m30s
Starting graceful shutdown
Waiting for all workers to finish...
Quitting worker. task id=media-<recurso>-hls_transcode
Pushed task id=media-<recurso>-hls_transcode back to queue
All workers have finished
[Worker Engine] drenaje terminado  duración_ms=9xxxx
```

> No esperes una línea `estado=interrumpido`: asynq devuelve la tarea y el
> proceso sale antes de que la gorutina del manejador vuelva de ffmpeg, así que
> el middleware no llega a registrarla. Aparece en las pruebas unitarias, no
> aquí.

Lo que cada línea demuestra:

| Línea | Criterio del issue |
| :--- | :--- |
| `drenaje iniciado` con su `plazo` | el plazo está configurado, y es el nuestro y no los 8 s de asynq |
| `Waiting for all workers to finish` | dejó de aceptar trabajos nuevos y espera a los activos |
| `Pushed task ... back to queue` | reencolado seguro: no se perdió |
| `duración_ms` ≈ 90 000 | el plazo se agotó, que es lo esperado con un trabajo de 720 s |

**Captura 1** → `capturas/AAAA-MM-DD_drenaje-en-el-registro.png`.

### Y la otra mitad: que la tarea esté de vuelta y no duplique nada

En la VM de la cola, donde Redis corre como paquete del sistema y no en un
contenedor —el único contenedor de esa máquina es el exportador de C6—:

```bash
gcloud compute ssh mooc-queue-server --zone=us-east1-b --tunnel-through-iap --command='
  redis-cli -n 0 LLEN "asynq:{default}:pending"'
```

Tiene que dar **1**, y sin reintento consumido. Después, al volver a arrancar el
servicio, el trabajo se rehace entero:

```bash
gcloud compute ssh "$W" --zone=us-east1-b --tunnel-through-iap --command='
  sudo systemctl start mooc-worker'
```

Y cuando termine, el recurso llega a `completed` **una sola vez** y los
derivados están donde tienen que estar, sin un segundo juego:

```bash
gcloud storage ls -r "gs://plataforma-mooc-entrega2-hls/hls/<stable_id>/**" | wc -l
```

**Captura 2** → `capturas/AAAA-MM-DD_tarea-devuelta-y-rehecha.png`.

## 3. La retirada de verdad, y el número que C7 justifica

La §2 da la captura, pero para una retirada del grupo. Esto mide **cuánto
sobrevive realmente una instancia que el grupo borra**, que es el techo (2) del
§3 del README y hasta ahora es una estimación:

```bash
# Encola otro trabajo largo y espera a ver `started`, como en el §1. Luego:
date -u +%H:%M:%SZ
gcloud compute instance-groups managed delete-instances mooc-worker-group \
  --instances="$W" --zone=us-east1-b --project=plataforma-mooc-entrega2
```

`delete-instances` es el mismo camino que usa el autoescalador al reducir, pero
sobre la instancia que tú eliges y cuando tú quieres — sin esperar los 900 s de
`scale_in_control`. El grupo levantará otra para respetar `min_replicas`, que es
lo correcto y no estorba.

**No busques el drenaje en el registro: no está.** El worker escribe a stdout y
docker lo guarda con el driver `json-file`, que no se envía a Cloud Logging; y
las líneas de systemd posteriores al borrado no llegan a despacharse porque la
máquina desaparece antes. Lo que se mide aquí es **dónde acaba la tarea**, en
Redis, que distingue los dos caminos sin ambigüedad: `pending` con `retry=0` es
el drenaje; `retry` con un intento gastado es el *lease* vencido.

```bash
gcloud compute ssh mooc-queue-server --zone=us-east1-b --tunnel-through-iap --command='
  for i in $(seq 1 60); do
    echo "$(date -u +%H:%M:%SZ) activas=$(redis-cli -n 0 LLEN "asynq:{default}:active") \
pendientes=$(redis-cli -n 0 LLEN "asynq:{default}:pending") \
retry=$(redis-cli -n 0 ZCARD "asynq:{default}:retry")"
    sleep 5
  done'
```

El instante en que la tarea vuelve, menos el plazo de drenaje, da el momento del
SIGTERM; y la instancia tuvo que seguir viva hasta ese instante para escribir en
Redis, lo que da el **suelo** de la ventana del invitado.

Para el contexto de la instancia —arranque, systemd— sí sirve Cloud Logging:

```bash
gcloud logging read \
  'resource.type="gce_instance" AND labels."compute.googleapis.com/resource_name"="'"$W"'"' \
  --project=plataforma-mooc-entrega2 --limit=100 --freshness=20m \
  --format='value(timestamp,textPayload)'
```

Lo que hay que sacar de ahí: **el tiempo entre la última línea del worker y el
borrado de la instancia**. Eso es la ventana real del invitado.

- Si es **mayor** que 90 s, hay margen para subir `mooc-worker-drain-timeout` —
  hasta los 120 s del `down --timeout` de la unidad, no más.
- Si es **menor**, hay que bajarlo: un plazo que no cabe en la ventana es un
  plazo que nunca se agota ordenadamente, y entonces el trabajo no lo devuelve
  el drenaje sino el *lease* de 30 s. Sigue sin perderse, pero tarda más en
  volver a estar disponible.

Ese número va al README de C7 como medida, sustituyendo a la estimación.

**Captura 3** → `capturas/AAAA-MM-DD_retirada-por-el-grupo.png`.

## 4. Antes de apagar

Los registros de Cloud Logging caducan y la instancia ya no existe, así que hay
que bajarlos en la misma sesión:

```bash
gcloud logging read '...' --format=json > docs/entrega3/evidencias/C7/drenaje_AAAAMMDD.json
```

Y repasar que no haya URLs firmadas: el worker puede traerlas en sus trazas.

El apagado es el §8 del [runbook de C5/C6](../C6/RUNBOOK.md), **más** el `stop`
de `mooc-queue-server`, que ese §8 no contempla porque no pertenece a ningún
grupo.

---

Este archivo no contiene credenciales, llaves ni secretos.
