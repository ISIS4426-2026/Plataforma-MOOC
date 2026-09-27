# Evidencia G1 — Datos sintéticos, multimedia de tres perfiles, siembra del bucket y conciliación (issue #127)

G1 tiene dos partes independientes: cuentas para los escenarios de carga
(`scripts/seeds/capacity_data.sql`) y multimedia real sembrada a través del
pipeline de producción (`cmd/seed-media`, este directorio). Documentado en
detalle en [`docs/DATOS_SINTETICOS.md`](../../../DATOS_SINTETICOS.md) §7.

## Estado

| Archivo | Qué demuestra | |
| :--- | :--- | :--- |
| [`manifest.json`](./manifest.json) | Los 714 objetos reales (1 original + derivados por perfil), sus tamaños, hashes SHA-256 del original, y `reconciliation_ok: true` con la lista de discrepancias vacía | ✅ |
| [`corrida_seed_media.txt`](./corrida_seed_media.txt) | Log completo de la corrida real: los tres perfiles procesados por el worker de producción, con tiempos de transcodificación | ✅ |

Corrida real, local, contra MinIO y PostgreSQL en Docker Compose
(2026-09-27):

| Perfil | Duración | Original | Objetos totales | Tiempo de transcodificación |
| :--- | ---: | ---: | ---: | ---: |
| Corto | 2 min | 2.6 MB | 38 (1 original + 37 derivados) | 43.7 s |
| Medio | 10 min | 13.0 MB | 172 (1 original + 171 derivados) | 3 min 52 s |
| Largo | 30 min | 39.0 MB | 504 (1 original + 503 derivados) | 11 min 13 s |
| **Total** | | **54.6 MB originales** | **714 objetos, 765.8 MB** | |

`reconciliation_ok: true` — cero discrepancias entre lo que el comando
subió y lo que el bucket reporta al volver a consultarlo objeto por objeto.
El curso técnico que los contiene (`d9000000-...-000000000001`) quedó
publicado al terminar los tres perfiles.

## No había objetos previos que migrar

Este issue absorbe el trabajo original de "migrar objetos existentes". No
aplica: la Entrega 1 nunca implementó almacenamiento de objetos (issue #23 del
backlog original nunca se construyó), así que no existía ningún objeto en
`originals/` ni `hls/` antes de este comando. Todo lo que hay bajo esos dos
prefijos en el bucket lo escribió `cmd/seed-media` la primera vez que corrió.
`manifest.json` lo declara también en su campo `no_prior_objects_note`.

## Cantidades declaradas

| | Cantidad | Fuente |
| :--- | ---: | :--- |
| Administradores | 2 | `scripts/seeds/synthetic_data.sql` (sin cambios) |
| Profesores | 2 | `scripts/seeds/synthetic_data.sql` (sin cambios) |
| Estudiantes funcionales | 4 | `scripts/seeds/synthetic_data.sql` (sin cambios) |
| Estudiantes de carga | 200 | `scripts/seeds/capacity_data.sql`, deliberadamente **sin inscribir** (ver más abajo) |
| Cursos publicados | 2 | 1 funcional (`Arquitectura Cloud...`) + 1 técnico (`Perfiles de Video...`, publicado solo cuando corren los tres perfiles) |
| Cursos en borrador | 2 | Sin cambios respecto a la semilla funcional |
| Cursos despublicados | 1 | Sin cambios |
| Quizzes | 1 | Sin cambios; A5 ya lo cubre |
| Inscripciones activas | 2 | Sin cambios; ver nota sobre estudiantes de carga |
| Intentos de quiz pre-sembrados | 0 | Deliberado — ver más abajo |
| Perfiles de video reales | 3 (corto/medio/largo) | `cmd/seed-media`, este directorio |

### Por qué los estudiantes de carga no están inscritos ni tienen intentos

El recorrido que H2 mide incluye la inscripción y el envío de quiz como pasos
propios (catálogo → curso → inscripción → contenido → progreso → quiz — ver
[`NOTAS_TECNICAS.md`](../../NOTAS_TECNICAS.md) nota 6). Pre-sembrar esos
pasos le restaría al guion de carga exactamente lo que se supone debe medir:
la escritura real de una inscripción y de un envío de quiz bajo concurrencia.
Los 200 estudiantes de carga llegan **activos y listos para autenticar**, y
el resto del recorrido lo ejecuta el guion de H2/H3 en el momento de la
corrida, no esta semilla.

