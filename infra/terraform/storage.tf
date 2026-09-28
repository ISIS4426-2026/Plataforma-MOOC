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

# D. Worker Server - Escritura de derivados HLS, en su propio bucket
#
# Los derivados viven en un bucket aparte desde #167, asi que aqui el worker ya
# no escribe nada: sobre el bucket de media conserva solo la lectura de C, que
# es lo que necesita para descargar el original. El permiso de escritura esta
# mas abajo, acotado al bucket publico.
#
# La condicion CEL que antes acotaba al prefijo hls/ desaparece porque ya no
# hace falta: el bucket entero es de derivados, y un permiso que no puede
# desbordarse es mas facil de razonar que uno que se contiene con una expresion.

# ---------------------------------------------------------------------------
# 4. Bucket de derivados HLS: el unico publico (#167)
# ---------------------------------------------------------------------------
#
# Por que existe, en una linea: un manifiesto HLS referencia sus variantes por
# ruta relativa, el reproductor no hereda la query string donde viaja la firma, y
# un prefijo publico dentro de un bucket privado no es representable en IAM
# --las condiciones no se admiten en enlaces a allUsers--. La nota 1b de
# NOTAS_TECNICAS.md tiene el analisis completo y G3 lo midio contra el bucket
# real.
#
# Lo que se gana separandolos: el bucket de media mantiene
# public_access_prevention = "enforced", de modo que lo que subio el autor, los
# documentos y las miniaturas siguen sin poder exponerse ni por error. Publico es
# unicamente lo que el worker genera, que es material derivado de contenido que
# el estudiante inscrito ya puede ver.
resource "google_storage_bucket" "hls" {
  name                        = var.storage_hls_bucket_name
  location                    = var.region
  storage_class               = "STANDARD"
  uniform_bucket_level_access = true

  # "inherited", no "enforced": este bucket existe precisamente para servir sin
  # firma. Es la unica excepcion del proyecto y esta acotada a los derivados.
  public_access_prevention = "inherited"

  force_destroy = true

  labels = var.labels

  # Un reproductor en una pagina de otro origen pide el manifiesto y los
  # segmentos por XHR, asi que sin CORS el navegador los rechaza aunque el
  # objeto sea publico.
  cors {
    origin          = var.storage_cors_origins
    method          = ["GET", "HEAD", "OPTIONS"]
    response_header = ["*"]
    max_age_seconds = 3600
  }
}

# Lectura publica, sin condicion: el bucket entero es contenido derivado.
resource "google_storage_bucket_iam_member" "hls_public_read" {
  bucket = google_storage_bucket.hls.name
  role   = "roles/storage.objectViewer"
  member = "allUsers"
}

# El worker es el unico que escribe aqui. La API no aparece: firma cargas de
# originales, y los derivados no los produce nadie mas que el pipeline.
resource "google_storage_bucket_iam_member" "hls_worker_writer" {
  bucket = google_storage_bucket.hls.name
  role   = "roles/storage.objectUser"
  member = "serviceAccount:${google_service_account.worker_server.email}"
}

output "storage_hls_bucket" {
  description = <<-EOT
    Bucket de los derivados HLS, de lectura publica. Es el valor de
    MEDIA_HLS_BUCKET en el Worker Server, y la base de la URL que consume un
    reproductor: https://storage.googleapis.com/<bucket>/hls/<stable_id>/master.m3u8
  EOT
  value       = google_storage_bucket.hls.name
}
