# Despliegue en Cloud Run y Cloud Pub/Sub (Entrega 4).
#
# Tres servicios independientes de Cloud Run (frontend, backend, workers), una cola de Cloud Pub/Sub entre el
# backend y los workers, y todo lo que las une: red, cuentas de servicio, secretos e IAM.
#
#   navegador --HTTPS--> mooc-frontend (nginx + SPA)
#                           |  /api  (proxy, mismo origen: la cookie de sesion no cambia)
#                           v
#                        mooc-backend (API) --publica--> topic mooc-media-jobs
#                           |                                  |
#                           |                  suscripcion push autenticada (OIDC)
#                           |                                  v
#                           |                        mooc-workers  POST /pubsub/push
#                           v                                  |
#                  Cloud SQL / Redis (VPC)  <------------------+--> Cloud Storage
#
# TODO ESTO ESTA APAGADO POR OMISION (`cloudrun_enabled = false`). La infraestructura de las Entregas 2 y 3 (VMs, grupos,
# balanceador) sigue en pie y un `plan` sin la variable no propone ni un cambio. Se enciende con:
#
#   terraform apply -var cloudrun_enabled=true <los -var del entorno>
#
# Las imagenes de `cloudrun_image_tag` (api, worker y web) tienen que estar publicadas antes (scripts/publish_images.sh).

variable "cloudrun_enabled" {
  description = "Crea los servicios de Cloud Run, el topic de Pub/Sub y lo que los une (Entrega 4). Apagado, no existe nada de esto."
  type        = bool
  default     = false
}

# `image_tag` lo comparten las plantillas de los grupos de la Entrega 3 (metadata `mooc-image-tag`): cambiarlo recrea
# esas instancias. Cloud Run tiene su propio tag para poder publicar imagenes nuevas sin tocar lo que ya corre. Vacio usa
# `image_tag`.
variable "cloudrun_image_tag" {
  description = "SHA completo (40 hex) de las imagenes api, worker y web que corre Cloud Run. Vacio usa image_tag."
  type        = string
  default     = ""

  validation {
    condition     = var.cloudrun_image_tag == "" || can(regex("^[0-9a-f]{40}$", var.cloudrun_image_tag))
    error_message = "cloudrun_image_tag debe ser el SHA completo de 40 caracteres hexadecimales, o vacio."
  }
}

variable "smtp_host" {
  description = "Servidor SMTP del proveedor de correo. No es secreto; la contrasena esta en Secret Manager."
  type        = string
  default     = "smtp-relay.brevo.com"
}

# Los limites que la rubrica pide documentar, uno por servicio. Se declaran juntos para que el informe (y el `plan`) los
# muestren en un solo sitio. El maximo de instancias es 3 en los tres, como exige el enunciado.
variable "cloudrun_services" {
  description = "Dimensionamiento de cada servicio de Cloud Run."
  type = map(object({
    cpu         = string
    memory      = string
    concurrency = number
    timeout     = number
    min         = number
    max         = number
  }))
  default = {
    # nginx + estaticos: casi no consume. Escala a cero cuando nadie lo usa.
    frontend = { cpu = "1", memory = "256Mi", concurrency = 80, timeout = 60, min = 0, max = 3 }

    # La API: una instancia atiende 80 peticiones concurrentes. min = 1 evita el arranque en frio en la primera
    # peticion (login), a un costo bajo porque con cpu_idle el CPU solo se cobra mientras atiende.
    backend = { cpu = "1", memory = "512Mi", concurrency = 80, timeout = 60, min = 1, max = 3 }

    # Un video por instancia (concurrency = 1): ffmpeg ya reparte el trabajo entre los nucleos, y asi una instancia que se
    # cae no se lleva consigo mas de un trabajo. 4Gi porque /tmp en Cloud Run es memoria, y ahi se descarga el original y
    # se escriben los segmentos HLS.
    #
    # timeout = 590 s: tiene que ser MENOR que el plazo de confirmacion de la suscripcion (600 s, el maximo que admite
    # Pub/Sub). Si una peticion durara mas, Pub/Sub la daria por perdida y la reentregaria a otra instancia mientras la
    # primera sigue trabajando.
    workers = { cpu = "2", memory = "4Gi", concurrency = 1, timeout = 590, min = 0, max = 3 }
  }
}

