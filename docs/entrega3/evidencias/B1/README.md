# Evidencia B1 — Cola compartida en un servicio dedicado fuera de ambos grupos elásticos ([#200](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/200))

| Criterio de aceptación | |
| :--- | :---: |
| Ni la API ni los workers dependen de un Redis local a la instancia | ✅ §4, y confirmado en el despliegue |
| Destruir cualquier instancia de los dos grupos elásticos no afecta la cola ni los trabajos encolados | ⏳ §6 — requiere C1 y C5 |
| La cola no es alcanzable desde internet, comprobado desde fuera de la VPC | ✅ §0 — comprobado el 2026-10-27 |
| Los trabajos encolados sobreviven al reinicio del servicio de cola | ✅ §0 — con claves sintéticas; alcance declarado |
| Anotado en `NOTAS_TECNICAS.md` | ✅ nota 30, y la nota 4 queda resuelta |

**Estado: escrito y validado estáticamente; sin desplegar.** Lo que falta para
cerrar son tres comprobaciones que exigen el entorno encendido, y una de ellas
exige además que existan los grupos administrados. Está detallado en la §6, con
lo que bloquea el primer `apply` de la Entrega 3.

---

## 0. Resultado del despliegue del 2026-10-27

### Un fallo real, encontrado solo al desplegar

La API moría en bucle con
`ping redis: dial tcp 10.0.1.8:6379: connect: connection refused`.

**Causa.** `apt-get install redis-server` **ya deja el servicio corriendo** con la
configuración por omisión de Debian, que escucha solo en `127.0.0.1`. El guión la
sobrescribía entera y hacía `systemctl enable --now`, pero **`--now` no hace nada
sobre un servicio que ya está activo**: no es `restart`. El fichero en disco decía
`bind 10.0.1.8`, el proceso escuchaba en loopback.

**Y la comprobación de arranque no lo detectó**, que es la mitad peor del fallo.
Hacía `CONFIG GET appendonly` y **mostraba** el resultado sin comprobarlo, así que
salía con 0 estando mal. Peor: `maxmemory-policy` coincidía **por casualidad**,
porque `noeviction` es también el valor por omisión de Debian, de modo que mirar
solo esa clave no habría delatado nada.

**Corregido en las dos mitades:**

- `systemctl enable` **y** `systemctl restart`, no `enable --now`;
- la comprobación **afirma** `appendonly = yes`, `maxmemory-policy = noeviction`
  y que escucha en la IP privada, y sale con 1 si algo no cuadra.

### Superficie pública

[`superficie_publica.txt`](./superficie_publica.txt):

| | |
| :--- | :--- |
| IP externa de `mooc-queue-server` | **ninguna** |
| IP externa de la instancia del grupo | **ninguna** |
| 6379 y 5432 contra la única IP pública que queda | cerrados o filtrados |
| `mooc-allow-queue` | origen y destino **por cuenta de servicio**, como se diseñó |

No hay dirección que barrer desde internet, así que la propiedad es estructural y
no depende del cortafuegos.

### Persistencia al reinicio

[`persistencia_reinicio.txt`](./persistencia_reinicio.txt). Reinicio **duro** del
servicio —`uptime_in_seconds:0` confirma proceso nuevo— y sobreviven las tres
cosas: el registro `asynq:queues`, las 2 tareas pendientes de la base 0 y la
clave de sesión de la base 1.

> **Alcance declarado.** Se probó la persistencia AOF con **claves sintéticas**
> con la forma que usa asynq. Lo que **no** se probó es que asynq relea su
> registro tras el reinicio, porque eso exige tareas reales y por tanto el grupo
> de workers: es **C5**. Sembrar cuentas y hacer una subida completa no entraba
> en esta sesión.

### La configuración efectiva es la declarada

