# Correcciones a la revisión de la Entrega 1

Respuesta a los seis hallazgos de [`correciones_entrega1.txt`](./correciones_entrega1.txt),
el archivo que entregó el revisor. Se atienden en una rama propia,
`fix/entrega1-review-findings`, **antes de empezar los issues de la Entrega 3**
y sin tocar la ruta que miden los escenarios de carga.

| | Hallazgo | Estado |
| :--- | :--- | :--- |
| 1 | Sin respaldo, restauración ni medición de RPO/RTO | ⏳ RTO ya cumplido · falta PITR |
| 2 | `TaskMalwareScan` sin nada que lo ejecute | ✅ corregido |
| 3 | Sin autosave; las versiones publicadas no son inmutables | ⏸ aplazado · va a H3 |
| 4 | Sin CDN; carga no reanudable y sin verificación de integridad | ⏸ aplazado · ver §4 |
| 5 | Quizzes sin snapshot, guardado parcial ni expiración | ⏳ pendiente en esta rama |
| 6 | Sin latencia documentada; PDF inaccesible | ✅ la parte de PDF · el resto lo cerró la Entrega 2 |

---

## Lo que cambia el orden de prioridades

**El hallazgo 2 no era una funcionalidad que faltara: era un defecto vivo que
dejaba inservible una parte del alcance mínimo.**

La ruta completa, tal y como estaba:

1. `internal/media/service.go:366` — `taskTypeForResource()` devuelve
   `TaskMalwareScan` para PDF, presentación, descargable e imagen.
2. `service.go:267` — se encola con ese tipo, que viaja como **campo del
   payload**, no como tipo de tarea de asynq.
3. `internal/worker/handler/mediaproc.go` — `process()` **nunca leía
   `payload.TaskType`**. Descargaba el objeto y llamaba directo a `Probe()` y al
   transcodificador HLS.
4. `internal/transcode/ffmpeg.go:161` — `ffprobe` sobre un PDF devuelve *«file
   carries no video or audio stream»*, envuelto en `ErrUnprocessable`.
5. `mediaproc.go` — `ErrUnprocessable` es permanente, así que `markFailed()`.

**Un PDF subido por el flujo normal terminaba en `failed` y nunca llegaba al
estudiante.** Eso es exactamente lo que el revisor describe en el hallazgo 6
(«no es posible ver o acceder a archivos PDF»): los dos apuntan al mismo
defecto.

Sobrevivió porque **nada lo ejercitaba**. Ni `scripts/e2e_cloud/main.go`, ni
`collection_media.postman_collection.json`, ni `cmd/seed-media/main.go` subían
un documento o una imagen: toda la cobertura era mp4.

---

## 1 · Enrutado de las tareas que no son audiovisuales — ✅

`internal/worker/handler/mediaproc.go` separa la canalización en dos tras el
bloque de idempotencia, que sigue siendo común:

| Rama | Qué hace |
| :--- | :--- |
| `processAudiovisual()` | lo de siempre, sin un cambio: descarga, `Probe`, escalera HLS, subida de derivados, `completed` |
| `processDocument()` | `StatObject`, comprobar tamaño y que el tipo almacenado case con la extensión, y marcar `completed`. Sin descarga, sin `ffprobe`, sin directorio de trabajo |
| tipo desconocido | `ErrUnprocessable` + `SkipRetry`, para que falle explícito en vez de transcodificar lo que no debe |

**Compatibilidad hacia atrás, que es el riesgo real del cambio:** un trabajo
encolado antes de esto no lleva `task_type`. Todos aquellos eran audiovisuales,
y un mensaje que ya está en la cola no puede cambiar de significado porque se
redesplegara el worker. El `switch` trata la cadena vacía como
`hls_transcode`, y hay una prueba que lo fija.

Un documento no necesita derivados: lo que subió el autor es lo que descarga el
estudiante. Por eso la rama no descarga el objeto, sólo confirma que está y que
es lo que dice ser.

### Lo que esta rama *no* verifica, a propósito

El tipo de contenido que se compara es el que registró el bucket, que es el que
**declaró el cliente** al hacer el `PUT`. Leer los bytes iniciales para
identificar el formato de verdad, y contrastar un checksum contra un resumen
que el cliente se comprometiera a enviar antes, son dos huecos reales. Los dos
pertenecen al trabajo de carga reanudable, que cambia la ruta de subida y por
tanto tiene que esperar. Ver §4.

### Pruebas

Siete casos nuevos en `internal/worker/handler/mediaproc_test.go`:

| | |
| :--- | :--- |
| `DejaUnDocumentoDisponible` | un PDF llega a `completed` |
| `NoTranscodificaUnDocumento` | no se invoca al transcodificador ni se suben derivados |
| `AceptaElContentTypeConParametros` | `application/pdf; charset=binary` sigue casando |
| `RechazaUnDocumentoConTipoQueNoCuadra` | guardado como `image/png` bajo clave `.pdf` → fallo permanente |
| `TrataUnDocumentoAusenteComoPermanente` | objeto que no está en el bucket |
| `RechazaUnTipoDeTareaDesconocido` | un tipo que nadie implementa falla, no se transcodifica |
| `TrataLaTareaSinTipoComoAudiovisual` | la compatibilidad hacia atrás |

---

## 2 · Cobertura de documento e imagen — ✅

