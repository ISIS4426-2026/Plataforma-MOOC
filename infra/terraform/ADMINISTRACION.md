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

Se comparte por un gestor de contraseñas, **nunca por el repositorio ni por
chat**. El issue **B4** (#117) la moverá a Secret Manager; hasta entonces se
gestiona fuera de banda.

Recuerda que el estado de Terraform **la contiene en claro**. Por eso el bucket
se creó con prevención de acceso público y solo el equipo tiene permiso sobre
él.
