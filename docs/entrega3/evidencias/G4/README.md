# Evidencia G4 — Escenario 2 sobre la arquitectura elástica: carga, procesamiento y consumo multimedia ([#224](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/224))

| | |
| :--- | :--- |
| **Bloque** | G — Pruebas de carga comparativas |
| **Responsable** | `@tmichelldiaz` |
| **Depende de** | A5, A7, E3, G1 (cerrados) |
| **Desbloquea** | G7 |
| **Plan** | [`capacity-planning/entrega3_g3_g4.md`](../../../../capacity-planning/entrega3_g3_g4.md) §4 |
| **Fecha** | 2026-11-02, 00:12–01:32 UTC, desde la VM `mooc-load-generator` (`us-east1-c`), contra `https://136.68.55.254.sslip.io` (balanceador), imagen `97120ca`, workers de 1 a 3 |

---

## 0. Resultado en una página

**Niveles 0 a 3 del escenario 2, 16 videos subidos y procesados (15 de los niveles y el de consumo), 0 fallos, y el drenaje de la cola observado hasta 0 (47 min).** Se verificó por la API que los 16 recursos de la campaña quedaron `completed`. El generador corrió desde una VM de la misma región y llevó toda la corrida con el sondeo de la cola y de la base desde dentro de la VPC.

| Criterio de aceptación | |
| :--- | :---: |
| 1. Mediciones separadas por etapa, sin colapsarlas | ✅ §2 (etapas 1 a 6 y consumo HLS por separado) |
| 2. La corrida apunta a la nube y la URL aparece en la evidencia | ✅ `destino` en la cabecera de [`campana/corrida_niveles_escenario2.txt`](./campana/corrida_niveles_escenario2.txt): `https://136.68.55.254.sslip.io [remoto]` |
| 3. Perfiles declarados = archivos usados, con `ffprobe` | ✅ [`perfiles/`](./perfiles/): 120, 600 y 1800 s, 1280×720, H.264 + AAC |
| 4. Tiempo al primer cuadro medido, no derivado; p50 ≤ p95 ≤ p99 | ✅ **24 mediciones con Chromium + hls.js**, 0 fallidas; p50 378 ms ≤ p95 414 ms (p99: n/d con 24, A6 exige 100) |
| 5. Patrón de consumo declarado | ✅ **cadencia de reproducción** (segmentos de 7,20 s, buffer inicial de 2, `-max-segmentos 10`); la descarga a máxima velocidad se mide aparte («Descarga Greedy») |
| 6. Instancias activas y concurrencia efectiva registradas durante toda la corrida | ✅ [`campana/serie_temporal.csv`](./campana/serie_temporal.csv), §3 |
| 7. Drenaje de la cola observado y documentado | ✅ [`campana/drenaje_cola_verificacion.txt`](./campana/drenaje_cola_verificacion.txt): pending 8 → 0 y active 6 → 0 en 47 min 5 s |
| 8. Cuello de botella identificado con datos de la corrida | ✅ §4: la CPU de los workers y, al final, un solo trabajo largo |
| 9. Nada del autoscaling de la API se atribuye sin evidencia | ✅ §4.3: la API **no escaló** (1 instancia en toda la campaña) |
| 10. Gráficas por etapa, drenaje e instancias capturadas durante la corrida | ✅ [`capturas/`](./capturas/) (generadas de las métricas de la ventana, el mismo día) |
| 11. Hallazgos en `NOTAS_TECNICAS.md` | ✅ nota 65 |

---

## 1. Qué se midió, y qué se cambió respecto a A7

**Mantenido (igual que A7):** los tres perfiles reales de A5 (corto 120 s, medio 600 s, largo 1800 s; la escalera HLS de 360p y 720p), `WORKER_CONCURRENCY=2` **por instancia**, los niveles 0 a 3 con sus mismas subidas y estudiantes (1/1, 2/10, 4/25 y 8/50), patrón de consumo `cadencia`.

**Corregido respecto a la campaña de A7:**

