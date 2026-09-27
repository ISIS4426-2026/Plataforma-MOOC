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

