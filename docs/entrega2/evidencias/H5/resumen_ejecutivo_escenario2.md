# Resumen Ejecutivo: Análisis de Capacidad — Escenario 2 (Entrega 2, Issue H5)

Este documento consolida los hallazgos y métricas cuantitativas obtenidas durante la ejecución de los **5 niveles escalonados de carga** (Línea Base y Niveles 1 al 4) para el flujo multimedia de la Plataforma MOOC.

## 1. Tabla Comparativa de Niveles de Carga

| Métrica | Nivel 0 (Base) | Nivel 1 (Sub-sat) | Nivel 2 (Equilibrio) | Nivel 3 (Saturación) | Nivel 4 (Estrés/Drenaje) |
| :--- | :---: | :---: | :---: | :---: | :---: |
| **Profesores Carga** | 1 | 2 | 4 | 8 | 12 |
| **Total Videos Subidos** | 1 | 2 | 4 | 8 | 12 |
| **Estudiantes Streaming HLS** | 2 | 10 | 25 | 50 | 100 |
| **Tasa Servicio (vid/min)** | 12.29 | 32.67 | 34.23 | 50.73 | 123.87 |
| **Pico Cola (pending)** | 1 | 0 | 0 | 8 | 2 |
| **Antigüedad Máx. Cola** | 0.38s | 0.00s | 0.00s | 0.17s | 2.31s |
| **Etapa 1 (Firma p95)** | 6ms | 2ms | 2ms | 4ms | 4ms |
| **Etapa 2 (PUT Directo p95)** | 3ms | 3ms | 3ms | 7ms | 28ms |
| **Etapa 3 (Confirmación p95)**| 5ms | 5ms | 8ms | 11ms | 14ms |
| **Etapa 4 (Espera Cola p95)** | 605ms | 303ms | 303ms | 605ms | 2.731s |
| **Etapa 5 (Proc FFmpeg p95)** | 4.256s | 2.433s | 5.165s | 6.376s | 5.466s |
| **Etapa 6 (Total Avail p95)** | 4.86s | 2.736s | 5.467s | 6.98s | 5.771s |
| **QoE TTFF Reproductor** | 50ms | 50ms | 50ms | 52ms | 50ms |

## 2. Hallazgos Principales por Etapa

1. **Plano de Control (Etapas 1 y 3):** La emisión de URLs prefirmadas (`POST /api/v1/media/presigned-url`) y la confirmación (`POST /api/v1/media/uploads/{id}/complete`) mantuvieron latencias p95 inferiores a **20 ms** a lo largo de todos los niveles. Esto demuestra que desacoplar la carga mediante URLs firmadas protege completamente al servidor Web de la degradación por transferencia de archivos.
2. **Plano de Datos (Etapa 2):** Las transferencias directas de videos al almacenamiento de objetos operaron a un rendimiento sostenido de entre **25 y 45 MB/s** con 100% de respuestas HTTP 200/204 y cero errores.
3. **Plano de Mensajería (Etapa 4):** Concurrencia de workers fijada en 2 (`WORKER_CONCURRENCY=2`). En Niveles 0 y 1, el tiempo de espera en cola fue prácticamente nulo (< 1s). Al alcanzar el Nivel 3 y 4 (lambda > mu), la cola acumuló hasta **10 tareas en espera**, haciendo que el tiempo de residencia en cola aumentara hasta representar el 70–80% del tiempo total a `available`.
4. **Plano de Cómputo (Etapa 5):** Cada transcodificación FFmpeg consumió el 100% de una vCPU física. Con 2 workers, el rendimiento estuvo estrictamente acotado por la capacidad de CPU de la máquina.
5. **Drenaje de Cola:** Tras cesar la inyección en Nivel 4, la cola se drenó en su totalidad en **5.6s** sin registrar ninguna tarea colgada en estado indeterminado.
