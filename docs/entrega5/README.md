# Entrega 5 — Funciones serverless e infraestructura como código

Se migró una tarea del worker a una **Cloud Run function de 2.ª generación** disparada por Pub/Sub, y todo el despliegue —servicios
de Cloud Run, topics, suscripciones, función, cuentas de servicio y permisos— se gestiona con **Terraform** (`init` → `plan` →
`apply`).

| Documento | Contenido |
| :--- | :--- |
| [`ANALISIS_SERVERLESS.md`](ANALISIS_SERVERLESS.md) | Qué ya es serverless, siete oportunidades (beneficio y límites), por qué se eligió esta tarea y el **diagrama** actualizado |
| Este documento | La migración, el despliegue con Terraform y su guía reproducible, la evidencia |
| [`GUION_VIDEO.md`](GUION_VIDEO.md) | Guion del video de sustentación |
| [`../entrega4/README.md`](../entrega4/README.md) | La base sobre la que se construye (Cloud Run y Pub/Sub) |

## 1. Qué pide la entrega y dónde está

| Requisito | Dónde |
| :--- | :--- |
| Análisis: componentes serverless, oportunidades con servicio, beneficio y límites | `ANALISIS_SERVERLESS.md` §1–§2 |
| Migrar ≥ 1 tarea a una Cloud Function 2.ª gen activada por Pub/Sub, justificada | §2 y `ANALISIS_SERVERLESS.md` §3 |
| Conservar el comportamiento y el resultado | §2.2 |
| El worker anterior deja de ejecutar la tarea | §2.3 |
| Reintentos sin duplicar efectos | §2.4 |
| Terraform gestiona Cloud Run, Pub/Sub, función, cuentas, permisos y secretos | §3 |
| Entorno parametrizado, prerrequisitos, imágenes y código, recursos existentes | §3.2–§3.5 |
| `init` / `plan` / `apply` para desplegar **y actualizar** | §4 y §6 |
| Sin credenciales, secretos ni estado en el repositorio | §3.6 |
| Evidencia del despliegue y de una tarea real de Pub/Sub a la función | §6 |

## 2. La migración

### 2.1 Qué tarea y por qué

La **validación de documentos** (`malware_scan`): PDF, presentaciones, archivos descargables e imágenes. Comprueba que el objeto está en
el bucket, que su tamaño es válido y que su tipo coincide con su extensión, y deja el recurso `completed` en Cloud SQL. Dura **~50 ms**,
no tiene estado en disco y no usa FFmpeg. El video se queda en el worker. La comparación completa está en
`ANALISIS_SERVERLESS.md` §3.

```text
ANTES (Entrega 4)   backend ──► topic mooc-media-jobs ──► push ──► mooc-workers   (video Y documentos)
AHORA (Entrega 5)   backend ──► topic mooc-media-jobs ──► push ──► mooc-workers   (solo video)
                    backend ──► topic mooc-media-documents ──► Eventarc ──► mooc-doccheck   (documentos)
```

### 2.2 El comportamiento y el resultado se conservan

La función **no reimplementa** la tarea: ejecuta el mismo `handler.MediaProcessor` que corría el worker (`doccheck_function.go` →
`internal/worker/eventfn` → `MediaProcessor.ProcessPayload`). Las mismas validaciones, las mismas transiciones del recurso
(`processing` → `completed` / `failed`), la misma entrada de auditoría y los mismos fallos permanentes. Lo que cambia es **quién la
invoca**, no lo que hace.

### 2.3 El worker deja de ejecutarla

Dos cambios, uno por cada lado:

- **Quien publica enruta.** El backend manda las tareas `malware_scan` al topic de documentos (`PUBSUB_DOCUMENTS_TOPIC`) y el resto al
  de siempre. El worker ya no recibe documentos por el camino normal.
- **El worker se niega.** Con `MEDIA_DOCUMENTS_IN_FUNCTION=true`, si le llegara un documento por error responde un error **no
  permanente** *antes* de tocar el recurso: el mensaje se reentrega hasta la cola de mensajes muertos, donde queda visible, en vez de
  ejecutarse una segunda vez o perderse.

El enrutamiento está en quien publica y no en filtros de quien consume: así «cada tarea tiene un solo dueño» es una propiedad de la
configuración y no depende de que cada consumidor filtre bien.

### 2.4 Reintentos sin duplicar efectos

La función termina de una de tres maneras (`internal/worker/eventfn`), y el disparador está configurado con
`RETRY_POLICY_RETRY`:

| Resultado de la tarea | La función devuelve | Efecto |
| :--- | :--- | :--- |
| Terminó bien | `nil` | el evento se consume |
| Falló **para siempre** (documento corrupto, recurso borrado) | `nil`, tras dejar el recurso en `failed` | se consume: reintentar repetiría el mismo fallo |
| Falló **transitoriamente** (base de datos, red) | un error | Eventarc reentrega con espera exponencial |

