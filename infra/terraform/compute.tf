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
# La VM fija del worker se retiro en C5 (issue #208)
# ==============================================================================
# La sustituye el grupo administrado de `worker_group.tf`. Se retira por la misma
# razon por la que C3 retiro la web, aunque el motivo no sea la superficie
# publica --esa ya se la quito B2--: un consumidor fijo **fuera** del grupo
# tambien saca tareas de la cola compartida, asi que una medicion del escalado
# (G6) no podria atribuir el trabajo al grupo.
#
# «El grupo escala instancias, no solo procesos dentro de una maquina» es un
# criterio de C5, y con una VM suelta consumiendo en paralelo no se puede
# demostrar.
#
# Con esto, `compute.tf` se queda sin instancias: las dos VMs fijas de la
# Entrega 2 han sido sustituidas por los dos grupos.
# ==============================================================================
