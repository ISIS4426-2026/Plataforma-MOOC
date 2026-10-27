# Informe de pruebas de carga y capacidad — Escenario 2

**Curso:** ISIS4426 — Desarrollo de Soluciones Cloud
**Semestre:** 2026-20
**Fecha de consolidación:** 2026-10-27T03:59:18Z

---

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
> corregir el generador. Las secciones 3 y 4 originales llevaban cifras fijas
> heredadas de la Entrega 2 que esta corrida contradice: afirmaban
> «pending = 0, processing = 0» con 2 recursos en `processing`, «el 100% de los
> trabajos confirmados terminaron en completed» con 19 sin estado terminal,
> «p95 inferiores a 20 ms» donde no hay p95, y «hasta 10 tareas en cola» con un
> pico de 14.

## Escenario 2: carga, procesamiento y consumo multimedia (10%)

### 1. Definición del escenario y condiciones de prueba

El escenario simula profesores subiendo contenido multimedia directamente al
almacenamiento de objetos mientras estudiantes reproducen flujos HLS ya
disponibles.

- **Arquitectura medida.** La de la Entrega 2: **dos VM fijas**
  (`mooc-web-server` y `mooc-worker-server`, ambas `e2-highcpu-2`), sin grupos
  administrados, sin balanceador y sin autoescalado. Cloud SQL PostgreSQL 16
  (`db-custom-1-3840`) con IP privada.
- **Código medido.** `3affa11`, el **corregido**, no el del tag `entrega-2`. Es
  el mismo que correrá la arquitectura elástica, para que la comparación de G3 sea
  puramente arquitectónica.
- **Concurrencia de workers.** Fijada en 2 (`WORKER_CONCURRENCY=2`), un proceso
  de transcodificación por vCPU de `mooc-worker-server`.
- **Perfiles multimedia y escalera.** Perfiles corto (2 min), medio (10 min) y
  largo (30 min) a 1280×720. Escalera HLS a 360p y 720p sin upscaling
  (`internal/transcode.DefaultLadder`). Son los perfiles **verificados en A5**,
  no los clips de 8/20/40 s con los que midió la Entrega 2.
- **Patrón de consumo declarado:** `cadencia`. Cada estudiante reproduce a 1×:
  buffer inicial de 2 segmentos y una pausa por segmento igual a su duración real,
  leída del `#EXTINF` de la lista de variante. La descarga masiva se mide aparte.
- **Acotación del consumo.** 10 segmentos por estudiante, para que el egreso del
  consumo HLS no costara más que todas las horas de máquina juntas.

### 2. Resultados numéricos por nivel

| Nivel | Subidas / concurrencia | Estudiantes HLS | Rendimiento en la ventana (vid/min) | Pico cola | Antigüedad cola | Etapa 1 p95 | Etapa 2 p95 | Etapa 3 p95 | Etapa 4 p95 | Etapa 5 p95 | Etapa 6 p95 |
| :---: | :---: | :---: | :---: | :---: | :---: | :---: | :---: | :---: | :---: | :---: | :---: |
| **0** | 1 (1 prof) | 2 est | 0,60 | 1 | 0,42 s | n/d (n=1) | n/d (n=1) | n/d (n=1) | n/d (n=1) | n/d (n=1) | n/d (n=1) |
| **1** | 2 (2 prof) | 10 est | 0,68 | 1 | 0,52 s | n/d (n=2) | n/d (n=2) | n/d (n=2) | n/d (n=2) | n/d (n=2) | n/d (n=2) |
| **2** | 4 (4 prof) | 25 est | 0,07 | 2 | 850,44 s | n/d (n=4) | n/d (n=4) | n/d (n=4) | n/d (n=1) | n/d (n=1) | n/d (n=1) |
| **3** | 8 (8 prof) | 50 est | 0,20 | 8 | 1 183,61 s | n/d (n=8) | n/d (n=8) | n/d (n=8) | n/d (n=4) | n/d (n=4) | n/d (n=4) |
| **4** | 12 (12 prof) | 100 est | 0,00 | 14 | 3 722,72 s | n/d (n=12) | n/d (n=12) | n/d (n=12) | n/d (n=0) | n/d (n=0) | n/d (n=0) |

Ningún p95 por etapa existe: con 12 subidas como máximo ningún nivel llega a las
20 observaciones que un p95 necesita (§4 de
`capacity-planning/criterios_de_medicion.md`). Las medias y el agregado de los 27
trabajos están en [`resumen_ejecutivo_escenario2.md`](./resumen_ejecutivo_escenario2.md)
y [`etapas_reconstruidas.txt`](./etapas_reconstruidas.txt).

El **rendimiento en la ventana del nivel** no es la tasa de servicio de la
plataforma: en los niveles 2 a 4 cae porque los trabajos terminan **después** de
cerrarse la ventana de su nivel, no porque la plataforma vaya más lenta.

### 3. Observación del drenaje de cola y verificación terminal

Tras cesar la inyección en el nivel 4 se observó el vaciado de la cola Asynq
durante **1 h 9 min 7,9 s**, partiendo de 14 tareas en espera y terminando con
**0 en espera y 2 en proceso**. Las completadas subieron de **29 a 44**. Ninguna
lectura de la cola falló, así que la cronología de
[`drenaje_cola_verificacion.txt`](./drenaje_cola_verificacion.txt) no tiene huecos.

> La ventana terminó con trabajo en vuelo, así que **esa duración no es el tiempo
> de drenaje**: es cuánto se observó.

La verificación directa sobre PostgreSQL
(`SELECT processing_status, COUNT(*) FROM resources WHERE object_key LIKE 'originals/%' GROUP BY processing_status`)
dio, en el instante del corte:

- `processing_status = "completed"`: **25** recursos
- `processing_status = "processing"`: **2** recursos

**Los 2 recursos sin estado terminal no son trabajos perdidos**: son trabajos que
la ventana de observación no alcanzó a cubrir. Cerraron después; el log del worker
registra **27 de 27** `job media:process completed`.

### 4. Cuello de botella sustentado y propuesta de evolución

1. **Cuello de botella primario.** La **vCPU del worker durante la
   transcodificación (etapa 5)**. Lo sostiene la cola —pico de **14 en espera** y
   antigüedad máxima de **3 723 s** con tasa de llegada constante— y lo confirma
   Cloud Monitoring: worker al **96,31 %** de CPU media frente al **2,67 %** del
   servidor web. El análisis completo está en
   [`analisis_cuello_de_botella.md`](./analisis_cuello_de_botella.md).
2. **El almacenamiento no queda descartado.** El PUT rindió 0,71–2,14 MB/s y la
   descarga masiva 3,51 MB/s, pero los dos miden el enlace del generador, no el
   almacenamiento. Pendiente de repetir desde `us-east1`.
3. **Evolución — CDN.** Frontalizar el prefijo público `/hls/` con un CDN, para
   servir los segmentos inmutables desde el borde y quitar el egreso del bucket de
   origen. Objetivos a verificar en C7.
4. **Evolución — autoescalado de workers.** Escalar `mooc-worker-server` por
   `worker.queue.oldest_pending_age_seconds`, la señal que aquí llegó a 3 723 s y
   que distingue una ráfaga absorbida de una cola que no se recupera. El nivel más
   cargado pide del orden de 12 ranuras de transcodificación, seis veces la
   capacidad actual; el dimensionado se cierra en C6 con los factores por perfil.

---

Este archivo no contiene credenciales, llaves ni secretos.