**Sin efectos duplicados.** El efecto de la tarea es una transición de estado, y es idempotente: si el recurso ya está `completed` sobre
ese mismo objeto, el procesador lo omite (`media job skipped; this object is already processed`). Una reentrega, o el mismo mensaje
publicado dos veces, no cambia nada. Pub/Sub entrega *al menos una vez*, así que esto no es opcional.

## 3. Terraform

### 3.1 Qué gestiona

Todo lo del alcance del enunciado está en `infra/terraform/`:

| Recurso | Archivo |
| :--- | :--- |
| Servicios de Cloud Run (frontend, backend, workers): imagen, CPU, memoria, concurrencia, timeout, escalamiento (máx. 3), red, variables y secretos | `cloudrun.tf` |
| Topics (`mooc-media-jobs`, `mooc-media-documents`, mensajes muertos), suscripción push autenticada y sus permisos | `cloudrun.tf`, `functions.tf` |
| **Función** `mooc-doccheck`, su **código desplegable** (zip determinista + bucket) y su **disparador de eventos** (Eventarc) | `functions.tf` |
| Cuentas de servicio, los permisos de cada una y las referencias a secretos | `cloudrun.tf`, `functions.tf` |
| Subred de Cloud Run, regla de firewall hacia Redis | `cloudrun.tf` |
| APIs necesarias (`run`, `pubsub`, `cloudfunctions`, `eventarc`) | `cloudrun.tf`, `functions.tf` |
| Encender/apagar la VM de Redis y la base de datos | `queue.tf` (`queue_vm_status`), `database.tf` (`db_activation_policy`) |

El **código de la función** lo empaqueta Terraform: `data "archive_file"` arma un zip con `go.mod`, `go.sum`, el paquete de la raíz
y `internal/**` sin las pruebas; el zip se sube a un bucket con el **hash en el nombre**, así que cualquier cambio de código cambia el
objeto y la función se redespliega sola, y un `plan` sin cambios no toca nada.

### 3.2 Parámetros

Nada del entorno está escrito en los `.tf`: es todo variable, con valor por omisión salvo lo que no se debe publicar.

| Variable | Para qué | Valor |
| :--- | :--- | :--- |
| `project_id`, `region`, `zone` | proyecto y ubicación | `plataforma-mooc-entrega2`, `us-east1`, … (por omisión) |
| `cloudrun_enabled` | crea Cloud Run y Pub/Sub | `false` por omisión |
| `docfunction_enabled` | crea la función, su topic y su disparador | `false` por omisión; exige `cloudrun_enabled` |
| `cloudrun_image_tag` | SHA de las imágenes `api`, `worker` y `web` | actualizar esto **actualiza los servicios** |
| `cloudrun_services` | CPU, memoria, concurrencia, timeout, mín. y máx. de cada servicio | ver `cloudrun.tf` |
| `docfunction_limits`, `function_runtime` | dimensionamiento y runtime de la función | 256 MiB, 1 vCPU, máx. 3, `go124` |
| `db_activation_policy`, `queue_vm_status` | encender o apagar la base y Redis | `ALWAYS` / `RUNNING` |
| `TF_VAR_db_password` | contraseña de la base | **secreto**: se toma de Secret Manager al aplicar, no se escribe en ningún archivo |
| `TF_VAR_smtp_from`, `TF_VAR_smtp_username` | cuenta de correo | no son secretos, pero tampoco se publican |
| `TF_VAR_image_tag` | tag de los grupos de la Entrega 3 | **no cambiar** (recrearía esas VMs) |

### 3.3 Prerrequisitos

- `terraform` 1.16.x (fijado en `versions.tf`), `gcloud`, y `gcloud auth login` + `gcloud auth application-default login`.
- El **bucket del estado** (`plataforma-mooc-entrega2-tfstate`) y el secreto `db-password` ya existen: se crean una vez, antes de
  Terraform, porque Terraform no puede guardar su propio estado en un bucket que aún no tiene (ver `infra/terraform/ADMINISTRACION.md`).
- Quien aplica necesita, además de edición del proyecto, poder cambiar permisos de los recursos nuevos: `roles/secretmanager.admin`,
  `roles/pubsub.admin`, `roles/run.admin`, `roles/cloudfunctions.admin` y `roles/eventarc.admin` (el rol `editor` no incluye
  `setIamPolicy` sobre ellos). Es el único paso manual de **permisos**, y es del operador, no de la infraestructura.
- La compilación de la función usa su propia cuenta (`sa-fn-build`), así que no depende de los permisos por omisión de Cloud Build.

