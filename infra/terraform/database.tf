# Base de datos administrada (issue #118, C1).
#
# La persistencia transaccional se traslada a Cloud SQL para PostgreSQL con
# acceso exclusivamente privado: la instancia no tiene IP publica y solo se
# alcanza desde dentro de la VPC de B3.
#
# Lo que hace posible la parte privada ya lo dejo B3 en network.tf: el rango
# reservado `private_ip_alloc` y el peering `private_vpc_connection`. Aqui solo
# se consume.

# Generacion del nombre de la instancia.
#
# Existe por una trampa de Cloud SQL: al borrar una instancia, Google **reserva
# su nombre durante una semana**. Si alguien hace `destroy` y vuelve a aplicar,
# el `create` falla con un conflicto de nombre y no hay forma de forzarlo.
#
# Subir este numero da un nombre libre sin tocar el resto de la configuracion.
# Es la unica salida practica, y tenerla declarada evita que se descubra en
# medio de una demostracion.
variable "db_instance_generation" {
  description = "Sufijo del nombre de la instancia. Subirlo si un destroy dejo el nombre anterior reservado (Google lo retiene una semana)."
  type        = string
  default     = "1"
}

# Presupuesto de conexiones de la instancia.
#
# Se declara en lugar de heredar el valor por defecto de Google, que depende de
# la memoria del perfil y cambia si el perfil cambia. Declarado, el limite es
# auditable en el codigo y el pool de la aplicacion se dimensiona contra un
# numero conocido en vez de contra uno que hay que ir a consultar.
#
# El reparto, que es lo que de verdad importa (nota 11 de NOTAS_TECNICAS.md):
#
#   API (D2)                  DB_MAX_OPEN_CONNS = 25
#   Worker (E1)               DB_MAX_OPEN_CONNS = 25
#   Reserva de superusuario                       3
#   Agentes de Google                            ~5
#   ------------------------------------------------
#   Comprometido                                 ~58   de 100
#
# Los ~42 restantes son el margen para las migraciones, para un `psql` de
# diagnostico y para el solapamiento de un despliegue --durante un reinicio
# conviven un momento el pool viejo y el nuevo--. Sumar antes de dimensionar,
# no despues del primer `too many connections` bajo carga.
variable "db_max_connections" {
  description = "Limite de conexiones de la instancia. Debe cubrir el pool de la API mas el del worker con margen."
  type        = number
  default     = 100
}

# Encendida o detenida.
#
# Existe por disciplina de coste, no por comodidad: la instancia son 49,31
# USD/mes de computo y el credito por integrante es de 50 USD, asi que dejarla
# encendida entre pruebas se lleva un cupon completo en un mes. Deberia existir
# para las corridas y estar detenida el resto del tiempo.
#
# **Se cambia aqui, en el codigo, y se aplica desde main.** Deliberadamente NO
# se usa `-var` ni terraform.tfvars, por el mismo motivo que el resto de
# variables trae su valor por defecto en el codigo: si el valor viviera en el
# entorno de cada uno, el `apply` de quien no lo exportara volveria a encender
# la instancia sin que nadie se entere, y el aviso llegaria en la factura.
#
#   ALWAYS = encendida
#   NEVER  = detenida (no se factura computo; el almacenamiento y las copias si)
#
# Antes de detenerla por primera vez hay que contrastar el ahorro real contra el
# informe de facturacion, que es lo que pide la regla 5 de
# docs/entrega2/CONFIGURACION_Y_COSTOS.md.
variable "db_activation_policy" {
  description = "ALWAYS para tener la instancia encendida, NEVER para detenerla y dejar de pagar computo."
  type        = string
  default     = "ALWAYS"

  validation {
    condition     = contains(["ALWAYS", "NEVER"], var.db_activation_policy)
    error_message = "Solo ALWAYS (encendida) o NEVER (detenida)."
  }
}

