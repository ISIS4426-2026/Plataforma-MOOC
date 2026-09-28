# Evidencia I6 — Cierre: recreación y eliminación controlada de recursos (issue #141)

Cierra la entrega apagando lo que el enunciado pide apagar, conservando lo que
permite reconstruir el entorno y dejando registrado qué sigue costando.

> **El enunciado lo exige, no es una decisión del equipo.** Sección de gestión de
> costos: *«Después de registrar las evidencias y cargar la entrega, eliminar la
> instancia de base de datos administrada del laboratorio, conservando
> previamente los respaldos o los datos sintéticos y scripts necesarios para
> reconstruirla. Documentar qué recursos se conservan, sus costos y cómo recrear
> el entorno para una sustentación.»*

| Tarea | |
| :--- | :---: |
| Evidencias cargadas antes de tocar nada | ✅ §1 |
| Respaldo final conservado | ✅ §2 |
| Recreación ensayada, con tiempo registrado | ✅ §3 — **verificada de nuevo sobre el entorno vacío: 61/61** |
| Instancia de base de datos eliminada | ⏳ §5 |
| Revisión de lo que sigue costando | ✅ §4 |
| Qué se conserva, su costo y cómo recrear | ✅ §4 y §6 |

---

## 1. Antes de tocar nada

| Condición | |
| :--- | :--- |
| Tag `entrega-2` creado y empujado | ✅ |
| El README del commit tagueado enlaza los videos de la Entrega 2 | ✅ |
| Evidencia de los 26 issues en `docs/entrega2/evidencias/` | ✅ |
| Informe de capacidad publicado en `capacity-planning/` | ✅ |

Es la primera tarea del issue y la condición que hace irreversible todo lo
demás: **una vez eliminada la instancia, las evidencias que dependan de
consultarla ya no se pueden rehacer.**

## 2. Lo que se conserva para reconstruir

El enunciado admite conservar **«los respaldos *o* los datos sintéticos y
scripts necesarios para reconstruirla»**. Se cumple por la segunda vía, y está en
el repositorio bajo el tag:

| | Dónde |
| :--- | :--- |
| Esquema completo, 7 migraciones reversibles | [`migrations/`](../../../../migrations/) |
| Datos sintéticos de la plataforma | [`scripts/seeds/synthetic_data.sql`](../../../../scripts/seeds/synthetic_data.sql) |
| Datos de las pruebas de carga | [`scripts/seeds/capacity_data.sql`](../../../../scripts/seeds/capacity_data.sql) |
| Aplicador de migraciones, repetible | [`scripts/migrate.sh`](../../../../scripts/migrate.sh) |
| Reconstrucción completa, de cero a utilizable | [`scripts/recrear-entorno.sh`](../../../../scripts/recrear-entorno.sh) |
| Infraestructura declarada | [`infra/terraform/`](../../../../infra/terraform/README.md) |

> **Las copias de Cloud SQL se borran con la instancia.** Al eliminarla
> desaparecen sus respaldos automáticos y bajo demanda. Por eso lo que se
> conserva son el esquema y las semillas versionadas, no una copia dentro del
> servicio que va a dejar de existir.
>
> Opcionalmente puede tomarse un volcado a Cloud Storage, que sí sobrevive:
>
> ```bash
> gcloud sql export sql mooc-db-1 \
>   gs://plataforma-mooc-entrega2-media/respaldos/moocdb-final.sql.gz \
>   --database=moocdb --project=plataforma-mooc-entrega2
> ```
>
> Requiere conceder escritura en el bucket a la cuenta de servicio de la
> instancia. El contenido es el mismo que reproducen las semillas.

## 3. La recreación, verificada de extremo a extremo

