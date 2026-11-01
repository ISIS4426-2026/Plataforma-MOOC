# Runbook — cerrar G5 y F1 en una sola sesión ([#225](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/225), [#219](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/219))

Los dos issues tienen una captura que **bloquea el cierre** y las dos piden lo
mismo: el entorno encendido. Lo que cada uno demuestra, en cambio, es distinto:

| | Qué tiene que quedar probado | Con qué |
| :--- | :--- | :--- |
| **F1** | desde internet solo responde el balanceador, en 80 y 443; cada regla de cortafuegos tiene su razón y no queda ninguna de la Entrega 2; los controles de la Entrega 2 siguen en pie | [`f1_superficie_red.sh`](../../../../scripts/f1_superficie_red.sh) (solo lectura, afirma) |
| **G5** | el autoescalador **solo** amplía el grupo de la API por carga, la instancia nueva atiende tráfico del balanceador, y el grupo vuelve a 1 al bajar la carga; con los tiempos de reacción e incorporación por separado | [`g5_carga_escalonada.sh`](../../../../scripts/g5_carga_escalonada.sh) (VM del generador) + [`g5_observar_escalamiento_api.sh`](../../../../scripts/g5_observar_escalamiento_api.sh) (portátil) + [`g5_linea_de_tiempo.py`](../../../../scripts/g5_linea_de_tiempo.py) |

**Antes de empezar, lee [`NOTAS_TECNICAS.md`](../../NOTAS_TECNICAS.md)**, en
particular las notas 1 (apply solo desde `main`), 10 (las métricas caducan), 37
(la señal de la API), 38 (encender es explícito), 40 (una réplica con acción en
curso no recibe tráfico), 43 (el autoescalador «miente» al encenderlo) y 49 (el
generador).

| | |
| :--- | :--- |
| Duración | **~2 h** con el entorno encendido: 15 min F1, ~45 min de carga, ~20 min de reducción, el resto montar y exportar — *estimación, no medida* |
| Coste | **~1 USD**: base, cola, balanceador, 1–2 API, 1 worker y el generador (~0,13 USD/h) — *estimación*; F3 lo contrasta con lo facturado |
| Qué hay que avisar | al equipo: **el balanceador cuenta el tráfico de todos** (nota 49). Si alguien prueba otra cosa durante la carga, la señal del autoescalador ya no es solo la nuestra |

---

## 0. Tres cosas que hay que saber antes de tocar nada

### El grupo tiene que arrancar en 1, y nadie puede subir el mínimo

C4 y E2 suben el mínimo del autoescalador a 2 para sus pruebas. Si se quedó así,
el grupo está en 2 **sin carga** y lo que se mediría sería configuración, no
autoescalado: el enunciado dice que «la ampliación manual del grupo no acredita
autoscaling». El observador se niega a empezar si el mínimo no es 1 o si el
grupo no está en una sola instancia sana. **No pases `-var api_min_replicas`.**

### La carga sale de la VM del generador, no del portátil

La señal de C2 son **peticiones por segundo y por instancia** (umbral: 80). Un
portátil perdió el 20 % de las solicitudes en la calibración de G1 (nota 49): la
tasa que vería el balanceador no sería la que dice el guion. El observador sí va
en el portátil, porque necesita tu `gcloud`; la cuenta del generador solo escribe
registros y métricas.

### Lo que la corrida no prueba, y cómo se cubre

La carga son lecturas (catálogo y `/ready`): a la señal le da igual qué petición
cuenta, y así se puede repetir sin ensuciar la base. La **integridad de las
escrituras** durante el ciclo se demuestra aparte, en el §5, con el arnés de
consistencia de E2 lanzado cuando el grupo ya está en 2 **por el
autoescalador**. Esa misma corrida es la «después de la corrección» que E2 tiene
pendiente: coordínalo con @tmichelldiaz para no hacerla dos veces.

---

## 1. Pre-vuelo · *tu máquina* · sin coste

```bash
git checkout main && git pull
gcloud auth login && gcloud auth application-default login

bash -n scripts/g5_carga_escalonada.sh scripts/g5_observar_escalamiento_api.sh \
        scripts/f1_superficie_red.sh scripts/f1_retirar_reglas_sobrantes.sh
python scripts/capacity_criterios.py --autoprueba
python scripts/g5_linea_de_tiempo.py --autoprueba        # tiene que acabar en "autoprueba: OK"
(cd infra/terraform && terraform init && terraform fmt -check && terraform validate)
```

## 2. Imagen · *tu máquina* · sin coste

