# Cloud Run function (2nd gen) que valida documentos (Entrega 5).
#
# Migra una tarea del worker: la validacion de documentos (task_type `malware_scan`: PDF, presentaciones, archivos
# descargables e imagenes). Es el trabajo mas pequeno del worker --comprueba que el objeto esta en el bucket, que su
# tamano es valido y que su tipo coincide con su extension, y marca el recurso `completed`; ~50 ms medidos-- y el unico
# que no necesita FFmpeg, disco ni minutos. El video se queda en el worker.
#
#   backend --publica--> topic mooc-media-documents --Eventarc--> mooc-doccheck --> Cloud SQL + Cloud Storage
#
# El ENRUTAMIENTO vive en quien publica: el backend manda las tareas de documentos a su propio topic
# (PUBSUB_DOCUMENTS_TOPIC) y el worker deja de ejecutarlas (MEDIA_DOCUMENTS_IN_FUNCTION). Asi cada tarea tiene un solo
# dueno por construccion, y no depende de que cada consumidor filtre lo que recibe.
#
# Apagado por omision y dependiente de `cloudrun_enabled` (reutiliza su subred, su secreto y sus topics). Con ambas en
# `false` un plan no toca nada.

variable "docfunction_enabled" {
  description = "Crea la funcion de validacion de documentos, su topic y su disparador (Entrega 5). Exige cloudrun_enabled."
  type        = bool
  default     = false
}

variable "function_runtime" {
  description = "Runtime de Go de la funcion. Tiene que ser compatible con el `go` de go.mod (1.24)."
  type        = string
  default     = "go124"
}

variable "docfunction_limits" {
  description = "Dimensionamiento de la funcion."
  type = object({
    cpu         = string
    memory      = string
    concurrency = number
    timeout     = number
    min         = number
    max         = number
  })
  # La tarea dura decenas de milisegundos: poca memoria, poco tiempo y varias peticiones por instancia. Maximo 3, como
  # los servicios de Cloud Run.
  default = { cpu = "1", memory = "256Mi", concurrency = 4, timeout = 60, min = 0, max = 3 }
}

locals {
  fn_count   = var.cloudrun_enabled && var.docfunction_enabled ? 1 : 0
  fn_enabled = local.fn_count == 1

  # El codigo de la funcion es el modulo de la plataforma: el paquete de la raiz (doccheck_function.go) y todo internal/,
  # sin las pruebas. Se arma un zip determinista dentro de Terraform --sin pasos previos a mano-- con exactamente lo que
  # el compilador necesita; cualquier cambio en esos archivos cambia el hash del zip y por tanto redespliega la funcion.
  fn_internal_files = [
    for f in fileset("${path.module}/../../internal", "**/*.go") : f if !endswith(f, "_test.go")
  ]
}

# --- APIs ---------------------------------------------------------------------
resource "google_project_service" "functions" {
  for_each = local.fn_enabled ? toset(["cloudfunctions.googleapis.com", "eventarc.googleapis.com"]) : toset([])

  project            = var.project_id
  service            = each.value
  disable_on_destroy = false
}

# --- Codigo desplegable -------------------------------------------------------
data "archive_file" "doccheck" {
  count = local.fn_count

  type        = "zip"
  output_path = "${path.module}/.terraform/doccheck-source.zip"

  source {
    filename = "go.mod"
    content  = file("${path.module}/../../go.mod")
  }
  source {
    filename = "go.sum"
    content  = file("${path.module}/../../go.sum")
  }
  source {
    filename = "doccheck_function.go"
    content  = file("${path.module}/../../doccheck_function.go")
  }

  dynamic "source" {
    for_each = toset(local.fn_internal_files)
    content {
      filename = "internal/${source.value}"
      content  = file("${path.module}/../../internal/${source.value}")
    }
  }
}

resource "google_storage_bucket" "function_source" {
  count = local.fn_count

  name                        = "${var.project_id}-function-source"
  location                    = var.region
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
  force_destroy               = true
  labels                      = var.labels
}

# El nombre lleva el hash: cada version del codigo es un objeto distinto, y la funcion solo se redespliega cuando cambia.
resource "google_storage_bucket_object" "doccheck_source" {
  count = local.fn_count

  name   = "doccheck-${data.archive_file.doccheck[0].output_md5}.zip"
  bucket = google_storage_bucket.function_source[0].name
  source = data.archive_file.doccheck[0].output_path
}

# --- Topic de documentos ------------------------------------------------------
resource "google_pubsub_topic" "media_documents" {
  count = local.fn_count

  name                       = "mooc-media-documents"
  labels                     = var.labels
  message_retention_duration = "86400s"

  depends_on = [google_project_service.cloudrun]
}

# Solo el backend publica, y solo aqui.
resource "google_pubsub_topic_iam_member" "backend_publishes_documents" {
  count = local.fn_count

  topic  = google_pubsub_topic.media_documents[0].name
  role   = "roles/pubsub.publisher"
  member = "serviceAccount:${google_service_account.web_server.email}"
}

# --- Cuentas de servicio ------------------------------------------------------
# Tres identidades, una por papel, sin mezclar:
#   sa-fn-doccheck   la que EJECUTA la funcion: lee del bucket y de la cadena de conexion; nada mas.
#   sa-fn-trigger    la que Eventarc usa para INVOCARLA: solo puede invocar esa funcion y recibir eventos.
#   sa-fn-build      la que COMPILA el codigo en Cloud Build: no corre nada despues.
resource "google_service_account" "fn_runtime" {
  count = local.fn_count

  project      = var.project_id
  account_id   = "sa-fn-doccheck"
  display_name = "Funcion de validacion de documentos (ejecucion)"
}

