# G3 — Runbook de las corridas del escenario 1 sobre la arquitectura elástica

Plan y razones: [`capacity-planning/entrega3_g3_g4.md`](../../../../capacity-planning/entrega3_g3_g4.md). Esto es solo el
procedimiento. Nada de aquí se ejecuta sin que la ventana esté acordada con G5 y G6 (plan, §6.4).

**Orden:** encender la VM → cargar cuentas → calibrar el generador → operando **R** → operando **E** → traer los
resultados y posprocesar → apagar la VM y restaurar los autoescaladores. Todo lo de la VM se guarda **al terminar cada
serie**: el entorno se destruye en I6 y las métricas caducan.

## 0. Antes (en tu máquina, desde `main` limpio)

```bash
# valores del entorno, de Terraform (no se escriben en ningún archivo)
export DB_HOST="$(terraform -chdir=infra/terraform output -raw db_private_ip)"
export BASE_URL="$(terraform -chdir=infra/terraform output -raw lb_url)"
```

Encender el generador es un `apply` desde `main` con **los mismos `-var` del entorno encendido** (nota 38) más
`-var generator_enabled=true`: crea la VM, su cuenta de servicio, el permiso de leer **solo** el secreto de la base y la regla
del `6379` para su cuenta. Siempre `plan -out`, se revisa, y se aplica el plan guardado. Se borra el archivo del plan al terminar
(guarda la contraseña de la base).

```bash
gcloud compute ssh mooc-load-generator --zone us-east1-c --tunnel-through-iap
```

La VM tarda unos minutos en instalar Docker y bajar la imagen de JMeter: `cat /var/log/mooc-generator-listo.txt`.

## 1. Entregar la contraseña de la base a la VM, y preparar

Si `generator_db_secret_access=true` la VM la lee sola de Secret Manager y este paso sobra. Por omisión **no** lo está
(aplicarlo exige poder cambiar la política de IAM del secreto). La contraseña **no se escribe en ninguna línea de comandos**:
se deja en un archivo temporal de tu máquina (solo para tu usuario), se copia con `scp` por el túnel de IAP y se borra
en el mismo comando; en la VM pasa a `/run/mooc-db-pw` (memoria, solo para su dueño) y se borra la copia del disco. `/run` y
no `/dev/shm`, porque Debian vacía `/dev/shm` de un usuario al cerrar su última sesión SSH. Desde tu máquina:

```bash
F=$(mktemp); ( umask 077; gcloud secrets versions access latest --secret=db-password | tr -d '[:space:]' > "$F" )
gcloud compute scp --zone us-east1-c --tunnel-through-iap "$F" mooc-load-generator:mooc-pw.tmp; rm -f "$F"
gcloud compute ssh mooc-load-generator --zone us-east1-c --tunnel-through-iap --command \
  'sudo install -m 600 -o "$USER" ~/mooc-pw.tmp /run/mooc-db-pw && shred -u ~/mooc-pw.tmp; wc -c < /run/mooc-db-pw'
```

Debe imprimir 32 (los caracteres guardados, no la contraseña). Al terminar la campaña: `sudo rm -f /run/mooc-db-pw`.

(El `ssh` con la contraseña por entrada estándar no sirve con el `gcloud` de Windows: la entrada no llega a la VM.)

Después, en la VM:

```bash
git clone https://github.com/ISIS4426-2026/Plataforma-MOOC.git && cd Plataforma-MOOC
git checkout <SHA de main que está desplegado>          # el mismo commit que corre la nube
export DB_HOST=<la IP de arriba> BASE_URL=<la URL de arriba>
export JMETER_IMAGE='justb4/jmeter@sha256:088ac52b759a198a5afa5ae13d0a6306e9f2017d71ad140ff57427f6930406f7'
export MODO=vm ENTORNO=nube-entrega3

python3 scripts/capacity_criterios.py --autoprueba                       # las reglas de A6 se cumplen
bash scripts/sql_nube_vm.sh scripts/seeds/capacity_data.sql              # 200 cuentas de carga (idempotente)
```

El último comando debe terminar diciendo `cuentas_de_carga | 200`. Si dice 0 o falla con un error de permisos, la regla de IAM
de `generator.tf` no está aplicada.

**Calibración del generador** (G1): `bash scripts/capacity_calibracion_generador.sh` y confirmar `NO LIMITA` antes de medir.

**Tokens** (duran 24 h; se generan a ~7 s por cuenta, unos 24 min, y piden la contraseña de prueba de la semilla por teclado):
los crea `scripts/h3_nube.sh` la primera vez; con `REUSAR_TOKENS=1` los reutiliza en las siguientes series del mismo día.