resource "google_sql_database_instance" "main" {
  name             = "mooc-db-${var.db_instance_generation}"
  database_version = "POSTGRES_16"
  region           = var.region
  project          = var.project_id

  # Sin esto, el `create` compite con el peering y falla con un error que no
  # menciona la red. Es la dependencia menos evidente de todo el archivo.
  depends_on = [
    google_service_networking_connection.private_vpc_connection,
    google_project_service.required,
  ]

  # Que un `terraform destroy` no pueda llevarse la base por delante. Tres
  # personas aplican sobre este mismo estado y la perdida seria irreversible.
  #
  # Para el cierre de la entrega (issue #133, I6) hay que ponerlo en false,
  # aplicar, y solo entonces destruir. El procedimiento esta en ADMINISTRACION.md.
  deletion_protection = false

  settings {
    # Las decisiones de B1, justificadas en
    # docs/entrega2/CONFIGURACION_Y_COSTOS.md: 1 vCPU dedicada y 3,75 GiB es el
    # minimo dedicado del catalogo. Los perfiles de nucleo compartido
    # (db-f1-micro, db-g1-small) estan excluidos del SLA y su CPU con rafagas
    # haria que el escenario 1 midiera creditos de rafaga en lugar de la
    # plataforma.
    tier    = "db-custom-1-3840"
    edition = "ENTERPRISE"

    # Una sola zona, sin replicas. Lo exige el enunciado y encaja con el
    # ejercicio: cero disponibilidad y cero escalabilidad por diseno.
    availability_type = "ZONAL"

    # Ver el comentario de la variable: detener la instancia entre corridas es
    # lo que hace que el credito alcance.
    activation_policy = var.db_activation_policy

    # La misma zona que las VMs. El trafico entre zonas de una region se
    # factura; el de dentro de una zona, no.
    location_preference {
      zone = var.zone
    }

    disk_type = "PD_SSD"
    disk_size = 10

    # El crecimiento automatico esta activo pero con techo. Un disco lleno deja
    # la instancia inaccesible y requiere intervencion manual; un crecimiento
    # sin limite se lleva el credito en silencio. 20 GiB es el doble de lo
    # estimado: margen suficiente para no quedarse parado y poco suficiente
    # para no arruinar la estimacion.
    disk_autoresize       = true
    disk_autoresize_limit = 20

    user_labels = var.labels

    # --- Acceso exclusivamente privado -------------------------------------
    ip_configuration {
      # El criterio de aceptacion literal: sin IP publica. Con esto, «desde
      # fuera no funciona» no es una regla de firewall que alguien pueda
      # cambiar, es que no hay a donde conectarse.
      ipv4_enabled = false

      private_network = google_compute_network.vpc.id

      # SSL obligatorio. ENCRYPTED_ONLY rechaza en el servidor cualquier
      # conexion en claro, asi que no depende de que el cliente pida cifrado.
      #
      # No se exige certificado de cliente
      # (TRUSTED_CLIENT_CERTIFICATE_REQUIRED) porque obliga a distribuir y
      # rotar certificados por VM, y lo que aporta --autenticar al cliente-- ya
      # lo da el acceso privado: para llegar al puerto hay que estar dentro de
      # la VPC.
      ssl_mode = "ENCRYPTED_ONLY"
    }

    # --- Copias de seguridad ------------------------------------------------
    backup_configuration {
      enabled = true

      # 03:00 UTC, de madrugada en Bogota y fuera de cualquier ventana de
      # pruebas de carga.
      start_time = "03:00"

      # Tres dias. El almacenamiento de copias se factura por tamano usado y
      # esta base ronda los megabytes, asi que el coste queda por debajo del
      # centimo al mes: no altera la estimacion de B1.
      backup_retention_settings {
        retained_backups = 3
        retention_unit   = "COUNT"
      }

      # Recuperacion a un punto en el tiempo desactivada: guarda el registro de
      # escritura de forma continua y ese si es un coste que crece con la
      # actividad. Para un ejercicio academico la copia diaria sobra.
      point_in_time_recovery_enabled = false
    }

    # --- Mantenimiento ------------------------------------------------------
    # Domingo a las 6:00 UTC (1:00 en Bogota). Google reinicia la instancia
    # durante la ventana; fijarla evita que ocurra en medio de una medicion.
    maintenance_window {
      day          = 7
      hour         = 6
      update_track = "stable"
    }

    # --- Observabilidad -----------------------------------------------------
    # Query Insights no se factura y da justo lo que el informe de capacidad
    # tiene que registrar: consultas lentas y carga de la base. Sin esto, la
    # unica fuente sobre el comportamiento de la base durante el escenario 1
    # serian las metricas de CPU.
    insights_config {
      query_insights_enabled  = true
      record_application_tags = true
      record_client_address   = false # no hacen falta para el informe
    }

    database_flags {
      name  = "max_connections"
      value = tostring(var.db_max_connections)
    }
  }
}

