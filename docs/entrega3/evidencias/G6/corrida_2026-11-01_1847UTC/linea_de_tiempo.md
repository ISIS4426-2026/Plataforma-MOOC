# Línea de Tiempo y Reconciliación — Escenario 3B (Issue G6)

## 1. Resumen de Criterios de Aceptación Verificados

| Criterio | Resultado | Evidencia Cuantitativa |
| :--- | :---: | :--- |
| Arranque con baja carga y 1 worker operativo | PASS | `targetSize` inicial = `1`, Etapa 1 procesada sin escalar |
| Creación automática de workers (`scale-out`) | PASS | `targetSize` `1 -> 3` (2 instancias nuevas creadas) |
| Consumo y procesamiento efectivo por workers nuevos | PASS | `2` trabajos completados por instancias nuevas (`es_worker_nuevo=true`) |
| Drenaje de la cola y reducción automática (`scale-in`) | PASS | `pending=0, active=0, retry=0, delta_archived=0`; `targetSize` `3 -> 1` |
| Reconciliación 1:1 sin pérdidas ni duplicados | PASS | `8/8` en PostgreSQL + `8/8` playlists HLS en GCS |
| Cero intervenciones manuales (Cloud Audit Logs) | PASS | `0` llamadas manuales a `resize` / `setAutoscaling` |

## 2. Desglose Separado: Tiempo de Reacción vs. Tiempo de Arranque e Incorporación

| Tramo del Ciclo | Inicio (UTC) | Fin (UTC) | Duración (s) | Explicación Técnica |
| :--- | :---: | :---: | :---: | :--- |
| **1. Latencia de exportación de señal** | `2026-11-01T18:56:31Z` | `2026-11-01T18:57:06Z` | `26.3 s` | Muestreo de `mooc-queue-exporter` (cada 30 s) hacia Cloud Monitoring |
| **2. Tiempo de reacción del autoescalador** (desde cruce en Redis) | `2026-11-01T18:56:31Z` | `2026-11-01T18:56:57Z` | **`26.3 s`** | Desde `outstanding > 2` en Redis hasta decisión de `mooc-worker-autoscaler` (`26.3 s` desde serie visible en CM) |
| **3a. Provisión de VM en GCE** | `2026-11-01T18:56:57Z` | `2026-11-01T18:57:06Z` | `8.7 s` | Creación de VM `e2-highcpu-2` (`PROVISIONING -> STAGING -> RUNNING`) |
| **3b. Bootstrap del contenedor (`arranque.json`)** | `2026-11-01T18:56:57Z` | `2026-11-01T18:58:36Z` | `98.7 s` | `mooc-bootstrap.sh` + arranque de `mooc-worker` y escucha en `:8080/readyz` |
| **3c. Tiempo de arranque e incorporación a consumo** | `2026-11-01T18:56:57Z` | `2026-11-01T18:58:36Z` | **`99.4 s`** | Desde la decisión del autoescalador hasta que el worker nuevo toma su **primer trabajo** (`job media:process started`) |
| **3d. Primer trabajo completado por worker nuevo** | `2026-11-01T18:56:57Z` | `2026-11-01T19:01:20Z` | `263.2 s` | Incluye descarga desde GCS, transcodificación HLS con `ffmpeg` y subida de segmentos |
| **Total extremo a extremo (demanda -> consumo nuevo)** | `2026-11-01T18:56:31Z` | `2026-11-01T18:58:36Z` | **`125.7 s`** | Tiempo total desde que la cola supera la capacidad de 1 worker hasta que el nuevo worker descarga cola |

## 3. Ciclo de Vida de las Instancias del Grupo (`mooc-worker-group`)

| Instancia | ID GCE | Rol | Creación (UTC) | `RUNNING` (UTC) | `arranque.json` (s) | Primer Job Iniciado | Primer Job Completado | Jobs Completados | Inicio Retiro (`DELETING`) |
| :--- | :--- | :--- | :---: | :---: | :---: | :---: | :---: | :---: | :---: |
| `mooc-worker-wxgc` | `8072972312435673316` | `inicial` | `2026-11-01T09:25:31.631-08:00` | `2026-11-01T18:50:49Z` | `90` | `2026-11-01T18:53:29.823739013Z` | `2026-11-01T18:56:23.322998616Z` | **6** | `activa (min=1)` |
| `mooc-worker-9td8` | `162295375347725718` | `nuevo_scale_out` | `2026-11-01T10:56:57.288-08:00` | `2026-11-01T18:57:06Z` | `60` | `2026-11-01T18:58:36.693427083Z` | `2026-11-01T19:01:20.476993862Z` | **2** | `2026-11-01T19:11:28Z` |
| `mooc-worker-lxnc` | `7913173235234649345` | `nuevo_scale_out` | `2026-11-01T10:58:54.948-08:00` | `2026-11-01T18:59:37Z` | `61` | `n/a` | `n/a` | **0** | `2026-11-01T19:26:35Z` |