C2 (#119) ya había ensayado la restauración, y aquí se repitió **sobre el entorno
realmente vacío**, que es el escenario que importa: la instancia eliminada, las
VMs apagadas y nada que reaprovechar.

| | Resultado |
| :--- | :--- |
| Instancia recreada con Terraform | ✅ `mooc-db-1`, IP privada **`10.171.240.10`** |
| Esquema aplicado | ✅ 7 migraciones |
| Datos sintéticos sembrados | ✅ |
| VMs encendidas y servicios arriba | ✅ |
| **Recorrido E2E completo** | ✅ **61 de 61, 0 fallos, 18 s** |

El ensayo previo de C2 —contenido idéntico fila por fila, **210 s** desde el
código y **528 s** desde una copia— está en [`../C2/README.md`](../C2/README.md).

### Tres cosas que esta recreación desmintió o dejó al descubierto

**1. El nombre de la instancia se pudo reutilizar de inmediato.**
[`infra/terraform/README.md`](../../../../infra/terraform/README.md) advierte que
Google retiene el nombre de una instancia borrada **durante siete días** y que por
eso existe `db_instance_generation`. No ocurrió: `mooc-db-1` se eliminó y, menos
de una hora después, un `apply` sin esa variable la recreó con el mismo nombre y
sin error. La variable sigue siendo la salida correcta **si** algún día choca,
pero la advertencia no debe leerse como una certeza.

**2. `scripts/recrear-entorno.sh` no completó desde Windows.** Falla al preparar
su VM temporal, en `gcloud compute ssh`, con **segmentation fault** de
`plink.exe` — el cliente SSH que gcloud usa en Windows, no la VM ni la base. El
script borró su VM temporal al abortar, así que no dejó recursos huérfanos.

> **La vía alterna que sí funcionó** —y que conviene preferir, porque no crea
> nada— es usar una VM que ya está dentro de la VPC, que es el único requisito
> real: la base no tiene IP pública. Está en los **pasos 5 y 6 de §6**.

**3. La IP privada de la base escrita a mano rompió el arranque dos veces.**
Cada recreación de la instancia da una dirección nueva del rango de peering
—`10.171.240.3` → `.8` → `.10` en esta sesión—, y los dos `/etc/mooc/*.conf` la
llevan copiada. El síntoma no señala a la causa: `/api/v1/health` responde `200`
porque el *ping* a una base equivocada o vacía funciona igual, y lo que falla es
cualquier consulta real.

Es el mismo patrón de la nota 19 con `QUEUE_PRIVATE_IP`. **Lo primero que habría
que arreglar si hay una entrega siguiente:** que `prepare_web_env.sh` resuelva la
dirección con `terraform output` en el arranque, como ya resuelve los secretos con
Secret Manager, en lugar de leerla de un archivo que alguien edita a mano.

### Qué comprobar siempre tras recrear

| Comprobación | Qué debe dar |
| :--- | :--- |
| `terraform output -raw db_private_ip` | La IP **nueva**; nunca reutilizar la anterior |
| `grep -c` de la IP nueva en `~/mooc/.env`, en cada VM | `1` — confirma que `prepare_env.sh` regeneró el archivo |
| `GET /api/v1/health` | `200` con `database: up` — **necesario pero no suficiente** |
| `GET /api/v1/courses` | `200` con datos — esto sí prueba que el esquema existe |
| `make test-e2e-cloud` | **61 de 61** |

## 4. Costos: antes y después

Precios de lista `us-east1`, fecha 2026-09-25, tomados de
[`../../CONFIGURACION_Y_COSTOS.md`](../../CONFIGURACION_Y_COSTOS.md) §4.

| Escenario | USD/mes |
| :--- | ---: |
| Todo encendido 24×7 — el estado durante la entrega | **133,73** |
| Solo eliminando Cloud SQL | 82,72 |
| **Eliminando Cloud SQL y apagando las dos VMs** ← lo aplicado | **≈ 20,67** |
| Además liberando las dos IPv4 | ≈ 6,08 |

### Qué se deja de pagar

| Concepto | USD/mes |
| :--- | ---: |
| Cloud SQL — instancia de 1 vCPU / 3,75 GiB | −49,31 |
| Cloud SQL — almacenamiento SSD de 10 GiB | −1,70 |
| Cómputo de las dos VMs `e2-highcpu-2` | −72,22 |
| Egreso a internet — sin tráfico con las VMs apagadas | −0,76 |
| Sobrecosto de las IPv4 al quedar reservadas sin usar | **+10,94** |
| **Ahorro neto** | **−113,06** |

La suma cuadra: 133,73 − 113,06 = **20,67**. La línea que sube es la de las
direcciones IP, explicada abajo.

### Qué sigue costando, y por qué se conserva

| Concepto | USD/mes | Por qué no se elimina |
| :--- | ---: | :--- |
| Discos de arranque, 30 GiB × 2 | 6,00 | Un disco se paga aunque la VM esté apagada. Borrarlos obligaría a reconstruir las VMs enteras, no solo a encenderlas |
| 2 direcciones IPv4 estáticas | 14,59 | **Con las VMs apagadas cuestan más que encendidas** (3,65). Se conservan porque liberarlas cambiaría la URL, y el README del commit tagueado ya la publica |
| Buckets de Cloud Storage | 0,08 | Contienen los originales, los derivados HLS y el estado de Terraform |
| Artifact Registry, Secret Manager | marginal | Las imágenes del tag y los secretos que la recreación necesita |
| **Total residual** | **≈ 20,67** | |

### Cuánto cuesta encender el entorno para una sustentación

**No hay tarifa por crear ni por eliminar recursos**: se paga por el tiempo que
existen. Derivado de la tabla mensual de arriba:

| | USD/hora |
| :--- | ---: |
| Instancia de Cloud SQL | 0,0675 |
| Almacenamiento de la base | 0,0023 |
| Cómputo de las dos VMs | 0,0989 |
| **Todo encendido** | **0,169** · 4,05 USD/día |
| Apagado: discos, IPv4 y buckets | 0,028 · 0,68 USD/día |

Un ensayo de recreación de dos horas cuesta **unos 0,34 USD**. Recrear es barato;
lo caro es olvidarse el entorno encendido: cada día completo se come el 8 % del
crédito activo de 50 USD.

### Por qué apagar las VMs y no solo borrar la base

El enunciado solo exige eliminar la instancia de base de datos, pero advierte en
la misma sección que *«detener una instancia no implica eliminar todos sus
costos; deben revisarse también almacenamiento, respaldos y recursos de red
asociados»*. Hacer únicamente lo exigido dejaría **82,72 USD/mes** corriendo, y
el cómputo de las dos VMs es la mayor parte de eso.

La restricción que lo vuelve urgente está en
[`CONFIGURACION_Y_COSTOS.md`](../../CONFIGURACION_Y_COSTOS.md) §4: **el techo
operativo es 50 USD, no 200**, porque los cuatro cupones se redimen en secuencia.
Con el entorno completo encendido, el crédito activo dura **once días**; con lo
que queda tras este cierre, más de dos meses.

**La trampa de las IPv4 conviene leerla dos veces:** una dirección estática
reservada cuesta **más apagada que encendida**, porque Google cobra las
direcciones sin usar. Es la única línea que sube al apagar las máquinas, y aun
así se conservan: cambiar la URL rompería el enlace publicado en el commit
evaluado.

## 5. Procedimiento de eliminación

### Desde dónde se ejecuta

Lo que más se presta a confusión al recrear bajo presión es **en qué máquina va
cada comando**. Los archivos `/etc/mooc/*.conf` viven dentro de las VMs, no en el
portátil, y Terraform solo se ejecuta desde fuera.

| Comando | Dónde |
| :--- | :--- |
| `terraform apply` · `terraform output` | Tu máquina, en `infra/terraform/`, desde `main` |
| `sed` sobre `/etc/mooc/*.conf` | **Dentro de cada VM**, por SSH con túnel IAP |
| `systemctl restart` | **Dentro de cada VM** |
| `gcloud compute instances start/stop` | Tu máquina — no depende de la rama ni del estado |
| `migrate.sh` y la semilla | **Dentro de una VM de la VPC** — la base no tiene IP pública |
| `make test-e2e-cloud` | Tu máquina, en la raíz del repositorio |


| | |
| :--- | :--- |
| **Rama** | **`main`**, después de hacer merge y con `git pull` hecho |
| **Directorio** | `infra/terraform/` para los comandos de Terraform |
| **Máquina** | La tuya, con `gcloud` autenticado — **no** por SSH dentro de una VM |

**El `apply` va desde `main`, nunca desde una rama de trabajo.** Es la regla 6 de
[`infra/terraform/README.md`](../../../../infra/terraform/README.md) y la que más
daño hace al romperse: el estado de Terraform es compartido pero el código no, así
que aplicar desde una rama que no tiene el archivo de otra persona hace que
Terraform **proponga destruir sus recursos** — y el `apply` no falla al hacerlo,
funciona perfectamente.

Aquí el riesgo es concreto: si `deletion_protection = false` está mergeado en
`main` pero ejecutas desde una rama anterior, el código dice `true`, la protección
no se quita y el `destroy` falla. Y al revés, una rama desactualizada puede
proponer borrar las VMs o los buckets.


```bash
#raiz del proyecto
git checkout main
git pull --ff-only
grep -n "deletion_protection" infra/terraform/database.tf   # debe decir false
```

### Los comandos

La instancia tenía protección contra borrado, así que **`terraform destroy` a
secas falla** con `cannot destroy instance because deletion_protection is set to
true`. Por eso el `apply` va primero: no borra nada, solo aplica el
`deletion_protection = false` que ya está en `main`.

```bash
cd infra/terraform
```

```bash
#asignar db pass
export TF_VAR_db_password="$(gcloud secrets versions access latest \
  --secret=db-password --project=plataforma-mooc-entrega2 | tr -d '\r\n')"
```
```bash
# 1. Aplica el cambio ya mergeado. No borra nada.
terraform apply #Si ud aplicó el cambio deletion_protection debe decir 1 change, sino 0 changes
```
```bash
# 2. Saca el usuario del estado. NO lo borra en GCP: solo deja de rastrearlo.
#    Sin esto, el paso 3 falla -- ver la nota de abajo.
terraform state rm google_sql_user.app
```
```bash
# 3. Elimina UNICAMENTE la instancia de base de datos.
terraform destroy -target=google_sql_database_instance.main #Debe decir Destroy complete!
```
```bash
# 4. Apaga las dos VMs conservando sus discos y sus IPs.
#    Esto es gcloud, no Terraform: no depende de la rama ni del estado.
cd ../..
gcloud compute instances stop mooc-web-server mooc-worker-server \
  --zone=us-east1-b --project=plataforma-mooc-entrega2
```

> **Por qué hay que sacar el usuario del estado.** Sin el paso 2, el `destroy`
> falla a mitad:
>
> ```
> Error: failed to delete user moocuser: role "moocuser" cannot be dropped
> because some objects depend on it. Details: 20 objects in database moocdb
> ```
>
> Es PostgreSQL haciendo lo correcto: no se elimina un rol que es dueño de
> objetos. Y ese fallo **aborta la secuencia antes de llegar a la instancia**, así
> que la base sigue en pie y el cierre queda a medias.
>
> Borrar el usuario no hace falta para nada: eliminar la instancia se lleva
> dentro sus bases, sus usuarios y sus copias. `terraform state rm` **no toca
> GCP**, solo deja de rastrearlo — es lo mismo que `database.tf` ya hace con la
> base mediante `deletion_policy = "ABANDON"` (línea 222). La asimetría entre
> ambos recursos es la causa de este tropiezo, y **se repetirá en cada `destroy`**
> mientras el usuario no tenga la misma política.

> **Leer el plan antes de confirmar.** Un `destroy` sin `-target` se llevaría
> también las VMs, los buckets y la red. Lo que este cierre elimina es **solo la
> instancia de base de datos**.

### Confirmación de terminación: si va a la ruta de acceso https://34.24.52.111.sslip.io/api/v1/health, tendra un error de "tiempo tardio", lo cual indica que la infraestructura estaría accesible en esa ruta, pero que no se ha perdido la ip asignada.

## 6. Cómo recrear el entorno, paso a paso

**Este es el orden que se ejecutó y verificó**, no un plan teórico. Cada paso dice
en qué máquina va, que es lo que más se presta a confusión.

### Paso 1 — Recrear la base · *tu máquina, en la rama `main`*

```bash
git checkout main && git pull --ff-only
cd infra/terraform
export TF_VAR_db_password="$(gcloud secrets versions access latest \
  --secret=db-password --project=plataforma-mooc-entrega2 | tr -d '\r\n')"

terraform apply
```

El plan debe decir **3 to add** —instancia, base y usuario— y nada que destruir. Este paso puede tomar un tiempo.

> Si el `apply` falla por conflicto de nombre, el nombre anterior sigue
> reservado: repite con `-var='db_instance_generation=2'`. En esta recreación
> **no hizo falta** (§3).

### Paso 2 — Anotar la IP privada nueva · *tu máquina*

```bash
terraform output -raw db_private_ip
```

**Nunca reutilices la anterior.** Cada instancia recibe una dirección distinta del
rango de peering; en esta sesión fueron `10.171.240.3`, `.8` y `.10`.

### Paso 3 — Encender las VMs · *tu máquina*

```bash
gcloud compute instances start mooc-web-server mooc-worker-server \
  --zone=us-east1-b --project=plataforma-mooc-entrega2

gcloud compute instances list --project=plataforma-mooc-entrega2
```

Comprueba de paso las IPs privadas de las VMs: las asigna DHCP y podrían haber
cambiado, lo que afectaría a `QUEUE_PRIVATE_IP`.

### Paso 4 — Apuntar las VMs a la base nueva · *dentro de cada VM*


```
# ingresa a la VM desde la terminal de tu máquina
! gcloud compute ssh mooc-web-server --zone=us-east1-b --tunnel-through-iap --project=plataforma-mooc-entrega2
```

```bash
# Dentro de la VM
# remplaza con la ip obtenida del paso 2
IP_BASE=10.171.240.10
```
```bash
sudo sed -i "s/^DB_PRIVATE_IP=.*/DB_PRIVATE_IP=${IP_BASE}/" /etc/mooc/web.conf
sudo grep -E '^DB_PRIVATE_IP=|^QUEUE_PRIVATE_IP=' /etc/mooc/web.conf
```
```bash
exit
```

Ahora, lo mismo ingresa a la VM del **worker**:
```bash
# ingresa a la VM desde la terminal de tu máquina
! gcloud compute ssh mooc-worker-server --zone=us-east1-b --tunnel-through-iap --project=plataforma-mooc-entrega2
```
```bash
# Dentro de la VM
# remplaza con la ip obtenida del paso 2
IP_BASE=10.171.240.10
```
```bash
sudo sed -i "s/^DB_PRIVATE_IP=.*/DB_PRIVATE_IP=${IP_BASE}/" /etc/mooc/worker.conf
sudo grep -E '^DB_PRIVATE_IP=|^QUEUE_PRIVATE_IP=' /etc/mooc/worker.conf
```
```bash
exit
```

### Paso 5 — Copiar esquema y semilla a la VPC · *tu máquina*
La base no tiene IP pública, así que hay que migrar desde dentro. Se usa la VM
web, que ya está ahí: **no hace falta crear nada**.

```bash
# Las rutas son relativas: esto SOLO funciona desde la raiz del repositorio.
# go.mod solo existe ahi, asi que sirve de comprobacion.
ls go.mod >/dev/null 2>&1 && echo "OK: estas en la raiz" || echo "NOTA: ve a la raiz del repositorio primero"
gcloud compute scp --recurse migrations scripts/migrate.sh scripts/seeds \
  mooc-web-server:/tmp/ --zone=us-east1-b --tunnel-through-iap \
  --project=plataforma-mooc-entrega2
```

### Paso 6 — Aplicar esquema y datos · *dentro de la VM web*

**Entra primero.** Este paso NO se ejecuta en tu portátil:

```
gcloud compute ssh mooc-web-server --zone=us-east1-b --tunnel-through-iap --project=plataforma-mooc-entrega2
```

Si lo corres fuera, los síntomas son inconfundibles: `/tmp/migrations` no existe,
`sudo: command not found` y Python abre el alias de Microsoft Store.

```bash
# Centinela: avisa si te equivocaste de maquina. No cierra la sesion.
[ "$(hostname)" = "mooc-web-server" ] && echo "OK: estas en la VM web" || echo "OJO: esto va DENTRO de la VM web"

mkdir -p ~/mig/scripts && cp -r /tmp/migrations /tmp/seeds ~/mig/
cp /tmp/migrate.sh ~/mig/scripts/
command -v psql >/dev/null || sudo apt-get install -y -qq postgresql-client

cd ~/mig
export PGPASSWORD=$(gcloud secrets versions access latest \
  --secret=db-password --project=plataforma-mooc-entrega2 | tr -d '\r\n')
ENC=$(python3 -c "import urllib.parse,sys;print(urllib.parse.quote(sys.argv[1],safe=''))" "$PGPASSWORD")
```
```bash
# la del paso 2
IP_BASE=10.171.240.10
```
```bash
export DATABASE_URL="postgres://moocuser:${ENC}@${IP_BASE}:5432/moocdb?sslmode=require"

bash ./scripts/migrate.sh
psql -h "${IP_BASE}" -U moocuser -d moocdb -v ON_ERROR_STOP=1 -f seeds/synthetic_data.sql
```

Deben aplicarse **7 migraciones** y la semilla terminar sin error.

### Paso 7 — Levantar la aplicación · *dentro de cada VM*

#### En **la VM web** donde ya estas:
```bash
sudo systemctl restart mooc-web.service
sudo grep -c 10.171.240.10 /home/dfortizr1/mooc/.env    # Remplaza la ip del paso 2; debe dar 1
```
```bash
sudo docker ps --format 'table {{.Names}}	{{.Status}}'
```

El `grep -c` en **1** es lo que confirma que `prepare_env.sh` regeneró el `.env`
leyendo el `.conf` del paso 4. Si da `0`, el `sed` no se aplicó antes del
reinicio.

Resultado esperado:
NAMES          STATUS
mooc-api-1     Up 15 seconds (healthy)
mooc-proxy-1   Up 10 seconds
mooc-redis-1   Up 36 minutes (healthy)

```bash
exit
```

Ahora lo mismo con "web worker". Ingresa a la VM de **worker server**:
```bash
# ingresa a la VM desde la terminal de tu máquina
! gcloud compute ssh mooc-worker-server --zone=us-east1-b --tunnel-through-iap --project=plataforma-mooc-entrega2
```
```bash
sudo systemctl restart mooc-worker.service
sudo grep -c 10.171.240.10 /home/dfortizr1/mooc/.env    # Remplaza la ip del paso 2; debe dar 1
```
```bash
sudo docker ps --format 'table {{.Names}}	{{.Status}}'
```
Resultado esperado:
NAMES         STATUS
mooc-worker   Up 17 seconds
mooc-queue    Up 23 seconds (healthy)

```bash
exit
```

### Paso 8 — Verificar · *tu máquina*

```bash
curl -s https://34.24.52.111.sslip.io/api/v1/health # 200, database: up
```
```bash
curl -s https://34.24.52.111.sslip.io/api/v1/courses # 200 CON datos
```
```bash
make test-e2e-cloud  # 61 de 61
```

**`health` no basta.** Responde `200` contra una base vacía, porque solo hace
*ping*. Lo que prueba que el esquema existe es que el catálogo devuelva cursos.

---

## 7. Restaurar la protección contra borrado

`deletion_protection` está hoy en **`false`** en `database.tf`, porque sin eso
`terraform destroy` no puede eliminar la instancia. Ese estado **no debe quedarse
así**: mientras lo esté, cualquier `destroy` —incluido uno accidental sin
`-target`— se lleva la base sin resistencia.

**El orden importa.** Si queda pendiente una eliminación definitiva, se hace
primero y se restaura la protección después, en el commit de cierre:

```
eliminar la instancia → deletion_protection = true → commit → merge a main
```

Así se ahorra un `apply` extra y, sobre todo, **el repositorio queda declarando la
postura segura** para quien recree el entorno más adelante desde este código.

## 8. Registro de la ejecución

| | |
| :--- | :--- |
| Fecha de eliminación | *(pendiente)* |
| Recreación verificada, 1.ª vez | 2026-09-28 · `mooc-db-1` en `10.171.240.10` · E2E **61/61** |
| Ciclo completo reproducido con el runbook | 2026-09-28 · eliminación y recreación siguiendo §5 y §6 · `mooc-db-1` en `10.171.240.12` · E2E **61/61** |
| `deletion_protection` restaurado a `true` | *(pendiente)* |
| Generación de la instancia eliminada | `mooc-db-1` |
| Copias existentes al eliminar | 3 — una automática y dos bajo demanda, borradas con la instancia |
| VMs apagadas | *(pendiente)* |
| Costo residual verificado | *(pendiente)* |

---

Este archivo no contiene credenciales, llaves ni secretos.
