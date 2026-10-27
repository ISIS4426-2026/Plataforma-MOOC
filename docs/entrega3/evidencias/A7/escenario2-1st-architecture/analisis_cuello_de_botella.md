# Análisis del cuello de botella y propuesta de evolución arquitectural

```
Destino medido   : https://34.24.52.111.sslip.io  [remoto]
Base HLS         : https://storage.googleapis.com/plataforma-mooc-entrega2-hls
Entorno          : nube-1st-architecture-codigo-corregido
Imagen desplegada: 3affa11394ea81c569b6eb111ec3e12b1df1bca8
Patron de consumo: cadencia (segmentos por estudiante: 10)
Generador        : DESKTOP-NHAIFDB
Ventana UTC      : 2026-10-27T02:08:12Z - 2026-10-27T03:59:18Z
```

> **Rederivado a mano el 2026-10-27** de los artefactos medidos, después de
> corregir el generador. La sección 1 original llevaba cifras fijas heredadas de
> la Entrega 2 que esta corrida contradice: afirmaba PUT «> 30 MB/s» con 0,71
> medidos, descarga HLS «> 120 MB/s» con 3,51, antigüedad de cola que «superó los
> 35 segundos» con 3 723, y cómputo «~2,8 s por tarea» con 95–168 s reales.

## 1. Identificación del cuello de botella primario

El cuello de botella es la **capacidad de cómputo (vCPU) del worker durante la
transcodificación con FFmpeg**, y está demostrado por dos vías independientes.

**Vía 1 — la cola, que es lo que mide la campaña.** Con `WORKER_CONCURRENCY=2` y
una tasa de llegada constante por nivel, la cola creció monótonamente:

| Nivel | Subidas | Pico en espera | Antigüedad máxima |
| :--- | ---: | ---: | ---: |
| 0 | 1 | 1 | 0,42 s |
| 1 | 2 | 1 | 0,52 s |
| 2 | 4 | 2 | 850 s |
| 3 | 8 | 8 | 1 184 s |
| 4 | 12 | **14** | **3 723 s** |

Una cola que crece con la tasa de llegada constante no tiene otra causa que una
tasa de servicio insuficiente. El salto está entre el nivel 1 y el 2: con 4
subidas concurrentes la antigüedad pasa de medio segundo a catorce minutos.

**Vía 2 — la CPU, de Cloud Monitoring.** La campaña no mide CPU; esto sale de
[`metricas_gcp.json`](./metricas_gcp.json), recogido en la ventana de la corrida:

| | `mooc-web-server` | `mooc-worker-server` |
| :--- | ---: | ---: |
| CPU media | 2,67 % | **96,31 %** |
| CPU p95 | 5,18 % | **99,95 %** |
| CPU máxima | 7,35 % | **99,97 %** |

Casi dos horas con el worker al 96 % de media y el servidor web por debajo del
7 %. El reparto no deja ambigüedad sobre dónde está el límite.

### Qué sostiene esta campaña, y qué no

1. **Descarte del servidor web (plano de control).** El p95 por nivel no es
   publicable: con 12 subidas como máximo ningún nivel llega a las 20
   observaciones que un p95 necesita. Lo medido son **27 firmas y 27
   confirmaciones con 0 fallos**, peor caso **300 ms** y **908 ms**, que incluyen
   el ida y vuelta de red del generador. Lo que sí queda demostrado sin
   percentiles: **cero bytes multimedia pasaron por la API**, y su CPU no se movió.

2. **El almacenamiento NO queda descartado.** El PUT directo rindió
   **0,71–2,14 MB/s** y la descarga masiva **3,51 MB/s**. Los dos miden el enlace
   de salida del portátil que ejecuta la campaña, no el almacenamiento. Decir que
   el almacenamiento no es el cuello de botella con estos números sería afirmar
   más de lo medido; hay que repetirlo desde una máquina en `us-east1`.

