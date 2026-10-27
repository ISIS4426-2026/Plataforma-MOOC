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

variable "storage_bucket_name" {
  description = <<-EOT
    Nombre unico del bucket de Cloud Storage para almacenar originales,
    derivados HLS, documentos y miniaturas (C3).
  EOT
  type        = string
  default     = "plataforma-mooc-entrega2-media"
}

variable "storage_hls_bucket_name" {
  description = <<-EOT
    Nombre del bucket de los derivados HLS, que es **de lectura publica** (#167).

    Va aparte del bucket de media a la fuerza, no por gusto: un reproductor
    resuelve las variantes de un manifiesto por ruta relativa y no hereda la
    firma, asi que el prefijo hls/ tiene que ser legible sin firmar. IAM no
    admite condiciones en enlaces concedidos a allUsers, de modo que un prefijo
    publico dentro de un bucket privado no se puede expresar. Separarlos es lo
    que permite que los originales, los documentos y las miniaturas conserven
    public_access_prevention = "enforced".
  EOT
  type        = string
  default     = "plataforma-mooc-entrega2-hls"
}

variable "storage_cors_origins" {
  description = <<-EOT
    Lista de origenes autorizados en la configuracion CORS para permitir
    cargas directas desde clientes web / navegadores externos (C3).
  EOT
  type        = list(string)
  default     = ["*"]
}

# ---------------------------------------------------------------------------
# Lo que el arranque de cada instancia necesita saber (B3 - issue #202)
# ---------------------------------------------------------------------------
# Estas cuatro viajan a la instancia por metadata y las lee
# `infra/bootstrap/mooc-bootstrap.sh` en tiempo de ejecucion. Antes de B3 vivian
# en `/etc/mooc/<rol>.conf`, escrito a mano dentro de cada VM: configuracion que
# solo existe en un disco y que una recreacion borra (nota tecnica 19).

variable "image_tag" {
  description = <<-EOT
    SHA completo del commit cuya imagen corre en las instancias.

    Es el SHA y no una etiqueta movil a proposito: con `latest`, dos instancias
    del mismo grupo pueden quedarse en imagenes distintas segun cuando
    arrancaron. El mismo valor pin la imagen del contenedor Y el commit que el
    arranque saca del repositorio, asi que el codigo y los ficheros de
    despliegue son siempre del mismo commit.
  EOT
  type        = string

  validation {
    condition     = can(regex("^[0-9a-f]{40}$", var.image_tag))
    error_message = "image_tag debe ser el SHA completo de 40 caracteres hexadecimales, no una etiqueta ni un SHA corto."
  }
}

variable "app_domain" {
  description = <<-EOT
    Dominio con el que responde la aplicacion.

    Vacio se calcula como `<ip-publica-de-la-vm-web>.sslip.io`, que es lo que la
    Entrega 2 usaba escrito a mano. Cuando C3 ponga el balanceador delante, el
    dominio sera el suyo y entonces hay que declararlo aqui.
  EOT
  type        = string
  default     = ""
}

# Estas dos no son secretos --la contrasena de SMTP vive en Secret Manager-- pero
# identifican la cuenta del equipo, y el repositorio es publico. Por eso no
# tienen valor por omision: se pasan con TF_VAR_smtp_from y TF_VAR_smtp_username,
# igual que TF_VAR_db_password. Un valor ausente para el `apply`, que es mejor
# que una instancia que arranca sin poder enviar correo.
variable "smtp_from" {
  description = "Remitente verificado en el proveedor de correo. Se pasa con TF_VAR_smtp_from."
  type        = string
}

variable "smtp_username" {
  description = "Login SMTP del proveedor de correo. Se pasa con TF_VAR_smtp_username."
  type        = string
}
