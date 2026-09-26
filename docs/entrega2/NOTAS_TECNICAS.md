# Notas técnicas de la Entrega 2

Hallazgos que afectan a más de un issue. Cada uno indica dónde falla, para que
no se redescubra a mitad de una corrida.

Cubre lo aprendido construyendo el bloque A (deuda funcional de la Entrega 1).
Al cerrar cada issue conviene repasar si dejó algo aquí: la mitad de estas notas
salieron de errores que costaron una tarde y se habrían evitado leyéndolas.

| | Hallazgo | Afecta a |
| :--- | :--- | :--- |
| 1 | La URL firmada lleva el host dentro de la firma | C4, G2, H4, H5 |
| 1b | HLS multi-archivo no funciona detrás de URLs firmadas | C3, C4, H4, H5, I1 |
| 2 | MinIO retiró sus imágenes públicas | cualquier `docker compose up` |
| 3 | Las posiciones del seed estaban desfasadas en uno | G1, G2, G3 |
| 4 | El estado `available` del enunciado es `completed` en el código | A3, I1, I3 |
| 5 | Carga reanudable: brecha declarada | I1 |
| 6 | El contenido del curso dejó de ser público | G1, G2, G3, H2, H3 |
| 7 | La semilla tenía progreso sin inscripciones, y porcentajes que no cuadraban — **resuelto** | G1, G3, H2, H3 |
| 8 | El contrato de OpenAPI ya documenta lo que falta construir | A5, G2, I1 |
| 9 | `APP_BASE_URL` ahora también arma los enlaces de verificación | D2, D3, D4, F1, I1 |
| 10 | Las tablas del estudiante no tienen clave ajena al curso, a propósito | C1, C2, I1 |
| 11 | Los latidos no pasan por el limitador, y los pools no están acotados | B4, C1, D4, H1, H2, H3 |
| 12 | Dos trampas del entorno local | todos |

---

## 1. La URL firmada lleva el host dentro de la firma

**Afecta a:** C4 (apuntar API y workers al bucket), G2 (colecciones Postman), H4 y H5 (escenario 2).

Una URL prefirmada incluye el host en el cálculo de la firma. Cambiar el host
de la URL después de emitirla la invalida.

La consecuencia es que **`S3_ENDPOINT` tiene que ser la dirección que alcanza el
cliente, no la que usa la API internamente**. Es el mismo problema de clase que
`APP_BASE_URL`: un valor que solo sirve dentro de la red de contenedores rompe a
cualquier cliente externo.

### En local

`docker-compose.yml` inyecta `S3_ENDPOINT=http://minio:9000`. La API firma
correctamente, pero la URL apunta al nombre interno de la red:

```
http://minio:9000/mooc-storage/originals/<stable_id>/<uuid>.mp4?X-Amz-Signature=...
```

Desde el host eso no resuelve, y reescribir `minio` por `localhost` rompe la
firma. Para probar la carga directa en local hay dos caminos:

- Ejecutar el PUT desde dentro de la red, que es como se verificó A2:

  ```bash
  docker compose exec -T minio curl -X PUT \
    -H 'Content-Type: video/mp4' --upload-file /tmp/clase.mp4 "$UPLOAD_URL"
  ```

- O arrancar la API con `S3_ENDPOINT=http://localhost:9000`, que hace las URLs
  utilizables desde el host a cambio de que la API no alcance MinIO para
  `StatObject`. Sirve para inspeccionar una URL, no para el flujo completo.

### En la nube

No hay conflicto: el endpoint del bucket administrado es la misma dirección
para la API y para el cliente. Pero **hay que verificarlo explícitamente en C4**
en lugar de asumirlo, y el entorno de Postman de G2 debe apuntar al endpoint
público, no a un alias interno.

### Para el escenario 2

El plan de H4 separa el tráfico de control de la transferencia de archivos. El
generador de carga corre **fuera de las dos VMs**, así que consume las URLs
firmadas desde fuera de la VPC: si el endpoint quedó configurado con una
dirección privada, las transferencias fallarán con firma inválida y el error
parecerá un problema de capacidad cuando es de configuración.

---

## 1b. HLS multi-archivo no funciona detrás de URLs firmadas

**Afecta a:** C3 (IAM del bucket), C4, H4 y H5 (escenario 2), I1 (decisiones).

