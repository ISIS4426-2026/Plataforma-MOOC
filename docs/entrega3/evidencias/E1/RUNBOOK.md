# Runbook — Ejecución y capturas en vivo de E1 ([#216](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/216))

Lo que bloquea el cierre de E1 es capturar **mientras ocurre**:
1. Una misma sesión atendida por al menos dos instancias distintas de `mooc-api-group` (con sus identificadores `X-Instance-ID`) y su invalidación inmediata tras `logout` y suspensión en todas las instancias.
2. El retiro de una instancia de `mooc-api-group` bajo tráfico sin pérdida de datos y con su reemplazo verificado por `id` numérico.
3. El retiro de un worker de `mooc-worker-group` con un trabajo activo bajo tráfico, su reencolado por drenaje ordenado (`asynq:{default}:pending` con `retry=0` tras ~90 s) y su finalización en la instancia sustituta sin pérdidas ni derivados duplicados.

> [!IMPORTANT]
> **Prerrequisito de entorno encendido**
> Si el entorno está aparcado (`db_activation_policy=NEVER`, `lb_enabled=false`, MIGs en 0), enciéndelo desde `main` siguiendo el §3 del [runbook de C5/C6](../C6/RUNBOOK.md), asegurándote de tener **`api_min_replicas = 2`** para que `mooc-api-group` mantenga dos réplicas activas detrás del balanceador.

---

## 1. Ejecución automatizada completa

Desde la raíz del repositorio, ejecuta el guion determinista de E1:

```bash
bash ./scripts/e1_integridad_y_retiro.sh \
  --url "https://136.68.55.254.sslip.io" \
  --project "plataforma-mooc-entrega2" \
  --zone "us-east1-b" \
  --salida "docs/entrega3/evidencias/E1"
```

El guion ejecuta las cuatro fases con aserciones estrictas (sale con código distinto de `0` si cualquier paso falla):
- **Fase 0**: comprueba `sessionAffinity: NONE` y `connectionDraining.drainingTimeoutSec: 60` en `mooc-api-backend` e inventaría los nombres, `id` numéricos y `creationTimestamp` de `mooc-api-group` (≥ 2 instancias) y `mooc-worker-group` (≥ 1 instancia).
- **Fase 1**: corre `go run ./scripts/e2e_cloud -base https://136.68.55.254.sslip.io -salida docs/entrega3/evidencias/E1 -min-instancias 2`, generando `resultados.md` y `corrida_e2e_cloud.txt` con las columnas `Instancia (X-Instance-ID)` y `X-Request-ID`, el flujo 7 (continuidad de sesión, logout y suspensión entre réplicas) y el flujo 8 (reconciliación de trabajos multimedia).
- **Fase 2**: retira una instancia de `mooc-api-group` bajo tráfico continuo (`gcloud compute instance-groups managed delete-instances mooc-api-group --instances=<nombre> --zone=us-east1-b`), registra cada petición en `retiro_api_trafico.tsv`, comprueba la instancia de reemplazo por su nuevo `id` numérico y verifica 0 pérdida de datos de negocio.
- **Fase 3 y 4**: sube un video de 240 s a 1280x720 (`scratch/video_e1_drenaje.mp4`, generado con ffmpeg si falta; la transcodificación tardó más de 90 s) y un video corto bajo tráfico, espera a `asynq:{default}:active >= 1`, retira el worker activo con `gcloud compute instance-groups managed delete-instances mooc-worker-group --instances=<nombre> --zone=us-east1-b`, muestrea Redis en `mooc-queue-server` cada 5 s (`retiro_worker_redis.tsv`) hasta observar el regreso a `asynq:{default}:pending` con `retry=0` (`retry_zset=0`, `archived` sin aumentar), espera a que el worker de reemplazo complete todos los recursos y verifica las tres defensas contra duplicados.

Si deseas correr únicamente la Fase 0 y Fase 1 (arnés `e2e_cloud` sin retirar VMs):

```bash
bash ./scripts/e1_integridad_y_retiro.sh --solo-e2e
```

---

## 2. Capturas obligatorias en `docs/entrega3/evidencias/E1/capturas/`

Recuerda las reglas de evidencia:
- Formato PNG con nombre `AAAA-MM-DD_descripcion.png` y peso `< 10 MiB`.
- **Nunca incluir credenciales, llaves, tokens Bearer, query strings de URLs firmadas ni IPs privadas a la vista.**

### Captura 1 — Sesión atendida por dos instancias distintas, logout y suspensión (`AAAA-MM-DD_sesion-atendida-por-dos-instancias.png`)

Muestra la sección de reparto por instancia y el Flujo 7 de `docs/entrega3/evidencias/E1/resultados.md` o las líneas correspondientes de `corrida_e2e_cloud.txt`:

```bash
grep -E "=== 7\.|=== 8\.|inst=" docs/entrega3/evidencias/E1/corrida_e2e_cloud.txt | tail -n 35
```

Debe verse claramente:
- Dos identificadores `X-Instance-ID` distintos respondiendo `200` a la misma sesión de `estudiante`.
- El `204` de `POST /api/v1/auth/logout` en una instancia y el `401` inmediato en todas las réplicas.
- El `200` de `PATCH /api/v1/admin/users/.../status` (`suspended`) y el `401` inmediato en todas las réplicas.

### Captura 2 — Retiro de instancia de API bajo tráfico y reemplazo sin pérdida (`AAAA-MM-DD_retiro-api-bajo-trafico.png`)

Muestra el resumen de la Fase 2 en `docs/entrega3/evidencias/E1/retiro_y_reconciliacion.txt` y el conteo por instancia en `retiro_api_trafico.tsv`:

```bash
grep -E "Fase 2|Instancia de reemplazo en mooc-api-group|Resumen de tráfico|Curso testigo" \
  docs/entrega3/evidencias/E1/retiro_y_reconciliacion.txt
awk -F'\t' 'NR>1 {print $3, $4, $5}' docs/entrega3/evidencias/E1/retiro_api_trafico.tsv | sort | uniq -c
```

### Captura 3 — Worker retirado con trabajo activo, reencolado por drenaje (`pending`, `retry=0`) y completado (`AAAA-MM-DD_worker-retirado-reencolado-y-completado.png`)

Muestra la serie de Redis (`retiro_worker_redis.tsv`) y el cierre de las Fases 3 y 4 en `retiro_y_reconciliacion.txt`:

```bash
column -t -s $'\t' docs/entrega3/evidencias/E1/retiro_worker_redis.tsv
grep -E "Fase 3|Fase 4|Tarea en vuelo devuelta|Worker de reemplazo|Defensa|RESULTADO GLOBAL" \
  docs/entrega3/evidencias/E1/retiro_y_reconciliacion.txt
```

Debe verse el paso de `active=1` a `pending>=1` con `retry=0` y `archived` sin aumentar tras los ~90–102 s de drenaje, el nuevo `id` numérico del worker de reemplazo y ambos recursos en `processing_status=completed` con un único `master.m3u8` en GCS.

---

## 3. Verificación final antes de cerrar y apagar el entorno

1. Comprueba que las capturas y evidencias cumplen las reglas del repositorio:
   ```bash
   bash ./scripts/check_evidencias.sh
   bash ./scripts/b4_barrido_secretos.sh
   ```
2. Si no va a correrse otro escenario inmediatamente después, aparca el entorno siguiendo el §8 del [runbook de C5/C6](../C6/RUNBOOK.md) (y detén `mooc-queue-server`) desde la rama `main` (Nota Técnica 1).

---

Este archivo no contiene credenciales, llaves ni secretos.