locals {
  run_count = var.cloudrun_enabled ? 1 : 0

  run_image_tag = var.cloudrun_image_tag != "" ? var.cloudrun_image_tag : var.image_tag

  # Cloud Run publica cada servicio en `<servicio>-<numero del proyecto>.<region>.run.app`. Es deterministico, y por eso
  # frontend y backend pueden conocerse mutuamente sin que Terraform tenga un ciclo (el frontend necesita el host del
  # backend; el backend, el origen del frontend para CSRF y los enlaces de correo).
  run_domain   = "${data.google_project.this.number}.${var.region}.run.app"
  frontend_url = "https://mooc-frontend-${local.run_domain}"
  backend_host = "mooc-backend-${local.run_domain}"
  backend_url  = "https://${local.backend_host}"
  workers_url  = "https://mooc-workers-${local.run_domain}"

  image_prefix = "${var.region}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.images.repository_id}"
}

# La imagen del frontend solo la usa Cloud Run, asi que nadie la comprobaba antes.
# Comprobarla aqui convierte «la imagen no esta publicada» en un error del plan y no en un servicio que no arranca.
data "google_artifact_registry_docker_image" "web" {
  count = local.run_count

  location      = google_artifact_registry_repository.images.location
  repository_id = google_artifact_registry_repository.images.repository_id
  image_name    = "web:${local.run_image_tag}"
}

data "google_project" "this" {
  project_id = var.project_id
}

# ---------------------------------------------------------------------------
# APIs
# ---------------------------------------------------------------------------
resource "google_project_service" "cloudrun" {
  for_each = var.cloudrun_enabled ? toset(["run.googleapis.com", "pubsub.googleapis.com"]) : toset([])

  project            = var.project_id
  service            = each.value
  disable_on_destroy = false
}

# ---------------------------------------------------------------------------
# Red: Direct VPC egress hacia Cloud SQL y Redis
# ---------------------------------------------------------------------------
# Backend y workers llegan a Cloud SQL (IP privada, peering) y a Redis (sesiones, limitador, idempotencia) por la VPC.
# Direct VPC egress da a cada instancia una IP de esta subred, sin conector ni VM intermedia. Solo el trafico a rangos
# privados entra a la VPC (`PRIVATE_RANGES_ONLY`); el resto (Cloud Storage, Pub/Sub, SMTP) sale directo por internet sin
# pasar por el NAT, que asi no se convierte en cuello de botella.
resource "google_compute_subnetwork" "run" {
  count = local.run_count

  name                     = "mooc-run-subnet"
  ip_cidr_range            = "10.0.2.0/26"
  region                   = var.region
  network                  = google_compute_network.vpc.id
  private_ip_google_access = true
  description              = "IPs de las instancias de Cloud Run (Direct VPC egress, Entrega 4)"
}

# Redis (la cola de la Entrega 3, que sigue guardando sesiones y limitador): solo desde la subred de Cloud Run.
resource "google_compute_firewall" "run_to_redis" {
  count = local.run_count

  name        = "mooc-allow-run-redis"
  network     = google_compute_network.vpc.id
  description = "Cloud Run (backend y workers) hacia el 6379 de Redis (Entrega 4)"

  allow {
    protocol = "tcp"
    ports    = ["6379"]
  }

  source_ranges           = [google_compute_subnetwork.run[0].ip_cidr_range]
  target_service_accounts = [google_service_account.queue_server.email]
}

# ---------------------------------------------------------------------------
# Cuentas de servicio
# ---------------------------------------------------------------------------
# Backend y workers reutilizan las cuentas de la Entrega 3 (`sa-web-server`, `sa-worker-server`): ya tienen exactamente los
# permisos de cada responsabilidad sobre los buckets y los secretos (storage.tf, mail.tf, database.tf), y duplicarlos en
# cuentas nuevas solo abriria la puerta a que diverjan. Lo que esta entrega agrega es lo nuevo:
#
#   sa-run-frontend   el frontend no habla con ningun recurso de GCP: cuenta sin ningun rol.
#   sa-pubsub-push    la identidad con la que Pub/Sub se presenta ante los workers. Su unico permiso es invocarlos.
resource "google_service_account" "run_frontend" {
  count = local.run_count

  project      = var.project_id
  account_id   = "sa-run-frontend"
  display_name = "Cloud Run frontend"
  description  = "Identidad del servicio del frontend. No tiene ningun rol: solo sirve estaticos y reenvia /api."
}

resource "google_service_account" "pubsub_push" {
  count = local.run_count

  project      = var.project_id
  account_id   = "sa-pubsub-push"
  display_name = "Pub/Sub push hacia los workers"
  description  = "Firma el token OIDC de la suscripcion push. Solo puede invocar el servicio de workers."
}

