# Runbook — Issue G6 (#226): Escenario 3B (Ciclo de Escalamiento Automático de Workers)

Este runbook describe paso a paso cómo ejecutar, observar, reconciliar y capturar el ciclo completo de escalamiento automático del grupo de procesamiento asíncrono (`mooc-worker-group`) bajo el **Escenario 3B** de la Entrega 3.

---

## 0. Reglas Operativas y Trampas que Debes Conocer Antes de Ejecutar

1. **El grupo `mooc-worker-group` debe iniciar en `targetSize = 1` con `minNumReplicas = 1` y `maxNumReplicas = 3`**:
   - La señal primaria del autoescalador (`mooc-worker-autoscaler`) es `custom.googleapis.com/mooc/queue/outstanding` (`pending + active`), publicada cada 30 s por `mooc-queue-exporter` desde `mooc-queue-server` bajo el recurso `generic_node` (`namespace = "mooc-cola"`), con `single_instance_assignment = 2` (`replicas = ceil(outstanding / 2)`).
   - El observador ([`scripts/g6_observar_escalamiento_workers.sh`](../../../../scripts/g6_observar_escalamiento_workers.sh)) ejecuta un pre-flight estricto sobre `gcloud compute instance-groups managed describe mooc-worker-group` y aborta si el autoescalador no está `ACTIVE`, si `minNumReplicas != 1` o si `targetSize != 1`.
2. **Los logs de los workers nuevos deben capturarse mientras están `RUNNING` (antes del `scale-in`)**:
   - A diferencia de la capa API (que devuelve `X-Instance-ID` en cada respuesta HTTP), los workers toman tareas de Redis (`asynq:{default}:pending`) y escriben sus eventos (`job media:process started`, `media rendered to HLS`, `job media:process completed`) en el log local de Docker (`json-file`).
   - Cuando el autoescalador ejecuta la reducción automática (`scale-in`), **destruye la VM y su disco**, por lo que cualquier inspección post-mortem perdería la evidencia de qué worker procesó cada trabajo.
   - Por ello, [`scripts/g6_observar_escalamiento_workers.sh`](../../../../scripts/g6_observar_escalamiento_workers.sh) descarga en vivo durante cada ciclo de muestreo `/var/lib/mooc/arranque.json` y `docker logs mooc-worker` de todas las instancias `RUNNING` hacia `workers_raw/`, consolidando `trabajos_por_worker.tsv` e `instancias.tsv` antes de que ocurra el retiro.
3. **Primero pre-subir a GCS, luego confirmar (`/complete`) por etapas**:
   - El trabajo entra a `asynq:{default}:pending` únicamente al invocar `POST /api/v1/media/uploads/{id}/complete` (`202 Accepted`).
   - [`scripts/g6_carga_escalonada_workers.sh`](../../../../scripts/g6_carga_escalonada_workers.sh) pre-sube primero los objetos MP4 canónicos (`scratch/video_corto.mp4`, `120 s` @ `1280x720`, verificados con `ffprobe`) mediante URL firmada y luego ejecuta las confirmaciones en 4 etapas deterministas.
4. **Coordinación con otras pruebas del bloque G (Nota Técnica 49)**:
   - Aunque los envíos de G6 hacen pocas peticiones HTTP al balanceador (`init`, `presigned-url`, `complete` y sondeo de estado), si otro miembro del equipo está corriendo simultáneamente G3, G4 o G5 desde `mooc-load-generator`, coordina la ventana para no superponer lecturas/escrituras sobre `mooc-api-group` y `mooc-db-instance`.

---

## 1. Pre-vuelo Local (Sin Coste)

Verifica la sintaxis y ejecuta las autopruebas deterministas de las tres herramientas de G6:

```bash
bash -n scripts/g6_carga_escalonada_workers.sh scripts/g6_observar_escalamiento_workers.sh
bash scripts/g6_carga_escalonada_workers.sh --autoprueba
bash scripts/g6_observar_escalamiento_workers.sh --autoprueba
python3 scripts/g6_linea_de_tiempo.py --autoprueba
```

Todas las autopruebas deben finalizar con código `0` y mensaje `OK`.

---

## 2. Verificación del Punto de Partida en GCP

Comprueba que el entorno esté encendido y con `mooc-worker-group` en 1 réplica sana y la cola en 0:

```bash
# 1. Estado del MIG y del autoescalador de workers
gcloud compute instance-groups managed describe mooc-worker-group \
  --zone=us-east1-b --project=plataforma-mooc-entrega2 \
  --format="json(targetSize,status.isStable,autoscaler.status,autoscaler.recommendedSize,autoscaler.autoscalingPolicy)"

# 2. Instancia inicial RUNNING + HEALTHY
gcloud compute instance-groups managed list-instances mooc-worker-group \
  --zone=us-east1-b --project=plataforma-mooc-entrega2

# 3. Cola Redis limpia antes de empezar (pending=0, active=0, retry=0)
gcloud compute ssh mooc-queue-server --zone=us-east1-b --project=plataforma-mooc-entrega2 \
  --tunnel-through-iap --command='
    printf "pending=%s active=%s retry=%s archived=%s\n" \
      "$(redis-cli -n 0 LLEN "asynq:{default}:pending")" \
      "$(redis-cli -n 0 LLEN "asynq:{default}:active")" \
      "$(redis-cli -n 0 ZCARD "asynq:{default}:retry")" \
      "$(redis-cli -n 0 ZCARD "asynq:{default}:archived")"
  '
```

