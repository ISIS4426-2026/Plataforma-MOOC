# A7 — Runbook de la línea base retrospectiva ([#199](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/199))

Recrea el entorno de dos VMs de la Entrega 2, mide los dos escenarios **contra
la nube** con los perfiles de A5 y los criterios de A6, repite el escenario 1 con
el código corregido para aislar su coste, y destruye.

**Antes de empezar, lee [`NOTAS_TECNICAS.md`](../../NOTAS_TECNICAS.md)**, en
particular las notas 1, 2, 20, 24, 25 y 26.

| | |
| :--- | :--- |
| Duración | ~7,5 h con el entorno encendido |
| Coste | **1,69 USD** acotando el consumo HLS · 3,92 USD sin acotarlo |
| Corrida 1 | `1827cdc2ee24c9c785529f0443dc76c7f607e0f5` (commit del tag `entrega-2`) |
| Corrida 2 | el SHA de `main` en el momento de medir |
| Estado remoto | prefijo `entrega2`, el que ya existe. **No se crea uno nuevo**: ver §0 |

---

## 0. Dos cosas del enunciado del issue que no se pueden hacer como están escritas

### El prefijo de estado separado no es viable, y no hace falta

El issue pide «usar un prefijo de estado remoto separado para no mezclar el
estado de la Entrega 2 con el de la Entrega 3», y la nota técnica 1 lo repite.
**No se puede**: la mitad del entorno de la Entrega 2 sigue existiendo —VPC,
NAT, las dos VMs apagadas, los buckets, Artifact Registry, las cuentas de
servicio: 50 recursos— y está registrada en el estado del prefijo `entrega2`.
Un prefijo nuevo intentaría **crear de cero** recursos que ya existen, y los
buckets tienen nombre único global, así que fallaría.

Lo correcto es al revés, y además es más seguro:

- **A7 usa el prefijo `entrega2`.** Es el estado de la Entrega 2 y describe
  exactamente lo que A7 quiere recrear. El plan desde el tag lo confirma: **3 a
  crear, 0 a cambiar, 0 a destruir.**
- **La Entrega 3 estrena prefijo `entrega3`** cuando B1 y B2 empiecen.

La condición que sí hay que respetar es la primera tarea del issue: **A7 corre y
destruye antes de que ninguna rama de la Entrega 3 aplique Terraform.** Mientras
eso se cumpla, un solo prefijo no mezcla nada, porque no hay dos arquitecturas
vivas a la vez.

`scripts/a7_preparar.sh` lo vigila: si el plan propone **cualquier**
destrucción, se para. Eso significa que el estado ya no es el de la Entrega 2 y
recrearla se llevaría el trabajo de otro.

### `terraform apply` desde el tag, no desde `main`

La sexta regla de `infra/terraform/README.md` y la nota 1 dicen «apply solo desde
`main`». A7 es la única excepción, y lo es por diseño: aplica **la configuración
del tag** desde un árbol de trabajo separado, porque medir la línea base con la
infraestructura de `main` ya no sería la línea base.

La regla existe para que dos personas no apliquen configuraciones divergentes
sobre el mismo estado. Aquí se cumple su intención con dos condiciones:

1. Nadie más aplica mientras A7 está en curso.
2. A7 destruye lo que creó antes de que la Entrega 3 empiece.

---

## 1. Pre-vuelo · *tu máquina* · sin coste

```bash
export TF_VAR_db_password="$(gcloud secrets versions access latest \
    --secret=db-password --project=plataforma-mooc-entrega2)"

bash ./scripts/a7_preparar.sh
```

Comprueba, sin cambiar nada: que el plan no destruye nada, que las imágenes de
las dos corridas están publicadas, que el nombre de la instancia de base está
libre, que los perfiles multimedia son los declarados y que la autoprueba de A6
pasa. Sale con 0 solo si todo está en su sitio.

### Publicar las dos imágenes

El pre-vuelo lo marcará como pendiente, porque **ninguna de las dos está
publicada**. En el registro hay cinco imágenes de la Entrega 2 y la más reciente
está **59 commits antes del tag**, con diferencias en `internal/worker/` —
incluido `queue_metrics.go`, que es justo lo que el escenario 2 mide. Ninguna
sirve como línea base.

