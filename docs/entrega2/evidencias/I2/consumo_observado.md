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

## Factura real (informe de Facturación)

Leído por Tania Díaz el **2026-09-27** (noche) en Facturación → Informes, agrupado por servicio, intervalo por período de
cargo **15 a 27 de septiembre de 2026**, cuenta *Billing Account for Education*. Columna «Costo por uso» (bruto, antes de
créditos):

| Servicio | Costo por uso (USD) | Ahorros / créditos (USD) | Subtotal (USD) |
| :--- | ---: | ---: | ---: |
| Cloud SQL | 1,13 | −1,12 | 0,00 |
| Compute Engine | 0,35 | −0,35 | 0,00 |
| Networking | 0,06 | −0,06 | 0,00 |
| Artifact Registry | 0,00 | — | 0,00 |
| **Total** | **1,54** | **−1,53** | **0,00** |

El resumen del propio informe dice: «Invertiste $0.00 … Ahorraste −$1.53 en este período». **El costo bruto acumulado hasta la
fecha del informe es de ≈ 1,54 USD, cubierto íntegramente por el cupón: 0,00 USD de gasto real.**

### Lectura frente a B1

| Servicio | B1: costo mensual 24×7 | Factura, 15–27 sept. (bruto) | Comentario |
| :--- | ---: | ---: | :--- |
| Compute Engine (2 VM + discos) | 78,22 | 0,35 | Consistente con un uso de decenas de horas, no de un mes |
| Cloud SQL (instancia + disco) | 51,01 | 1,13 | A ≈ 0,070 USD/h (51,01 ÷ 730) equivale a ≈ 16 h de instancia encendida (derivado, no medido) |
| Networking (IPs, egreso) | 4,41 (IPs 3,65 + egreso 0,76) | 0,06 | Órdenes de magnitud por debajo |
| Cloud Storage | 0,08 | (no aparece: < 0,005) | — |
| Artifact Registry | 0,00 | 0,00 | Bajo el umbral gratuito (415,5 MB de 500 MB) |
| **Total** | **133,73 / mes** | **1,54 en ≈ 13 días** | **≈ 1,2 % de la estimación mensual** |

**Cómo leer esta comparación, con sus límites:**

- No son periodos equivalentes: B1 es un mes de operación 24×7 y la factura cubre ≈ 13 días de uso intermitente con la política
  de encendido y apagado. Lo que demuestra es que **esa política funciona**: la estimación de 133,73 USD/mes es un techo de
  operación continua, no lo que se gastó.
- **Los datos de facturación llevan retraso** (los últimos 1–2 días se completan después): el informe solo muestra costos
  hasta el 27 de septiembre y las VMs actuales se crearon ese día. El costo de cómputo real será mayor al 0,35 mostrado;
  a precio de lista, ≈ 23 h de VM son ≈ 1,2 USD (tabla anterior). **Conviene volver a leer el informe pasados 2–3 días**
  y anotar aquí la cifra definitiva.
- El informe se leyó con el filtro *Ahorros (todos)* activo; se usó la columna «Costo por uso», que ya es bruta.

## Lo que no se pudo medir

- **Factura definitiva.** La leída arriba llega solo hasta el 27 de septiembre por el retraso de facturación; hay que
  releerla pasados 2–3 días.
- **Horas de Cloud SQL medidas.** La métrica de disponibilidad no permite reconstruir el total de horas encendida; solo
  se derivó del costo (≈ 16 h).
- **Presupuesto de 50 USD con alertas.** Diseñado en B1; no se pudo confirmar que esté activo.

## Advertencias sobre los datos

- Los objetos de la nube son pocos porque **no se sembraron allí los tres perfiles de video de G1** (esa corrida,
  765 MB, fue local). No representa el volumen que tendría un uso real.
- El egreso a internet no distingue tráfico interno (métrica de la VM): por eso es una cota superior.
- Artifact Registry ya está en 415,5 MB, al 83 % del medio gigabyte gratuito (un tag inmutable por commit).
