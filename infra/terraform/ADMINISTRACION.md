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

Después, dar acceso a los otros tres integrantes. Son **cuatro roles por
persona**, y ninguno sobra:

```bash
CORREO="companero@gmail.com"

for ROL in \
  roles/editor \
  roles/resourcemanager.projectIamAdmin \
  roles/iam.serviceAccountAdmin \
  roles/storage.admin
do
  gcloud projects add-iam-policy-binding plataforma-mooc-entrega2 \
    --member="user:${CORREO}" --role="${ROL}" --condition=None
done
```

| Rol | Para qué |
| :--- | :--- |
| `editor` | Crear la mayoría de los recursos y habilitar APIs |
| `resourcemanager.projectIamAdmin` | Asignar roles a las cuentas de servicio |
| `iam.serviceAccountAdmin` | Crear las cuentas de servicio y editar sus políticas |
| `storage.admin` | Leer y escribir el estado en el bucket, y más adelante el IAM del bucket de la aplicación (C3) |

**`roles/editor` no alcanza por sí solo**, y esto sorprende: editor permite
crear casi cualquier recurso pero **no gestionar políticas de IAM**. Sin
`projectIamAdmin`, el `terraform apply` de un compañero avanza hasta los
`google_project_iam_member` y falla ahí por permisos.

La alternativa contundente es `roles/owner`, que lo cubre todo de una vez. Es
defendible en un equipo de cuatro que ya comparte un proyecto de clase; los
cuatro roles de arriba son la versión acotada.

Conviene además darles **`roles/billing.costsManager` sobre la cuenta de
facturación**, para que las alertas de presupuesto de B1 les lleguen también a
ellos y no solo a quien la creó:

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
  roles/storage.admin
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
