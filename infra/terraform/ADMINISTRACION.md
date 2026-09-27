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

Después, dar acceso a los otros tres integrantes. Son **siete roles por
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
  roles/servicenetworking.networksAdmin
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

**Por qué siete y no uno.** `roles/editor` permite crear casi cualquier recurso
pero **no gestiona ni IAM ni redes de servicio**. Eso se descubrió tres veces por
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
| `SMTP_USERNAME` · `SMTP_PASSWORD` | Vacías: Mailpit no autentica | **Secret Manager**, credenciales del proveedor (issue #126) |
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

### Qué sobrevive al `destroy`

| | |
| :--- | :--- |
| El bucket del estado de Terraform | Se creó fuera de Terraform, a propósito |
| Los secretos de Secret Manager | Los gestiona este documento, no el código |
| Las APIs habilitadas | `disable_on_destroy = false`; apagarlas no ahorra nada |

Nada de eso cuesta dinero apreciable, y conservarlo es lo que permite volver a
levantar el entorno para la sustentación.
