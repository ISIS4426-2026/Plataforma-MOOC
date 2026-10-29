# Runbook — cerrar C5 y C6 en una sola sesión ([#208](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/208), [#209](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/209))

Las dos capturas que bloquean el cierre piden lo mismo —entorno desplegado más
trabajo real en la cola— así que se toman en la misma sesión. Lo que cada issue
necesita demostrar, en cambio, es distinto:

| | Qué tiene que quedar probado | Con qué |
| :--- | :--- | :--- |
| **C5** | una instancia **creada por el grupo** consume y **termina** su primer trabajo sin que nadie la toque | `a7_verificar_e2e.sh` (PDF + imagen, segundos) |
| **C6** | el **autoescalador** amplía el grupo por la señal de cola, y lo reduce al vaciarse | `c6_encolar_carga.sh` (vídeos, minutos) |

**Antes de empezar, lee [`NOTAS_TECNICAS.md`](../../NOTAS_TECNICAS.md)**, en
particular las notas 1 (apply solo desde `main`), 10 (las métricas caducan), 34
(publicar antes de aplicar), 36 (un MIG reutiliza el nombre) y 42 (la señal de C6).

| | |
| :--- | :--- |
| Duración | **~2 h** con el entorno encendido — *estimación, no medida* |
| Coste | **~0,5 USD** sobre los 0,30 USD/día del entorno aparcado — *estimación* |
| Qué se recrea | la **VM de la cola**, una vez (ver §2) |

---

## 0. Dos cosas que hay que saber antes de tocar nada

### El `apply` recrea la VM de la cola, y es irrepetible

C6 mueve el arranque de la cola de `metadata_startup_script` a
`metadata["startup-script"]`. Mover el guion de un sitio al otro es **ForceNew**,
así que este `apply` destruye y vuelve a crear `mooc-queue-server` y **pierde lo
que haya encolado**.

En un entorno aparcado no hay nada encolado, así que no cuesta nada. Pero el plan
lo va a decir y no hay que confundirlo con un error. A cambio, desde esta sesión
tocar el arranque de la cola se aplica **reiniciando** la VM, que conserva el
disco y con él el AOF.

### `cargar_cuentas_carga_nube.sh` apunta a una VM que ya no existe

El guion tiene `VM="mooc-web-server"` por omisión y esa VM **se retiró en C3**:
`compute.tf` ya no declara ninguna instancia. Su `DB_IP` por omisión también es de
antes de que la base se recreara.

Los dos son parametrizables, así que no hay que tocar el guion — hay que pasarle
los valores. En §4 está la línea exacta. **Es la trampa que más tiempo cuesta en
esta sesión**, porque el fallo aparece como un `scp` que no resuelve el nombre.

---

## 1. Pre-vuelo · *tu máquina* · sin coste

C6 tiene que estar mergeado: `terraform apply` solo desde `main` (nota 1).

```bash
git checkout main && git pull
bash ./scripts/lint.sh                      # gofmt, go vet, spectral
bash ./scripts/b4_barrido_secretos.sh       # tiene que dar 0 hallazgos

# Terraform y los guiones no los cubre lint.sh: van aparte
(cd infra/terraform && terraform fmt -check && terraform validate)
bash -n scripts/c6_prueba_autoescalado.sh scripts/c6_encolar_carga.sh
```

Y el vídeo del perfil corto, **verificado campo por campo**:

```bash
bash ./scripts/generar_media_prueba.sh corto
```

Falla si el archivo no es lo que declara. Si falla, no sigas: la Entrega 1 perdió
un tercio de la nota por declarar 2 minutos y subir 8 segundos.

## 2. Publicar imagen · *tu máquina* · sin coste

C6 toca `cmd/queue-exporter/`, `Dockerfile.worker` y `go.mod`, así que entra en la
regla de la nota 34. Aquí importa el doble: un `pull` fallido en la VM de la cola
**no** se ve como un grupo recreando instancias, se ve como una métrica que nunca
aparece.

```bash
bash ./scripts/publish_images.sh            # exige el árbol limpio
export IMAGE_TAG="$(git rev-parse HEAD)"
```

El `data "google_artifact_registry_docker_image"` de `bootstrap.tf` convierte un
tag sin publicar en un error del **plan**, no en un grupo en bucle.

## 3. Encender · *tu máquina* · **empieza el gasto**

```bash
cd infra/terraform

terraform plan -out=encender.tfplan \
  -var "image_tag=${IMAGE_TAG}" \
  -var db_activation_policy=ALWAYS \
  -var lb_enabled=true \
  -var api_autoscaling_enabled=true \
  -var worker_autoscaling_enabled=true
```

Lee el plan antes de aplicar. Lo que tiene que aparecer:

| | |
| :--- | :--- |
| `mooc-queue-server` | **destroy + create** — es el §0, no un error |
| `mooc-worker-autoscaler` | create |
| `mooc-api-autoscaler` | create |
| reglas de reenvío del balanceador | create |
| `artifactregistry.reader` de la cola | create |

