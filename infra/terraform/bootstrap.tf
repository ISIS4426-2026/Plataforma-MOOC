# ==============================================================================
# Arranque de las instancias (B3 - issue #202)
# ==============================================================================
# Lo que una instancia nueva necesita para quedar lista sin que nadie entre por
# SSH, listo para que las plantillas de C1 y C5 lo consuman.
#
# El guion vive en `infra/bootstrap/mooc-bootstrap.sh` y **no se parametriza con
# `templatefile`**: lo lee todo del servidor de metadatos en tiempo de ejecucion.
# Asi sus bytes son identicos para los dos roles y para todas las instancias, y
# lo unico que distingue una de otra es su metadata. Es lo que convierte el
# criterio «todas las instancias de un grupo y una corrida ejecutan la misma
# version de codigo y configuracion» en una propiedad estructural en vez de algo
# que haya que vigilar.
#
# POR QUE ESTO NO TOCA LAS DOS VMs FIJAS
#
# `metadata_startup_script` es **ForceNew** en el proveedor, en los dos
# sentidos: cambiar su contenido recrea la VM, y quitarlo para mover el guion a
# `metadata["startup-script"]` tambien. No hay forma de migrar las dos VMs
# heredadas de la Entrega 2 al arranque nuevo sin destruirlas y volverlas a
# crear.
#
# Para una plantilla de grupo eso es irrelevante --una plantilla es inmutable y
# cambiarla es justamente crear otra-- asi que el mecanismo entra limpio en C1 y
# C5. Pero migrar las VMs fijas seria una destruccion de infraestructura
# existente que B3 no necesita: su comprobacion bloqueante es una captura de **la
# consola del grupo** creando una instancia, que exige C1 y C5 de todos modos.
#
# Las dos VMs fijas se van cuando los grupos las sustituyan.

locals {
  # Metadata comun a los dos roles. C1 y C5 la combinan con la de su rol:
  #
  #   metadata = merge(local.mooc_metadata_comun, {
  #     mooc-role   = "web"       # o "worker"
  #     ...
  #   })
  #
  # La IP de la cola sale de la direccion interna **estatica** de B1 y no de la
  # VM: por eso se reservo. Si la asignara el DHCP, recrear la cola obligaria a
  # rotar los dos grupos.
  mooc_metadata_comun = {
    enable-osconfig = "TRUE"

    # Dentro del mapa, no en `metadata_startup_script`: ver la cabecera.
    startup-script = file("${path.module}/../bootstrap/mooc-bootstrap.sh")

    mooc-image-tag  = var.image_tag
    mooc-project-id = var.project_id
    mooc-region     = var.region
    mooc-db-ip      = google_sql_database_instance.main.private_ip_address
    mooc-queue-ip   = google_compute_address.queue_internal_ip.address
  }

  # Lo que anade cada rol. Se expone resuelto para que C1 y C5 no vuelvan a
  # decidir el dominio ni el nombre del bucket.
  mooc_metadata_web = {
    mooc-role          = "web"
    mooc-app-domain    = local.mooc_app_domain
    mooc-smtp-from     = var.smtp_from
    mooc-smtp-username = var.smtp_username
  }

  mooc_metadata_worker = {
    mooc-role       = "worker"
    mooc-hls-bucket = google_storage_bucket.hls.name

    # Medido en A7: subir la concurrencia dentro de una VM de dos vCPU compra un
    # 10 %, no el doble, porque ffmpeg ya reparte el trabajo entre los nucleos.
    # La palanca es mas instancias, que es lo que hace C6.
    mooc-worker-concurrency = "2"
  }

  # El dominio por omision es el que la Entrega 2 escribia a mano en web.conf.
  # Calcularlo de la direccion reservada quita un valor que se podia quedar
  # desfasado al recrearla. Cuando C3 ponga el balanceador delante, el dominio
  # sera el suyo y hay que declararlo en `var.app_domain`.
  mooc_app_domain = var.app_domain != "" ? var.app_domain : "${google_compute_address.web_server_ip.address}.sslip.io"
}

output "mooc_bootstrap_image_tag" {
  description = "Commit cuya imagen y cuyos ficheros de despliegue corren las instancias (B3)."
  value       = var.image_tag
}

output "mooc_bootstrap_app_domain" {
  description = "Dominio efectivo que recibe la instancia web por metadata (B3)."
  value       = local.mooc_app_domain
}
