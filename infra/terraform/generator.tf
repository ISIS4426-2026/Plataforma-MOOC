# Generador de carga EXTERNO (G1, issue #222).
#
# El enunciado exige ejecutar el generador fuera de las maquinas de aplicacion y comprobar que no limita
# la medicion. La linea base de A7 se midio desde un portatil, y A7 documento que su red no reproduce lo
# publicado: un generador que no esta en la misma region mide su propia conexion.
#
# Esta VM esta FUERA de los dos grupos elasticos y de la cola, en otra zona de la misma region. No tiene
# IP publica (sale por la NAT de B2) y solo se entra por IAP, asi que no abre ningun puerto y F1 no
# encuentra nada nuevo. Su cuenta de servicio (que tampoco existe mientras esta apagado) escribe registros y
# metricas, y para G3 y G4 abre el 6379 de la cola (inspeccionarla durante el escenario 2) y, si se pide con
# `generator_db_secret_access`, lee el secreto de la base; todo ATADO A `generator_enabled` y por lo tanto
# inexistente con la VM apagada. No toca los buckets y no puede firmar URLs.
#
# APAGADA POR OMISION, como el balanceador y los autoescaladores: `generator_enabled = false` no crea
# nada, y una VM de 4 vCPU encendida de mas cuesta dinero (~0,13 USD/h). Para una campana:
#
#   terraform apply -var generator_enabled=true  <los -var del entorno encendido>
#   gcloud compute ssh mooc-load-generator --zone us-east1-c --tunnel-through-iap
#   ... correr las corridas (scripts/h3_nube.sh, scripts/run_capacity_escenario2.sh) ...
#   terraform apply -var generator_enabled=false <los -var del entorno encendido>
#
# La VM trae Docker, Git y Python; la imagen de JMeter se fija por DIGEST, el mismo que uso la linea base
# (justb4/jmeter:latest resolvia a JMeter 5.5): "misma herramienta y version de la entrega anterior".

variable "generator_enabled" {
  description = "true crea la VM del generador de carga. Por omision false: no existe, y no cuesta."
  type        = bool
  default     = false
}

variable "generator_db_secret_access" {
  description = <<-EOT
    true deja que la cuenta de la VM lea el secreto db-password por si misma (requiere quien aplique pueda cambiar la
    politica de IAM de ese secreto). Por omision false: la contrasena se entrega a la VM desde la sesion de quien opera.
  EOT
  type        = bool
  default     = false
}

variable "generator_zone" {
  description = "Zona del generador: la misma region que el entorno y OTRA zona que las VMs de aplicacion."
  type        = string
  default     = "us-east1-c"
}

variable "generator_machine_type" {
  description = <<-EOT
    Tamano del generador. e2-standard-4 (4 vCPU, 16 GiB): el nivel de 200 usuarios del escenario 1 movio
    ~100 KB/s y poca CPU en la linea base, y el escenario 2 transfiere archivos de hasta ~1 GiB; con 4 vCPU
    el contenedor de JMeter no pasa del 70 % de sus nucleos (umbral de scripts/capacity_contraste_generador.py).
  EOT
  type        = string
  default     = "e2-standard-4"
}

variable "generator_jmeter_image" {
  description = "Imagen de JMeter por digest (JMeter 5.5), la que resolvia justb4/jmeter:latest en la linea base de A7."
  type        = string
  default     = "justb4/jmeter@sha256:088ac52b759a198a5afa5ae13d0a6306e9f2017d71ad140ff57427f6930406f7"
}

resource "google_service_account" "load_generator" {
  count = var.generator_enabled ? 1 : 0

  project      = var.project_id
  account_id   = "sa-load-generator"
  display_name = "Generador de carga externo"
  description  = "Identidad de la VM del generador (G1). Solo escribe registros y metricas; no toca datos de la plataforma."
}

resource "google_project_iam_member" "load_generator" {
  for_each = var.generator_enabled ? toset([
    "roles/logging.logWriter",
    "roles/monitoring.metricWriter",
  ]) : toset([])

  project = var.project_id
  role    = each.value
  member  = "serviceAccount:${google_service_account.load_generator[0].email}"
}

