# Notas técnicas de la Entrega 2

Hallazgos que afectan a más de un issue. Cada uno indica dónde falla, para que
no se redescubra a mitad de una corrida.

Cubre lo aprendido construyendo el bloque A (deuda funcional de la Entrega 1).
Al cerrar cada issue conviene repasar si dejó algo aquí: la mitad de estas notas
salieron de errores que costaron una tarde y se habrían evitado leyéndolas.

| | Hallazgo | Afecta a |
| :--- | :--- | :--- |
| 1 | La URL firmada lleva el host dentro de la firma | C4, G2, H4, H5 |
| 1b | HLS multi-archivo no funciona detrás de URLs firmadas — *y en la nube el prefijo público no es aplicable, **#167*** | C3, C4, G3, H4, H5, I1 |
| 2 | MinIO retiró sus imágenes públicas | cualquier `docker compose up` |
| 3 | Las posiciones del seed estaban desfasadas en uno | G1, G2, G3 |
| 4 | El estado `available` del enunciado es `completed` en el código | A3, I1, I3 |
| 5 | Carga reanudable: brecha declarada | I1 |
| 6 | El contenido del curso dejó de ser público | G1, G2, G3, H2, H3 |
| 7 | La semilla tenía progreso sin inscripciones, y porcentajes que no cuadraban — **resuelto** | G1, G3, H2, H3 |
| 8 | El contrato de OpenAPI ya documenta lo que falta construir | A5, G2, I1 |
| 9 | `APP_BASE_URL` ahora también arma los enlaces de verificación | D2, D3, G4, F1, I1 |
| 10 | Las tablas del estudiante no tienen clave ajena al curso, a propósito | C1, C2, I1 |
| 11 | Los latidos no pasan por el limitador, y los pools no están acotados — *el presupuesto de conexiones, **resuelto** en C1* | B4, C1, G4, H1, H2, H3 |
| 12 | Tres trampas del entorno local | todos |
| 13 | Reproducir videos completos en las pruebas cuesta más que las dos VMs | B1, C3, H2, H4, H5 |
| 14 | Tres cosas que GCP decidió por nosotros al habilitar las APIs | B3, D2, G4, E1, F1 |
| 15 | Cloud SQL no tiene el hook que aplica el esquema en local | C1, C2, D2, E1, I6 |
| 16 | Un `apply` desde una rama borra el trabajo de otro, y no falla al hacerlo | **todos los de infra**: B3, C1, C3, D2, E1, F1, G4, I6 |
| 17 | SMTP 587, 465 y 2525 no hablan exactamente igual | F1, G3, I1 |
| 18 | Terraform y el arranque deben interpretar igual los secretos creados desde Windows | C1, D2, E1, F1 |
| 19 | Lo versionado tiene que reproducir lo que hacía lo que reemplaza — *`REDIS_URL`, **#166*** | D2, E1, F1, G3 |
| 20 | El worker no podía arrancar en producción: validaba la configuración de la API | E1, F1, G3, I1 |

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

En local lo aplica el servicio `minio-policy` de `docker-compose.yml`.

### En la nube esa decisión no se puede aplicar tal como está escrita

Esta nota decía que C3 solo tenía que replicar la política en el bucket
administrado. **No es posible**, y G3 lo descubrió midiéndolo: `hls/` responde
`403` de forma anónima, y la condición IAM que parecería resolverlo no existe
como opción. **IAM no admite condiciones en enlaces concedidos a `allUsers` ni a
`allAuthenticatedUsers`**, así que con acceso uniforme a nivel de bucket o es
público el bucket entero —que es justo lo que esta decisión descarta— o no hay
prefijo público.

Lo que separa «no está» de «no puedo leer» es pedir un objeto que no existe:

```
GET /hls/no-existe-a-proposito/master.m3u8   → 403   (404 si el prefijo fuera público)
GET /originals/no-existe/x.mp4               → 403   (correcto, es privado)
```

Queda abierto en **#167** con las tres salidas reales evaluadas; la de un bucket
aparte para los derivados es la única que no degrada la postura de seguridad.

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

