# Configuración de Red para la Infraestructura (Entrega 2 B3, Entrega 3 B2)
#
# Define la VPC privada, la subred para las VMs, el rango privado reservado para
# Cloud SQL (Private Services Access), el Cloud NAT para la salida a internet de
# instancias sin IP pública, y las reglas de firewall que aíslan los componentes.
#
# B2 (#201) sustituyó la regla interna única por reglas con propósito. La
# anterior, `mooc-allow-internal`, abría **todos** los puertos TCP y UDP entre
# cualquier par de instancias de la subred. Con dos VMs fijas era una comodidad;
# con grupos administrados que levantan y retiran instancias es una superficie
# que crece sola, y hacía decorativa cualquier regla más específica —incluida la
# que B1 escribió para el 6379 de la cola—.
#
# El modelo de las dos etiquetas de los grupos:
#
#   mooc-api-node     instancias del grupo de la API (C1). Entrada solo desde los
#                     rangos del balanceador y de las verificaciones de salud.
#   mooc-worker-node  instancias del grupo de workers (C5). **Sin ninguna entrada**
#                     salvo IAP: nadie les habla, ellas consumen de la cola.
#
# Ninguna de las dos lleva IP externa (eso lo fijan las plantillas de C1 y C5),
# así que un barrido desde internet no tiene a qué dirección llamar.

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

# A. La regla de entrada publica 80/443 se retiro en C3 (issue #206)
#
# Abria 80 y 443 desde internet hacia la etiqueta `web-server`, que solo llevaba
# la VM fija de la Entrega 2. Esa VM se retiro con C3, porque el criterio es que
# la API solo sea alcanzable por el balanceador.
#
# Los backends del grupo entran por `mooc-allow-lb-health`, acotada a los rangos
# del balanceador y de las comprobaciones de salud. **Ya no hay ninguna regla con
# origen 0.0.0.0/0 en esta VPC.**

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

# C. Entrada al grupo de la API: solo el balanceador y las verificaciones de salud
#
# Los dos rangos son los de Google y no son opcionales: un balanceador de
# aplicación externo **global** entrega el tráfico a los backends desde
# `130.211.0.0/22` y `35.191.0.0/16`, y desde esos mismos rangos llegan las
# comprobaciones de salud. Sin esta regla el grupo entero sale como no sano y el
# balanceador no manda nada, que es un fallo que se lee como «la aplicación no
# responde».
#
# **Esto presupone un balanceador global, no regional.** Un balanceador de
# aplicación *regional* se basa en Envoy y entrega el tráfico desde una subred
# de solo proxy (`REGIONAL_MANAGED_PROXY`) que habría que crear aquí, y entonces
# el origen sería el rango de esa subred y no estos dos. **C3 tiene que elegir
# global**, o volver a este fichero. Está anotado en la nota técnica 31.
#
# El puerto es el 80 y no el 443: el balanceador termina TLS (C3) y habla con los
# backends por HTTP dentro de la VPC. Las instancias del grupo no tienen IP
# externa, así que ese HTTP no viaja por internet en ningún tramo.
resource "google_compute_firewall" "allow_lb_and_health_checks" {
  name        = "mooc-allow-lb-health"
  network     = google_compute_network.vpc.name
  description = "Entrada al grupo de la API solo desde el balanceador y las verificaciones de salud de Google (B2)"

  allow {
    protocol = "tcp"
    ports    = ["80"]
  }

  source_ranges = ["130.211.0.0/22", "35.191.0.0/16"]
  target_tags   = ["mooc-api-node"]
}

# D. Diagnóstico dentro de la VPC
#
# Lo único que queda de la regla interna abierta. ICMP sirve para comprobar
# alcanzabilidad sin abrir ningún puerto de servicio, y es lo que hace que un
# problema de red se distinga de un problema de aplicación.
resource "google_compute_firewall" "allow_internal_icmp" {
  name        = "mooc-allow-internal-icmp"
  network     = google_compute_network.vpc.name
  description = "ICMP dentro de la subred para diagnostico. No abre ningun puerto de servicio (B2)"

  allow {
    protocol = "icmp"
  }

  source_ranges = [google_compute_subnetwork.subnet.ip_cidr_range]
}

# ------------------------------------------------------------------------------
# Lo que NO hay aquí, y es deliberado
# ------------------------------------------------------------------------------
#
#   * **Ninguna entrada hacia los workers.** No hay regla con destino
#     `mooc-worker-node`. Nadie les habla: ellos sacan trabajos de la cola y
#     escriben en la base y en el bucket. La salida no necesita regla porque la
#     salida por omisión está permitida.
#
#   * **Ninguna regla hacia Cloud SQL.** Su IP privada vive en el rango de
#     Private Services Access, al otro lado del peering, y el cortafuegos de
#     esta VPC no gobierna su entrada. Lo que la aísla es no tener IP pública y
#     que el peering solo exista con esta VPC.
#
#   * **Ninguna regla para el 9090 de métricas.** El agente de operaciones raspa
#     `localhost:9090` en cada instancia (`infra/ops-agent/*.yaml`), así que ese
#     puerto nunca cruza la red. Abrirlo «para monitorizar» sería superficie sin
#     uso.
#
#   * **Ninguna regla de deny explícita.** La entrada por omisión en una VPC ya
#     es denegar; una regla de deny solo añadiría algo si hubiera un allow más
#     amplio al que ganarle, y después de B2 no lo hay.
