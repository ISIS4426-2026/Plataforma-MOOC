# ==============================================================================
# Cola compartida fuera de los grupos elásticos (B1 - issue #200)
# ==============================================================================
# La cola de asynq vivía dentro de `docker-compose.worker.yml`, en la misma VM
# que los workers. En la Entrega 2 era aceptable porque esa máquina era fija; en
# la Entrega 3 pertenece a un grupo administrado que el autoescalado puede
# destruir, y con ella se irían los trabajos encolados. El enunciado lo prohíbe
# explícitamente y la nota técnica 4 lo registra.
#
# POR QUÉ UNA VM Y NO MEMORYSTORE
#
# Se compararon las tres opciones con precios de lista de `us-east1` y 730 h/mes:
#
#   | Opción                  | Persistencia          | Pérdida al reiniciar | USD/mes |
#   | Memorystore Basic M1    | solo RDB, mínimo 1 h  | hasta 1 hora de cola |  35,77  |
#   | Memorystore Standard M1 | RDB + réplica         | reinicio completo    |  46,72  |
#   | VM con Redis (esta)     | AOF appendfsync 1 s   | <= 1 segundo         |  ~13    |
#
# Lo que decide no es el coste, es el criterio de aceptación «los trabajos
# encolados sobreviven al reinicio del servicio de cola»: **Memorystore no
# soporta AOF**. Su única persistencia son instantáneas RDB con intervalo mínimo
# de una hora, y el nivel Basic se recupera precisamente de la última
# instantánea. Una cola que pierde una hora de trabajos en un reinicio no cumple
# ese criterio, y pagar 2,7 veces más por incumplirlo no tiene defensa.
#
# Lo que se pierde: la alta disponibilidad gestionada. Esta VM es un **punto
# único de fallo** para los dos grupos elásticos. Se mitiga con reinicio
# automático y migración en mantenimiento, y el dato sobrevive por AOF, pero no
# se disimula: está declarado en la evidencia de B1 y en la nota técnica 30.
# ==============================================================================

# ------------------------------------------------------------------------------
# Dirección interna estática
# ------------------------------------------------------------------------------
# Sin esto, la IP privada la asigna el DHCP de la subred y **cambia si la VM se
# recrea**. `REDIS_URL` se escribe en la configuración de cada instancia que
# arranca (B3) y en las plantillas de los grupos (B4, C5), así que una dirección
# que cambia obliga a rotar los dos grupos cada vez que se toca la cola.
#
# Es la misma lección de la IP privada de Cloud SQL en A7: al recrear la base,
# las dos VMs quedaron apuntando a una dirección que ya no existía.
resource "google_compute_address" "queue_internal_ip" {
  name         = "mooc-queue-ip"
  region       = var.region
  subnetwork   = google_compute_subnetwork.subnet.id
  address_type = "INTERNAL"
  purpose      = "GCE_ENDPOINT"
  description  = "IP privada estable de la cola compartida (B1). La leen B3, B4 y C5."
  labels       = var.labels
}

# ------------------------------------------------------------------------------
# La instancia
# ------------------------------------------------------------------------------
resource "google_compute_instance" "queue_server" {
  name         = "mooc-queue-server"
  machine_type = "e2-small"
  zone         = var.zone
  description  = "Cola compartida asynq/Redis, fuera de los dos grupos elasticos (B1)"

  # Sin la etiqueta `web-server`: esta máquina no publica 80 ni 443.
  tags = ["queue-server", "allow-iap-ssh"]

  labels = var.labels

  # `e2-small` es de núcleo compartido, que es exactamente lo que
  # docs/entrega2/CONFIGURACION_Y_COSTOS.md descartó para las VMs de aplicación.
  # Aquí sí sirve, y por el motivo contrario: Redis es de un solo hilo y la carga
  # es minúscula --el pico medido en A7 fueron 14 tareas en cola--, así que lo
  # que hace falta es memoria con holgura y no vCPU dedicada. El tamaño no lo
  # dicta el volumen de datos, que son kilobytes, sino el no bajar de la máquina
  # más pequeña que deja margen cómodo.
  boot_disk {
    initialize_params {
      image  = "debian-cloud/debian-12"
      size   = 10
      type   = "pd-balanced"
      labels = var.labels
    }
  }

  # Sin `access_config`: **la VM no tiene IP pública**. Es la mitad del criterio
  # «la cola no es alcanzable desde internet»; la otra mitad es el cortafuegos de
  # más abajo. La salida para `apt` va por el Cloud NAT que ya existe.
  network_interface {
    subnetwork = google_compute_subnetwork.subnet.id
    network_ip = google_compute_address.queue_internal_ip.address
  }

  service_account {
    email  = google_service_account.queue_server.email
    scopes = ["https://www.googleapis.com/auth/cloud-platform"]
  }

  # Reinicio automático y migración en vez de terminación durante el
  # mantenimiento del anfitrión. No convierte el punto único de fallo en alta
  # disponibilidad, pero quita de la ecuación las dos causas de caída que no son
  # un fallo real de la máquina.
  scheduling {
    automatic_restart   = true
    on_host_maintenance = "MIGRATE"
    preemptible         = false
  }

  allow_stopping_for_update = true

  metadata = {
    enable-osconfig = "TRUE"
  }

  # El guion vive en su propio fichero, no en un heredoc aqui: escribe un
  # redis.conf con heredoc, y un heredoc anidado dentro del de Terraform depende
  # de como `<<-EOT` recorta la sangria comun. Un espacio de mas al principio de
  # una directiva deja a redis-server sin arrancar, en una maquina sin IP publica
  # a la que hay que entrar por IAP para averiguarlo.
  metadata_startup_script = templatefile("${path.module}/queue_startup.sh", {
    queue_ip = google_compute_address.queue_internal_ip.address
  })

  depends_on = [
    google_compute_subnetwork.subnet,
    google_compute_router_nat.nat,
    google_service_account.queue_server,
  ]
}

# ------------------------------------------------------------------------------
# Cortafuegos
# ------------------------------------------------------------------------------
# Esta regla declara la intención: al 6379 de la cola solo se llega desde dentro
# de la subred.
#
# **Hoy no restringe nada todavía**, y conviene decirlo en vez de aparentar lo
# contrario: `mooc-allow-internal` abre todos los puertos entre todo lo que esté
# en la subred, así que esta regla es redundante mientras aquella exista.
# Empieza a servir cuando **F1** acote `mooc-allow-internal` a los puertos que
# cada componente necesita de verdad. Lo que sí garantiza ya, junto con la
# ausencia de IP pública, es que no hay camino desde internet.
resource "google_compute_firewall" "allow_queue_internal" {
  name        = "mooc-allow-queue"
  network     = google_compute_network.vpc.id
  description = "Acceso a la cola compartida (6379) solo desde la subred privada (B1)"

  allow {
    protocol = "tcp"
    ports    = ["6379"]
  }

  source_ranges = [google_compute_subnetwork.subnet.ip_cidr_range]
  target_tags   = ["queue-server"]
}
