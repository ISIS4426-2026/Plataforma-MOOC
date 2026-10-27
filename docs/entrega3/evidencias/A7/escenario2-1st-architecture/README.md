# Escenario 2 de la primera arquitectura, con el código corregido

Línea base retrospectiva del flujo multimedia sobre la arquitectura de la
Entrega 2 —**dos VM fijas, sin grupos administrados, sin balanceador y sin
autoescalado**— medida contra la nube con los perfiles reales de A5.

```
Destino medido   : https://34.24.52.111.sslip.io  [remoto]
Base HLS         : https://storage.googleapis.com/plataforma-mooc-entrega2-hls
Entorno          : nube-1st-architecture-codigo-corregido
Imagen desplegada: 3affa11394ea81c569b6eb111ec3e12b1df1bca8 (web y worker)
Patron de consumo: cadencia (segmentos por estudiante: 10)
Generador        : DESKTOP-NHAIFDB
Ventana UTC      : 2026-10-27T02:08:12Z - 2026-10-27T03:59:18Z
```

**Esta es la línea base de G3 para el escenario 2.** Corre el mismo código
`3affa11` que correrá la arquitectura elástica, así que su diferencia con ella es
puramente arquitectónica: no hay que descontar el coste de las correcciones.

**No empieces por aquí.** El resultado está en
[`../COMPARACION.md`](../COMPARACION.md) §3. Si solo lees un número de este
directorio, que sea este:

| | `mooc-web-server` | `mooc-worker-server` |
| :--- | ---: | ---: |
| CPU media durante 1 h 51 min | 2,67 % | **96,31 %** |
| CPU p95 | 5,18 % | **99,95 %** |

## Medición incompleta, y por qué eso es el hallazgo

De 27 trabajos, la campaña alcanzó a cronometrar las etapas 4, 5 y 6 de **8**.
Los otros 19 no alcanzaron estado terminal dentro del presupuesto de su nivel.

**No es un fallo de la plataforma: la plataforma procesó los 27.** Es que con
`WORKER_CONCURRENCY=2` la cola llegó a 14 tareas en espera y 3 723 s de
antigüedad, y la espera en cola es el **73 %** del tiempo hasta disponible. La
campaña sondea la API para ver cuándo un recurso pasa a `completed`; un trabajo
encolado detrás de dieciséis no cambia de estado durante una hora.

Lo que la plataforma sí registró está en
[`etapas_reconstruidas.txt`](./etapas_reconstruidas.txt), reconstruido del log del
worker y de la hora de creación de los objetos en el bucket: **27 de 27**.

| Etapa, agregado (n=27) | p50 | p95 |
| :--- | ---: | ---: |
| Espera en cola | 974 s | 3 724 s |
| Procesamiento FFmpeg | 177 s | 863 s |
| **Total hasta disponible** | **1 325 s** | **4 557 s** |

## Estado de los criterios de aceptación

| Criterio de la rúbrica | Evidencia | Estado |
| :--- | :--- | :---: |
| Línea base y al menos tres niveles crecientes | 5 niveles en [`corrida_niveles_escenario2.txt`](./corrida_niveles_escenario2.txt) | ✅ |
| Métricas separadas por etapa, no un agregado único | 6 etapas en [`resumen_ejecutivo_escenario2.md`](./resumen_ejecutivo_escenario2.md); las 4–6 completas en [`etapas_reconstruidas.txt`](./etapas_reconstruidas.txt) | ✅ |
| Trabajos por unidad de tiempo, reintentos y fallos | 0 fallos reales, 0 reintentos, 19 sin estado terminal al cierre de su nivel | ⚠️ |
| Evolución de profundidad y antigüedad de la cola | pico 14 en espera, antigüedad máxima 3 723 s, **0 errores de lectura** de la cola | ✅ |
| Latencia y throughput de storage frente a la API de control | tráfico de control y transferencia binaria separados; PUT 0,71–2,14 MB/s, **medido desde el generador, no desde la región** | ⚠️ |
| Latencia y errores de manifiestos y segmentos HLS | 120 descargas de segmento medidas; **3 de 5 niveles sin ningún segmento** porque sus vídeos seguían en la cola | ⚠️ |
| Decisión de reproductor real (TTFF y stalls) | **Pendiente.** Esta campaña mide transporte, no reproducción. Criterio en `capacity-planning/criterios_de_medicion.md` §6 | ⏳ |
| Observación del drenaje y consistencia | [`drenaje_cola_verificacion.txt`](./drenaje_cola_verificacion.txt): 14 → 0 en espera en 1 h 9 min, con 2 en proceso y 2 recursos sin estado terminal al cierre | ⚠️ |
| Cuello de botella identificado y evolución (CDN / escalado) | [`analisis_cuello_de_botella.md`](./analisis_cuello_de_botella.md) | ✅ |
| Seguridad de secretos | sin credenciales, tokens ni firmas V4 en la evidencia | ✅ |

