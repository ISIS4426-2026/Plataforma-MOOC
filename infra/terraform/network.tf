# Configuración de Red para la Infraestructura (Issue B3)
#
# Define la VPC privada, la subred para las VMs, el rango privado reservado para
# Cloud SQL (Private Services Access), el Cloud NAT para la salida a internet de
# instancias sin IP pública, y las reglas de firewall que aíslan los componentes.

# 1. VPC Principal Privada
resource "google_compute_network" "vpc" {
  name                    = "mooc-vpc"
  auto_create_subnetworks = false
  description             = "VPC principal de la Plataforma MOOC"
}

# 2. Subred Única para VMs (Web Server y Worker Server)
resource "google_compute_subnetwork" "subnet" {
  name                     = "mooc-subnet"
  ip_cidr_range            = "10.0.1.0/24"
  region                   = var.region
  network                  = google_compute_network.vpc.id
  private_ip_google_access = true
  description              = "Subred para Web Server y Worker Server"
}

# 3. Reservar rango privado para servicios administrados (Cloud SQL - C1)
resource "google_compute_global_address" "private_ip_alloc" {
  name          = "mooc-private-ip-alloc"
  purpose       = "VPC_PEERING"
  address_type  = "INTERNAL"
  prefix_length = 20
  network       = google_compute_network.vpc.id
  description   = "Rango de IP privadas reservado para Cloud SQL (Private Services Access)"
}

# 4. Conexión de Peering de Red de Servicios (Service Networking Connection)
resource "google_service_networking_connection" "private_vpc_connection" {
  network                 = google_compute_network.vpc.id
  service                 = "servicenetworking.googleapis.com"
  reserved_peering_ranges = [google_compute_global_address.private_ip_alloc.name]
}

# 5. Cloud Router y Cloud NAT para Salida a Internet (Egress para dependencias/actualizaciones sin exposición pública)
resource "google_compute_router" "router" {
  name    = "mooc-router"
  region  = var.region
  network = google_compute_network.vpc.id
}

resource "google_compute_router_nat" "nat" {
  name                               = "mooc-nat"
  router                             = google_compute_router.router.name
  region                             = var.region
  nat_ip_allocate_option             = "AUTO_ONLY"
  source_subnetwork_ip_ranges_to_nat = "ALL_SUBNETWORKS_ALL_IP_RANGES"

  log_config {
    enable = true
    filter = "ERRORS_ONLY"
  }
}

# 6. Reglas de Firewall

# A. HTTP y HTTPS públicos únicamente hacia instancias con tag 'web-server'
resource "google_compute_firewall" "allow_web_ingress" {
  name        = "mooc-allow-web-ingress"
  network     = google_compute_network.vpc.name
  description = "Permite trafico HTTP (80) y HTTPS (443) desde internet únicamente hacia el Web Server"

  allow {
    protocol = "tcp"
    ports    = ["80", "443"]
  }

  source_ranges = ["0.0.0.0/0"]
  target_tags   = ["web-server"]
}

# B. Acceso de administración SSH mediante Google Cloud IAP (Identity-Aware Proxy)
resource "google_compute_firewall" "allow_ssh_iap" {
  name        = "mooc-allow-ssh-iap"
  network     = google_compute_network.vpc.name
  description = "Permite acceso SSH (22) proveniente de Google IAP (35.235.240.0/20) para administración segura"

  allow {
    protocol = "tcp"
    ports    = ["22"]
  }

  source_ranges = ["35.235.240.0/20"]
  target_tags   = ["allow-iap-ssh"]
}

# C. Tráfico Interno VPC entre Web Server y Worker Server (cola Asynq/Redis 6379, etc.)
resource "google_compute_firewall" "allow_internal" {
  name        = "mooc-allow-internal"
  network     = google_compute_network.vpc.name
  description = "Permite comunicación interna completa dentro de la subred de la VPC"

  allow {
    protocol = "icmp"
  }

  allow {
    protocol = "tcp"
    ports    = ["0-65535"]
  }

  allow {
    protocol = "udp"
    ports    = ["0-65535"]
  }

  source_ranges = [google_compute_subnetwork.subnet.ip_cidr_range]
}
