# Evidencia F3 — Costos: región, tamaños, estimación con 1 y 3 instancias por grupo y consumo observado ([#221](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/221))

| | |
| :--- | :--- |
| **Bloque** | F — Seguridad, operación y costos |
| **Rúbrica** | Documentación y deuda |
| **Responsable** | `@CrispisCas9` |
| **Depende de** | C1, C2, C5, C6, D1 — cerrados |
| **Desbloquea** | I2 (#232), I6 (#236) |

**Estado: completo.** Estimación reproducible, contrastada tarifa por tarifa con lo facturado el 29 de octubre
(que corrigió el precio de las reglas de reenvío), y presupuesto activo comprobado.

Operar la arquitectura elástica 24×7 cuesta **168,69 USD/mes con los dos grupos
a 1 instancia y 329,23 USD/mes con los dos a 3**, frente a 133,73 USD/mes de la
Entrega 2. Aparcado, el entorno cuesta **0,33 USD/día** con la cola parada y
**0,74 USD/día** con la cola encendida. Una sesión de pruebas de 2 h con el
generador, **0,74 USD**. **Del cupón activo quedan unos 10,40 USD** (§6).

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
una `e2-highcpu-2` y 49,31 de Cloud SQL) y lo facturado por las reglas de reenvío el 2026-10-29 (§5).

Supuestos de volumen, que pesan centavos: 3 GiB en buckets y 5 GiB/mes de HLS
hacia Sudamérica (los de la Entrega 2), 5 GiB/mes procesados por el balanceador,
2 GiB/mes por Cloud NAT (paquetes, `git clone` y correo de las instancias
nuevas; Artifact Registry y las APIs de Google van por Private Google Access, no
por NAT). Durante una corrida de carga el tráfico del generador también pasa por
el NAT (nota 57), pero son lecturas pequeñas: la de G5 procesa menos de 1 GiB,
unos 4 centavos y 1 GiB de copias y registros de PITR.

#### Escenario 1: los dos grupos a 1 instancia

| Concepto | USD/h | USD/mes (730 h) | Nota |
| :--- | ---: | ---: | :--- |
| Grupo de la API: 1 x e2-highcpu-2 + 30 GiB (incluye el frontend) | 0,0536 | 39,11 | MIG |
| Grupo de workers: 1 x e2-highcpu-2 + 30 GiB | 0,0536 | 39,11 | MIG |
| Cola: mooc-queue-server, e2-small + 10 GiB | 0,0181 | 13,23 | NO es de ningun grupo |
| Balanceador: reglas de reenvio (HTTPS y redireccion; tarifa minima de hasta 5) | 0,0250 | 18,25 | NO es instancia: factura en reposo |
| Balanceador: datos procesados | 0,0001 | 0,08 | supuesto de volumen |
| Cloud SQL: 1 vCPU, 3,75 GiB, zonal | 0,0675 | 49,31 |  |
| Cloud SQL: 10 GiB SSD + copias y PITR | 0,0024 | 1,78 |  |
| Cloud NAT: pasarela | 0,0042 | 3,07 | por VM que sale por ella |
| Cloud NAT: 1 IP externa de la pasarela | 0,0050 | 3,65 | una IP para ~8 VM a 2 048 puertos cada una (nota 57) |
| Cloud NAT: datos procesados | 0,0001 | 0,09 | supuesto de volumen |
| Cloud Storage: 2 buckets STANDARD | 0,0001 | 0,06 | supuesto de volumen |
| Salida a internet: HLS a Sudamerica | 0,0013 | 0,95 | supuesto de volumen |
| Monitoring: metrica personalizada de la cola | 0,0000 | 0,00 | NO es instancia; dentro de lo gratuito |
| **Total** | **0,2311** | **168,69** | |

#### Escenario 3: los dos grupos a 3 instancias