resource "google_service_account" "fn_trigger" {
  count = local.fn_count

  project      = var.project_id
  account_id   = "sa-fn-trigger"
  display_name = "Disparador Eventarc de la funcion de documentos"
}

resource "google_service_account" "fn_build" {
  count = local.fn_count

  project      = var.project_id
  account_id   = "sa-fn-build"
  display_name = "Compilacion de la funcion de documentos (Cloud Build)"
}

# La funcion solo LEE del bucket de medios (comprueba el objeto, no lo toca) y lee la cadena de conexion.
resource "google_storage_bucket_iam_member" "fn_reads_media" {
  count = local.fn_count

  bucket = google_storage_bucket.media.name
  role   = "roles/storage.objectViewer"
  member = "serviceAccount:${google_service_account.fn_runtime[0].email}"
}

resource "google_secret_manager_secret_iam_member" "fn_reads_database_url" {
  count = local.fn_count

  project   = var.project_id
  secret_id = google_secret_manager_secret.database_url[0].secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.fn_runtime[0].email}"
}

resource "google_project_iam_member" "fn_trigger_receives_events" {
  count = local.fn_count

  project = var.project_id
  role    = "roles/eventarc.eventReceiver"
  member  = "serviceAccount:${google_service_account.fn_trigger[0].email}"
}

# Invocar SOLO esta funcion (el servicio de Cloud Run que la respalda), no todos los servicios del proyecto.
resource "google_cloud_run_v2_service_iam_member" "fn_trigger_invokes" {
  count = local.fn_count

  project  = var.project_id
  location = var.region
  name     = google_cloudfunctions2_function.doccheck[0].service_config[0].service
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.fn_trigger[0].email}"
}

resource "google_project_iam_member" "fn_build" {
  count = local.fn_count

  project = var.project_id
  role    = "roles/cloudbuild.builds.builder"
  member  = "serviceAccount:${google_service_account.fn_build[0].email}"
}

# --- La funcion ---------------------------------------------------------------
resource "google_cloudfunctions2_function" "doccheck" {
  count = local.fn_count

  name        = "mooc-doccheck"
  location    = var.region
  description = "Valida los documentos subidos (malware_scan) y los deja disponibles. Entrega 5."
  labels      = var.labels

  build_config {
    runtime         = var.function_runtime
    entry_point     = "DocCheck"
    service_account = google_service_account.fn_build[0].id

    source {
      storage_source {
        bucket = google_storage_bucket.function_source[0].name
        object = google_storage_bucket_object.doccheck_source[0].name
      }
    }
  }

  service_config {
    service_account_email = google_service_account.fn_runtime[0].email

    available_cpu                    = var.docfunction_limits.cpu
    available_memory                 = var.docfunction_limits.memory
    max_instance_request_concurrency = var.docfunction_limits.concurrency
    timeout_seconds                  = var.docfunction_limits.timeout
    min_instance_count               = var.docfunction_limits.min
    max_instance_count               = var.docfunction_limits.max

    # Solo lo que viene de dentro de Google (Eventarc) puede llamarla; no tiene URL publica util.
    ingress_settings               = "ALLOW_INTERNAL_ONLY"
    all_traffic_on_latest_revision = true

    # Cloud SQL esta en una IP privada: la funcion sale por la misma subred que los servicios de Cloud Run.
    direct_vpc_egress = "VPC_EGRESS_PRIVATE_RANGES_ONLY"
    direct_vpc_network_interface {
      network    = google_compute_network.vpc.id
      subnetwork = google_compute_subnetwork.run[0].id
    }

    environment_variables = {
      APP_ENV           = "production"
      STORAGE_BACKEND   = "gcs"
      S3_BUCKET         = google_storage_bucket.media.name
      GCP_PROJECT_ID    = var.project_id
      DB_MAX_OPEN_CONNS = "4"
      DB_MAX_IDLE_CONNS = "4"
    }

    secret_environment_variables {
      key        = "DATABASE_URL"
      project_id = var.project_id
      secret     = google_secret_manager_secret.database_url[0].secret_id
      version    = "latest"
    }
  }

  # RETRY_POLICY_RETRY: si la funcion devuelve un error, el evento se reentrega con espera exponencial. Lo que la
  # funcion considera terminado o imposible de terminar lo confirma devolviendo nil (internal/worker/eventfn).
  event_trigger {
    trigger_region        = var.region
    event_type            = "google.cloud.pubsub.topic.v1.messagePublished"
    pubsub_topic          = google_pubsub_topic.media_documents[0].id
    retry_policy          = "RETRY_POLICY_RETRY"
    service_account_email = google_service_account.fn_trigger[0].email
  }

  depends_on = [
    google_project_service.functions,
    google_project_iam_member.fn_build,
    google_project_iam_member.fn_trigger_receives_events,
    google_storage_bucket_iam_member.fn_reads_media,
    google_secret_manager_secret_iam_member.fn_reads_database_url,
  ]
}

output "docfunction_name" {
  description = "Funcion que valida documentos (Entrega 5)."
  value       = local.fn_enabled ? google_cloudfunctions2_function.doccheck[0].name : ""
}

output "documents_topic" {
  description = "Topic al que el backend publica las tareas de documentos."
  value       = local.fn_enabled ? google_pubsub_topic.media_documents[0].id : ""
}
