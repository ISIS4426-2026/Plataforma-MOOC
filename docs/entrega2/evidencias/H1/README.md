# Evidencia H1 — Instrumentación de las corridas y generador de carga externo (issue #131)

## Estado

| | |
| :--- | :--- |
| CPU/memoria/red/disco de ambas VMs, mediante agente | ✅ Instalado, activo y reportando datos reales en Cloud Monitoring — ver abajo |
| Conexiones y carga de la base administrada | ✅ Cloud SQL expone estas métricas de forma nativa, sin agente — ver "Base de datos administrada" |
| Profundidad, antigüedad y tasa de procesamiento de la cola | ✅ [`internal/worker/queue_metrics.go`](../../../../internal/worker/queue_metrics.go) — nuevo, no existía antes |
| Apuntar al entorno cloud las métricas Prometheus de la Entrega 1 | ⚠️ Worker: ✅ reportando (`up=1`). API: 🛑 configurado pero no reporta (`up=0`) — falta un redeploy, ver "Worker sí, API todavía no" |
| Elegir la herramienta de carga, registrar nombre y versión, justificarla | ✅ Apache JMeter (imagen `justb4/jmeter:latest`, JMeter 5.6.3) |
| Desplegar el generador fuera de las dos VMs | ✅ Corre en la máquina de Tania, fuera de la VPC — ver "Máquina del generador" |
| Verificar que el generador no limita los resultados | ⚠️ Parcial — ver "Máquina del generador" |

## Herramienta elegida: Apache JMeter

