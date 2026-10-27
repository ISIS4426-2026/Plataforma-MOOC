# Evidencia B2 — Red, salida a internet, firewall e identidades de servicio de los grupos ([#201](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/201))

| Criterio de aceptación | |
| :--- | :---: |
| Un barrido desde internet no encuentra puertos de aplicación abiertos en las instancias de los grupos | ✅ §2 — por estructura; barrido en §6 |
| El balanceador y las verificaciones de salud sí llegan, con evidencia | ⏳ §6 — requiere C1 y C3 |
| La base de datos, la cola y los workers no tienen superficie pública | ✅ §3 |
| Cada grupo usa su propia identidad y ninguna tiene permisos que no use | ✅ §4, con **una excepción declarada** |
| Anotado en `NOTAS_TECNICAS.md` | ✅ nota 31 |

**Estado: escrito y validado estáticamente; sin desplegar.** `terraform validate`
pasa, `fmt` no propone cambios y el plan está en [`plan_b2.txt`](./plan_b2.txt).

---

## 1. Lo que ya estaba, y no hacía falta tocar

Dos tareas del issue estaban cubiertas desde la Entrega 2, y conviene decirlo en
vez de reescribirlas para aparentar trabajo:

| Tarea | Dónde está | Estado |
| :--- | :--- | :--- |
| Acceso privado a Google para Artifact Registry, Secret Manager, Cloud Storage y Cloud Logging | `private_ip_google_access = true` en `mooc-subnet` | ya estaba |
| Cloud NAT para la salida de instancias sin IP pública | `mooc-router` + `mooc-nat`, con `ALL_SUBNETWORKS_ALL_IP_RANGES` | ya estaba |
| Acceso administrativo por IAP, sin SSH público | `mooc-allow-ssh-iap`, origen `35.235.240.0/20` | ya estaba |
| Identidades separadas para API y workers | `sa-web-server` y `sa-worker-server` | ya estaban |

Las dos primeras importan juntas: con acceso privado a Google **y** NAT, una
instancia sin IP externa puede bajar su imagen del registro, leer su secreto y
enviar registros y métricas. Es lo que hace viable que **ninguna** instancia de
los grupos tenga dirección pública.

## 2. El cambio de fondo: la regla interna abierta ya no existe

Esto es lo que B2 realmente arregla.

```hcl
# Antes
resource "google_compute_firewall" "allow_internal" {
  allow { protocol = "tcp"  ports = ["0-65535"] }
  allow { protocol = "udp"  ports = ["0-65535"] }
  source_ranges = ["10.0.1.0/24"]
}
```

**Todos** los puertos TCP y UDP entre cualquier par de instancias de la subred.
Con dos VMs fijas era una comodidad; con grupos administrados que levantan y
retiran instancias es una superficie que crece sola. Y hacía **decorativa**
cualquier regla más específica: la que B1 escribió para el 6379 de la cola no
restringía nada mientras esta existiera, y así quedó dicho en su evidencia.

En su lugar, tres reglas con propósito y una ausencia deliberada:

| Regla | Origen | Destino | Puerto |
| :--- | :--- | :--- | :--- |
| `mooc-allow-lb-health` | `130.211.0.0/22`, `35.191.0.0/16` | etiqueta `mooc-api-node` | 80/tcp |
| `mooc-allow-queue` | cuentas de la API y de los workers | cuenta de la cola | 6379/tcp |
| `mooc-allow-internal-icmp` | rango de la subred | toda la subred | ICMP |
| `mooc-allow-ssh-iap` | `35.235.240.0/20` | etiqueta `allow-iap-ssh` | 22/tcp |

### Las dos etiquetas de los grupos

| Etiqueta | Quién la lleva | Entrada permitida |
| :--- | :--- | :--- |
| `mooc-api-node` | instancias del grupo de la API (C1) | solo balanceador y verificaciones de salud |
| `mooc-worker-node` | instancias del grupo de workers (C5) | **ninguna**, salvo IAP |

**No hay ninguna regla con destino `mooc-worker-node`,** y es deliberado: nadie
le habla a un worker. Saca trabajos de la cola y escribe en la base y en el
bucket, y la salida no necesita regla porque la salida por omisión está
permitida.

### Por qué la regla de la cola va por cuenta de servicio y no por etiqueta

```hcl
source_service_accounts = [sa-web-server, sa-worker-server]
target_service_accounts = [sa-queue-server]
```

Una etiqueta de red la puede poner cualquiera que sepa crear una instancia, y
entonces la regla protege un nombre, no un componente. La cuenta de servicio la
fija la plantilla del grupo (C5) y cambiarla es una operación de IAM, no de red.