**A5 ya aterrizó**, y confirmó el patrón: el spec documentaba solo el envío del
quiz, así que los tres endpoints de autoría y lectura hubo que **añadirlos** al
contrato, no reconciliarlos. El que sí estaba documentado se corrigió en dos
puntos —le faltaban `security`, el 403 de inscripción y el 409 de intentos
agotados, y declaraba `score` como decimal cuando la columna es entera.

Sobre la aprobación del curso, la solución fue más simple de lo previsto.
`resourceCounts.approved()` **no cambió**: sigue siendo «todos los recursos
obligatorios completados». Lo que ocurre es que **aprobar un quiz completa su
recurso**, y lo registra el servidor al calificar en lugar de esperar un latido
del cliente. Así los quizzes entran en la aprobación del curso por la puerta que
ya existía, sin un segundo término que mantener en dos sitios.

---

## 9. `APP_BASE_URL` ahora también arma los enlaces de verificación

**Afecta a:** D2 y D3 (proxy inverso y HTTPS), G4, F1 (SMTP), I1.

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

**Afecta a:** B4 (configuración), C1 (Cloud SQL), G4 (verificacion de red y seguridad),
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
de latidos** salvo el tope de 3600 s de permanencia por reporte. Si G4 exige
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

Para **C1 esto importaba de verdad**: con dos VMs (API y workers) sumando contra
el mismo tope, había que sumar antes de dimensionar y no después del primer `too
many connections` bajo carga.

**Resuelto en C1, y de una forma que conviene conocer.** En lugar de heredar el
`max_connections` que Google asigna según la memoria del perfil —que cambiaría
solo si alguien cambia el perfil—, `infra/terraform/database.tf` lo **declara**:

| | Conexiones |
| :--- | ---: |
| API (VM de D2) | 25 |
| Worker (VM de E1) | 25 |
| Reserva de superusuario | 3 |
| Agentes de Google | ~5 |
| **Comprometido** | **~58** de 100 |

Los ~42 restantes son el margen para las migraciones, un `psql` de diagnóstico y
el solapamiento de un reinicio.

**Lo que hay que recordar al tocar el pool:** `DB_MAX_OPEN_CONNS` es *por
proceso*, no del sistema. Subirlo en la VM de la API no avisa de que la del
worker también está consumiendo. El límite efectivo se consulta con
`bash ./scripts/migrate.sh --verify`.

Y una comprobación nueva que se aplica en todo entorno: pedir más conexiones
inactivas que abiertas ya no pasa desapercibido. `database/sql` recortaba el
valor en silencio, así que era una configuración que parecía aplicada y no lo
estaba; ahora el arranque la rechaza.

---

## 12. Tres trampas del entorno local

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

**Git Bash puede dejar un `CR` al leer secretos de `gcloud`.** En Windows,
`gcloud` termina la salida con CRLF. La sustitución `$(...)` elimina el LF,
pero puede conservar el CR, de modo que Terraform recibe una contraseña un
carácter más larga que Secret Manager. El plan entonces propone modificar
`google_sql_user.app` aunque nadie haya rotado la credencial.

La instrucción compartida elimina ambos caracteres con `tr -d '\r\n'`, y
`database.tf` aplica `trimspace` como segunda barrera. Nunca se corrige
copiando la contraseña a mano.

---

## 13. Reproducir videos completos en las pruebas cuesta más que las dos VMs

**Afecta a:** B1 (estimación), C3 (IAM del bucket), H2 y H4 (planes de carga),
H5 (ejecución del escenario 2).

Salió al estimar costos en B1, y es el único renglón de la estimación que puede
descarrilar el crédito.

El prefijo `hls/` se sirve **sin firma y con lectura pública** (nota 1b), porque
un reproductor no hereda la *query string* al resolver las variantes del
manifiesto. La consecuencia de costo es directa: **cada reproducción completa es
transferencia de salida a internet**, y el generador de carga corre fuera de la
nube, así que todo lo que descargue sale por el enlace facturado.

Las cuentas, con la escalera declarada —360p a 800+96 kbps y 720p a
2500+128 kbps, segmentos de 6 s—:

```
reproducción íntegra en 720p de un video de 10 min  ≈ 197 MB
mil reproducciones íntegras                         ≈ 192 GB de egreso
```

A cualquier precio plausible de transferencia, esos 192 GB cuestan bastante más
que las dos máquinas virtuales juntas operando 24×7 (24,46 USD/mes con
`e2-small`). Y con 50 USD de cupón por integrante, no es un detalle contable: es
la diferencia entre poder repetir los niveles de carga o no.