## Los tres perfiles multimedia

Los mismos que ya costó
[`CONFIGURACION_Y_COSTOS.md` §3](../../CONFIGURACION_Y_COSTOS.md#3-volumen-de-objetos-y-operaciones):
Corto (2 min), Medio (10 min), Largo (30 min), los tres a 1280×720 — la
resolución fuente que hace que la escalera declarada (360p+800/96kbps +
720p+2500/128kbps, `internal/transcode.DefaultLadder`) se aplique completa
sin hacer *upscaling* de nada.

`cmd/seed-media` no reimplementa el pipeline de transcodificación: construye
la misma tarea (`task.NewMediaProcessTask`) que una confirmación de carga
real encola, y se la entrega directamente al mismo
`handler.MediaProcessor` que corre el worker en producción
(`internal/worker/handler/mediaproc.go`). Los derivados que quedan en el
bucket son exactamente los que el worker real habría producido a partir del
mismo original — no una aproximación construida por un segundo camino de
código.

Los tres recursos son `is_mandatory = false` y viven en un curso propio
("Perfiles de Video para Pruebas de Capacidad", técnico, sin contenido
académico) en vez de agregarse al curso publicado funcional: así no alteran
el denominador de porcentaje de progreso que `docs/DATOS_SINTETICOS.md` ya
fija en 3 recursos obligatorios para `estudiante1`/`estudiante2` — ver nota 7
de `NOTAS_TECNICAS.md` sobre por qué ese denominador es frágil ante cualquier
cambio en la cantidad de recursos del curso.

### Reproducir

```bash
# 1. Datos funcionales + de carga (rápido, sin ffmpeg)
make seed
make seed-capacity

# 2. Multimedia real (lento: genera y transcodifica tres videos con ffmpeg
#    dentro del contenedor del worker; el perfil largo, de 30 min, domina
#    el tiempo total)
make seed-media
```

Es un solo comando por paso, tal como pide el criterio de aceptación. `make
seed-media` es re-ejecutable: si el curso multimedia ya estaba publicado de
una corrida anterior, lo vuelve a `draft` antes de reprocesar (misma técnica
que usa `synthetic_data.sql` para fijar el estado de un curso directamente),
y solo lo publica de nuevo al terminar los tres perfiles con éxito. Una
corrida parcial (`-profiles=corto`, útil para verificar el pipeline sin
esperar el perfil largo) deja el curso en `draft` a propósito, para no
publicar un curso que no contiene los tres perfiles que declara.

## Conciliación

`cmd/seed-media` no confía en su propio cálculo de cuántos segmentos debería
haber producido ffmpeg — el número real depende del punto de corte de
*keyframes* del codificador, no de una división limpia de la duración entre 6
segundos. En su lugar, **sigue los manifiestos como lo haría un reproductor
real**: lee `master.m3u8` para encontrar las dos listas de variante, lee cada
una para encontrar sus segmentos, y hace `StatObject` sobre cada archivo
referenciado. `domain.StorageProvider` no tiene una operación de listado (la
API tampoco la usa — ver
[`EJECUCION_PRUEBAS_MULTIMEDIA.md`](../../EJECUCION_PRUEBAS_MULTIMEDIA.md)),
así que seguir los manifiestos es la única forma de enumerar lo que un
perfil produjo de verdad.

El resultado de la conciliación —tamaño esperado contra tamaño real de cada
objeto, original y cada derivado— queda en `manifest.json`, en
`reconciliation_ok` y `reconciliation_discrepancies`. Una corrida limpia
reporta `reconciliation_ok: true` y una lista vacía.

## `manifest.json`

Generado por el comando, nunca editado a mano. Por perfil: duración,
resolución fuente, escalera usada, y para el original y cada derivado su
clave, tamaño en bytes y SHA-256 (el original solo — los derivados se
verifican por tamaño vía `StatObject`, no se descargan de vuelta para
hashear, ya que el objetivo es que el bucket contenga los bytes correctos, no
duplicar la subida). Totales de objetos y bytes al final.

## Nunca

Credenciales, llaves ni secretos. `manifest.json` no contiene ninguno: solo
claves de objeto, tamaños y hashes de contenido.