Corolario de la nota anterior, y se descubrió probando con un reproductor real.

Un manifiesto maestro referencia sus variantes por ruta **relativa**:

```
#EXT-X-STREAM-INF:BANDWIDTH=2628000,RESOLUTION=1280x720
720p.m3u8
```

El reproductor resuelve `720p.m3u8` contra la URL del maestro, pero **no hereda
la query string**, que es donde va la firma. Resultado con el maestro firmado:

```
[hls] Empty segment [http://.../360p.m3u8]
[hls] Empty segment [http://.../720p.m3u8]
```

El maestro se lee, las variantes no. Firmar cada segmento tampoco sirve: habría
que reescribir los manifiestos en cada lectura y cada URL vencería por separado.

### La decisión

**El prefijo `hls/` se sirve sin firma; `originals/` sigue privado.** Es lo que
el enunciado de la Entrega 2 permite de forma explícita —«acceso mediante URLs
firmadas **(o no firmadas)**»— y lo que hace cualquier entrega de HLS real. Solo
es público lo que el worker generó; lo que subió el autor no lo es.

Verificado en ambas direcciones:

```
ffprobe http://.../hls/<stable_id>/master.m3u8   → h264 640x360 + aac
                                                   h264 1280x720 + aac
wget    http://.../originals/<stable_id>/...      → 403 Forbidden
```

En local lo aplica el servicio `minio-policy` de `docker-compose.yml`. **En C3
hay que replicarlo en el bucket administrado**: lectura pública acotada al
prefijo `hls/`, nunca al bucket entero.

---

## 2. MinIO retiró sus imágenes públicas

**Afecta a:** cualquier uso de `docker compose up`.

`quay.io/minio/minio` responde `401 Unauthorized` en todos los tags y
`docker.io/minio/minio` devuelve `404`: el repositorio ya no existe. Fijar una
versión sobre ese origen no resuelve nada.

El Compose pasó a `bitnamilegacy/minio:2025.7.23-debian-12-r5`, que es el mismo
servidor MinIO empaquetado por Bitnami. Diferencias que obligaron a ajustar el
servicio:

| | MinIO oficial | Bitnami |
| :--- | :--- | :--- |
| Arranque | `command: server /data` | entrypoint propio, sin `command` |
| Usuario | root | uid 1001 |
| Datos | `/data` | `/bitnami/minio/data` |
| Buckets iniciales | servicio `minio-init` con `mc` | variable `MINIO_DEFAULT_BUCKETS` |

Como Bitnami crea los buckets al arrancar, **el servicio `minio-init` y
`scripts/init-minio.sh` se eliminaron**: solo creaban el bucket. Están en el
historial de git si hiciera falta recuperarlos.

Todas las imágenes del Compose quedaron con versión fija. `latest` es
exactamente lo que convirtió esto en una sorpresa en vez de una decisión.

---

## 3. Las posiciones del seed estaban desfasadas en uno

**Afecta a:** G1 (datos sintéticos), G2 (Postman), G3 (E2E), y cualquier prueba
de carga que cree contenido.

Los repositorios asignan `Position = COUNT(hermanos)`, o sea **base 0**, de
forma consistente en módulos, unidades y recursos. El seed los insertaba **base
1**.

El efecto: sobre datos sembrados, la primera creación de un recurso por la API
siempre colisiona.

```
pq: duplicate key value violates unique constraint "uq_resources_unit_position"
```

Se corrigió `scripts/seeds/synthetic_data.sql` desplazando las posiciones a base
0. No se tocó el código: es consistente consigo mismo y el paquete
`internal/domain/ordering` también razona con índices base 0.

---

## 4. El estado `available` del enunciado es `completed` en el código

**Afecta a:** A3 (worker de media), I1 e I3 (documentación).

El enunciado habla del estado `available`. El esquema de la Entrega 1 usa
`completed` con ese mismo significado, y `course.validateForPublish` ya lo lee
para decidir si un curso puede publicarse.

Se conservó `completed`. Renombrarlo obligaría a una migración y a tocar las
reglas de publicación de la Entrega 1 sin ganar nada. **Conviene declarar esta
equivalencia en el documento de arquitectura** para que un evaluador que busque
`available` sepa dónde mirar.

---

## 5. Carga reanudable: brecha declarada

**Afecta a:** I1 (documentación de decisiones).

