# Administración del proyecto de GCP

Tareas que se hacen **una sola vez y por una sola persona**: la que administra
el proyecto y la cuenta de facturación.

**Si vas a trabajar con la infraestructura pero no a administrarla, este
documento no es para ti** — ve a [`README.md`](README.md). Estos comandos
fallarán por permisos si no eres administrador, y no necesitas ejecutarlos:
basta con que alguien te haya dado acceso.

---

## Puesta a punto del equipo, una sola vez

Lo hace **una persona**; los demás no repiten nada de esto.

```bash
./scripts/bootstrap-tfstate.sh
```

Crea el bucket del estado con versionado y sin posibilidad de hacerse público.
Es el único recurso que Terraform no puede crear solo: necesita un sitio donde
guardar el estado antes de tener estado.

Después, dar acceso a los otros tres integrantes. Son **ocho roles por
persona**, y esa cifra es la lección de este issue: `roles/editor` parece
bastarse solo y no se basta.

```bash
CORREO="companero@gmail.com"

for ROL in \
  roles/editor \
  roles/resourcemanager.projectIamAdmin \
  roles/iam.serviceAccountAdmin \
  roles/iam.serviceAccountUser \
  roles/storage.admin \
  roles/compute.networkAdmin \
  roles/servicenetworking.networksAdmin \
  roles/artifactregistry.admin
do
  gcloud projects add-iam-policy-binding plataforma-mooc-entrega2 \
    --member="user:${CORREO}" --role="${ROL}" --condition=None
done
```

| Rol | Sin él falla |
| :--- | :--- |
| `editor` | Crear la mayoría de recursos y habilitar APIs |
| `resourcemanager.projectIamAdmin` | Asignar roles a las cuentas de servicio (B2) |
| `iam.serviceAccountAdmin` | Crear las cuentas de servicio y editar sus políticas (B2) |
| `iam.serviceAccountUser` | **Adjuntar** una cuenta de servicio a una VM (D2, E1) |
| `storage.admin` | El bucket del estado, y el IAM del bucket de la aplicación (C3) |
| `compute.networkAdmin` | Reservar el rango de direcciones de la conexión privada (C1) |
| `servicenetworking.networksAdmin` | Crear el *peering* con Cloud SQL (C1) |
| `artifactregistry.admin` | Asignar permisos IAM en repositorios de Artifact Registry (`google_artifact_registry_repository_iam_member`) |

**Por qué ocho y no uno.** `roles/editor` permite crear casi cualquier recurso
pero **no gestiona IAM de proyecto/recursos ni redes de servicio**. Eso se descubrió por
las malas, siempre igual: alguien a mitad de un `terraform apply`, un error de
permisos, y el trabajo detenido hasta que quien administra estuviera disponible.
Concederlos todos de entrada evita esa ronda.

### Sobre conceder Owner

Sería más simple que mantener siete roles, pero **no se puede hacer por línea de
comandos en este proyecto**:

```
INVALID_ARGUMENT: SOLO_MUST_INVITE_OWNERS
```

Los proyectos **sin organización** —como el nuestro— obligan a invitar a los
propietarios desde la consola web, y la persona invitada debe **aceptar la
invitación por correo**. No es una limitación de permisos de quien la concede,
es una protección de la plataforma.

Si se decide hacerlo: Consola → **IAM y administración** → **IAM** → *Otorgar
acceso* → el correo, rol **Propietario** → y que cada uno acepte el correo que
recibe. Los siete roles de arriba quedarían redundantes y se pueden retirar.

Ten en cuenta lo que implica: un propietario puede borrar el proyecto, cambiar
la facturación y quitarle el acceso a los demás. En un equipo de cuatro que ya
comparte un proyecto de clase es asumible, pero es una decisión, no un trámite.

Conviene además darles **`roles/billing.costsManager` sobre la cuenta de
facturación**, para que las alertas de presupuesto configuradas en B1 les
lleguen también a ellos y no solo a quien la creó:

```bash
gcloud billing accounts add-iam-policy-binding CUENTA_DE_FACTURACION \
  --member="user:${CORREO}" --role="roles/billing.costsManager"
```

---

---

## Dar de alta a alguien nuevo

Si entra un integrante más adelante, son los mismos dos bloques de arriba —los
cuatro roles del proyecto y el de facturación— más pasarle la contraseña de la
base por un gestor de contraseñas. Nada más: no hay que regenerar credenciales
ni tocar el estado.

