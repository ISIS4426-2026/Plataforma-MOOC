# Resumen ejecutivo — Escenario 2 de capacidad, primera arquitectura con el código corregido

```
Destino medido   : https://34.24.52.111.sslip.io  [remoto]
Base HLS         : https://storage.googleapis.com/plataforma-mooc-entrega2-hls
Entorno          : nube-1st-architecture-codigo-corregido
Imagen desplegada: 3affa11394ea81c569b6eb111ec3e12b1df1bca8 (web y worker, verificado en el contenedor en marcha)
Patron de consumo: cadencia (segmentos por estudiante: 10)
Generador        : DESKTOP-NHAIFDB
Ventana UTC      : 2026-10-27T02:08:12Z - 2026-10-27T03:59:18Z
```

> **Este archivo se rederivó a mano el 2026-10-27**, de los artefactos medidos de
> esta misma corrida, después de corregir los escritores de evidencia. La sección
> 2 original eran cinco frases escritas fijas en el generador, heredadas de la
> Entrega 2, que contradecían a la tabla de la sección 1 en la misma página:
> afirmaban «p95 inferior a 20 ms» donde la tabla dice `n/d`, «entre 25 y 45 MB/s»
> donde se midió 0,71, «hasta 10 tareas en espera» con un pico de 14, y un drenaje
> «sin ninguna tarea colgada» con 19 trabajos sin estado terminal. Las correcciones
> del generador están en `scripts/capacity_escenario2/main.go`; a partir de la
> siguiente corrida esta sección se deriva sola.

## 1. Tabla comparativa de niveles de carga

| Métrica | Nivel 0 (Base) | Nivel 1 (Sub-sat) | Nivel 2 (Equilibrio) | Nivel 3 (Saturación) | Nivel 4 (Estrés/Drenaje) |
| :--- | :---: | :---: | :---: | :---: | :---: |
| **Profesores de carga** | 1 | 2 | 4 | 8 | 12 |
| **Vídeos subidos** | 1 | 2 | 4 | 8 | 12 |
| **Estudiantes HLS** | 2 | 10 | 25 | 50 | 100 |
| **Rendimiento en la ventana del nivel (vid/min)** | 0,60 | 0,68 | 0,07 | 0,20 | 0,00 |
| **Pico de cola (pending)** | 1 | 1 | 2 | 8 | **14** |
| **Antigüedad máxima en cola** | 0,42 s | 0,52 s | 850 s | 1 184 s | **3 723 s** |
| **Etapa 1 — firma, p95** | n/d (n=1) | n/d (n=2) | n/d (n=4) | n/d (n=8) | n/d (n=12) |
| **Etapa 1 — firma, media** | 300 ms | 129 ms | 122 ms | 125 ms | 174 ms |
| **Etapa 2 — PUT directo, p95** | n/d (n=1) | n/d (n=2) | n/d (n=4) | n/d (n=8) | n/d (n=12) |
| **Etapa 2 — caudal** | 2,14 MB/s | 1,77 MB/s | 1,73 MB/s | 1,22 MB/s | 0,71 MB/s |
| **Etapa 3 — confirmación, p95** | n/d (n=1) | n/d (n=2) | n/d (n=4) | n/d (n=8) | n/d (n=12) |
| **Etapa 3 — confirmación, media** | 156 ms | 110 ms | 299 ms | 178 ms | 908 ms |
| **Etapa 4 — espera en cola, p95** | n/d (n=1) | n/d (n=2) | n/d (n=1) | n/d (n=4) | n/d (n=0) |
| **Etapa 5 — FFmpeg, p95** | n/d (n=1) | n/d (n=2) | n/d (n=1) | n/d (n=4) | n/d (n=0) |
| **Etapa 6 — total a disponible, p95** | n/d (n=1) | n/d (n=2) | n/d (n=1) | n/d (n=4) | n/d (n=0) |
| **Primer segmento descargado** | 449 ms | 403 ms | 255 ms | 262 ms | 244 ms |
| **Latencia de segmento, p95** | 181 ms | 203 ms | n/d (n=0) | n/d (n=0) | n/d (n=0) |
| **Margen de buffer** | no publicable | no publicable | no aplica | no aplica | no aplica |
| **Interrupciones** | no publicable | no publicable | no aplica | no aplica | no aplica |

Tres lecturas de esta tabla, porque sin ellas engaña:

- **Ningún p95 por etapa es publicable.** Con 12 subidas como máximo, ningún nivel
  alcanza las 20 observaciones que un p95 necesita como estadístico de orden
  (§4 de `capacity-planning/criterios_de_medicion.md`). No es que estuvieran mal
  calculados: es que no existen. Por eso se publica la media, que con n pequeño
  sí significa algo.
- **Las etapas 4, 5 y 6 solo se registraron para 8 de los 27 trabajos.** Los otros
  19 no alcanzaron estado terminal dentro del presupuesto de su nivel, así que esas
  tres filas **subestiman el trabajo que la plataforma hizo**. La plataforma procesó
  los 27: están reconstruidos en [`etapas_reconstruidas.txt`](./etapas_reconstruidas.txt).
- **Los niveles 2, 3 y 4 no descargaron ni un segmento.** Sus vídeos seguían en la
  cola, así que no había manifiesto que pedir. «0 interrupciones» ahí no es
  reproducción perfecta: es que no se reprodujo nada.