# ---------------------------------------------------------------------------
# Secretos
# ---------------------------------------------------------------------------
# La aplicacion lee DATABASE_URL entera, asi que la contrasena tiene que viajar dentro de una cadena de conexion. Esa
# cadena se guarda como secreto y Cloud Run la inyecta en tiempo de ejecucion: no esta en el repositorio, ni en la imagen,
# ni en la configuracion del servicio (que solo guarda la referencia al secreto).
resource "google_secret_manager_secret" "database_url" {
  count = local.run_count

  secret_id = "mooc-database-url"
  project   = var.project_id
  labels    = var.labels

  replication {
    auto {}
  }

  depends_on = [google_project_service.required["secretmanager.googleapis.com"]]
}

resource "google_secret_manager_secret_version" "database_url" {
  count = local.run_count

  secret      = google_secret_manager_secret.database_url[0].id
  secret_data = "postgres://${google_sql_user.app.name}:${urlencode(trimspace(var.db_password))}@${google_sql_database_instance.main.private_ip_address}:5432/${google_sql_database.mooc.name}?sslmode=require"
}

resource "google_secret_manager_secret_iam_member" "database_url_readers" {
  for_each = var.cloudrun_enabled ? local.service_accounts : {}

  project   = var.project_id
  secret_id = google_secret_manager_secret.database_url[0].secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${each.value}"
}

# ---------------------------------------------------------------------------
# Cloud Pub/Sub
# ---------------------------------------------------------------------------
resource "google_pubsub_topic" "media_jobs" {
  count = local.run_count

  name   = "mooc-media-jobs"
  labels = var.labels

  # Si no hay ninguna suscripcion capaz de entregar, el mensaje se conserva un dia en vez de perderse.
  message_retention_duration = "86400s"

  depends_on = [google_project_service.cloudrun]
}

# Mensajes que agotaron sus intentos. Se conservan para inspeccionarlos y reintentarlos a mano; el trabajo ya quedo marcado
# como fallido en su recurso, asi que nadie espera en vano.
resource "google_pubsub_topic" "media_jobs_dead" {
  count = local.run_count

  name   = "mooc-media-jobs-dead"
  labels = var.labels

  depends_on = [google_project_service.cloudrun]
}

resource "google_pubsub_subscription" "media_jobs_dead" {
  count = local.run_count

  name  = "mooc-media-jobs-dead-pull"
  topic = google_pubsub_topic.media_jobs_dead[0].id

  message_retention_duration = "604800s"
  ack_deadline_seconds       = 60
  expiration_policy {
    ttl = "" # nunca expira
  }
}

# La suscripcion push que entrega cada trabajo al servicio de workers.
#
#   ack_deadline_seconds = 600   el maximo de Pub/Sub; es el tiempo que tiene un worker para terminar y responder 2xx. El
#                                timeout de Cloud Run (590 s) es menor a proposito: ver `cloudrun_services`.
#   oidc_token                   Pub/Sub firma una identidad con `sa-pubsub-push` y Cloud Run la verifica contra IAM antes
#                                de dejar pasar la peticion: sin `run.invoker` no llega nada al codigo.
#   retry_policy                 una entrega fallida (5xx, timeout) se reintenta con espera exponencial de 10 s a 10 min.
#   dead_letter_policy           tras 20 intentos el mensaje pasa al topic de mensajes muertos. Con 5, una tanda de 9 videos
#                                contra 3 workers de concurrencia 1 dejo a dos mensajes en su ULTIMO intento: los mensajes
#                                4 a 9 chocan con una pared de instancias ocupadas, Cloud Run los rechaza (429/500 «aborted
#                                after waiting too long for an available instance») y cada rechazo cuenta como intento.
#                                Un rechazo por falta de capacidad no es un fallo del trabajo; el limite alto sigue acotando
#                                el caso de un mensaje que de verdad no se puede procesar.
resource "google_pubsub_subscription" "media_jobs_push" {
  count = local.run_count

  name  = "mooc-media-jobs-push"
  topic = google_pubsub_topic.media_jobs[0].id

  ack_deadline_seconds       = 600
  message_retention_duration = "86400s"
  retain_acked_messages      = false

  expiration_policy {
    ttl = "" # nunca expira
  }

  push_config {
    push_endpoint = "${local.workers_url}/pubsub/push"

    oidc_token {
      service_account_email = google_service_account.pubsub_push[0].email
      audience              = local.workers_url
    }
  }

  retry_policy {
    minimum_backoff = "10s"
    maximum_backoff = "600s"
  }

  dead_letter_policy {
    dead_letter_topic     = google_pubsub_topic.media_jobs_dead[0].id
    max_delivery_attempts = 20
  }

  depends_on = [
    google_cloud_run_v2_service_iam_member.workers_invoker,
    google_pubsub_topic_iam_member.dead_letter_publisher,
  ]
}