| Defecto de A7 | Qué se hizo |
| :--- | :--- |
| El consumo HLS tomaba los videos que estaban en la cola: en los niveles 2 a 4 no descargó **ni un segmento** | un video del perfil corto se procesa **antes** de medir nada; los estudiantes de cada nivel lo consumen **mientras** las subidas del nivel alimentan la cola. Esta vez hay observaciones en los cuatro niveles: 20, 100, 250 y 500 latencias de segmento |
| No había tiempo al primer cuadro | Chromium 156 sin ventana + hls.js, en contenedor ([`ttff/`](../../../../scripts/capacity_escenario2/ttff/)): seis reproducciones por nivel desde una sesión nueva |
| Un trabajo que esperaba su turno se daba por «atascado» a los 12 min | el atasco es del **sistema** (ningún trabajo cambia de estado en 45 min); un trabajo detrás de otros no está colgado |
| Marcas de la campaña en una VM de la API | desde la VM generadora, dentro de la VPC |

**Diferencias declaradas con A7:** workers de 1 a 3 (concurrencia total efectiva de 2 a 6), generador en la región (la subida a Cloud Storage y la bajada de segmentos ya no miden un enlace doméstico), código `97120ca` en vez de `3affa11`, y **se corrieron los niveles 0 a 3, no el 4** (el 4 sube 12 videos y 100 estudiantes; se recortó a pedido del equipo para terminar antes). **El drenaje se observa al final del nivel 3.**

---

## 2. Resultados por etapa

Con 1 a 8 observaciones por nivel, el p95 no se publica (A6: un p95 necesita 20; un p99, 100); se informan **promedio y mediana** y se deja `n/d`. Detalle completo: [`campana/corrida_niveles_escenario2.txt`](./campana/corrida_niveles_escenario2.txt).

| Nivel | Subidas | Perfiles | Completados | Pico de cola (pend. / activas) | Antigüedad máx. en cola | Instancias de worker |
| :---: | ---: | :--- | :---: | :---: | ---: | :---: |
| 0 | 1 | corto | 1/1 | 1 / 1 | 0,3 s | 1 |
| 1 | 2 | corto ×2 | 2/2 | 0 / 2 | 0 s | 1 |
| 2 | 4 | corto ×2, medio ×2 | **2/4 en su ventana** (los 2 medios terminaron en el drenaje) | 4 / 4 | 152 s | 1 → 2 → 1 |
| 3 | 8 | corto ×4, medio ×3, largo ×1 | **8/8** | 8 / 6 | 305 s | 1 → 3 → 2 → 1 |

**Etapas del nivel 3 (8 trabajos):**

| Etapa | Promedio | Mediana |
| :--- | ---: | ---: |
| 1. Firma de la URL (API) | 81 ms | 66 ms |
| 2. Transferencia directa al almacenamiento | 277 ms (38 MB/s) | 198 ms |
| 3. Confirmación de carga (API) | 56 ms | 55 ms |
| 4. Espera en cola | **3 min 31 s** | **2 min 38 s** |
| 5. Procesamiento (ffmpeg) | 13 min 32 s | 2 min 52 s |
| 6. Total hasta `available` | 17 min 03 s | 5 min 29 s |

**Cómo se lee:**
- **El control de la API es constante** a lo largo de los cuatro niveles: firma 72–81 ms y confirmación 56–66 ms de media, aunque la cola se multiplicó. Los archivos (etapa 2) no pasan por la API.
- **El procesamiento depende del perfil y de la contención, no de la arquitectura:** un video corto solo tarda **1 min 29 s (0,74×)**; dos a la vez en la misma instancia, **2 min 48 s (1,40×)** cada uno. Es el mismo comportamiento que midió A7 (0,78× solo, 1,38–1,42× con dos).
- **El consumo HLS** (cadencia, segmentos de 7,20 s): latencia de segmento p50 23–36 ms y p95 42–97 ms, **0 interrupciones en 696 segmentos con plazo** y margen de buffer en el peor caso de 14,2 s, **en los cuatro niveles y mientras la cola estaba llena**.

### Tiempo al primer cuadro (criterio 4)

Medido por Chromium + hls.js: desde asignar el `master.m3u8` hasta el primer evento `playing`; la decodificación inicial es `playing` menos la llegada del segmento 0. **24 mediciones, 6 por nivel, 0 fallidas** ([`campana/ttff.json`](./campana/ttff.json)):

| | p50 | p95 | mín. – máx. |
| :--- | ---: | ---: | :---: |
| **Primer cuadro** | **378 ms** | **414 ms** | 333 – 431 ms |
| Manifiesto | 42 ms | — | |
| Lista de variante | 29 ms | — | |
| Segmento 0 | 42 ms | 54 ms | |
| **Decodificación inicial** | **238 ms** | 280 ms | |

