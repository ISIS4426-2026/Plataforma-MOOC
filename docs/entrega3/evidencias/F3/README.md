# Evidencia F3 — Costos: región, tamaños, estimación con 1 y 3 instancias por grupo y consumo observado ([#221](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/221))

| | |
| :--- | :--- |
| **Bloque** | F — Seguridad, operación y costos |
| **Rúbrica** | Documentación y deuda |
| **Responsable** | `@CrispisCas9` |
| **Depende de** | C1, C2, C5, C6, D1 — cerrados |
| **Desbloquea** | I2 (#232), I6 (#236) |

**Estado: estimación hecha y reproducible; faltan el consumo facturado y la
comprobación de las alertas, que salen de la consola de facturación (§5 y §6).**

Operar la arquitectura elástica 24×7 cuesta **183,29 USD/mes con los dos grupos
a 1 instancia y 343,83 USD/mes con los dos a 3**, frente a 133,73 USD/mes de la
Entrega 2. Aparcado, el entorno cuesta **0,33 USD/día** con la cola parada y
**0,74 USD/día** con la cola encendida. Una sesión de pruebas de 2 h con el
generador, **0,78 USD**.

---

## 1. Región y tamaños efectivos

Desplegados, no previstos: salen de `infra/terraform/` en `main` (2026-11-01).

| Componente | Recurso | Tamaño | Zona |
| :--- | :--- | :--- | :--- |
| Grupo de la API (incluye el frontend) | `mooc-api-group` | 1–3 × `e2-highcpu-2` (2 vCPU, 2 GiB), 30 GiB `pd-balanced` | `us-east1-b` |
| Grupo de workers | `mooc-worker-group` | 1–3 × `e2-highcpu-2` (2 vCPU, 2 GiB), 30 GiB `pd-balanced` | `us-east1-b` |
| Cola | `mooc-queue-server` | `e2-small` (2 vCPU compartidas, 2 GiB), 10 GiB `pd-balanced` | `us-east1-b` |
| Base de datos | `mooc-db-1` | Cloud SQL Enterprise, `db-custom-1-3840` (1 vCPU, 3,75 GiB), 10 GiB SSD, zonal | `us-east1-b` |
| Balanceador | `mooc-api-https`, `mooc-api-http` | 2 reglas de reenvío globales | global |
| Generador (solo en pruebas) | `mooc-load-generator` | `e2-standard-4` (4 vCPU, 16 GiB), 30 GiB | `us-east1-c` |
| Objetos | `…-media`, `…-hls` | STANDARD | `us-east1` |

El **frontend no añade infraestructura**: lo sirve el nginx de cada réplica de
la API (D1), así que su costo está dentro del del grupo de la API.

## 2. Estimación

**Fecha:** 2026-11-01. **Horas:** 730 h/mes por recurso, la misma ventana 24×7
que la Entrega 2, para poder compararlas. **Precios:** de lista, `us-east1`, sin
descuentos por uso comprometido ni créditos (§7). **Generadas por**
[`scripts/f3_estimacion_costos.py`](../../../../scripts/f3_estimacion_costos.py):
los precios y tamaños están declarados arriba del todo del script, con su fuente,
y `--autoprueba` comprueba que reproduce las cifras de la Entrega 2 (39,11 USD de
una `e2-highcpu-2` y 49,31 de Cloud SQL) y las 1,20 USD/día de las reglas (C3).

Supuestos de volumen, que pesan centavos: 3 GiB en buckets y 5 GiB/mes de HLS
hacia Sudamérica (los de la Entrega 2), 5 GiB/mes procesados por el balanceador,
2 GiB/mes por Cloud NAT (paquetes, `git clone` y correo de las instancias
nuevas; Artifact Registry y las APIs de Google van por Private Google Access, no
por NAT) y 1 GiB de copias y registros de PITR.

#### Escenario 1: los dos grupos a 1 instancia

| Concepto | USD/h | USD/mes (730 h) | Nota |
| :--- | ---: | ---: | :--- |
| Grupo de la API: 1 x e2-highcpu-2 + 30 GiB (incluye el frontend) | 0,0536 | 39,11 | MIG |
| Grupo de workers: 1 x e2-highcpu-2 + 30 GiB | 0,0536 | 39,11 | MIG |
| Cola: mooc-queue-server, e2-small + 10 GiB | 0,0181 | 13,23 | NO es de ningun grupo |
| Balanceador: 2 reglas de reenvio (HTTPS y redireccion) | 0,0500 | 36,50 | NO es instancia: factura en reposo |
| Balanceador: datos procesados | 0,0001 | 0,08 | supuesto de volumen |
| Cloud SQL: 1 vCPU, 3,75 GiB, zonal | 0,0675 | 49,31 |  |
| Cloud SQL: 10 GiB SSD + copias y PITR | 0,0024 | 1,78 |  |
| Cloud NAT: pasarela | 0,0042 | 3,07 | por VM que sale por ella |
| Cloud NAT: datos procesados | 0,0001 | 0,09 | supuesto de volumen |
| Cloud Storage: 2 buckets STANDARD | 0,0001 | 0,06 | supuesto de volumen |
| Salida a internet: HLS a Sudamerica | 0,0013 | 0,95 | supuesto de volumen |
| Monitoring: metrica personalizada de la cola | 0,0000 | 0,00 | NO es instancia; dentro de lo gratuito |
| **Total** | **0,2511** | **183,29** | |

#### Escenario 3: los dos grupos a 3 instancias

| Concepto | USD/h | USD/mes (730 h) | Nota |
| :--- | ---: | ---: | :--- |
| Grupo de la API: 3 x e2-highcpu-2 + 30 GiB (incluye el frontend) | 0,1607 | 117,34 | MIG |
| Grupo de workers: 3 x e2-highcpu-2 + 30 GiB | 0,1607 | 117,34 | MIG |
| Cola: mooc-queue-server, e2-small + 10 GiB | 0,0181 | 13,23 | NO es de ningun grupo |
| Balanceador: 2 reglas de reenvio (HTTPS y redireccion) | 0,0500 | 36,50 | NO es instancia: factura en reposo |
| Balanceador: datos procesados | 0,0001 | 0,08 | supuesto de volumen |
| Cloud SQL: 1 vCPU, 3,75 GiB, zonal | 0,0675 | 49,31 |  |
| Cloud SQL: 10 GiB SSD + copias y PITR | 0,0024 | 1,78 |  |
| Cloud NAT: pasarela | 0,0098 | 7,15 | por VM que sale por ella |
| Cloud NAT: datos procesados | 0,0001 | 0,09 | supuesto de volumen |
| Cloud Storage: 2 buckets STANDARD | 0,0001 | 0,06 | supuesto de volumen |
| Salida a internet: HLS a Sudamerica | 0,0013 | 0,95 | supuesto de volumen |
| Monitoring: metrica personalizada de la cola | 0,0000 | 0,00 | NO es instancia; dentro de lo gratuito |
| **Total** | **0,4710** | **343,83** | |

Pasar de 1 a 3 instancias por grupo cuesta **0,2199 USD/h** mas (+88 %): solo crecen las instancias y su paso por Cloud NAT; todo lo demas es fijo.

#### Sesion de pruebas: escenario 1 mas el generador

| Concepto | USD/h | USD/mes (730 h) | Nota |
| :--- | ---: | ---: | :--- |
| Grupo de la API: 1 x e2-highcpu-2 + 30 GiB (incluye el frontend) | 0,0536 | 39,11 | MIG |
| Grupo de workers: 1 x e2-highcpu-2 + 30 GiB | 0,0536 | 39,11 | MIG |
| Cola: mooc-queue-server, e2-small + 10 GiB | 0,0181 | 13,23 | NO es de ningun grupo |
| Balanceador: 2 reglas de reenvio (HTTPS y redireccion) | 0,0500 | 36,50 | NO es instancia: factura en reposo |
| Balanceador: datos procesados | 0,0001 | 0,08 | supuesto de volumen |
| Cloud SQL: 1 vCPU, 3,75 GiB, zonal | 0,0675 | 49,31 |  |
| Cloud SQL: 10 GiB SSD + copias y PITR | 0,0024 | 1,78 |  |
| Cloud NAT: pasarela | 0,0056 | 4,09 | por VM que sale por ella |
| Cloud NAT: datos procesados | 0,0001 | 0,09 | supuesto de volumen |
| Cloud Storage: 2 buckets STANDARD | 0,0001 | 0,06 | supuesto de volumen |
| Salida a internet: HLS a Sudamerica | 0,0013 | 0,95 | supuesto de volumen |
| Monitoring: metrica personalizada de la cola | 0,0000 | 0,00 | NO es instancia; dentro de lo gratuito |
| Generador de carga: e2-standard-4 + 30 GiB | 0,1381 | 100,83 | solo durante las pruebas |
| **Total** | **0,3906** | **285,14** | |
| Una sesion de 2 h | | **0,78** | 2 h |
| Una sesion de 3 h | | **1,17** | 3 h |

#### Entorno aparcado, con la cola parada

| Concepto | USD/h | USD/mes (730 h) | Nota |
| :--- | ---: | ---: | :--- |
| Direccion global del balanceador, reservada sin regla | 0,0100 | 7,30 | se conserva a proposito |
| Certificado administrado, mapas, proxies, backend | 0,0000 | 0,00 | sin coste por hora |
| Cloud SQL detenida: 10 GiB SSD + copias | 0,0024 | 1,78 | el computo deja de facturar |
| Cloud Storage: 2 buckets | 0,0001 | 0,06 |  |
| Disco de la cola, 10 GiB | 0,0014 | 1,00 | aunque la VM este parada |
| **Total** | **0,0139** | **10,14** | |
| Por dia | | **0,33** | 24 h |

#### Entorno aparcado, con la cola encendida

| Concepto | USD/h | USD/mes (730 h) | Nota |
| :--- | ---: | ---: | :--- |
| Direccion global del balanceador, reservada sin regla | 0,0100 | 7,30 | se conserva a proposito |
| Certificado administrado, mapas, proxies, backend | 0,0000 | 0,00 | sin coste por hora |
| Cloud SQL detenida: 10 GiB SSD + copias | 0,0024 | 1,78 | el computo deja de facturar |
| Cloud Storage: 2 buckets | 0,0001 | 0,06 |  |
| Disco de la cola, 10 GiB | 0,0014 | 1,00 | aunque la VM este parada |
| VM de la cola encendida (e2-small) | 0,0168 | 12,23 | el resize --size=0 no la toca |
| **Total** | **0,0306** | **22,37** | |
| Por dia | | **0,74** | 24 h |

La metrica personalizada son ~2,7 MiB/mes (4 series, un punto cada 30 s, 8 bytes por punto), frente a 150 MiB gratuitos por cuenta de facturacion.


### Los tres que no son instancias de grupo, y facturan igual

| | Cuánto | Por qué se escapa |
| :--- | ---: | :--- |
| **Reglas de reenvío del balanceador** | 0,05 USD/h, **36,50 USD/mes** | Facturan **por hora, sirvan o no tráfico**. Con el entorno a 1 instancia son el **segundo concepto más caro** después de Cloud SQL, por encima de cada grupo |
| **`mooc-queue-server`** | 0,018 USD/h, 13,23 USD/mes | No pertenece a ningún grupo: `resize --size=0` no la toca y se queda encendida si nadie la para. Aparcado, es **más de la mitad** del costo diario |
| **Métrica personalizada de C6** | ~0 USD | 4 series a un punto cada 30 s son ~2,7 MiB/mes: caben más de 50 veces en los 150 MiB gratuitos. Pasaría a costar algo solo con cientos de series o con intervalos de segundos |

## 3. Qué cuesta el escalamiento, frente a la Entrega 2

| | Entrega 2 (24×7) | Entrega 3, 1 instancia por grupo | Entrega 3, 3 por grupo |
| :--- | ---: | ---: | ---: |
| Total | 133,73 | **183,29** | **343,83** |
| Diferencia con la Entrega 2 | — | +49,56 (+37 %) | +210,10 (+157 %) |

Los **49,56 USD/mes** de la arquitectura mínima no son instancias: con un grupo
de cada a 1 hay las mismas dos `e2-highcpu-2` que en la Entrega 2. Son la **entrada
y la cola**:

| Concepto nuevo | USD/mes |
| :--- | ---: |
| Reglas de reenvío del balanceador | +36,50 |
| VM dedicada de la cola | +13,23 |
| Cloud NAT (pasarela y datos) | +3,16 |
| Las dos IPv4 externas de la Entrega 2, retiradas | −3,65 |
| Copias de Cloud SQL, datos del balanceador, egreso al precio actual | +0,32 |
| **Total** | **+49,56** |

Lo que añade **escalar** es otra cosa: cada instancia más de cualquiera de los
dos grupos son 0,055 USD/h (39,11 USD/mes más 1 USD de NAT). Con un ciclo como
el de G5 —una instancia extra durante ~30 min— el escalamiento cuesta **unos 3
centavos**. El costo de la elasticidad está en mantenerla disponible, no en
usarla.

## 4. El entorno aparcado no cuesta cero

Al aparcar (runbook de C6, §8) se van los autoescaladores y las reglas de
reenvío, y los grupos bajan a 0. **Se quedan**, y facturan: la dirección global
reservada, el disco de la base detenida y sus copias, los buckets, el disco de la
cola y, si nadie la para, la propia VM de la cola. Las tablas del §2 lo
desglosan: **0,33 USD/día** con la cola parada, **0,74 USD/día** encendida.

> **Corrección a la nota 38.** La nota dice que dirección, certificado, mapas y
> proxies «no tienen coste por hora». El certificado, los mapas y los proxies
> no; **la dirección sí**: una IP externa estática reservada **sin uso** se
> factura a **0,01 USD/h** (0,24 USD/día), y deja de estar en uso justo cuando
> se borran las reglas de reenvío. Mientras la regla existe no paga. Sigue
> siendo la decisión correcta conservarla —rehacerla cambia el dominio y obliga
> a reaprovisionar el certificado, hasta una hora—, pero hay que contarla: es
> **el 72 %** del costo aparcado con la cola parada.

Lo que no está en la tabla porque falta medirlo: **las imágenes de Artifact
Registry** (0,10 USD/GiB-mes por encima de 0,5 GiB gratis). Cada commit
publicado sube tres imágenes, y la del worker lleva FFmpeg. Se mide con:

```bash
gcloud artifacts repositories describe mooc --location=us-east1 \
  --project=plataforma-mooc-entrega2 --format="value(sizeBytes)"
```

## 5. Consumo observado

*Pendiente de la consola de facturación.* Dos sesiones del 29 de octubre dan
costo real sin gastar nada nuevo: la de **C5 y C6** (ciclo de autoescalado
completo, 1 → 3 → 1 en 33 min) y la de **C7**. Para sacarlo:

1. Consola › **Facturación › Informes**. Proyecto `plataforma-mooc-entrega2`,
   intervalo **2026-10-29 a 2026-10-29**, agrupar por **SKU**, y **desmarcar
   los créditos** (mismo motivo que el presupuesto, §6).
2. Captura → `capturas/2026-10-29_facturacion-por-sku.png`, y **Descargar CSV**
   → `consumo_2026-10-29.csv` en esta carpeta.
3. Lo mismo para la sesión de G5 y F1 cuando se haga.

Y el contraste, que es lo que pide el criterio: horas medidas × tarifa del §2
frente a lo facturado, SKU por SKU. Dos diferencias que hay que esperar y
explicar:

| Diferencia esperada | Por qué |
| :--- | :--- |
| Cloud SQL factura **menos** de lo que dice `C1/coste_sesion.txt` | Ese archivo usa 0,0855 USD/h; la tarifa de lista de 1 vCPU y 3,75 GiB es **0,0676 USD/h** (§2). El de C1 sobrestimaba un 27 % |
| Aparece un SKU de **IP estática sin uso** los días aparcados | El §4: 0,24 USD/día que la nota 38 no contaba |

## 6. Alertas de presupuesto

*Pendiente de comprobar en la consola.* La configuración que se estableció en la
Entrega 2
([`CONFIGURACION_Y_COSTOS.md` §5](../../../entrega2/CONFIGURACION_Y_COSTOS.md)):
50 USD en un **rango personalizado** —no mensual, porque el cupón no se reinicia
el día 1—, umbrales del 25, 50, 80 y 100 % y los **créditos excluidos**, para
que avise mientras se gasta el cupón y no cuando ya se acabó.

Para la arquitectura nueva los umbrales siguen valiendo: con el costo diario
real (sesiones de 1–2 USD y 0,33–0,74 USD/día aparcado), el 25 % de 50 USD llega
en semanas, no en horas, y da margen. Lo que sí hay que revisar:

- [ ] El presupuesto sigue **activo** y su rango cubre noviembre: Consola ›
      Facturación › **Presupuestos y alertas**. Captura →
      `capturas/2026-11-01_presupuesto-activo.png`.
- [ ] El monto refleja el **cupón activo en este momento**, no el de septiembre.
- [ ] Siguen sin créditos (*Savings* desmarcado).

## 7. Fuentes de precios

Consultadas el 2026-11-01. Compute Engine, almacenamiento de objetos y egreso
de Compute se toman de la Entrega 2 y C1, y la autoprueba del script comprueba
que se reproducen.

- [Cloud SQL pricing](https://cloud.google.com/sql/pricing): 0,0413 USD por vCPU-h y 0,007 por GiB-h (Enterprise, zonal); SSD y copias por GiB-h.
- [Cloud Load Balancing pricing](https://cloud.google.com/load-balancing/pricing): 0,025 USD/h por regla de reenvío (primeras 5); 0,008 USD/GiB procesado.
- [Cloud NAT pricing](https://cloud.google.com/nat/pricing): 0,0014 USD/h por VM, hasta 32; 0,045 USD/GiB.
- [VPC network pricing](https://cloud.google.com/vpc/network-pricing): IP estática sin uso 0,01 USD/h; la de una regla de reenvío no paga; egreso Premium a Sudamérica 0,19 USD/GiB.
- [Cloud Monitoring pricing](https://cloud.google.com/monitoring/pricing): 8 bytes por punto escalar, 150 MiB/mes gratis, después 0,258 USD/MiB.

---

Este archivo no contiene credenciales, llaves ni secretos.
