# Análisis del Cuello de Botella y Propuesta de Evolución Arquitectural (H5)

## 1. Identificación y Sustentación del Cuello de Botella Primario

A partir de la instrumentación desacoplada en 6 etapas y los 5 niveles evaluados, el **cuello de botella primario** del flujo multimedia queda identificado de forma concluyente en la **capacidad de cómputo (vCPU) de los workers durante la transcodificación con FFmpeg (Etapa 5)**.

### Evidencia Cuantitativa que lo Sustenta

1. **Descarte del Servidor Web (Control Plane):**
   - Latencia p95 de emisión de URL firmada: **< 15 ms** en todos los niveles.
   - Latencia p95 de confirmación de carga: **< 18 ms** en todos los niveles.
   - Tasa de error en la API: **0.00%** (cero errores 5xx).
   - Cero bytes multimedia pasaron por la interfaz de red de la API.

2. **Descarte del Almacenamiento de Objetos (Data Plane):**
   - Rendimiento sostenido de subida directa (PUT): **> 30 MB/s**.
   - Rendimiento sostenido de descarga HLS (GET): **> 120 MB/s** en modo ráfaga greedy.
   - Tasa de errores en el bucket: **0.00%** (cero respuestas 429, 403 o 503).

3. **Saturación en Worker Server (Cómputo Asíncrono):**
   - En la instancia `mooc-worker-server` (`e2-highcpu-2`), cada proceso FFmpeg transcodificando a 360p y 720p sin upscaling satura exactamente el 100% de una vCPU dedicada.
   - Con `WORKER_CONCURRENCY=2`, la capacidad de servicio máxima del nodo es de **2 tareas simultáneas**.
   - Cuando la tasa de llegada superó mu en los Niveles 3 y 4, la profundidad de la cola creció monótonamente hasta alcanzar tareas en espera y la antigüedad máxima de la cola superó los 35 segundos.
   - En Nivel 4, el tiempo de espera en cola dominó el ciclo de vida, elevando el tiempo total hasta `available` a más de 18 segundos, a pesar de que el cómputo puro de transcodificación se mantuvo constante en ~2.8s por tarea.

## 2. Modelado de Mejoras Arquitecturales y Evolución

### A. Red de Entrega de Contenidos (CDN) para Distribución HLS
- **Situación actual:** Los reproductores solicitan manifiestos y segmentos `.ts` directamente al endpoint del bucket de almacenamiento. Aunque el almacenamiento tiene alta disponibilidad, cada petición consume operaciones GET facturadas y ancho de banda de egreso a internet.
- **Impacto de CDN:**
  * **Cache Hit Ratio esperado:** > 95% para segmentos de video estáticos.
  * **Reducción de latencia:** El TTFB del manifiesto y segmentos baja de ~70–120 ms a **< 15 ms** desde nodos perimetrales (Edge).
  * **Ahorro de costos y egreso:** Alivio total del ancho de banda de salida del bucket origin (Nota técnica 13).

### B. Escalamiento Dinámico de Workers (Auto-scaling / MIG)
- **Métrica de escalamiento:** Se propone autoescalar el grupo de workers utilizando la métrica `worker.queue.pending` o `worker.queue.oldest_pending_age_seconds`.
- **Regla de escalado:**
  * Si `worker.queue.oldest_pending_age_seconds > 45s` durante 1 minuto -> Disparar escalamiento horizontal agregando 1 o 2 instancias `mooc-worker-server`.
  * Cada nueva instancia duplica la tasa de servicio mu (+2 vCPUs), vaciando la cola en ráfagas sin incurrir en costos fijos durante periodos valle.

### C. Ajuste de Concurrencia por Nodo
- **Justificación de concurrencia actual:** En `e2-highcpu-2` (2 vCPU, 2 GiB RAM), `WORKER_CONCURRENCY=2` es óptimo. Intentar forzar concurrencia 4 en esta máquina provocaría contención severa de cambios de contexto en CPU y riesgo inminente de *Out Of Memory* (OOM), ya que cada transcodificación demanda ~500 MiB de RAM.
- **Evolución de tipo de máquina:** Para mayor densidad sin aumentar el número de VMs, migrar a `c2-standard-4` (4 vCPUs dedicadas, 16 GiB RAM) permitiría elevar `WORKER_CONCURRENCY=4` de forma segura, duplicando el rendimiento por nodo.
