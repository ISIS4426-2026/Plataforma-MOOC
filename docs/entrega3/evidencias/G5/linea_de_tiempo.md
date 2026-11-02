# G5 — Línea de tiempo del escenario 3A (generada)

Generada por `scripts/g5_linea_de_tiempo.py` el 2026-11-01 23:59 UTC. No editar a mano: se regenera con los datos crudos de esta carpeta.

- Destino: `https://136.68.55.254.sslip.io` · entorno `nube-entrega3` · commit `ac3f7fa`
- Generador: justb4/jmeter@sha256:088ac52b759a198a5afa5ae13d0a6306e9f2017d71ad140ff57427f6930406f7 en mooc-load-generator (4 núcleos)
- Umbral de la política (C2): **80 pet/s por instancia** (target 0,8 × max_rate 100)
- Resolución del observador: un sondeo cada ~20 s; los hitos observados tienen ese margen

## Los dos tiempos

| Tiempo | Desde | Hasta | Duración |
| :--- | :--- | :--- | ---: |
| **Reacción** | la carga supera 80 pet/s (23:33:40) | decisión de ampliar (23:34:28) | **48 s** |
| **Incorporación** | decisión (23:34:28) | primera petición atendida por la nueva (23:37:05) | **157 s (2 min 37 s)** |

La incorporación, desglosada:

| Tramo | Duración | Qué lo explica |
| :--- | ---: | :--- |
| decisión → instancia creada | 0 s | el grupo pide la VM |
| creada → `RUNNING` | 19 s | arranque de la VM (resolución del sondeo) |
| `RUNNING` → sana ante el balanceador | 138 s (2 min 18 s) | arranque de la imagen y `/api/v1/ready` (B3: 62 s medidos) |
| sana → atiende | 0 s | el balanceador la incorpora al reparto |

## Línea de tiempo

| Hora UTC | Fuente | Qué |
| :--- | :--- | :--- |
| 23:29:25 | observador | PARTIDA: 1 instancia operativa (mooc-api-m2b7), autoescalador ACTIVE, minimo 1 |
| 23:29:40 | carga | empieza e1 (base): 10 pet/s pedidas |
| 23:31:40 | carga | empieza e2 (subida-40): 40 pet/s pedidas |
| 23:32:40 | carga | empieza e3 (subida-70): 70 pet/s pedidas |
| 23:33:40 | carga | empieza e4 (alta-sostenida): 130 pet/s pedidas |
| 23:33:40 | carga | primera cubeta de 10 s por encima de 80 pet/s |
| 23:34:27 | auditoría | `v1.compute.instances.insert` sobre mooc-api-b18m por agente de servicio de Google APIs (grupo administrado / autoescalador) |
| 23:34:28 | GCE | creationTimestamp de mooc-api-b18m (id 3476718266121613931) |
| 23:34:47 | observador | DECISION_AMPLIAR: el autoescalador recomienda 2 y el objetivo es 2 (partida: 1) |
| 23:34:48 | observador | INSTANCIA_RUNNING: mooc-api-b18m (id 3476718266121613931) arrancada; creada 2026-11-01T15:34:28.221-08:00 |
| 23:34:48 | observador | INSTANCIA_CREADA: mooc-api-b18m (id 3476718266121613931, X-Instance-ID 3fc0f66b96c6), creationTimestamp 2026-11-01T15:34:28.221-08:00 |
| 23:37:05 | observador | INSTANCIA_SANA: mooc-api-b18m HEALTHY ante el balanceador y sin accion en curso |
| 23:37:05 | observador | INSTANCIA_ATIENDE: mooc-api-b18m respondio peticiones del balanceador (3fc0f66b96c6:5 8f18e6086033:5) |
| 23:43:40 | carga | empieza e5 (bajada): 10 pet/s pedidas |
| 23:54:53 | observador | DECISION_REDUCIR: el objetivo baja de 2 a 1 |
| 23:56:36 | auditoría | `v1.compute.instances.delete` sobre mooc-api-b18m por agente de servicio de Google APIs (grupo administrado / autoescalador) |
| 23:57:09 | observador | INSTANCIA_RETIRADA: mooc-api-b18m (id 3476718266121613931) ya no esta en el grupo |
| 23:57:29 | observador | CICLO_COMPLETO: subio a 2 y volvio a 1 (que nadie lo toco lo confirma la auditoria en g5_linea_de_tiempo.py) |
| 23:58:52 | auditoría | `v1.compute.instances.delete` sobre mooc-load-generator por persona |

## Carga por etapa (JMeter, desde la VM del generador)

| Etapa | Pedida (pet/s) | Lograda (pet/s) | n | p50 ms | p95 ms | p99 ms | Errores reales | 4xx |
| :--- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| e1 base | 10 | 9,6 | 1152 | 9 | 18 | 35 | 0 | 0 |
| e2 subida-40 | 40 | 37,9 | 2271 | 8 | 14 | 24 | 0 | 0 |
| e3 subida-70 | 70 | 69,1 | 4148 | 8 | 14 | 24 | 0 | 0 |
| e4 alta-sostenida | 130 | 124,6 | 74751 | 7 | 13 | 22 | 0 | 0 |
| e5 bajada | 10 | 10,7 | 7712 | 9 | 18 | 28 | 0 | 0 |

Latencias de las peticiones exitosas, por rango más cercano (`capacity_criterios.py`, A6). Son latencias HTTP de lecturas; no se mezclan con trabajos asíncronos.

## Integridad durante el ciclo

- Errores reales en todo el ciclo: **0 de 90080** (0.000 %).
- Durante la incorporación (decisión → atiende + 2 min): 0 de 34597.
- Durante el retiro (decisión de reducir − 1 min → retirada + 1 min): 0 de 1116. El backend drena 60 s (`connection_draining_timeout_sec`, C3).
- La integridad de **escrituras** entre réplicas (envíos, inscripciones, progreso) la cubre la corrida del arnés de consistencia lanzada con el grupo ya en 2 instancias: ver el README de G5.

## Quién cambió el grupo (registro de auditoría)

| Hora UTC | Método | Recurso | Quién |
| :--- | :--- | :--- | :--- |
| 23:34:27 | `v1.compute.instances.insert` | mooc-api-b18m | agente de servicio de Google APIs (grupo administrado / autoescalador) |
| 23:56:36 | `v1.compute.instances.delete` | mooc-api-b18m | agente de servicio de Google APIs (grupo administrado / autoescalador) |
| 23:58:52 | `v1.compute.instances.delete` | mooc-load-generator | persona |

Los correos de personas no se publican: aparecen como «persona».

## Comprobaciones

- ✅ la etapa alta alcanzo la tasa pedida: 124.6 de 130 pet/s (minimo 90 %); si no, el generador o sus hilos limitaron la corrida
- ✅ la carga supero el umbral de 80 pet/s con una instancia
- ✅ la decision de ampliar es POSTERIOR al aumento de carga (si fuera anterior, la causa no es esta carga)
- ✅ se creo al menos una instancia nueva
- ✅ la instancia nueva atendio peticiones entrando por el balanceador
- ✅ la reduccion empezo despues de bajar la carga
- ✅ el grupo volvio a 1 instancia
- ✅ errores reales (5xx, timeouts, conexion) en todo el ciclo: 0 de 90080 = 0.000 % (maximo 1.0 %)
- ✅ percentiles monotonos p50 <= p95 <= p99 en todas las etapas
- ✅ se pudo leer el registro de auditoria de la ventana
- ✅ ninguna PERSONA cambio el tamano ni la politica del grupo durante la ventana

Este archivo no contiene credenciales, llaves ni secretos.