# --- IAM de Pub/Sub ---------------------------------------------------------

# El agente de servicio de Pub/Sub existe en cuanto la API esta habilitada. Es quien firma el token OIDC y quien mueve
# mensajes al topic de mensajes muertos.
locals {
  pubsub_agent = "serviceAccount:service-${data.google_project.this.number}@gcp-sa-pubsub.iam.gserviceaccount.com"
}

resource "google_service_account_iam_member" "pubsub_agent_signs_push_token" {
  count = local.run_count

  service_account_id = google_service_account.pubsub_push[0].name
  role               = "roles/iam.serviceAccountTokenCreator"
  member             = local.pubsub_agent

  depends_on = [google_project_service.cloudrun]
}

resource "google_pubsub_topic_iam_member" "dead_letter_publisher" {
  count = local.run_count

  topic  = google_pubsub_topic.media_jobs_dead[0].name
  role   = "roles/pubsub.publisher"
  member = local.pubsub_agent
}

# Va despues de la suscripcion (necesita su nombre), asi que la suscripcion no puede depender de ella; solo hace falta
# cuando un mensaje llega a agotar sus intentos.
resource "google_pubsub_subscription_iam_member" "dead_letter_subscriber" {
  count = local.run_count

  subscription = google_pubsub_subscription.media_jobs_push[0].name
  role         = "roles/pubsub.subscriber"
  member       = local.pubsub_agent
}

# Quien publica: solo la cuenta del backend, y solo en este topic (no a nivel de proyecto).
resource "google_pubsub_topic_iam_member" "backend_publisher" {
  count = local.run_count

  topic  = google_pubsub_topic.media_jobs[0].name
  role   = "roles/pubsub.publisher"
  member = "serviceAccount:${google_service_account.web_server.email}"
}

# ---------------------------------------------------------------------------
# Cloud Run: los tres servicios
# ---------------------------------------------------------------------------
resource "google_cloud_run_v2_service" "workers" {
  count = local.run_count

  name                = "mooc-workers"
  location            = var.region
  ingress             = "INGRESS_TRAFFIC_ALL" # Pub/Sub llega desde internet; lo que decide es IAM
  deletion_protection = false
  labels              = var.labels

  template {
    service_account                  = google_service_account.worker_server.email
    timeout                          = "${var.cloudrun_services["workers"].timeout}s"
    max_instance_request_concurrency = var.cloudrun_services["workers"].concurrency

    scaling {
      min_instance_count = var.cloudrun_services["workers"].min
      max_instance_count = var.cloudrun_services["workers"].max
    }

    vpc_access {
      egress = "PRIVATE_RANGES_ONLY"
      network_interfaces {
        network    = google_compute_network.vpc.id
        subnetwork = google_compute_subnetwork.run[0].id
      }
    }

    containers {
      image = "${local.image_prefix}/worker:${local.run_image_tag}"

      resources {
        limits = {
          cpu    = var.cloudrun_services["workers"].cpu
          memory = var.cloudrun_services["workers"].memory
        }
        cpu_idle          = true
        startup_cpu_boost = true
      }

      ports {
        container_port = 8080
      }

      env {
        name  = "APP_ENV"
        value = "production"
      }
      env {
        name  = "QUEUE_BACKEND"
        value = "pubsub"
      }
      env {
        name = "DATABASE_URL"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.database_url[0].secret_id
            version = "latest"
          }
        }
      }
      env {
        name  = "REDIS_URL"
        value = "${google_compute_address.queue_internal_ip.address}:6379"
      }
      env {
        name  = "STORAGE_BACKEND"
        value = "gcs"
      }
      env {
        name  = "S3_BUCKET"
        value = google_storage_bucket.media.name
      }
      env {
        name  = "GCP_PROJECT_ID"
        value = var.project_id
      }
      env {
        name  = "MEDIA_HLS_BUCKET"
        value = google_storage_bucket.hls.name
      }
      env {
        name  = "MEDIA_WORK_DIR"
        value = "/tmp/mooc-media"
      }
      # Entrega 5: con la funcion encendida, los documentos ya no son del worker.
      env {
        name  = "MEDIA_DOCUMENTS_IN_FUNCTION"
        value = tostring(local.fn_enabled)
      }
      env {
        name  = "DB_MAX_OPEN_CONNS"
        value = "5"
      }
      env {
        name  = "DB_MAX_IDLE_CONNS"
        value = "5"
      }

      startup_probe {
        http_get {
          path = "/healthz"
        }
        period_seconds    = 3
        failure_threshold = 10
      }
    }
  }

  depends_on = [
    google_project_service.cloudrun,
    google_secret_manager_secret_iam_member.database_url_readers,
    google_secret_manager_secret_version.database_url,
    # Entrega 5: la funcion existe antes de que el worker deje de ejecutar documentos.
    google_cloudfunctions2_function.doccheck,
  ]
}

