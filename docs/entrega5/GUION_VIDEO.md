# Guion del video — Entrega 5 (≈ 15 min, máximo 20)

Una sola persona puede grabarlo de corrido: cada bloque dice **qué se ve**, **qué se dice** y de dónde sale la cifra. Las cifras
están en [`README.md`](README.md) y [`ANALISIS_SERVERLESS.md`](ANALISIS_SERVERLESS.md); si el video y el documento difieren, manda el
documento. Es un video **independiente** del de la Entrega 4.

## Antes de grabar (10 min)

- [ ] Cloud Run, Cloud SQL y la VM `mooc-queue-server` **encendidos**, y la función `mooc-doccheck` en estado `ACTIVE`.
- [ ] Un curso borrador con módulo, unidad y dos recursos vacíos: **un PDF** y **un video**. Para subir: `docs/media/guia.pdf` y un
      video corto (`scratch/clase_e5.mp4`, 20 s).
- [ ] **Iniciar sesión antes de grabar** como `profesor1@plataforma-mooc.test` (nadie escribe una contraseña en pantalla).
- [ ] Una terminal PowerShell abierta en `infra/terraform` de un clon limpio de `main`, con `TF_VAR_db_password`, `TF_VAR_smtp_from`,
      `TF_VAR_smtp_username` y `TF_VAR_image_tag` ya definidos (se definen **antes** de grabar para no mostrar valores).
- [ ] Pestañas en orden: (1) `docs/entrega5/ANALISIS_SERVERLESS.md` en el diagrama, (2) editor con `functions.tf`, (3) terminal,
      (4) frontend, (5) consola de Pub/Sub, (6) consola de Cloud Run functions, (7) Logs Explorer, (8) consola de IAM.
- [ ] Zoom del navegador al 125 %. Que no se vea tu correo personal, ni valores de secretos, ni el archivo de estado de Terraform,
      ni el contenido de ningún `.tfplan`.

Rango de tiempo del Logs Explorer (hora de Colombia, UTC-5): el recorrido con la reentrega duplicada fue el **4 de nov. entre 11:57 y
12:01**. Los registros se conservan 30 días. Si se prefiere un rastro reciente, basta subir un documento en vivo en el bloque 7:30.

Consulta del Logs Explorer (pegarla antes de grabar):

```text
resource.type="cloud_run_revision"
(resource.labels.service_name="mooc-backend" OR resource.labels.service_name="mooc-doccheck" OR resource.labels.service_name="mooc-workers")
("media job published" OR "document job" OR "media job skipped" OR "media job received")
```

## Guion