### 3.4 Preparación de imágenes y código

- **Imágenes** (`api`, `worker`, `web`): se construyen **antes** del `apply` y se publican en Artifact Registry con el SHA del commit
  como tag inmutable. Con Docker local, `scripts/publish_images.sh`; sin él, Cloud Build con `scripts/e4_cloudbuild.yaml`. Terraform
  solo *referencia* la imagen (y el `plan` falla si el tag no existe), no la construye: construir imágenes es un paso de CI, no de
  infraestructura.
- **Código de la función**: no hay paso previo. Terraform lo empaqueta del propio repositorio y Cloud Build lo compila al desplegar. Por
  eso el `plan` debe correr **desde un clon limpio de `main`**: lo que despliega es el código que ese clon tiene.

### 3.5 Recursos existentes

El estado ya contiene la infraestructura de las Entregas 2 y 3 (red, Cloud SQL, buckets, grupos, cola). Esta entrega **no la toca**: se
añade con las banderas apagadas por omisión, y un `plan` sin ellas no propone ningún cambio.

Lo que existe **fuera** de Terraform y se *referencia*, no se crea: el secreto `db-password` (`data "google_secret_manager_secret"`) y el
bucket del estado. Si un recurso de esta entrega ya existiera en GCP sin estar en el estado, se incorpora con
`terraform import <dirección> <id>` antes del `plan`; en este despliegue no hizo falta ninguno.

### 3.6 Credenciales, secretos y estado

- **Ninguna credencial en el repositorio ni en las imágenes.** La cadena de conexión a la base la compone Terraform y vive en **Secret
  Manager** (`mooc-database-url`); Cloud Run y la función la reciben como variable de entorno **referenciada a un secreto** (el recurso
  guarda la referencia, no el valor). La contraseña del correo es otro secreto.
- **El estado está en un bucket privado** (`backend "gcs"`), nunca en el repositorio (`*.tfstate` y `*.tfplan` están en `.gitignore`).
- Los archivos `.tfplan` llevan la contraseña de la base: se borran después de aplicar.

## 4. Desplegar y actualizar

Desde un clon limpio de `main`:

```bash
cd infra/terraform
export TF_VAR_db_password="$(gcloud secrets versions access latest --secret=db-password | tr -d '\r\n')"
export TF_VAR_smtp_from='…' TF_VAR_smtp_username='…'
export TF_VAR_image_tag='<tag de las plantillas de la Entrega 3>'      # no cambia

terraform init
terraform plan -out=e5.tfplan \
  -var cloudrun_enabled=true \
  -var docfunction_enabled=true \
  -var cloudrun_image_tag=<SHA de las imágenes publicadas> \
  -var queue_vm_status=RUNNING \
  -var db_activation_policy=ALWAYS
terraform apply e5.tfplan
rm e5.tfplan
```

**Actualizar** es repetir lo mismo con otro valor, y Terraform solo cambia lo que difiere:

| Cambio | Qué hace el `plan` |
| :--- | :--- |
| Otro `cloudrun_image_tag` | nueva revisión de `mooc-frontend`, `mooc-backend` y `mooc-workers` (cambio *in situ*) |
| Cambio en `internal/**` o en la función | nuevo zip → nuevo objeto → la función se redespliega |
| Otro límite en `cloudrun_services` o `docfunction_limits` | cambio de escalamiento sin tocar el resto |
| Nada | `No changes` |

**Apagar al terminar:** `docfunction_enabled=false` y `cloudrun_enabled=false` destruyen lo de esta entrega y la de la 4; con
`db_activation_policy=NEVER` y `queue_vm_status=TERMINATED` se apagan la base y Redis. Siempre se pasan **todas** las banderas del
ambiente que existe: omitir `cloudrun_enabled=true` sobre un ambiente que lo tiene **lo destruye**.

## 5. Permisos de la función (mínimo privilegio)

| Cuenta | Para qué | Permisos |
| :--- | :--- | :--- |
| `sa-fn-doccheck` | **ejecuta** la función | `objectViewer` sobre el bucket de medios (lee, no escribe); `secretAccessor` solo sobre `mooc-database-url` |
| `sa-fn-trigger` | Eventarc la **invoca** | `run.invoker` solo sobre el servicio de esta función; `eventarc.eventReceiver` |
| `sa-fn-build` | Cloud Build la **compila** | `cloudbuild.builds.builder`; no ejecuta nada después |
| `sa-web-server` (backend) | publica | `pubsub.publisher` sobre `mooc-media-documents`, solo ese topic |

La función **no tiene URL pública utilizable**: su ingreso es `ALLOW_INTERNAL_ONLY` y solo la cuenta del disparador puede invocarla.

## 6. Evidencia

