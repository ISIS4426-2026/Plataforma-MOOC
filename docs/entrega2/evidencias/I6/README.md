# Evidencia I6 — Cierre: recreación y eliminación controlada de recursos (issue #141)

Cierra la entrega apagando lo que el enunciado pide apagar, conservando lo que
permite reconstruir el entorno y dejando registrado qué sigue costando.

> **El enunciado lo exige, no es una decisión del equipo.** Sección de gestión de
> costos: *«Después de registrar las evidencias y cargar la entrega, eliminar la
> instancia de base de datos administrada del laboratorio, conservando
> previamente los respaldos o los datos sintéticos y scripts necesarios para
> reconstruirla. Documentar qué recursos se conservan, sus costos y cómo recrear
> el entorno para una sustentación.»*

| Tarea | |
| :--- | :---: |
| Evidencias cargadas antes de tocar nada | ✅ §1 |
| Respaldo final conservado | ✅ §2 |
| Recreación ensayada, con tiempo registrado | ✅ §3 — ensayada en C2, no se repite |
| Instancia de base de datos eliminada | ⏳ §5 |
| Revisión de lo que sigue costando | ✅ §4 |
| Qué se conserva, su costo y cómo recrear | ✅ §4 y §6 |

---

## 1. Antes de tocar nada

| Condición | |
| :--- | :--- |
| Tag `entrega-2` creado y empujado | ✅ |
| El README del commit tagueado enlaza los videos de la Entrega 2 | ✅ |
| Evidencia de los 26 issues en `docs/entrega2/evidencias/` | ✅ |
| Informe de capacidad publicado en `capacity-planning/` | ✅ |

Es la primera tarea del issue y la condición que hace irreversible todo lo
demás: **una vez eliminada la instancia, las evidencias que dependan de
consultarla ya no se pueden rehacer.**

## 2. Lo que se conserva para reconstruir

El enunciado admite conservar **«los respaldos *o* los datos sintéticos y
scripts necesarios para reconstruirla»**. Se cumple por la segunda vía, y está en
el repositorio bajo el tag:

| | Dónde |
| :--- | :--- |
| Esquema completo, 7 migraciones reversibles | [`migrations/`](../../../../migrations/) |
| Datos sintéticos de la plataforma | [`scripts/seeds/synthetic_data.sql`](../../../../scripts/seeds/synthetic_data.sql) |
| Datos de las pruebas de carga | [`scripts/seeds/capacity_data.sql`](../../../../scripts/seeds/capacity_data.sql) |
| Aplicador de migraciones, repetible | [`scripts/migrate.sh`](../../../../scripts/migrate.sh) |
| Reconstrucción completa, de cero a utilizable | [`scripts/recrear-entorno.sh`](../../../../scripts/recrear-entorno.sh) |
| Infraestructura declarada | [`infra/terraform/`](../../../../infra/terraform/README.md) |

> **Las copias de Cloud SQL se borran con la instancia.** Al eliminarla
> desaparecen sus respaldos automáticos y bajo demanda. Por eso lo que se
> conserva son el esquema y las semillas versionadas, no una copia dentro del
> servicio que va a dejar de existir.
>
> Opcionalmente puede tomarse un volcado a Cloud Storage, que sí sobrevive:
>
> ```bash
> gcloud sql export sql mooc-db-1 \
>   gs://plataforma-mooc-entrega2-media/respaldos/moocdb-final.sql.gz \
>   --database=moocdb --project=plataforma-mooc-entrega2
> ```
>
> Requiere conceder escritura en el bucket a la cuenta de servicio de la
> instancia. El contenido es el mismo que reproducen las semillas.

## 3. La recreación, ya ensayada en C2

