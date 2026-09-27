# ==============================================================================
# Infraestructura de Cómputo - Web Server (Issue #123 / D2)
# ==============================================================================
# Define la máquina virtual pública 'mooc-web-server' según el perfil B1:
# - Tipo de máquina: e2-highcpu-2 (2 vCPU dedicadas, 2 GiB de RAM)
# - Zona: us-east1-b (misma zona de Cloud SQL para cero costo de egreso interno)
# - Disco de arranque: 30 GiB balanced (pd-balanced) con Debian 12
# - IP pública estática IPv4
# - Reglas de firewall: etiquetas 'web-server' (HTTP/HTTPS público) y 'allow-iap-ssh'
# - Cuenta de servicio adjunta: sa-web-server (permiso de firma y lectura de db-password)
# ==============================================================================

resource "google_compute_address" "web_server_ip" {
  name        = "mooc-web-server-ip"
  region      = var.region
  description = "Direccion IPv4 estatica externa para el Web Server (D2)"
  labels      = var.labels
}

resource "google_compute_instance" "web_server" {
  name         = "mooc-web-server"
  machine_type = "e2-highcpu-2"
  zone         = var.zone
  description  = "Instancia Web Server: API modular en Go y proxy inverso Nginx"

  tags = ["web-server", "allow-iap-ssh"]

  labels = var.labels

  boot_disk {
    initialize_params {
      image  = "debian-cloud/debian-12"
      size   = 30
      type   = "pd-balanced"
      labels = var.labels
    }
  }

  network_interface {
    subnetwork = google_compute_subnetwork.subnet.id
    access_config {
      nat_ip = google_compute_address.web_server_ip.address
    }
  }

  service_account {
    email  = google_service_account.web_server.email
    scopes = ["https://www.googleapis.com/auth/cloud-platform"]
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
    apt-get install -y -qq ca-certificates curl gnupg lsb-release postgresql-client jq git

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
    google_service_account.web_server
  ]
}