```bash
# Corrida 1: la imagen del tag. El cd NO es opcional
cd /tmp/a7-entrega2 && bash ./scripts/publish_images.sh

# Corrida 2: la imagen de main
cd "$REPO" && bash ./scripts/publish_images.sh
```

`publish_images.sh` etiqueta con `git rev-parse HEAD` y construye con `.` como
contexto, **las dos cosas relativas al directorio actual**. Invocarlo por ruta
absoluta desde el repositorio etiquetaría la imagen del tag con el SHA de `main`
y construiría el código de `main`: una imagen que miente sobre lo que contiene.

El SHA que acaba en la etiqueta tiene que ser el de 40 caracteres:
`prepare_web_env.sh` y `prepare_worker_env.sh` lo validan y se niegan a arrancar
con otra cosa. Para el tag, el SHA es el del **commit**, no el del objeto de tag:
`entrega-2` es un tag anotado, así que `git rev-parse entrega-2` devuelve el
objeto. Usa `git rev-parse entrega-2^{commit}`.

---

## 2. Provisionar · *tu máquina* · empieza el gasto

```bash
cd /tmp/a7-entrega2/infra/terraform
terraform plan    # confirma: 3 to add, 0 to change, 0 to destroy
terraform apply
```

Crea la instancia de Cloud SQL, la base y el usuario. Tarda unos 10 minutos.

> Si falla con que el nombre `mooc-db-1` está en uso, el borrado anterior aún lo
> reserva (Google lo retiene una semana). Aplica con
> `-var db_instance_generation=2` y **anótalo**: cambia el nombre de conexión y
> la IP privada que las VMs tienen escritas.

```bash
terraform output -raw db_private_ip   # anótala: la necesitan los pasos 3 y 4
```

La IP privada **será distinta** de la de la Entrega 2 (`10.171.240.12`). Las
salidas del estado todavía muestran la vieja hasta que el apply termina.

---

## 3. Encender las VMs · *tu máquina*

Terraform **no gestiona el encendido**: `desired_status` no está declarado, y por
eso el plan dice «0 to change» aunque las dos máquinas estén apagadas.

```bash
gcloud compute instances start mooc-web-server mooc-worker-server \
    --zone=us-east1-b --project=plataforma-mooc-entrega2
```

Conservan disco e IP, así que la URL publicada sigue siendo
`https://34.24.52.111.sslip.io`.

Al arrancar, los contenedores intentan levantar contra la base **vieja**, que ya
no existe: verás `mooc-api-1` sin pasar su comprobación de salud y
`mooc-proxy-1` sin llegar a arrancar. Es lo esperado y se resuelve en el paso 4;
no hay nada que depurar ahí.

---

## 4. Apuntar las VMs a la base nueva y al código del tag · *dentro de cada VM*

El procedimiento es el del [paso 4 de I6](../../../entrega2/evidencias/I6/README.md),
con un cambio: además de la IP de la base, hay que fijar `IMAGE_TAG` al commit
del tag.

```bash
gcloud compute ssh mooc-web-server --zone=us-east1-b \
    --tunnel-through-iap --project=plataforma-mooc-entrega2
```

Dentro de la VM, sobre `/etc/mooc/web.conf`:

```bash
sudo sed -i "s/^DB_PRIVATE_IP=.*/DB_PRIVATE_IP=<la ip del paso 2>/" /etc/mooc/web.conf
sudo sed -i "s/^IMAGE_TAG=.*/IMAGE_TAG=1827cdc2ee24c9c785529f0443dc76c7f607e0f5/" /etc/mooc/web.conf
sudo systemctl restart mooc-web.service
sudo docker ps --format 'table {{.Names}}\t{{.Status}}'
```

Lo mismo en `mooc-worker-server` con `/etc/mooc/worker.conf`. Haz `cp` de
respaldo antes de editar: es lo que permite volver atrás sin adivinar.

