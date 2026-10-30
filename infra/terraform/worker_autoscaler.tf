# ==============================================================================
# Politica de autoescalado de los workers (C6 - issue #209)
# ==============================================================================
# LA SENAL ES LA COLA, Y NO ES NEGOCIABLE
#
# El enunciado admite «profundidad o antiguedad de la cola, trabajos pendientes
# por consumidor u otra senal que represente la demanda de procesamiento». La CPU
# no esta en esa lista y aqui tampoco serviria, por la razon contraria a la de la
# API (C2): A7 midio el worker al **96,31 % de CPU media durante casi dos horas**.
#
# Un worker que transcodifica esta saturado de CPU mientras tenga **un** trabajo.
# La CPU no distingue «un trabajo en curso» de «cien esperando»: satura igual con
# los dos, asi que con un objetivo de CPU el grupo pediria el maximo de replicas
# por un solo video y las mantendria hasta que la cola se vaciara.
#
#   senal                                que mide            por que no
#   cpu_utilization                      saturacion de ffmpeg  satura con 1 trabajo
#   load_balancing_utilization           peticiones/instancia  nadie les habla
#   custom: outstanding (esta)           trabajo por atender   --
#
# `load_balancing_utilization`, que es la senal de C2, aqui no existe: a los
# workers no les llega ningun balanceador y el grupo no tiene `named_port`.
#
# QUE ES `outstanding` Y POR QUE NO `pending`
#
# `outstanding` = `pending` + `active`: el trabajo que hay en la cola sin
# terminar, lo haya empezado alguien o no. Lo publica `cmd/queue-exporter`.
#
# Con `pending` a secas el grupo oscila, y es aritmetica, no mala suerte: un
# trabajo desaparece de `pending` en el instante en que un worker lo empieza, asi
# que `pending` baja **porque alguien se puso a trabajar**. El autoescalador lee
# esa bajada como que sobra capacidad, retira la replica, el trabajo en vuelo se
# reencola y `pending` vuelve a subir.
#
# `outstanding` solo baja cuando un trabajo **termina**, asi que durante un
# vaciado decrece de forma monotona. Es lo que un autoescalador necesita para no
# perseguirse a si mismo.
#
# DONDE SE PUBLICA, Y POR QUE NO EN EL WORKER
#
# La serie la escribe la VM de la cola, que es el unico computo del despliegue
# fuera de los grupos elasticos. Tres razones, cualquiera de ellas suficiente:
#
#  1. **El autoescalador SUMA las series que devuelve este filtro.** La
#     profundidad de la cola es una magnitud global, asi que con tres workers
#     publicandola el autoescalador veria el triple del trabajo que hay.
#  2. Publicada desde el grupo, la senal desaparece durante una rotacion.
#  3. Con el tipo de recurso `gce_instance` --el que usaria el agente de
#     operaciones-- el autoescalador exige **una serie por instancia del grupo**
#     y **no admite filtrar por etiquetas de recurso**. Una serie de la VM de la
#     cola es de otra instancia: no la encontraria nunca. De ahi `generic_node`,
#     que si es filtrable.
# ==============================================================================

# ------------------------------------------------------------------------------
# Interruptor para aparcar el grupo
# ------------------------------------------------------------------------------
# Lo mismo que `api_autoscaling_enabled` y por lo mismo: `min_replicas` manda
# sobre cualquier `resize`, asi que mientras el autoescalador exista el grupo
# vuelve a levantar una instancia. Con `true` por omision, **cualquier `apply`
# desaparca el entorno**.
#
#   terraform apply -var worker_autoscaling_enabled=true -var db_activation_policy=ALWAYS
#
# y aparcar es lo contrario mas un `resize --size=0`, que no cuenta como deriva
# porque `target_size` esta en `ignore_changes`.
variable "worker_autoscaling_enabled" {
  description = "true crea el autoescalador de workers y enciende el grupo. Por omision false: el entorno se queda aparcado."
  type        = bool
  default     = false
}

variable "worker_min_replicas" {
  description = "Minimo de replicas del grupo de workers. 1 en operacion normal."
  type        = number
  default     = 1

  validation {
    condition     = var.worker_min_replicas >= 1 && var.worker_min_replicas <= 3
    error_message = "El enunciado fija el grupo entre 1 y 3 instancias."
  }
}

