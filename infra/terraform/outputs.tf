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