Las claves reales de cada `.conf`, comprobadas: el web tiene `IMAGE_TAG`,
`APP_DOMAIN`, `DB_PRIVATE_IP`, `SMTP_HOST`, `SMTP_PORT`, `SMTP_FROM`,
`SMTP_USERNAME` y `QUEUE_PRIVATE_IP` —esta última **duplicada**, con el mismo
valor las dos veces, así que es inocua—; el worker, `IMAGE_TAG` y
`DB_PRIVATE_IP`.

> **Anota qué `IMAGE_TAG` había antes de sobrescribirlo.** En este entorno era
> `b393116bf17234ec435f341fe816816420e5be6e`, **59 commits antes del tag**: lo
> que estuvo vivo en la Entrega 2 no era el código del tag. Es un dato que la
> comparación necesita declarar, no esconder.

Después del reinicio, `grep -c` de la IP nueva en el `.env` debe dar 1: es lo que
confirma que `prepare_env.sh` lo regeneró leyendo el `.conf`. Si da 0, el `sed`
no se aplicó antes del reinicio.

---

## 5. Esquema y datos · *dentro de la VPC*

La instancia no tiene IP pública, así que esto no se puede hacer desde el
portátil. Pasos 5 y 6 de I6, o directamente:

```bash
bash ./scripts/recrear-entorno.sh --en-la-vpc
```

Con el código del tag, el esquema llega hasta `000007`. **No apliques `000008`
todavía**: el snapshot de quiz es parte de las correcciones y entra en la
corrida 2.

---

## 6. Los perfiles multimedia · *tu máquina*

**Esto es lo que la Entrega 2 se saltó.** Sin este paso, la campaña mide lo que
haya en `scratch/`, y lo que había eran clips de 8 segundos.

```bash
bash ./scripts/generar_media_prueba.sh    # genera y verifica campo por campo
```

Falla si un archivo no es lo que declara. Si falla, no sigas: la campaña entera
quedaría invalidada igual que la anterior. Los archivos se quedan en `scratch/`
de **tu máquina**: la campaña los sube por URL firmada, no hace falta copiarlos
a ninguna VM.

### `cmd/seed-media` no se usa, y por qué

Parecía el paso natural para dejar el bucket «sembrado», pero no lo es:

- **Transcodifica en proceso.** Llama a `mediaProcessor.Handle` directamente, no
  encola; así que escribe los derivados con las credenciales de la máquina donde
  corre.
- **Por eso solo puede correr en la VM worker.** El reparto de permisos le
  prohíbe al SA web escribir derivados *a propósito*
  (`api_creator_non_derivatives` frente a `hls_worker_writer`). Ejecutarlo en la
  VM web da un 403 que es la política funcionando, no un fallo de configuración.
  Y escribe hacia `media/hls/` en vez del bucket público, porque el bucket de
  derivados solo está declarado en el `.env` del worker.
- **No aporta nada a la medición.** La campaña del escenario 2 sube los tres
  perfiles reales **27 veces** por el camino de producción —URL firmada, asynq,
  worker, HLS— y deja el bucket sembrado con evidencia de la propia medición.

Si se ejecuta y falla a mitad, deja un recurso en `processing` y un original
huérfano. **Hay que limpiarlos antes de medir**: la verificación terminal del
escenario 2 cuenta estados de `resources` bajo `originals/` y contaría un trabajo
colgado que no es suyo.

---

## 7. Escenario 1, código del tag · *tu máquina*

El generador va fuera de las máquinas medidas, como declara
[`escenario1.md`](../../../../capacity-planning/escenario1.md), y mide la latencia
de extremo a extremo incluida la red.

Primero las cuentas de carga y los tokens, que **no** van dentro de la medición.
La contraseña de prueba es la que documenta el encabezado de
`scripts/seeds/capacity_data.sql`; se pasa por entorno y no se escribe aquí, por
la misma razón que `capacity_login_tokens.sh` tampoco la lleva dentro:

```bash
export CAPACITY_PASSWORD='<la del encabezado de capacity_data.sql>'
```


```bash
# Las 200 cuentas, una sola vez
bash scripts/cargar_cuentas_carga_nube.sh scripts/seeds/capacity_data.sql

# Los tokens: ~23 min, porque el login está limitado a 10/min por IP
BASE_URL=https://34.24.52.111.sslip.io CAPACITY_PASSWORD="$CAPACITY_PASSWORD" \
  bash scripts/capacity_login_tokens.sh 200 1
```