Todo en [`evidencias/`](evidencias/). Las corridas son del 4 de noviembre de 2026, contra el ambiente real
(`plataforma-mooc-entrega2`, `us-east1`).

### 6.1 Terraform: despliegue y actualización

| Corrida | Qué pasó |
| :--- | :--- |
| **1. Despliegue** (`apply` desde `main` tras el PR #319) | Creó la API de Eventarc/Functions, el bucket del código, el zip, el topic `mooc-media-documents`, las tres cuentas de servicio y sus permisos, y actualizó backend y workers. **La función falló al compilar**: el zip no incluía `api/`, que un paquete del backend importa, y Cloud Build compila el módulo entero. |
| **2. Actualización** (`apply` tras el PR #320) | Terraform detectó la función dañada (*tainted*) y reemplazó solo lo necesario: `Plan: 3 to add, 2 to change, 2 to destroy.` Resumen en [`6_plan_de_actualizacion.txt`](evidencias/funcion/6_plan_de_actualizacion.txt). No tocó la base, Redis ni el ambiente de la Entrega 3. |

La segunda corrida es a la vez la demostración de **actualizar**: cambió el código desplegable (nuevo zip, nuevo hash en el
nombre del objeto) y dos servicios de Cloud Run sin recrearlos. El estado resultante está en
[`5_terraform_state_list.txt`](evidencias/funcion/5_terraform_state_list.txt) y la función desplegada, con su configuración, en
[`4_funcion_desplegada.yaml`](evidencias/funcion/4_funcion_desplegada.yaml): `ACTIVE`, runtime `go124`, ingreso
`ALLOW_INTERNAL_ONLY`, `RETRY_POLICY_RETRY`, máximo 3 instancias, cuenta de servicio propia.

### 6.2 Una tarea real: backend → Pub/Sub → función → resultado

Recorrido completo de la plataforma contra el frontend de Cloud Run con `scripts/e2e_cloud`
([`evidencias/recorrido/`](evidencias/recorrido/resultados.md)): **103 pasos, 0 fallos**, y la reconciliación final confirma
`3/3 trabajos reconciliados en completed` (video, PDF e imagen). Lo que muestran los logs de ese mismo recorrido:

| Paso | Evidencia |
| :--- | :--- |
| El backend publica cada tarea en **su** topic | [`1_backend_publica.txt`](evidencias/funcion/1_backend_publica.txt): el video (`hls_transcode`) va a `mooc-media-jobs`; el PDF y la imagen (`malware_scan`) van a `mooc-media-documents` |
| La **función** recibe y completa los documentos | [`2_funcion_procesa_y_reentrega.txt`](evidencias/funcion/2_funcion_procesa_y_reentrega.txt): `document job received` → `document job completed; event consumed` para el PDF (`89b467f9…`) y la imagen (`8a11b401…`) |
| El **worker ya no ejecuta documentos** | [`3_worker_solo_video.txt`](evidencias/funcion/3_worker_solo_video.txt): solo procesa el video; no aparece ningún `malware_scan` |
| El recurso queda igual que antes | Los tres recursos terminan en `completed`, igual que cuando el worker validaba los documentos |

### 6.3 Reintentos sin efectos duplicados

Se publicó **dos veces a mano** el mismo mensaje del PDF ya procesado (`gcloud pubsub topics publish mooc-media-documents …`),
simulando una reentrega ([mensaje usado](evidencias/funcion/7_mensaje_de_reentrega.json)). Las dos veces la función respondió `media job skipped; this object is already processed` y confirmó el
evento, sin repetir la validación ni cambiar el recurso (en [`2_funcion_procesa_y_reentrega.txt`](evidencias/funcion/2_funcion_procesa_y_reentrega.txt),
las últimas filas). Esa es la garantía: un duplicado se consume sin efecto, y un fallo transitorio devuelve error para que Eventarc
reintente.

La consola de `terraform init`, `plan` y `apply` se muestra en vivo en el video.

## 7. Limitaciones

- **El plazo de reintento de la función.** Eventarc reentrega con espera exponencial pero **no hay una cola de mensajes muertos
  configurable** en este disparador: un documento que siga fallando de forma transitoria se reintenta mientras el mensaje esté
  retenido (24 h en el topic). Un fallo permanente se consume y queda registrado en el recurso (`failed`).
- **Arranque en frío.** Con mínimo 0, el primer documento tras un rato sin uso espera a que arranque la función.
- **Cloud SQL por red privada.** La función sale por la misma subred que los servicios de Cloud Run (*Direct VPC egress*); si esa subred
  se agotara, también se vería afectada la función.
- **Las imágenes no las construye Terraform.** Es un paso previo documentado; si el tag no existe, el `plan` falla con un error claro.
- **El permiso de aplicar** (§3.3) es manual y del operador.