# La base de datos de la aplicacion.
#
# Mismo nombre que en local (`moocdb`) a proposito: asi la unica diferencia
# entre la cadena de conexion de desarrollo y la de la nube son el host, la
# contrasena y el sslmode. Cuanto menos cambie, menos hay que recordar al
# depurar.
resource "google_sql_database" "mooc" {
  name     = "moocdb"
  instance = google_sql_database_instance.main.name
  project  = var.project_id

  # ABANDON: si alguien quita este recurso del codigo, Terraform lo saca del
  # estado en vez de borrar la base con todos sus datos. Para el borrado real
  # se elimina la instancia entera en I6, que es una decision explicita.
  deletion_policy = "ABANDON"
}

# El usuario de la aplicacion.
#
# La contrasena entra por TF_VAR_db_password y sale de Secret Manager; no vive
# en el repositorio. Si queda en claro en el estado de Terraform, y por eso el
# estado vive en un bucket privado y versionado (ver ADMINISTRACION.md).
resource "google_sql_user" "app" {
  name     = "moocuser"
  instance = google_sql_database_instance.main.name
  # gcloud on Windows writes CRLF. Git Bash command substitution removes LF
  # but can leave CR, which made the shared state differ from Secret Manager
  # and proposed a password change on every plan.
  password = trimspace(var.db_password)
  project  = var.project_id
}

# ---------------------------------------------------------------------------
# Quien puede leer la contrasena de la base
# ---------------------------------------------------------------------------
#
# El secreto NO se declara aqui: lo creo a mano el issue B4 y su ciclo de vida
# --crearlo, rotarlo, anadir versiones-- vive en ADMINISTRACION.md. Meterlo en
# Terraform obligaria a que su valor pasara por el estado, que es justo lo que
# se evito. Se referencia con un data source y ya.
data "google_secret_manager_secret" "db_password" {
  secret_id = "db-password"
  project   = var.project_id
}

# Las dos VMs necesitan leer la contrasena al arrancar, porque el Compose la
# inyecta en DATABASE_URL en tiempo de ejecucion en lugar de llevarla en la
# imagen (issue #117). Antes de esto solo la podian leer los tres integrantes
# como usuarios, asi que **D2 y E1 se habrian quedado sin poder arrancar**: el
# sintoma habria sido un contenedor que no levanta por una variable vacia, lejos
# de la causa.
#
# Con alcance al secreto y no al proyecto: `secretAccessor` a nivel de proyecto
# daria acceso a las credenciales de SMTP del issue #126, que la API necesita y
# el worker no. El principio es el mismo que sostiene las dos cuentas separadas.
#
# Sigue siendo de solo lectura de un unico secreto: ninguna VM puede crear
# versiones nuevas ni cambiar la contrasena. Rotarla es una tarea de
# administracion, no algo que una maquina pueda hacer sola.
resource "google_secret_manager_secret_iam_member" "db_password_readers" {
  for_each = {
    web    = google_service_account.web_server.email
    worker = google_service_account.worker_server.email
  }

  project   = var.project_id
  secret_id = data.google_secret_manager_secret.db_password.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${each.value}"
}
