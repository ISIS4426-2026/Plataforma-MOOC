# ==============================================================================
# Grupo administrado de la API (C1 - issue #204)
# ==============================================================================
# Plantilla de instancia, comprobacion de salud y grupo administrado de la API.
# El tamano lo decide el autoescalador de C2; aqui el grupo nace con una
# instancia y una politica de autohealing.
#
# POR QUE ZONAL Y NO REGIONAL
#
# Un grupo regional reparte las instancias entre zonas y sobrevive a la caida de
# una. **Aqui no sobreviviria**, y la razon esta en las dependencias:
#
#   * Cloud SQL es `availability_type = "ZONAL"` en `us-east1-b` (database.tf).
#     Si esa zona cae, la base se va con ella.
#   * La cola compartida es una VM, tambien en `us-east1-b` (B1).
#
# Un grupo regional de API sobre una base y una cola de una sola zona compra
# redundancia que no se puede ejercer: las replicas de las otras zonas
# arrancarian y fallarian su comprobacion de salud, porque `/api/v1/ready` prueba
# la base. Lo unico que si cambiaria es el coste: el trafico entre zonas se
# factura, del orden de 0,01 USD/GiB, y cada peticion cruzaria de zona para
# llegar a la base.
#
# **La alta disponibilidad de verdad empieza por Cloud SQL**, no por el grupo de
# la API: `availability_type = "REGIONAL"` casi dobla el coste de la instancia de
# base. Mientras esa decision no se tome, un grupo regional es teatro caro.
#
# Si en el futuro se pasa la base a regional, este grupo se convierte cambiando
# `google_compute_instance_group_manager` por
# `google_compute_region_instance_group_manager` y anadiendo `distribution_policy`.
# Queda anotado en la nota tecnica 34.
# ==============================================================================

# ------------------------------------------------------------------------------
# Plantilla de instancia
# ------------------------------------------------------------------------------
resource "google_compute_instance_template" "api" {
  # `name_prefix` y no `name`: una plantilla es inmutable, asi que cualquier
  # cambio crea otra. Con un nombre fijo el `apply` fallaria por nombre duplicado
  # antes de poder sustituirla.
  name_prefix = "mooc-api-"
  description = "Plantilla de las instancias del grupo de la API (C1)"
  region      = var.region

  # El mismo tipo que la VM fija de la Entrega 2, a proposito: G3 compara la
  # arquitectura elastica contra la linea base de A7, y si tambien cambiara el
  # tipo de maquina la diferencia dejaria de ser solo arquitectonica.
  machine_type = "e2-highcpu-2"

  labels = var.labels

  # `mooc-api-node` es la etiqueta que B2 declaro para la entrada del
  # balanceador y de las comprobaciones de salud. Sin ella el grupo entero sale
  # como no sano y el balanceador no manda nada: un fallo que se lee como «la
  # aplicacion no responde».
  tags = ["mooc-api-node", "allow-iap-ssh"]

  disk {
    source_image = "debian-cloud/debian-12"
    disk_size_gb = 30
    disk_type    = "pd-balanced"
    auto_delete  = true
    boot         = true
    labels       = var.labels
  }

  # **Sin `access_config`**: ninguna IP externa. Es la mitad estructural del
  # criterio de B2 «un barrido desde internet no encuentra puertos de aplicacion
  # abiertos en las instancias de los grupos»: sin direccion no hay a que
  # llamar. La salida va por el Cloud NAT que ya existe.
  network_interface {
    subnetwork = google_compute_subnetwork.subnet.id
  }

  service_account {
    email  = google_service_account.web_server.email
    scopes = ["https://www.googleapis.com/auth/cloud-platform"]
  }

  # Toda la configuracion viaja por metadata y la lee `mooc-bootstrap.sh` en
  # ejecucion (B3). `mooc-nginx-config` es lo unico que distingue a una instancia
  # de grupo de la VM fija: sirve HTTP plano en el 80 en vez de redirigir a HTTPS
  # y terminar TLS con un certificado que en un disco nuevo no existe, y que sin
  # IP publica certbot no podria obtener.
  #
  # `nginx.web.conf` es ese mismo perfil (HTTP plano en el 80, las mismas cabeceras de
  # reenvio, /api/v1/metrics denegada) mas el frontend de D1: el proxy es la imagen
  # `web`, publicada con `scripts/publish_images.sh`, que sirve la interfaz y reenvia
  # /api/ a la API en el mismo origen. `nginx.mig.conf` se conserva por si el grupo
  # tuviera que servir solo la API.
  metadata = merge(local.mooc_metadata_comun, local.mooc_metadata_web, {
    mooc-nginx-config = "./nginx.web.conf"
  })

  scheduling {
    automatic_restart   = true
    on_host_maintenance = "MIGRATE"
    preemptible         = false
  }

  # Sin esto, sustituir la plantilla exige borrar antes el grupo que la usa.
  lifecycle {
    create_before_destroy = true
  }

  depends_on = [
    google_compute_router_nat.nat,
    google_compute_address.queue_internal_ip,
  ]
}

