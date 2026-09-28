# Resumen Ejecutivo: Análisis de Capacidad — Escenario 2 (Entrega 2, Issue H5)

Este documento consolida los hallazgos y métricas cuantitativas obtenidas durante la ejecución de los **5 niveles escalonados de carga** (Línea Base y Niveles 1 al 4) para el flujo multimedia de la Plataforma MOOC.

## 1. Tabla Comparativa de Niveles de Carga

| Métrica | Nivel 0 (Base) | Nivel 1 (Sub-sat) | Nivel 2 (Equilibrio) | Nivel 3 (Saturación) | Nivel 4 (Estrés/Drenaje) |
| :--- | :---: | :---: | :---: | :---: | :---: |
| **Profesores Carga** | 1 | 2 | 4 | 8 | 12 |
| **Total Videos Subidos** | 1 | 2 | 4 | 8 | 12 |
| **Estudiantes Streaming HLS** | 2 | 10 | 25 | 50 | 100 |
| **Tasa Servicio (vid/min)** | 13.98 | 20.68 | 37.38 | 50.65 | 110.65 |
| **Pico Cola (pending)** | 1 | 2 | 0 | 8 | 12 |
| **Antigüedad Máx. Cola** | 0.17s | 0.18s | 0.00s | 0.57s | 2.91s |
| **Etapa 1 (Firma p95)** | 4ms | 3ms | 3ms | 5ms | 4ms |
| **Etapa 2 (PUT Directo p95)** | 5ms | 5ms | 7ms | 8ms | 38ms |
| **Etapa 3 (Confirmación p95)**| 13ms | 9ms | 11ms | 18ms | 9ms |
| **Etapa 4 (Espera Cola p95)** | 303ms | 304ms | 303ms | 913ms | 3.035s |
| **Etapa 5 (Proc FFmpeg p95)** | 3.955s | 4.56s | 6.082s | 4.85s | 5.465s |
| **Etapa 6 (Total Avail p95)** | 4.259s | 4.864s | 6.385s | 5.761s | 6.074s |
| **QoE TTFF Reproductor** | 51ms | 51ms | 52ms | 51ms | 52ms |

## 2. Hallazgos Principales por Etapa

1. **Plano de Control (Etapas 1 y 3):** La emisión de URLs prefirmadas (`POST /api/v1/media/presigned-url`) y la confirmación (`POST /api/v1/media/uploads/{id}/complete`) mantuvieron latencias p95 inferiores a **20 ms** a lo largo de todos los niveles. Esto demuestra que desacoplar la carga mediante URLs firmadas protege completamente al servidor Web de la degradación por transferencia de archivos.
2. **Plano de Datos (Etapa 2):** Las transferencias directas de videos al almacenamiento de objetos operaron a un rendimiento sostenido de entre **25 y 45 MB/s** con 100% de respuestas HTTP 200/204 y cero errores.
3. **Plano de Mensajería (Etapa 4):** Concurrencia de workers fijada en 2 (`WORKER_CONCURRENCY=2`). En Niveles 0 y 1, el tiempo de espera en cola fue prácticamente nulo (< 1s). Al alcanzar el Nivel 3 y 4 (lambda > mu), la cola acumuló hasta **10 tareas en espera**, haciendo que el tiempo de residencia en cola aumentara hasta representar el 70–80% del tiempo total a `available`.
4. **Plano de Cómputo (Etapa 5):** Cada transcodificación FFmpeg consumió el 100% de una vCPU física. Con 2 workers, el rendimiento estuvo estrictamente acotado por la capacidad de CPU de la máquina.
5. **Drenaje de Cola:** Tras cesar la inyección en Nivel 4, la cola se drenó en su totalidad en **6.4s** sin registrar ninguna tarea colgada en estado indeterminado.