Detalle de la API de Compute que condiciona la forma: `source_service_accounts`
**no se puede combinar con etiquetas en ninguno de los dos lados**, así que el
destino también va por cuenta. Por eso esa regla no usa
`target_tags = ["queue-server"]`.

### Lo que esta regla presupone de C3, y hay que respetar

`mooc-allow-lb-health` abre los dos rangos que usa un **balanceador de
aplicación externo global**, que entrega el tráfico a los backends desde
`130.211.0.0/22` y `35.191.0.0/16` —los mismos de las comprobaciones de salud—.

Un balanceador de aplicación **regional** se basa en Envoy y entrega desde una
**subred de solo proxy** (`REGIONAL_MANAGED_PROXY`) que habría que crear, y
entonces el origen sería el rango de esa subred y no estos dos. **C3 tiene que
elegir global, o volver a `network.tf`.** El puerto es el 80 porque el
balanceador termina TLS y habla con los backends por HTTP dentro de la VPC; sin
IP externa en las instancias, ese HTTP no viaja por internet en ningún tramo.

### Por qué el barrido desde internet no puede encontrar nada

No depende del cortafuegos: **las instancias de los grupos no llevan IP
externa**, así que no hay dirección a la que llamar. El cortafuegos gobierna lo
que llega por dentro de la VPC y lo que entrega Google —balanceador, salud,
IAP—, que no son el camino público hacia la instancia.

Esto convierte el criterio en estructural y no en una regla que alguien pueda
ampliar por error. Las plantillas de C1 y C5 son las que tienen que omitir
`access_config`; está anotado para las dos.

## 3. El worker se queda sin IP pública

`mooc-worker-server` tenía una **IP externa estática**, descrita en su día como
«cerrada por firewall perimetral para aislar la cola». Es decir: superficie sin
función. Esa máquina no recibe tráfico de nadie, su salida va por el Cloud NAT
que ya existía, y la administración entra por IAP, que no necesita dirección
externa.

Retirada, junto con la reserva `google_compute_address.worker_server_ip` —una IP
estática reservada y sin adjuntar **se factura**, así que dejar el recurso
habría sido pagar por nada— y la salida `worker_server_public_ip`.

Con eso, el criterio «la base de datos, la cola y los workers no tienen
superficie pública» se cumple **entero y por estructura**:

| Componente | Superficie pública |
| :--- | :--- |
| Cloud SQL | ninguna: solo IP privada, al otro lado del peering de Private Services Access |
| Cola (`mooc-queue-server`) | ninguna: sin `access_config` desde B1 |
| Worker fijo (`mooc-worker-server`) | **ninguna, desde B2** |
| Instancias de los grupos (C1, C5) | ninguna: sus plantillas no declaran `access_config` |

La única dirección pública que queda es la de `mooc-web-server`, la VM fija de la
Entrega 2, y su regla 80/443 sigue acotada a la etiqueta `web-server`, que solo
lleva ella. **No alcanza a los grupos elásticos**, que entran por
`mooc-api-node`. Se retira cuando el balanceador de C3 sea el único camino de
entrada y C1 sustituya esa VM; quitarla ahora dejaría la arquitectura de la
Entrega 2 inalcanzable sin nada que la reemplace.

## 4. Auditoría de identidades: qué tiene cada cuenta y para qué

El criterio es «ninguna tiene permisos que no use», así que la prueba es esta
tabla y no la afirmación.

### `sa-web-server` — la API

| Permiso | Alcance | Para qué, concretamente |
| :--- | :--- | :--- |
| `cloudsql.client` | proyecto | abre la conexión a la base; no puede administrar la instancia |
| `logging.logWriter` | proyecto | el agente de operaciones envía los registros |
| `monitoring.metricWriter` | proyecto | el agente envía las métricas de `infra/ops-agent/web.yaml` |
| `iam.serviceAccountTokenCreator` | **sobre sí misma** | firma URLs V4 sin archivo de llave, delegando en `signBlob` |
| `storage.objectViewer` | bucket de medios | lee objetos para servirlos y firmarlos |
| `storage.objectCreator` | bucket de medios, sin derivados | lo que **la firma concede al cliente** en la subida directa |
| `secretmanager.secretAccessor` | secreto `db-password` | lee la contraseña al preparar el entorno |
| `secretmanager.secretAccessor` | secreto SMTP | envía correo |
| `artifactregistry.reader` | repositorio | baja su propia imagen |