resource "google_cloud_run_v2_service" "backend" {
  count = local.run_count

  name                = "mooc-backend"
  location            = var.region
  ingress             = "INGRESS_TRAFFIC_ALL"
  deletion_protection = false
  labels              = var.labels

  template {
    service_account                  = google_service_account.web_server.email
    timeout                          = "${var.cloudrun_services["backend"].timeout}s"
    max_instance_request_concurrency = var.cloudrun_services["backend"].concurrency

    scaling {
      min_instance_count = var.cloudrun_services["backend"].min
      max_instance_count = var.cloudrun_services["backend"].max
    }

    vpc_access {
      egress = "PRIVATE_RANGES_ONLY"
      network_interfaces {
        network    = google_compute_network.vpc.id
        subnetwork = google_compute_subnetwork.run[0].id
      }
    }

    containers {
      image = "${local.image_prefix}/api:${local.run_image_tag}"

      resources {
        limits = {
          cpu    = var.cloudrun_services["backend"].cpu
          memory = var.cloudrun_services["backend"].memory
        }
        cpu_idle          = true
        startup_cpu_boost = true
      }

      ports {
        container_port = 8080
      }

      env {
        name  = "APP_ENV"
        value = "production"
      }
      env {
        name  = "QUEUE_BACKEND"
        value = "pubsub"
      }
      env {
        name  = "PUBSUB_TOPIC"
        value = google_pubsub_topic.media_jobs[0].id
      }
      # Entrega 5: las tareas de documentos van a su propio topic, del que se alimenta la funcion. Vacio las deja en
      # PUBSUB_TOPIC, que es lo que hacia la Entrega 4.
      env {
        name  = "PUBSUB_DOCUMENTS_TOPIC"
        value = local.fn_enabled ? google_pubsub_topic.media_documents[0].id : ""
      }
      env {
        name = "DATABASE_URL"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.database_url[0].secret_id
            version = "latest"
          }
        }
      }
      env {
        name  = "REDIS_URL"
        value = "${google_compute_address.queue_internal_ip.address}:6379"
      }

      # La API exige un proxy de confianza para creer en X-Forwarded-For. En Cloud Run no hay una IP fija que declarar (el
      # trafico llega desde el frontend de Google), asi que se pone una direccion de documentacion (RFC 5737) que nunca
      # coincide con un par real: las cabeceras X-Forwarded-* se ignoran y el limitador de intentos ve la IP del
      # intermediario. Por eso sus limites suben mas abajo: documentado como limitacion en docs/entrega4.
      env {
        name  = "TRUSTED_PROXY_IP"
        value = "192.0.2.1"
      }
      env {
        name  = "STORAGE_BACKEND"
        value = "gcs"
      }
      env {
        name  = "S3_BUCKET"
        value = google_storage_bucket.media.name
      }
      env {
        name  = "GCP_PROJECT_ID"
        value = var.project_id
      }
      env {
        name  = "GCS_SIGNER_ACCOUNT"
        value = google_service_account.web_server.email
      }
      env {
        name  = "DB_MAX_OPEN_CONNS"
        value = "15"
      }
      env {
        name  = "DB_MAX_IDLE_CONNS"
        value = "15"
      }
      env {
        name  = "SMTP_HOST"
        value = var.smtp_host
      }
      env {
        name  = "SMTP_PORT"
        value = "587"
      }
      env {
        name  = "SMTP_FROM"
        value = var.smtp_from
      }
      env {
        name  = "SMTP_USERNAME"
        value = var.smtp_username
      }
      env {
        name = "SMTP_PASSWORD"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.smtp_password.secret_id
            version = "latest"
          }
        }
      }
      env {
        name  = "APP_BASE_URL"
        value = local.frontend_url
      }
      env {
        name  = "CSRF_ALLOWED_ORIGINS"
        value = local.frontend_url
      }
      env {
        name  = "RATE_LIMIT_LOGIN_ATTEMPTS"
        value = "1000"
      }

      startup_probe {
        http_get {
          path = "/api/v1/health"
        }
        period_seconds    = 3
        failure_threshold = 10
      }
    }
  }

  depends_on = [
    google_project_service.cloudrun,
    google_secret_manager_secret_iam_member.database_url_readers,
    google_secret_manager_secret_version.database_url,
    google_pubsub_topic_iam_member.backend_publisher,
    # Entrega 5: la funcion y su topic existen antes de que el backend empiece a publicar documentos alli.
    google_cloudfunctions2_function.doccheck,
    google_pubsub_topic_iam_member.backend_publishes_documents,
  ]
}