### Lo que eso obliga en los planes de carga

* **El escenario 1 no necesita descargar video.** Es actividad académica:
  catálogo, inscripción, lectura de contenido, progreso y quizzes. Pedir el
  manifiesto es parte del recorrido; descargar todos los segmentos no lo es.
* **El escenario 2 mide subida y procesamiento**, no reproducción. Lo que se
  observa es el *throughput* del worker —videos por minuto, según la aclaración
  del docente— y la profundidad de la cola, no cuántos MB bajó el cliente.
* Si un recorrido tiene que probar reproducción, basta con el manifiesto y los
  **primeros segmentos**, y hay que declararlo en el plan. Descargar videos
  enteros a escala convierte una prueba de capacidad en una factura de
  transferencia, y además mide el enlace doméstico del generador en lugar de la
  plataforma.

Conviene medir el egreso acumulado durante las corridas y contrastarlo con lo
estimado, que es justo lo que el enunciado pide al exigir que los costos se
contrasten con el consumo observado.

---

## 14. Tres cosas que GCP decidió por nosotros al habilitar las APIs

**Afecta a:** B3 (VPC y firewall), D2 y E1 (las VMs), G4 (verificación de
seguridad), F1 (SMTP).

Aparecieron al mirar la consola después del primer `terraform apply` de B2.
Ninguna la creó nuestro Terraform: son comportamientos del proveedor que
conviene conocer antes de tropezar con ellos.

### La red `default` se crea sola, y viene abierta

Habilitar `compute.googleapis.com` hace que GCP cree una red llamada `default`
en modo automático, con **una subred en cada región del mundo** —42 en el
proyecto— y un juego de reglas de firewall por defecto. Entre esas reglas hay
una que **permite SSH desde `0.0.0.0/0`**.

Es decir: al habilitar la API de cómputo, el proyecto quedó con una red global y
una puerta de SSH abierta a internet que nadie decidió abrir. Todavía no hay
ninguna VM detrás, así que no hay nada expuesto, pero la primera instancia que
se cree en esa red lo estaría.

**Lo que toca hacer en B3:** crear la VPC propia con solo la subred que la
entrega necesita, y **eliminar la red `default` con sus reglas**. Si se deja,
G4 —la verificación de red y seguridad del despliegue— va a encontrar
exactamente eso, y con razón.

Conviene además que el Terraform de B3 sea explícito al respecto en lugar de
confiar en que alguien se acuerde de borrarla a mano: lo que no está en el
código vuelve a aparecer en el siguiente `apply` sobre un proyecto limpio.

### El puerto 25 saliente está bloqueado y no se puede abrir

La consola lo anuncia sin que se lo pidan: *«SMTP port 25 disallowed in this
project»*. Google bloquea el tráfico saliente por el puerto 25 en Compute
Engine y **no ofrece forma de desbloquearlo**; es una medida antispam de la
plataforma, no una regla de firewall que podamos cambiar.

**Lo que significa para F1** (sustituir Mailpit por SMTP en la nube): el
proveedor que se elija tiene que hablar por **587 (STARTTLS) o 465 (TLS
implícito)**, nunca por 25. La mayoría los ofrece, pero hay que configurarlo
explícitamente.

El modo en que esto falla es especialmente molesto: la conexión no se rechaza,
se queda esperando hasta agotar el tiempo. Si los correos de verificación dejan
de llegar sin un error claro en los registros, esta es la primera sospecha.

### La cuenta de servicio por defecto de Compute Engine tiene rol de Editor

Junto con la API de cómputo aparece una tercera cuenta de servicio que nadie
creó, `PROJECT_NUMBER-compute@developer.gserviceaccount.com`, y en la política
de IAM del proyecto figura con **rol `Editor`**.

Importa porque **es la que GCP adjunta a cualquier VM que se cree sin
especificar otra**. Una instancia creada sin cuenta explícita —desde la consola,
desde `gcloud`, o desde un Terraform que se olvide del campo— queda con permiso
de Editor sobre todo el proyecto. Eso anula de un plumazo la separación entre la
cuenta de la API y la del worker que B2 construyó: daría igual quién firma y
quién escribe si la VM puede hacer cualquier cosa.