## Dar de baja a alguien

```bash
CORREO="quien-se-va@gmail.com"

for ROL in \
  roles/editor \
  roles/resourcemanager.projectIamAdmin \
  roles/iam.serviceAccountAdmin \
  roles/iam.serviceAccountUser \
  roles/storage.admin \
  roles/compute.networkAdmin \
  roles/servicenetworking.networksAdmin
do
  gcloud projects remove-iam-policy-binding plataforma-mooc-entrega2 \
    --member="user:${CORREO}" --role="${ROL}"
done
```

Aquí se nota la ventaja de no usar archivos de llave: quitar el acceso es
revocar unos roles. Con llaves de cuenta de servicio habría que rotarlas y
averiguar quién tiene copias.

## La contraseña de la base

Se guarda en **Secret Manager, en el mismo proyecto**. No en un gestor externo,
y desde luego no en el repositorio ni por chat.

La razón no es solo comodidad: si cada integrante guardara su propia copia,
bastaría una errata para que dos personas tuvieran valores distintos, y a partir
de C1 cada `apply` cambiaría la contraseña de la base por la de quien lo
ejecutó. Leyéndola de una única fuente, eso no puede pasar.

Ventaja adicional sobre cualquier herramienta externa: no hay cuentas nuevas
que crear, el control de acceso es el IAM que ya se repartió, y queda registro
de quién la leyó.

### Crearla, una vez

Requiere que `terraform apply` haya habilitado `secretmanager.googleapis.com`.

```bash
# Generar y guardar en un solo paso, sin que pase por el historial del shell
openssl rand -base64 24 | tr -d '\n' | gcloud secrets create db-password \
  --project=plataforma-mooc-entrega2 \
  --replication-policy=user-managed --locations=us-east1 \
  --data-file=-
```

El `tr -d` quita el salto de línea final: sin él, la contraseña llevaría un
carácter invisible al final y las conexiones fallarían por una razón muy difícil
de ver.

Comprueba que quedó bien:

```bash
gcloud secrets versions access latest --secret=db-password \
  --project=plataforma-mooc-entrega2
```

### Dar acceso al equipo

```bash
CORREO="companero@gmail.com"

gcloud secrets add-iam-policy-binding db-password \
  --project=plataforma-mooc-entrega2 \
  --member="user:${CORREO}" \
  --role="roles/secretmanager.secretAccessor"
```

### Rotarla

Si alguna vez hace falta, se añade una versión nueva; las anteriores quedan
archivadas:

```bash
openssl rand -base64 24 | tr -d '\n' | gcloud secrets versions add db-password \
  --project=plataforma-mooc-entrega2 --data-file=-
```

Ojo: **rotarla no cambia la contraseña de la base**. Hay que volver a aplicar
Terraform para que el cambio llegue a PostgreSQL, y avisar al equipo, porque la
aplicación desplegada seguirá usando la anterior hasta que se reinicie.

### Y una advertencia que no cambia

El estado de Terraform **contiene la contraseña en claro**. Por eso el bucket se
creó con prevención de acceso público y solo el equipo tiene permiso sobre él.
Secret Manager es de dónde se lee, no un sustituto de esa precaución.

---

## Inventario de configuración sensible

