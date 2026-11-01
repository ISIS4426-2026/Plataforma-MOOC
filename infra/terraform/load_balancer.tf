# ==============================================================================
# Balanceador de aplicacion externo HTTPS (C3 - issue #206)
# ==============================================================================
# El unico camino publico hacia la API. Termina TLS con un certificado
# administrado por Google y habla HTTP con los backends dentro de la VPC, porque
# las instancias del grupo no pueden terminar TLS: en un disco nuevo no hay
# certificado y, sin IP publica, certbot no podria obtener uno (nota tecnica 34).
#
# POR QUE GLOBAL Y NO REGIONAL
#
# B2 abrio la entrada de los backends a `130.211.0.0/22` y `35.191.0.0/16`, que
# son los rangos desde los que entrega un balanceador de aplicacion **global**.
# Un balanceador **regional** se basa en Envoy y entrega desde una subred de solo
# proxy (`REGIONAL_MANAGED_PROXY`) que habria que crear, y entonces el origen
# seria el rango de esa subred. Esta anotado en la nota 31: C3 elige global o
# vuelve a `network.tf`. Elige global.
# ==============================================================================

# ------------------------------------------------------------------------------
# La direccion, que ademas es el nombre
# ------------------------------------------------------------------------------
# `sslip.io` resuelve `<ip>.sslip.io` a la IP incrustada en el propio nombre, asi
# que reservar la direccion **es** elegir el dominio. Por eso va primero: el
# certificado se pide para un nombre que se deriva de ella.
resource "google_compute_global_address" "lb" {
  name         = "mooc-lb-ip"
  description  = "IP estatica global del balanceador. El dominio se deriva de ella via sslip.io (C3)"
  address_type = "EXTERNAL"
  ip_version   = "IPV4"
  labels       = var.labels
}

locals {
  # El nombre de acceso. Si se declara `var.app_domain`, manda ese: es la via
  # para pasar a un dominio propio sin tocar este fichero.
  lb_domain = var.app_domain != "" ? var.app_domain : "${google_compute_global_address.lb.address}.sslip.io"
}

# ------------------------------------------------------------------------------
# Certificado administrado por Google
# ------------------------------------------------------------------------------
# SE ELIGE ESTE Y NO UNO PROPIO, y el issue pide registrar el riesgo.
#
# Lo comprobado antes de decidir, porque las tres cosas podian romperlo:
#
#   * `34.24.52.111.sslip.io` devuelve **A = 34.24.52.111**: el nombre resuelve a
#     la IP que lleva dentro, que es lo que Google valida.
#   * **No hay registro AAAA.** Un AAAA que apunte a otro sitio es la causa mas
#     frecuente de que el certificado se quede en `FAILED_NOT_VISIBLE`.
#   * **No hay registros CAA** ni en `sslip.io` ni en el subdominio, asi que
#     Google Trust Services puede emitir. Un CAA que solo autorizara a Let's
#     Encrypt lo habria impedido en silencio.
#
# LOS RIESGOS QUE SE ACEPTAN
#
#   1. **Aprovisionar tarda**, hasta una hora, y hasta que el certificado esta
#      `ACTIVE` el 443 no sirve. El `apply` termina antes que el certificado: no
#      es un fallo, es espera. Se comprueba con
#      `gcloud compute ssl-certificates describe mooc-lb-cert`.
#   2. **`sslip.io` es un servicio de terceros.** Si deja de resolver, el nombre
#      muere y el certificado no puede **renovarse** --la renovacion revalida el
#      DNS--, asi que una caida suya coincidiendo con la renovacion tumbaria
#      HTTPS. Para una entrega de curso es aceptable; para produccion no, y la
#      salida es un dominio propio en `var.app_domain`.
#   3. **La IP no puede cambiar.** El dominio lleva la IP dentro, asi que
#      liberar esta direccion invalida el certificado. Es estatica y reservada
#      por eso.
#
# La alternativa era subir el certificado de Let's Encrypt que ya tenia la VM
# fija. Se descarta: esta emitido para la IP de esa VM y no para la del
# balanceador, caduca cada 90 dias, y renovarlo exigiria mantener certbot con
# alcance HTTP-01 desde internet, que es justo lo que este balanceador elimina.
resource "google_compute_managed_ssl_certificate" "lb" {
  name        = "mooc-lb-cert"
  description = "Certificado administrado por Google para el balanceador (C3)"

  managed {
    domains = [local.lb_domain]
  }

  # El nombre del certificado contiene el dominio, que contiene la IP. Si la IP
  # cambiara habria que crear otro antes de destruir este, o el 443 se queda sin
  # certificado durante el cambio.
  lifecycle {
    create_before_destroy = true
  }
}