---

## 3. Ejecución del Ciclo Completo (Escenario 3B)

Define el directorio de la corrida para que tanto el observador como el generador de carga escriban en la misma carpeta:

```bash
export CORRIDA_DIR="docs/entrega3/evidencias/G6/corrida_$(date -u +%Y-%m-%d_%H%MUTC)"
mkdir -p "${CORRIDA_DIR}"
```

### Terminal A — Observador Activo (`scripts/g6_observar_escalamiento_workers.sh`)

Inicia primero el observador para registrar el estado inicial con 1 worker (`PREFLIGHT_OK`):

```bash
bash scripts/g6_observar_escalamiento_workers.sh "${CORRIDA_DIR}"
```

### Terminal B — Carga Escalonada por Etapas (`scripts/g6_carga_escalonada_workers.sh`)

En cuanto el observador registre la iteración `1` con `PREFLIGHT_OK`, lanza el generador de carga escalonada:

```bash
bash scripts/g6_carga_escalonada_workers.sh "${CORRIDA_DIR}"
```

#### Qué ocurre en cada etapa:

| Etapa | Trabajos Confirmados | Comportamiento Esperado |
| :--- | :---: | :--- |
| **Etapa 1 (`etapa1_baja_carga`)** | `2` videos (`120 s`) | `outstanding <= 2` (`ceil(2/2) = 1`). El worker inicial los procesa en paralelo (`WORKER_CONCURRENCY=2`) en ~145 s sin que el autoescalador pida más réplicas. |
| **Etapa 2 (`etapa2_aumento_demanda`)** | `6` videos (`120 s`) | `outstanding` salta a `6` (`> 2`). En el siguiente ciclo de `mooc-queue-exporter` (≤ 30 s) + evaluación del autoescalador, `recommendedSize` y `targetSize` suben de `1 → 3`, creando 2 instancias nuevas (`mooc-worker-<nuevo1>`, `mooc-worker-<nuevo2>`). |
| **Etapa 3 (`etapa3_sostenimiento_consumo_nuevos`)** | `4` videos (`120 s`) | Se confirman a los `90 s` del inicio de la Etapa 2 (justo cuando las instancias nuevas están completando su bootstrap de ~100 s), sosteniendo la cola pendiente para que los workers nuevos tomen y completen múltiples trabajos reales. |
| **Etapa 4 (`etapa4_reduccion_drenaje_y_reconciliacion`)** | `0` (cese de llegadas) | Los 3 workers vacían la cola (`pending → 0`, `active → 0`, `outstanding → 0`, `retry = 0`). El script reconcilia los `12` trabajos contra la API/PostgreSQL (`processing_status = completed`) y GCS (`master.m3u8` + segmentos `.ts`). |
| **Reducción automática (`scale-in`)** | — | Con `outstanding = 0`, tras la ventana `scale_in_control` (`1` réplica cada `900 s`), el autoescalador reduce `targetSize` de `3 → 2 → 1` sin perder ningún trabajo activo. |

---

## 4. Post-procesamiento, Línea de Tiempo y Capturas

Una vez completado el ciclo, ejecuta el reconstructor de línea de tiempo y exporta las métricas crudas de Cloud Monitoring (Nota Técnica 10):

```bash
# 1. Generar linea_de_tiempo.md, auditoria.tsv y las gráficas PNG en capturas/
python3 scripts/g6_linea_de_tiempo.py "${CORRIDA_DIR}" \
  --capturas-dir docs/entrega3/evidencias/G6/capturas \
  --fecha-tag "$(date -u +%Y-%m-%d)"

# 2. Renderizar las capturas de consola a partir de las salidas crudas reales
/usr/bin/python3.10 scripts/render_consola.py \
  "${CORRIDA_DIR}/trabajos_por_worker.tsv" \
  "docs/entrega3/evidencias/G6/capturas/$(date -u +%Y-%m-%d)_consumo_efectivo_workers_nuevos_terminal.png"

/usr/bin/python3.10 scripts/render_consola.py \
  "${CORRIDA_DIR}/cola_drenaje.tsv" \
  "docs/entrega3/evidencias/G6/capturas/$(date -u +%Y-%m-%d)_drenaje_cola_y_scale_in_terminal.png"

/usr/bin/python3.10 scripts/render_consola.py \
  "${CORRIDA_DIR}/reconciliacion.tsv" \
  "docs/entrega3/evidencias/G6/capturas/$(date -u +%Y-%m-%d)_reconciliacion_final_y_auditoria_terminal.png"

# 3. Verificar ausencia de secretos, tokens o IPs privadas en toda la evidencia
bash scripts/check_evidencias.sh
```