No se repite el ensayo. **C2 (#119) lo cerró con evidencia** y repetirlo tendría
un costo concreto: Google **retiene el nombre de una instancia borrada durante
siete días**, así que cada ciclo de destrucción y recreación consume una
generación del nombre. Si el equipo docente pide sustentación síncrona dentro de
esa ventana, cada ensayo extra obliga a subir `db_instance_generation` otra vez.

| Lo que C2 demostró | |
| :--- | :--- |
| Restauración sobre una instancia nueva | Contenido idéntico fila por fila |
| Procedimiento ejecutable por cualquier integrante | `scripts/recrear-entorno.sh`, corrido contra la nube |
| Tiempo desde el código | **210 s** |
| Tiempo desde una copia | **528 s** |

Detalle en [`../C2/README.md`](../C2/README.md) y
[`../C2/ensayo_de_restauracion.txt`](../C2/ensayo_de_restauracion.txt).

## 4. Costos: antes y después

Precios de lista `us-east1`, fecha 2026-09-25, tomados de
[`../../CONFIGURACION_Y_COSTOS.md`](../../CONFIGURACION_Y_COSTOS.md) §4.

| Escenario | USD/mes |
| :--- | ---: |
| Todo encendido 24×7 — el estado durante la entrega | **133,73** |
| Solo eliminando Cloud SQL | 82,72 |
| **Eliminando Cloud SQL y apagando las dos VMs** ← lo aplicado | **≈ 20,67** |
| Además liberando las dos IPv4 | ≈ 6,08 |

### Qué se deja de pagar

| Concepto | USD/mes |
| :--- | ---: |
| Cloud SQL — instancia de 1 vCPU / 3,75 GiB | −49,31 |
| Cloud SQL — almacenamiento SSD de 10 GiB | −1,70 |
| Cómputo de las dos VMs `e2-highcpu-2` | −72,22 |
| Egreso a internet — sin tráfico con las VMs apagadas | −0,76 |
| Sobrecosto de las IPv4 al quedar reservadas sin usar | **+10,94** |
| **Ahorro neto** | **−113,06** |

La suma cuadra: 133,73 − 113,06 = **20,67**. La línea que sube es la de las
direcciones IP, explicada abajo.

### Qué sigue costando, y por qué se conserva

| Concepto | USD/mes | Por qué no se elimina |
| :--- | ---: | :--- |
| Discos de arranque, 30 GiB × 2 | 6,00 | Un disco se paga aunque la VM esté apagada. Borrarlos obligaría a reconstruir las VMs enteras, no solo a encenderlas |
| 2 direcciones IPv4 estáticas | 14,59 | **Con las VMs apagadas cuestan más que encendidas** (3,65). Se conservan porque liberarlas cambiaría la URL, y el README del commit tagueado ya la publica |
| Buckets de Cloud Storage | 0,08 | Contienen los originales, los derivados HLS y el estado de Terraform |
| Artifact Registry, Secret Manager | marginal | Las imágenes del tag y los secretos que la recreación necesita |
| **Total residual** | **≈ 20,67** | |

### Por qué apagar las VMs y no solo borrar la base

El enunciado solo exige eliminar la instancia de base de datos, pero advierte en
la misma sección que *«detener una instancia no implica eliminar todos sus
costos; deben revisarse también almacenamiento, respaldos y recursos de red
asociados»*. Hacer únicamente lo exigido dejaría **82,72 USD/mes** corriendo, y
el cómputo de las dos VMs es la mayor parte de eso.

La restricción que lo vuelve urgente está en
[`CONFIGURACION_Y_COSTOS.md`](../../CONFIGURACION_Y_COSTOS.md) §4: **el techo
operativo es 50 USD, no 200**, porque los cuatro cupones se redimen en secuencia.
Con el entorno completo encendido, el crédito activo dura **once días**; con lo
que queda tras este cierre, más de dos meses.

**La trampa de las IPv4 conviene leerla dos veces:** una dirección estática
reservada cuesta **más apagada que encendida**, porque Google cobra las
direcciones sin usar. Es la única línea que sube al apagar las máquinas, y aun
así se conservan: cambiar la URL rompería el enlace publicado en el commit
evaluado.

## 5. Procedimiento de eliminación

La instancia tiene protección contra borrado, así que **`terraform destroy` a
secas falla** con `cannot destroy instance because deletion_protection is set to
true`. Hay que quitarla en el código primero, y el `apply` va desde `main`.

```bash
# 1. En infra/terraform/database.tf: deletion_protection = false
#    (commit y merge a main antes de aplicar)

cd infra/terraform
export TF_VAR_db_password="$(gcloud secrets versions access latest \
  --secret=db-password --project=plataforma-mooc-entrega2 | tr -d '\r\n')"

terraform apply                                    # quita la protección, no borra nada
terraform destroy -target=google_sql_database_instance.main   # ahora sí

# 2. Apagar las dos VMs, conservando sus discos y sus IPs
gcloud compute instances stop mooc-web-server mooc-worker-server \
  --zone=us-east1-b --project=plataforma-mooc-entrega2
```

> **Leer el plan antes de confirmar.** Un `destroy` sin `-target` se llevaría
> también las VMs, los buckets y la red. Lo que este cierre elimina es **solo la
> instancia de base de datos**.

## 6. Cómo recrear el entorno para una sustentación

```bash
cd infra/terraform
export TF_VAR_db_password="$(gcloud secrets versions access latest \
  --secret=db-password --project=plataforma-mooc-entrega2 | tr -d '\r\n')"

# El nombre anterior queda reservado siete días: hay que subir la generación.
terraform apply -var='db_instance_generation=2'

gcloud compute instances start mooc-web-server mooc-worker-server \
  --zone=us-east1-b --project=plataforma-mooc-entrega2
```

Después, el esquema y los datos con
[`scripts/recrear-entorno.sh`](../../../../scripts/recrear-entorno.sh), que
ejecuta las tres fases —aprovisionar, migrar y sembrar— y que C2 midió en
**210 s** desde el código.

**Dos cosas cambian y hay que comprobarlas al encender:**

1. **Las IPs privadas de las VMs no son fijas.** La subred las asigna por DHCP y
   una máquina recreada recibe otra. `QUEUE_PRIVATE_IP` en `/etc/mooc/web.conf`
   y `DB_PRIVATE_IP` en ambos archivos se leen con `terraform output`, nunca se
   copian. Al arrancar, `grep '^REDIS_URL=' ~/mooc/.env` debe apuntar al Worker
   Server.
2. **La base nace vacía.** Terraform crea la instancia y el usuario, no el
   esquema.

La comprobación final, de punta a punta:

```bash
make test-e2e-cloud     # 61 pasos; el recurso de video debe llegar a "completed"
```

Procedimiento detallado en
[`../../OPERACION_Y_CAPACIDAD.md`](../../OPERACION_Y_CAPACIDAD.md) y en
[`ADMINISTRACION.md`](../../../../infra/terraform/ADMINISTRACION.md).

## 7. Registro de la ejecución

| | |
| :--- | :--- |
| Fecha de eliminación | *(pendiente)* |
| Generación de la instancia eliminada | `mooc-db-1` |
| Copias existentes al eliminar | 3 — una automática y dos bajo demanda, borradas con la instancia |
| VMs apagadas | *(pendiente)* |
| Costo residual verificado | *(pendiente)* |

---

Este archivo no contiene credenciales, llaves ni secretos.