# G3 y G4: leer la contrasena de la base. Es lo que permite reiniciar las cuentas de carga y verificar el estado
# DESDE la VM, sin pasar la contrasena por la maquina de quien opera. Alcance: un solo secreto, solo lectura
# (el mismo patron que `db_password_readers`), y desaparece con la VM.
#
# OPCIONAL (`generator_db_secret_access`, apagado por omision): aplicarlo exige poder cambiar la politica de IAM de
# ESE secreto (`secretmanager.secrets.setIamPolicy`), que no tienen todas las cuentas del equipo que aplican
# Terraform. Sin el, la contrasena llega a la VM por el tunel de IAP desde la sesion de quien opera y se queda
# solo en un archivo en memoria (docs/entrega3/evidencias/G3/RUNBOOK.md, seccion 1).
resource "google_secret_manager_secret_iam_member" "load_generator_db_password" {
  count = var.generator_enabled && var.generator_db_secret_access ? 1 : 0

  project   = var.project_id
  secret_id = data.google_secret_manager_secret.db_password.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.load_generator[0].email}"
}

# G4: la campana del escenario 2 inspecciona la cola de asynq para medir espera, profundidad y drenaje por
# trabajo. El 6379 solo lo abren la API y el worker (`allow_queue_internal`); esta regla anade UNA cuenta mas, la
# del generador, y existe solo mientras exista la VM. Destino por cuenta de servicio, como la regla original.
resource "google_compute_firewall" "allow_queue_from_generator" {
  count = var.generator_enabled ? 1 : 0

  name        = "mooc-allow-queue-generator"
  network     = google_compute_network.vpc.id
  description = "6379 de la cola solo desde la cuenta del generador de carga (G4); existe mientras exista la VM"

  allow {
    protocol = "tcp"
    ports    = ["6379"]
  }

  source_service_accounts = [google_service_account.load_generator[0].email]
  target_service_accounts = [google_service_account.queue_server.email]
}

resource "google_compute_instance" "load_generator" {
  count = var.generator_enabled ? 1 : 0

  name         = "mooc-load-generator"
  machine_type = var.generator_machine_type
  zone         = var.generator_zone
  description  = "Generador de carga externo a los grupos (G1): JMeter fijado por digest"

  # `allow-iap-ssh` es el unico acceso; no lleva la etiqueta de la API, asi que las reglas del balanceador no la alcanzan.
  tags   = ["load-generator", "allow-iap-ssh"]
  labels = merge(var.labels, { rol = "generador" })

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
    # Sin access_config: no hay IP publica. La salida a internet (hacia el balanceador) es la NAT de B2.
  }

  service_account {
    email  = google_service_account.load_generator[0].email
    scopes = ["https://www.googleapis.com/auth/cloud-platform"]
  }

  scheduling {
    automatic_restart   = true
    on_host_maintenance = "MIGRATE"
    preemptible         = false
  }

  allow_stopping_for_update = true

  metadata = {
    enable-osconfig = "TRUE"

    startup-script = <<-EOT
      #!/usr/bin/env bash
      set -euo pipefail
      while fuser /var/lib/dpkg/lock-frontend >/dev/null 2>&1; do sleep 2; done
      export DEBIAN_FRONTEND=noninteractive
      apt-get -o DPkg::Lock::Timeout=300 update -qq
      apt-get -o DPkg::Lock::Timeout=300 install -y -qq docker.io git python3 jq curl
      systemctl enable --now docker
      docker pull ${var.generator_jmeter_image}
      docker run --rm --entrypoint sh ${var.generator_jmeter_image} -c 'jmeter --version 2>&1 | grep -m1 -E "^[0-9.]+$|[0-9]\.[0-9]" || true' > /var/log/mooc-generator-jmeter.txt 2>&1 || true
      echo "generador listo: $(nproc) nucleos, $(free -g | awk '/Mem:/{print $2}') GiB, $(date -u +%FT%TZ)" > /var/log/mooc-generator-listo.txt
    EOT
  }

  depends_on = [
    google_compute_subnetwork.subnet,
    google_compute_router_nat.nat,
    google_service_account.load_generator,
  ]
}

output "load_generator" {
  description = "Nombre y zona del generador cuando existe; vacio cuando esta apagado."
  value       = var.generator_enabled ? "mooc-load-generator (${var.generator_zone})" : ""
}
