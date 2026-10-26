# Evidencia A7 — Línea base retrospectiva de la Entrega 2 medida contra la nube ([#199](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/199))

| Criterio de aceptación | |
| :--- | :---: |
| La línea base de los dos escenarios está medida contra la nube | ⏳ §4 |
| Usa los archivos verificados de A5 y pasa las comprobaciones de A6 | ✅ §2 |
| Queda marcada como medición retrospectiva, con commit, configuración y fecha | ✅ §2 |
| Tabla de parámetros constantes y diferencias hacia la Entrega 3 | ✅ [RUNBOOK §12](./RUNBOOK.md) |
| Dos corridas del escenario 1: código del tag y código corregido | ⏳ §4 |
| El coste por petición de las correcciones queda cuantificado | ⏳ §4 |
| El entorno queda destruido y el coste registrado | ⏳ §4 |
| Anotado en `NOTAS_TECNICAS.md` | ✅ nota 26 y corrección de la nota 1 |

**Estado: preparado y verificado, pendiente de ejecutar.** Todo lo que se puede
comprobar sin encender el entorno está comprobado, con su salida en
[`prevuelo.txt`](./prevuelo.txt). El procedimiento está en
[`RUNBOOK.md`](./RUNBOOK.md) y el pre-vuelo se repite con
`bash ./scripts/a7_preparar.sh`.

---

## 1. Dos cosas del issue que no se pueden hacer como están escritas

### El prefijo de estado separado no es viable, y es al revés

El issue y la nota técnica 1 piden «un prefijo de estado remoto separado para no
mezclar el estado de la Entrega 2 con el de la Entrega 3». **No se puede.** El
entorno de la Entrega 2 sigue existiendo a medias y está registrado en el prefijo
`entrega2`: 50 recursos, entre ellos los buckets, que tienen **nombre único
global**. Un prefijo nuevo intentaría crearlos de cero y fallaría.

Lo correcto, y más seguro: **A7 usa `entrega2`** y la **Entrega 3 estrena
`entrega3`**. La condición que sí hay que respetar es la primera tarea del issue:
A7 corre y destruye antes de que ninguna rama de la Entrega 3 aplique Terraform.
Con una sola arquitectura viva a la vez, un prefijo no mezcla nada.

El plan desde el tag lo confirma: **3 a crear, 0 a cambiar, 0 a destruir**. Y el
pre-vuelo se para si aparece **cualquier** destrucción, porque eso significaría
que el estado ya no es el de la Entrega 2. Corregido en la nota 1 y anotado para
B1, B2, B3, C1, C3, C5, F2 e I6.

### `apply` desde el tag, no desde `main`

A7 es la única excepción a «apply solo desde `main`», y por diseño: aplica la
configuración **del tag**, porque medir la línea base con la infraestructura de
`main` ya no sería la línea base. Se cumple la intención de la regla con dos
condiciones, escritas en el runbook: nadie más aplica mientras A7 está en curso,
y A7 destruye antes de que la Entrega 3 empiece.

## 2. Lo que ya está comprobado, sin encender nada

```
== 3. Plan de Terraform desde el tag (solo lectura)
  Plan: 3 to add, 0 to change, 0 to destroy.
  ok     ninguna destruccion propuesta
         crea   google_sql_database.mooc will be created
         crea   google_sql_database_instance.main will be created
         crea   google_sql_user.app will be created

== 5. Estado actual del entorno
  ok     mooc-web-server apagada (hay que encenderla: Terraform no gestiona el encendido)
  ok     mooc-worker-server apagada (hay que encenderla: Terraform no gestiona el encendido)
  ok     no hay instancia de base: la crea el apply
  ok     la base se borro hace 27 dias: el nombre ya esta libre
  ok     el secreto db-password existe (no se lee su valor aqui)

== 6. Comprobaciones de A6 y A5
  ok     las reglas de medicion se cumplen (capacity_criterios.py)
  ok     los tres perfiles multimedia son los declarados
```

La medición queda identificada como retrospectiva por construcción: el pre-vuelo
imprime los dos commits que se van a medir, cada corrida escribe su
`destino.json` con la URL efectiva, el commit, el generador y la ventana UTC, y
el resumen se niega a publicar tablas sin él (A6).