Si propone destruir algo más, **para**: alguien aplicó desde una rama (nota 1).

```bash
terraform apply encender.tfplan
```

> Un plan guardado **caduca** en cuanto otro `plan` refresque el estado. Si
> `apply` dice `Saved plan is stale`, vuelve a planificar; no fuerces.

Y los datos que necesitan los pasos siguientes:

```bash
terraform output -raw lb_url            # https://<ip>.sslip.io
terraform output -raw db_private_ip
terraform output      worker_scaling_metric   # métrica y filtro, para pegarlos
cd ../..
```

El certificado gestionado puede tardar hasta una hora en aprovisionarse si se
rehízo. Para esta sesión no estorba: las dos pruebas pueden ir por HTTP si hace
falta.

## 4. Sembrar · *tu máquina*

Las cuentas que usan los dos guiones (`profesor1@…`, `estudiante1@…`) están en
`scripts/seeds/synthetic_data.sql`. La base solo tiene IP privada, así que la
carga va **desde una instancia**, por túnel IAP — y la que había ya no existe
(§0), así que se usa una del grupo de la API:

```bash
export SALTO="$(gcloud compute instance-groups managed list-instances mooc-api-group \
    --zone=us-east1-b --project=plataforma-mooc-entrega2 --format='value(name)' | head -1)"
echo "salto: ${SALTO}"     # tiene que ser mooc-api-algo

VM_NAME="${SALTO}" \
DB_IP="$(cd infra/terraform && terraform output -raw db_private_ip)" \
  bash ./scripts/cargar_cuentas_carga_nube.sh scripts/seeds/synthetic_data.sql
```

Sirve cualquier instancia del grupo: lleva `allow-iap-ssh`, tiene Docker y su
cuenta `sa-web-server` puede leer `db-password` de Secret Manager. La contraseña
no pasa por tu máquina.

## 5. C5 · la instancia nueva consume y termina · ~10 min

C5 se cierra **antes** que C6, porque es el caso barato y porque si falla aquí no
tiene sentido seguir: un grupo cuyas instancias no consumen no va a autoescalar
nada.

```bash
BASE_URL="$(cd infra/terraform && terraform output -raw lb_url)" \
  bash ./scripts/a7_verificar_e2e.sh
```

Sube un PDF y un PNG por el flujo normal —URL prefirmada, PUT, `complete`— y
espera a `completed`. Los dos pasan por el worker como trabajos de cola (nota 16)
y cuestan segundos.

### Las tres capturas de C5 → `evidencias/C5/capturas/`

1. **`mooc-worker-group` en la consola**, con su instancia y la columna de
   creación. Lo que tiene que quedar claro es que **la creó el grupo**: nadie
   ejecutó un `instances create`.
2. **El registro del worker consumiendo**, con `job media:process started` y
   `completed` y su `duración_ms`:

   ```bash
   W="$(gcloud compute instance-groups managed list-instances mooc-worker-group \
       --zone=us-east1-b --project=plataforma-mooc-entrega2 --format='value(name)' | head -1)"
   gcloud compute ssh "${W}" --zone=us-east1-b --tunnel-through-iap \
     --command='sudo docker logs --tail 80 $(sudo docker ps -q --filter name=mooc-worker)'
   ```
3. **El recurso en `completed`**, que es la salida del propio guion.

### Y la medida que C5 dejó pendiente

`initial_delay_sec = 420` es una estimación. Con la instancia ya arrancada:

```bash
gcloud compute ssh "${W}" --zone=us-east1-b --tunnel-through-iap \
  --command='sudo cat /var/lib/mooc/arranque.json'
```

Ese número **más** el arranque del sistema operativo es lo que hay que comparar
contra los 420 (nota al margen: B3 mide desde el inicio del guion, no desde la
creación de la instancia). Si sobra mucho margen, bájalo — y entonces baja
también `cooldown_period` de C6, que es el mismo número por la misma razón.

## 6. C6 · el autoescalador decide sobre la cola · ~60–75 min

### Primero la terminal que observa

```bash
bash ./scripts/c6_prueba_autoescalado.sh docs/entrega3/evidencias/C6/cronologia.txt
```

**Su fase 1 es la que importa**, y no es un adorno. Lee la serie con el filtro
exacto del autoescalador y falla si no devuelve nada, porque:

| Lo que devuelve el filtro | Qué significa |
| :--- | :--- |
| `0` | la serie existe, la cola está vacía. **Correcto** |
| nada | el autoescalador está ciego. **Roto** |

Un filtro que no encuentra la serie **no es un error para GCE**: el autoescalador
sin datos mantiene el tamaño actual, así que el síntoma no es un fallo sino un
grupo que no escala. Si la fase 1 falla, el guion imprime los cuatro sitios donde
mirar, en orden.

No sigas al paso siguiente hasta que la fase 1 pase.

### Después la terminal que carga

```bash
BASE_URL="$(cd infra/terraform && terraform output -raw lb_url)" \
  bash ./scripts/c6_encolar_carga.sh 10
```