3. **Reparto del ciclo de vida.** De 27 trabajos, la campaña solo cronometró las
   etapas 4, 5 y 6 de **8**: los otros 19 no alcanzaron estado terminal dentro del
   presupuesto de su nivel. El reparto real, reconstruido de los registros del
   propio worker en [`etapas_reconstruidas.txt`](./etapas_reconstruidas.txt) para
   los 27:

   | Etapa (n=27) | p50 | p95 |
   | :--- | ---: | ---: |
   | Espera en cola | 974 s | 3 724 s |
   | Procesamiento FFmpeg | 177 s | 863 s |
   | **Total hasta disponible** | **1 325 s** | **4 557 s** |

   **La espera en cola es el 73 % del p50.** El transcodificado de un vídeo no es
   lento; lo que es insuficiente es el número de vídeos que se pueden transcodificar
   a la vez.

## 2. Modelado de mejoras arquitecturales

> Las cifras de esta sección son **objetivos de diseño a verificar**, no mediciones
> de esta corrida.

### A. Red de entrega de contenidos (CDN) para la distribución HLS

- **Situación actual.** Los reproductores piden manifiestos y segmentos
  directamente al bucket: cada petición es una operación GET facturada más egreso
  a internet. Esta campaña lo acotó a 10 segmentos por estudiante justamente para
  que el egreso no costara más que todas las horas de máquina juntas.
- **Qué cambia.** Un CDN delante del prefijo público `/hls/` sirve los segmentos
  desde el borde. Los segmentos son inmutables, así que la tasa de acierto debería
  ser alta. El objetivo concreto y la forma de comprobarlo quedan para **C7**; el
  ahorro de egreso está razonado en la nota técnica 13.
- **Lo que esta corrida aporta al dimensionado.** La latencia de segmento medida
  desde el generador fue p95 **181–203 ms** en los dos niveles que descargaron
  algo. Es el número contra el que el CDN tendrá que mostrar mejora.

### B. Escalado dinámico de workers (MIG con autoescalado)

- **Métrica de escalado.** `worker.queue.oldest_pending_age_seconds`, que esta
  corrida vio llegar a **3 723 s** (Cloud Monitoring registra un p95 de 2 746 s en
  la ventana). Es la señal que distingue una ráfaga absorbida de una cola que no se
  recupera, y la profundidad (`pending`) sola no lo distingue: el nivel 1 y el
  nivel 0 tuvieron el mismo pico de 1 con antigüedades de 0,52 y 0,42 s, mientras
  el nivel 2 con pico 2 llegó a 850 s.
- **Regla.** Umbral sobre esa antigüedad sostenido durante un minuto, añadiendo
  instancias de worker; retirada en el valle. Cada instancia añade 2 vCPU, es decir
  **duplica** la tasa de servicio actual.
- **Cuánto hace falta.** Con los factores medidos —1,39× el perfil corto, 1,42× el
  medio— la campaña inyectó 9 720 s de vídeo, que son unos **13 600 s de vCPU**.
  La corrida duró 6 666 s de pared, y 2 vCPU en ese tiempo dan **13 330 s**. La
  demanda excedió a la capacidad del principio al fin, y de ahí sale tanto el 96 %
  de CPU media como una cola que no se recupera en ningún nivel: no es un pico mal
  absorbido, es que la capacidad nunca alcanzó.
- **Dimensionado del nivel más cargado.** El nivel 4 inyectó 12 vídeos
  simultáneos. Para que la antigüedad en cola no crezca, hace falta una ranura por
  subida concurrente: con 12 ranuras el nivel se cierra en el tiempo del trabajo
  más largo —unos 850 s para un perfil medio— en vez de en los 3 723 s que tardó
  el último. Son 12 vCPU, es decir **6 instancias** de `e2-highcpu-2` frente a la
  una actual. Ese es el dimensionado que C6 tiene que justificar, y sale de números
  medidos, no de un supuesto.

### C. Ajuste de concurrencia por nodo

- **Por qué 2.** En `e2-highcpu-2` (2 vCPU) hay un proceso de transcodificación por
  vCPU. Subir la concurrencia en esta misma máquina reparte las mismas vCPU entre
  más procesos: no añade capacidad, añade cambios de contexto. La memoria del
  worker se quedó en 42 % de media y 49,6 % de máximo, así que el límite aquí es
  CPU, no RAM.
- **Evolución de tipo de máquina.** Más vCPU por nodo permite más concurrencia real.
  El tipo concreto y su coste se deciden en **C6** con los factores por perfil de
  `etapas_reconstruidas.txt`, no con un supuesto.

---

Este archivo no contiene credenciales, llaves ni secretos.
