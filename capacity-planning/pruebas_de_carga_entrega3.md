# Informe consolidado de pruebas de carga y capacidad — Entrega 3

Entregable 5 del enunciado, issue **G7** ([#227](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/227)).

> **2026-11-02.** Consolida la línea base retrospectiva (A7), el escenario 1 (G3), el escenario 2 (G4), el ciclo de
> la API (G5) y el ciclo de workers (G6). Cada cifra enlaza a la evidencia de la que sale.

| Sección | Fuente | Estado |
| :--- | :--- | :---: |
| [1. Método y diferencias declaradas](#1-método-y-diferencias-declaradas) | A6, A7, G1, G3 | ✅ |
| [2. Escenario 1: actividad académica concurrente](#2-escenario-1-actividad-académica-concurrente) | G3 | ✅ |
| [3. Escenario 2: carga, procesamiento y consumo multimedia](#3-escenario-2-carga-procesamiento-y-consumo-multimedia) | G4 | ✅ |
| [4. Escenario 3A: ciclo de escalamiento de la API](#4-escenario-3a-ciclo-de-escalamiento-de-la-api) | G5 | ✅ |
| [5. Escenario 3B: ciclo de escalamiento de workers](#5-escenario-3b-ciclo-de-escalamiento-de-workers) | G6 | ✅ |
| [6. Qué mejoró, qué quedó igual, qué empeoró](#6-qué-mejoró-qué-quedó-igual-qué-empeoró) | 2–5 | ✅ |
| [7. El escalamiento: cuándo, qué pasó mientras, qué lo limita y cuánto cuesta](#7-el-escalamiento-cuándo-qué-pasó-mientras-qué-lo-limita-y-cuánto-cuesta) | 2–5, F3 | ✅ |
| [8. Defectos de la Entrega 2 corregidos](#8-defectos-de-la-entrega-2-corregidos) | A5, A6, A7, G4, G6 | ✅ |
| [9. Limitaciones](#9-limitaciones) | todas | ✅ |
| [10. Revisión final de tablas](#10-revisión-final-de-tablas) | — | ✅ |

---

## 1. Método y diferencias declaradas

**Reglas de medición** ([`criterios_de_medicion.md`](./criterios_de_medicion.md), A6), implementadas en
[`scripts/capacity_criterios.py`](../scripts/capacity_criterios.py): un solo estimador de percentiles (rango más
cercano), **p95 solo con 20 observaciones y p99 con 100** (por debajo se publica `n/d (n=k)`), comprobación de
`p50 ≤ p95 ≤ p99` antes de publicar, destino de cada corrida registrado en `destino.json`, latencias HTTP nunca
mezcladas con la duración de trabajos asíncronos, y **percentiles nunca promediados entre corridas**: la variación
se da como media ± desviación estándar [mín–máx] del percentil de cada corrida.

**Generador** (G1): VM `mooc-load-generator`, `e2-standard-4`, `us-east1-c` (otra zona que la aplicación), JMeter
5.5 fijado por digest. Toda corrida publicada pasa `capacity_contraste_generador.py`, que la rechaza si la CPU del
generador, la brecha de latencia con el balanceador o el conteo de peticiones indican que el generador limitó.

**Qué cambia entre la Entrega 2 y la 3**, y por qué no todo se puede comparar:

| Diferencia | Línea base (A7) | Entrega 3 |
| :--- | :--- | :--- |
| Generador | portátil, red doméstica | VM en la región |
| Camino de red | directo a la VM | Cloud NAT de la VPC (2 048 puertos por VM, nota 57) → balanceador |
| Cómputo de la API | 1 VM fija | grupo de 1 a 3 instancias |
| Código | `3affa11` (con las correcciones de la Entrega 1) | `97120ca` (añade solo la corrección de E2) |

Por eso G3 midió además un **operando de control R**: la arquitectura de hoy con la API **fijada en 1 instancia**,
desde el mismo generador, por el mismo balanceador y con el mismo código. **R frente a E** (la elástica, de 1 a 3) es
el efecto de la elasticidad; **A7 frente a la Entrega 3** mezcla generador, red y código, y solo se compara en las
magnitudes que A7 §1.4 declara comparables.

---

## 2. Escenario 1: actividad académica concurrente

Campaña de G3 del 2026-11-01 (19:37–21:54 UTC): **32 corridas, 153 113 peticiones, 0 timeouts, 0 fallos de
validación funcional, 1 fallo real** (un 503 de 3 ms en `E-C200-r1`), y el estado en la base cuadra en las 32 (3
envíos por estudiante, ningún intento duplicado). Detalle, tablas por corrida y datos crudos en
[`evidencias/G3/`](../docs/entrega3/evidencias/G3/README.md).

### 2.1 Tabla comparativa con la Entrega 2

| Escenario y operación | Nivel de carga | Métrica y unidad | Entrega 2 (A7) | Entrega 3 | Cambio | Explicación |
| :--- | :--- | :--- | ---: | ---: | :---: | :--- |
| Recorrido completo, serie A (pausa 4–6 s) | 200 usuarios | rendimiento, pet./s | 32,5 | E 33,0 ± 0,3 · R 33,0 ± 0,7 | **igual** | La carga la fija la pausa entre pasos; la plataforma la absorbe en las dos arquitecturas |
| Recorrido completo, serie B (pausa 1–2 s) | 200 usuarios | rendimiento, pet./s | 104,6 | E 107,5 ± 3,3 · R 108,2 ± 4,8 | **igual** (+3 %, dentro de la variación) | Una instancia de API trabaja al 13–25 % de CPU: no hay nada que la elasticidad pueda mejorar a este nivel |
| Recorrido completo, serie A | 200 usuarios | latencia caliente p95, ms | 128 | E 36 ± 2 · R 40 ± 1 | **no comparable** | Cambió el generador y el camino de red (§1). R y E, que sí son comparables, no se distinguen |
| Recorrido completo, serie B | 200 usuarios | latencia caliente p95, ms | 123 | E 48 ± 14 · R 45 ± 4 | **no comparable** con A7; **igual** entre R y E | Ídem |
| Recorrido completo, serie C (pausa 0,25–0,75 s, más allá de A7) | 200 usuarios | rendimiento, pet./s | — | E 260,5 ± 10,4 · R 250,1 (1 corrida) | **igual** | Ni la API (≤ 29 % de CPU) ni el balanceador se acercan a su capacidad |
| Recorrido completo, serie C | 200 usuarios | latencia caliente p95, ms | — | E 163 ± 45 · R 99 (1 corrida) | **peor en E** | Coincide con el autoescalador sumando instancias; hipótesis en §7, no demostrada |
| Recorrido completo, serie B | 200 usuarios | CPU máx. de Cloud SQL, % | 35,2 | E 34 ± 4 · R 34 ± 3 | **igual** | |
| Recorrido completo, serie sostenida (8 × B200, API ya en 3) | 200 usuarios | CPU de Cloud SQL, % | — | 47–52 (3 instancias) frente a 29–36 (1 instancia) | **peor con 3** | La misma carga cuesta más a la base servida por tres instancias (§7) |
| Recorrido completo, todas | hasta 200 usuarios | errores reales, % | 0 | 1 de 153 113 (0,0007 %) | **igual** | El único 503 coincide con la entrada de una instancia nueva |

Fuentes: los valores de A7 son los que G3 toma como línea base (G3 §2, rendimiento y latencia caliente), que salen de A7 [`COMPARACION.md`](../docs/entrega3/evidencias/A7/COMPARACION.md); la tabla general de A7 §1.1 da 32,6 y 103,5 pet./s para las mismas corridas con otra ventana. G3 [`README.md`](../docs/entrega3/evidencias/G3/README.md) §2 y [`tablas.md`](../docs/entrega3/evidencias/G3/tablas.md).

### 2.2 Lo que dice el escenario 1

**La arquitectura elástica y la de una instancia no se distinguen hasta unas 110 pet./s**, y a unas 255 pet./s la
elástica no mejora el rendimiento y su cola de latencia es peor mientras suma instancias. **El punto de degradación
no se alcanzó**: la API no pasó del 51 % de CPU en un minuto ni PostgreSQL del 52 %, con 52 de 100 conexiones. Con
200 cuentas y 3 intentos de cuestionario por cuenta, el escenario no puede empujar mucho más. **No se presenta 255
pet./s como la capacidad máxima**: es lo máximo probado.

---

## 3. Escenario 2: carga, procesamiento y consumo multimedia

Campaña de G4 del 2026-11-02 (00:12–01:32 UTC), desde la VM del generador contra el balanceador, imagen `97120ca`,
workers de 1 a 3 con `WORKER_CONCURRENCY=2` por instancia. **Niveles 0 a 3** (1/1, 2/10, 4/25 y 8/50 subidas y
estudiantes), perfiles reales de A5 verificados con `ffprobe` (120, 600 y 1800 s, 1280×720), consumo con patrón de
**cadencia de reproducción**. **16 de 16 videos `completed`** (verificado por la API), 0 fallos, y la cola observada
hasta 0 (47 min 5 s de drenaje). **El nivel 4 no se corrió** (recorte por tiempo). Detalle en
[`evidencias/G4/`](../docs/entrega3/evidencias/G4/README.md).

### 3.1 Tabla comparativa con la Entrega 2

| Escenario y operación | Nivel de carga | Métrica y unidad | Entrega 2 (A7) | Entrega 3 | Cambio | Explicación |
| :--- | :--- | :--- | ---: | ---: | :---: | :--- |
| Procesamiento de videos | nivel 3 (8 subidas) | trabajos terminados en su ventana | 4 de 8 | **8 de 8** | **mejor** | El grupo de workers pasa de 1 a 3 y la concurrencia efectiva de 2 a 6 |
| Espera en cola | nivel 3 | antigüedad máxima en cola, s | 1 184 | **305** | **mejor** (3,9 veces menos) | Es lo que cambia el escalamiento de workers: más trabajos atendidos a la vez |
| Espera en cola | nivel 3 | pico de cola, pendientes / activos | 8 / 2 | 8 / **6** | **mejor** | Ídem |
| Procesamiento (ffmpeg), video corto | 1 trabajo por instancia · 2 a la vez | duración / duración del video | 0,78× · 1,38× | 0,74× · 1,40× | **igual** | Depende del perfil y de compartir CPU, no de la arquitectura: escalar acorta la espera, no el procesamiento de un video |
| Consumo HLS (cadencia) | niveles 0 a 3 | segmentos descargados | 0 en los niveles 2 a 4 | 696 con plazo, **0 interrupciones** | **mejor** (corregido) | A7 consumía los videos que estaban en cola; G4 procesa antes uno corto y lo consume mientras la cola está llena |
| Consumo HLS (cadencia) | niveles 0 a 3 | latencia de segmento p50 / p95, ms | — | 23–36 / 42–97 | — | Los segmentos salen del almacenamiento, no de la API ni de los workers |
| Tiempo al primer cuadro | 24 reproducciones, 6 por nivel | p50 / p95, ms | no medido (A7 no lo publica) | **378 / 414** (p99 n/d, n=24) | **medido** | Chromium + hls.js; la decodificación inicial es el 63 % (238 ms). No sube con la cola llena |
| Firma y confirmación de carga (API) | niveles 0 a 3 | promedio, ms | — | 72–81 · 56–66 | **no comparable** | Cambió el generador (§1); constante aunque la cola se multiplicó |

Con 1 a 8 trabajos por nivel, **las etapas de los trabajos no tienen p95 publicable** (A6 exige 20): G4 da promedio
y mediana. Nivel 3: espera en cola 3 min 31 s de media (mediana 2 min 38 s), procesamiento 13 min 32 s (2 min 52 s),
total hasta `available` 17 min 03 s (5 min 29 s); la media la arrastra el video de 30 min. La comparación con A7
**declara sus diferencias** (workers elásticos frente a uno fijo, generador en la región, nivel 4 sin correr) y vale
para la espera en cola y la concurrencia efectiva, **no** para la subida, que dependía del enlace doméstico (A7 §3.5).

Fuentes: G4 [`README.md`](../docs/entrega3/evidencias/G4/README.md) §2 y §4.4,
[`campana/corrida_niveles_escenario2.txt`](../docs/entrega3/evidencias/G4/campana/corrida_niveles_escenario2.txt),
[`campana/ttff.json`](../docs/entrega3/evidencias/G4/campana/ttff.json),
[`campana/drenaje_cola_verificacion.txt`](../docs/entrega3/evidencias/G4/campana/drenaje_cola_verificacion.txt) y la
[gráfica de instancias, cola y CPU](../docs/entrega3/evidencias/G4/capturas/2026-11-02_g4-campana-instancias-cola-cpu.png);
A7 [`COMPARACION.md`](../docs/entrega3/evidencias/A7/COMPARACION.md).

### 3.2 Lo que dice el escenario 2

**El cuello de botella es la CPU de los workers** (67–100 % con la cola llena; la base 14–17 % y la API en 1 instancia
toda la campaña, así que nada de este escenario se atribuye al escalamiento de la API). Escalar workers lo alivia
en lo que puede: la cola pasó de 10 a 4 trabajos en unos 6 min con 3 instancias. Lo que queda hasta el final del
drenaje es **un solo trabajo**, el video de 30 min, que tarda lo mismo con 1 o con 3 workers.

**Hallazgo:** a las 00:34 el grupo bajó de 2 a 1 con dos videos medios en curso, que terminaron ~22 min después de
empezar en vez de 9–14 min. Es consistente con que el trabajo de la instancia retirada **volvió a empezar** (el plazo
de drenaje es de 90 s y un video de 10 min no cabe), aunque **no se midió directamente**. No se perdió nada (16 de
16 `completed`), pero se retrasó.

---

## 4. Escenario 3A: ciclo de escalamiento de la API

Corrida de G5 del 2026-11-01 (23:29–23:57 UTC), carga escalonada por **tasa fija** desde la VM del generador:
10 → 40 → 70 → **130** → 10 pet/s. El umbral de la política (C2) son 80 pet/s por instancia. Ciclo **1 → 2 → 1**
en `mooc-api-group`, todo por decisión del autoescalador: el registro de auditoría solo tiene `instances.insert` e
`instances.delete` del agente del grupo administrado. Detalle en
[`evidencias/G5/`](../docs/entrega3/evidencias/G5/README.md).

| Tramo | Duración | Qué lo compone |
| :--- | ---: | :--- |
| **Reacción** | **48 s** | La carga supera 80 pet/s con una instancia (23:33:40) → el grupo crea la VM (23:34:28, `creationTimestamp`) |
| **Incorporación** | **157 s** | Decisión → primera petición atendida por la nueva por el balanceador (23:37:05, reparto 5/5); 138 s son el arranque de la imagen hasta `/api/v1/ready` |
| De extremo a extremo | 205 s | La suma de los dos |
| Reducción | ~11 min hasta la decisión | Baja la carga a las 23:43:40; el objetivo baja a 1 a las 23:54:53 y la instancia sale a las 23:57:09 |

| Etapa | Pedida (pet/s) | Lograda (pet/s) | n | p50 / p95 / p99 (ms) | Errores reales |
| :--- | ---: | ---: | ---: | :---: | ---: |
| base | 10 | 9,6 | 1 152 | 9 / 18 / 35 | 0 |
| subida | 40 | 37,9 | 2 271 | 8 / 14 / 24 | 0 |
| subida | 70 | 69,1 | 4 148 | 8 / 14 / 24 | 0 |
| **alta** | **130** | **124,6** | 74 751 | 7 / 13 / 22 | 0 |
| bajada | 10 | 10,7 | 7 712 | 9 / 18 / 28 | 0 |

**0 errores reales en 90 080 peticiones**, también durante la incorporación y el retiro. El generador no limitó.
La latencia **no cambió** al entrar la segunda instancia: una sola atendía 124 pet/s con p95 de 13 ms y la API no pasó
del 28 % de CPU. Igual que en el escenario 1, **el ciclo demuestra el mecanismo, no una mejora de rendimiento**: no
había saturación que la segunda instancia tuviera que aliviar.

**Frente a los workers (§5):** la API reacciona más lento (48 s frente a 26 s) y se incorpora más lento (157 s frente
a 99 s). La reacción depende de cómo muestrea cada señal; la incorporación de la API incluye esperar a que el
balanceador la declare sana, y la del worker no: el worker toma trabajo en cuanto arranca.

## 5. Escenario 3B: ciclo de escalamiento de workers

Corrida de G6 del 2026-11-01 (18:50:46–19:26:49 UTC): ciclo **1 → 2 → 3 → 2 → 1** en `mooc-worker-group`, todo por
decisión del autoescalador (**0 operaciones manuales** en el registro de auditoría). Detalle en
[`evidencias/G6/`](../docs/entrega3/evidencias/G6/README.md).

| Tramo | Duración | Qué lo compone |
| :--- | ---: | :--- |
| **Reacción** | **26,3 s** | La cola supera 2 trabajos por worker → el exportador publica la métrica (cada 30 s) → el autoescalador decide y crea la instancia |
| **Incorporación** | **99,4 s** | Creación de la VM → arranque → el worker nuevo toma sus dos primeros trabajos de la cola |
| De extremo a extremo | 125,7 s | La suma de los dos |
| Reducción | 3 → 2 a las 19:10:55, 2 → 1 a las 19:26:15 | `scale_in_control`: una instancia cada 900 s |

**8 de 8 trabajos** aceptados terminaron en `completed`, cada uno con un solo `master.m3u8` y 34 segmentos, **sin
pérdidas ni duplicados**. El worker nuevo empezó a consumir **unos 60 s antes** de que la comprobación de salud de
GCE lo marcara `HEALTHY`: para un worker, la incorporación real es el primer trabajo tomado, no el estado de salud.

---

## 6. Qué mejoró, qué quedó igual, qué empeoró

| | Qué | Bajo qué condiciones | Por qué |
| :--- | :--- | :--- | :--- |
| **Mejoró** | La capacidad de procesamiento multimedia crece con la demanda | Cola por encima de 2 trabajos por worker | El grupo de workers pasa de 1 a 3 en ~2 min (§5); antes la capacidad era fija |
| **Mejoró** | Ninguna réplica es imprescindible | Retiro o fallo de una instancia | Sesiones, cola y datos fuera de las réplicas (E1); retiro de una API bajo tráfico sin perder peticiones |
| **Igual** | Rendimiento y latencia del escenario 1 | Hasta ~110 pet./s | Una instancia de API está lejos de saturarse (§2) |
| **Empeoró** | Cola de latencia durante el escalamiento | ~255 pet./s mientras el grupo suma instancias | Hipótesis de instancias que entran frías (§7) |
| **Empeoró** | Carga de PostgreSQL por petición | Con 3 instancias de API | §7 |
| **Mejoró** | La capacidad de la API crece sola con la demanda | Más de 80 pet/s por instancia | El grupo pasa de 1 a 2 en 48 s y la nueva atiende a los 157 s (§4), sin errores |
| **Mejoró** | Espera en cola de los videos | Nivel 3 del escenario 2 (8 subidas) | Antigüedad máxima 305 s frente a 1 184 s y 8 de 8 terminados frente a 4 de 8, con 3 workers en vez de 1 (§3) |
| **Igual** | Duración del procesamiento de cada video | Cualquier nivel | Un trabajo ocupa un worker: 0,74× solo y 1,40× con dos, como en A7 (§3) |
| **Empeoró** | Un video largo puede terminar mucho después | Reducción del grupo con trabajos largos en curso | El autoescalador ve la cola pendiente, no el trabajo en curso; el retirado vuelve a empezar (§3.2) |

---

## 7. El escalamiento: cuándo, qué pasó mientras, qué lo limita y cuánto cuesta

**Cuándo se activó.** Workers: a los 26,3 s de superar el umbral en G6 (§5), medido hasta la creación de la
instancia; en G4, **~3 min** entre que la cola crece y hay otra instancia operativa (dos veces), medido con la serie
por minuto e incluyendo el arranque. Son dos tramos distintos, no una contradicción: G4 suma reacción e
incorporación (G6: 26 + 99 s ≈ 2 min) con resolución de un minuto. API: a los 48 s de superar el umbral, con la
nueva atendiendo a los 157 s (§4); en G3, sin carga escalonada, se vio de 3 a 8 minutos tras empezar.

**Qué pasó mientras arrancaban las instancias.** En workers, nada que se pierda: la cola retiene el trabajo y el
worker nuevo lo toma al terminar su arranque. **Al reducir sí hay un coste**: en G4 un retiro con dos videos medios
en curso los retrasó ~10 min (§3.2), porque el drenaje de 90 s no alcanza para un trabajo largo. En la API, en el ciclo instrumentado de G5, **ninguno**: 0 errores en
90 080 peticiones mientras la nueva arrancaba y mientras la retirada drenaba. En G3, el único 503 de la campaña y el p95 más alto (163 ms
frente a 99 ms) coinciden con la entrada de una instancia nueva. La explicación más probable —**no demostrada**— es
que la instancia entra con cachés y pools fríos.

**Qué dependencia limita ahora la capacidad.** En el escenario 2, **la CPU de los workers** (§3.2), y por encima de
eso la duración de cada video, que no escala. En el escenario 1 no se llegó a saturar ninguna. El candidato es **PostgreSQL**: 1 vCPU,
`max_connections = 100`, y con 3 instancias de API su CPU fue de 47–52 % para el mismo rendimiento que con 1
instancia daba 29–36 %, con más conexiones abiertas (30–51 frente a 20–27). C2 ya extrapolaba que el techo de 3
instancias cae donde cae el de la base. **Esta campaña no lo demuestra**: hace falta una carga que G3 no podía generar
con 200 cuentas.

**Qué coste adicional implicó** (F3, [`evidencias/F3/`](../docs/entrega3/evidencias/F3/README.md)): operar 24×7 cuesta
**168,69 USD/mes con los dos grupos a 1 instancia** frente a 133,73 de la Entrega 2. De los 34,96 USD/mes de más, 18,25
son las reglas de reenvío del balanceador y 13,23 la VM de la cola: **lo caro es tener la entrada y la cola, no
escalar**. Cada instancia extra cuesta 0,055 USD/h; un ciclo como el de G6 (dos workers extra durante ~12 y ~29 min)
cuesta unos 4 centavos.

---

## 8. Defectos de la Entrega 2 corregidos

La retroalimentación de la Entrega 2 (criterio *Análisis de capacidad — Escenario 2*, 6,67/10) señaló los cuatro
primeros defectos; el quinto lo añade el propio issue. Esta tabla dice cómo se corrigió cada uno y dónde se verifica.

| Defecto de la Entrega 2 | Corrección | Dónde se verifica | Estado |
| :--- | :--- | :--- | :---: |
| El escenario 2 se midió contra Docker Compose (`http://api:8080`), no contra la nube | Toda corrida registra su destino en `destino.json` y el resumen **se niega a publicar** si el destino es local (`EXIGIR_NUBE=1`) | A6; `destino.json` de cada corrida de G3 y G5; en G4, la cabecera de [`corrida_niveles_escenario2.txt`](../docs/entrega3/evidencias/G4/campana/corrida_niveles_escenario2.txt) (`https://136.68.55.254.sslip.io [remoto]`) | ✅ |
| Perfiles de 2, 10 y 30 minutos que eran clips de 8, 20 y 40 segundos | Perfiles regenerados y **verificados campo por campo con `ffprobe`**; el guion falla si el archivo no es lo que declara | [A5](../docs/entrega3/evidencias/A5/README.md) | ✅ |
| Tiempo al primer cuadro calculado como latencia HTTP más 45 ms fijos | Medido con reproductor real: Chromium + hls.js, de asignar el manifiesto al primer `playing` | [G4](../docs/entrega3/evidencias/G4/README.md) §2 (`ttff.json`, 24 mediciones) | ✅ |
| p95 por debajo del p50 | Un solo estimador y comprobación de monotonía **dentro** del script que publica | [A6](./criterios_de_medicion.md) | ✅ |
| Etapas mezcladas y drenaje no observado | Etapas separadas (G4); drenaje de la cola observado y fechado | [G4](../docs/entrega3/evidencias/G4/README.md) §2 (etapas 1 a 6 y consumo por separado; drenaje 8 → 0 en 47 min); [G6](../docs/entrega3/evidencias/G6/README.md) (`COLA_DRENADA_CERO` a las 19:02:01) | ✅ |

---

## 9. Limitaciones

- **No se alcanzó la saturación** en el escenario 1: 255 pet./s es lo máximo probado, no la capacidad máxima, y no
  se extrapola a escalas no evaluadas.
- **R tiene una sola corrida publicable en la serie C**: su variación no se puede informar.
- La causa de la mayor carga de PostgreSQL con 3 instancias **no se midió**; quedan tres hipótesis (G3 §3.4).
- Las métricas de infraestructura por corrida son una o dos muestras de Cloud Monitoring (una por minuto); son
  indicativas. La serie continua está en `G3/campana_completa/`.
- La línea base de A7 salió de un portátil: sus latencias no son comparables con las de la Entrega 3 (§1).
- El ciclo de la API (G5) se corrió **una vez** y se acortó por tiempo (alta de 10 min y no de 15); llegó a 2
  instancias, no a 3. Los tiempos de reacción e incorporación son una observación, no una distribución.
- El escenario 2 se corrió **hasta el nivel 3**, no el 4 (12 subidas y 100 estudiantes), y **sin operando de control
  con workers fijos**: su mejora se sostiene contra A7, con las diferencias declaradas en §3.1.
- Las etapas de los trabajos del escenario 2 tienen 1 a 8 observaciones por nivel: se dan promedio y mediana, sin p95.
  Los dos videos medios del nivel 2 terminaron fuera de su ventana (n/d en sus etapas 4–6).
- La causa del retraso de esos dos videos (retiro del worker con trabajo en curso) es la explicación consistente con
  la serie, **no una medición**: los registros de los workers retirados no llegaron a Cloud Logging.
- El tiempo al primer cuadro tiene 24 mediciones: p99 n/d.

---

## 10. Revisión final de tablas

Hecha el 2026-11-02 sobre §2–§5:

- [x] Unidades en todas las cabeceras.
- [x] Ventana de agregación y número de observaciones de cada percentil.
- [x] `p50 ≤ p95 ≤ p99` en todas las filas (`python scripts/capacity_criterios.py`).
- [x] Ningún percentil promediado entre corridas.
- [x] Ninguna latencia HTTP mezclada con duración de trabajos asíncronos.
- [x] Enlaces a scripts, datos, resultados originales y gráficas de cada escenario.

---

Este archivo no contiene credenciales, llaves ni secretos.
