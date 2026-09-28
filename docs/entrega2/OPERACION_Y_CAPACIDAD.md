# Operación, recuperación, capacidad, costos y limitaciones — Entrega 2

Issue **I2** (#137). Es el documento que se abre cuando hay que **levantar, mantener, recuperar o evaluar** el entorno de la Plataforma MOOC en GCP, y que el enunciado pide en `docs/entrega2`.

**Para quién:** un integrante que **no participó en el despliegue** y necesita reconstruir el entorno o responder qué pasa si algo se cae. No supone haber leído nada más: cada paso dice qué ejecutar y cómo saber que salió bien. Donde un procedimiento ya tiene su propio detalle (Terraform, worker, correo), aquí está el orden y los comandos esenciales y el enlace al detalle.

**Alcance de las cifras:** todo número de este documento indica su fecha y de dónde sale. Lo que **no se pudo medir** figura como pendiente, con quién puede cerrarlo; no se rellena con estimaciones disfrazadas de mediciones.

| Parte | Qué contiene |
| :--- | :--- |
| [1. El entorno de un vistazo](#1-el-entorno-de-un-vistazo) | Qué existe, con qué nombre y dónde vive cada configuración |
| [2. Operación y recuperación](#2-operación-y-recuperación) | Reconstruir, desplegar, migrar, verificar, reiniciar, respaldar, cerrar |
| [3. Capacidad, costo y limitaciones](#3-capacidad-costo-y-limitaciones) | Configuración exacta, estimación frente a consumo, puntos únicos de falla, evolución, límites del laboratorio |
| [4. Cómo comprobar este documento](#4-cómo-comprobar-este-documento) | La prueba de que alguien ajeno puede seguirlo |

Documentos relacionados: [`ARQUITECTURA.md`](./ARQUITECTURA.md) (qué es cada pieza y por qué), [`CONFIGURACION_Y_COSTOS.md`](./CONFIGURACION_Y_COSTOS.md) (estimación B1, presupuesto y política de encendido), [`infra/terraform/README.md`](../../infra/terraform/README.md) y [`ADMINISTRACION.md`](../../infra/terraform/ADMINISTRACION.md) (Terraform, accesos, secretos, worker, correo), [`NOTAS_TECNICAS.md`](./NOTAS_TECNICAS.md) (hallazgos que cuestan una tarde).

---

## 1. El entorno de un vistazo

Proyecto `plataforma-mooc-entrega2`, región `us-east1`, zona `us-east1-b`. Una instancia por componente, sin autoescalado ni réplicas.

| Recurso | Nombre | Detalle |
| :--- | :--- | :--- |
| VM web | `mooc-web-server` | `e2-highcpu-2` (2 vCPU, 2 GiB), disco 30 GiB balanced, IP estática pública. Contenedores `nginx`, `api` y un `redis` **sin uso** (residuo, ver §3.4). Arranque: `mooc-web.service` |
| VM worker | `mooc-worker-server` | `e2-highcpu-2`, disco 30 GiB, IP estática pública **sin regla de ingreso**. Contenedores `redis` (la cola, y también sesiones, límites e idempotencia) y `worker`. Arranque: `mooc-worker.service` |
| Base de datos | `mooc-db-1` | Cloud SQL PostgreSQL 16, `db-custom-1-3840` (1 vCPU dedicada, 3,75 GiB), 10 GiB SSD, **zona única**, solo IP privada, copias diarias a las 03:00 |
| Objetos | `…-media` · `…-hls` · `…-tfstate` | `media` privado (originales, documentos, miniaturas), `hls` de lectura pública (derivados de video), `tfstate` con el estado de Terraform |
| Imágenes | Artifact Registry `mooc` (`us-east1`) | `api` y `worker`, un tag por commit (inmutable) |
| Secretos | Secret Manager | `db-password`, `smtp-password` |
| Red | `mooc-vpc` | Subred `10.0.1.0/24`; firewall: 80/443 al Web, SSH solo por IAP, tráfico interno; Cloud NAT `mooc-nat` |
| Cuentas de servicio | `sa-web-server`, `sa-worker-server` | Sin archivos de llave; permisos mínimos por componente |

**Entrada de usuarios:** `https://34.24.52.111.sslip.io`. El dominio es la IP estática dentro de un nombre de `sslip.io`; el certificado es de Let's Encrypt (ver evidencia D3).

### Dónde vive cada configuración

| Qué | Dónde | En Git |
| :--- | :--- | :---: |
| Recursos de nube (VMs, red, base, buckets, IAM) | `infra/terraform/*.tf`, estado en `gs://…-tfstate` | sí (el estado no) |
| Configuración operativa de cada VM (tag de imagen, IPs privadas, remitente) | `/etc/mooc/web.conf` y `/etc/mooc/worker.conf`, en la VM, modo 640 | **no** |
| Contraseña de la base | Secret Manager `db-password` | **no** |
| Contraseña SMTP | Secret Manager `smtp-password` | **no** |
| `.env` de cada VM | Lo **genera** `scripts/prepare_web_env.sh` / `prepare_worker_env.sh` al arrancar, modo 600 | **no** |
| Definición de contenedores | `docker-compose.prod.yml` (web), `docker-compose.worker.yml` (worker) | sí |
| Ejemplo de variables sin valores secretos | `.env.example` | sí |

**Manejo de secretos, en corto:** no existen archivos de llave (ni de Terraform ni de las VMs: cada persona usa su identidad y cada VM su cuenta de servicio adjunta); las dos contraseñas viven solo en Secret Manager y las VMs las leen al arrancar; `.env.example` lleva los secretos vacíos y el arranque en producción rechaza los valores de desarrollo. Inventario completo y rotación en [`ADMINISTRACION.md`](../../infra/terraform/ADMINISTRACION.md#inventario-de-configuración-sensible). La verificación de que no hay secretos en el repositorio ni en las evidencias está en [`evidencias/G4`](./evidencias/G4/README.md).

---

## 2. Operación y recuperación

### 2.1 Antes de empezar: accesos y herramientas

Cada persona necesita, una sola vez:

1. **Herramientas:** Terraform 1.16.x, Google Cloud CLI y Docker. Instalación por sistema operativo en [`infra/terraform/README.md`](../../infra/terraform/README.md#puesta-a-punto-una-vez-por-persona).
2. **Identidad propia** (sin archivos de llave):
   ```bash
   gcloud auth login
   gcloud auth application-default login
   gcloud config set project plataforma-mooc-entrega2
   ```
3. **Roles en el proyecto:** para aplicar Terraform, los que da [`ADMINISTRACION.md`](../../infra/terraform/ADMINISTRACION.md#puesta-a-punto-del-equipo-una-sola-vez); para entrar a una VM, además `roles/iap.tunnelResourceAccessor` (sin él, el SSH por IAP falla con «Remote side unexpectedly closed»). Hoy solo **2 personas** tienen este último rol (ver §3.4).
4. **Contraseña de la base para Terraform**, leída de Secret Manager (nunca copiada a mano):
   ```bash
   export TF_VAR_db_password="$(gcloud secrets versions access latest \
     --secret=db-password --project=plataforma-mooc-entrega2 | tr -d '\r\n')"
   ```
   Hay que repetirlo en cada terminal nueva.

**Entrar a una VM** (sin IP pública de SSH: solo por el túnel de IAP):

```bash
gcloud compute ssh mooc-web-server --zone=us-east1-b --tunnel-through-iap --project=plataforma-mooc-entrega2
```

Si la ventana interactiva no deja escribir o pegar, los comandos se pueden lanzar sin sesión con `--command='…'` y los archivos copiar con `gcloud compute scp … --tunnel-through-iap` (así lo hace `scripts/cargar_cuentas_carga_nube.sh`).

### 2.2 Reconstruir el entorno desde cero

Orden obligatorio. Cada paso termina con una comprobación; **no se pasa al siguiente si no da lo esperado**.

| # | Paso | Cómo | Se sabe que salió bien cuando |
| :-: | :--- | :--- | :--- |
| 1 | **Infraestructura** | Desde `main` actualizado: `cd infra/terraform && terraform init && terraform plan && terraform apply` | El `plan` final no propone destruir nada inesperado (línea `Plan: … to destroy`). Aplicar **siempre desde `main`**: un `apply` desde una rama borra el trabajo de otro y no falla al hacerlo (nota 16) |
| 2 | **Imágenes** | Desde un árbol Git limpio: `./scripts/publish_images.sh` | Publica `api` y `worker` con el SHA completo de `HEAD` como tag. Anotar ese SHA |
| 3 | **Esquema de la base** | Desde **dentro** de la VPC (la base no tiene IP pública), ver §2.4 | `migrate.sh --verify` sin diferencias |
| 4 | **Datos sintéticos** | Misma restricción que el paso 3: `scripts/seed.sh` con `DATABASE_URL` definida | `seed.sh --status` muestra los conteos esperados |
| 5 | **Configuración del Worker Server** | Crear `/etc/mooc/worker.conf`, copiar `prepare_worker_env.sh` y el compose, crear `mooc-worker.service`. **Se hace primero** porque la API depende de la cola que vive aquí | `sudo docker ps` muestra `mooc-queue` y `mooc-worker` en `Up` (no `Restarting`). Detalle exacto en [`ADMINISTRACION.md`](../../infra/terraform/ADMINISTRACION.md#worker-server-despliegue-y-reconstrucción) |
| 6 | **Configuración del Web Server** | Crear `/etc/mooc/web.conf` (con la IP privada del worker como `QUEUE_PRIVATE_IP`), `./prepare_env.sh`, `sudo systemctl restart mooc-web.service`. Detalle en [`ADMINISTRACION.md`](../../infra/terraform/ADMINISTRACION.md#4-configurar-la-vm-sin-datos-personales-en-git) | `sudo systemctl status mooc-web.service` activo y `curl -s https://<dominio>/api/v1/health` responde `200` |
| 7 | **Verificación de punta a punta** | `make test-e2e-cloud` | El recorrido llega hasta `processing_status="completed"` (el video de prueba tarda ~10 s). Es lo único que demuestra la cadena API → cola → worker → bucket |
| 8 | **Verificación de red y seguridad** | `bash scripts/verify_g4.sh` (solo lectura) | Los seis archivos de `evidencias/G4` sin hallazgos críticos |

**Las direcciones privadas se leen de Terraform, no se copian de un documento:** una VM recreada recibe otra IP (ya pasó: de `10.0.1.2/3` a `10.0.1.4/5`).

```bash
cd infra/terraform
terraform output -raw db_private_ip
terraform output -raw worker_server_private_ip
terraform output -raw storage_hls_bucket
```

**El error que no falla:** si en `web.conf` falta `QUEUE_PRIVATE_IP`, `prepare_web_env.sh` detiene el despliegue a propósito. Apuntar la API al `redis` de su propio compose hace que las subidas respondan `202` y el recurso se quede en `pending` para siempre, sin ningún error (defecto #166, nota 19).

**Tiempos medidos (C2, 2026-09-27):** aprovisionar una instancia de base nueva, 204 s; migrar y sembrar, 6 s; recrear la base desde el código, **210 s**; desde una copia, 528 s. El resto del entorno (VMs) se levanta con el mismo `terraform apply`.

### 2.3 Desplegar una versión nueva

1. Integrar en `main` y publicar las imágenes: `./scripts/publish_images.sh` (anota el SHA).
2. En **cada** VM, con el **mismo** `IMAGE_TAG` (el worker y la API comparten esquema y convenciones de claves: mezclar commits produce fallos que no se parecen a su causa): editar `IMAGE_TAG` en `/etc/mooc/web.conf` o `/etc/mooc/worker.conf`.
3. Regenerar el `.env` y reiniciar:
   ```bash
   ./prepare_env.sh
   sudo systemctl restart mooc-web.service      # o mooc-worker.service en el worker
   ```
4. Si la versión trae migraciones nuevas: aplicarlas **antes** de reiniciar la API (§2.4).
5. Comprobar (§2.5).

**No usar `latest`.** El tag es el SHA completo: es lo que permite saber qué corre y volver atrás.

### 2.4 Migraciones

La instancia nace con `moocdb` y **sin una tabla**: Cloud SQL no tiene el hook que aplica el esquema en local. `scripts/migrate.sh` lo hace y **solo puede ejecutarse dentro de la VPC** (desde una de las VMs):

```bash
# en la VM
sudo apt-get install -y postgresql-client
export DATABASE_URL="postgres://moocuser:$(gcloud secrets versions access latest --secret=db-password | tr -d '\r\n')@<db_private_ip>:5432/moocdb?sslmode=require"
bash ./scripts/migrate.sh            # aplica lo que falte
bash ./scripts/migrate.sh --verify   # comprueba el esquema
```

Es repetible (una tabla `schema_migrations` registra lo aplicado) y cada migración va en una transacción con su registro. Alternativa sin instalar nada en las VMs: `scripts/cargar_cuentas_carga_nube.sh <archivo.sql>` copia un SQL a la VM web y lo aplica con un cliente `psql` en contenedor, leyendo la contraseña de Secret Manager **dentro** de la VM (así se cargaron y reinician las cuentas de carga de H2/H3).

### 2.5 Verificación (¿está sano el entorno?)

| Comprobación | Comando | Esperado |
| :--- | :--- | :--- |
| API viva | `curl -s -o /dev/null -w "%{http_code}" https://<dominio>/api/v1/health` | `200` |
| HTTP redirige a HTTPS | `curl -sI http://<dominio>/` | `301` a `https://` |
| Contenedores | `sudo docker ps` en cada VM | Todos `Up`; el worker no en `Restarting` |
| Cadena completa | `make test-e2e-cloud` | Llega a `completed` |
| Red y seguridad | `bash scripts/verify_g4.sh` | Solo 80/443 abiertos en el Web; el worker sin puertos; sin llaves de cuenta de servicio |
| Funcional | `make test-postman-cloud` | Las colecciones pasan (paso pausado por el límite de login, ver `evidencias/G2`) |
| Cuentas de carga (solo si se va a medir capacidad) | `bash scripts/h2_nube.sh tokens` | Sesiones guardadas sin fallos |

### 2.6 Reinicio

| Situación | Acción |
| :--- | :--- |
| Reiniciar la API | `sudo systemctl restart mooc-web.service` (en el Web Server) |
| Reiniciar el worker y la cola | `sudo systemctl restart mooc-worker.service` (en el Worker Server) |
| Reinicio de una VM | Los dos servicios de systemd están habilitados: los contenedores vuelven solos (probado en D2, `supervivencia_reinicio.txt`) |
| Orden si se reinician **las dos** VMs | Primero el worker (la cola y las sesiones viven ahí), después la API |
| Se reinicia el Worker Server | La API pierde la cola y el almacén de sesiones mientras dura: todas las sesiones dependen de ese Redis (§3.4) |

### 2.7 Respaldo y reconstrucción de la base

Procedimiento y mediciones completas en [`evidencias/C2`](./evidencias/C2/README.md) y en [`ADMINISTRACION.md`](../../infra/terraform/ADMINISTRACION.md#cierre-de-la-entrega-issue-133-i6).

- **Copias automáticas** diarias a las 03:00 UTC, con la instancia encendida (las últimas copias listadas, del 2026-09-27, figuran `SUCCESSFUL`). Con la instancia **detenida no se ejecutan**.
- **Copia manual:** `gcloud sql backups create --instance=mooc-db-1`. **Exportación** a un bucket (la que sobrevive a borrar la instancia): `gcloud sql export sql mooc-db-1 gs://<bucket>/respaldo.sql --database=moocdb`.
- **El respaldo de este proyecto es el repositorio:** siete migraciones y una semilla determinista reconstruyen la base en 6 s. Lo único que no está en el repositorio son los datos que deja una corrida de capacidad: **exportarlos antes de detener o borrar la base**.
- **Recrear desde el código** (camino por defecto): `bash ./scripts/recrear-entorno.sh` orquesta aprovisionar → migrar → sembrar (crea una VM temporal en la VPC y la borra al terminar). **Restaurar una copia** es 2,5 veces más lento y solo aporta lo que no se puede regenerar.
- **Los buckets `media` y `hls`** contienen lo que el pipeline produjo; los derivados HLS se regeneran con el worker desde los originales.

### 2.8 Cierre de la entrega y recreación para sustentar

El enunciado exige **eliminar la instancia de base administrada** al cerrar y poder recrearla. Orden (los tres pasos no se pueden juntar):

1. **Conservar** lo necesario: exportar a un bucket (copia y datos de las corridas).
2. **Quitar la protección** en `database.tf` (`deletion_protection = false`) y `terraform apply`.
3. **`terraform destroy`** (o solo el recurso de la base, según se decida).

Al recrear: el nombre de una instancia borrada queda **reservado siete días** (subir `db_instance_generation`: `terraform apply -var='db_instance_generation=2'`) y la instancia nueva viene **vacía** (volver a migrar y sembrar, §2.4). Lo que sobrevive y su costo (≈ 0,12 USD/mes) está medido en `ADMINISTRACION.md`. Ciclo completo eliminar + recrear con datos: unos 6 minutos medidos, 10 con margen.

### 2.9 Problemas conocidos, en una tabla

| Síntoma | Causa | Solución |
| :--- | :--- | :--- |
| Subidas responden `202` y el recurso queda `pending` | `REDIS_URL` apunta a un Redis sin worker (falta `QUEUE_PRIVATE_IP`) | Corregir `web.conf` (nota 19) |
| El worker muere con `permission denied` en el trabajo de video | Espacio de trabajo montado como carpeta del host con un contenedor sin privilegios | Volumen con nombre (nota 21; ya en el compose) |
| El worker no arranca pidiendo `smtp-password` | Validaba la configuración de la API | Corregir el validador, no los permisos (nota 20) |
| `terraform plan` propone destruir recursos | Rama desactualizada o `apply` desde una rama | `git pull` desde `main` y releer el plan (nota 16) |
| La base rechaza la contraseña | El secreto trae `CR`/espacio de un `gcloud` de Windows | `tr -d '\r\n'` o `.strip()`, como hacen los scripts (nota 18) |
| `429` en el login | Límite de 10 intentos por minuto por IP | Esperar; para carga usar tokens preemitidos (H2) |
| `Remote side unexpectedly closed` al hacer SSH | Falta `iap.tunnelResourceAccessor` | Pedir el rol |
| Correo no llega | El puerto 25 saliente está bloqueado por GCP | SMTP por 587/465/2525 con Brevo (nota 14, 17) |

---

## 3. Capacidad, costo y limitaciones

### 3.1 Configuración exacta durante las pruebas

Idéntica en todas las corridas de capacidad y **sin cambios durante ellas**:

| Componente | Configuración |
| :--- | :--- |
| Web Server | `e2-highcpu-2`, 2 vCPU, 2 GiB, 30 GiB balanced; nginx 1.27 + API Go 1.24; `DB_MAX_OPEN_CONNS=25` |
| Worker Server | `e2-highcpu-2`; `WORKER_CONCURRENCY=2`; Redis 7 (cola asynq, sesiones 24 h, límites, idempotencia) |
| Base | Cloud SQL PostgreSQL 16, `db-custom-1-3840`, 10 GiB SSD, zona única; conexiones comprometidas 58 (API 25 + worker 25 + margen) |
| Objetos | Standard, `us-east1` |
| Región / zona | `us-east1` / `us-east1-b` |
| Generador de carga | JMeter 5.6.3 en Docker, **fuera de la VPC** (portátil de un integrante), ver H1 |

### 3.2 Estimación frente a consumo observado

**Estimación (B1, fechada 2026-09-25):** operación 24×7 = 730 h por recurso, precios de lista `us-east1`: **133,73 USD/mes** (Web 39,11 + Worker 39,11 + Cloud SQL 49,31 + disco SQL 1,70 + Storage 0,08 + IPs 3,65 + egreso 0,76). Un cupón de 50 USD paga ≈ 11 días de operación continua; por eso rige la política de encendido y apagado ([`CONFIGURACION_Y_COSTOS.md`](./CONFIGURACION_Y_COSTOS.md#política-de-encendido-y-apagado)).

**Consumo observado** (medido el 2026-09-28 con `gcloud` y Cloud Monitoring, solo lectura; detalle en [`evidencias/I2`](./evidencias/I2/consumo_observado.md)):

| Concepto | Supuesto de B1 | Observado | Lectura |
| :--- | :--- | :--- | :--- |
| Horas de VM | 730 h × 2 = 1 460 h/mes | ≈ 13 h (web) + 9 h (worker) en las instancias actuales, más 2 VM temporales de 0,4 h y 0,1 h: **≈ 23 h** desde el 15-sep | ≈ 1,6 % de la operación 24×7: el consumo lo gobierna el apagado, no el mes completo. A precio de lista, ≈ 1,2 USD de cómputo |
| Objetos almacenados | 3 GiB | `media` 5,0 MB (40 objetos), `hls` 3,4 MB (14), `tfstate` 0,1 MB: **≈ 8,5 MB** | Muy por debajo: en la nube **no se sembraron los tres perfiles multimedia de G1** (la corrida de 765 MB fue local) |
| Egreso a internet | 5 GiB | Bytes servidos por los buckets: ≈ 4,6 MB; bytes enviados por las VMs (incluye tráfico interno, cota superior): ≈ 0,6 GB | Órdenes de magnitud por debajo de lo estimado. Las pruebas de carga solo mueven JSON (~1 KB por petición); **no se reprodujeron videos completos** (nota 13) |
| Disco de Cloud SQL | 10 GiB aprovisionados | 79 MB usados | Sin presión de almacenamiento |
| Artifact Registry | 106 MB al medir C2 | **415,5 MB** | **Cerca del medio gigabyte gratuito**: hay un tag inmutable por commit y se acumulan. Limpiar tags viejos antes de superar 500 MB |
| Cloud SQL (horas) | 730 h | Facturado 1,13 USD en 15–27 sept.; a 0,070 USD/h equivale a ≈ 16 h de instancia (derivado del costo) | Consistente con el encendido intermitente |

**Contraste con la factura real** (informe de Facturación, 15–27 de septiembre de 2026, costo bruto antes de créditos): **1,54 USD** en total (Cloud SQL 1,13; Compute Engine 0,35; Networking 0,06; Artifact Registry 0,00), cubiertos íntegramente por el cupón: **0,00 USD de gasto real**. Frente a los 133,73 USD/mes de la estimación 24×7 es ≈ 1,2 %: la estimación es un techo de operación continua y la política de encendido y apagado la mantiene muy por debajo. **Límite de la lectura:** la facturación llega con retraso de 1–2 días y solo cubre hasta el 27 de septiembre, cuando se crearon las VMs actuales; el costo de cómputo definitivo será mayor a los 0,35 USD mostrados. Detalle y salvedades en [`evidencias/I2/consumo_observado.md`](./evidencias/I2/consumo_observado.md); hay que **releer el informe pasados 2–3 días** y anotar la cifra final. **El presupuesto está activo** (verificado en Facturación → Budgets & caps el 2026-09-27): «Entrega 2 - cupón activo», importe especificado de **50 USD**, período desde el 26-sep-2026 sin fecha de fin, con **alertas al 25 %, 50 %, 90 % y 100 %** (B1 lo diseñó con 80 % en lugar de 90 %; es una diferencia menor) y sin contar créditos, de modo que mide el consumo bruto (1,53 de 50 USD al momento de verlo). Recuérdese que **notifica, no corta**.

**Utilización durante las pruebas de capacidad del Escenario 1** (H3, 2026-09-27; hasta 200 usuarios concurrentes ≈ 32 peticiones/s, tres corridas en el nivel máximo): CPU del Web Server 11–19 % de media y 15–34 % de máxima, memoria ≈ 31 %; CPU de Cloud SQL 18–20 % de media y 26–27 % de máxima, ≈ 350 transacciones/s; 15, 27 y 17 conexiones abiertas máximas en la base (tope práctico del pool de la API ≈ 29). Datos y análisis en [`evidencias/H3`](./evidencias/H3/README.md); Escenario 2 en [`evidencias/H5`](./evidencias/H5/README.md); informe consolidado en `capacity-planning/pruebas_de_carga_entrega2.md` (I3).

### 3.3 Qué límite se observó

Las conclusiones de capacidad están en H3 (Escenario 1) y H5 (Escenario 2). Para operar hay que recordar cuatro cosas:

- **En el Escenario 1 no se alcanzó saturación** hasta 200 usuarios concurrentes con la pausa nominal: 0 errores reales, 0 timeouts y la integridad de intentos, calificación y progreso confirmada en Cloud SQL tras cada corrida. **200 usuarios / 32,8 peticiones/s es el máximo probado, no la capacidad máxima.** La serie de presión que buscaría el punto real de degradación no se ejecutó (H3, §6).
- **La cola de latencia del Escenario 1 viene de abrir conexiones nuevas**, no del procesamiento: sobre conexiones reutilizadas el p95 es ≈ 117 ms; las peticiones que conectan tienen un tiempo de conexión p95 de ≈ 1,3 s (H3, §3). El origen (red del generador o cola de conexiones del servidor) está sin confirmar y requiere leer contadores de descartes de SYN en la VM web.
- **Recurso con menos margen medido:** la CPU de Cloud SQL (máx. 27 %) y el pool de conexiones de la API (rozó 27 de ≈ 29). Ninguno se agotó; cualquier proyección a mayor carga es una extrapolación, no una medición.
- **Según H5, en el Escenario 2 el límite lo pone la CPU del worker** (2 transcodificaciones simultáneas, una por vCPU).

El inicio de sesión es la operación más cara por diseño (cifrado de la contraseña): 20 inicios simultáneos tardaron ≈ 1,7 s cada uno.

### 3.4 Puntos únicos de falla

Todo componente es único y está en una sola zona. Ordenados por lo que se pierde:

| # | Punto | Qué pasa si falla | Detección | Mitigación posible |
| :-: | :--- | :--- | :--- | :--- |
| 1 | **Zona `us-east1-b`** | Todo el entorno se cae a la vez | Health check externo | Multi-zona (§3.5) |
| 2 | **Redis del Worker Server** | Sin cola **y sin sesiones**: todos los usuarios quedan sin sesión y la API no encola trabajos. Sus datos están en un volumen, pero no hay réplica | `up`/`docker ps` en el worker; error de la API al encolar | Redis administrado (Memorystore) o réplica; separar sesiones de cola |
| 3 | **`mooc-web-server`** (nginx + API) | Sin servicio para los usuarios | Health check `/api/v1/health` | Grupo de instancias + balanceador |
| 4 | **Cloud SQL, zona única, sin réplica** | Sin lectura ni escritura; la recuperación es recrear (210 s) o restaurar (528 s) | Métricas de Cloud SQL, `up` | Alta disponibilidad de Cloud SQL (regional) |
| 5 | **IP estática + `sslip.io`** | Si se libera la IP, cambia el dominio y hay que reemitir el certificado | Expiración de certificado | Dominio propio y balanceador con certificado administrado |
| 6 | **Certificado de Let's Encrypt**, renovado por `certbot.timer` en el Web Server (D3) | Si la renovación falla, el certificado vence y el HTTPS se rompe | Fecha de expiración; `certbot renew --dry-run` | Certificado administrado |
| 7 | **Un solo proveedor de correo (Brevo)** | Sin verificación de cuenta ni recuperación de contraseña | Errores SMTP en la API | Segundo proveedor o cola de correo con reintentos |
| 8 | **Persona:** solo 2 integrantes con acceso SSH (IAP) | Si no están, nadie puede operar las VMs | — | Ampliar el rol (decisión del equipo) |
| 9 | **Estado de Terraform** en un solo bucket | Perder el estado = perder el mapa de lo desplegado | El bucket existe fuera de Terraform a propósito | Versionado del bucket (verificar que está activo) |

Además, **`redis` del Web Server es un residuo** del despliegue en una sola VM: hoy no se usa (`REDIS_URL` apunta al worker). Puede quitarse del compose; no afecta a la operación.

### 3.5 Cambios para evolucionar hacia una aplicación elástica

Cada cambio va con la medición que lo justifica o, si aún no existe, la que habría que tomar.

| Cambio | Qué problema resuelve | Medición que lo respalda |
| :--- | :--- | :--- |
| **API sin estado detrás de un balanceador con autoescalado** | Punto único #3 y techo de CPU del Web | En el Escenario 1 el Web usó 12–34 % de CPU a 33 peticiones/s (H3): hay margen hoy; la medición que faltaría es la que sature (serie de presión). Ya cumple el requisito: la sesión no vive en la VM sino en el Redis del worker, y los archivos van directo al bucket |
| **Sesiones y cola en Redis administrado o réplica** | Punto único #2 | El worker es hoy quien aloja a la vez cola y sesiones (E1) |
| **Workers como grupo autoescalado por profundidad de cola** | Techo del Escenario 2: 1 transcodificación por vCPU | Según el análisis de H5, la CPU del worker es el límite (esa conclusión es de H5, no se re-midió aquí); la métrica de escalado ya existe (`worker.queue.pending`, `oldest_pending_age_seconds`, H1) |
| **Cloud SQL con alta disponibilidad y réplica de lectura** | Punto único #4; el 64 % del tráfico del Escenario 1 son lecturas | La base usó 20 % de CPU de media (27 % de máxima) a 33 peticiones/s (H3); la réplica se justifica cuando la CPU de la base o el pool (25) se acerquen al límite |
| **Pooler de conexiones** (p. ej. PgBouncer) | El pool de la API es fijo por proceso (25) y las conexiones comprometidas suman 58 (nota 11): escalar horizontalmente las multiplica | Conexiones abiertas observadas a 200 usuarios: 15, 27 y 17 (H3); el pool de 25 rozó su tope una vez |
| **CDN para los derivados HLS** | Egreso facturado (nota 13) y latencia | Excluido por el enunciado en esta etapa |
| **Dominio propio y certificado administrado** | Puntos únicos #5 y #6 | — |
| **Observabilidad de aplicación completa** | Hoy la API no exporta sus métricas a Cloud Monitoring hasta redesplegar el compose de H1 | `up=0` para la API en Cloud Monitoring |

### 3.6 Limitaciones del laboratorio que afectaron el trabajo

- **Crédito acotado** (cupones de 50 USD redimidos en secuencia): obliga a apagar y encender y a no hacer corridas largas. Un presupuesto **notifica, no corta**.
- **Una sola IP y un solo portátil como generador de carga:** el login (10/min por IP) tuvo que salir del recorrido medido; el generador mide con ~600 MiB libres y ~5,8 Mbps de bajada.
- **Sin acceso SSH automatizado a producción para agentes:** la verificación se hizo desde fuera y con la configuración de GCP; por dentro solo la puede hacer una persona con IAP.
- **Puerto 25 saliente bloqueado** y correo por un proveedor externo con cuenta gratuita.
- **Windows como estación de trabajo:** compilar bajo OneDrive falla, `gcloud` en Windows devuelve CRLF que rompía contraseñas, la ventana SSH de PuTTY no siempre permite pegar.
- **Cloud SQL se recrea, no se «apaga» gratis:** el nombre reservado siete días y la base vacía al recrear.
- **Facturación y presupuestos** dependen de permisos de la cuenta de facturación que no todos tienen.
- **Métricas de la API pendientes** del redespliegue de H1.
- **No se sembraron en la nube los tres perfiles multimedia de G1** (764 MB): sus números de consumo de objetos son de un ambiente local.

---

## 4. Cómo comprobar este documento

El criterio de aceptación pide que **alguien que no desplegó pueda reconstruir el entorno siguiéndolo**. La comprobación es esta, y debe hacerla una persona distinta de quien escribió el documento:

1. Con las herramientas y los accesos de la §2.1, ejecutar la §2.2 paso a paso en un entorno desechable (o solo el ensayo `scripts/recrear-entorno.sh`, que ya se probó de extremo a extremo).
2. Anotar cada punto donde el documento no alcanzó (un comando faltante, una comprobación ambigua).
3. Corregir el documento con esas notas y guardar el registro en `evidencias/I2`.

**Estado:** la redacción está completa. **La prueba con una persona ajena está pendiente.**