Y después la escalera:

```bash
MODO=nube REUSAR_TOKENS=1 CAPACITY_PASSWORD="$CAPACITY_PASSWORD" \
BASE_URL=https://34.24.52.111.sslip.io \
ENTORNO=nube-entrega2-retrospectiva-tag \
RESULTS_ROOT="$PWD/docs/entrega3/evidencias/A7/escenario1-1st-architecture" \
  bash scripts/h3_nube.sh escalera
```

Tres cosas que no son opcionales:

- **`escalera`** es un subcomando obligatorio. Sin él el script solo imprime su
  uso y sale con 1.
- **`REUSAR_TOKENS=1`**, o `escalera` borra `tokens.csv` y vuelve a iniciar
  sesión **en paralelo a las corridas**: 8,5 logins/min dentro de la ventana
  medida, que es exactamente lo que el paso separado de tokens existe para
  evitar (`escenario1.md` §4). Úsalo solo si los tokens son de este entorno y de
  hace menos de 24 h.
- **`CAPACITY_PASSWORD`** exportada, o el script la pide por teclado y se cuelga
  cuando se ejecuta sin terminal interactiva.

`EXIGIR_NUBE=1` lo activa `h3_nube.sh` solo en modo nube: hace que el resumen
**falle** si el destino registrado es local. Cada corrida deja su `destino.json`
con la URL efectiva, el commit, el generador y la ventana; sin él no se publican
tablas (A6).

---

## 7 bis. Dos trampas al ejecutar cosas dentro de las VMs

Cuestan media hora cada una si se descubren a mitad.

### El `.env` de las VMs es de root

`prepare_env.sh` corre como root desde systemd, así que `/home/dfortizr1/mooc/.env`
no es legible por tu usuario. Un `source` normal falla con `Permission denied`
y, al quedarse sin `DATABASE_URL`, el binario se va a buscar Postgres en
`localhost` y falla con un error que no menciona el permiso.

```bash
sudo bash -c 'set -a; source /home/dfortizr1/mooc/.env; set +a; exec <binario> <args>'
```

Así se lee en el shell de root y **no queda ninguna copia del secreto en disco**.

### Las comillas no sobreviven a `plink`

En Windows, `gcloud compute ssh --command` con comillas anidadas —`psql -c "... '...' ..."`,
`$(...)`, `$$...$$`— llega destrozado al otro lado y da un error de sintaxis con
un `^` que no señala nada útil. La salida es no pelearse:

```bash
# escribe el script, cópialo, ejecútalo
gcloud compute scp ./mi_script.sh mooc-web-server:/tmp/ --zone=us-east1-b \
    --tunnel-through-iap --project=plataforma-mooc-entrega2 --quiet
gcloud compute ssh mooc-web-server --zone=us-east1-b --tunnel-through-iap \
    --project=plataforma-mooc-entrega2 --quiet --command "bash /tmp/mi_script.sh"
```

Y **`--quiet` siempre**: sin él, `pscp` y `plink` pueden quedarse esperando una
confirmación que nadie va a teclear, sin escribir una sola línea de salida.

> El túnel IAP hacia la VM worker dio 14 kB/s con cortes mientras hacia la web
> daba 3,4 MB/s, con las dos máquinas ociosas. Si tienes que mover algo grande a
> una VM, pásalo por el bucket (`gcloud storage cp`): es dentro de la región, no
> cuesta egreso y no depende del túnel.

---

## 8. Escenario 2, código del tag · *tu máquina, con dos túneles*

### Por qué la Entrega 2 lo midió contra Docker Compose

No fue descuido: la campaña necesita **el inspector de la cola** (Redis) y, para
la verificación terminal, la base. El cortafuegos solo abre 6379 dentro de
`10.0.1.0/24`, y Cloud SQL no tiene IP pública. Desde un portátil no hay a dónde
conectarse, y `-base http://api:8080` estaba ahí como valor por omisión.

