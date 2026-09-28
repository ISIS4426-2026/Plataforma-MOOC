# Evidencia I1 — Modelo de componentes, despliegue y decisiones (issue #136)

Entregable: [`../../ARQUITECTURA.md`](../../ARQUITECTURA.md), referenciado desde
el README en el índice de documentación.

| Tarea del issue | Dónde |
| :--- | :--- |
| Módulos, workers, cola, base, almacenamiento e integraciones, con comunicaciones síncronas y asíncronas | §2 |
| Región, red privada, subredes, VMs, contenedores, base administrada, bucket, **volúmenes** y reglas de acceso | §3, con los volúmenes persistentes en §3.4 |
| Decisiones: ubicación de la cola, **migración al almacenamiento administrado**, carga directa y URLs firmadas, configuración de los workers | §4.1, §4.3, §4.4, §4.5 |
| Diferencias frente a la arquitectura objetivo | §5 |
| Diagramas con sus archivos fuente versionados | Tres bloques Mermaid; el bloque **es** la fuente |
| Alcance funcional construido y lo que quedó fuera | §6 y §5 |

| Criterio de aceptación | |
| :--- | :---: |
| Documento en `docs/entrega2/`, referenciado desde el README | ✅ |
| Archivos fuente de los diagramas en el repositorio | ✅ |

## Dos precisiones que el enunciado pide y no eran obvias

**«Migración al almacenamiento administrado» no fue un traslado de archivos.** La
Entrega 1 no construyó nada de media, así que no había corpus en MinIO que mover.
Consistió en repuntar la aplicación al servicio administrado —el adaptador se
elige en ejecución, y el validador de arranque rechaza `minio` en producción— y
sembrar el bucket con los tres perfiles del escenario 2. La verificación de
integridad que el enunciado exige se hizo sobre esa siembra: 714 objetos,
`reconciliation_ok: true`, en [`../G1/`](../G1/README.md). Está en §4.3 del
documento, dicho con esas palabras, para que un evaluador no busque una migración
que no ocurrió.

**Los volúmenes persistentes son pocos a propósito.** Ninguna VM guarda estado de
la aplicación: el que importa vive en Cloud SQL y en los dos buckets. §3.4 lo
tabula con qué se pierde si cada volumen desaparece, que es la pregunta que
importa de cara a I2.

## El documento describe lo desplegado, no lo planeado

Cada dato de infraestructura se comprobó contra el proyecto real antes de
escribirlo, no se tomó de documentos previos. Tres resultaron estar
desactualizados, y los tres se corrigieron en su origen:

| Afirmación | Cómo se comprobó | Resultado |
| :--- | :--- | :--- |
| IPs privadas de las VMs | `gcloud compute instances list` | **Corregido.** `10.0.1.4` y `10.0.1.5`, no `10.0.1.2`/`10.0.1.3` |
| El Worker Server no tiene IP pública | `gcloud compute instances list`, `compute.tf` | **Falso.** Sí la tiene, estática: `35.237.6.244` |
| Rango de Private Services Access | `gcloud compute addresses list --global` | `10.171.240.0/20`, confirmado |
| IP privada de Cloud SQL | `gcloud sql instances list` | `10.171.240.3`, confirmado |
| Perfil de las VMs | `gcloud compute instances list` | `e2-highcpu-2`, confirmado |
| El bucket de derivados es público | `GET` anónimo a una clave inexistente | `404` — público. El de media responde `403` |
| Número de rutas de la API | `grep` sobre `internal/http/server.go` | 53 |
| Migraciones aplicadas | `ls migrations/*.up.sql` | 7 |

### Correcciones aplicadas a documentos previos

[`../B3/DIAGRAMA_RED.md`](../B3/DIAGRAMA_RED.md) afirmaba que el Worker Server no
tenía dirección pública y salía por Cloud NAT. Ninguna de las dos cosas es cierta
hoy. Se corrigió el diagrama y se añadieron dos notas:

- **Las direcciones privadas no son fijas**: la subred las asigna por DHCP y una
  VM recreada recibe otra. Lo que el diseño fija es la subred y las reglas de
  firewall, que se expresan por etiqueta de red y por rango, nunca por IP.
- **Tener IP pública no es estar expuesto.** Es la decisión de costos de B1: dos
  IPv4 externas cuestan 3,65 USD/mes frente a 6,73 de una IPv4 más Cloud NAT. La
  protección la da `mooc-allow-web-ingress`, que aplica solo a instancias con la
  etiqueta `web-server`.

## Un hallazgo para I2

**El Cloud NAT quedó aprovisionado y no se usa.** B1 recomendó dar al worker su
propia IPv4 externa *en lugar* de Cloud NAT; B3 implementó las dos cosas. Una VM
con dirección externa sale por ella, así que `mooc-nat` no procesa tráfico de
ninguna de las dos máquinas.

O se retira el NAT, o se retiran las IPs externas y el worker sale por él. Tener
ambos paga dos veces por la misma salida. La decisión y su efecto en el consumo
observado corresponden a I2 (#137).

## Lo que este documento no cubre

Operación, recuperación, capacidad, costos y limitaciones son la otra mitad de la
rúbrica de arquitectura y corresponden a **I2 (#137)**. La sección 7 de
`ARQUITECTURA.md` enlaza el material que ya existe para ellas
(`ADMINISTRACION.md`, `infra/terraform/README.md`, evidencia de C2).

Este archivo no contiene credenciales, llaves ni secretos. Las direcciones IP
privadas que aparecen son de una red interna y no son alcanzables desde internet.