Issue **B4** (#117). Qué es secreto, dónde vive en cada entorno y cómo se rota.

| Variable | En local | En la nube |
| :--- | :--- | :--- |
| `POSTGRES_PASSWORD` · `DATABASE_URL` | Valor por defecto de `docker-compose.yml` | **Secret Manager** → `TF_VAR_db_password` y el Compose de la VM |
| `MINIO_ROOT_PASSWORD` · `S3_ACCESS_KEY` · `S3_SECRET_KEY` | Valor por defecto de `docker-compose.yml` | **No existen.** Con `STORAGE_BACKEND=gcs` las credenciales vienen de la cuenta de servicio adjunta a la VM |
| `SMTP_PASSWORD` | Vacía: Mailpit no autentica | **Secret Manager** → `smtp-password`, leído por `sa-web-server` (issue #126) |
| `SMTP_USERNAME` | Vacía | **Variable normal.** No es un secreto: en SendGrid es la cadena literal `apikey` y en Brevo el correo de la cuenta |
| `SMTP_FROM` | Remitente de mentira | **Variable normal.** Debe ser el remitente verificado en el proveedor |
| Credenciales de Terraform | — | **No existen.** Cada integrante usa su identidad personal (ADC) |
| Credenciales de las VMs | — | **No existen.** Cuenta de servicio adjunta, sin archivo de llave |

**Tres de las cinco filas dicen «no existen», y es el resultado que más vale.**
Un secreto que no existe no se filtra, no caduca y no hay que rotarlo. Se
consiguió eligiendo identidades adjuntas en lugar de archivos de llave, tanto
para Terraform como para las VMs.

### Por qué `docker-compose.yml` sí lleva contraseñas

`moocpassword` y `minioadmin` están en el repositorio, en los valores por
defecto del Compose, y eso es deliberado: **no protegen nada**. Son las
credenciales de unos contenedores que solo existen en la máquina de quien los
levanta, y tenerlas ahí es lo que permite clonar el repositorio y arrancar sin
configurar nada.

Lo que las hace inofensivas no es la promesa de que nadie las use en la nube,
sino que **el arranque en producción las rechaza**:
`internal/config/validate.go` comprueba con `APP_ENV=production` que la cadena
de conexión no las conserve, que el almacenamiento no sea MinIO, que
`APP_BASE_URL` no apunte a localhost y que el correo no salga por Mailpit ni
por el puerto 25. Si alguna sobrevive a un despliegue, el proceso no arranca y
dice cuál.

### Rotar la contraseña de la base

1. Añadir una versión nueva al secreto (ver [Rotarla](#rotarla) más arriba).
2. `terraform apply` desde `main`, para que el cambio llegue a PostgreSQL.
3. **Reiniciar la API y el worker**, o seguirán usando la anterior: la leyeron
   al arrancar.
4. Avisar al equipo: quien tenga una terminal abierta debe volver a exportar
   `TF_VAR_db_password`.

El orden importa. Rotar el secreto sin aplicar deja la base con la contraseña
vieja y al equipo con la nueva; aplicar sin reiniciar deja los procesos con la
vieja contra una base que ya cambió.

### Rotar las credenciales de SMTP

Las emite el proveedor, así que la rotación empieza allí. Después:

```bash
gcloud secrets versions add smtp-password \
  --project=plataforma-mooc-entrega2 --data-file=-
```

Y reiniciar la API, que es quien envía correo. El worker no lo necesita.

### Si una credencial se filtra

Rotar primero, investigar después. Una versión nueva en Secret Manager y un
`terraform apply` tardan minutos; averiguar quién vio qué, no.

Si lo filtrado fue **el estado de Terraform** —que contiene la contraseña de la
base en claro—, hay que rotarla aunque el bucket vuelva a estar privado: el
estado la lleva dentro, y un objeto que estuvo expuesto se considera expuesto.

---

## Cierre de la entrega (issue #133, I6)

El enunciado **exige eliminar la instancia de base de datos administrada** al
cerrar la entrega. Con la protección de borrado que puso C1, un `terraform
destroy` a secas falla, y eso es intencionado: la protección está ahí para el
resto del semestre, no para este momento.

El orden importa, porque los tres pasos no se pueden juntar en uno:

**1. Guardar lo que haga falta conservar.** Una vez borrada la instancia, sus
copias de seguridad se van con ella.

```bash
gcloud sql export sql mooc-db-1 gs://<bucket>/respaldo-final.sql \
  --database=moocdb --project=plataforma-mooc-entrega2
```

**2. Quitar la protección en el código y aplicar.** En `database.tf`,
`deletion_protection = false`. Se hace por código y no con `gcloud`, porque de
otro modo el siguiente `plan` propondría volver a activarla.

```bash
terraform apply     # solo quita la protección; no borra nada
```

**3. Destruir.**

```bash
terraform destroy
```

### Dos cosas que sorprenden al recrear

**El nombre queda reservado siete días.** Google retiene el nombre de una
instancia borrada, así que un `apply` posterior falla con un conflicto de nombre
que no se puede forzar. La salida está prevista: subir
`db_instance_generation` en `database.tf` o pasarlo por línea de comandos.

```bash
terraform apply -var='db_instance_generation=2'
```

**La instancia nueva viene vacía.** Terraform crea la base y el usuario, nunca
el esquema. Hay que volver a migrar desde una VM, con
`scripts/migrate.sh` — el procedimiento está en
[`README.md`](./README.md#las-migraciones-no-las-hace-terraform).

### Qué sobrevive al `destroy`, y cuánto cuesta

Medido el 2026-09-27 (issue **C2**, #119). Los tamaños son reales, leídos con
`gcloud`; los precios son los de lista de `us-east1` y hay que contrastarlos con
el informe de facturación.

| Recurso | Tamaño medido | Coste/mes | Por qué se conserva |
| :--- | :--- | :--- | :--- |
| Bucket del estado de Terraform | 79,77 KiB | ~0,00 USD | Se creó fuera de Terraform; si `destroy` se lo llevara, se llevaría el registro de lo que hay que reconstruir |
| Bucket de multimedia (C3) | 4 B (solo los prefijos) | ~0,00 USD | Lo gestiona `storage.tf`; con datos reales sí crecería |
| Artifact Registry (D1) | 106,39 MB | **0,00 USD** | Por debajo del medio gigabyte gratuito. Conservar las imágenes es lo que hace rápida la recreación |
| Secret Manager | 2 secretos, 2 versiones activas | ~0,12 USD | Contraseñas de base de datos y SMTP; sus valores no pasan por Terraform |
| VPC, subred, firewall, rango reservado | — | 0,00 USD | No se factura su existencia |
| Cloud NAT | — | ~0,00 USD **sin VMs** | Se factura por instancia-hora y por datos; con cero VMs no cobra |
| APIs habilitadas | — | 0,00 USD | Habilitar una API no cuesta; solo los recursos que se creen con ella |

**Total conservado: del orden de 0,12 USD al mes.**

La conclusión es robusta aunque los precios unitarios varíen, porque **todas las
cantidades están órdenes de magnitud por debajo de los umbrales de pago**: 106 MB
frente a los 500 MB gratuitos del registro, kilobytes frente a gigabytes en los
buckets. Lo único que factura de verdad es la instancia de base de datos, y es
precisamente lo que se elimina.

### Lo que NO sobrevive, y hay que tener en cuenta

| | |
| :--- | :--- |
| **Las copias de seguridad** | Se van con la instancia. Si hace falta conservar los datos, exportar antes a un bucket |
| El esquema y los datos | Terraform recrea la base vacía, nunca el esquema. Hay que volver a migrar y sembrar |
| El nombre de la instancia | Google lo reserva **siete días**. Subir `db_instance_generation` |

El primero es el que sorprende: una copia de seguridad no es un respaldo si vive
dentro de lo que vas a borrar.

---

## Correo transaccional en la nube

Issue **F1** (#126). El proveedor elegido es **Brevo SMTP**, plan gratuito:
`smtp-relay.brevo.com:587` con STARTTLS. Permite 300 envíos diarios, suficiente
para las pruebas funcionales. El equipo no comprará dominio, por lo que se usa
un remitente individual verificado con el código que Brevo envía a esa dirección.
Brevo puede reemplazar visualmente remitentes de dominios gratuitos; es una
limitación de entrega aceptada, no afecta los enlaces de la plataforma.

Referencias del proveedor:

- <https://help.brevo.com/hc/en-us/articles/7924908994450>
- <https://help.brevo.com/hc/en-us/articles/208836149>
- <https://help.brevo.com/hc/en-us/articles/10905415650322>

### 1. Configurar Brevo

1. Crear la cuenta gratuita.
2. En **Settings → Senders, Domains & Dedicated IPs → Senders**, añadir el correo
   remitente y verificar el código de seis dígitos.
3. En **Settings → SMTP & API**, copiar el **SMTP login** y crear una **SMTP
   key**. La clave se muestra una sola vez. No usar una API key.

### 2. Crear el contenedor con Terraform

`mail.tf` crea `smtp-password` y concede lectura únicamente a
`sa-web-server`. Se mezcla primero y se ejecuta `terraform apply` desde
`main`. Las altas propias de F1 son **2 to add**: el secreto vacío y su permiso
IAM. En el estado actual también aparece **1 to change** sobre
`google_sql_user.app`: elimina una marca CR que Git Bash dejó en el estado y
reconcilia la contraseña con la versión de Secret Manager. Es una corrección
única; el plan debe mantener **0 to destroy**.

### 3. Añadir el valor sin guardarlo en disco

```bash
gcloud secrets versions add smtp-password \
  --project=plataforma-mooc-entrega2 --data-file=-
# Pegar la SMTP key, luego Ctrl+D (Ctrl+Z y Enter en Windows).
```

Con `--data-file=-` la clave no pasa por el historial ni por Terraform. El
login SMTP no es la clave: Brevo muestra un correo técnico que puede terminar en
`@smtp-brevo.com`.

### 4. Configurar la VM sin datos personales en Git

Las dos direcciones privadas **se leen de Terraform, no se copian de aquí**. Una
VM recreada recibe otra IP, y un literal en este documento envejece sin avisar:
las VMs ya se recrearon una vez y pasaron de `10.0.1.2`/`10.0.1.3` a
`10.0.1.4`/`10.0.1.5`. Desde tu máquina, antes de entrar a la VM:

```bash
cd infra/terraform
terraform output -raw db_private_ip              # DB_PRIVATE_IP
terraform output -raw worker_server_private_ip   # QUEUE_PRIVATE_IP
```

Desde tu máquina, **no desde la VM**: `sa-web-server` no tiene
`compute.instances.get` sobre la instancia del worker, y no hay que concedérselo
—esa separación es justo lo que E1 construyó—. Preguntar por la IP allí dentro
falla con `Required 'compute.instances.get' permission`.

En `mooc-web-server`, crear el archivo de configuración operativa:

```bash
sudo install -d -m 0755 /etc/mooc
sudo tee /etc/mooc/web.conf >/dev/null <<'EOF'
IMAGE_TAG=<SHA_COMPLETO_DE_LA_IMAGEN_PUBLICADA>
APP_DOMAIN=34.24.52.111.sslip.io
DB_PRIVATE_IP=<terraform output -raw db_private_ip>
QUEUE_PRIVATE_IP=<terraform output -raw worker_server_private_ip>
SMTP_HOST=smtp-relay.brevo.com
SMTP_PORT=587
SMTP_FROM=<CORREO_REMITENTE_VERIFICADO>
SMTP_USERNAME=<LOGIN_SMTP_DE_BREVO>
EOF
sudo chown root:dfortizr1 /etc/mooc/web.conf
sudo chmod 640 /etc/mooc/web.conf
```

Ese archivo no contiene contraseñas, pero queda fuera del repositorio porque el
remitente y el login identifican la cuenta del equipo.

**`QUEUE_PRIVATE_IP` es la IP privada del Worker Server, no la del propio Web
Server.** Ahí viven la cola de asynq y el almacén de sesiones, límites e
idempotencia (E1). Si falta, `prepare_web_env.sh` detiene el despliegue a
propósito: la alternativa —apuntar al contenedor `redis` del propio Compose— deja
a la API encolando tareas de media en un broker que ningún worker lee, y eso no
falla, solo deja cada video en `pending` para siempre. Es el defecto #166, que
encontró la verificación E2E de G3.

El script versionado sustituye el `prepare_env.sh` manual de D2. Lee
`db-password` y `smtp-password` con la cuenta de servicio de la VM, escribe
`.env` con modo 600 y no imprime los valores:

```bash
cd ~/mooc
chmod +x scripts/prepare_web_env.sh
ln -sfn scripts/prepare_web_env.sh prepare_env.sh
./prepare_env.sh
sudo systemctl restart mooc-web.service
sudo systemctl --no-pager status mooc-web.service
```

No reiniciar antes de crear la versión de `smtp-password`: la validación de
producción detendrá la API si falta una credencial, un remitente o un puerto
admitido.

### Plan alterno si SMTP saliente falla

1. Probar 587 desde la VM: `nc -vz smtp-relay.brevo.com 587`.
2. Si la red lo bloquea, cambiar `SMTP_PORT` a **465** (TLS desde el primer
   byte). El cliente ya implementa ese protocolo.
3. Si también falla, usar **2525** con STARTTLS, puerto alterno soportado por
   Brevo.
4. Si los tres puertos estuvieran bloqueados, la alternativa es implementar el
   adaptador de API transaccional de Brevo sobre HTTPS/443. Hasta hacerlo se
   documenta como limitación y no se declara la prueba F1 como aprobada.

### Rotar la credencial

Crear una SMTP key nueva en Brevo, añadirla como versión de `smtp-password`,
reiniciar solo la API y comprobar un correo. Revocar la clave anterior después
de esa prueba. El worker no usa SMTP.