# ------------------------------------------------------------------------------
# Backend service
# ------------------------------------------------------------------------------
resource "google_compute_backend_service" "api" {
  name        = "mooc-api-backend"
  description = "Backend de la API: el grupo administrado de C1 por su named port http (C3)"

  protocol = "HTTP"

  # El named port que declara el grupo de C1, no un numero. Si el puerto del
  # backend cambiara, se cambia en un sitio.
  port_name = "http"

  load_balancing_scheme = "EXTERNAL_MANAGED"

  # La misma comprobacion de salud que el autohealing de C1: `/api/v1/ready` de
  # A2. Que el balanceador decidiera con una sonda distinta de la que decide el
  # autohealing seria tener dos opiniones sobre si una replica sirve.
  health_checks = [google_compute_health_check.api.self_link]

  # `proxy_read_timeout 60s` era lo que ponia nginx en la VM fija. Se conserva
  # para no cambiar el comportamiento al mover el proxy de sitio.
  timeout_sec = 60

  # DRENAJE DE CONEXIONES, que es el criterio «el manejo de las peticiones en
  # curso al retirar una instancia esta documentado».
  #
  # Cuando una instancia sale del grupo --el autoescalador reduce, el autohealer
  # la recrea, una rotacion de plantilla la sustituye-- el balanceador deja de
  # mandarle peticiones nuevas y le da este plazo para terminar las que ya tiene
  # en vuelo. Las peticiones de esta API son de milisegundos --las etapas 1 y 3
  # del escenario 2 midieron 300 y 908 ms de peor caso incluyendo el ida y vuelta
  # del generador-- asi que 60 s es holgura amplia, no un calculo ajustado.
  #
  # Sin esto el valor por omision es 300 s, que retrasaria cada reduccion cinco
  # minutos sin que nadie lo pidiera.
  connection_draining_timeout_sec = 60

  backend {
    group = google_compute_instance_group_manager.api.instance_group

    # `RATE` Y NO `UTILIZATION`, y no es una preferencia.
    #
    # El autoescalador de C2 usa `load_balancing_utilization`, que es una
    # fraccion de la capacidad que declara el backend. Con `UTILIZATION` esa
    # capacidad se mide en CPU, y C2 demostro con los datos de A7 que la CPU de
    # esta API no se mueve con la carga: a 103,5 peticiones/s seguia en el 22 %.
    # El grupo se quedaria en una instancia mientras la base se funde.
    #
    # Con `RATE`, la capacidad es peticiones por segundo. Ver la nota tecnica 37.
    balancing_mode = "RATE"

    # El denominador de C2. 103,5 peticiones/s medidas en A7 sobre **una** VM del
    # mismo tipo, con p95 de 122 ms y sin saturarse, redondeado a la baja.
    #
    # Con el `target = 0.8` del autoescalador, se anade instancia al pasar de
    # **80 peticiones/s por instancia**.
    max_rate_per_instance = 100

    capacity_scaler = 1.0
  }

  log_config {
    enable      = true
    sample_rate = 1.0
  }
}

# ------------------------------------------------------------------------------
# Reglas de reenvio: 443 sirve, 80 redirige
# ------------------------------------------------------------------------------
resource "google_compute_url_map" "api" {
  name            = "mooc-api-urlmap"
  description     = "Todo el trafico HTTPS al backend de la API (C3)"
  default_service = google_compute_backend_service.api.self_link
}

# El 80 no sirve nada: redirige. Es lo que hacia `nginx.conf` con
# `return 301 https://$host$request_uri`, movido al balanceador, donde no cuesta
# una instancia atenderlo.
#
# `strip_query = false` conserva la cadena de consulta, que un 301 que la pierda
# rompe cualquier enlace con parametros.
resource "google_compute_url_map" "redirect_to_https" {
  name        = "mooc-api-urlmap-redirect"
  description = "Redirige el 80 a HTTPS (C3)"

  default_url_redirect {
    https_redirect         = true
    redirect_response_code = "MOVED_PERMANENTLY_DEFAULT"
    strip_query            = false
  }
}