El **tiempo al primer cuadro** no aparece. Solo se publica si lo produce un
reproductor real; ver §6 de `capacity-planning/criterios_de_medicion.md`. La
Entrega 2 lo reportó como latencia HTTP más 45 ms fijos, que es una constante y
no una medición.

## 2. Hallazgos por etapa

Cada número de esta sección sale de esta corrida, o no se escribe.

**1. Plano de control (etapas 1 y 3).** El p95 por nivel no es publicable, como
dice la tabla. Lo que sí está medido: **27 firmas y 27 confirmaciones con 0
fallos**, con un peor caso de **300 ms** al firmar y **908 ms** al confirmar.
Esos peores casos incluyen el ida y vuelta de red del generador, que es un
portátil fuera de la región, así que acotan el plano de control **por arriba**:
menos de un segundo frente a etapas de minutos. Y queda demostrado sin
necesidad de percentiles lo que el desacople pretende: **cero bytes multimedia
pasaron por la API**, porque el PUT va del cliente al almacenamiento con URL
prefirmada. La CPU del servidor web lo confirma: media de **2,67 %**, máximo
**7,35 %** mientras el worker estaba al 96 %.

**2. Plano de datos (etapa 2).** El PUT directo rindió entre **0,71 y 2,14 MB/s**,
y la descarga masiva **3,51 MB/s** como máximo. **Ninguno de los dos califica al
almacenamiento**: los dos miden el enlace de salida del generador. Para descartar
el almacenamiento como cuello de botella hay que medirlo desde una máquina en
`us-east1`. Queda anotado para la Entrega 3.

**3. Plano de mensajería (etapa 4).** Con `WORKER_CONCURRENCY=2`, la cola llegó a
un pico de **14 tareas en espera** (2 activas) y una antigüedad máxima de
**3 723 s**. Ninguna lectura de la cola falló, así que el pico y la antigüedad son
fiables. Esto es el hallazgo central: **lo que domina el tiempo hasta `available`
es la espera en cola, no el transcodificado**.

**4. Plano de cómputo (etapa 5).** La campaña **no mide CPU**: solo observa la cola
y la API. La evidencia de saturación está en
[`metricas_gcp.json`](./metricas_gcp.json), de Cloud Monitoring en la ventana:

| | `mooc-web-server` | `mooc-worker-server` |
| :--- | ---: | ---: |
| CPU media | 2,67 % | **96,31 %** |
| CPU p95 | 5,18 % | **99,95 %** |
| CPU máxima | 7,35 % | **99,97 %** |
| Memoria media | 28,04 % | 42,31 % |

Dos horas de worker clavado al 96 % de media mientras el servidor web no pasa del
7 % es el cuello de botella demostrado, no supuesto.

**5. Drenaje de cola.** Observable de principio a fin: ninguna lectura falló. La
cola bajó de **14 a 0 en espera** y las completadas subieron de **29 a 44** en
**1 h 9 min 7,9 s** de observación. El plazo se agotó con **2 tareas en proceso**,
así que **esa duración no es el tiempo de drenaje**: es cuánto se observó. Las dos
cerraron después; los 27 trabajos completaron.

> **Medición incompleta.** 19 trabajos no alcanzaron estado terminal dentro del
> presupuesto de su nivel y 0 fallaron. Las etapas 4, 5 y 6 solo se registraron
> para 8 trabajos.

## 3. Lo que la plataforma sí registró

Reconstruido del log del worker y de la hora de creación de los objetos en el
bucket, en [`etapas_reconstruidas.txt`](./etapas_reconstruidas.txt): **27 de 27
trabajos**, frente a los 8 que la campaña alcanzó a cronometrar.

| Etapa, agregado de los tres perfiles (n=27) | p50 | p95 |
| :--- | ---: | ---: |
| Espera en cola | 974 s | 3 724 s |
| Procesamiento FFmpeg | 177 s | 863 s |
| **Total hasta disponible** | **1 325 s** | **4 557 s** |

Con 27 observaciones el p95 existe; el p99 necesita 100 y no. Por perfil no se
llega a 20, así que allí se publica mediana, mínimo y máximo.

**La espera en cola es el 73 % del p50 del total.** El transcodificado no es el
problema: la capacidad para atenderlo, sí.

El factor de transcodificación por perfil, que es lo que dimensiona el autoescalado:

| Perfil | n | Mediana | Factor sobre la duración del vídeo |
| :--- | ---: | ---: | ---: |
| corto (120 s) | 16 | 170,2 s | **1,39×** |
| medio (600 s) | 10 | 855,7 s | **1,42×** |
| largo (1 800 s) | 1 | 1 711,3 s | 0,95× |

## 4. Qué invalida esto de la Entrega 2

| Publicado en la Entrega 2 | Medido aquí |
| :--- | :--- |
| Tiempo hasta disponible 4,259–6,385 **s** | **1 325 s** de mediana |
| «0 interrupciones, +5,99 s de margen» | No medible: derivado de una constante |
| p95 por etapa | No existían: ningún nivel llega a 20 observaciones |
| Medido contra Docker Compose | Medido contra la nube, con los perfiles reales |

La diferencia de tres órdenes de magnitud en el tiempo hasta disponible no es una
regresión de la plataforma: es que la Entrega 2 midió con clips de 8, 20 y 40
segundos, no con los perfiles de 2, 10 y 30 minutos que declara el plan.

---

Este archivo no contiene credenciales, llaves ni secretos.