| Concepto | USD/h | USD/mes (730 h) | Nota |
| :--- | ---: | ---: | :--- |
| Grupo de la API: 3 x e2-highcpu-2 + 30 GiB (incluye el frontend) | 0,1607 | 117,34 | MIG |
| Grupo de workers: 3 x e2-highcpu-2 + 30 GiB | 0,1607 | 117,34 | MIG |
| Cola: mooc-queue-server, e2-small + 10 GiB | 0,0181 | 13,23 | NO es de ningun grupo |
| Balanceador: reglas de reenvio (HTTPS y redireccion; tarifa minima de hasta 5) | 0,0250 | 18,25 | NO es instancia: factura en reposo |
| Balanceador: datos procesados | 0,0001 | 0,08 | supuesto de volumen |
| Cloud SQL: 1 vCPU, 3,75 GiB, zonal | 0,0675 | 49,31 |  |
| Cloud SQL: 10 GiB SSD + copias y PITR | 0,0024 | 1,78 |  |
| Cloud NAT: pasarela | 0,0098 | 7,15 | por VM que sale por ella |
| Cloud NAT: 1 IP externa de la pasarela | 0,0050 | 3,65 | una IP para ~8 VM a 2 048 puertos cada una (nota 57) |
| Cloud NAT: datos procesados | 0,0001 | 0,09 | supuesto de volumen |
| Cloud Storage: 2 buckets STANDARD | 0,0001 | 0,06 | supuesto de volumen |
| Salida a internet: HLS a Sudamerica | 0,0013 | 0,95 | supuesto de volumen |
| Monitoring: metrica personalizada de la cola | 0,0000 | 0,00 | NO es instancia; dentro de lo gratuito |
| **Total** | **0,4510** | **329,23** | |

Pasar de 1 a 3 instancias por grupo cuesta **0,2199 USD/h** mas (+95 %): solo crecen las instancias y su paso por Cloud NAT; todo lo demas es fijo.

#### Sesion de pruebas: escenario 1 mas el generador

| Concepto | USD/h | USD/mes (730 h) | Nota |
| :--- | ---: | ---: | :--- |
| Grupo de la API: 1 x e2-highcpu-2 + 30 GiB (incluye el frontend) | 0,0536 | 39,11 | MIG |
| Grupo de workers: 1 x e2-highcpu-2 + 30 GiB | 0,0536 | 39,11 | MIG |
| Cola: mooc-queue-server, e2-small + 10 GiB | 0,0181 | 13,23 | NO es de ningun grupo |
| Balanceador: reglas de reenvio (HTTPS y redireccion; tarifa minima de hasta 5) | 0,0250 | 18,25 | NO es instancia: factura en reposo |
| Balanceador: datos procesados | 0,0001 | 0,08 | supuesto de volumen |
| Cloud SQL: 1 vCPU, 3,75 GiB, zonal | 0,0675 | 49,31 |  |
| Cloud SQL: 10 GiB SSD + copias y PITR | 0,0024 | 1,78 |  |
| Cloud NAT: pasarela | 0,0056 | 4,09 | por VM que sale por ella |
| Cloud NAT: 1 IP externa de la pasarela | 0,0050 | 3,65 | una IP para ~8 VM a 2 048 puertos cada una (nota 57) |
| Cloud NAT: datos procesados | 0,0001 | 0,09 | supuesto de volumen |
| Cloud Storage: 2 buckets STANDARD | 0,0001 | 0,06 | supuesto de volumen |
| Salida a internet: HLS a Sudamerica | 0,0013 | 0,95 | supuesto de volumen |
| Monitoring: metrica personalizada de la cola | 0,0000 | 0,00 | NO es instancia; dentro de lo gratuito |
| Generador de carga: e2-standard-4 + 30 GiB | 0,1381 | 100,83 | solo durante las pruebas |
| **Total** | **0,3706** | **270,54** | |
| Una sesion de 2 h | | **0,74** | 2 h |
| Una sesion de 3 h | | **1,11** | 3 h |

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
| **Reglas de reenvío del balanceador** | 0,025 USD/h por el conjunto, **18,25 USD/mes** | Facturan **por hora, sirvan o no tráfico** (tarifa mínima por hasta cinco reglas, §5). Es la mitad de un grupo a 1 instancia, y aun así lo tercero más caro del entorno mínimo |
| **`mooc-queue-server`** | 0,018 USD/h, 13,23 USD/mes | No pertenece a ningún grupo: `resize --size=0` no la toca y se queda encendida si nadie la para. Aparcado, es **más de la mitad** del costo diario |
| **Métrica personalizada de C6** | ~0 USD | 4 series a un punto cada 30 s son ~2,7 MiB/mes. Con las métricas del agente, el volumen total fue de 3,04 MiB en un día (§5): sigue dentro de los 150 MiB gratuitos al mes |

