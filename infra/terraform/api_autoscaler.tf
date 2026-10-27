# ==============================================================================
# Politica de autoescalado de la API (C2 - issue #205)
# ==============================================================================
# POR QUE LA SENAL NO ES LA CPU, Y ESTA MEDIDO
#
# La eleccion obvia seria `cpu_utilization`. Con los datos de A7 no sirve, y la
# razon se ve en una tabla:
#
#   corrida       pet/s    CPU web (media/max)   CPU SQL      tps
#   A-nivel1        0,2      3,74 / 8,37 %     12,3 / 12,7 %    4,7
#   A-nivel200     32,5     12,00 / 15,55 %    24,7 / 30,7 %  350,9
#   B-nivel200    103,5     22,42 / 37,33 %    24,9 / 35,2 % 1039,7
#
# A **500 veces** el caudal, la CPU de la API pasa del 3,7 % al 22,4 %. La
# pendiente marginal es de unos **0,18 puntos de CPU por peticion/s**, asi que un
# objetivo del 60 % --el valor habitual-- se alcanzaria alrededor de
# **310 peticiones/s por instancia**.
#
# Y eso es imposible de sostener: a 103,5 peticiones/s, Cloud SQL ya iba a
# **1 039,7 tps**. Tres instancias a 310 peticiones/s pedirian del orden de
# 9 000 tps a una base de **1 vCPU**.
#
# Conclusion: con un objetivo de CPU, **el grupo se quedaria en una instancia
# mientras la base se funde**. La CPU de esta API no mide su carga porque la API
# no computa: traduce HTTP a SQL y espera. Lo que se agota primero esta en la
# base, no en el proceso.
#
# LA SENAL ELEGIDA: PETICIONES POR SEGUNDO Y POR INSTANCIA
#
# `load_balancing_utilization` escala sobre la fraccion de capacidad que el
# balanceador declara por instancia. Es la senal que corresponde a lo que la
# aplicacion hace --atender peticiones-- y la unica que se mueve de verdad con la
# carga en los datos de arriba.
#
# **DEPENDE DE C3 PARA TENER SENTIDO.** El denominador no esta aqui: es
# `max_rate_per_instance` en el backend service, que crea C3 con
# `balancing_mode = "RATE"`. Sin ese valor esta politica no tiene con que
# comparar y el autoescalador no escala. Esta anotado en la nota tecnica 37.
# ==============================================================================

resource "google_compute_autoscaler" "api" {
  name        = "mooc-api-autoscaler"
  zone        = var.zone
  target      = google_compute_instance_group_manager.api.self_link
  description = "Autoescalado de la API por peticiones por instancia (C2)"

  autoscaling_policy {
    # Los limites del enunciado. El minimo es 1 y no 0: con 0 no hay nadie que
    # responda la primera peticion, y el arranque son minutos.
    min_replicas = 1
    max_replicas = 3

    # `cooldown_period` es el nombre real del proveedor para el **periodo de
    # inicializacion**: cuanto ignora el autoescalador las metricas de una
    # instancia recien creada.
    #
    # 300 s, el mismo valor que `initial_delay_sec` del autohealing de C1, y por
    # la misma medida: B3 midio **62 s** de arranque y del borrado de una
    # instancia a que el grupo la declare sana pasaron **160 s**.
    #
    # Si se pone por debajo del arranque real, el autoescalador cuenta como
    # sobrecargada una replica que todavia se esta montando y pide otra, que hara
    # lo mismo. A2 avisa de esto mismo.
    cooldown_period = 300

    # `mode = "ON"` escala en los dos sentidos. Las alternativas del proveedor
    # son `ONLY_SCALE_OUT` --nunca reduce-- y `OFF`. Se declara explicito porque
    # G5 tiene que medir un ciclo completo, subida y bajada.
    mode = "ON"

    load_balancing_utilization {
      # `target` es la fraccion de `max_rate_per_instance` que el autoescalador
      # mantiene. Con el 0,8 que C3 declarara sobre 100 peticiones/s, se anade
      # una instancia al pasar de **80 peticiones/s por instancia**.
      #
      # De donde sale el 0,8 y no el 1,0: a 103,5 peticiones/s una sola
      # instancia daba p95 de 122 ms sin saturarse, asi que 80 deja margen para
      # que la instancia nueva arranque --sus 160 s-- antes de que la que ya
      # esta se acerque a lo medido.
      target = 0.8
    }

    # VENTANA DE ESTABILIZACION DE LA REDUCCION
    #
    # Sin esto, el autoescalador puede retirar instancias en cuanto la carga
    # baja, y una rafaga posterior encuentra el grupo en una sola replica con
    # 160 s de arranque por delante. El resultado es oscilacion: sube, baja,
    # sube.
    #
    # `max_scaled_in_replicas` acota **cuantas** puede retirar dentro de la
    # ventana, y `time_window_sec` es la ventana. Con 1 y 600 s, el grupo no
    # puede bajar mas de una instancia cada diez minutos: de 3 a 1 tarda al
    # menos veinte.
    #
    # Diez minutos es deliberadamente mas que los 160 s de arranque. Retirar es
    # barato y equivocarse al retirar es caro, asi que la asimetria es a
    # proposito.
    scale_in_control {
      max_scaled_in_replicas {
        fixed = 1
      }
      time_window_sec = 600
    }
  }

  depends_on = [google_compute_instance_group_manager.api]
}

output "api_autoscaler_name" {
  description = "Autoescalador de la API (C2). G5 mide su ciclo completo."
  value       = google_compute_autoscaler.api.name
}