## 2. Operando R: la API fija en 1 instancia

Desde tu máquina:

```bash
gcloud compute instance-groups managed update-autoscaling mooc-api-group --zone=us-east1-b \
  --min-num-replicas=1 --max-num-replicas=1
```

Esperar a que el grupo quede en 1 instancia estable (`list-instances`). En la VM, con `OPERANDO=R`:

```bash
export OPERANDO=R REUSAR_TOKENS=1
bash scripts/h3_nube.sh repetir 100 4000 2000 3 A100
bash scripts/h3_nube.sh repetir 200 4000 2000 3 A200
RAMPA_FIJA=15 MARGEN_CALENTAMIENTO=10 bash scripts/h3_nube.sh repetir 100 1000 1000 3 B100
RAMPA_FIJA=15 MARGEN_CALENTAMIENTO=10 bash scripts/h3_nube.sh repetir 200 1000 1000 3 B200
RAMPA_FIJA=10 MARGEN_CALENTAMIENTO=5  bash scripts/h3_nube.sh repetir 200 250 500 3 C200
```

Las rampas y márgenes de B son los de A7 (15 s y 10 s); los de C son los de la serie de presión (10 s y 5 s).

## 3. Operando E: la elástica (1 a 3)

Desde tu máquina:

```bash
gcloud compute instance-groups managed update-autoscaling mooc-api-group --zone=us-east1-b \
  --min-num-replicas=1 --max-num-replicas=3
```

En la VM, con `OPERANDO=E`:

```bash
export OPERANDO=E REUSAR_TOKENS=1
bash scripts/h3_nube.sh escalera                                  # niveles 1,10,25,50,100,200 + B100 y B200 (si ninguno satura)
bash scripts/h3_nube.sh repetir 100 4000 2000 2 A100              # completar las 3 repeticiones de cada nivel comparado
bash scripts/h3_nube.sh repetir 200 4000 2000 2 A200
RAMPA_FIJA=15 MARGEN_CALENTAMIENTO=10 bash scripts/h3_nube.sh repetir 100 1000 1000 2 B100
RAMPA_FIJA=15 MARGEN_CALENTAMIENTO=10 bash scripts/h3_nube.sh repetir 200 1000 1000 2 B200
RAMPA_FIJA=10 MARGEN_CALENTAMIENTO=5  bash scripts/h3_nube.sh repetir 200 250 500 3 C200
# serie sostenida: 12 corridas B-200 seguidas (unos 20 min), para que el autoescalador alcance a actuar
PAUSA_ENTRE_CORRIDAS=10 RAMPA_FIJA=15 MARGEN_CALENTAMIENTO=10 bash scripts/h3_nube.sh repetir 200 1000 1000 12 S200
```

La escalera ya deja una repetición de A100, A200, B100 y B200, por eso las demás piden 2. La etiqueta de cada corrida
lleva el operando (`E-A100-r1_…`), así que **R y E conviven en el mismo directorio** sin pisarse.

`h3_nube.sh` se detiene solo si un nivel activa un criterio de **parada** (`escenario1.md` §8.3). Si para, se anota el
motivo y no se reintenta ese nivel sin cambiar algo y documentarlo.

## 4. Traer los resultados y posprocesar (en tu máquina)

Después de **cada** operando, no al final del día:

```bash
gcloud compute scp --recurse --zone us-east1-c --tunnel-through-iap \
  mooc-load-generator:Plataforma-MOOC/docs/entrega3/evidencias/G3/resultados docs/entrega3/evidencias/G3/
bash scripts/capacity_postproceso.sh docs/entrega3/evidencias/G3/resultados
```

El posproceso reúne las métricas de Cloud Monitoring de cada ventana y comprueba que el generador no limitó. Una corrida
`LIMITADA` **no se publica** y se repite. Los tokens (`capacity-planning/datos/tokens.csv`) se quedan en la VM; no se copian.

Por cada operando, la gráfica de cuatro paneles (`python scripts/capacity_graficas.py <corrida>/serie_temporal.csv`) y una
captura del tablero, guardadas como `capturas/AAAA-MM-DD_descripcion.png`.

## 5. Al terminar

1. Restaurar la API a lo que dice Terraform (`--min-num-replicas=1 --max-num-replicas=3`).
2. Apagar el generador: `apply` con `-var generator_enabled=false` y los demás `-var` del entorno encendido.
3. Comprobar con `gcloud compute instances list` que la VM ya no existe, y registrar las horas que estuvo encendida
   (el coste se informa al cerrar, como hizo A7).

Este archivo no contiene credenciales, llaves ni secretos.