## 3. Qué cuesta el escalamiento, frente a la Entrega 2

| | Entrega 2 (24×7) | Entrega 3, 1 instancia por grupo | Entrega 3, 3 por grupo |
| :--- | ---: | ---: | ---: |
| Total | 133,73 | **168,69** | **329,23** |
| Diferencia con la Entrega 2 | — | +34,96 (+26 %) | +195,50 (+146 %) |

Los **34,96 USD/mes** de la arquitectura mínima no son instancias: con un grupo
de cada a 1 hay las mismas dos `e2-highcpu-2` que en la Entrega 2. Son la **entrada
y la cola**:

| Concepto nuevo | USD/mes |
| :--- | ---: |
| Reglas de reenvío del balanceador | +18,25 |
| VM dedicada de la cola | +13,23 |
| Cloud NAT (pasarela, su IP y datos) | +6,81 |
| Las dos IPv4 externas de la Entrega 2, retiradas | −3,65 |
| Copias de Cloud SQL, datos del balanceador, egreso al precio actual | +0,32 |
| **Total** | **+34,96** |

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
> se borran las reglas de reenvío. Mientras la regla existe no paga. **La factura
> del 29 de octubre lo confirma**: 10,75 h de IP sin uso, justo las horas sin reglas (§5). Sigue
> siendo la decisión correcta conservarla —rehacerla cambia el dominio y obliga
> a reaprovisionar el certificado, hasta una hora—, pero hay que contarla: es
> **el 72 %** del costo aparcado con la cola parada.

Lo que no está en las tablas por ser marginal: **las imágenes de Artifact
Registry**. Medido el 2026-11-01: el repositorio `mooc` ocupa **1 287 MB**, con
40 versiones de `api`, 39 de `worker` y 12 de `web`. Por encima de los 0,5 GiB
gratuitos, a 0,10 USD/GiB-mes, son **unos 0,07 USD/mes**, aparcado o encendido.
Crece unos 30 MB por cada commit publicado; no merece una política de limpieza
antes de I6, que lo borra todo.

```bash
gcloud artifacts repositories describe mooc --location=us-east1 \
  --project=plataforma-mooc-entrega2 --format="value(sizeBytes)"
```

## 5. Consumo observado

**Lo facturado el 2026-10-29** —el día de las sesiones de C5, C6 y C7— exportado de Consola › Facturación › Informes,
agrupado por SKU y **sin créditos** (costo bruto): [`consumo_2026-10-29.csv`](./consumo_2026-10-29.csv). Total
**3,45 USD**, cubierto entero por el cupón (−3,45 de créditos): Compute Engine 1,85, Cloud SQL 0,95 y redes 0,65.

### Tarifa por tarifa, contra la estimación

Lo que se contrasta no es el total del día —que depende de cuántas horas estuvo encendido cada recurso— sino **el
precio unitario que salió en la factura frente al que usa la estimación**. Si la tarifa cuadra, la estimación del §2
vale para cualquier número de horas.

| SKU facturado | Uso | USD | USD por unidad | En la estimación | |
| :--- | ---: | ---: | ---: | ---: | :---: |
| Cloud SQL, vCPU zonal | 13,26 h | 0,55 | 0,0415 /h | 0,0413 /h | ✅ |
| Cloud SQL, RAM zonal | 49,73 GiB·h | 0,35 | 0,0070 /GiB·h | 0,007 /GiB·h | ✅ |
| E2, núcleo | 60,4 h | 1,32 | 0,0219 /vCPU·h | `e2-highcpu-2` = 0,0495 /h | ✅ |
| E2, RAM | 80,74 GiB·h | 0,24 | 0,0030 /GiB·h | (incluida arriba) | ✅ |
| Disco `pd-balanced` | 1,76 GiB·mes | 0,18 | 0,102 /GiB·mes | 0,10 | ✅ |
| Cloud NAT, pasarela | 40,43 VM·h | 0,06 | 0,0015 /VM·h | 0,0014 | ✅ |
| Cloud NAT, IP | 13,58 h | 0,07 | 0,0052 /h | 0,005 | ✅ |
| Cloud NAT, datos | 1,17 GiB | 0,05 | 0,043 /GiB | 0,045 | ✅ |
| **IP estática sin uso** | **10,75 h** | **0,11** | **0,0102 /h** | **0,01** | ✅ confirma el §4: son justo las horas del día **sin** reglas de reenvío (24 − 13,23) |
| **Reglas de reenvío, tarifa mínima global** | **13,23 h** | **0,33** | **0,025 /h** | **0,05 /h (0,025 × 2)** | ❌ **corregida** |
| Métricas de Monitoring | 3,04 MiB | 0,00 | — | gratis hasta 150 MiB/mes | ✅ |
| Network Intelligence Center (3 SKU) | 49 recursos·h c/u | 0,14 | — | **no estimado** | ⚠️ |