Los cuatro ⚠️ son limitaciones **declaradas**, no fallos ocultos. Las tres
primeras tienen la misma causa —la cola no absorbe los perfiles reales con 2
vCPU—, que es justamente lo que esta línea base tenía que demostrar.

## Qué hay en cada archivo

| | |
| :--- | :--- |
| [`corrida_niveles_escenario2.txt`](./corrida_niveles_escenario2.txt) | Registro por nivel, con las seis etapas, la cola y el consumo HLS |
| [`drenaje_cola_verificacion.txt`](./drenaje_cola_verificacion.txt) | Cronología segundo a segundo del vaciado y verificación terminal en PostgreSQL |
| [`resumen_ejecutivo_escenario2.md`](./resumen_ejecutivo_escenario2.md) | Tabla comparativa de los cinco niveles y los hallazgos por etapa |
| [`analisis_cuello_de_botella.md`](./analisis_cuello_de_botella.md) | Qué sostiene la campaña y qué no, y la propuesta de evolución con números |
| [`etapas_reconstruidas.txt`](./etapas_reconstruidas.txt) | Etapas 4, 5 y 6 de **todos** los trabajos, desde el log del worker |
| [`metricas_gcp.json`](./metricas_gcp.json) | CPU, memoria, conexiones y cola de Cloud Monitoring en la ventana. Las métricas caducan: se recogieron el mismo día |
| [`worker_crudo.log`](./worker_crudo.log) | Log del contenedor del worker, origen de la reconstrucción |
| [`objetos_originales.txt`](./objetos_originales.txt) | Listado del bucket con la hora de creación de cada original, el otro origen |
| [`ventana.txt`](./ventana.txt) | Inicio y fin UTC. Lo lee `capacity_metricas_gcp.py` |
| [`tunel.log`](./tunel.log) | Reinicios del túnel IAP: dice si la ventana tuvo huecos de visibilidad. **Cero reinicios** |

## Correcciones aplicadas a esta evidencia

Los documentos de conclusiones se rederivaron a mano el **2026-10-27**, después de
corregir tres defectos del generador. El proceso que escribió la evidencia ya no
existía, así que no se pudo volver a pasar sobre los mismos datos.

| Defecto | Qué publicaba | Medido |
| :--- | :--- | :--- |
| Conclusiones escritas fijas, heredadas de la Entrega 2 | PUT «> 30 MB/s», HLS «> 120 MB/s», antigüedad «> 35 s», cómputo «~2,8 s» | 0,71–2,14 MB/s, 3,51 MB/s, 3 723 s, 95–168 s |
| Veredicto del drenaje escrito fijo | «sin estados indeterminados», dos líneas después de «huérfanos: 2» | 2 en `processing` |
| Reloj del instante de reproducción | margen −14,40 s fijo y el **100 %** de los segmentos con plazo como interrupción | no publicable; con el reloj correcto el margen es la profundidad del buffer |

Las tres están corregidas en `scripts/capacity_escenario2/main.go`: a partir de la
siguiente corrida estas secciones se derivan solas, y un veredicto que no puede
fallar ya no existe.

## Cómo se reprodujo

```bash
ENTORNO=nube-1st-architecture-codigo-corregido \
HLS_BASE=https://storage.googleapis.com/plataforma-mooc-entrega2-hls \
PATRON=cadencia MAX_SEGMENTOS=10 \
  bash ./scripts/run_capacity_escenario2.sh https://34.24.52.111.sslip.io \
    -worker-concurrency 2 -redis localhost:6379 -db "postgres://...@localhost:5432/moocdb?sslmode=require"
```

La cola y la base se alcanzan por el túnel IAP (`scripts/a7_tunel.sh`), que usa el
22 y no abre ningún puerto de datos: **F1** sigue pudiendo demostrar que no hay
puertos de aplicación ni de datos expuestos.

El entorno está destruido. Estos archivos no se pueden regenerar.

---

Este archivo no contiene credenciales, llaves ni secretos.
