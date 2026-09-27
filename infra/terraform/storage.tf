# Aprovisionamiento de Cloud Storage para la Plataforma MOOC (Issue C3 - #120)
#
# Define el bucket administrado para persistencia de objetos en la misma region (us-east1),
# previene el acceso publico directo (public_access_prevention = "enforced"),
# establece los 4 prefijos organizativos (originals, hls, documents, thumbnails),
# configura politicas de IAM diferenciadas por componente mediante condiciones CEL,
# habilita CORS para la carga directa y aplica politica de ciclo de vida para cargas incompletas.

# 1. Bucket administrado de Cloud Storage
resource "google_storage_bucket" "media" {
  name                        = var.storage_bucket_name
  location                    = var.region
  storage_class               = "STANDARD"
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
  force_destroy               = true

  labels = var.labels

  cors {
    origin          = var.storage_cors_origins
    method          = ["GET", "HEAD", "PUT", "OPTIONS"]
    response_header = ["*"]
    max_age_seconds = 3600
  }

  lifecycle_rule {
    action {
      type = "AbortIncompleteMultipartUpload"
    }
    condition {
      age = 1
    }
  }
}

# 2. Prefijos organizativos (folders virtuales en Cloud Storage)
# Reflejan las cuatro areas logicas declaradas en internal/storage/keys.go
locals {
  storage_prefixes = [
    "originals/",
    "hls/",
    "documents/",
    "thumbnails/",
  ]
}

resource "google_storage_bucket_object" "prefixes" {
  for_each = toset(local.storage_prefixes)

  name    = each.value
  content = " "
  bucket  = google_storage_bucket.media.name
}

# 3. IAM Diferenciado por Componente (Mínimo Privilegio)

# A. API (Web Server) - Lectura general y metadatos (StatObject, GetObject)
resource "google_storage_bucket_iam_member" "api_viewer" {
  bucket = google_storage_bucket.media.name
  role   = "roles/storage.objectViewer"
  member = "serviceAccount:${google_service_account.web_server.email}"
}

# B. API (Web Server) - Escritura para subidas directas autorizadas
# La API firma URLs de carga directa V4 para originales, documentos y miniaturas.
# Cloud Storage valida en tiempo de subida los permisos de la cuenta firmante.
# Mediante esta condicion CEL, se autoriza la creacion de objetos UNICAMENTE
# en esos prefijos y se DENIEGA terminantemente cualquier escritura en el prefijo hls/ (derivados).
resource "google_storage_bucket_iam_member" "api_creator_non_derivatives" {
  bucket = google_storage_bucket.media.name
  role   = "roles/storage.objectCreator"
  member = "serviceAccount:${google_service_account.web_server.email}"

  condition {
    title       = "api_deny_derivatives_write"
    description = "La cuenta de la API solo puede crear objetos en originals, documents y thumbnails, nunca en derivados HLS"
    expression  = <<-EOT
      resource.type == "storage.googleapis.com/Object" && (
        resource.name.startsWith("projects/_/buckets/${google_storage_bucket.media.name}/objects/originals/") ||
        resource.name.startsWith("projects/_/buckets/${google_storage_bucket.media.name}/objects/documents/") ||
        resource.name.startsWith("projects/_/buckets/${google_storage_bucket.media.name}/objects/thumbnails/")
      )
    EOT
  }
}

# C. Worker Server - Lectura de objetos para descarga de originales
# El worker necesita leer el video original subido para pasarlo al pipeline de transcodificacion.
resource "google_storage_bucket_iam_member" "worker_viewer" {
  bucket = google_storage_bucket.media.name
  role   = "roles/storage.objectViewer"
  member = "serviceAccount:${google_service_account.worker_server.email}"
}

# D. Worker Server - Gestion y escritura exclusiva de derivados HLS
# El worker escribe los manifiestos master.m3u8, variantes y segmentos .ts exclusivamente
# bajo el prefijo hls/. No puede escribir en ningun otro prefijo ni sobreescribir originales.
resource "google_storage_bucket_iam_member" "worker_derivatives_manager" {
  bucket = google_storage_bucket.media.name
  role   = "roles/storage.objectUser"
  member = "serviceAccount:${google_service_account.worker_server.email}"

  condition {
    title       = "worker_write_derivatives_only"
    description = "El worker solo puede crear, actualizar y gestionar objetos bajo el prefijo hls/"
    expression  = <<-EOT
      resource.type == "storage.googleapis.com/Object" &&
      resource.name.startsWith("projects/_/buckets/${google_storage_bucket.media.name}/objects/hls/")
    EOT
  }
}