[`config_efectiva_cola.txt`](./config_efectiva_cola.txt): `appendonly yes`,
`appendfsync everysec`, `maxmemory 1468006400`, `maxmemory-policy noeviction`,
`databases 16`, escuchando en `10.0.1.8:6379` y `127.0.0.1:6379`.

Y el candado de la nota 27 funciona en el servidor:

```
redis-cli FLUSHDB
ERR unknown command 'FLUSHDB', with args beginning with:
```

## 1. La decisión, y qué la decide

El issue pide elegir entre Memorystore for Redis y una VM dedicada gestionada por
Terraform, comparando coste, operación y persistencia. **Se elige la VM
dedicada**, y lo que decide no es el coste.

| | Persistencia | Pérdida al reiniciar | USD/mes |
| :--- | :--- | :--- | ---: |
| Memorystore Basic M1 (1 GiB) | solo instantáneas RDB, intervalo mínimo 1 h | **hasta 1 hora de cola** | 35,77 |
| Memorystore Standard M1 (1 GiB) | RDB + réplica | cubre el primario, no un reinicio completo | 46,72 |
| **VM dedicada con Redis** | **AOF, `appendfsync everysec`** | **≤ 1 segundo** | **13,23** |

**Lo decisivo es el criterio de aceptación «los trabajos encolados sobreviven al
reinicio del servicio de cola»: Memorystore no soporta AOF.** Su única
persistencia son instantáneas RDB, y el nivel Basic se recupera precisamente de
la última instantánea cada vez que la instancia se reinicia, se escala o se
actualiza. Con el intervalo mínimo de una hora, una cola puede perder una hora de
trabajos. Pagar 2,7 veces más por incumplir un criterio de aceptación no tiene
defensa.

Tres cosas que **no** diferencian, y conviene decirlo para que no parezca que la
comparación se inclinó sola:

- **`maxmemory-policy noeviction` sí es configurable en Memorystore.** El
  requisito de asynq se puede cumplir en las dos opciones.
- **La IP privada también.** Memorystore se despliega en una IP privada de la
  VPC sin acceso público, igual que esta VM.
- **La operación es más sencilla en Memorystore**, y es su ventaja real: parches,
  supervisión y reemplazo del nodo son de Google. Aquí los asumimos nosotros.

### Lo que sí separa, además de la persistencia

**Las bases lógicas.** El código ya separa las sesiones en la base **1** y la
cola de asynq en la **0** (`REDIS_SESSION_DB`, `internal/cache/redis.go`).
Memorystore no documenta `SELECT`, y en modo Cluster solo existe la base 0. Con
Memorystore habría que mover las sesiones a prefijos de clave sobre la misma
base, que es trabajo de migración para perder una separación que ya funciona.

### Lo que se pierde, declarado

**La alta disponibilidad gestionada.** `mooc-queue-server` es un **punto único de
fallo** para los dos grupos elásticos: si cae, ni la API encola ni los workers
consumen. Se mitiga, no se resuelve:

- `automatic_restart = true` y `on_host_maintenance = "MIGRATE"`, que quitan de
  la ecuación las dos caídas que no son un fallo real de la máquina.
- El AOF, que hace que un reinicio no sea una pérdida de datos.

No se disimula porque es la contrapartida honesta de la decisión: el enunciado
pide que la cola esté **fuera** de los grupos elásticos, no que sea
redundante. Si la Entrega 3 quisiera redundancia, la opción es Memorystore
Standard y hay que aceptar su ventana de pérdida, o un segundo nodo con
replicación que habría que operar.

## 2. Qué se aprovisiona

En [`infra/terraform/queue.tf`](../../../../infra/terraform/queue.tf) y
[`queue_startup.sh`](../../../../infra/terraform/queue_startup.sh):

