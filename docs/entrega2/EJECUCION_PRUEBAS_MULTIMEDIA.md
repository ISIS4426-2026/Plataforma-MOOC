# Ejecución de las pruebas de multimedia

Guía paso a paso para correr `collection_media.postman_collection.json`, que
cubre el flujo de carga directa al almacenamiento de objetos: autorización,
transferencia, confirmación, idempotencia e inmutabilidad.

**25 peticiones, 43 aserciones.** No necesita ningún archivo externo: el video
de prueba va como cuerpo de la petición, así que la colección es autocontenida.

---

## Antes de empezar

El entorno completo tiene que estar arriba y con datos sembrados.

```bash
docker compose up -d --build
docker compose ps
```

Los seis servicios deben reportar `healthy`, salvo `minio-policy`, que es de un
solo uso y debe aparecer como `Exited (0)`. Ese servicio es el que hace legible
el prefijo `hls/` sin firma; si salió con código distinto de 0, las pruebas de
reproducción fallarán.

Cargue los datos sintéticos:

```bash
docker compose exec -T postgres psql -U moocuser -d moocdb < scripts/seeds/synthetic_data.sql
```

La colección usa dos identificadores sembrados —`seededPublishedVideoResourceId`
y `seededPublishedUnitId`— para probar la inmutabilidad sobre un curso ya
publicado. Vienen en los archivos de entorno, no hay que escribirlos.

---

## Opción A — Newman dentro de la red de Compose (recomendada)

No requiere instalar nada ni tocar la configuración de red de su máquina.

```bash
make test-postman-media
```

Lo que hace que esta opción funcione sin ajustes no es newman en sí, sino **el
contenedor**: corre dentro de la red de Docker Compose, donde el host
`minio:9000` que lleva la URL firmada ya resuelve. Newman instalado por npm en
su máquina caería en el mismo caso que la Opción B. Salida esperada:

```
requests      25    0 failed
assertions    43    0 failed
```

---

## Opción B — Cualquier cliente que corra en su máquina

Postman Desktop, o newman instalado por npm. Funciona igual de bien, pero
**requiere una entrada en el archivo de hosts**.

### Por qué

La API firma la URL de carga usando `S3_ENDPOINT`, que en Compose vale
`http://minio:9000`. La firma **cubre el host**, así que reescribir `minio` por
`localhost` en la URL la invalida.

El puerto ya está publicado en su máquina —`localhost:9000` responde— y lo único
que falta es que el nombre `minio` apunte ahí. Con eso la firma sigue siendo
válida, porque la cabecera `Host` sigue diciendo `minio:9000`.

Sin la entrada, 18 de las 25 peticiones pasan y fallan 7: las dos cargas
directas y las cinco que dependen de que el objeto exista.

### 1. Agregar la entrada de hosts

**Windows.** Abra el Bloc de notas **como administrador**, abra
`C:\Windows\System32\drivers\etc\hosts` y agregue al final:

```
127.0.0.1 minio
```

**macOS o Linux:**

```bash
sudo sh -c 'echo "127.0.0.1 minio" >> /etc/hosts'
```

### 2. Comprobar que quedó bien

```bash
curl -s -o /dev/null -w "%{http_code}\n" http://minio:9000/minio/health/live
```

Debe imprimir `200`. Si imprime `000`, el nombre todavía no resuelve: revise que
guardó el archivo con permisos de administrador.

### 3. Importar en Postman

*(Si prefiere newman en su máquina, sáltese este paso: con la entrada de hosts
puesta, `newman run docs/postman/collection_media.postman_collection.json -e
docs/postman/mooc_local.postman_environment.json` funciona igual.)*


1. Abra **Postman Desktop**.
2. **Import** y seleccione de `docs/postman/`:
   - `collection_media.postman_collection.json`
   - `mooc_local.postman_environment.json`
3. Arriba a la derecha seleccione el entorno **`Plataforma MOOC - Local`**.
4. Abra la colección, entre a **Runner** y pulse **Run Collection**.

Las 43 aserciones deben pasar. Los scripts encadenan todo solos: extraen el
token, el identificador del recurso, la URL firmada y la clave del objeto.

---

## Qué verifica la colección