**La decodificación es el 63 % del tiempo al primer cuadro:** el transporte (manifiesto + variante + segmento) son unos 113 ms y se queda igual al cambiar de nivel. La mediana por nivel (366, 368, 362 y 379 ms) **no sube con la cola llena**, como era de esperar: los segmentos salen del almacenamiento, no de la API ni de los workers. p99: n/d (24 muestras).

---

## 3. Instancias activas y concurrencia efectiva

La serie por minuto de toda la campaña ([`campana/serie_temporal.csv`](./campana/serie_temporal.csv)) y su [gráfica](./capturas/2026-11-02_g4-campana-instancias-cola-cpu.png):

| Hora (UTC) | Workers existentes | Concurrencia efectiva (×2) | Qué pasaba |
| :---: | :---: | :---: | :--- |
| 00:12 – 00:21 | 1 | 2 | niveles 0 y 1 |
| **00:21** | **2** | **4** | la cola subió a 4 a las 00:18 (nivel 2): el autoescalador tardó **~3 min** en pedir otro worker |
| **00:34 – 00:35** | **1** | **2** | **se retiró un worker con 2 trabajos activos** |
| 00:44 | 1 | 2 | nivel 3: 8 subidas confirmadas en 1 s |
| **00:47** | **3** | **6** | otra vez ~**3 min** de reacción; pico: pending 8, active 6 |
| 01:02 – 01:03 | 3 → 2 | 6 → 4 | la cola bajó a 4 |
| 01:18 | 1 | 2 | queda un solo trabajo (el video largo) |
| 01:32 | 1 | — | cola en 0 |

La API tuvo **1 instancia durante toda la campaña.**

---

## 4. Análisis

### 4.1 Cuello de botella: la CPU de los workers, y al final un solo trabajo

- Con la cola llena, la CPU del grupo de workers está entre **67 % y 100 %** (la gráfica, panel 2); la API, el balanceador, la base (14–17 %) y la VM de la cola no pasan de unos puntos.
- El tiempo hasta `available` lo domina la **espera en cola** (3 min 31 s de media en el nivel 3, frente a 2 s en los niveles 0 a 2) y el **procesamiento** de los videos largos.
- Una vez que el grupo llega a 3 instancias, la cola se vacía en pocos minutos (de 10 a 4 trabajos en unos 6 min). **Lo que queda hasta las 01:32 es un único trabajo**: el video de 30 minutos (1800 s), que tarda ~30–40 min sin importar cuántos workers haya, porque cada trabajo ocupa un worker. Escalar workers acorta la espera, **no acorta el procesamiento de un video**.

### 4.2 El autoescalador de workers: tarda ~3 min en reaccionar, y retiró un worker con trabajo activo

- **Reacción:** ~3 min entre que la cola crece y hay otra instancia operativa, en las dos ocasiones (00:18 → 00:21 y 00:44 → 00:47).
- **Retiro con trabajo activo (hallazgo):** a las 00:34, con la cola en 2 trabajos activos y 0 pendientes, el grupo bajó de 2 a 1. Los dos videos **medios** (600 s) del nivel 2, que debían tardar ~9–14 min, **seguían activos a las 00:44**, ~22 min después de empezar, y no terminaron dentro de la ventana de 26 min del nivel: por eso el nivel 2 muestra «2/4» y sus etapas 4–6 con n/d. Coincide con que se retiró una instancia con un trabajo en curso y que ese trabajo volvió a empezar (el plazo de drenaje es de 90 s, `WORKER_DRAIN_TIMEOUT`; un video de 10 min no lo cumple). **No se midió directamente** (los registros de los contenedores de los workers casi no llegan a Cloud Logging en las instancias nuevas, y las instancias retiradas desaparecen con ellos): es la explicación consistente con la serie, no una medición. Lo cierto es que **dos trabajos terminaron mucho después de lo calculado y el sistema los terminó todos** (16 de 16 `completed`).
- Para **G6**: la señal `outstanding / 2` por worker ve el trabajo **pendiente**, no el que lleva 20 minutos procesándose; retirar una instancia con un trabajo largo en curso lo obliga a empezar de cero.

### 4.3 El autoscaling de la API no se atribuye a nada

La API se mantuvo en **1 instancia**. Las etapas 1 y 3 (control) fueron 72–81 ms y 56–66 ms en todos los niveles, y las descargas de segmentos, que no pasan por la API, no cambiaron con la cola. No hay mejora de descarga ni reproducción que atribuir a que la API escale.