| Recurso | Qué es |
| :--- | :--- |
| `google_compute_address.queue_internal_ip` | IP privada **estática** dentro de `mooc-subnet` |
| `google_compute_instance.queue_server` | `mooc-queue-server`, `e2-small`, Debian 12, disco 10 GiB, **sin IP pública** |
| `google_compute_firewall.allow_queue_internal` | 6379/tcp solo desde el rango de la subred, hacia la etiqueta `queue-server` |
| `google_service_account.queue_server` | `sa-queue-server`, con dos roles de telemetría y nada más |

### Por qué una IP interna estática

Sin reservarla, la asigna el DHCP de la subred y **cambia si la VM se recrea**.
`REDIS_URL` se escribe en la configuración de cada instancia que arranca (B3) y
en las plantillas de los grupos (B4, C5): una dirección que cambia obliga a rotar
los dos grupos cada vez que se toca la cola.

Es la misma lección que costó tiempo en A7: al recrear Cloud SQL, las dos VMs
quedaron apuntando a una IP privada que ya no existía y hubo que reapuntarlas a
mano.

### Por qué `e2-small`, que el proyecto había descartado

`docs/entrega2/CONFIGURACION_Y_COSTOS.md` descarta `e2-small` para las VMs de
aplicación **porque es de núcleo compartido**. Aquí sirve, y por el motivo
contrario: Redis es de un solo hilo y la carga es minúscula —el pico medido en
A7 fueron **14 tareas en cola**—, así que lo que hace falta es memoria con
holgura, no vCPU dedicada.

El tamaño no lo dicta el volumen de datos. Los trabajos de asynq son de unos
cientos de bytes y las sesiones también: con los números de A7, la cola entera
son **kilobytes**. Lo que dicta el tamaño es no bajar de la máquina más pequeña
que deja margen cómodo, porque con `noeviction` quedarse sin memoria no corrompe
nada pero **detiene el encolado**.

### La cuenta de servicio no hereda los roles comunes

`sa-queue-server` está deliberadamente **fuera** de `local.service_accounts` y
por tanto del producto cartesiano de `common_roles`, porque ese incluye
`roles/cloudsql.client` y la cola no abre la base de datos. Solo recibe
`logging.logWriter` y `monitoring.metricWriter`.

Esos dos no son decorativos: **C6** necesita leer `worker.queue.pending` y
`oldest_pending_age_seconds`, y el exportador de esa señal tiene que vivir fuera
de los grupos elásticos o el autoescalador se queda ciego justo cuando reduce a
una instancia (nota técnica 4). Esta VM es su sitio natural.

## 3. Persistencia, tamaño y expulsión

La configuración completa está en
[`queue_startup.sh`](../../../../infra/terraform/queue_startup.sh), comentada
directiva a directiva. Lo que importa:

| Directiva | Valor | Por qué |
| :--- | :--- | :--- |
| `appendonly` | `yes` | Es la razón de no usar Memorystore |
| `appendfsync` | `everysec` | Pierde ≤ 1 s. Con `always` el encolado se vuelve síncrono con el disco; con `no` se pierde lo que el sistema operativo tenga en caché |
| `save` | 900/300/60 | RDB **además** del AOF: segundo camino de recuperación si el AOF se corrompe |
| `maxmemory` | `1400mb` | De los 2 GiB de la máquina, dejando el resto al sistema |
| `maxmemory-policy` | **`noeviction`** | asynq lo exige. Un trabajo expulsado es un trabajo perdido, y se pierde en silencio |
| `databases` | `16` | La 0 es la cola, la 1 las sesiones |
| `bind` | IP privada + loopback | No escucha en ninguna interfaz pública |

### La decisión que la nota 23 dejaba abierta

La nota 23 planteaba el problema y exigía elegir: la política de expulsión es de
la instancia entera, no de la base, así que si las sesiones llenaran la memoria
**lo que rompe es la cola**. Las dos salidas eran dimensionar con holgura y
vigilar, o dar instancias separadas.