| Peticiones | Qué comprueban |
| :--- | :--- |
| 01–05 | Login y creación de curso, módulo, unidad y recurso de video |
| 06 | La URL firmada, con la clave bajo `originals/<stable_id>/`, el método y el content-type que cubre la firma, y la vigencia de 24 horas |
| 07 | La carga directa al bucket, sin que el archivo pase por la API |
| 08–09 | La confirmación y su reintento: la segunda responde 202, no 500, y no encola un segundo trabajo |
| 10 | La URL firmada de lectura, mucho más corta que la de carga |
| 11–18 | Rechazos: extensión no permitida, mime incongruente, tamaño excedido, recurso de texto, carga inexistente, clave de otro recurso |
| 19–21 | Control de acceso: estudiante y petición anónima |
| 22–25 | Inmutabilidad: firmar y cargar sobre un curso publicado se permite, confirmar se rechaza con 409, y el recurso publicado queda intacto |

---

## Qué estado deja la colección, y cómo obtener uno real

**Los recursos que crea la colección terminan en `failed`, y es lo esperado.**

La colección sube un cuerpo de texto como marcador —`bytes de prueba que hacen
las veces de un mp4`— para ser autocontenida, sin depender de un archivo
externo. El plano de control no nota la diferencia: la firma se emite, la
transferencia llega al bucket y la confirmación registra el objeto. Pero cuando
el worker lo recoge, ffprobe lo rechaza y el recurso pasa a `failed`.

Eso es exactamente lo que la colección verifica: **autorización, transferencia
directa, confirmación, idempotencia, validaciones e inmutabilidad**. La
transcodificación en sí se prueba aparte, con las pruebas de Go y con el
procedimiento de abajo.

Las aserciones afirman `processing_status: pending` justo después de confirmar,
que es el estado correcto en ese momento. El `failed` llega segundos más tarde,
cuando el worker ya terminó.

### Generar un video real

Para ver HLS de verdad —para la sustentación, o para revisar la salida— hace
falta un video real. El contenedor del worker trae ffmpeg, así que puede
generarlo sin instalar nada:

```bash
docker compose exec worker sh -c \
  "ffmpeg -hide_banner -loglevel error \
     -f lavfi -i testsrc=size=1280x720:rate=25:duration=8 \
     -f lavfi -i sine=frequency=440:duration=8 \
     -c:v libx264 -pix_fmt yuv420p -c:a aac -shortest /tmp/clase.mp4"

docker compose cp worker:/tmp/clase.mp4 ./clase.mp4
```

Quedan 8 segundos de video 1280x720 con audio, unos 170 KB. Está en
`.gitignore`, así que no se va a colar en un commit.

### Pasarlo por el flujo desde Postman

1. **Corra las peticiones 01 a 06 de la colección Multimedia**, tal cual: login como
   profesor, curso, módulo, unidad, recurso de video y la URL prefirmada.
2. **Duplique la petición 07 -PUT** y cambie su cuerpo a **Body → binary → Select
   File**, eligiendo `clase.mp4` o su video de prueba. La 07 original manda texto; esta manda el
   video. Verifique que la cabecera `Content-Type` sea `video/mp4`, el mismo
   valor que devolvió la 06, porque va dentro de la firma. Espere **200**.

Si corre el `PUT` desde su máquina necesita la entrada de hosts de la Opción B;
desde un contenedor de la red no hace falta.

3. **Espere unos segundos y corra la peticion 08**. El `stable_id` que va a
   necesitar está en la respuesta de la 08, y el "processing_status" será "completed".Espere **202**.