# ------------------------------------------------------------------------------
# Comprobacion de salud
# ------------------------------------------------------------------------------
# `/api/v1/ready` de A2, no `/api/v1/health`. La diferencia importa y A2 la
# explica: `health` dice que el proceso vive y aguanta la caida de la base;
# `ready` prueba la base **y** la bandera de arranque, y responde 503 mientras la
# instancia se esta montando. Lo que decide si una replica recibe trafico tiene
# que ser lo segundo.
#
# Es la misma sonda que mide B3, para que lo medido y lo decidido sean lo mismo.
resource "google_compute_health_check" "api" {
  name        = "mooc-api-ready"
  description = "Comprobacion de salud de la API sobre /api/v1/ready (C1, sonda de A2)"

  # Un fallo no recrea la instancia: hacen falta tres seguidos. Con `check_interval`
  # de 10 s, una instancia tarda 30 s en declararse no sana, que es tiempo de
  # sobra para distinguir un reinicio de contenedor de una maquina perdida.
  check_interval_sec  = 10
  timeout_sec         = 5
  healthy_threshold   = 2
  unhealthy_threshold = 3

  http_health_check {
    request_path = "/api/v1/ready"
    port         = 80
  }

  log_config {
    enable = true
  }
}

# ------------------------------------------------------------------------------
# El grupo
# ------------------------------------------------------------------------------
resource "google_compute_instance_group_manager" "api" {
  name               = "mooc-api-group"
  base_instance_name = "mooc-api"
  zone               = var.zone
  description        = "Grupo administrado de la API (C1). El tamano lo decide el autoescalador de C2."

  version {
    instance_template = google_compute_instance_template.api.self_link
  }

  # Una instancia. El rango 1-3 lo impone el autoescalador de **C2**, no este
  # recurso: un grupo con autoescalador ignora `target_size`, y declararlo en
  # tres aqui seria la «ampliacion manual» que el criterio de aceptacion
  # descarta explicitamente como evidencia.
  target_size = 1

  # El puerto con nombre que el balanceador de C3 pedira al grupo. El 80 porque
  # el balanceador termina TLS y habla HTTP con los backends dentro de la VPC.
  named_port {
    name = "http"
    port = 80
  }

  auto_healing_policies {
    health_check = google_compute_health_check.api.self_link

    # EL VALOR MAS DELICADO DE ESTE FICHERO.
    #
    # Si el retardo inicial queda por debajo del arranque real, el autohealer
    # mata instancias que todavia se estan montando y las sustituye por otras que
    # haran lo mismo: un bucle de recreacion que se lee como «la imagen esta
    # rota». Errar por arriba solo retrasa la sustitucion de una instancia de
    # verdad muerta, asi que el lado seguro es claro.
    #
    # CALIBRADO CON LA MEDIDA REAL del 2026-10-27, no a ojo.
    #
    # Se puso en 600 s a ciegas antes del primer despliegue. Medido:
    #
    #   62 s   lo que tarda `mooc-bootstrap.sh` de principio a `/api/v1/ready`
    #          (`/var/lib/mooc/arranque.json`, B3)
    #  160 s   del borrado a mano de una instancia a que el grupo la declare
    #          sana, incluyendo arrancar la VM y el sondeo del health check
    #
    # 300 s deja casi el doble de margen sobre los 160 s medidos y reduce a la
    # mitad lo que tarda en sustituirse una instancia de verdad muerta.
    #
    # Sigue siendo mejor errar por arriba: por debajo del arranque real, el
    # autohealer mata instancias que se estan montando y las sustituye por otras
    # que haran lo mismo. Si el arranque se alarga --mas migraciones, una imagen
    # mas grande-- hay que volver a medirlo, no suponerlo.
    initial_delay_sec = 300
  }

  # Rotar a una version nueva sin bajar del minimo, que es el criterio de
  # aceptacion. `max_unavailable_fixed = 0` es lo que lo garantiza: el grupo
  # levanta la instancia nueva **antes** de retirar la vieja, en vez de dejar un
  # hueco. Con `max_surge_fixed = 1` el pico es de una instancia por encima del
  # objetivo, que cabe de sobra en el techo de 3 de C2.
  update_policy {
    type                           = "PROACTIVE"
    minimal_action                 = "REPLACE"
    most_disruptive_allowed_action = "REPLACE"
    max_surge_fixed                = 1
    max_unavailable_fixed          = 0
    replacement_method             = "SUBSTITUTE"
  }

  # `false` para que el `apply` no se quede esperando a que la instancia quede
  # lista. El arranque completo son minutos, y bloquear el `apply` todo ese
  # tiempo esconde el fallo en un tiempo de espera de Terraform en vez de
  # mostrarlo en el registro serie de la instancia, que es donde esta.
  wait_for_instances = false

  lifecycle {
    # El tamano pasa a ser del autoescalador en C2. Ignorarlo aqui evita que un
    # `apply` posterior lo devuelva a 1 y deshaga una decision del autoescalador
    # a mitad de una corrida de carga.
    ignore_changes = [target_size]
  }
}

output "api_group_name" {
  description = "Nombre del grupo administrado de la API (C1)."
  value       = google_compute_instance_group_manager.api.name
}

output "api_group_instance_group" {
  description = "Grupo de instancias que C3 usara como backend del balanceador."
  value       = google_compute_instance_group_manager.api.instance_group
}

output "api_health_check" {
  description = "Comprobacion de salud de la API, compartida por el autohealing (C1) y el backend (C3, C4)."
  value       = google_compute_health_check.api.self_link
}