La corrección de E2 (#279) cambió `internal/postgres`, así que la imagen de
`main` es nueva (nota 34). Si alguien ya la publicó para la corrida de E2, el
`publish_images.sh` lo detecta y no la repite.

```bash
bash ./scripts/publish_images.sh            # exige el árbol limpio
export IMAGE_TAG="$(git rev-parse HEAD)"
```

## 3. Encender · *tu máquina* · **empieza el gasto**

```bash
cd infra/terraform
terraform plan -out=encender.tfplan \
  -var "image_tag=${IMAGE_TAG}" \
  -var db_activation_policy=ALWAYS \
  -var lb_enabled=true \
  -var api_autoscaling_enabled=true \
  -var worker_autoscaling_enabled=true \
  -var generator_enabled=true
```

Lee el plan antes de aplicar. Lo que tiene que aparecer: las **dos reglas de
reenvío**, los **dos autoescaladores**, la **VM del generador** con su cuenta y
sus dos roles, y las plantillas de API y worker reemplazadas si cambió la imagen
(`create_before_destroy`). **Si propone destruir cualquier otra cosa, para**:
alguien aplicó desde una rama (nota 1).

```bash
terraform apply encender.tfplan
terraform output -raw lb_url      # https://<ip>.sslip.io
cd ../..
```

Antes de seguir, espera a que:

```bash
# el grupo de la API en 1 instancia RUNNING / HEALTHY / NONE
gcloud compute instance-groups managed list-instances mooc-api-group \
  --zone=us-east1-b --project=plataforma-mooc-entrega2
# el autoescalador ACTIVE (los primeros minutos puede decir ERROR: nota 43, se limpia solo)
gcloud compute instance-groups managed describe mooc-api-group \
  --zone=us-east1-b --project=plataforma-mooc-entrega2 \
  --format="value(autoscaler.status,autoscaler.recommendedSize)"
# el generador listo
gcloud compute ssh mooc-load-generator --zone=us-east1-c --tunnel-through-iap \
  --command='cat /var/log/mooc-generator-listo.txt'
```

## 4. F1 · superficie de red · ~15 min

Va primero porque no genera carga y se hace mientras el autoescalador termina de
estabilizarse. Corre **desde tu portátil**: el barrido tiene que salir de
internet, no de dentro de la VPC.

```bash
bash ./scripts/f1_superficie_red.sh
```

Escribe `a_` a `d_` y `resumen.txt` en [`../F1/`](../F1/) y sale con **1 si hay
alguna FALLA**. Lo que probablemente falle la primera vez son **reglas
sobrantes**: la red `default` de la Entrega 2 con sus reglas (hallazgo 2 de
aquella verificación) o algo que A7 dejara al recrear la Entrega 2. Para
retirarlas, **revísalo antes con el equipo** —es irreversible—:

```bash
bash ./scripts/f1_retirar_reglas_sobrantes.sh   # pregunta una por una; deja retiro_reglas.txt
bash ./scripts/f1_superficie_red.sh             # hasta que salga con 0
```

Si una de las cinco reglas del código sale **DISTINTA**, no se borra: se corrige
con `terraform apply` desde `main`. El guion de retiro no las toca nunca.

### Las tres capturas de F1 → `evidencias/F1/capturas/`

| Archivo | Qué tiene que verse |
| :--- | :--- |
| `AAAA-MM-DD_barrido-externo.png` | la terminal con el barrido del balanceador (solo 80 y 443) y la **fecha** de la cabecera. **Es la que bloquea el cierre** |
| `AAAA-MM-DD_reglas-firewall.png` | consola › Red de VPC › Cortafuegos, con las reglas que quedan |
| `AAAA-MM-DD_instancias-sin-ip-externa.png` | consola › Compute Engine › Instancias de VM, con la columna «IP externa» vacía en todas |

Sin IPs privadas a la vista: recorta la columna de IP interna.

## 5. G5 · el ciclo de escalamiento · ~70 min

Tres terminales. **El orden importa**: primero la que observa.

### Terminal A · *tu portátil* · el observador

```bash
bash ./scripts/g5_observar_escalamiento_api.sh \
  "docs/entrega3/evidencias/G5/observacion_$(date -u +%Y%m%d)"
```

Su fase 1 comprueba el punto de partida y **se para** si no es el que pide el
issue (autoescalador `ACTIVE`, mínimo 1, una sola instancia sana, y que el
`X-Instance-ID` que responde corresponda a esa instancia). No sigas hasta que
diga `PARTIDA` y pida la **CAPTURA 1**.

### Terminal B · *la VM del generador* · la carga

Los guiones llegan en un paquete (el repositorio es privado y la VM no tiene
credenciales de GitHub):

```bash
# en el portátil
git archive --format=tar.gz -o g5.tgz HEAD \
  scripts/g5_carga_escalonada.sh scripts/capacity_resumen_jtl.py scripts/capacity_criterios.py \
  capacity-planning/escenario3a_api.jmx
gcloud compute scp g5.tgz mooc-load-generator:~ --zone=us-east1-c --tunnel-through-iap && rm g5.tgz
gcloud compute ssh mooc-load-generator --zone=us-east1-c --tunnel-through-iap
```

```bash
# dentro de la VM
mkdir -p ~/mooc && tar -xzf ~/g5.tgz -C ~/mooc && cd ~/mooc
sudo env BASE_URL="https://<ip>.sslip.io" ENTORNO=nube-entrega3 \
  COMMIT="<git rev-parse --short HEAD del portátil>" \
  JMETER_IMAGE="justb4/jmeter@sha256:088ac52b759a198a5afa5ae13d0a6306e9f2017d71ad140ff57427f6930406f7" \
  nohup bash scripts/g5_carga_escalonada.sh > g5_carga.log 2>&1 &
tail -f g5_carga.log
```

`nohup` porque son ~45 minutos y el túnel IAP se puede caer. Lo primero que
imprime es la tabla de etapas con su hora UTC:

| Etapa | Tasa | Duración | Qué tiene que pasar |
| :--- | ---: | ---: | :--- |
| e1 base | 10 pet/s | 5 min | nada: una instancia, muy por debajo del umbral |
| e2 subida | 40 pet/s | 2 min | nada todavía |
| e3 subida | 70 pet/s | 2 min | nada todavía: 70 < 80 |
| **e4 alta** | **130 pet/s** | 15 min | 130 > 80 con una instancia → el autoescalador pide **2**. Con dos, ~65 cada una: se estabiliza en 2, no oscila |
| e5 bajada | 10 pet/s | 20 min | el autoescalador reduce. `scale_in_control` (C2): una cada 600 s |

### Qué va a anunciar el observador, y cuándo capturar

| T+ aprox. desde e4 | Hito | Captura → `evidencias/G5/capturas/` |
| ---: | :--- | :--- |
| antes de e1 | `PARTIDA` | **1** `AAAA-MM-DD_g5-partida-1-instancia.png`: el grupo con 1 instancia y la pestaña *Supervisión* |
| 1–2 min | `DECISION_AMPLIAR` | **2** `AAAA-MM-DD_g5-autoescalador-recomienda-2.png`: *Supervisión* con el tamaño recomendado > 1 y la utilización del balanceador |
| 1–2 min | `INSTANCIA_CREADA`, `INSTANCIA_RUNNING` | — (quedan en `hitos.tsv` con el `creationTimestamp` exacto) |
| 3–5 min | `INSTANCIA_SANA` | **3** `AAAA-MM-DD_g5-instancia-nueva-sana.png`: la lista de instancias del grupo con las dos `HEALTHY` |
| 3–5 min | `INSTANCIA_ATIENDE` | **4** `AAAA-MM-DD_g5-reparto-dos-instancias.png`: esa línea de la terminal, con dos `X-Instance-ID` repartiéndose |
| e5 + ~10 min | `DECISION_REDUCIR` | **5** `AAAA-MM-DD_g5-reduccion.png`: la instancia en `DELETING` o ya retirada |
| e5 + ~12 min | `CICLO_COMPLETO` | **6** `AAAA-MM-DD_g5-ciclo-completo.png`: el grupo en 1 y la gráfica del tamaño en *Supervisión* con la subida y la bajada |

Las capturas 1–6 son lo que **bloquea el cierre** y el material del vídeo (I4).
No se pueden rehacer después de I6.

### Terminal C · *tu portátil* · integridad de escrituras con el grupo en 2

En cuanto el observador anuncie `INSTANCIA_ATIENDE` —el grupo está en 2 **por
el autoescalador**, no porque alguien subiera el mínimo—:

```bash
go run ./scripts/e2e_cloud -escenario consistencia -min-instancias 2 \
  -base "https://<ip>.sslip.io" \
  -salida docs/entrega3/evidencias/G5/consistencia_durante_el_ciclo
```

Son unos minutos y caben de sobra en los 15 de la etapa alta. Es la corrida de
E2 (envíos duplicados, inscripciones simultáneas, progreso repetido, lecturas
entre réplicas) **con la instancia nueva recién incorporada**. Si E2 la necesita
como su corrida «después de la corrección», que @tmichelldiaz enlace esta
carpeta en vez de repetirla.

## 6. Traer y cruzar los datos · *antes de apagar*

Las métricas caducan y desaparecen en I6 (nota 10): esto va **antes** del §7.

```bash
# la corrida de carga, de la VM al portátil (la ruta exacta la imprime el guion al terminar)
gcloud compute scp --recurse --zone=us-east1-c --tunnel-through-iap \
  "mooc-load-generator:~/mooc/docs/entrega3/evidencias/G5/carga/escenario3a_*" \
  docs/entrega3/evidencias/G5/carga/

# métricas de Cloud Monitoring y contraste del generador (G1)
bash scripts/capacity_postproceso.sh docs/entrega3/evidencias/G5/carga

# la línea de tiempo: reacción, incorporación, auditoría y comprobaciones
python scripts/g5_linea_de_tiempo.py \
  docs/entrega3/evidencias/G5/carga/escenario3a_<fecha> \
  docs/entrega3/evidencias/G5/observacion_<fecha>

# la gráfica del ciclo (cuatro paneles: instancias, CPU, cola, balanceador)
python scripts/capacity_graficas.py \
  docs/entrega3/evidencias/G5/carga/escenario3a_<fecha>/serie_temporal.csv \
  docs/entrega3/evidencias/G5/capturas/AAAA-MM-DD_g5-tablero-del-ciclo.png \
  --titulo "G5 - escenario 3A"
```

`g5_linea_de_tiempo.py` sale con **1** si algo no cuadra. Las que pueden fallar
de verdad, y qué significan:

| Comprobación que falla | Qué quiere decir |
| :--- | :--- |
| la etapa alta no alcanzó la tasa pedida | el generador o sus hilos limitaron; la corrida no prueba el umbral. Revisa `generador.csv` y repite e4 con más hilos |
| la decisión es anterior al aumento de carga | había tráfico de otra persona, o el grupo ya estaba subiendo. No se publica |
| una PERSONA cambió el tamaño o la política | alguien hizo `resize` o `update-autoscaling` en la ventana. La corrida no vale como evidencia |
| errores reales por encima del 1 % | mira en qué tramo: la incorporación (`currentAction`, nota 40) o el retiro (drenaje de 60 s, C3) |

Revisa los archivos antes de versionarlos: **sin credenciales, tokens ni IPs
privadas**. El observador solo guarda nombres, IDs numéricos de instancia y
`X-Instance-ID`; la auditoría sustituye los correos de personas por «persona».

## 7. Apagar · *tu máquina*

Igual que el §8 del [runbook de C6](../C6/RUNBOOK.md), más el generador:

```bash
cd infra/terraform
terraform apply -var "image_tag=${IMAGE_TAG}"          # sin banderas: aparca y apaga el generador

for G in mooc-api-group mooc-worker-group; do
  gcloud compute instance-groups managed resize "$G" --size=0 \
    --zone=us-east1-b --project=plataforma-mooc-entrega2 --quiet
done

gcloud sql instances patch "$(terraform output -raw db_instance_name)" \
  --activation-policy=NEVER --project=plataforma-mooc-entrega2
```

Ojo con la VM de la cola: no pertenece a ningún grupo y **el `resize` no la
toca** (F3 lo apunta como el coste que se escapa). Si el equipo decide pararla al
aparcar, es `gcloud compute instances stop mooc-queue-server --zone=us-east1-b`,
y la próxima sesión tiene que arrancarla (`start`) antes de encender los grupos;
conserva el disco y con él el AOF.

Si G6 (@DiegoOrtizRuiz) quiere aprovechar el entorno encendido para el ciclo de
workers, este es el momento de pasárselo en vez de apagar.

---

## Después de la sesión

| | |
| :--- | :--- |
| `G5/README.md` | completar el estado con los números de `linea_de_tiempo.md` y enlazar las capturas |
| `F1/README.md` | lo mismo con `resumen.txt` |
| `NOTAS_TECNICAS.md` | lo que el equipo deba saber: los dos tiempos medidos (para G7 e I2), las reglas retiradas (para F2 e I6) |
| `bash ./scripts/check_evidencias.sh --informe` | G5 y F1 ya no tienen que aparecer como pendientes |

---

Este archivo no contiene credenciales, llaves ni secretos.
