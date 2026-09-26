# Variables de la infraestructura (issue #115).
#
# Todo lo que no es secreto lleva valor por defecto, y los defectos son las
# decisiones ya tomadas y justificadas en docs/entrega2/CONFIGURACION_Y_COSTOS.md
# (issue #114). La razon es practica: si el proyecto, la region o la zona
# vivieran solo en el terraform.tfvars de cada uno --que no se versiona-- cada
# integrante podria aplicar contra una configuracion distinta sin enterarse.
#
# Con defectos en el codigo, lo unico que cada persona aporta desde su entorno es
# la contrasena de la base.

variable "project_id" {
  description = "Proyecto de GCP donde vive toda la infraestructura de la entrega."
  type        = string
  default     = "plataforma-mooc-entrega2"
}

variable "region" {
  description = <<-EOT
    Region unica de la entrega. us-east1 se eligio en B1 por dos razones
    verificadas: esta en el nivel de precios mas bajo, y es la region de menor
    latencia medida desde Bogota (74 ms, frente a 178 ms de Sao Paulo).
  EOT
  type        = string
  default     = "us-east1"
}

variable "zone" {
  description = <<-EOT
    Zona unica para las dos VMs y la base de datos.

    Una sola zona, y la misma para todo, por dos motivos que apuntan igual: el
    enunciado exige la base en una sola zona de disponibilidad y el ejercicio
    tiene cero disponibilidad por diseno; y el trafico entre zonas de una misma
    region se factura, mientras que el de dentro de una zona no.
  EOT
  type        = string
  default     = "us-east1-b"
}

variable "db_password" {
  description = <<-EOT
    Contrasena del usuario de la base de datos.

    Sin valor por defecto y sin pasar nunca por el repositorio: se entrega por
    entorno, con TF_VAR_db_password. Ojo, esto no la oculta del estado de
    Terraform, que la guarda en claro -- por eso el estado vive en un bucket
    privado y versionado, y nunca se versiona en git.
  EOT
  type        = string
  sensitive   = true
}

variable "labels" {
  description = <<-EOT
    Etiquetas aplicadas a todo recurso que las admita.

    Sirven para atribuir el consumo en los informes de facturacion, que es lo
    que despues permite contrastar el gasto observado contra la estimacion de
    B1.
  EOT
  type        = map(string)
  default = {
    proyecto = "plataforma-mooc"
    entrega  = "2"
    gestion  = "terraform"
  }
}