La salida no es medir Compose: es **llevar los dos puertos por el túnel de IAP**,
que usa el 22 y no abre nada nuevo —F1 sigue pudiendo demostrar que no hay
puertos de aplicación ni de datos expuestos—.

```bash
# Terminal A: deja el túnel abierto
gcloud compute ssh mooc-worker-server --zone=us-east1-b \
    --tunnel-through-iap --project=plataforma-mooc-entrega2 \
    -- -N -L 6379:localhost:6379 -L 5432:<ip privada de la base>:5432
```

```bash
# Terminal B: la campaña, contra la URL pública real
ENTORNO=nube-entrega2-retrospectiva-tag \
HLS_BASE=https://storage.googleapis.com/plataforma-mooc-entrega2-hls \
PATRON=cadencia \
MAX_SEGMENTOS=10 \
OUT_DIR="$PWD/docs/entrega3/evidencias/A7/escenario2-1st-architecture" \
bash ./scripts/run_capacity_escenario2.sh https://34.24.52.111.sslip.io \
    -redis localhost:6379 \
    -db "postgres://moocuser:$TF_VAR_db_password@localhost:5432/moocdb?sslmode=require"
```

La campaña se niega a arrancar si no se le declara `-base`, `-entorno` y
`-hls-base` (A6).

### Cuánto tarda y por qué no se puede acelerar

Son **162 minutos de vídeo** repartidos en 27 subidas, y transcodificar cuesta
1,2× la duración (nota 24): **194 minutos de vCPU, ~97 minutos de pared** con
`WORKER_CONCURRENCY=2`. El nivel 3 solo son 41 minutos. No hay forma de
acortarlo sin cambiar los perfiles, y cambiarlos es volver al defecto.

### Por qué `MAX_SEGMENTOS=10`

Descargar los vídeos completos desde el portátil son **22,1 GB de egreso, 2,65
USD**: el 5 % del crédito, más que todo el cómputo de la campaña (nota 9). Con 10
segmentos por estudiante son 3,55 GB y 0,43 USD, y siguen siendo 70 s de
reproducción: con el buffer inicial de 2 segmentos, quedan 8 con instante de
reproducción que cumplir, que es lo que hace medibles el margen de buffer y las
interrupciones.

Si quieres el egreso completo, `MAX_SEGMENTOS=0` y decláralo en la evidencia.

---

## 9. Desplegar el código corregido · *tu máquina y las VMs*

Misma infraestructura, mismo generador, mismos niveles. Solo cambia el código y
PITR. Eso es lo que aísla el coste de las correcciones.

```bash
# PITR, que es la única diferencia de infraestructura entre el tag y main
cd "$REPO/infra/terraform"   # el repositorio, en main, no el árbol del tag
terraform apply              # habilita PITR sobre la misma instancia, sin recrearla
```

En cada VM, `IMAGE_TAG` al SHA de `main` y reiniciar. Después, el esquema:

```bash
bash ./scripts/migrate.sh              # aplica 000008
bash ./scripts/migrate.sh --require    # falla si falta algo que la imagen necesita
```

### Cambiar `IMAGE_TAG` **no basta**

La unidad de systemd no apunta a un checkout del repositorio:

```
WorkingDirectory=/home/dfortizr1/mooc
ExecStartPre=/bin/bash /home/dfortizr1/mooc/prepare_env.sh
ExecStart=/usr/bin/docker compose -f /home/dfortizr1/mooc/docker-compose.prod.yml up -d
```

`prepare_env.sh` y el compose son **copias congeladas de la época del tag** en el
home del servicio. La imagen cambia; esos dos archivos no. Y `main` añade en
`prepare_web_env.sh` tres variables que importan:

```
REDIS_SESSION_DB='1'
INSTANCE_ID=...
SESSION_COOKIE_SAMESITE='...'
```

Sin copiarlos, **las sesiones se quedan en la base lógica 0, mezcladas con la
cola de asynq** —lo que advierte la nota 23— y la corrida 2 no mediría la
configuración con la que correrá la Entrega 3. Se comprueba en el Redis del
worker: con el código del tag, `info keyspace` muestra las sesiones en `db0`.