# F1 (#219): version minima de TLS. Sin una politica propia el balanceador usa la de Google por omision, que acepta
# TLS 1.0 y 1.1: el primer barrido real de F1 lo comprobo con un cliente limitado a 1.1 (negocio `TLSv1.1`). La
# Entrega 2 lo rechazaba con Nginx y ese control se habia perdido al pasar al balanceador. TLS 1.2 como minimo y el
# perfil MODERN, que admite navegadores y clientes actuales; RESTRICTED seria mas estricto, pero deja fuera clientes
# legitimos sin que F1 lo pida.
resource "google_compute_ssl_policy" "api" {
  name            = "mooc-api-tls"
  profile         = "MODERN"
  min_tls_version = "TLS_1_2"
}

resource "google_compute_target_https_proxy" "api" {
  name             = "mooc-api-https-proxy"
  url_map          = google_compute_url_map.api.self_link
  ssl_certificates = [google_compute_managed_ssl_certificate.lb.self_link]
  ssl_policy       = google_compute_ssl_policy.api.self_link
}

resource "google_compute_target_http_proxy" "redirect" {
  name    = "mooc-api-http-proxy"
  url_map = google_compute_url_map.redirect_to_https.self_link
}

# ------------------------------------------------------------------------------
# El interruptor del balanceador: solo las reglas de reenvio
# ------------------------------------------------------------------------------
# Las reglas de reenvio se facturan **por hora, sirvan o no trafico**: 0,025
# USD/h cada una para las cinco primeras, y hay dos (80 y 443). Son 1,20 USD/dia
# con el entorno apagado, veinte veces lo que cuesta la base detenida.
#
# Pero destruir el balanceador entero entre sesiones es peor: el certificado
# administrado tarda **hasta una hora** en aprovisionarse, asi que cada sesion de
# medicion empezaria esperando.
#
# El compromiso: se conservan la direccion, el certificado, el backend, los mapas
# y los proxies --que **no tienen coste por hora**-- y se apagan solo las dos
# reglas de reenvio. El certificado sigue ACTIVE entre sesiones, el dominio no
# cambia, y aparcado solo queda el coste de la direccion reservada sin usar.
#
# Igual que `api_autoscaling_enabled`, el valor por omision es el seguro.
variable "lb_enabled" {
  description = "true publica el balanceador creando sus reglas de reenvio. Por omision false: no hay entrada publica."
  type        = bool
  default     = false
}

resource "google_compute_global_forwarding_rule" "https" {
  count = var.lb_enabled ? 1 : 0

  name                  = "mooc-api-https"
  target                = google_compute_target_https_proxy.api.self_link
  ip_address            = google_compute_global_address.lb.address
  port_range            = "443"
  load_balancing_scheme = "EXTERNAL_MANAGED"
  labels                = var.labels
}

resource "google_compute_global_forwarding_rule" "http_redirect" {
  count = var.lb_enabled ? 1 : 0

  name                  = "mooc-api-http"
  target                = google_compute_target_http_proxy.redirect.self_link
  ip_address            = google_compute_global_address.lb.address
  port_range            = "80"
  load_balancing_scheme = "EXTERNAL_MANAGED"
  labels                = var.labels
}

# ------------------------------------------------------------------------------
# Salidas
# ------------------------------------------------------------------------------

output "lb_url" {
  description = "La URL publica de la plataforma. Es la unica entrada (C3). Vacia si lb_enabled es false."
  value       = var.lb_enabled ? "https://${local.lb_domain}" : ""
}

output "lb_domain" {
  description = "El dominio del balanceador, exista o no la regla de reenvio. Es el APP_DOMAIN de las instancias."
  value       = local.lb_domain
}

output "lb_ip" {
  description = "IP estatica global del balanceador. El dominio se deriva de ella."
  value       = google_compute_global_address.lb.address
}

output "lb_certificate_name" {
  description = <<-EOT
    Certificado administrado. Aprovisionar tarda hasta una hora y el 443 no
    sirve hasta que esta ACTIVE; se comprueba con

      gcloud compute ssl-certificates describe mooc-lb-cert \
        --global --format="value(managed.status,managed.domainStatus)"
  EOT
  value       = google_compute_managed_ssl_certificate.lb.name
}
