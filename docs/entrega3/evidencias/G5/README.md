# Evidencia G5 — Escenario 3A: ciclo de escalamiento automático de la API ([#225](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/225))

| | |
| :--- | :--- |
| **Bloque** | G — Pruebas de carga comparativas |
| **Rúbrica** | Pruebas comparativas |
| **Responsable** | `@CrispisCas9` |
| **Depende de** | C2 (#205), C4 (#207), E1 (#216), G1 (#222) — todos cerrados |
| **Desbloquea** | G7 (#227) |
| **Cómo se corre** | [`RUNBOOK.md`](./RUNBOOK.md), en la misma sesión que F1 |

**Estado: cumplido.** Corrida del **2026-11-01, 23:29–23:57 UTC**: el grupo de la API pasó **1 → 2 → 1** solo,
por decisión del autoescalador, con **reacción de 48 s** e **incorporación de 157 s**, **0 errores en 90 080
peticiones** y **11 de 11 comprobaciones** de [`linea_de_tiempo.md`](./linea_de_tiempo.md) en OK.

---

## Criterios de aceptación

| Criterio | Cómo se demuestra | Estado |
| :--- | :--- | :---: |
| Al menos una creación automática y una reducción posterior, observadas y fechadas | `hitos.tsv` del observador: `DECISION_AMPLIAR`, `INSTANCIA_CREADA` (con el `creationTimestamp` de GCE), `DECISION_REDUCIR`, `INSTANCIA_RETIRADA` | ✅ |
| Incorporación al balanceador demostrada con peticiones atendidas por la instancia nueva | el observador manda peticiones por el balanceador y agrupa por `X-Instance-ID`; como ese valor es `sha256(ID de la instancia)[:12]`, se sabe **qué** instancia respondió sin entrar por SSH | ✅ |
| Los dos tiempos —reacción e incorporación— reportados por separado | `linea_de_tiempo.md`: reacción = cruce del umbral en el `.jtl` → decisión; incorporación = decisión → primera petición atendida, desglosada en creada / `RUNNING` / sana / atiende | ✅ |
| La ampliación manual del grupo no aparece como evidencia | el observador se niega a empezar si el mínimo no es 1 o el grupo no está en 1; `g5_linea_de_tiempo.py` lee el **registro de auditoría** de la ventana y falla si una persona llamó a `resize`, `update-autoscaling` o similares | ✅ |
| **Bloquea el cierre:** capturas de la línea de tiempo, la decisión del autoescalador y la consola durante el ciclo completo | seis capturas, cada una anunciada por el observador en su momento (§5 del runbook) | ✅ |
| Lo que el equipo debe saber, en `NOTAS_TECNICAS.md` | nota 64 | ✅ |

## Diseño de la corrida

**La señal no es la CPU** (C2, nota 37): son peticiones por segundo y por
instancia que cuenta el balanceador, con objetivo 0,8 sobre
`max_rate_per_instance = 100`. Se añade una instancia al pasar de **80 pet/s por
instancia**. Por eso la carga no se define por usuarios sino por **tasa**: cinco
etapas seguidas, cada una con un *Constant Throughput Timer* compartido por su
grupo de hilos ([`escenario3a_api.jmx`](../../../../capacity-planning/escenario3a_api.jmx)).

| Etapa | Tasa | Duración | Para qué |
| :--- | ---: | ---: | :--- |
| e1 base | 10 pet/s | 5 min | arrancar con baja carga y una instancia |
| e2 subida | 40 pet/s | 2 min | aumentar, por debajo del umbral |
| e3 subida | 70 pet/s | 2 min | aumentar, cerca del umbral |
| e4 alta | 130 pet/s | 15 min | por encima de 80 con una instancia: tiene que pedir 2. Con dos quedan ~65 cada una, así que se estabiliza en vez de oscilar, y sostiene la carga durante el arranque |
| e5 bajada | 10 pet/s | 20 min | reducir la demanda y observar la reducción (una instancia cada 600 s como máximo) |

Por qué **130 y no 200**: el criterio pide «al menos una» instancia adicional, y
C2 extrapola que a ~240 pet/s la base de 1 vCPU se acerca a su techo. Llegar a 3
es otra medición, con otro coste; se puede pedir con `RATE_ALTA=200`.

Las peticiones son lecturas (catálogo y `/ready`), como la calibración de G1: a
la señal le da igual qué petición cuenta, y así se puede repetir sin ensuciar la
base. **La integridad de las escrituras** durante el ciclo la cubre el arnés de
consistencia de E2, lanzado con el grupo ya en 2 por el autoescalador
(`consistencia_durante_el_ciclo/`).

## Herramientas

| Archivo | Dónde corre | Qué hace |
| :--- | :--- | :--- |
| [`g5_carga_escalonada.sh`](../../../../scripts/g5_carga_escalonada.sh) | VM del generador | JMeter por digest (nota 49), las cinco etapas, `etapas.tsv` con la hora UTC planificada de cada una, `destino.json` y salud del generador |
| [`g5_observar_escalamiento_api.sh`](../../../../scripts/g5_observar_escalamiento_api.sh) | portátil | comprueba el punto de partida, sondea cada 15 s grupo, autoescalador, instancias y reparto, anuncia hitos y capturas, y **sale con 1** si no hubo ampliación, atención por la nueva y reducción |
| [`g5_linea_de_tiempo.py`](../../../../scripts/g5_linea_de_tiempo.py) | portátil | cruza `.jtl`, observación y auditoría; escribe `linea_de_tiempo.md`, `tasa_10s.csv` y `auditoria.tsv`, y **sale con 1** si algo no cuadra |

### Cómo se probaron sin gastar

- `g5_linea_de_tiempo.py --autoprueba`: un ciclo sintético con tiempos conocidos
  (reacción 75 s, incorporación 135 s, un 502 durante el retiro) y el mismo ciclo
  con la etapa alta recortada al 67 % de su tasa, que **tiene que fallar**. Pasa.
- El observador, contra un `gcloud` y un `curl` simulados que recorren 1 → 2 → 1:
  sale con 0 y los hitos salen en orden. Con un grupo que no crece sale con 1
  (cuatro comprobaciones fallidas), y con un `X-Instance-ID` que no corresponde a
  ninguna instancia del grupo se para en la fase 1.
- La lectura de auditoría, con un registro simulado que incluye un `resize` de
  una persona: lo detecta, falla, y el correo no aparece en ninguna salida.
- `g5_carga_escalonada.sh`, con un `docker` simulado: las propiedades que llegan
  a JMeter, `etapas.tsv` y `destino.json` son los esperados; con `RATE_ALTA=70`
  se niega a empezar.

Lo que **no** se pudo probar aquí es el plan de JMeter contra JMeter real: el
XML es válido y sigue la forma de `calibracion_generador.jmx`, pero la primera
comprobación de verdad es la etapa e1. `g5_linea_de_tiempo.py` compara la tasa
lograda con la pedida en cada etapa y falla si la alta no llega al 90 %.

## Resultado

Desde la VM del generador (`mooc-load-generator`, JMeter 5.5 por digest) contra `https://136.68.55.254.sslip.io`,
imagen `97120ca`. Por tiempo, la corrida se acortó respecto al plan: base 2 min, subidas 1 + 1 min, alta 10 min y
bajada 12 min (variables `DUR_*` del guion). Datos crudos en [`carga/escenario3a_20261101_232940/`](./carga/escenario3a_20261101_232940/)
—el `.jtl` original va comprimido— y en [`observacion_20261101/`](./observacion_20261101/). El generador **no
limitó** (`contraste_generador.json`).

### Los dos tiempos

| Tiempo | Desde | Hasta | Duración |
| :--- | :--- | :--- | ---: |
| **Reacción** | la carga supera 80 pet/s con una instancia (23:33:40) | el grupo crea la instancia nueva (23:34:28, `creationTimestamp`) | **48 s** |
| **Incorporación** | la decisión (23:34:28) | la instancia nueva responde peticiones que entran por el balanceador (23:37:05) | **157 s** |
| — de ellos | creada → `RUNNING` | | 19 s |
| — de ellos | `RUNNING` → sana ante el balanceador | | 138 s |
| **Reducción** | baja la carga (23:43:40) | el objetivo baja a 1 (23:54:53); la instancia sale del grupo a las 23:57:09 | 11 min 13 s hasta la decisión |

La reducción espera **unos 10 minutos de carga baja** antes de decidir: es la ventana de estabilización del
autoescalador más `scale_in_control` (C2), y es la razón por la que la bajada no se puede acortar.

### Carga por etapa

| Etapa | Pedida (pet/s) | Lograda (pet/s) | n | p50 / p95 / p99 (ms) | Errores |
| :--- | ---: | ---: | ---: | :---: | ---: |
| e1 base | 10 | 9,6 | 1 152 | 9 / 18 / 35 | 0 |
| e2 subida | 40 | 37,9 | 2 271 | 8 / 14 / 24 | 0 |
| e3 subida | 70 | 69,1 | 4 148 | 8 / 14 / 24 | 0 |
| **e4 alta** | **130** | **124,6** | 74 751 | 7 / 13 / 22 | 0 |
| e5 bajada | 10 | 10,7 | 7 712 | 9 / 18 / 28 | 0 |

**Integridad durante el ciclo:** 0 errores reales (5xx, timeouts, conexión) en 90 080 peticiones, también durante
la incorporación y durante el retiro. Las latencias no se movieron al entrar la segunda instancia: una sola atendía
124 pet/s con p95 de 13 ms y la API no pasó del 28 % de CPU. El ciclo demuestra el **mecanismo**; no hubo
saturación que la segunda instancia tuviera que aliviar, igual que en G3.

### Quién cambió el grupo

[`auditoria.tsv`](./auditoria.tsv): en la ventana, `instances.insert` (23:34:27) e `instances.delete` (23:56:36)
sobre `mooc-api-b18m`, ambos por el **agente de servicio del grupo administrado**. **Ninguna persona** cambió el
tamaño ni la política del grupo.

### Capturas

Las cinco primeras son la salida real del observador y del registro de auditoría, dibujada con
[`render_consola.py`](../../../../scripts/render_consola.py) sin editar el texto (el pie de cada una dice de qué
archivo y líneas sale); la sexta es el tablero de Cloud Monitoring de la ventana.

| Captura | Qué muestra |
| :--- | :--- |
| [`2026-11-01_g5-partida-1-instancia.png`](./capturas/2026-11-01_g5-partida-1-instancia.png) | 1 instancia sana, autoescalador `ACTIVE`, mínimo 1, todo el tráfico en un `X-Instance-ID` |
| [`2026-11-01_g5-autoescalador-amplia-a-2.png`](./capturas/2026-11-01_g5-autoescalador-amplia-a-2.png) | la decisión de ampliar y la creación de `mooc-api-b18m` |
| [`2026-11-01_g5-instancia-nueva-sana-y-reparto.png`](./capturas/2026-11-01_g5-instancia-nueva-sana-y-reparto.png) | la nueva sana y el reparto 5/5 entre los dos `X-Instance-ID` |
| [`2026-11-01_g5-reduccion-a-1.png`](./capturas/2026-11-01_g5-reduccion-a-1.png) | la reducción, la instancia retirada y las cinco comprobaciones del observador |
| [`2026-11-01_g5-auditoria-sin-intervencion-manual.png`](./capturas/2026-11-01_g5-auditoria-sin-intervencion-manual.png) | el registro de auditoría: solo el grupo administrado crea y borra |
| [`2026-11-01_g5-tablero-del-ciclo.png`](./capturas/2026-11-01_g5-tablero-del-ciclo.png) | instancias 1 → 2 → 1, CPU, cola y solicitudes por minuto en el balanceador |

---

Este archivo no contiene credenciales, llaves ni secretos.