```bash
# desde tu máquina, para cada VM
gcloud compute scp scripts/prepare_web_env.sh \
    mooc-web-server:/tmp/prepare_env.sh --zone=us-east1-b --tunnel-through-iap \
    --project=plataforma-mooc-entrega2 --quiet
gcloud compute scp docker-compose.prod.yml \
    mooc-web-server:/tmp/ --zone=us-east1-b --tunnel-through-iap \
    --project=plataforma-mooc-entrega2 --quiet
```

Y dentro de la VM, con respaldo antes de sustituir:

```bash
sudo cp /home/dfortizr1/mooc/prepare_env.sh{,.tag-respaldo}
sudo cp /home/dfortizr1/mooc/docker-compose.prod.yml{,.tag-respaldo}
sudo cp /tmp/prepare_env.sh /tmp/docker-compose.prod.yml /home/dfortizr1/mooc/
sudo systemctl restart mooc-web.service
```

Lo mismo en el worker con `prepare_worker_env.sh` y `docker-compose.worker.yml`.
Los respaldos son lo que permite volver a la corrida 1 si hay que repetirla.

### Lo que hay que mirar: `REDIS_URL`

`docker-compose.prod.yml` de `main` **ya no levanta un Redis propio** y exige
`REDIS_URL` (A4). En la disposición de dos VMs eso funciona sin cambios, porque
`prepare_web_env.sh` ya escribe `REDIS_URL='${QUEUE_PRIVATE_IP}:6379'`, es decir
la cola de la VM worker por la VPC. Las sesiones van a la base lógica 1 de ese
mismo Redis.

Confirma que `QUEUE_PRIVATE_IP` está en `/etc/mooc/web.conf` y apunta a
`10.0.1.5`. Si estuviera vacía, el arranque falla **en voz alta** —es el guarda
de A4 funcionando—, no en silencio.

---

## 10. Escenario 1, código corregido · *tu máquina*

Idéntico al paso 7 cambiando la etiqueta:

```bash
export ENTORNO=nube-entrega2-retrospectiva-corregida
export RESULTS_ROOT="$PWD/docs/entrega3/evidencias/A7/escenario1-1st-architecture"
MODO=nube bash scripts/h3_nube.sh
```

El escenario 2 **no se repite**: su coste de corrección está en el worker, que no
cambia de forma apreciable, y repetirlo costaría otros 97 minutos. Si se quiere,
se repite solo el nivel 2.

---

## 11. Verificar el E2E que nunca se pudo ejecutar · *tu máquina*

Aprovechando que hay entorno con el código corregido:

- **Un documento y una imagen** por el flujo normal, hasta `completed`. Es el
  defecto de la nota 16: antes acababan en `failed` y nada lo ejercitaba.
- **Un intento de quiz congelado**: empezar el intento, cambiar la pregunta como
  profesor, enviar, y comprobar que se califica contra el enunciado congelado
  (nota 17).
- **La sesión servida por el Redis compartido**, que es lo que A4 cambió.

---

## 12. Comparar y cerrar

### Las tres comparaciones

| Comparación | Qué aísla | Para quién |
| :--- | :--- | :--- |
| Corrida 1 contra lo publicado en `pruebas_de_carga_entrega2.md` | Que la medición retrospectiva reproduce la Entrega 2: **avala el método** | G7 |
| Corrida 2 contra corrida 1 | El coste por petición de las correcciones de la Entrega 1 | H3, G7 |
| **Entrega 3 contra corrida 2** | **El efecto de la arquitectura elástica.** Es la que se publica | **G3, G4** |

**La tercera es la que importa, y su línea base es la corrida 2.** No hay nada
que restar: la Entrega 3 elástica corre el mismo código corregido, así que el
coste de las correcciones está en los dos lados y se cancela por construcción.
La corrida 1 **no es operando** de esa comparación.

Entonces, ¿para qué la corrida 1? Para dos cosas que la corrida 2 no puede dar
sola:

- **Avalar el método.** Reproduce las cifras publicadas de la Entrega 2 en
  rendimiento, latencia de servidor sobre conexión caliente, transacciones por
  segundo y utilización. Sin ese contraste, la línea base corregida es un número
  que nadie puede verificar contra nada.
