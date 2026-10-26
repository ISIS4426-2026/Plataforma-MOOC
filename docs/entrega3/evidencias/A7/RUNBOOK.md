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

Lo mismo en `mooc-worker-server` con `/etc/mooc/worker.conf`. Confirma los
nombres exactos de las variables en el `.conf` de cada máquina antes de editarlo:
`prepare_web_env.sh` exige `IMAGE_TAG`, `APP_DOMAIN`, `DB_PRIVATE_IP`,
`QUEUE_PRIVATE_IP`, `SMTP_FROM` y `SMTP_USERNAME`, y el de worker `IMAGE_TAG` y
`DB_PRIVATE_IP`.

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

## 6. Sembrar los perfiles multimedia · *dentro de la VPC*

**Esto es lo que la Entrega 2 se saltó.** Sin este paso, la campaña mide lo que
haya en `scratch/`, y lo que había eran clips de 8 segundos.

```bash
bash ./scripts/generar_media_prueba.sh    # genera y verifica campo por campo
go run ./cmd/seed-media
```

`generar_media_prueba.sh` falla si un archivo no es lo que declara. Si falla, no
sigas: la campaña entera quedaría invalidada igual que la anterior.

---

## 7. Escenario 1, código del tag · *tu máquina*

El generador va fuera de las máquinas medidas, como declara
[`escenario1.md`](../../../../capacity-planning/escenario1.md), y mide la latencia
de extremo a extremo incluida la red.

```bash
export BASE_URL=https://34.24.52.111.sslip.io
export ENTORNO=nube-entrega2-retrospectiva-tag
export EXIGIR_NUBE=1
export RESULTS_ROOT="$PWD/docs/entrega3/evidencias/A7/escenario1-tag"

bash scripts/capacity_login_tokens.sh
MODO=nube bash scripts/h3_nube.sh
```

`EXIGIR_NUBE=1` hace que el resumen **falle** si el destino registrado es local.
Cada corrida deja su `destino.json` con la URL efectiva, el commit, el generador
y la ventana; sin él no se publican tablas (A6).

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
OUT_DIR="$PWD/docs/entrega3/evidencias/A7/escenario2-tag" \
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
export RESULTS_ROOT="$PWD/docs/entrega3/evidencias/A7/escenario1-corregido"
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
| Corrida 1 contra lo publicado en `pruebas_de_carga_entrega2.md` | El efecto de medir contra la nube con perfiles reales en vez de contra Compose con clips | G7 |
| Corrida 2 contra corrida 1 | El coste por petición de las correcciones de la Entrega 1 | **G3** |
| Entrega 3 contra corrida 2 | El efecto de la arquitectura elástica, ya sin la variable de las correcciones | G3, G4 |

La segunda es la razón de ser de la doble corrida: deja el coste de las
correcciones **como un número** que G3 resta, en vez de como una advertencia.

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
