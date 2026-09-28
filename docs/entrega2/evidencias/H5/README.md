# Evidencia H5 — Análisis de Capacidad: Escenario 2 (Carga, Procesamiento y Consumo Multimedia)

Este directorio documenta el cumplimiento exhaustivo del entregable **H5**, correspondiente a la rúbrica **Análisis de capacidad — escenario 2 (10%)** de la Entrega 2.

## Estado y Verificación de Criterios de Aceptación

| Criterio de la Rúbrica / Tarea | Evidencia Generada | Estado |
| :--- | :--- | :---: |
| **Línea base y al menos tres niveles crecientes** | 5 niveles ejecutados (Nivel 0 Base, Niveles 1 al 4) documentados en [`corrida_niveles_escenario2.txt`](./corrida_niveles_escenario2.txt) | ✅ |
| **Métricas separadas por etapa (no agregado único)** | 6 etapas desacopladas medidas y cronometradas en [`resumen_ejecutivo_escenario2.md`](./resumen_ejecutivo_escenario2.md) | ✅ |
| **Trabajos completados por unidad de tiempo, reintentos y fallos** | Throughput registrado (videos/min), cero fallos y cero retries en [`corrida_niveles_escenario2.txt`](./corrida_niveles_escenario2.txt) | ✅ |
| **Evolución de profundidad y antigüedad de la cola** | Métricas `worker.queue.pending` y `oldest_pending_age_seconds` capturadas por nivel | ✅ |
| **Latencia, throughput y errores en storage vs control API** | Tráfico de control (API REST) vs transferencia binaria (Object Storage) rigurosamente segregados | ✅ |
| **Latencia y errores de manifiestos y segmentos HLS** | Streaming con cadencia real (pacing 6.0s) vs greedy bulk download medidos en todos los niveles | ✅ |
| **Decisión de reproductor real (TTFF y stalls)** | Sonda emuladora de eventos HTML5/MSE documentada, justificando por qué HTTP puro no prueba renderizado | ✅ |
| **Observación del drenaje y verificación de consistencia** | Vaciado total cronometrado en [`drenaje_cola_verificacion.txt`](./drenaje_cola_verificacion.txt), 0 trabajos huérfanos | ✅ |
| **Cuello de botella identificado y evolución (CDN / escalado)** | Análisis cuantitativo sustentado en [`analisis_cuello_de_botella.md`](./analisis_cuello_de_botella.md) | ✅ |
| **Seguridad de secretos estricta** | Cero credenciales, tokens o firmas V4 en repositorios ni logs (sanitización automática) | ✅ |

## Archivos de Evidencia en este Directorio

1. [`corrida_niveles_escenario2.txt`](./corrida_niveles_escenario2.txt): Registro crudo y estructurado de la ejecución de los 5 niveles.
2. [`drenaje_cola_verificacion.txt`](./drenaje_cola_verificacion.txt): Traza de segundo a segundo del vaciado de la cola y verificación terminal en PostgreSQL.
3. [`resumen_ejecutivo_escenario2.md`](./resumen_ejecutivo_escenario2.md): Tabla consolidada y comparativa de métricas por etapa a lo largo de los niveles.
4. [`analisis_cuello_de_botella.md`](./analisis_cuello_de_botella.md): Demostración del cuello de botella en vCPU de FFmpeg y modelado de CDN y autoescalado.

## Cómo Reproducir las Pruebas

```bash
# 1. Levantar servicios locales
docker compose up -d

# 2. Ejecutar la suite completa de capacidad de Escenario 2
bash ./scripts/run_capacity_escenario2.sh

# 3. Opcional: Ejecutar contra despliegue en la nube
bash ./scripts/run_capacity_escenario2.sh https://34.24.52.111.sslip.io
```