- **Cuantificar lo que costaron las correcciones**, que es un hallazgo con valor
  propio para H3 y para explicar en G7 por qué el criterio del escenario 2 se
  calificó 6.67.

### Parámetros que se mantienen constantes hacia la Entrega 3

Si uno de estos cambia, la comparación de G3 deja de ser válida y hay que decirlo.

| Parámetro | Valor |
| :--- | :--- |
| Perfiles multimedia | 120 / 600 / 1800 s, 1280×720, h264 + aac, 174 kbps |
| Escalera HLS | 360p + 720p, sin upscaling (`internal/transcode.DefaultLadder`) |
| Concurrencia de workers | `WORKER_CONCURRENCY=2` |
| Niveles del escenario 1 | 1, 10, 25, 50, 100, 200 usuarios; rampa 30 s; pausa 4000 ± 2000 ms; 3 sesiones |
| Calentamiento descontado | rampa + 15 s |
| Niveles del escenario 2 | 1 / 2 / 4 / 8 / 12 subidas; 2 / 10 / 25 / 50 / 100 estudiantes |
| Patrón de consumo HLS | `cadencia`, buffer inicial de 2 segmentos |
| Generador | el portátil, fuera de las máquinas medidas |
| Región y zona | `us-east1`, `us-east1-b` |
| Perfil de Cloud SQL | `db-custom-1-3840`, PD_SSD 10 GB, `max_connections` declarado |
| Estimador de percentil y mínimos de observaciones | los de A6 |
| Criterios de éxito y parada | `escenario1.md` §8 y `escenario2.md` §7 |

### Diferencias declaradas, cada una posible causa de los resultados

| Diferencia | Corrida 1 | Corrida 2 | Entrega 3 |
| :--- | :--- | :--- | :--- |
| Imagen | commit del tag | SHA de `main` | SHA de `main` |
| Esquema | hasta `000007` | `+000008` (snapshot de quiz) | igual o posterior |
| PITR | desactivado | activado | activado |
| Cómputo de la API | 1 VM fija | 1 VM fija | MIG de 1 a 3 instancias |
| Cómputo de workers | 1 VM fija | 1 VM fija | MIG de 1 a 3 instancias |
| Entrada | Nginx en la VM, TLS con sslip.io | igual | balanceador HTTPS gestionado |
| Cola | Redis en la VM worker | igual | servicio compartido fuera de los grupos |
| Sesiones | caché local de la VM web | Redis compartido, base lógica 1 | igual que la corrida 2 |
| Sondas de salud | `/api/v1/health` único | liveness y readiness separadas | igual que la corrida 2 |

### Destruir y registrar el coste

El procedimiento de eliminación es el [§5 de I6](../../../entrega2/evidencias/I6/README.md).
Lo que A7 añade: hacerlo **el mismo día**. Con todo encendido son 4,05 USD/día,
el 8 % del crédito activo.

```bash
# Coste real de la campaña, por servicio y por día
gcloud billing accounts list
# y en la consola: Facturación > Informes, filtrando por las fechas de la campaña
```

Registra en la evidencia el coste medido, no el estimado, y la diferencia con las
1,69 USD previstas.

---

## Lista de comprobación antes de publicar cualquier cifra

Es la §10 de [`criterios_de_medicion.md`](../../../../capacity-planning/criterios_de_medicion.md).
Lo que A7 no puede omitir:

- [ ] Cada corrida tiene `destino.json` y su clase es `remoto`.
- [ ] Las dos corridas del escenario 1 declaran su `IMAGE_TAG`.
- [ ] Los perfiles pasaron `generar_media_prueba.sh --verificar` **antes** de medir.
- [ ] Ningún p95 publicado baja de 20 observaciones.
- [ ] El tiempo al primer cuadro no aparece en ninguna tabla.
- [ ] La medición está marcada como **retrospectiva**, con commit, configuración y fecha.
- [ ] El entorno está destruido y el coste registrado.
- [ ] La evidencia no contiene credenciales, llaves ni secretos.

---

Este archivo no contiene credenciales, llaves ni secretos.
