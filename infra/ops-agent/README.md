# Configuración del Ops Agent

Lo que cada VM le dice al agente de Google Cloud que recoja: métricas del sistema,
métricas de la aplicación por Prometheus y **los registros de los contenedores**.

| Archivo | Va en | Se instala en |
| :--- | :--- | :--- |
| [`web.yaml`](./web.yaml) | `mooc-web-server` | `/etc/google-cloud-ops-agent/config.yaml` |
| [`worker.yaml`](./worker.yaml) | `mooc-worker-server` | `/etc/google-cloud-ops-agent/config.yaml` |

Están aquí y no solo en las máquinas por la razón de la nota 19 de
`NOTAS_TECNICAS.md`: la configuración que vive únicamente en una VM la borra la
siguiente recreación, y nadie se entera hasta que hace falta.

## Por qué hacía falta la parte de registros

El agente venía recogiendo métricas, pero la salida de los contenedores se
quedaba en la máquina: Cloud Logging solo tenía `syslog`, los agentes del sistema
y el log de Cloud SQL. Con la API y el worker en máquinas distintas, diagnosticar
significaba entrar por SSH a adivinar en cuál mirar, y una prueba de carga no
podía correlacionar sus propias métricas con lo que la aplicación estaba
diciendo.

## Cómo llegan los registros

Docker usa el controlador `json-file`, así que escribe cada línea en
`/var/lib/docker/containers/<id>/<id>-json.log` envuelta en su propio JSON:

```json
{"log":"{\"level\":\"INFO\",\"msg\":\"media rendered to HLS\"}\n","stream":"stdout","time":"..."}
```

Por eso hay **dos pasos de análisis**: el primero abre el sobre de Docker y el
segundo el JSON estructurado que escribe la aplicación, de modo que `level`,
`msg` y los demás campos lleguen a Cloud Logging como campos consultables y no
como una cadena. Las líneas que no son JSON —el worker imprime algunas con el
`log` estándar de Go— sobreviven como texto.

**Se conserva el controlador `json-file` a propósito.** Existe la alternativa de
poner `gcplogs` como controlador de Docker, que envía directo y ahorra el análisis
en dos pasos, pero entonces `docker logs` deja de funcionar en la máquina. Se
prefirió no perder esa herramienta: es la primera que se usa cuando algo falla.

## Instalar o actualizar

```bash
gcloud compute scp infra/ops-agent/worker.yaml mooc-worker-server:/tmp/ops-agent.yaml \
  --zone=us-east1-b --tunnel-through-iap --project=plataforma-mooc-entrega2

# En la VM:
sudo cp /etc/google-cloud-ops-agent/config.yaml /etc/google-cloud-ops-agent/config.yaml.bak
sudo install -o root -g root -m 0644 /tmp/ops-agent.yaml /etc/google-cloud-ops-agent/config.yaml
sudo service google-cloud-ops-agent restart
sudo service google-cloud-ops-agent status
```

Lo mismo con `web.yaml` en `mooc-web-server`.

## Comprobar que llegan

```bash
gcloud logging read 'logName:"docker_containers"' \
  --project=plataforma-mooc-entrega2 --limit=5 --freshness=10m
```

En la consola: *Logging → Explorador de registros*, y filtrar por
`logName="projects/plataforma-mooc-entrega2/logs/docker_containers"`.

## Costo

Cloud Logging no cobra los primeros 50 GiB por proyecto al mes. El volumen normal
de esta plataforma queda muy por debajo, pero **una prueba de carga sostenida sí
puede mover la aguja**: conviene revisar el volumen ingerido después de las
corridas de H3 y H5 y, si hiciera falta, excluir los registros de acceso del
proxy, que son los más numerosos y los menos informativos.
