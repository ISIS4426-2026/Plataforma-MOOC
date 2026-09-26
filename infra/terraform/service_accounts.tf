# Cuentas de servicio con permisos diferenciados (issue #115).
#
# Una cuenta por componente, y no una compartida, porque es lo que hace que un
# fallo en un componente no herede los permisos del otro. El comentario de
# internal/storage/gcs.go lo dice desde el lado del codigo: «la cuenta de la API
# puede firmar y leer, la del worker escribe derivados. Esa division la impone
# IAM sobre el bucket, no este codigo, asi que un error aqui no puede entregarle
# a un cliente mas derechos de los que tiene su cuenta de servicio».
#
# Ninguna de las dos tiene archivo de llave. Las VMs las llevan adjuntas y
# obtienen credenciales del servidor de metadatos, asi que no hay ningun secreto
# que guardar, rotar ni filtrar.

resource "google_service_account" "web_server" {
  project      = var.project_id
  account_id   = "sa-web-server"
  display_name = "Web Server: API modular y proxy inverso"
  description  = "Identidad de la VM que ejecuta la API. Firma URLs y lee objetos; no escribe derivados."
}

resource "google_service_account" "worker_server" {
  project      = var.project_id
  account_id   = "sa-worker-server"
  display_name = "Worker Server: workers, FFmpeg y la cola asynq"
  description  = "Identidad de la VM que procesa multimedia. Escribe derivados; no firma URLs."
}

# ---------------------------------------------------------------------------
# Permisos comunes: hablar con la base y reportar telemetria.
# ---------------------------------------------------------------------------

locals {
  # cloudsql.client permite abrir la conexion, no administrar la instancia. Ni
  # la API ni el worker pueden borrar la base, cambiar su tamano ni leer sus
  # respaldos.
  common_roles = [
    "roles/cloudsql.client",
    "roles/logging.logWriter",
    "roles/monitoring.metricWriter",
  ]

  service_accounts = {
    web    = google_service_account.web_server.email
    worker = google_service_account.worker_server.email
  }

  # Producto cartesiano cuenta x rol, para no repetir seis bloques casi
  # identicos.
  common_bindings = {
    for pair in setproduct(keys(local.service_accounts), local.common_roles) :
    "${pair[0]}-${replace(pair[1], "roles/", "")}" => {
      member = "serviceAccount:${local.service_accounts[pair[0]]}"
      role   = pair[1]
    }
  }
}

resource "google_project_iam_member" "common" {
  for_each = local.common_bindings

  project = var.project_id
  role    = each.value.role
  member  = each.value.member
}

# ---------------------------------------------------------------------------
# Lo que distingue a la API: firmar URLs.
# ---------------------------------------------------------------------------

# internal/storage/gcs.go firma con las credenciales por defecto y sin archivo
# de llave. Desde una VM no hay clave privada disponible, asi que la libreria
# delega la firma en signBlob de IAM Credentials, y eso exige que la cuenta
# pueda suplantarse a si misma.
#
# El permiso se concede SOBRE LA PROPIA CUENTA, no sobre el proyecto: la API
# puede firmar como ella misma y como nadie mas. Concederlo a nivel de proyecto
# le permitiria suplantar tambien a la del worker, que es justo la separacion
# que estas dos cuentas existen para mantener.
resource "google_service_account_iam_member" "web_server_can_sign" {
  service_account_id = google_service_account.web_server.name
  role               = "roles/iam.serviceAccountTokenCreator"
  member             = "serviceAccount:${google_service_account.web_server.email}"
}

# El worker no recibe este permiso, y es deliberado: no emite URLs firmadas.
# Sube derivados directamente con las credenciales de su propia cuenta.

# ---------------------------------------------------------------------------
# Pendiente para C3
# ---------------------------------------------------------------------------
#
# Los permisos sobre el bucket --la API lee, el worker escribe derivados-- se
# conceden cuando el bucket exista, en el issue C3 (#120), porque una politica
# de IAM necesita el recurso al que se aplica. La forma prevista es:
#
#   * cuenta de la API    -> lectura de objetos del bucket
#   * cuenta del worker   -> lectura y escritura, acotada al prefijo de
#                            derivados en la medida en que el proveedor lo
#                            permita
#
# Hasta entonces ninguna de las dos cuentas puede tocar el almacenamiento, que
# es el valor por defecto correcto: sin permiso explicito, no hay acceso.