**Lo que toca en D2 y E1:** adjuntar explícitamente `sa-web-server` y
`sa-worker-server` a sus respectivas instancias, y no dar por bueno el valor por
defecto. Y en **G4**, comprobarlo: mirar qué cuenta lleva adjunta cada VM, no
solo qué cuentas existen.

Si al final ninguna VM la usa, lo más limpio es **quitarle el rol de Editor** a
esa cuenta por defecto. No se puede borrar mientras la API esté habilitada, pero
sí dejarla sin permisos.

---

## 15. Cloud SQL no tiene el hook que aplica el esquema en local

**Afecta a:** C1 (migrar la instancia), C2 (respaldos y restauración), D2 y E1
(arranque de las VMs), I6 (recrear el entorno).

En local, el esquema lo crea el hook de inicialización de Postgres:
`scripts/init-db.sh` montado en `/docker-entrypoint-initdb.d`, que recorre
`migrations/*.up.sql` **una sola vez**, cuando el volumen está vacío.

**Cloud SQL no tiene ese hook.** La instancia nace con `moocdb` creada y sin una
sola tabla, y nada la migra. El síntoma es el de siempre en este proyecto: la
aplicación arranca —la conexión funciona— y el primer registro devuelve 500.

Por eso C1 añadió `scripts/migrate.sh`, con una tabla `schema_migrations` que lo
hace repetible. Tres cosas que conviene saber antes de usarlo:

**1. Hay que ejecutarlo desde dentro de la VPC.** La instancia no tiene IP
pública, así que desde un portátil no hay a dónde conectarse. Desde una VM, con
`postgresql-client` instalado.

**2. Contra tu base de desarrollo, la primera vez es `--baseline`.** Ahí el
esquema ya existe pero sin tabla de control, así que el script intentaría aplicar
`000001` y fallaría con «relation already exists». El baseline registra lo
aplicado sin ejecutarlo.

**3. Recrear la instancia da una base vacía.** Terraform crea la base y el
usuario, nunca el esquema. Después de cualquier `destroy`/`apply` —y en **I6** eso
es obligatorio— hay que volver a migrar. No es un paso opcional del despliegue.

### Dos trampas de la propia instancia, ya previstas en el código

**El nombre queda reservado una semana.** Google retiene el nombre de una
instancia borrada siete días, y el `apply` siguiente falla con un conflicto que
no se puede forzar. La salida es subir `db_instance_generation` en `database.tf`.

**`terraform destroy` falla a propósito.** La instancia lleva
`deletion_protection = true`, porque tres personas aplican sobre el mismo estado
y perder la base no se deshace. Para I6 hay que ponerlo en `false`, aplicar, y
solo entonces destruir. El procedimiento completo está en
[`infra/terraform/ADMINISTRACION.md`](../../infra/terraform/ADMINISTRACION.md).

### La buena noticia: las migraciones ya eran portables

Se revisaron contra las restricciones de Cloud SQL y no hubo que tocar ninguna:
**cero `CREATE EXTENSION`**, nada no transaccional (ningún `CONCURRENTLY`, ningún
`VACUUM`), **ningún rol**. El único `BEGIN` del árbol es el cuerpo de una función
PL/pgSQL, no control de transacción.

Que no haga falta ninguna extensión pese a usar `gen_random_uuid()` en 22
columnas es porque esa función es núcleo desde PostgreSQL 13. **Es la razón por
la que `database_version` está fijado a `POSTGRES_16`**: sobre una versión
anterior, estas migraciones fallarían todas en la primera línea de la 000001 sin
`pgcrypto`.

---

## 16. Un `apply` desde una rama borra el trabajo de otro, y no falla al hacerlo

**Afecta a:** todos los issues de infraestructura — B3, C1, C3, D2, E1, F1, G4, I6.

Esta no salió de leer documentación: ocurrió mientras se construía C1, y la
primera señal fue una línea del `plan` que había que mirar dos veces.

```
Plan: 5 to add, 0 to change, 9 to destroy.
```

Los 5 eran C1. **Los 9 eran el bucket de C3, su IAM y sus prefijos**, y estaban
ahí porque el estado es compartido mientras el código no lo es: la rama de C1 no
contenía el `storage.tf` de C3, así que para Terraform esos nueve recursos
sobraban.