El enunciado del proyecto pide carga «multipart directa y reanudable durante 24
horas». Lo implementado es una URL PUT firmada con vigencia de 24 horas, que es
directa pero **no reanudable**: reanudar requiere sesiones de carga reanudable
del proveedor, que es otro mecanismo.

La Entrega 1 no construyó nada de media, así que no hay reanudación que
preservar, y el enunciado de la Entrega 2 excusa explícitamente lo que no
estuviera implementado antes. Se deja como brecha declarada en lugar de
arriesgar el camino que se evalúa.

---

## 6. El contenido del curso dejó de ser público

**Afecta a:** G1 (datos sintéticos), G2 (Postman en cloud), G3 (E2E), H2 y H3
(escenario 1).

Hasta A4 los tres listados de contenido eran públicos. Ahora exigen sesión **y
una inscripción activa**:

```
GET /api/v1/courses/{courseID}/modules
GET /api/v1/modules/{moduleID}/units
GET /api/v1/units/{unitID}/resources
```

El catálogo (`GET /api/v1/courses`) sigue siendo público: navegar no es consumir.

Quien lee sin inscripción recibe **403**, y sin sesión **401**. El autor y el
administrador siguen leyendo el curso sin inscribirse, incluso en borrador —esa
era la regresión que más importaba no introducir.

La consecuencia práctica es para los guiones de carga: **un estudiante virtual
tiene que iniciar sesión e inscribirse antes de poder leer contenido o reportar
progreso.** Un guión que vaya directo al listado de módulos medirá 403 a toda
velocidad y el informe parecerá un problema de capacidad.

---

## 7. La semilla tenía progreso sin inscripciones, y porcentajes que no cuadraban

**Afecta a:** G1 (datos sintéticos), G3 (E2E), H2 y H3 (escenario 1).
**Estado: resuelto.** Queda aquí porque describe dos invariantes que un cambio
futuro de la semilla puede romper otra vez, y porque explica de dónde salen los
números que ahora siembra.

Eran dos problemas distintos en `scripts/seeds/synthetic_data.sql`, los dos
consecuencia de que A4 y A6 llegaron después de que se escribiera la semilla.

**Primero: había progreso de estudiantes que no estaban inscritos.** Recién
sembrada la base había 2 filas de `student_progress`, 1 insignia y **0
inscripciones**. `estudiante2` tenía el curso al 100%, aprobado y con insignia, en
un curso en el que nunca se inscribió. Desde A4 ese estado se contradice: ese mismo
estudiante no podía leer el contenido que supuestamente completó, ni reportar un
latido más. El catálogo `docs/DATOS_SINTETICOS.md` ya afirmaba que estaba inscrito;
la semilla nunca lo creó.

**Segundo: los porcentajes guardados no coincidían con los que la API calcula.**
A6 recalcula el porcentaje en cada lectura, contra los recursos obligatorios y
visibles que el curso tiene *ahora*. La semilla guardaba un número calculado contra
otro total:

```
student_progress.percent_completed guardado : 50.00
GET /api/v1/progress/courses/{id} reportaba : 33.33  (1 de 3)
```

La API tenía razón: el curso sembrado tiene 3 recursos obligatorios y el 50.00 se
calculó sobre 2.

### Lo que siembra ahora

| | `estudiante1` | `estudiante2` |
| :--- | :--- | :--- |
| Inscripción | `active` | `active` |
| Recursos completados | 1 de 3 | 3 de 3 |
| `percent_completed` | 33.33 | 100.00 |
| `is_approved` | `false` | `true` |
| Latidos en `progress_events` | 2 | 3 |
| Insignia | — | sí, verificable |

Los latidos se sembraron porque faltaban del todo: `progress_events` es el registro
de hechos y `student_progress` la proyección que se deriva de él, así que una
proyección sin los latidos que la produjeron contradice el diseño que la API
implementa.

La llave de imagen de la insignia pasó a `badges/{course_stable_id}/{student_id}.png`,
que es lo que `domain.BadgeImageKey` deriva. La anterior estaba escrita a mano con
otra forma —exactamente la deriva que esa función existe para evitar.

### Los dos invariantes, con prueba

`TestSyntheticDataSeed` en `internal/postgres/seed_test.go` los comprueba, y se
verificó que detecta ambas regresiones:

```
50.00 en lugar de 33.33  → student c0…01: seed stores 50.00% but the course's
                            resources give 33.33%
sin las inscripciones     → 2 seeded progress rows have no active enrollment
                            behind them
```

Si G1 amplía la semilla —más estudiantes, más cursos, más avance— esas dos
comprobaciones son las que avisan si el estado nuevo vuelve a contradecirse. El
segundo invariante es el que importa recordar al agregar un curso con más recursos:
cambiar el denominador invalida todos los porcentajes sembrados de ese curso.

---

## 8. El contrato de OpenAPI ya documenta lo que falta construir

**Afecta a:** A5 (quizzes), G2 (Postman en cloud), I1 (decisiones).

`api/openapi.yaml` se escribió en la Entrega 1 con un diseño más amplio que lo
implementado. De sus 37 rutas, **dos no tienen ruta en `internal/http/server.go`**:

| Ruta documentada | Estado |
| :--- | :--- |
| `POST /api/v1/quizzes/{quiz_id}/submissions` | la construye A5 |
| `POST /api/v1/users/professors` | brecha sin issue |

`/users/professors` aprovisionaba profesores. No está en ningún issue del bloque A
y **puede quedarse sin construir**: `PATCH /api/v1/admin/users/{userID}/role` ya
logra lo mismo, y la colección de administración lo cubre. Conviene decidirlo
explícitamente en I1 en lugar de dejar una ruta documentada que no responde.

**La lección de A6, que A5 va a encontrarse igual:** el spec no es un borrador. Al
implementar progreso e insignias el contrato ya estaba escrito, y en dos puntos
contradecía lo que se había codificado. Ganó el contrato:

- `/badges/verify/{verification_code}` estaba descrito como *«public
  privacy-preserving badge verification … without revealing student personal
  identity or email»*. La implementación devolvía el nombre del estudiante. Se
  quitó: hoy responde `{valid, course_title, issued_at, revoked}` y nada más.
- Los nombres `percentage` y `completed_resource_stable_ids` venían del spec y se
  adoptaron en lugar de inventar otros.

Solo se corrigió el spec donde estaba realmente mal: el latido pedía `course_id` +
`resource_stable_id`, con lo que un cliente podía reclamar progreso en un curso que
nunca abrió, y un `stable_id` no identifica *qué versión*.

Para A5 hay un detalle más: el spec documenta **solo el envío** del quiz, no la
autoría (crear preguntas y opciones). Esos endpoints habrá que añadirlos al
contrato, no reconciliarlos. Y `is_approved` **todavía no considera quizzes**:
hoy es «todos los recursos obligatorios completados». Cuando A5 aterrice, la
aprobación gana un segundo término, y eso toca `resourceCounts.approved()` en
`internal/postgres/progress_repository.go`.

---

## 9. `APP_BASE_URL` ahora también arma los enlaces de verificación

**Afecta a:** D2 y D3 (proxy inverso y HTTPS), D4, F1 (SMTP), I1.

Hasta A6, `APP_BASE_URL` solo aparecía en los correos de activación y de
recuperación de contraseña. Ahora también construye el `verification_url` que
lleva cada insignia:

```
{APP_BASE_URL}/api/v1/badges/verify/{verification_code}
```

Es el mismo problema de clase que la nota 1: **un valor que solo sirve dentro de
la red de contenedores rompe a cualquier cliente externo**, y aquí rompe en
silencio —la insignia se emite igual, el enlace simplemente no abre.

Cuando D3 ponga HTTPS delante, `APP_BASE_URL` tiene que pasar a `https://` y al
nombre público. Si queda vacío, `BadgeVerificationURL` devuelve cadena vacía a
propósito: un enlace que apunta a un host que nadie configuró es peor que ninguno,
y el código por sí solo verifica.

---

## 10. Las tablas del estudiante no tienen clave ajena al curso, a propósito

**Afecta a:** C1 (Cloud SQL y migraciones), C2 (respaldo y restauración), I1.

`enrollments`, `student_progress`, `badges` y `progress_events` se llavean por
`(student_id, course_stable_id)` y **no tienen `REFERENCES` al curso**. No es un
olvido: `courses` declara `UNIQUE (stable_id, version)`, así que `stable_id` por sí
solo **no es único** y no puede ser destino de una clave ajena.

Es justamente lo que permite que el progreso sobreviva a la publicación de una
versión nueva, que es lo que pide la sección 5.1 del enunciado.