## 4. Trabajos Concretos Procesados por Cada Instancia Worker

| Worker | ¿Nuevo (`scale-out`)? | `resource_id` | `asynq_task_id` | Inicio (`started`) | Fin (`completed`) | Duración (`ms`) | Segmentos HLS | Estado |
| :--- | :---: | :--- | :--- | :---: | :---: | :---: | :---: | :---: |
| `mooc-worker-wxgc` | `false` | `4955e6cb-f65c-4964-9594-c5f120d0f887` | `media-originals/fbfa6968-f483-434a-b35d-4a81d98fe0e2/43f099b5-41d2-4415-a3c5-fb440c1475f4.mp4` | `2026-11-01T18:53:29.823739013Z` | `2026-11-01T18:56:23.322998616Z` | `173502` | `37` | `completed` |
| `mooc-worker-wxgc` | `false` | `37db5ff9-50fb-4f69-82c1-33b035529c62` | `media-originals/bfcfe6f6-f906-4c9f-85b2-33b7c6d90ae9/170d3d24-dfdb-4a8f-84aa-d2f7f4588fa7.mp4` | `2026-11-01T18:53:31.339227973Z` | `2026-11-01T18:56:22.210268135Z` | `170871` | `37` | `completed` |
| `mooc-worker-wxgc` | `false` | `71d08a34-9541-4ec9-b1f4-0203168b72d7` | `media-originals/5ffe0cd0-ae83-4c65-82a8-226d7ad660eb/45836188-a04e-45ae-b496-ea4337b1006a.mp4` | `2026-11-01T18:56:31.562613258Z` | `2026-11-01T18:59:15.569200507Z` | `164006` | `37` | `completed` |
| `mooc-worker-wxgc` | `false` | `dfaefe4a-6da2-4428-a081-4524d63468a9` | `media-originals/494617b8-ac2a-4002-b5fe-89238d699297/75451a69-9ea2-4ccc-b8c6-d0368ffcfac7.mp4` | `2026-11-01T18:56:31.563623805Z` | `2026-11-01T18:59:15.362843068Z` | `163799` | `37` | `completed` |
| `mooc-worker-9td8` | `true` | `7e0c8444-0f34-42b5-9e3a-750ff06cf953` | `media-originals/f8aeee24-c12e-4e2e-a80b-6145c13f3402/f4eac3c1-11c9-460c-b1bf-6a3394843b9d.mp4` | `2026-11-01T18:58:36.693427083Z` | `2026-11-01T19:01:20.476993862Z` | `163783` | `37` | `completed` |
| `mooc-worker-9td8` | `true` | `c0e76f4a-6986-4944-b4ca-392d165d87fe` | `media-originals/d1208a19-6aab-489c-85c8-ea786635ca2e/4332fb2c-78cf-4079-be49-350333721900.mp4` | `2026-11-01T18:58:36.694766115Z` | `2026-11-01T19:01:19.760321498Z` | `163064` | `37` | `completed` |
| `mooc-worker-wxgc` | `false` | `f3c68377-ab46-4779-8e6b-0196289154a1` | `media-originals/63369504-81b1-47ed-8789-f1ebc2c1f5a3/6d35aa58-3eb5-441c-9dc4-0f28c7a604bf.mp4` | `2026-11-01T18:59:15.367681902Z` | `2026-11-01T19:01:58.118923845Z` | `162751` | `37` | `completed` |
| `mooc-worker-wxgc` | `false` | `2c0de452-34da-40a2-afad-49cf9b047e71` | `media-originals/cbfda205-98da-46d8-8698-4f263ba339ed/da348e40-3d6f-4486-b151-1d37db98976f.mp4` | `2026-11-01T18:59:15.577333916Z` | `2026-11-01T19:02:00.081046808Z` | `164503` | `37` | `completed` |