Sube 10 copias de `scratch/video_corto.mp4` y **después** confirma las 10
seguidas. El orden es el único truco del guion: el trabajo se encola en la
confirmación, no en el PUT, así que subir todo primero hace que `outstanding`
salte a 10 en segundos **sea cual sea tu velocidad de subida**. Si se confirmara
cada una al subirla, con un enlace doméstico el primer vídeo estaría transcodificado
antes de encolarse el sexto y la cola no pasaría de 2.

Diez vídeos de 120 s, a ~144 s de transcodificación cada uno (A7: 1,2× la
duración) y concurrencia 2, son unos **12 min de cola** con un solo worker: de
sobra para que el autoescalador muestree, decida y arranque.

### Qué va a pasar, y cuándo mirar

| T+ aprox. | Qué |
| ---: | :--- |
| 0 | las 10 confirmaciones; `outstanding` salta a 10 |
| 1–2 min | el autoescalador muestrea y recomienda **3** |
| 2–9 min | las réplicas arrancan: `cooldown_period = 420` |
| ~10 min | tres workers consumiendo, la cola bajando |
| ~15 min | `outstanding` llega a 0 |
| +15, +30 min | la reducción, **una réplica cada 900 s** |

El plazo por omisión del guion que observa son 60 min, dimensionado para ver el
ciclo completo. Termina solo cuando el grupo vuelve al tamaño inicial con la cola
a 0.

### Las cuatro capturas de C6 → `evidencias/C6/capturas/`

1. **El Explorador de Métricas** con `custom.googleapis.com/mooc/queue/outstanding`
   subiendo. El filtro para pegarlo tal cual sale de
   `terraform output worker_scaling_metric`.
2. **El autoescalador** mostrando su tamaño recomendado y el motivo.
3. **El grupo pasando de 1 a 3**, con la hora visible.
4. **El grupo volviendo a 1** al vaciarse la cola.

Lo que las capturas tienen que dejar claro es que **nadie redimensionó el grupo**:
el enunciado dice que «la ampliación manual del grupo no acredita autoscaling».
Por eso el guion compara contra el `targetSize` inicial y solo anuncia el hito
cuando el cambio lo pidió el autoescalador.

## 7. Exportar los datos crudos · *antes de apagar*

La nota 10 aplica entera: esta métrica personalizada **caduca**, y desaparece del
todo cuando el proyecto pierde los recursos en I6. La cronología del guion ya
quedó en `evidencias/C6/cronologia.txt`; añade el registro del worker:

```bash
gcloud compute ssh "${W}" --zone=us-east1-b --tunnel-through-iap \
  --command='sudo docker logs $(sudo docker ps -q --filter name=mooc-worker)' \
  > docs/entrega3/evidencias/C5/worker.log 2>&1

gcloud compute ssh mooc-queue-server --zone=us-east1-b --tunnel-through-iap \
  --command='sudo journalctl -u mooc-queue-exporter --no-pager -n 200' \
  > docs/entrega3/evidencias/C6/exportador.log 2>&1
```

Y revisa los dos archivos antes de versionarlos: **no pueden llevar
credenciales**. El entorno del exportador no tiene ninguna —la cola no lleva
contraseña y Monitoring va por el servidor de metadatos— pero el registro del
worker sí puede traer URLs firmadas. Si aparecen, recórtalas.

## 8. Apagar · *tu máquina*

```bash
cd infra/terraform
terraform apply -var "image_tag=${IMAGE_TAG}"
```

Sin banderas, los tres interruptores vuelven a `false` y el entorno queda
aparcado: se van los autoescaladores y las reglas de reenvío, que facturan
0,025 USD/h cada una aun en reposo. La dirección global y el certificado se
quedan, porque rehacerlos cuesta hasta una hora de aprovisionamiento.

Y los grupos a cero, que sin autoescalador ya no vuelven a levantarse:

```bash
for G in mooc-api-group mooc-worker-group; do
  gcloud compute instance-groups managed resize "$G" --size=0 \
    --zone=us-east1-b --project=plataforma-mooc-entrega2 --quiet
done
```

El `resize` no cuenta como deriva: `target_size` está en `ignore_changes`.

> **`db_activation_policy` no se puede invertir aquí.** Apagar la base requiere
> que la instancia exista primero, así que el `apply` de arriba la deja
> encendida. Pararla es un paso aparte y hay que acordarse:
>
> ```bash
> gcloud sql instances patch "$(terraform output -raw db_instance_name)" \
>   --activation-policy=NEVER \
>   --project=plataforma-mooc-entrega2
> ```

---

## Qué queda abierto después de esta sesión

| | |
| :--- | :--- |
| **C7 (#210)** | el drenaje ordenado del worker. `scale_in_control` acota la frecuencia del daño, no lo evita: 900 s no cubren los 1 711 s del perfil largo |
| **G6** | mide el ciclo de escalado con esta misma máquina. Los números de esta sesión son una comprobación, no la medición |
| `cooldown_period` / `initial_delay_sec` | calibrados con el dato de §5 si sobró margen |

---

Este archivo no contiene credenciales, llaves ni secretos.
