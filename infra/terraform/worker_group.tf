# ==============================================================================
# Grupo administrado de workers (C5 - issue #208)
# ==============================================================================
# Plantilla, comprobacion de salud y grupo de los workers de procesamiento
# multimedia. El tamano lo decidira el autoescalador de C6 sobre la senal de
# cola; aqui el grupo nace con una instancia y una politica de autohealing.
#
# ZONAL, por lo mismo que el grupo de la API (C1): Cloud SQL es
# `availability_type = "ZONAL"` y la cola compartida es una VM, las dos en
# `us-east1-b`. Un grupo regional de workers no sobrevivira a la caida de esa
# zona --se queda sin cola de la que consumir-- y si pagaria trafico entre zonas.
# ==============================================================================

# ------------------------------------------------------------------------------
# La comprobacion de salud: proceso vivo Y conectado a la cola
# ------------------------------------------------------------------------------
# El criterio que pide C5 no se puede comprobar con un sondeo de TCP ni con
# `/metrics`: las dos cosas pasan igual con un worker que vive y no alcanza la
# cola, que es una instancia ocupada sin hacer trabajo.
#
# Por eso C5 anadio `/readyz` al worker (`internal/worker/readiness.go`), que
# hace PING sobre **la misma conexion** con la que consume. Y por eso el
# autohealing mira `/readyz` y no `/healthz`.
#
# La contrapartida, declarada: si la cola cae, **todas** las instancias se
# declaran no sanas a la vez y el grupo intentara recrearlas. No arregla nada
# --el problema esta fuera-- pero tampoco empeora el servicio, porque sin cola no
# hay trabajo. Es una consecuencia mas de que la cola sea punto unico de fallo
# (nota tecnica 30).
resource "google_compute_health_check" "worker" {
  name        = "mooc-worker-ready"
  description = "Salud del worker: proceso vivo y conectado a la cola (C5)"

  # Mas tolerante que la de la API, y a proposito. Un worker no atiende
  # peticiones de usuarios: que tarde unos segundos de mas en declararse enfermo
  # no le cuesta a nadie una pagina de error, y a cambio evita recrear una
  # maquina por un hipo de la cola. Es la leccion de la nota 40 aplicada aqui.
  check_interval_sec  = 15
  timeout_sec         = 5
  healthy_threshold   = 2
  unhealthy_threshold = 4

  http_health_check {
    request_path = "/readyz"
    port         = 9090
  }

  log_config {
    enable = true
  }
}

# ------------------------------------------------------------------------------
# Entrada al 9090, que B2 no habia previsto
# ------------------------------------------------------------------------------
# B2 dejo escrito «ninguna entrada hacia los workers: nadie les habla». Era
# cierto entonces y **deja de serlo con el autohealing**: la comprobacion de
# salud llega desde los rangos de Google y tiene que alcanzar el 9090.
#
# Tambien decia «ninguna regla para el 9090 de metricas, porque el agente de
# operaciones raspa localhost». Eso sigue siendo cierto para las **metricas**;
# lo que se abre aqui es la **sonda**, que comparte puerto con ellas porque el
# worker no tiene otro servidor HTTP.
#
# Acotada a los dos rangos de comprobacion de salud y solo a la etiqueta del
# grupo: ni el balanceador ni la subred entran por aqui. Corregido en la nota 31.
resource "google_compute_firewall" "allow_worker_health" {
  name        = "mooc-allow-worker-health"
  network     = google_compute_network.vpc.name
  description = "Comprobacion de salud de Google al 9090 de los workers (C5)"

  allow {
    protocol = "tcp"
    ports    = ["9090"]
  }

  source_ranges = ["130.211.0.0/22", "35.191.0.0/16"]
  target_tags   = ["mooc-worker-node"]
}

