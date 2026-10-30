# Registro de imagenes de API y worker (D1). Las etiquetas por commit son
# inmutables para que un despliegue nunca cambie de contenido bajo el mismo tag.
resource "google_artifact_registry_repository" "images" {
  project       = var.project_id
  location      = var.region
  repository_id = "mooc"
  description   = "Immutable API and worker images for Plataforma MOOC"
  format        = "DOCKER"

  docker_config {
    immutable_tags = true
  }
  depends_on = [google_project_service.required["artifactregistry.googleapis.com"]]
}

# LA COLA TAMBIEN LEE EL REGISTRO, DESDE C6 (#209)
#
# B4 (#203) inventario la cuenta de la cola y escribio que «no lee secretos y no
# baja imagenes». La primera mitad sigue siendo cierta; la segunda deja de serlo
# aqui: el exportador que alimenta al autoescalador del grupo de workers corre en
# la VM de la cola --tiene que vivir fuera de los grupos elasticos-- y es un
# binario de la imagen `worker`.
#
# Lo que NO se le concede, y es lo que hace que esto no deshaga la separacion de
# B2: `artifactregistry.reader` es de solo lectura sobre el repositorio. La cola
# puede bajar la imagen y no puede publicar ninguna, ni leer el secreto de SMTP,
# ni abrir la base, ni firmar URLs.
resource "google_artifact_registry_repository_iam_member" "runtime_readers" {
  for_each = {
    api    = google_service_account.web_server.email
    worker = google_service_account.worker_server.email
    queue  = google_service_account.queue_server.email
  }

  project    = var.project_id
  location   = google_artifact_registry_repository.images.location
  repository = google_artifact_registry_repository.images.name
  role       = "roles/artifactregistry.reader"
  member     = "serviceAccount:${each.value}"
}