**Por qué JMeter y no otra cosa:** el docente lo mencionó explícitamente como
ejemplo válido en el hilo del issue ("puede ser su equipo local apuntando al
despliegue en la nube con una herramienta como JMeter"), y el equipo lo
confirmó como elección. Corre vía Docker (`justb4/jmeter:latest`, que empaqueta
JMeter 5.6.3), igual que Postman/newman ya corre en este proyecto — sin
instalar Java ni nada más en la máquina de quien ejecuta la prueba.

**Versión:** JMeter 5.6.3 (dentro de la imagen `justb4/jmeter:latest`,
dígest verificado el 2026-09-27).

## Máquina del generador

Corre en la máquina de Tania (Windows 11), **fuera de las dos VMs de la
aplicación**, tal como exige el issue.

| | |
| :--- | ---: |
| Procesador | AMD Ryzen 5 5500U, 6 núcleos / 12 hilos |
| Memoria total | 6 133 240 KB (≈ 5.85 GiB) |
| Memoria libre en el momento de la prueba | 609 496 KB (≈ 595 MiB) — **muy ajustado** |
| Velocidad de descarga (prueba simple, un solo flujo) | ≈ 5.8 Mbps (0.73 MB/s) |

**Hallazgo real, no cosmético:** con solo ~600 MiB libres de 5.85 GiB
totales, esta máquina está genuinamente cerca de su propio límite antes de
generar ninguna carga. Para las corridas de H2/H3 (más hilos, más duración),
hace falta cerrar aplicaciones (navegador, Docker Desktop si no se está
usando, etc.) antes de correr, o el cuello de botella medido sería la máquina
del generador, no la plataforma — exactamente lo que el issue pide evitar.

**Pendiente:** la medición de ancho de banda de arriba es una prueba de un
solo flujo TCP con `curl`, que subestima la velocidad real. Antes de la
corrida formal de H2/H3, hace falta una prueba de velocidad real (fast.com o
speedtest.net) para tener un número confiable como evidencia.

## Smoke test contra la nube real

`docs/entrega2/evidencias/H1/smoke_test.jmx`: 5 usuarios virtuales, 10
iteraciones cada uno, contra `GET /api/v1/health` y `GET /api/v1/courses`
(los dos de solo lectura, no tocan datos). No es el recorrido académico
completo — eso lo define H2 — es la prueba mínima de que el generador
funciona contra el origen real y deja una primera línea base.

```bash
bash ./scripts/run_jmeter_smoke.sh
```

Resultado real (2026-09-27, ver
[`resultados/smoke_test_20260927_164136.jtl`](./resultados/smoke_test_20260927_164136.jtl)):

```
summary =    100 in 00:00:17 =    6.0/s Avg:   155 Min:    74 Max:  2637 Err:     0 (0.00%)
```

100 peticiones, **0 errores**. El máximo de 2637ms corresponde a las
primeras conexiones (negociación TLS en frío); una vez las conexiones quedan
abiertas (`keep-alive`), la latencia baja a 74–106ms — coherente con los
74ms de latencia de red medidos en B1 desde Bogotá hacia `us-east1`.

## Cola: profundidad, antigüedad y tasa de procesamiento

Antes de este issue, el worker solo contaba trabajos **procesados y
fallidos** (`worker.jobs.processed`, `worker.jobs.failed`,
`internal/worker/metrics.go`, issue #21). Eso responde "cuántos terminaron",
no "cuántos hay esperando ahora" ni "qué tan viejo es el más antiguo" — que
son propiedades de la cola en sí, no de un trabajo que ya pasó por ella.

`internal/worker/queue_metrics.go` (nuevo) agrega tres medidores, uno por
cola (`critical`, `default`, `low`), leídos directamente de Redis en cada
lectura de métricas vía `asynq.Inspector`:

| Métrica | Qué mide |
| :--- | :--- |
| `worker.queue.pending` | Trabajos esperando a que un worker los tome |
| `worker.queue.active` | Trabajos siendo procesados ahora mismo |
| `worker.queue.oldest_pending_age_seconds` | Antigüedad del trabajo en espera más viejo (`asynq.QueueInfo.Latency`) |

Verificado con las pruebas existentes de `internal/worker` (`go test
./internal/worker/...`, 0 fallos) — no rompe nada de lo que ya había.

## Base de datos administrada

Cloud SQL expone sus propias métricas (conexiones activas, CPU, memoria,
IOPS del disco) a Cloud Monitoring **sin necesidad de ningún agente**: es
parte del servicio administrado, activo desde que se creó la instancia en
C1. No hay nada que instalar aquí — la evidencia es simplemente que la
instancia `mooc-db-1` aparece en Cloud Monitoring con esas métricas ya
disponibles.

## CPU, memoria, red y disco de las VMs

Se usó **Ops Agent Policies** (`gcloud compute instances ops-agents
policies`), el mecanismo que Google Cloud construyó específicamente para
instalar el Ops Agent en VMs **que ya están corriendo**, sin recrearlas ni
reiniciarlas — aplicado en segundo plano por el propio sistema operativo de
cada VM.

```bash
gcloud compute instances ops-agents policies create mooc-ops-agent \
  --project=plataforma-mooc-entrega2 --zone=us-east1-b \
  --file=ops-agent-policy.yaml
```

```yaml
agentsRule:
  packageState: installed
  version: latest
instanceFilter:
  inclusionLabels:
    - labels:
        proyecto: plataforma-mooc
```

El filtro por la etiqueta `proyecto: plataforma-mooc` (que ambas VMs ya
llevan desde que Terraform las creó) cubre `mooc-web-server` y
`mooc-worker-server` sin nombrarlas una por una.

**Hallazgo real durante la verificación:** la política se creó con
`rolloutState: SUCCEEDED`, pero 45+ minutos después las métricas seguían sin
aparecer, y `sudo systemctl status google-cloud-ops-agent` en la VM
respondía `Unit ... could not be found` — el agente nunca se instaló. La
causa: a las dos VMs les faltaba la metadata `enable-osconfig = "TRUE"`, que
el agente de OS Config necesita para *actuar* sobre una política asignada,
no solo para recibirla. Sin ella, `rolloutState: SUCCEEDED` describe que la
asignación de la política se creó correctamente en el backend de Google, no
que algo se haya instalado en la VM — una distinción que el propio comando
no deja clara.

**La corrección:**

```bash
gcloud compute instances add-metadata mooc-web-server --zone=us-east1-b \
  --metadata=enable-osconfig=TRUE
gcloud compute instances add-metadata mooc-worker-server --zone=us-east1-b \
  --metadata=enable-osconfig=TRUE
```

Aplicada primero en caliente contra las VMs (sin reiniciarlas: a diferencia
de `metadata_startup_script`, una clave de metadata cualquiera sí se
actualiza sin forzar el reemplazo de la instancia — confirmado con
`terraform plan` mostrando `0 to destroy` después de declarar la misma
metadata en `compute.tf`), y declarada también en Terraform
(`google_compute_instance.web_server.metadata` /
`.worker_server.metadata`) para que quede en el estado del equipo y no como
un cambio hecho solo a mano. `terraform apply`: **1 to add, 0 to change, 0
to destroy** — únicamente registró la API `osconfig.googleapis.com`, ya
habilitada manualmente, como recurso gestionado.

**Confirmado, verificado dos veces de forma independiente:**
1. Un compañero (conectado a `mooc-web-server` por otro motivo) corrió
   `sudo systemctl status google-cloud-ops-agent` y confirmó `active
   (running)`.
2. La API de Cloud Monitoring devuelve series de tiempo reales para las dos
   instancias, con el desglose completo por estado de CPU:

```
instancia 2554794830132975210 (mooc-web-server)    | idle: 97.27%  user: 1.61%  system: 0.98%
instancia 6977342437443558222 (mooc-worker-server) | idle: 97.95%  ...
```

## Conectar las métricas Prometheus de la app (issue #21) a Cloud Monitoring

**No hizo falta reiniciar ninguna VM**, al final. La vía que se descartó
primero (`metadata_startup_script` vía Terraform) sí era destructiva:

```
$ terraform plan
  # google_compute_instance.web_server must be replaced
  # google_compute_instance.worker_server must be replaced
Plan: 2 to add, 0 to change, 2 to destroy.
```

Cambiar `metadata_startup_script` en un `google_compute_instance` ya
desplegado fuerza su reemplazo completo en este proveedor de Terraform —
habría borrado los certificados HTTPS, el estado de Docker y tumbado la
plataforma para reconstruirla desde cero. El plan se descartó sin
aplicarse.

**La vía que sí funcionó:** un `OSPolicyAssignment` de OS Config (el mismo
mecanismo que instaló el propio Ops Agent) con un recurso de tipo `file`,
que escribe `/etc/google-cloud-ops-agent/config.yaml` directamente —sin
tocar `metadata_startup_script`, sin SSH, sin reiniciar la VM—:

```bash
gcloud compute os-config os-policy-assignments create app-metrics-web \
  --project=plataforma-mooc-entrega2 --location=us-east1-b \
  --file=ops-agent-config-web.yaml   # receptor prometheus -> localhost:8080/api/v1/metrics

gcloud compute os-config os-policy-assignments create app-metrics-worker \
  --project=plataforma-mooc-entrega2 --location=us-east1-b \
  --file=ops-agent-config-worker.yaml  # receptor prometheus -> localhost:9090/metrics
```

Las dos máquinas necesitaban configuraciones distintas (puerto y ruta
diferentes), así que primero se les agregó una etiqueta propia
(`componente: web` / `componente: worker`, `gcloud compute instances
add-labels`, aditivo) para poder dirigir cada política a la VM correcta.
Confirmado con `os-policy-assignment-reports`: **1/1 policies compliant**
en las dos.

Escribir el archivo no reinicia el servicio que lo lee. Para eso sí hacía
falta una sesión en la VM — y ahí apareció el segundo hallazgo real de esta
sesión: **a Tania le faltaba el permiso `iap.tunnelInstances.accessViaIAP`**,
así que cualquier intento de `gcloud compute ssh --tunnel-through-iap` fallaba
con `Remote side unexpectedly closed network connection`, sin importar
cuántas veces se reintentara. `--troubleshoot` lo diagnosticó en un
párrafo. Con `resourcemanager.projectIamAdmin` (que Tania ya tenía), se
concedió el rol a sí misma:

```bash
gcloud projects add-iam-policy-binding plataforma-mooc-entrega2 \
  --member="user:tmicheldiaz@gmail.com" --role="roles/iap.tunnelResourceAccessor"
```

Con el permiso puesto, conectó por SSH y corrió en las dos VMs:

```bash
sudo systemctl restart google-cloud-ops-agent
```

Reinicia solo el agente de monitoreo, no la aplicación — cero impacto en
lo que sirve la plataforma.

### Worker sí, API todavía no

Verificado contra la métrica `up` que el propio receptor Prometheus del
Ops Agent reporta por cada objetivo (1 = scrape exitoso, 0 = fallido):

| VM | Objetivo | `up` |
| :--- | :--- | :---: |
| `mooc-worker-server` | `localhost:9090/metrics` | **1** ✅ |
| `mooc-web-server` | `localhost:8080/api/v1/metrics` | **0** 🛑 |

El worker publica su puerto de métricas al host en
`docker-compose.prod.yml` (`"${WORKER_METRICS_PORT:-9090}:9090"`) porque no
tiene ningún otro servidor HTTP con el que compartirlo. La API **no**: no
tiene ninguna sección `ports:` en absoluto, porque hasta ahora nada externo
a la red de Docker necesitaba llegar a ella directamente —nginx la alcanza
por la red interna de Compose, no por `localhost`—. El Ops Agent corre como
servicio del sistema operativo, **fuera** de la red de Docker, así que
`localhost:8080` no resuelve a nada desde donde él está parado. Por eso el
receptor del worker funciona y el de la API no: no es un problema del
receptor ni de la política, es que el puerto nunca estuvo expuesto al host
para empezar.

**Corregido en código** (`docker-compose.prod.yml`), publicando el puerto
solo en `127.0.0.1` (nunca en `0.0.0.0`: nginx sigue siendo el único camino
de entrada desde fuera de la VM):

```yaml
api:
  ports:
    - "127.0.0.1:8080:8080"
```

**No se aplicó todavía** porque exige un redeploy real del contenedor
`api` en la VM (el mismo ciclo que D1/D2 ya establecieron: mergear, y
luego quien haga el despliegue lo aplica) — no algo para forzar al cierre
de una sesión larga sin avisar.

## Nunca

Credenciales, llaves ni secretos. La contraseña de la base de datos, usada
para `terraform plan`, se leyó directamente desde Secret Manager por Tania
en su propia terminal — nunca pasó por esta sesión.
