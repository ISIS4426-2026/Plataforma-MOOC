# Evidencia C1 — Plantilla de instancia y grupo administrado de API (1-3) con autohealing ([#204](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/204))

| Criterio de aceptación | |
| :--- | :---: |
| Borrar una instancia a mano hace que el grupo la recree y quede operativa sin intervención | ⏳ §7 — requiere `apply` |
| Todas las instancias del grupo responden la misma versión | ✅ §3 — estructural |
| El grupo nunca baja de 1 ni sube de 3 | ✅ §4 — el techo lo impone C2 |
| La ampliación manual queda descartada como evidencia | ✅ §4 — `target_size = 1` y `ignore_changes` |
| **Bloquea el cierre:** la instancia borrada y el grupo recreándola, con tiempos | ⏳ §7 — requiere `apply` |
| Todo en Terraform, **aplicado desde `main`** | ⏳ §7 — escrito; el `apply` va después del merge |
| Anotado en `NOTAS_TECNICAS.md` | ✅ notas 34 y 35 |

**Estado: escrito y validado; sin desplegar.** `terraform validate` pasa, `fmt`
limpio, y el plan está en [`plan_c1.txt`](./plan_c1.txt): **tres recursos nuevos,
ninguna VM recreada**.

---

## 1. El bloqueo que había que resolver antes de poder escribir el autohealing

Esto es lo que de verdad costó C1, y no estaba en la lista de tareas del issue.

**Una instancia de grupo no puede servir con la configuración de nginx de la
Entrega 2.** Por dos razones independientes, y cada una basta:

1. **El certificado no existe.** `nginx.conf` termina TLS en el 443 con
   `ssl_certificate /etc/letsencrypt/live/${APP_DOMAIN}/fullchain.pem`. Los
   certificados viven en `/etc/letsencrypt` del anfitrión y **no hay contenedor
   de certbot**: se obtuvieron a mano. En un disco recién creado ese directorio
   está vacío, así que **nginx no arranca**.
2. **Certbot nunca podría obtenerlo.** Una instancia de grupo no tiene IP pública
   (B2), así que el desafío HTTP-01 no tiene por dónde llegar. No se sortea con
   más configuración.

Y encima, el puerto 80 de `nginx.conf` hace `return 301 https://$host$request_uri`
para todo lo que no sea el desafío de ACME. Una comprobación de salud de GCP
**solo acepta 200**:

> 301 → instancia marcada no sana → el autohealer la recrea → la nueva hace lo
> mismo. **Un bucle de recreación que se lee como «la imagen está rota».**

### Cómo se resuelve

TLS se termina en el **balanceador** (C3), con un certificado administrado por
Google, y los backends sirven **HTTP plano en el 80** dentro de la VPC. Como las
instancias no tienen IP pública, ese HTTP no viaja por internet en ningún tramo:
el único que habla con el 80 es el balanceador, más los rangos de comprobación de
salud, y el cortafuegos de B2 no deja entrar a nadie más.

El perfil nuevo es [`nginx.mig.conf`](../../../../nginx.mig.conf), y conserva de
`nginx.conf` las dos cosas que no son decorativas:

- la **denegación de `/api/v1/metrics`**, que de otro modo publicaría el
  inventario de rutas de la API, sus códigos de respuesta y su volumen de
  tráfico;
- las **cabeceras de proxy**, porque la API identifica al cliente por ellas y
  confía en `172.30.0.2`, la IP fija de ese contenedor en el Compose.

Con un detalle: `X-Forwarded-Proto` pasa por un `map` que cae al esquema real
cuando la cabecera no viene. El balanceador siempre la pone, pero una
comprobación de salud o una prueba desde dentro de la VPC no, y dejarla vacía es
lo que vuelve inseguras las cookies y rompe la generación de enlaces.

**No había que reinventar el mecanismo**: `docker-compose.prod.yml` ya tenía
`${NGINX_CONFIG_FILE:-./nginx.conf}`, con `nginx.bootstrap.conf` como precedente
para la fase previa al certificado. C1 añade el tercer perfil y lo selecciona por
metadata (`mooc-nginx-config`), validando en `prepare_web_env.sh` que sea uno de
los tres conocidos.

**La VM fija no cambia:** su valor por omisión sigue siendo `./nginx.conf`.

## 2. Zonal, y por qué no regional

