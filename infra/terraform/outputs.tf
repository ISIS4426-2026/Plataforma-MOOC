# Salidas (issue #115).
#
# Son las que los issues siguientes consumen para adjuntar las cuentas a sus
# VMs (D2 y E1) y para conceder los permisos sobre el bucket (C3). Nada de lo
# que sale por aqui es secreto: son identificadores, no credenciales.

output "project_id" {
  description = "Proyecto sobre el que se aplico la configuracion."
  value       = var.project_id
}

output "region" {
  description = "Region unica de la entrega."
  value       = var.region
}

output "zone" {
  description = "Zona unica de la entrega."
  value       = var.zone
}

output "web_server_service_account" {
  description = "Cuenta de servicio de la VM que ejecuta la API. Se adjunta a la instancia en D2."
  value       = google_service_account.web_server.email
}

output "worker_server_service_account" {
  description = "Cuenta de servicio de la VM de workers. Se adjunta a la instancia en E1."
  value       = google_service_account.worker_server.email
}

output "enabled_services" {
  description = "APIs habilitadas en el proyecto."
  value       = sort([for s in google_project_service.required : s.service])
}

output "vpc_name" {
  description = "Nombre de la VPC principal."
  value       = google_compute_network.vpc.name
}

output "vpc_id" {
  description = "ID de la VPC principal."
  value       = google_compute_network.vpc.id
}

output "subnet_name" {
  description = "Nombre de la subred para las VMs."
  value       = google_compute_subnetwork.subnet.name
}

output "subnet_id" {
  description = "ID de la subred para las VMs."
  value       = google_compute_subnetwork.subnet.id
}

output "subnet_cidr" {
  description = "Rango CIDR de la subred de las VMs."
  value       = google_compute_subnetwork.subnet.ip_cidr_range
}

output "private_vpc_connection_id" {
  description = "ID de la conexión de peering de red privada para Cloud SQL (C1)."
  value       = google_service_networking_connection.private_vpc_connection.id
}

# --- Almacenamiento de objetos (issue #120, C3) ------------------------------

output "storage_bucket_name" {
  description = "Nombre del bucket de Cloud Storage para persistencia multimedia y documentos (C3)."
  value       = google_storage_bucket.media.name
}

output "storage_bucket_url" {
  description = "URL gs:// del bucket de Cloud Storage (C3)."
  value       = google_storage_bucket.media.url
}

# --- Base de datos administrada (issue #118, C1) -----------------------------
#
# Nada de lo que sale por aqui es secreto. La contrasena no es una salida a
# proposito: vive en Secret Manager y en el estado, y no hay razon para que
# ademas la imprima `terraform output`.

output "db_instance_name" {
  description = "Nombre de la instancia de Cloud SQL."
  value       = google_sql_database_instance.main.name
}

output "db_connection_name" {
  description = <<-EOT
    Identificador de conexion (proyecto:region:instancia). Lo consume el proxy
    de autenticacion de Cloud SQL y aparece en los comandos de `gcloud sql`.
  EOT
  value       = google_sql_database_instance.main.connection_name
}

output "db_private_ip" {
  description = <<-EOT
    IP privada de la instancia. Es la unica direccion que tiene: el host de
    DATABASE_URL en las VMs de D2 y E1.
  EOT
  value       = google_sql_database_instance.main.private_ip_address
}

output "db_public_ip" {
  description = <<-EOT
    Debe salir vacio. Es la comprobacion en codigo del criterio «sin IP
    publica»: si algun dia trae valor, alguien activo ipv4_enabled.
  EOT
  value       = google_sql_database_instance.main.public_ip_address
}

output "db_name" {
  description = "Nombre de la base de datos de la aplicacion."
  value       = google_sql_database.mooc.name
}

output "db_user" {
  description = "Usuario de la aplicacion. La contrasena va aparte, por Secret Manager."
  value       = google_sql_user.app.name
}

output "db_max_connections" {
  description = <<-EOT
    Limite de conexiones declarado en la instancia. Es el numero contra el que
    hay que dimensionar DB_MAX_OPEN_CONNS en la API y en el worker; el reparto
    esta en los comentarios de database.tf.
  EOT
  value       = var.db_max_connections
}

output "database_url_template" {
  description = <<-EOT
    Cadena de conexion lista para las VMs, con la contrasena como marcador.
    Quien despliega sustituye CONTRASENA por el valor de Secret Manager.

    El `sslmode=require` no es decorativo: la instancia esta en ENCRYPTED_ONLY
    y rechaza cualquier conexion en claro, y el arranque en produccion tambien
    lo exige (internal/config/validate.go).
  EOT
  value = format(
    "postgres://%s:CONTRASENA@%s:5432/%s?sslmode=require",
    google_sql_user.app.name,
    google_sql_database_instance.main.private_ip_address,
    google_sql_database.mooc.name,
  )
}

# --- Maquinas Virtuales de Computo (issue #123, D2) -------------------------

output "web_server_name" {
  description = "Nombre de la instancia de cómputo del Web Server (D2)."
  value       = google_compute_instance.web_server.name
}

output "web_server_public_ip" {
  description = "Dirección IPv4 pública estática del Web Server (D2)."
  value       = google_compute_address.web_server_ip.address
}

output "web_server_private_ip" {
  description = "Dirección IPv4 privada en mooc-subnet del Web Server (D2)."
  value       = google_compute_instance.web_server.network_interface[0].network_ip
}

# --- Maquinas Virtuales de Computo - Worker Server (issue #124, E1) ---------

output "worker_server_name" {
  description = "Nombre de la instancia de cómputo del Worker Server (E1)."
  value       = google_compute_instance.worker_server.name
}

output "worker_server_public_ip" {
  description = "Dirección IPv4 pública estática del Worker Server (E1)."
  value       = google_compute_address.worker_server_ip.address
}

output "worker_server_private_ip" {
  description = "Dirección IPv4 privada en mooc-subnet del Worker Server (E1)."
  value       = google_compute_instance.worker_server.network_interface[0].network_ip
}