Sin esto el arreglo se vuelve a pudrir sin que nadie lo note.

**Fixtures nuevos**, pequeños y versionados —a diferencia de `clase.mp4`, que
está en `.gitignore` por tamaño y se genera con ffmpeg:

| | |
| :--- | :--- |
| `docs/media/guia.pdf` | 613 B · PDF 1.7 de una página |
| `docs/media/diagrama.png` | 463 B · PNG 16×16 |

**`scripts/e2e_cloud/main.go`** gana el tramo *«4b. Documentos e imágenes sin
transcodificación»*, que para el PDF y para la imagen recorre: crear el recurso,
firmar la carga comprobando que la clave cae bajo `documents/` o `thumbnails/` y
no bajo `originals/`, subir directo al bucket, confirmar, **esperar a que el
estado llegue a `completed`** —el paso que antes habría acabado en `failed`
pasara el tiempo que pasara—, leer con URL firmada contrastando el tamaño, y
comprobar que sin firma el objeto responde 403.

Dos flags nuevos, `-documento` y `-imagen`, con la misma forma que `-video`.

**`docs/postman/collection_media.postman_collection.json`** suma cuatro
peticiones, de la 26 a la 29: crear el recurso PDF, firmar su carga verificando
el prefijo y el `content_type`, y dos rechazos —mime que no corresponde, y un
estudiante pidiendo la URL—. Van al final, así que no renumeran nada.

> **Lo que la colección de Postman no puede cubrir:** ninguna de sus peticiones
> espera a que el worker termine. Estructuralmente no podía haber detectado este
> defecto, y sigue sin poder. El guardián de la regresión es el tramo del E2E.

### Lo que falta verificar

**Estos pasos no se han podido ejecutar.** El entorno de la nube está apagado
desde el cierre de la Entrega 2 —la instancia de base de datos eliminada y las
dos VM detenidas—, así que el tramo nuevo compila y pasa `vet`, pero **no se ha
corrido contra un despliegue real**. La primera oportunidad es cuando exista
entorno otra vez.

La corrección del worker sí queda verificada: los siete casos nuevos corren en
`go test` sin necesidad de nube.

**La insignia de E2E del README sigue diciendo 61/61 y enlazando a
`docs/entrega2/evidencias/G3/`.** Es correcto así: ese es el registro de lo que
se calificó en la Entrega 2 y no se toca. La corrida con el tramo nuevo
escribirá en `docs/entrega3/evidencias/correcciones-entrega1/`, que es el valor
por omisión nuevo de `-salida`, y entonces se actualiza la insignia.

---

## 3 · PITR y la sección RPO/RTO — ⏳ pendiente en esta rama

El RTO **ya está cumplido y medido** por el issue C2 de la Entrega 2:
aprovisionar 204 s + restaurar 324 s ≈ **9 minutos frente a un techo de 4
horas**, con la restauración probada fila por fila sobre una instancia nueva.

Lo que falta es el RPO. En `infra/terraform/database.tf:178`,
`point_in_time_recovery_enabled = false` más una copia diaria a las 03:00 UTC
dan un **RPO de hasta 24 horas**, frente al ≤ 15 min que exige la
especificación.

---

## 4 · Lo que se aplaza, y por qué

**Versiones inmutables reales (hallazgo 3).** `internal/course/service.go:72`
fija `Version: 1` al crear; `Publish` sólo cambia el estado y `Unpublish` edita
la misma fila. El propio código lo reconoce en la línea 158. Hacerlo de verdad
toca a la vez cursos, módulos, unidades, recursos, progreso e inscripciones:
rompe el E2E, las colecciones de Postman y la semántica que miden los escenarios
de carga. Va al inventario de deuda técnica de H3 con esta justificación.

**Autosave (hallazgo 3).** Es independiente y barato, pero necesita el editor
del frontend, que se construye en D3 (#213). Tiene más sentido allí que aquí.

**Carga reanudable y verificación de integridad (hallazgo 4).**
`internal/storage/gcs.go:78` firma un `PUT` plano: lo de las 24 horas es el TTL
de la URL, no una sesión reanudable de GCS. Arreglarlo cambia la ruta de subida,
que es justo lo que mide el escenario 2. Tiene que ir **después de G4** o
declararse como diferencia en la comparación.

**CDN (hallazgo 4).** El enunciado de la Entrega 3 lo excluye de forma expresa:
*«No se exige alta disponibilidad multizonal, failover de base de datos, CDN,
migración a Kubernetes…»*. Y G4 pide sustentar el cuello de botella *«con el
efecto esperado de una CDN o más capacidad»*, es decir como análisis. Montarlo
antes de G4 invalidaría la comparación contra la línea base, que es el error que
ya costó un tercio del criterio del escenario 2. **No se registra ni como deuda.**

---

## La regla de secuencia

> Lo que toque la ruta de carga o la de quizzes tiene que aterrizar **antes de
> A7 (#199) o después de G4 (#224)**, nunca en medio. A7 mide la línea base
> retrospectiva desde el tag `entrega-2`, así que es inmune; pero G3 y G4 corren
> sobre el código de la Entrega 3, y cualquier cambio entre medias hay que
> declararlo como diferencia.

Por eso esta rama va antes de empezar los issues de la Entrega 3.

---

Este archivo no contiene credenciales, llaves ni secretos.