El issue pide justificar la elección. Un grupo regional reparte las instancias
entre zonas y sobrevive a la caída de una. **Aquí no sobreviviría.**

| Dependencia | Dónde vive |
| :--- | :--- |
| Cloud SQL | `availability_type = "ZONAL"`, `us-east1-b` |
| Cola compartida (B1) | VM en `us-east1-b` |

Si esa zona cae, la base se va con ella. Las réplicas de las otras zonas
arrancarían y **fallarían su comprobación de salud**, porque `/api/v1/ready`
prueba la base. Lo único que sí cambiaría es el coste: el tráfico entre zonas se
factura, del orden de 0,01 USD/GiB, y cada petición cruzaría de zona para llegar
a la base.

**La alta disponibilidad de verdad empieza por Cloud SQL**, no por el grupo de la
API: `availability_type = "REGIONAL"` casi dobla el coste de la instancia. Mientras
esa decisión no se tome, un grupo regional es teatro caro.

La conversión, si algún día se toma: cambiar
`google_compute_instance_group_manager` por
`google_compute_region_instance_group_manager` y añadir `distribution_policy`.

## 3. Todas las instancias responden la misma versión, por construcción

No hace falta vigilarlo:

| | |
| :--- | :--- |
| La plantilla es **inmutable** | cambiarla crea otra, y el grupo rota a la nueva |
| `mooc-image-tag` es un SHA de 40 hex | validado en `variables.tf`; una etiqueta móvil dejaría dos instancias en imágenes distintas según cuándo arrancaron |
| Ese mismo SHA pin el código | `git checkout --detach` en `/opt/mooc` (B3): la imagen y los Compose son del mismo commit |
| El guion de arranque es idéntico | no se parametriza con `templatefile`; lee la metadata en ejecución (B3) |

`name_prefix` en vez de `name` en la plantilla no es cosmético: con un nombre
fijo, el `apply` fallaría por nombre duplicado antes de poder sustituirla.

## 4. Tamaño, rotación y la ampliación manual descartada

```hcl
target_size = 1
lifecycle { ignore_changes = [target_size] }
```

El rango **1–3 lo impone el autoescalador de C2**, no este recurso: un grupo con
autoescalador ignora `target_size`. Declararlo en tres aquí sería exactamente la
«ampliación manual» que el criterio de aceptación descarta como evidencia.

`ignore_changes` evita además que un `apply` posterior devuelva el tamaño a 1 y
deshaga una decisión del autoescalador a mitad de una corrida de carga — que es
el tipo de interferencia que invalidaría una medición de G5.

### La rotación no baja del mínimo

```hcl
max_surge_fixed       = 1
max_unavailable_fixed = 0
replacement_method    = "SUBSTITUTE"
```

`max_unavailable_fixed = 0` es lo que garantiza el criterio: el grupo levanta la
instancia nueva **antes** de retirar la vieja, en vez de dejar un hueco. El pico
es de una instancia por encima del objetivo, que cabe de sobra bajo el techo de 3.

## 5. El valor más delicado del fichero

```hcl
initial_delay_sec = 600
```

**Si el retardo inicial queda por debajo del arranque real, el autohealer mata
instancias que todavía se están montando** y las sustituye por otras que harán lo
mismo. Errar por arriba solo retrasa la sustitución de una instancia de verdad
muerta, así que el lado seguro es claro.

600 s cubren con holgura lo que hace `mooc-bootstrap.sh`: arrancar la VM,
instalar docker, git y psql, clonar el repositorio, leer dos secretos, aplicar
las migraciones y bajar y levantar los contenedores.

**Hay que calibrarlo a la baja** con el número que B3 escribe en
`/var/lib/mooc/arranque.json` en cuanto exista una medida real. A2 ya avisa de lo
mismo para el periodo de inicialización del autoescalador.

La comprobación de salud en sí es tolerante por diseño: tres fallos seguidos a
intervalos de 10 s, o sea **30 s** antes de declarar una instancia no sana. Es
tiempo de sobra para distinguir el reinicio de un contenedor de una máquina
perdida.

### Y es la sonda de A2, no la otra

`/api/v1/ready`, no `/api/v1/health`. A2 explica la diferencia: `health` dice que
el proceso vive y **aguanta la caída de la base**; `ready` prueba la base y la
bandera de arranque, y responde 503 mientras la instancia se está montando. Lo
que decide si una réplica recibe tráfico tiene que ser lo segundo.

