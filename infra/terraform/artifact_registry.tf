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

resource "google_artifact_registry_repository_iam_member" "runtime_readers" {
  for_each = {
    api    = google_service_account.web_server.email
    worker = google_service_account.worker_server.email
  }

  project    = var.project_id
  location   = google_artifact_registry_repository.images.location
  repository = google_artifact_registry_repository.images.name
  role       = "roles/artifactregistry.reader"
  member     = "serviceAccount:${each.value}"
}