# Consumo observado frente a la estimación de B1 (issue #137, I2)

Medido el **2026-09-28** (≈ 03:50 UTC) con `gcloud` y la API de Cloud Monitoring, **solo lectura**, con la
identidad personal de quien lo midió. Estimación de referencia: B1, fechada **2026-09-25**
([`CONFIGURACION_Y_COSTOS.md` §4](../../CONFIGURACION_Y_COSTOS.md#4-estimación-de-costos)).

## Lo medido

| Concepto | Cómo se midió | Resultado |
| :--- | :--- | :--- |
| Horas de VM encendida | Métrica `compute.googleapis.com/instance/uptime`, suma por instancia desde 2026-09-15 | 4 instancias: 13,4 h (`mooc-web-server`), 9,0 h (`mooc-worker-server`), 0,4 h y 0,1 h (dos temporales) → **≈ 22,9 h** |
| Creación de las VMs actuales | `gcloud compute instances list` | Web 2026-09-27 07:30, Worker 2026-09-27 11:51 (hora local de quien midió), ambas `RUNNING` |
| Discos | `gcloud compute disks list` | 2 × 30 GiB `pd-balanced` |
| Bucket `media` | `gcloud storage du -s` | 5 008 934 B, 40 objetos |
| Bucket `hls` | `gcloud storage du -s` | 3 429 114 B, 14 objetos |
| Bucket `tfstate` | `gcloud storage du -s` | 115 179 B, 1 objeto |
| Artifact Registry `mooc` | `gcloud artifacts repositories describe` | **415,5 MB** |
| Cloud SQL, disco usado | Métrica `cloudsql.googleapis.com/database/disk/bytes_used` | 79 200 256 B (≈ 79 MB de 10 GiB) |
| Cloud SQL, perfil | `gcloud sql instances describe` | `db-custom-1-3840`, 10 GiB `PD_SSD`, `ZONAL`, copias activas 03:00 |
| Bytes servidos por buckets | Métrica `storage.googleapis.com/network/sent_bytes_count`, suma desde 2026-09-24 | `media` 2 983 031 B · `hls` 1 621 656 B · `tfstate` 9 335 423 B |
| Bytes enviados por las VMs | Métrica `compute.googleapis.com/instance/network/sent_bytes_count`, suma | 343 MB (web) y 249 MB (worker). **Incluye tráfico interno**, así que el egreso a internet es menor o igual |

## Contraste

| Concepto | B1 (2026-09-25) | Observado | Proporción |
| :--- | :--- | :--- | :--- |
| Horas de VM | 1 460 h/mes (2 × 730) | ≈ 22,9 h | ≈ 1,6 % |
| Costo de cómputo a precio de lista | 78,22 USD/mes (2 VM) | ≈ 1,2 USD (22,4 h × 0,0536 USD/h por VM, disco incluido) | — |
| Objetos | 3 GiB | ≈ 8,5 MB | 0,3 % |
| Egreso a internet | 5 GiB | ≤ ≈ 0,6 GB (cota superior) | ≤ 12 % |
| Disco de la base | 10 GiB aprovisionados | 79 MB | 0,8 % |

## Lo que no se pudo medir

- **Costo real de la factura.** Requiere el informe de Facturación de la cuenta (Facturación → Informes, agrupado por
  SKU y filtrado por el proyecto). Con la cuenta usada para medir, la API de presupuestos no está habilitada y no hay
  rol de lectura de facturación. **Quien lo tenga debe pegar aquí el costo acumulado por servicio.**
- **Horas de Cloud SQL.** La métrica de disponibilidad de la instancia no permite reconstruir el total de horas
  encendida desde su creación con la resolución necesaria.
- **Presupuesto de 50 USD con alertas.** Diseñado en B1; no se pudo confirmar que esté activo.

## Advertencias sobre los datos

- Los objetos de la nube son pocos porque **no se sembraron allí los tres perfiles de video de G1** (esa corrida,
  765 MB, fue local). No representa el volumen que tendría un uso real.
- El egreso a internet no distingue tráfico interno (métrica de la VM): por eso es una cota superior.
- Artifact Registry ya está en 415,5 MB, al 83 % del medio gigabyte gratuito (un tag inmutable por commit).