Es la misma sonda que mide B3 y la que usará el backend del balanceador en C3 y
C4, para que **lo medido y lo decidido sean lo mismo**.

## 6. Un requisito para C3 que hay que dejar escrito

Las instancias del grupo reciben `mooc-app-domain` de `local.mooc_app_domain`,
que vacío se calcula como `<ip-publica-de-la-vm-fija>.sslip.io`. Y
`prepare_web_env.sh` deriva de ese valor **dos** cosas:

```
APP_BASE_URL='https://${APP_DOMAIN}'
CSRF_ALLOWED_ORIGINS='https://${APP_DOMAIN}'
```

Cuando el balanceador de C3 sirva en otra dirección, **toda petición mutante
que entre por él será rechazada como origen ajeno**. Es exactamente el
`csrf_origin_rejected` 403 que costó el primer intento del escenario 2 en A7, y
se parece a un fallo de la aplicación y no a un valor de configuración.

**Esto no bloquea los criterios de C1**, y conviene ser preciso sobre por qué: la
comprobación de salud es `/api/v1/ready`, que prueba la base y la bandera de
arranque, **no el origen**. Así que una instancia con el dominio equivocado sale
sana, el grupo la recrea igual, y las capturas de B1, B3 y C1 se pueden tomar sin
que el balanceador exista.

Lo que no se puede es **usar** la plataforma por el balanceador hasta que C3 fije
`var.app_domain` a su dirección. El patrón ya existe: igual que
`local.mooc_app_domain` calcula hoy `<ip>.sslip.io` de la dirección reservada de
la VM fija, C3 debe calcularlo de la dirección reservada del balanceador.

> Y hay una decisión de A4 que depende de esto: con el frontend **detrás del
> mismo balanceador** no hace falta ni CORS ni `SESSION_COOKIE_SAMESITE=none`.
> Si acabara en otro origen, hacen falta las tres cosas a la vez. Ver A4 §«Dónde
> se sirve el frontend».

## 7. Qué falta para cerrar

| Criterio | Qué necesita |
| :--- | :--- |
| Borrar una instancia y ver al grupo recrearla | el `apply` |
| **La captura mientras ocurre** (bloquea el cierre) | el `apply` |

Lo que hay que capturar, con los tiempos visibles:

1. `gcloud compute instances delete mooc-api-<sufijo>` y la hora;
2. la consola del grupo mostrando la instancia en recreación;
3. el instante en que la nueva pasa a sana, y el log serie con
   `B3: ===== LISTO en Ns =====`.

**No se puede rehacer después:** I6 destruye el entorno y las métricas de Cloud
Monitoring caducan (nota 6).

### C1 es lo que desbloquea el primer despliegue

Sus cuatro dependencias —A2, A3, B2, B3— están cerradas, así que con C1 escrito
ya se puede aplicar. Y conviene hacerlo **en una sola sesión**, porque tres issues
esperan la misma puesta en marcha:

| Issue | Qué captura en ese `apply` |
| :--- | :--- |
| **B1** | la cola inalcanzable desde fuera de la VPC y los trabajos sobreviviendo a su reinicio |
| **B3** | la consola del grupo creando una instancia y su log hasta lista |
| **C1** | la instancia borrada a mano y el grupo recreándola |

A7 costó 4,88 USD y el sobrecoste fue entorno encendido sin medir. Agrupar las
tres comprobaciones es la diferencia entre una sesión y tres.

### Sigue pendiente la decisión del prefijo del estado

El backend está en `prefix = "entrega2"` y hay que decidir si la Entrega 3 se
queda ahí o migra el objeto antes de cambiarlo. Un prefijo nuevo arranca con
estado vacío e intentaría recrear los buckets, que tienen nombre único global, y
fallaría. Lo documentó B1 §6.

### Tres variables para el `apply`

`TF_VAR_db_password`, `TF_VAR_image_tag` (SHA de 40 hex **ya publicado** con
`publish_images.sh`), `TF_VAR_smtp_from` y `TF_VAR_smtp_username`.

> Si `mooc-image-tag` nombra un commit cuya imagen no está en Artifact Registry,
> el arranque falla al bajar la imagen y el autohealer entra en bucle. Publicar
> antes de aplicar no es opcional.

---

Este archivo no contiene credenciales, llaves ni secretos.