### 4.4 Contra A7 (primera arquitectura, 1 worker fijo de 2 vCPU)

| | A7, nivel 3 | G4, nivel 3 |
| :--- | :--- | :--- |
| Trabajos terminados en su ventana | 4 de 8 (los otros 4 «sin terminar de medir») | **8 de 8** |
| Pico de cola (pending / active) | 8 / 2 | 8 / **6** |
| Antigüedad máxima en cola | **1 184 s** | **305 s** (3,9 veces menos) |
| Consumo HLS | no descargó ni un segmento | **500 latencias de segmento, 0 interrupciones** |
| Tiempo al primer cuadro | no medido | **378 ms** (mediana, 24 mediciones) |

**Esta comparación declara sus diferencias** (§1): workers elásticos frente a uno fijo, generador en la región frente a un portátil, y el nivel 4 sin correr. Es válida para la **espera en cola** y la **concurrencia efectiva**, que son lo que el escalamiento de workers cambia; **no** para las etapas 1 y 2, que dependen del enlace del generador (A7 §3.5).

---

## 5. Lo que no quedó medido

- **El nivel 4** (12 subidas, 100 estudiantes) y por tanto el punto de estrés máximo de A7. La campaña lo admite (`-hasta-nivel 4`).
- **Un operando de control con workers fijos** (el equivalente de R en G3): no se corrió, así que la mejora de la elástica se sustenta contra la línea base de A7, con sus diferencias, no contra el mismo entorno con 1 worker.
- **Etapas 4–6 de los 2 trabajos medios del nivel 2** (n/d): terminaron después de la ventana del nivel, en el drenaje.
- **p95 de las etapas por nivel**: con 1 a 8 observaciones por nivel no es publicable (A6). Para publicarlo hacen falta ≥ 20 trabajos de un mismo perfil.
- **La causa exacta del trabajo que tardó de más en el nivel 2** (§4.2): sin registros de los workers.
- La consulta de verificación terminal que hace el programa (`SELECT … FROM resources WHERE object_key LIKE 'originals/%'`) cuenta **todos** los recursos del proyecto (57 `completed`, 3 `failed` de pruebas anteriores, p. ej. E1) y no es la verificación de esta corrida; la de esta corrida es la de la API: **16 de 16 `completed`** (§0).

---

## 6. Para G6, G7 e I4

1. **G6:** el retiro de un worker con un video largo en curso reinicia el trabajo (§4.2). Un `scale_in_control` más lento o un drenaje más largo lo evitan.
2. **G7:** el efecto del escalamiento de workers es sobre la **espera en cola** (305 s frente a 1 184 s de antigüedad máxima), no sobre el procesamiento de cada video.
3. **G7 / I4:** primer cuadro ≈ 380 ms, con la decodificación como 63 % del total; es independiente de la cola.

---

## 7. Coste y estado del entorno

VM generadora: encendida a las 00:00 UTC y apagada tras bajar los resultados (~2 h × 0,13 USD/h ≈ **0,3 USD**). Workers de 1 a 3 durante ~1 h 15 min. El importe real se consulta en la consola de facturación. Al cerrar: API 1, workers 1, VM generadora apagada.

---

## 8. Archivos

| Qué | Dónde |
| :--- | :--- |
| Log de la corrida, por etapa y por nivel | [`campana/corrida_niveles_escenario2.txt`](./campana/corrida_niveles_escenario2.txt) |
| Drenaje de la cola, cada 0,8 s | [`campana/drenaje_cola_verificacion.txt`](./campana/drenaje_cola_verificacion.txt) |
| Tiempo al primer cuadro, las 24 mediciones | [`campana/ttff.json`](./campana/ttff.json) |
| Métricas de Cloud Monitoring de la ventana | [`campana/metricas_gcp.json`](./campana/metricas_gcp.json) · [`campana/serie_temporal.csv`](./campana/serie_temporal.csv) |
| Gráfica | [`capturas/2026-11-02_g4-campana-instancias-cola-cpu.png`](./capturas/2026-11-02_g4-campana-instancias-cola-cpu.png) |
| Perfiles y su `ffprobe` | [`perfiles/`](./perfiles/) |
| Registro completo de la campaña | [`registros/campana.log`](./registros/campana.log) |
| Código | `scripts/capacity_escenario2/`, `scripts/g4_campana.sh` |

Este archivo no contiene credenciales, llaves ni secretos.