## 3. Lo que el pre-vuelo encontró y nadie esperaba

### El entorno está a medias, no apagado

Las dos VMs existen **apagadas**, pero **Cloud SQL se borró el 2026-09-28** y
también salió del estado de Terraform. Así que encender no basta: hace falta un
`apply` que cree la base, y luego reapuntar las VMs, porque **la IP privada será
distinta** de la que tienen escrita.

Y Terraform **no gestiona el encendido**: `desired_status` no está declarado, por
eso el plan dice «0 a cambiar» con las dos máquinas apagadas. Se encienden con
`gcloud`.

### Ninguna imagen del código que hay que medir está publicada

En Artifact Registry hay cinco imágenes, todas del 2026-09-27, y **ninguna
corresponde al commit del tag ni a `main`**. La más reciente está **59 commits
antes del tag**, y la diferencia incluye `internal/worker/queue_metrics.go`, que
es justamente lo que el escenario 2 instrumenta.

Medir la línea base exige construir y publicar la imagen del tag. Con dos
trampas:

- `publish_images.sh` usa `git rev-parse HEAD` y `.` como contexto, **los dos
  relativos al directorio actual**: llamarlo por ruta absoluta desde el
  repositorio etiquetaría el código de `main` con el SHA del tag.
- `entrega-2` es un **tag anotado**, así que `git rev-parse entrega-2` devuelve el
  objeto de tag (`e81fa4ef…`) y no el commit (`1827cdc2…`).
  `prepare_web_env.sh` solo valida que sean 40 hexadecimales, así que el objeto de
  tag pasaría la validación y luego la imagen no existiría.

### Por qué la Entrega 2 midió el escenario 2 contra Docker Compose

No fue descuido: la campaña necesita el inspector de la cola, y el cortafuegos
solo abre el 6379 dentro de `10.0.1.0/24`; Cloud SQL no tiene IP pública. Desde
un portátil no hay a dónde conectarse, y `-base http://api:8080` estaba como
valor por omisión.

La salida es llevar los dos puertos por el **túnel SSH de IAP**, que usa el 22 y
no abre nada nuevo, así que **F1** sigue pudiendo demostrar que no hay puertos de
aplicación ni de datos expuestos. Está en el paso 8 del runbook.

### `REDIS_URL` no bloquea la corrida corregida

`docker-compose.prod.yml` de `main` ya no levanta Redis y exige `REDIS_URL`
(A4), lo que parecía obligar a esperar a B1. No: `prepare_web_env.sh` ya escribe
`REDIS_URL='${QUEUE_PRIVATE_IP}:6379'`, la cola de la VM worker por la VPC, con
las sesiones en la base lógica 1. La corrida 2 se puede desplegar tal cual.

### Lo que cuesta, y qué partida se dispara

| | |
| :--- | :--- |
| Campaña completa, ~7,5 h | **1,69 USD** acotando el consumo HLS |
| Consumo HLS sin acotar, desde el portátil | 22,1 GB de egreso → **2,65 USD** |
| Escenario 2 con perfiles reales | 162 min de vídeo a 1,2× en 2 vCPU → **97 min de pared** |

Lo que se dispara no es el cómputo: es el **egreso** del consumo HLS, que sin
acotar cuesta más que todas las horas de máquina juntas. `MAX_SEGMENTOS=10` lo
baja a 0,43 USD dejando 8 segmentos con instante de reproducción que cumplir, que
es el mínimo para que el margen de buffer y las interrupciones signifiquen algo.

## 4. Lo que falta, y por qué

**Ejecutar la campaña.** Necesita tres cosas que no son mías:

1. La contraseña de la base, que sale de Secret Manager con las credenciales de
   quien ejecuta.
2. `terraform apply`, que crea infraestructura facturable.
3. ~7,5 horas de entorno encendido y entre 1,69 y 3,92 USD del crédito.

Cuando se ejecute, aquí entran: las dos corridas del escenario 1 con su
`destino.json`, la del escenario 2, la comparación contra lo publicado en la
Entrega 2, el coste por petición de las correcciones como número, la verificación
del E2E pendiente, las capturas y el coste real medido.

---

Este archivo no contiene credenciales, llaves ni secretos.
