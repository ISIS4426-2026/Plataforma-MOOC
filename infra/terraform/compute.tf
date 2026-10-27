# ==============================================================================
# La VM web fija se retiro en C3 (issue #206)
# ==============================================================================
# Servia la API en su propia IP publica con nginx terminando TLS. El criterio de
# aceptacion de C3 es que **la API solo sea alcanzable publicamente a traves del
# balanceador y por HTTPS**, y esa VM era un segundo camino publico hacia la
# misma aplicacion.
#
# Lo que la sustituye: el grupo administrado de C1 detras del balanceador de C3.
# Con ella se fueron su direccion estatica, la regla de cortafuegos
# `mooc-allow-web-ingress`, `nginx.conf`, `nginx.bootstrap.conf` y el directorio
# `certbot/`, que solo existian para terminar TLS en la instancia.
#
# La VM del worker sigue aqui: no tiene superficie publica desde B2 y la
# sustituye el grupo de C5.
# ==============================================================================

# ==============================================================================
# Infraestructura de Cómputo - Worker Server (Issue #124 / E1)
# ==============================================================================
# Define la máquina virtual dedicada 'mooc-worker-server' según el perfil B1:
# - Tipo de máquina: e2-highcpu-2 (2 vCPU dedicadas, 2 GiB de RAM)
# - Zona: us-east1-b (misma zona de Cloud SQL y Web Server para latencia mínima)
# - Disco de arranque: 30 GiB balanced (pd-balanced) con Debian 12
# - **Sin IP pública** (B2, #201): la tenía estática y cerrada por cortafuegos,
#   es decir, superficie sin función. Esta máquina no recibe tráfico de nadie
#   --saca trabajos de la cola-- y su salida va por el Cloud NAT, que ya existía.
#   La administración entra por IAP, que no necesita IP externa.
# - Reglas de firewall: etiquetas 'worker-server' y 'allow-iap-ssh' (sin 'web-server')
# - Cuenta de servicio adjunta: sa-worker-server (lectura de originales, escritura hls/*)
# ==============================================================================

resource "google_compute_instance" "worker_server" {
  name         = "mooc-worker-server"
  machine_type = "e2-highcpu-2"
  zone         = var.zone
  description  = "Instancia Worker Server: Procesamiento multimedia FFmpeg y cola Asynq/Redis"

  tags = ["worker-server", "allow-iap-ssh"]

  labels = var.labels

  boot_disk {
    initialize_params {
      image  = "debian-cloud/debian-12"
      size   = 30
      type   = "pd-balanced"
      labels = var.labels
    }
  }

  # Sin `access_config`: ninguna IP externa. El criterio de B2 «la base de datos,
  # la cola y los workers no tienen superficie publica» se cumple por estructura
  # y no por cortafuegos: sin direccion externa no hay a que llamar desde
  # internet.
  network_interface {
    subnetwork = google_compute_subnetwork.subnet.id
  }

  service_account {
    email  = google_service_account.worker_server.email
    scopes = ["https://www.googleapis.com/auth/cloud-platform"]
  }

  # El agente de OS Config, que las imagenes de Debian activan solas. Declararlo
  # mantiene el plan limpio: sin esta linea Terraform propone quitarlo en cada
  # `apply`, y la siguiente recreacion de la VM lo vuelve a poner.
  metadata = {
    enable-osconfig = "TRUE"
  }

  metadata_startup_script = <<-EOT
    #!/usr/bin/env bash
    set -euo pipefail

    # Esperar liberación de bloqueos de dpkg si cloud-init está corriendo
    while fuser /var/lib/dpkg/lock-frontend >/dev/null 2>&1; do
      sleep 2
    done

    # Actualizar e instalar dependencias básicas
    apt-get update -qq
    apt-get install -y -qq ca-certificates curl gnupg lsb-release postgresql-client jq git ffmpeg

    # Instalar Docker Engine oficial y plugin de Compose
    install -m 0755 -d /etc/apt/keyrings
    if [[ ! -f /etc/apt/keyrings/docker.gpg ]]; then
      curl -fsSL https://download.docker.com/linux/debian/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
      chmod a+r /etc/apt/keyrings/docker.gpg
    fi

    echo \
      "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/debian \
      $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null

    apt-get update -qq
    apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

    systemctl enable --now docker
  EOT

  depends_on = [
    google_compute_subnetwork.subnet,
    google_compute_router_nat.nat,
    google_service_account.worker_server
  ]
}