resource "google_cloud_run_v2_service" "frontend" {
  count = local.run_count

  name                = "mooc-frontend"
  location            = var.region
  ingress             = "INGRESS_TRAFFIC_ALL"
  deletion_protection = false
  labels              = var.labels

  template {
    service_account                  = google_service_account.run_frontend[0].email
    timeout                          = "${var.cloudrun_services["frontend"].timeout}s"
    max_instance_request_concurrency = var.cloudrun_services["frontend"].concurrency

    scaling {
      min_instance_count = var.cloudrun_services["frontend"].min
      max_instance_count = var.cloudrun_services["frontend"].max
    }

    containers {
      image = "${local.image_prefix}/web:${local.run_image_tag}"

      resources {
        limits = {
          cpu    = var.cloudrun_services["frontend"].cpu
          memory = var.cloudrun_services["frontend"].memory
        }
        cpu_idle = true
      }

      # nginx escucha en el 80 (Dockerfile.web).
      ports {
        container_port = 80
      }

      # El proxy de /api apunta al backend por HTTPS y con el Host del backend: Cloud Run enruta por Host.
      env {
        name  = "API_UPSTREAM"
        value = local.backend_host
      }
      env {
        name  = "API_SCHEME"
        value = "https"
      }
      env {
        name  = "API_HOST_HEADER"
        value = local.backend_host
      }
      env {
        name  = "HLS_BASE_URL"
        value = "https://storage.googleapis.com/${google_storage_bucket.hls.name}"
      }

      startup_probe {
        tcp_socket {
          port = 80
        }
        period_seconds    = 2
        failure_threshold = 10
      }
    }
  }

  depends_on = [google_project_service.cloudrun]
}

# --- Quien puede invocar cada servicio ---------------------------------------

# Frontend y backend son publicos (el backend se protege con la sesion y el CSRF de la propia API, igual que detras del
# balanceador). Los workers NO: solo la cuenta de la suscripcion push.
resource "google_cloud_run_v2_service_iam_member" "frontend_public" {
  count = local.run_count

  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_service.frontend[0].name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

resource "google_cloud_run_v2_service_iam_member" "backend_public" {
  count = local.run_count

  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_service.backend[0].name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

resource "google_cloud_run_v2_service_iam_member" "workers_invoker" {
  count = local.run_count

  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_service.workers[0].name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.pubsub_push[0].email}"
}

# ---------------------------------------------------------------------------
# Salidas
# ---------------------------------------------------------------------------
output "cloudrun_frontend_url" {
  description = "URL publica del frontend en Cloud Run (la que usa el navegador)."
  value       = var.cloudrun_enabled ? google_cloud_run_v2_service.frontend[0].uri : ""
}

output "cloudrun_backend_url" {
  description = "URL del backend (API) en Cloud Run."
  value       = var.cloudrun_enabled ? google_cloud_run_v2_service.backend[0].uri : ""
}

output "cloudrun_workers_url" {
  description = "URL del servicio de workers. Privada: solo la cuenta de la suscripcion push puede invocarla."
  value       = var.cloudrun_enabled ? google_cloud_run_v2_service.workers[0].uri : ""
}

output "pubsub_topic" {
  description = "Topic al que el backend publica los trabajos de medios."
  value       = var.cloudrun_enabled ? google_pubsub_topic.media_jobs[0].id : ""
}