### Lo que la factura corrigió

**Las reglas de reenvío se cobran por el conjunto, no por regla.** La tarifa de 0,025 USD/h es el **mínimo por las
primeras cinco reglas**, y el SKU se llama exactamente así, *Forwarding Rule Minimum Global*: con las dos reglas
encendidas 13,23 h se facturaron 0,33 USD, es decir 0,025 /h por las dos. La estimación las contaba por separado y
**duplicaba ese concepto**: son **18,25 USD/mes y no 36,50**. Corregido en el script y en las tablas del §2; la
misma cifra de 1,20 USD/día que circulaba desde C3 (nota 38) era el doble de lo real, 0,60.

**Dos cosas que la estimación no tenía:**

- **Network Intelligence Center**, 0,14 USD el día observado: Network Analyzer, la topología y el rendimiento de
  internet a Google Cloud, a 49 recursos·hora cada uno. Factura por recurso supervisado, no por uso. Si nadie lo
  consulta, desactivar el análisis del proyecto lo ahorra; se anota para I6.
- **Volumen de métricas**: 3,04 MiB en un día, frente a los ~2,7 MiB **al mes** de la métrica personalizada de la
  cola. El resto son las métricas del agente de las VM. Sigue siendo gratis —unos 90 MiB/mes a ese ritmo, dentro
  de los 150—, pero un mes con los grupos más tiempo encendidos podría pasar del tramo gratuito.

### La sesión de hoy (G3, G6, F1 y G5)

La facturación tarda hasta 24 horas en consolidarse: el 2026-11-01 no se exporta el mismo día. Cuando esté, se
añade aquí con el mismo procedimiento —Consola › Facturación › Informes, intervalo del día, por SKU, sin créditos,
**Descargar CSV**— y debería confirmar las mismas tarifas.

## 6. Alertas de presupuesto

**Activo**, comprobado el 2026-11-01 en Consola › Facturación › Presupuestos y límites:
[`capturas/2026-11-01_presupuesto-activo.png`](./capturas/2026-11-01_presupuesto-activo.png).

| | Configurado | Comentario |
| :--- | :--- | :--- |
| Nombre | *Entrega 2 - cupon activo* | |
| Importe | **50 USD** | el cupón activo |
| Periodo | desde el 26 sept 2026, **sin fecha de fin** | equivale al rango personalizado de la Entrega 2: acumula y no se reinicia el día 1 |
| Alertas | **25, 50, 90 y 100 %** | la Entrega 2 documentaba 80 % en vez de 90 %; hoy está en 90 % |
| Créditos | **no se usaron créditos** | mide el consumo bruto, como debe |
| **Gastado** | **39,60 USD de 50 (79 %)** | |

> **Atención al equipo: quedan unos 10,40 USD del cupón.** La alerta del 90 % salta a los 45 USD. Al ritmo
> observado —0,33–0,74 USD/día aparcado y 0,74–1,11 USD por sesión de pruebas, más días como hoy con tres
> sesiones— alcanza para las pruebas que faltan y la sustentación, **pero no para dejar el entorno encendido**: un día
> entero con los dos grupos a 1 instancia son unos 5,55 USD. **Hay que aparcar al terminar cada sesión.** Si hace
> falta más margen, hay que activar el siguiente cupón antes de la sustentación.

La consola ofrece ahora también **límites de gasto** (*spend caps*) que pausan servicios al superar el importe; el
presupuesto no lo tiene activado («No aplicable»). La Entrega 2 los descartó porque no cubrían Compute Engine ni
Cloud SQL. No se activa sin decisión del equipo: pausar la base o los grupos en mitad de una prueba o de la
sustentación sería peor que el aviso.

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