Un detalle que no es obvio: `storage.objectCreator` parece sin uso porque la
API no escribe el objeto —lo escribe el navegador contra el almacenamiento—,
pero **una URL firmada concede exactamente lo que puede hacer quien firma**. Sin
ese permiso, el PUT del cliente recibiría 403. Se usa, por delegación.

### `sa-worker-server` — los workers

| Permiso | Alcance | Para qué |
| :--- | :--- | :--- |
| `cloudsql.client` | proyecto | actualiza el estado del recurso al terminar |
| `logging.logWriter` | proyecto | agente de operaciones |
| `monitoring.metricWriter` | proyecto | agente de operaciones y métricas de cola |
| `storage.objectViewer` | bucket de medios | lee el original a transcodificar |
| `storage.objectUser` | bucket HLS | escribe los derivados |
| `secretmanager.secretAccessor` | secreto `db-password` | contraseña al preparar el entorno |
| `artifactregistry.reader` | repositorio | baja su propia imagen |

Lo que **no** tiene, y es la separación que las dos cuentas existen para
mantener: no puede firmar URLs (`serviceAccountTokenCreator`) ni leer el secreto
de SMTP. No emite enlaces y no manda correo.

### `sa-queue-server` — la cola

| Permiso | Alcance | Para qué |
| :--- | :--- | :--- |
| `logging.logWriter` | proyecto | agente de operaciones |
| `monitoring.metricWriter` | proyecto | **pendiente de C6** — ver la excepción |

Deliberadamente **fuera** del producto cartesiano de `common_roles`, porque ese
incluye `cloudsql.client` y la cola no abre la base. Tampoco está en los mapas
explícitos de `db_password_readers` ni de `runtime_readers`: no lee secretos y no
baja imágenes, porque instala Redis desde el repositorio de Debian.

### La excepción, declarada

**`monitoring.metricWriter` en `sa-queue-server` hoy no se usa.** Está concedido
en previsión de **C6**: la señal de autoescalado se calcula sobre la cola, y el
exportador de esa métrica tiene que vivir fuera de los grupos elásticos o el
autoescalador se queda ciego justo cuando reduce a una instancia (nota técnica
4). Esta VM es su sitio natural.

Si C6 pone el exportador en otro lado, **este permiso hay que retirarlo**. Queda
escrito aquí y en la nota 31 para que no se quede como un permiso huérfano que
nadie recuerde por qué existe, que es exactamente la forma en que los permisos
sin uso sobreviven a las auditorías.

## 5. Las dos destrucciones del plan son el objeto del issue

```
Plan: 11 to add, 3 to change, 2 to destroy.
```

| Se destruye | Por qué |
| :--- | :--- |
| `google_compute_firewall.allow_internal` | es la regla que abría todos los puertos; retirarla **es** B2 |
| `google_compute_address.worker_server_ip` | IP externa que no recibía tráfico, y que reservada sin adjuntar se factura |

`mooc-web-server` y `mooc-worker-server` **no se recrean**: quitar un
`access_config` es una actualización en sitio. El plan lo confirma
(`will be updated in-place`).

> La regla de A7 §1 —«el pre-vuelo se para si aparece cualquier destrucción»— era
> una condición **de A7**, para garantizar que el estado seguía siendo el de la
> Entrega 2 mientras se medía la línea base. A7 está cerrado y su entorno
> destruido, así que ya no aplica. Lo que sí sigue vigente es mirar cada
> destrucción y poder explicarla, que es esta tabla.

## 6. Qué falta para cerrar

| Criterio | Cómo se comprueba | Qué necesita |
| :--- | :--- | :--- |
| El balanceador y las verificaciones de salud sí llegan | backends sanos en la consola y una petición servida a través del balanceador | **C1 y C3** |
| Barrido desde internet | `nmap` contra la IP del balanceador y comprobar que ninguna instancia del grupo tiene dirección propia | **C1**, y el `apply` |

El barrido está marcado ✅ en la tabla de arriba porque la propiedad es
estructural —sin IP externa no hay a qué llamar— pero **la demostración con
`nmap` necesita el entorno**. Las capturas que el issue pide son de esas mismas
comprobaciones, así que `capturas/` está creado y vacío a propósito.

Sigue en pie el bloqueo que B1 documentó: el backend del estado remoto está en
`prefix = "entrega2"`, y hay que decidir si la Entrega 3 se queda ahí o migra el
objeto de estado antes de cambiar el prefijo. **Un prefijo nuevo arranca con
estado vacío e intentaría recrear los buckets, que tienen nombre único global, y
fallaría.**

---

Este archivo no contiene credenciales, llaves ni secretos.