Dos consecuencias:

- Un revisor que audite el esquema va a ver cuatro tablas sin FK al curso y
  preguntará. **Conviene declararlo en I1** antes de que lo pregunten.
- Borrar un curso no arrastra el progreso de sus estudiantes. En el ensayo de
  eliminación y recreación controlada de **I6** eso se va a notar: las filas
  quedan huérfanas, apuntando a un `course_stable_id` que ya no existe, y **la API
  no las muestra** —`GET /api/v1/progress/courses/{course_id}` resuelve el curso
  por su id de fila y responde 404—, así que la limpieza hay que hacerla en SQL.

  Recrear el curso con el mismo `stable_id` las resucita, lo cual puede ser
  exactamente lo que se quiere en ese ensayo, pero conviene que sea una decisión y
  no una sorpresa.

---

## 11. Los latidos no pasan por el limitador, y los pools no están acotados

**Afecta a:** B4 (configuración), C1 (Cloud SQL), D4 (seguridad tras el proxy),
H1, H2 y H3 (escenario 1).

**El limitador de tasa cubre tres rutas, no todas.** Se aplica por ruta a login,
registro y recuperación; la cadena global (`RequestID`, `Tracing`,
`RequestLogger`, `Recoverer`, `CSRF`) no incluye limitación:

```go
limitLogin    := middleware.RateLimit(..., "login", ...)
limitRegister := middleware.RateLimit(..., "register", ...)
limitRecovery := middleware.RateLimit(..., "recovery", ...)
```

Para el escenario 1 eso es conveniente: los latidos no se van a estrangular y lo
que se mida será la plataforma. Pero también significa que **nada acota el volumen
de latidos** salvo el tope de 3600 s de permanencia por reporte. Si D4 agrega
limitación en el proxy, el camino del latido hay que exceptuarlo o dimensionarlo a
conciencia, o el informe de capacidad medirá el proxy en lugar de la API.

El latido tampoco lleva `Idempotency-Key`, y es deliberado: la proyección se
recalcula en vez de incrementarse, así que no hay doble aplicación de la que
protegerse, y llenar el almacén de idempotencia con miles de latidos por minuto
sería gratis solo en apariencia.

**Los pools de conexión.** La API abre hasta `DB_MAX_OPEN_CONNS` (25 por defecto).
Postgres en local permite 100. Las bases de prueba de `internal/postgres` **no
acotan su pool** —el de Go es ilimitado—, y eso ya rompió una prueba ajena cuando
A6 añadió una que abría 8 transacciones a la vez: el síntoma fue un fallo
intermitente en un test sin relación, no un error de conexión. Se acotó ese test a
4.

Para **C1 esto importa de verdad**: las instancias pequeñas de Cloud SQL permiten
bastante menos de 100 conexiones, y con dos VMs (API y workers) más el pool de
asynq hay que sumar antes de dimensionar, no después del primer `too many
connections` bajo carga.

---

## 12. Dos trampas del entorno local

**Afecta a:** todos.

**`go test ./...` borra la base de desarrollo.** El paquete `migrations` recrea
todas las tablas del esquema por defecto, así que después de correr la suite
completa la base queda vacía. El síntoma no se parece a la causa: las colecciones
de Postman empiezan a responder **401 en todos los logins**, como si las
credenciales estuvieran mal.

```bash
bash ./scripts/seed.sh --reset   # y listo
```

Los tests de repositorio sí se aíslan —cada uno crea su propio esquema—; es el
paquete de migraciones el que trabaja sobre el esquema por defecto.

**El `make` de GnuWin32 no resuelve rutas con espacios.** En una ruta como
`D:\...\Cloud architecture\...`, los objetivos que invocan `./scripts/*.sh`
fallan con `bash: D:\Fredy\Uniandes\Materias\Cloud: No such file or directory`.
Afecta a `make lint`, a los cuatro objetivos `seed*` y a `make check`, que
depende de `lint`. Invócalos directamente:

```bash
bash ./scripts/lint.sh
bash ./scripts/seed.sh --reset
```

Los objetivos de newman sí funcionan porque ejecutan `docker run` sin pasar por un
script —eso sí, todos llevan `MSYS_NO_PATHCONV=1`, sin el cual Git Bash reescribe
`/etc/newman` a una ruta de Windows y newman falla con `ENOENT`.