**Se elige una sola instancia con holgura**, porque los datos reales son
kilobytes contra 1,4 GiB de techo y dos instancias duplicarían el coste y el
punto único de fallo. Lo que esa elección obliga es a **vigilar la memoria**: la
alerta es parte del trabajo de G1, y sin ella la decisión está a medias. Queda
anotado como tal, no como resuelto.

### Dos comandos desactivados en el servidor

```
rename-command FLUSHALL ""
rename-command FLUSHDB ""
```

Mientras cada máquina tenía su propio Redis, un `FLUSHDB` equivocado era un
problema de laboratorio. Con una cola compartida se lleva la cola real: los
trabajos pendientes y, peor, el registro `asynq:queues`, que deja al inspector
respondiendo `NOT_FOUND: queue "default" does not exist` mientras los trabajos
siguen fluyendo. Es la **nota técnica 27**, y costó tres corridas de A7 medidas a
oscuras.

El candado va en el servidor porque **el agujero ya existía en un guion del
repositorio**: `scripts/seed.sh` hacía `FLUSHDB` contra `${REDIS_URL}`, que en un
despliegue es la cola de producción. Ese guion también se corrigió —solo vacía el
Redis local de Compose—, pero la defensa no puede depender de la buena memoria de
quien lo ejecuta.

## 4. Ni la API ni los workers dependen de un Redis local

| Cambio | Antes | Ahora |
| :--- | :--- | :--- |
| `docker-compose.worker.yml` | servicio `redis` + volumen `redis_data` + `depends_on` | **eliminados**; solo queda `worker` |
| `REDIS_URL` del worker | `${REDIS_URL:-redis:6379}` | `${REDIS_URL:?...}` — **obligatoria** |
| `scripts/prepare_worker_env.sh` | `REDIS_URL='redis:6379'` | `REDIS_URL='${QUEUE_PRIVATE_IP}:6379'`, con `QUEUE_PRIVATE_IP` requerida y validada |
| `scripts/prepare_web_env.sh` | ya usaba `QUEUE_PRIVATE_IP` | el texto decía «Worker Server»; ahora dice la cola compartida |
| `docker-compose.prod.yml` | ya exigía `REDIS_URL` (A4) | sin cambios |

`docker-compose.yml`, el de desarrollo local, **conserva su `redis`** a
propósito: en local no hay VPC ni cola compartida, y los guiones de validación
siguen necesitándolo.

### Comprobado

```
$ IMAGE_TAG=x DATABASE_URL=y docker compose -f docker-compose.worker.yml config
error while interpolating services.worker.environment.[]: required variable
REDIS_URL is missing a value: REDIS_URL debe apuntar a la cola compartida, no a
este host

$ IMAGE_TAG=x DATABASE_URL=y REDIS_URL=<ip-de-la-cola>:6379 \
    docker compose -f docker-compose.worker.yml config --services
worker
```

Un valor ausente **para el despliegue**, en vez de elegir el Redis equivocado en
silencio. Es la lección de G3 en la Entrega 2: con un valor por omisión, la API
encolaba en un intermediario que nadie leía, las subidas respondían 202 y el
recurso se quedaba `pending` para siempre.

`terraform validate` pasa y `terraform fmt -check` no propone cambios. El plan de
solo lectura está en [`plan_b1.txt`](./plan_b1.txt):

```
Plan: 9 to add, 1 to change, 0 to destroy.
```

**Cero destrucciones**, que es la condición que A7 §1 dejó escrita para cualquier
rama de la Entrega 3 que toque Terraform: si apareciera una, significaría que el
estado ya no es el que A7 dejó. De las nueve altas, **seis son de B1**; las tres
`google_sql_*` son el entorno que A7 destruyó al cerrar y las recrea quien
despliegue. El único cambio en sitio es la descripción de `sa-worker-server`, que
ya no aloja la cola.

## 5. Coste mensual

Precios de lista de `us-east1`, 730 h/mes, el mismo marco que
`docs/entrega2/CONFIGURACION_Y_COSTOS.md`.