| Tiempo | Qué se ve | Qué decir |
| :--- | :--- | :--- |
| **0:00–2:00** Análisis serverless | `ANALISIS_SERVERLESS.md`: tabla de oportunidades y diagrama | «Ya es serverless: el frontend, el backend y los workers en **Cloud Run**, y Pub/Sub. No lo es: Cloud SQL, la VM de Redis y la red. Evaluamos siete oportunidades; la mejor para migrar es la **validación de documentos**: dura decenas de milisegundos, no necesita FFmpeg ni disco, y es independiente de los demás trabajos. El video **se queda** en el worker porque dura minutos y necesita 4 GiB.» |
| **2:00–4:00** La migración | El diagrama final de `ANALISIS_SERVERLESS.md` y README §2 | «Antes, el worker ejecutaba dos tipos de trabajo. Ahora el **backend** publica los documentos en **su propio topic**, `mooc-media-documents`, y una **Cloud Run function de 2.ª generación** los recibe por **Eventarc**. El enrutamiento está en quien publica, y el worker **se niega** a ejecutar documentos: cada tarea tiene **un solo dueño**. La función corre **el mismo código** que el worker (`MediaProcessor`), así que el resultado es idéntico.» |
| **4:00–6:00** Terraform: qué gestiona | Editor: `functions.tf`, luego `cloudrun.tf` (desplazarse sin leer línea por línea) | «Terraform gestiona todo: los **servicios de Cloud Run** con imagen, configuración y escalamiento; los **topics y suscripciones**; **la función, su código y su disparador**; las **cuentas de servicio y permisos**; y las **referencias a secretos**. El código de la función se empaqueta **dentro de Terraform** con `archive_file`: un zip determinista cuyo hash va en el nombre del objeto; si el código cambia, la función se redespliega.» |
| **6:00–8:30** Terraform en vivo | Terminal: `terraform init`, luego `terraform plan -out=e5.tfplan` con **todas** las banderas (§4 del README), y `terraform apply e5.tfplan` | «`init` descarga los proveedores; `plan` muestra qué va a cambiar sin tocar nada; `apply` lo ejecuta. Siempre pasamos **todas** las banderas del ambiente: omitir `cloudrun_enabled` lo destruiría. Como el ambiente ya existe, el plan dice **sin cambios**.» Luego mostrar una **actualización**: `plan` con `docfunction_limits` en `max=2` → un solo cambio *in situ*; **no aplicarlo**. Borrar el `.tfplan` al terminar (lleva la contraseña de la base). |
| **8:30–9:30** Parametrización y secretos | `variables` en el README §3.2; en la consola de Cloud Run functions, pestaña de variables | «Todo está **parametrizado**: proyecto, región, tags de imagen, límites, bandera `docfunction_enabled`. El repositorio **no tiene credenciales, secretos ni estado**; el estado vive en un bucket remoto y `DATABASE_URL` entra por **referencia a Secret Manager**, no como valor.» No abrir el valor del secreto. |
| **9:30–12:00** Una tarea real | Frontend: el profesor sube `guia.pdf` al recurso de PDF → pasa a **disponible**. Después, en Logs Explorer, la consulta; abrir las líneas del PDF | «Subí el PDF. Este es su recorrido: **`media job published`** en el backend, con `topic=mooc-media-documents`; **`document job received`** y **`completed; event consumed`** en la **función**; y el recurso queda `completed`. En los logs del **worker** no aparece ningún `malware_scan`.» Subir también el video y mostrar que ese sí lo procesa el worker, por `mooc-media-jobs`. |
| **12:00–13:30** Reintentos sin duplicados | Terminal: `gcloud pubsub topics publish mooc-media-documents --message=<el mismo mensaje del PDF>` **dos veces**; Logs Explorer | «Pub/Sub entrega **al menos una vez**, así que un duplicado es normal. Publico dos veces el mismo mensaje: la función responde **`skipped; this object is already processed`** y confirma el evento. **No repite el trabajo ni cambia el recurso.** Si en cambio hubiera un fallo transitorio, la función **devuelve error** y Eventarc **reintenta** con espera creciente.» (El mensaje de prueba está en [`evidencias/funcion/`](evidencias/funcion/).) |
| **13:30–14:30** Permisos | Consola de IAM: las cuentas `sa-fn-doccheck`, `sa-fn-trigger`, `sa-fn-build`; README §5 | «Tres identidades, una por papel. La que **ejecuta** solo lee del bucket y de un secreto. La que **dispara** solo puede invocar esta función. La que **compila** no ejecuta nada. La función tiene ingreso **solo interno**: no se puede llamar desde internet.» |
| **14:30–15:30** Lo que encontramos y cierre | README §6.1 y §7 | «Nuestro primer despliegue **falló**: el zip no incluía la carpeta `api/` y Cloud Build compila el módulo entero. Lo corregimos y Terraform repuso solo lo dañado: **3 por crear, 2 por cambiar, 2 por reemplazar**, sin tocar la base. Limitaciones: este disparador no tiene cola de mensajes muertos configurable, el primer documento tras un rato sin uso espera el arranque en frío, y las imágenes se publican antes de Terraform. Todo está documentado en el repositorio.» |

## Si algo falla en vivo

- **El `plan` pide variables:** faltan las `TF_VAR_*`; defínelas antes y repite. No pegues los valores en pantalla.
- **El documento se queda «en cola» más de un minuto:** abre los logs de `mooc-doccheck`; si no hay nada, revisa que el recurso de
  Cloud Run `mooc-backend` tenga `PUBSUB_DOCUMENTS_TOPIC` (variables del servicio).
- **El tiempo no alcanza:** el bloque 8:30–9:30 se puede resumir en una frase; no recortes el de la tarea real ni el de reintentos.