Lo que hace esto peligroso es que **el `apply` no habría fallado**. Habría
funcionado, habría borrado el trabajo de otra persona y habría terminado en verde.
No hay ninguna salvaguarda técnica: la única barrera es leer la última línea del
plan.

### La segunda mitad, que es peor

Al investigar apareció algo más: ese `storage.tf` **no estaba en `main` ni en
ninguna rama remota**. La rama de C3 tenía cero commits. Los recursos se habían
aplicado desde un archivo que existía solo en el portátil de quien lo hizo.

Eso deja la infraestructura real **por delante del repositorio**, y el problema
pasa a ser de todos:

* Cualquiera que aplique, desde `main` incluido, propone destruir esos recursos.
* Nadie más puede declararlos, porque no tiene el archivo.
* Si esa copia local se pierde, quedan huérfanos en el estado y hay que
  reconstruirlos a mano o importarlos.

### Qué hacer

**Al trabajar:** `plan` desde tu rama todo lo que quieras —valida tu
configuración y es lo que confirma qué vas a crear—, pero el `apply` **solo desde
`main` actualizado y después de mezclar**. Está como sexta regla en
[`infra/terraform/README.md`](../../infra/terraform/README.md).

**Al revisar un plan:** la última línea, siempre. Un `to destroy` que no
esperabas no es ruido; es que tu código y el estado no cuentan la misma historia.

**Si ya aplicaste desde una rama:** sube el archivo cuanto antes y mézclalo. No
hay que deshacer nada —los recursos están bien— pero hasta que el código esté en
`main`, nadie puede aplicar sin borrarlos.

### Por qué no basta con `git pull`

El README decía «`git pull` antes de planificar», y es necesario pero no
suficiente: en una rama de trabajo, `git pull` trae los cambios *de esa rama*, no
los de `main`. La rama sigue sin el archivo del compañero y el plan sigue
proponiendo destruirlo. Lo que resuelve es el orden completo:

```
rama → plan → PR → merge → git pull en main → apply
```

---

## 17. SMTP 587, 465 y 2525 no hablan exactamente igual

**Afecta a:** F1, G3 e I1.

Google Compute Engine bloquea el puerto 25. Brevo recomienda 587, con una
conexión inicial en claro que se eleva obligatoriamente mediante STARTTLS. El
puerto 2525 funciona igual y sirve como alternativa en redes restrictivas. El
465 es distinto: negocia TLS desde el primer byte.

El cliente anterior continuaba cuando el servidor no anunciaba STARTTLS. La
biblioteca de Go terminaba rechazando la autenticación, pero la protección
dependía de un detalle interno y el error no explicaba la causa. Desde F1:

- con credenciales, 587 y 2525 exigen STARTTLS;
- 465 usa TLS directo;
- una sesión sin cifrado solo se admite sin credenciales, para Mailpit local;
- producción rechaza puertos diferentes de 587, 465 y 2525.

La configuración efectiva tampoco puede quedar escrita a mano en la VM. D2
creó un `prepare_env.sh` no versionado que reintroducía una clave de ejemplo en
cada reinicio. F1 lo reemplaza por `scripts/prepare_web_env.sh`: el script está
en Git, mientras que la contraseña viene de Secret Manager y el remitente/login
permanecen en `/etc/mooc/web.conf`, fuera del repositorio.

---

## 18. Terraform y el arranque deben interpretar igual los secretos creados desde Windows

**Afecta a:** C1, D2, E1 y F1.

Una contraseña creada con PowerShell o Git Bash puede conservar un retorno de
carro final. C1 ya aplica `trimspace(var.db_password)` al usuario de Cloud SQL;
el script de arranque debe hacer la misma normalización antes de construir
`DATABASE_URL`. Si cada lado interpreta el mismo secreto de forma distinta, la
API recibe `28P01 password authentication failed` aunque Secret Manager,
Terraform e IAM estén configurados correctamente.

El script también puede ejecutarse mediante un enlace simbólico ubicado en la
raíz del despliegue. Derivar el repositorio directamente desde
`BASH_SOURCE[0]` hacía que escribiera `.env` en el directorio padre. Primero se
resuelve la ruta real con `readlink -f`; así una invocación directa y la de
systemd preparan exactamente el mismo archivo.

