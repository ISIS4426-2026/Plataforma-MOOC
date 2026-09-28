# Sección del Escenario 1 para el informe consolidado (I3), con los números reales de H3

Texto para reemplazar las secciones **1.4 a 1.9** de `capacity-planning/pruebas_de_carga_entrega2.md`. Todas las cifras salen de
las corridas guardadas en [`resultados/`](./resultados/) y de [`analisis_h3.txt`](./analisis_h3.txt). Detalle y salvedades en el
[`README`](./README.md) de esta carpeta. Configuración real: Web Server **`e2-highcpu-2`** (no `e2-small`).

---

### 1.4 Resultados numéricos por corrida y variación

| Métrica | Línea base (1) | 10 | 25 | 50 | 100 | 200 (corrida 1) | 200 (rep. 1) ‡ | 200 (rep. 2) |
| :--- | :-: | :-: | :-: | :-: | :-: | :-: | :-: | :-: |
| Peticiones medidas | 33 | 352 | 882 | 1 770 | 3 234 | 6 473 | 6 529 | 6 482 |
| Rendimiento (pet./s) | 0,2 | 1,8 | 4,5 | 8,9 | 16,5 | 32,8 | 32,0 | 32,3 |
| p50 (ms) | 113 | 92 | 96 | 93 | 93 | 94 | 96 | 94 |
| p95 (ms) | 1 576 * | 328 | 375 | 338 | 372 | 380 | 789 | 377 |
| p99 (ms) | 1 711 * | 1 298 | 1 378 | 1 289 | 1 373 | 1 365 | 1 901 | 1 364 |
| Errores reales (5xx / timeouts) | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| Fallos de validación funcional | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |

\* La línea base tiene solo ≈ 30 muestras y una conexión nueva lenta; no es un resultado representativo.  
‡ Corrida con el generador perturbado (pico de CPU del contenedor de JMeter de 688 % frente a 193–233 % en las otras).

**Variación en 200 usuarios (3 corridas):** rendimiento 32,4 ± 0,4 pet./s (CV 1,1 %); p50 94,7 ± 0,9 ms; p95 515 ± 194 ms
(377–789); p99 1 543 ± 253 ms. El rendimiento y la carga de la base son estables; la variación está en la cola de la distribución.

### 1.5 Métricas de infraestructura

| Recurso | 10 usuarios | 50 | 100 | 200 (3 corridas) | Diagnóstico |
| :--- | :-: | :-: | :-: | :-: | :--- |
| CPU `mooc-web-server` (media / máx., %) | 5,8 / 8,1 | 7,2 / 10,4 | 8,8 / 13,3 | 11,5–19,4 / 15,0–33,9 | Holgada |
| Memoria `mooc-web-server` (%) | 30,7 | 31,0 | 30,4 | 30,9–31,8 | Estable |
| CPU Cloud SQL (media / máx., %) | 10,6 / 11,0 | 12,8 / 15,6 | 14,3 / 18,1 | 17,9–20,3 / 26,0–27,1 | Holgada |
| Conexiones a la base (máx.) | 4 | 6 | 7 | 15 · 27 · 17 | Tope del pool de la API ≈ 29 (25 + 4): se rozó una vez |
| Transacciones de PostgreSQL por segundo (máx.) | 19 | 93 | 174 | 342–356 | ≈ 10 por petición |
| CPU `mooc-worker-server` (máx., %) | 8,5 | 3,0 | 6,4 | 6,9 | Solo aloja Redis de sesiones |

### 1.6 Punto de degradación y cuello de botella

**No se alcanzó saturación.** Hasta 200 usuarios concurrentes (≈ 32 peticiones/s) ningún nivel activó los criterios del plan
(errores reales > 1 %, p95 > 1 s sostenido, aplanamiento del rendimiento, fallos de validación). El rendimiento creció de forma lineal
con la carga. **El máximo probado es 200 usuarios / 32,8 peticiones/s y no debe leerse como la capacidad máxima.** No se ejecutó la serie de
presión que buscaría el punto real de degradación.

Recurso con menos margen entre los medidos: la CPU de Cloud SQL (media 20 %, máx. 27 %); con una extrapolación **lineal** (estimación,
no medición) llegaría a 80 % hacia las 240 peticiones/s. El pool de conexiones de la API (`DB_MAX_OPEN_CONNS = 25`) rozó su tope (27 conexiones
abiertas de ≈ 29) en un pico. La CPU del Web Server no es el límite en el rango probado (12–34 %).

**Origen de la cola de latencia:** sobre conexiones ya abiertas, el p95 del servidor es ≈ 117 ms; las peticiones que abren una conexión nueva
(≈ 9,5 % del total) tienen un tiempo de conexión p95 de ≈ 1 285 ms, y el 100 % de las peticiones de 1,2–1,7 s son de ese tipo.

### 1.7 Integridad y validación funcional

Tras cada corrida se consultó Cloud SQL directamente (`scripts/seeds/capacity_verificar_estado.sql`): con N usuarios, exactamente N estudiantes
con exactamente 3 envíos, N inscripciones y **0 envíos duplicados del mismo intento**, incluso con los reenvíos con la misma `Idempotency-Key`
(paso 11). Las 8 corridas dan `OK`. Control negativo: [`H2/resultados/local-sin-reinicio_*`](../H2/resultados/local-sin-reinicio_20260927_202240/resumen.txt).

### 1.8 Limitaciones

Un solo generador con una sola IP (login fuera del recorrido, ráfaga aparte: 10 respuestas 200 y 10 respuestas 429 con 20 cuentas); memoria limitada
del portátil generador; una de las tres corridas de 200 usuarios con el generador perturbado; **serie de presión y métricas de la API no disponibles**.

### 1.9 Propuesta de evolución con respaldo

Primero confirmar el origen de la espera de ≈ 1 s al conectar (contadores de descartes de SYN en la VM web antes y después de una corrida) y
habilitar `ssl_session_cache` y HTTP/2 en nginx; después medir el pool antes de subir `DB_MAX_OPEN_CONNS`. **No** hay evidencia que respalde
cambiar el tipo de VM, agregar réplicas de lectura ni un balanceador con la CPU del web en 12–34 % y la de la base en 27 %.