| Concepto | USD/mes |
| :--- | ---: |
| `e2-small` (2 vCPU de núcleo compartido, 2 GiB) | 12,23 |
| Disco de arranque 10 GiB `pd-balanced`, a 0,10 USD/GiB-mes | 1,00 |
| IP interna estática en uso | 0,00 |
| Egreso dentro de la zona hacia las VMs y los grupos | 0,00 |
| **Total** | **13,23** |

Frente a **35,77** de Memorystore Basic M1 y **46,72** de Standard M1: se ahorran
**22,54 USD/mes** y además se cumple el criterio de persistencia.

Para dimensionar: los cupones son 50 USD por integrante, y A7 entera —dos
escenarios, dos códigos, 26 h de entorno— costó 4,88 USD. Una cola encendida un
mes completo cuesta casi tres veces esa campaña, así que **esta VM se apaga con
el resto del entorno** cuando no se está midiendo. El coste real lo cierra F3 con
el consumo observado.

## 6. Qué falta para cerrar, y qué lo bloquea

Tres criterios de aceptación no se pueden comprobar sin el entorno encendido.

| Criterio | Cómo se comprueba | Qué necesita |
| :--- | :--- | :--- |
| La cola no es alcanzable desde internet | `nmap`/`nc` al 6379 desde fuera de la VPC, y confirmar que la VM no tiene IP pública | `apply` |
| Los trabajos encolados sobreviven al reinicio | encolar, `systemctl restart redis-server`, comprobar que el inspector los sigue viendo | `apply` |
| Destruir una instancia de cualquier grupo no afecta la cola | destruir una instancia del MIG y verificar que la cola y sus trabajos siguen | **C1 y C5**: los grupos administrados no existen todavía |

El tercero **no es un bloqueo de B1**: la cola ya está fuera de cualquier grupo
por construcción, puesto que es una instancia independiente que ningún MIG
gestiona. Pero demostrarlo exige que haya un MIG que destruir.

`capturas/` está creado y vacío a propósito: las tres capturas que el issue pide
—la instancia sin IP pública en la consola, el 6379 cerrado desde fuera de la
VPC, y el inspector con los trabajos intactos tras el reinicio— son de esas
mismas comprobaciones, así que llegan con el `apply`.

**Conviene no encender el entorno solo para esto.** A7 costó 4,88 USD y el
sobrecoste fue entorno encendido sin medir. Lo eficiente es desplegar una vez y
cerrar en la misma sesión las comprobaciones de B1, B2 y B3.

### Lo que bloquea el primer `apply` de la Entrega 3

El backend sigue en `prefix = "entrega2"`
([`versions.tf`](../../../../infra/terraform/versions.tf)), y el README de A7 §1
dejó anotado que la Entrega 3 estrena `entrega3`. **Hay que resolverlo antes del
primer `apply`, y no es una decisión de B1.**

El detalle que importa: un prefijo nuevo arranca con **estado vacío** e intentaría
crear de cero los recursos que A7 dejó vivos a propósito —los buckets, que tienen
nombre único global, el Artifact Registry, la VPC, los secretos y las cuentas de
servicio—, y fallaría en los buckets.

Las dos salidas, para decidir en B2 o I6:

1. **Seguir en `entrega2`.** Técnicamente no cuesta nada y respeta el invariante
   que A7 estableció: una sola arquitectura viva a la vez. El nombre del prefijo
   deja de coincidir con la entrega, que es cosmético.
2. **Migrar el estado** copiando el objeto de `entrega2/` a `entrega3/` en el
   bucket, y entonces cambiar el prefijo. Mantiene el nombre coherente sin
   recrear nada.

Lo que **no** vale es cambiar el prefijo y aplicar: eso intenta recrear los
buckets y falla.

---

Este archivo no contiene credenciales, llaves ni secretos.