---

## 19. Lo versionado tiene que reproducir lo que hacía lo que reemplaza

**Afecta a:** D2, E1, F1 y G3.

Corolario de la nota 18, con otra víctima y peores síntomas.

E1 dejó la cola en el Worker Server, así que el `.env` de la VM web apuntaba
`REDIS_URL` a `10.0.1.3:6379` —se ve en su propia evidencia—. Ese valor lo
escribía el `prepare_env.sh` **no versionado** de D2. Cuando F1 lo sustituyó por
`scripts/prepare_web_env.sh`, que sí está en Git, el script fijó
`REDIS_URL='redis:6379'`: el contenedor Redis del propio Web Server.

El resultado no se parece a un fallo. La API encola en un broker que nadie lee,
así que una carga de video responde `202`, el original queda guardado en el
bucket y el recurso se queda en `pending` para siempre. No hay error en ningún
log, porque nada falló: el mensaje se entregó a un Redis que está perfectamente
sano.

```
POST /api/v1/media/uploads/{id}/complete   → 202  processing_status="pending"
… 4 minutos después                        →      processing_status="pending"
```

`prepare_web_env.sh` ahora **exige** `QUEUE_PRIVATE_IP` en `/etc/mooc/web.conf`,
sin valor por omisión, y construye `REDIS_URL` con ella. Una dirección ausente
tiene que detener el despliegue; elegir el Redis equivocado en silencio es el
modo de fallo que costó esta nota. Registrado en **#166**.

**Y esa IP se lee de Terraform, nunca se copia de un documento.** El `10.0.1.3`
del párrafo anterior ya no existe: las dos VMs se recrearon en algún punto y
pasaron a `10.0.1.4` (web) y `10.0.1.5` (worker). Un literal escrito en una guía
sobrevive a la infraestructura que describía, así que
`ADMINISTRACION.md` pide el valor con
`terraform output -raw worker_server_private_ip` en lugar de imprimirlo.

La lección general: al versionar un script que reemplaza a otro que vivía solo en
una VM, la configuración que aquel producía es parte de lo que hay que portar. El
contenido del `.env` viejo es la especificación del script nuevo.

---

## 20. El worker no podía arrancar en producción: validaba la configuración de la API

**Afecta a:** E1, F1, G3 e I1.

El binario del worker llamaba a `config.Validate()`, que es la comprobación de
arranque de la API. En `APP_ENV=production` eso le exigía `APP_BASE_URL`,
`TRUSTED_PROXY_IP`, `CSRF_ALLOWED_ORIGINS` y las cuatro variables de SMTP.

El worker no usa ninguna —se comprueba con un `grep`: no aparecen en `cmd/worker`
ni en `internal/worker`—, y con una de ellas la exigencia era **imposible de
satisfacer**. `mail.tf` le niega `smtp-password` a propósito y lo dice con todas
las letras: «el worker no aparece a propósito, y no es un olvido: procesa video y
no manda mensajes». Así que la única forma de arrancarlo en producción era darle
una credencial que la infraestructura decidió que no debía tener, o inventar un
valor falso para engañar al validador.

Lo encontró G3 al redesplegar el Worker Server. El contenedor entraba en bucle de
reinicio quejándose de tres cosas que no usa, mientras la cola acumulaba tareas:

```
Invalid configuration: la configuracion de produccion conserva valores de desarrollo:
  - APP_BASE_URL apunta a localhost
  - CSRF_ALLOWED_ORIGINS vacio en produccion
  - SMTP_PASSWORD vacio: la contrasena del proveedor vive en Secret Manager
```

Ahora hay dos comprobaciones. `Validate()` es la de la API y no cambió;
`ValidateWorker()` omite la superficie HTTP y el correo, y **conserva entero lo
compartido**: la contraseña de desarrollo en la base, el `sslmode`, el
almacenamiento y el tamaño de los pools. Ahí un valor de desarrollo tiene las
mismas consecuencias en los dos procesos, y relajarlo sería otro agujero.

La lección: una comprobación de arranque que solo se puede satisfacer
contradiciendo el modelo de permisos no protege nada, empuja a saltárselo. Si dos
procesos comparten binario de configuración pero no superficie, la validación
tiene que distinguirlos.