# ------------------------------------------------------------------------------
# Plantilla de instancia
# ------------------------------------------------------------------------------
resource "google_compute_instance_template" "worker" {
  name_prefix = "mooc-worker-"
  description = "Plantilla de las instancias del grupo de workers (C5)"
  region      = var.region

  # El mismo tipo que la VM fija que sustituye, para que G4 pueda comparar
  # contra la linea base de A7 sin que el tipo de maquina sea otra variable.
  # A7 midio el worker al 96,31 % de CPU media durante casi dos horas: es la
  # maquina la que se agota, y por eso C6 escala instancias.
  machine_type = "e2-highcpu-2"

  labels = var.labels

  tags = ["mooc-worker-node", "allow-iap-ssh"]

  # 30 GiB, como la VM fija. No es holgura: el volumen de trabajo temporal
  # guarda el original que se esta transcodificando, y el perfil largo son
  # 39 MB por tarea con dos en vuelo. Lo que llena el disco no es el tamano de
  # un original sino los restos de tareas interrumpidas, y de eso se encarga la
  # limpieza del arranque de la unidad (B3).
  disk {
    source_image = "debian-cloud/debian-12"
    disk_size_gb = 30
    disk_type    = "pd-balanced"
    auto_delete  = true
    boot         = true
    labels       = var.labels
  }

  # **Sin IP externa.** Un worker no recibe trafico de nadie: saca tareas de la
  # cola. Su salida va por el Cloud NAT.
  network_interface {
    subnetwork = google_compute_subnetwork.subnet.id
  }

  # `sa-worker-server`, que NO es la de la API. Escribe derivados en el bucket
  # HLS y lee originales, pero no puede firmar URLs ni leer el secreto de SMTP:
  # esa separacion es la que hace que una credencial filtrada del worker no
  # alcance lo de la API (B2 §4).
  service_account {
    email  = google_service_account.worker_server.email
    scopes = ["https://www.googleapis.com/auth/cloud-platform"]
  }

  metadata = merge(local.mooc_metadata_comun, local.mooc_metadata_worker)

  scheduling {
    automatic_restart   = true
    on_host_maintenance = "MIGRATE"
    preemptible         = false
  }

  lifecycle {
    create_before_destroy = true
  }

  depends_on = [
    google_compute_router_nat.nat,
    google_compute_address.queue_internal_ip,
  ]
}

# ------------------------------------------------------------------------------
# El grupo
# ------------------------------------------------------------------------------
resource "google_compute_instance_group_manager" "worker" {
  name               = "mooc-worker-group"
  base_instance_name = "mooc-worker"
  zone               = var.zone
  description        = "Grupo administrado de workers (C5). El tamano lo decide el autoescalador de C6."

  version {
    instance_template = google_compute_instance_template.worker.self_link
  }

  # Una instancia. El rango 1-3 lo impone el autoescalador de C6.
  target_size = 1

  # Sin `named_port`: a los workers no les llega trafico de ningun balanceador.

  auto_healing_policies {
    health_check = google_compute_health_check.worker.self_link

    # Mas que los 300 s de la API, y por una razon medida: el arranque del
    # worker instala lo mismo que el de la API --B3 midio 62-66 s-- pero ademas
    # baja una imagen que lleva FFmpeg dentro, que es sensiblemente mayor.
    #
    # Y el lado seguro es el mismo: por debajo del arranque real, el autohealer
    # mata instancias que se estan montando y las sustituye por otras que haran
    # lo mismo. Hay que calibrarlo con `/var/lib/mooc/arranque.json` del rol
    # worker en cuanto exista una medida, que es parte de lo que C5 demuestra.
    initial_delay_sec = 420
  }

  # ROTAR UN WORKER NO ES ROTAR UNA REPLICA DE LA API.
  #
  # Al sustituir una instancia, la que se retira puede estar a mitad de un
  # transcodificado: A7 midio 1 711 s en el perfil largo. `max_unavailable_fixed
  # = 0` levanta la nueva antes de retirar la vieja, que es lo mismo que hace la
  # API, pero **no espera a que la vieja termine su tarea**.
  #
  # Que eso no sea perdida de datos lo garantiza la idempotencia: asynq reencola
  # y el worker repite sin duplicar derivados. Pero es trabajo tirado, y el
  # drenaje ordenado --apagar el consumo de tareas nuevas antes de parar-- es
  # **C7**. Hasta que C7 cierre, una rotacion tira como mucho una tarea en vuelo
  # por instancia.
  update_policy {
    type                           = "PROACTIVE"
    minimal_action                 = "REPLACE"
    most_disruptive_allowed_action = "REPLACE"
    max_surge_fixed                = 1
    max_unavailable_fixed          = 0
    replacement_method             = "SUBSTITUTE"
  }

  wait_for_instances = false

  lifecycle {
    ignore_changes = [target_size]
  }
}

output "worker_group_name" {
  description = "Nombre del grupo administrado de workers (C5)."
  value       = google_compute_instance_group_manager.worker.name
}

output "worker_health_check" {
  description = "Comprobacion de salud del worker: proceso vivo y conectado a la cola (C5)."
  value       = google_compute_health_check.worker.self_link
}

output "worker_concurrency" {
  description = <<-EOT
    Procesos de transcodificacion por instancia, iguales en todas las del grupo
    porque salen de la misma plantilla. Subirlo dentro de una VM de dos vCPU
    compra un 10 % y no el doble (A7): la palanca es mas instancias, que es C6.
  EOT
  value       = local.mooc_metadata_worker["mooc-worker-concurrency"]
}
