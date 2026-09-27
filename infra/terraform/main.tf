# Proveedor y habilitacion de APIs (issue #115).

provider "google" {
  project = var.project_id
  region  = var.region
  zone    = var.zone

  # Sin `credentials`: nadie usa archivos de llave. Terraform toma las
  # Application Default Credentials de quien ejecuta, es decir la identidad
  # personal de cada integrante tras `gcloud auth application-default login`.
  # No hay ninguna credencial compartida que filtrar ni que rotar.
}

# Las APIs que la entrega necesita. Habilitarlas es idempotente, asi que el
# `apply` de un proyecto limpio las activa sin pasos manuales previos.
locals {
  required_services = [
    # Computo y red: las dos VMs, la VPC, el firewall y las IP externas.
    "compute.googleapis.com",

    # Base de datos administrada (C1).
    "sqladmin.googleapis.com",

    # Almacenamiento de objetos (C3).
    "storage.googleapis.com",

    # Registro de imagenes Docker (D1).
    "artifactregistry.googleapis.com",

    # Cuentas de servicio y sus politicas.
    "iam.googleapis.com",

    # Firma de URLs sin archivo de llave. internal/storage/gcs.go usa las
    # credenciales por defecto y `SignedURL`; desde una VM eso delega la firma
    # en signBlob de esta API. Sin ella, la API no puede emitir URLs firmadas.
    "iamcredentials.googleapis.com",

    # Conexion privada hacia Cloud SQL, que necesita C1 para no exponer la base
    # a internet.
    "servicenetworking.googleapis.com",

    # Almacen de secretos. B4 lo usara para que la aplicacion lea sus
    # credenciales en tiempo de ejecucion; mientras tanto ya sirve como el sitio
    # donde el equipo comparte la contrasena de la base, con el mismo IAM que
    # gobierna todo lo demas y dejando registro de quien la lee.
    "secretmanager.googleapis.com",

    # Politicas de agentes de observabilidad (H1, issue #131). Instala el Ops
    # Agent en las VMs existentes sin recrearlas: cambiar
    # metadata_startup_script en un google_compute_instance ya desplegado
    # fuerza su reemplazo (verificado con `terraform plan`, ver
    # docs/entrega2/evidencias/H1), asi que el agente se instala por esta via
    # -- una politica de OS Config -- en lugar de por el startup-script.
    "osconfig.googleapis.com",
  ]
}

resource "google_project_service" "required" {
  for_each = toset(local.required_services)

  project = var.project_id
  service = each.value

  # Un `destroy` no debe apagar las APIs del proyecto. Desactivarlas es lento,
  # afecta a cualquier otra cosa que dependa de ellas y no ahorra un centavo:
  # habilitar una API no cuesta nada, solo cuestan los recursos que se creen
  # con ella.
  disable_on_destroy = false
}