El recurso queda en `completed` y aparecen los 7 objetos bajo
`hls/<stable_id>/`: `master.m3u8`, `360p.m3u8`, `720p.m3u8` y sus segmentos.
Puede comprobarlo con [Leer un manifiesto](#leer-un-manifiesto), más abajo, o
abrirlo [desde el navegador](#desde-el-navegador-con-el-host-configurado) para
reproducir el video.

---

## Inspeccionar lo que quedó en el bucket

**No hay consola web.** MinIO la retiró del servidor comunitario: el puerto 9001
no responde ni desde dentro del contenedor, así que el Compose ya no lo publica.
La herramienta es `mc`, que viene en la imagen.

### Configurar el alias (una vez por contenedor)

```bash
docker compose exec minio mc alias set local http://minio:9000 minioadmin minioadmin
```

Dura mientras el contenedor viva. Si recrea `minio`, repítalo.

### Listar

```bash
# Todo el bucket, con totales
docker compose exec minio mc ls --recursive --summarize local/mooc-storage/

# Solo lo que subió el profesor
docker compose exec minio mc ls --recursive local/mooc-storage/originals/

# Solo lo que generó el worker
docker compose exec minio mc ls --recursive local/mooc-storage/hls/
```

### Cuánto pesa cada lado

```bash
docker compose exec minio mc du local/mooc-storage/originals local/mooc-storage/hls
```

```
173KiB   7 objects   mooc-storage/originals
1.6MiB   7 objects   mooc-storage/hls
```

Útil para el escenario 2: es la relación entre lo que se subió y lo que produjo
la transcodificación.

### Leer un manifiesto

Necesita el `stable_id` de un recurso **que el worker haya procesado de verdad**:

```bash
docker compose exec -T postgres psql -U moocuser -d moocdb -t -A -F' | ' -c "SELECT stable_id, processing_status FROM resources WHERE object_key LIKE 'originals/%' AND processing_status = 'completed';"
```

Los dos filtros hacen falta, y cada uno descarta una fuente distinta de
confusión:

- **`object_key LIKE 'originals/%'`** descarta los datos sintéticos. El seed trae
  dos recursos con `processing_status = 'completed'` y una clave ficticia como
  `courses/c1/video_workers_hls.m3u8`: son de relleno, nunca pasaron por el
  worker y no tienen nada en `hls/`.
- **`processing_status = 'completed'`** descarta lo que dejó la colección, que
  termina en `failed` por diseño.

**Si la consulta no devuelve ninguna fila, es la respuesta correcta:** todavía no
ha procesado un video real. Vuelva a la sección anterior y genere uno; sin eso no
hay manifiesto que leer, y `mc cat` responderá `Object does not exist`.

Con un <stable_id> | completed, realice:

```bash
docker compose exec minio mc cat local/mooc-storage/hls/<stable_id>/master.m3u8
```

```
#EXTM3U
#EXT-X-VERSION:3
#EXT-X-STREAM-INF:BANDWIDTH=896000,RESOLUTION=640x360
360p.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=2628000,RESOLUTION=1280x720
720p.m3u8
```
docker compose exec minio mc cat local/mooc-storage/hls/0d33dc26-1359-423d-bef6-37adfaf4b190/master.m3u8
### Ver metadatos o descargar un objeto

```bash
docker compose exec minio mc stat local/mooc-storage/hls/<stable_id>/master.m3u8
docker compose exec minio mc cp   local/mooc-storage/hls/<stable_id>/720p_0000.ts /tmp/
```

### Desde el navegador con el host configurado

Con la entrada de hosts minio puesta en el txt, los derivados son legibles directamente, porque
el prefijo `hls/` se sirve sin firma:

```
http://minio:9000/mooc-storage/hls/<stable_id>/master.m3u8
```

El original, en cambio, devuelve **403**: es privado a propósito, y esa asimetría
es la que hay que replicar en el bucket administrado.

---

## Si algo falla

**`Error: connect ECONNREFUSED` o código `000` en la petición 07.**
El nombre `minio` no resuelve. Vuelva al paso 1.

**`403 Forbidden` en la petición 07.**
La firma no coincide. Suele ser por haber editado la URL a mano o por un reloj
desfasado: la firma lleva marca de tiempo. Regenere la URL corriendo la
petición 06 otra vez.

**Los recursos de la colección quedaron en `failed`.**
Es lo esperado: sube un marcador de texto, no un mp4. Ver «Qué estado deja la
colección».

**`Object does not exist` al leer un manifiesto.**
El `stable_id` que usó no corresponde a un video procesado. Dos causas posibles:
es un recurso sembrado —`completed` con clave ficticia, nunca pasó por el
worker— o es uno de la colección, que termina en `failed`. Use la consulta con
los dos filtros; si no devuelve nada, genere primero un recurso real.

**`409 upload_not_found` en la petición 08.**
La carga de la 07 no llegó al bucket. Ejecute la 07 antes que la 08; el runner
las corre en orden, pero una ejecución suelta de la 08 falla así.

**`minio-policy` salió con código distinto de 0.**
Reviselo con `docker compose logs minio-policy`. Si dice
`The specified bucket does not exist`, arránquelo de nuevo con
`docker compose up -d minio-policy --force-recreate`: espera a que el bucket
exista, pero con un límite de 30 segundos.

---

## Nota sobre la nube

Nada de esto aplica al entorno desplegado. Allá el endpoint del bucket
administrado es la misma dirección para la API y para el cliente, así que la URL
firmada resuelve desde cualquier parte. Es un artefacto puramente local, de
tener MinIO detrás de un nombre de red de Docker.

El detalle está en [`NOTAS_TECNICAS.md`](NOTAS_TECNICAS.md), nota 1.