resource "google_compute_autoscaler" "worker" {
  count = var.worker_autoscaling_enabled ? 1 : 0

  name        = "mooc-worker-autoscaler"
  zone        = var.zone
  target      = google_compute_instance_group_manager.worker.self_link
  description = "Autoescalado de los workers por trabajos por atender en la cola (C6)"

  autoscaling_policy {
    # Los limites del enunciado. El minimo es 1 y no 0 por una razon de este
    # componente y no copiada de la API: con 0 replicas **nadie publica nada**,
    # pero la senal la escribe la VM de la cola, que no pertenece al grupo, asi
    # que escalar desde cero si funcionaria aqui.
    #
    # No se hace porque el arranque del worker son minutos --B3 midio 62-66 s de
    # arranque mas la imagen con FFmpeg-- y una subida de un alumno esperaria todo
    # eso antes de que alguien la mirara. El primer trabajo pagaria el arranque
    # entero.
    min_replicas = var.worker_min_replicas
    max_replicas = 3

    # `mode = "ON"`: escala en los dos sentidos. G6 tiene que medir el ciclo
    # completo, subida y bajada.
    mode = "ON"

    # `cooldown_period` es el nombre del proveedor para el **periodo de
    # inicializacion**: cuanto ignora el autoescalador los datos de una instancia
    # recien creada.
    #
    # 420 s, el mismo numero que `initial_delay_sec` del autohealing en C5, y a
    # proposito: las dos cosas contestan la misma pregunta --cuanto tarda un
    # worker nuevo en estar consumiendo de verdad-- y tener dos numeros distintos
    # para una sola pregunta es como se acaba con un autohealer matando lo que el
    # autoescalador acaba de pedir.
    #
    # Sigue siendo una estimacion y no una medida: hay que calibrarlo con
    # `/var/lib/mooc/arranque.json` del rol worker en cuanto C5 deje una. Si queda
    # por debajo del arranque real, el autoescalador ve una cola que no baja
    # --porque el worker nuevo aun no consume-- y pide otra replica, que hara lo
    # mismo.
    cooldown_period = 420

    metric {
      # El tipo de metrica es una constante de `cmd/queue-exporter/main.go`:
      # nombra un esquema. El **filtro**, en cambio, sale de
      # `local.queue_metric_namespace`, que es tambien lo que `queue.tf` le pasa
      # al exportador por entorno: el filtro y lo que se publica no pueden
      # divergir porque son el mismo valor.
      name = "custom.googleapis.com/mooc/queue/outstanding"

      # SIN `type` NI `target`, Y NO ES UN OLVIDO.
      #
      # El proveedor acepta dos modos de metrica personalizada y son
      # **mutuamente excluyentes**: `target` + `type` --mantener una metrica en un
      # valor, como una latencia media-- o `single_instance_assignment`
      # --repartir un total de trabajo entre instancias--. Declarar los dos es un
      # error de la API de Compute, no un aviso.
      #
      # Aqui es el segundo: `outstanding` es una cantidad de trabajo que se
      # reparte, no un nivel que se mantiene. `type = "GAUGE"` habria descrito
      # bien la metrica y habria roto el `apply`.

      # SIN ESTE FILTRO EL AUTOESCALADOR NO ENCUENTRA LA SERIE.
      #
      # Por omision busca el tipo de recurso `gce_instance` y una serie por cada
      # instancia del grupo. La nuestra es un `generic_node` escrito desde la VM
      # de la cola, asi que hay que decirselo. Con cualquier tipo distinto de
      # `gce_instance` la metrica se trata como **del grupo** y el filtro si se
      # aplica, que es exactamente lo que hace falta.
      filter = "resource.type = \"generic_node\" AND resource.labels.namespace = \"${local.queue_metric_namespace}\""

      # «TRABAJOS PENDIENTES POR CONSUMIDOR», que es lo que pide el enunciado.
      #
      # `single_instance_assignment` es el valor de la metrica que el
      # autoescalador asigna a cada instancia: replicas = outstanding / 2,
      # acotado entre 1 y 3.
      #
      #   outstanding   replicas
      #       2            1
      #       4            2
      #      >=6           3
      #
      # El 2 no es redondo por casualidad: es `mooc-worker-concurrency = 2`, los
      # procesos de transcodificacion que una instancia atiende a la vez. Con ese
      # valor, el tamano que pide el autoescalador es el numero de instancias con
      # el que **todo el trabajo esta siendo atendido y nada espera**.
      #
      # Subirlo encola trabajo a proposito --menos maquinas, mas espera-- y
      # bajarlo pide instancias que no tendrian con que llenar sus dos ranuras.
      # Si alguien cambia la concurrencia en `local.mooc_metadata_worker`, este
      # numero cambia con ella.
      single_instance_assignment = 2
    }

    # REDUCIR UN GRUPO DE WORKERS NO ES REDUCIR EL DE LA API.
    #
    # Retirar una replica de la API tira, como mucho, las peticiones en vuelo que
    # el drenaje de conexiones de C3 no alcance: 60 s. Retirar un worker
    # interrumpe un transcodificado, y A7 midio **1 711 s** en el perfil largo.
    #
    # Que no sea perdida de datos lo garantiza la idempotencia --asynq reencola y
    # el worker repite sin duplicar derivados-- pero es trabajo tirado, y el
    # trabajo reencolado vuelve a `outstanding`, asi que una reduccion agresiva se
    # desanda a si misma.
    #
    # Una replica cada **15 minutos** acota el dano a un trabajo en vuelo por
    # ventana, y de 3 a 1 tarda al menos media hora. Es deliberadamente mas lento
    # que los 600 s de la API.
    #
    # ESTO NO ES UN DRENAJE, Y NO HAY QUE LEERLO COMO SI LO FUERA. Acota la
    # frecuencia del dano; no espera a que el trabajo termine, y 900 s no cubren
    # los 1 711 s del perfil largo ni pretenden. Apagar el consumo de tareas
    # nuevas antes de parar es **C7 (#210)**, y hasta que C7 cierre esta ventana
    # es lo unico que hay.
    scale_in_control {
      max_scaled_in_replicas {
        fixed = 1
      }
      time_window_sec = 900
    }
  }

  depends_on = [google_compute_instance_group_manager.worker]
}

output "worker_autoscaler_name" {
  description = "Autoescalador de los workers (C6), o la cadena vacia si el grupo esta aparcado."
  value       = var.worker_autoscaling_enabled ? google_compute_autoscaler.worker[0].name : ""
}

output "worker_scaling_metric" {
  description = <<-EOT
    Metrica y filtro con los que el autoescalador decide. Sirve para pegarlos tal
    cual en el Explorador de Metricas: si ahi no sale serie, el autoescalador
    tampoco la ve, y un filtro vacio no da error --deja el grupo congelado--.
  EOT
  value = {
    metrica        = "custom.googleapis.com/mooc/queue/outstanding"
    filtro         = "resource.type = \"generic_node\" AND resource.labels.namespace = \"${local.queue_metric_namespace}\""
    por_consumidor = 2
  }
}
