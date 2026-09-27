# Configuración efectiva y marco de costos — Entrega 2

Issue **B1** (#114). Este documento fija la configuración que no debe cambiar
durante las corridas de capacidad, y el marco de costos con el que se contrasta
después el consumo observado.

**Fecha de la estimación: 2026-09-26.** Todas las cifras salen de la
[calculadora oficial de Google Cloud](https://cloud.google.com/products/calculator),
consultada ese día con **precios de lista** —sin vincular la cuenta de
facturación, para que un evaluador pueda reproducirlas— y sin descuentos por uso
comprometido.

**La estimación completa es pública y verificable línea por línea:**
[abrir en la calculadora de Google Cloud](https://cloud.google.com/products/calculator?dl=CjhDaVEwTVRWbE9HRTJaUzB6TVRFNExUUTBZalV0WWpNeE15MDRObUZsWlRKbE1tWmhPVGdRQVE9PRAOGiRFNDVDOUIwOS1EODRDLTRGRTUtOTk5MC0xRkZCMDk1NTU4QjI).
El resto de la evidencia está en [`evidencias/B1/`](evidencias/B1/README.md).

---

## 1. Proveedor, proyecto y región

| | |
| :--- | :--- |
| **Proveedor** | Google Cloud Platform |
| **Facturación** | Cuenta de facturación con cupones *Google Cloud for Education*, 50 USD por integrante, **redimidos de forma secuencial** (§4). *Usage scope:* «Any service on this billing account» |
| **Proyecto** | `plataforma-mooc-entrega2` — dedicado a la entrega, para que el consumo sea atribuible |
| **Región y zona** | `us-east1` (Carolina del Sur), **una sola zona para todo** (p. ej. `us-east1-b`) |

**Por qué GCP.** El equipo dispone de cupones educativos en esa plataforma, y su
catálogo cubre los cuatro servicios que el enunciado exige con productos
administrados directos: Compute Engine, Cloud SQL para PostgreSQL, Cloud Storage
y VPC. No se introduce ningún proveedor adicional.

**Por qué `us-east1`.** Dos razones, en ese orden:

1. **Costo.** `us-east1` está en el nivel de precios más bajo de GCP, idéntico a
   `us-central1`. `southamerica-east1` (São Paulo) tiene sobreprecio en cómputo,
   almacenamiento y transferencia de salida. Con 50 USD por integrante, ese
   sobreprecio se paga en horas de corrida que no se podrían ejecutar.
2. **Latencia desde el generador de carga**, medida y no supuesta. Las pruebas se
   ejecutan desde una máquina local en Bogotá (§6). Medición con `gcping.com` el
   2026-09-26:

   | Región | Latencia mediana |
   | :--- | ---: |
   | **`us-east1`** (South Carolina) | **74 ms** |
   | `us-east4` (North Virginia) | 93 ms |
   | `us-central1` (Iowa) | 114 ms |
   | `northamerica-south1` (México) | 123 ms |
   | `southamerica-east1` (São Paulo) | **178 ms** |

   `us-east1` no solo «no está más lejos» que São Paulo: es **la región de menor
   latencia de todas** desde Bogotá, y São Paulo responde 2,4 veces peor.

   El dato que mejor ilustra por qué la cercanía geográfica no predice la latencia
   de red no es São Paulo sino **México, a 123 ms** —peor que Carolina del Sur
   pese a estar mucho más cerca—, coherente con que el tráfico desde Colombia
   hacia el sur se enrute por Miami.

**No hay compromiso que resolver entre costo y latencia:** `us-east1` es a la vez
la opción más barata y la más rápida. La evidencia está en
[`evidencias/B1/latencia_us-east1.txt`](evidencias/B1/latencia_us-east1.txt).

**Por qué una sola zona, y la misma para todo.** Dos razones que apuntan en la
misma dirección:

1. **El enunciado lo exige** para la base de datos —«despliegue en una sola zona
   de disponibilidad, sin réplicas de lectura»— y el docente fue explícito en que
   en este ejercicio hay **cero escalabilidad y cero disponibilidad**. Repartir
   las VMs entre zonas fingiría una tolerancia a fallos que el ejercicio pide no
   tener.
2. **El tráfico interno deja de costar.** Dentro de una región, GCP factura el
   tráfico entre zonas distintas (del orden de 0,01 USD/GiB) pero no el que ocurre
   dentro de una misma zona por IP interna. Con las dos VMs y la base en la misma
   zona, el tráfico de la cola Redis/asynq, el de la base y el del bucket son
   gratuitos, y por eso la línea de *internal data transfer* de la estimación va
   en cero.

Esto es un requisito para **B2**: el Terraform debe fijar la zona explícitamente
en los tres recursos, no dejar que el proveedor la elija.

> **Pendiente para H1:** falta caracterizar la máquina del generador —CPU,
> memoria y sobre todo el ancho de banda del enlace— y verificar que no sea ella
> la que limite los resultados. Si el cuello de botella fuera la conexión
> doméstica, el informe mediría la casa y no la plataforma.

---

## 2. Configuración efectiva

Una sola instancia por componente. **Cero escalabilidad y cero disponibilidad**:
sin autoescalado, sin réplicas, sin balanceadores, y el número de máquinas
permanece fijo durante las pruebas. El objetivo de esta entrega es obtener la
línea base de capacidad contra la cual se medirá lo que aporte la escalabilidad
más adelante.

| Recurso | Componentes | Perfil | Almacenamiento |
| :--- | :--- | :--- | :--- |
| **Web Server** (VM) | API modular en Go + proxy inverso | `e2-highcpu-2` — 2 vCPU, 2 GiB | 30 GiB *balanced* |
| **Worker Server** (VM) | Workers en Go con FFmpeg + Redis/asynq | `e2-highcpu-2` — 2 vCPU, 2 GiB | 30 GiB *balanced* |
| **Base de datos** | Cloud SQL para PostgreSQL, Enterprise, **zona única, sin réplicas de lectura** | 1 vCPU dedicada, 3,75 GiB | 10 GiB SSD |
| **Almacenamiento de objetos** | Cloud Storage, clase Standard, región única | — | Según uso |
| **Red** | VPC propia, subredes y reglas de firewall (B3); 2 IPv4 externas estáticas | — | — |
| **Correo** | SMTP administrado externo — **sin VM** (§7) | — | — |

### El perfil de VM: calce exacto, sin desviación

El enunciado fija «2 vCPU, 2 GiB de RAM y 30 GiB» por máquina y pide identificar
el perfil disponible que corresponda, o «la más cercana que **cubra** los recursos
indicados», justificando la desviación.

El primer candidato obvio, `e2-small`, **no sirve**, y conviene dejar escrito por
qué: la propia calculadora de Google lo declara con **0,5 vCPU**, no con 2. Las
familias `e2-micro`, `e2-small` y `e2-medium` son de núcleo compartido y esa es su
fracción garantizada. Medio vCPU no cubre dos.

Precios verificados en la calculadora el 2026-09-26, los tres con región
`us-east1`, disco de 30 GiB y 730 h/mes, de modo que son directamente
comparables:

| Perfil | vCPU | RAM | USD/mes | ¿Cubre 2 vCPU + 2 GiB? |
| :--- | ---: | ---: | ---: | :--- |
| `e2-small` | 0,5 | 2 GiB | 15,23 | **No** — núcleo compartido |
| `e2-medium` | 1 | 4 GiB | 27,46 | **No** |
| **`e2-highcpu-2`** | **2** | **2 GiB** | **39,11** | **Sí, exacto** |
| `n2d-highcpu-2` | 2 | 2 GiB | 48,53 | Sí |
| `e2-standard-2` | 2 | 8 GiB | 51,92 | Sí, con 4× la RAM |
| `n1-highcpu-2` | 2 | 1,8 GiB | 54,72 | **No** — no llega a 2 GiB |
| `n2-highcpu-2` | 2 | 2 GiB | 55,34 | Sí |
| `c2d-highcpu-2` | 2 | 4 GiB | 57,72 | Sí |
| `c4d-highcpu-2` | 2 | 3 GiB | 58,67 | Sí |

**La decisión: `e2-highcpu-2` en ambas máquinas.** Es el calce exacto —2 vCPU y
2 GiB, con vCPU dedicadas, porque las variantes `highcpu` de la familia E2 no son
de núcleo compartido— y el más barato de todos los que cubren el requisito, por
9,42 USD/mes sobre el siguiente.

**No hay desviación que justificar**: la configuración efectiva es exactamente la
que definió la organización.

Cada máquina son 36,11 USD de cómputo más 3,00 USD del disco de 30 GiB
*balanced*, a 0,10 USD por GiB-mes.

**Lo único que queda por vigilar es la memoria.** 2 GiB para FFmpeg, Redis y
asynq conviviendo en el Worker Server es justo. Si alguna dependencia no arranca
o el sistema empieza a paginar durante el escenario 2, el camino está previsto por
el propio enunciado: documentar el fallo observado, el ajuste mínimo aplicado y su
efecto en costo y capacidad, y reportar los resultados como una configuración
diferente. El ajuste natural sería `e2-standard-2` —2 vCPU y 8 GiB por 51,92
USD/mes— solo en esa máquina.

## 3. Volumen de objetos y operaciones

Las cantidades se derivan del sistema construido, no de supuestos genéricos. Es
lo que hace la estimación contrastable después.

**Cómo se estructura un video procesado.** La escalera declarada es 360p + 720p
(`internal/transcode/rendition.go`) y los segmentos HLS son de **6 segundos**
(`internal/transcode/ffmpeg.go`). Un video de duración *D* produce:

```
1 original                       (originals/<stable_id>/<uuid>.mp4, privado)
1 master.m3u8                    (hls/<stable_id>/, lectura pública)
2 playlists de variante
2 × ⌈D / 6⌉ segmentos
```

| Perfil de medio | Duración | Objetos | Derivados | Original (supuesto 5 Mbps) | Total |
| :--- | ---: | ---: | ---: | ---: | ---: |
| Corto | 2 min | 44 | 53 MB | 75 MB | 128 MB |
| Medio | 10 min | 204 | 264 MB | 375 MB | 639 MB |
| Largo | 30 min | 604 | 792 MB | 1 125 MB | 1,9 GB |

*Derivados calculados con 800 + 96 kbps (360p) y 2 500 + 128 kbps (720p).*

> Estos tres perfiles deben ser **los mismos que G1 (#127) siembre**, o la
> estimación y el consumo observado no serán comparables.

**Precios de Cloud Storage, clase Standard, región única** (`us-east1`):

| Concepto | Precio | Nuestro volumen |
| :--- | :--- | :--- |
| Almacenamiento | 0,020 USD / GB-mes | 3 GiB aprovisionados (2,62 GiB de los tres perfiles, más margen) |
| Operaciones clase A (escritura) | 0,05 USD / 10 000, primeras 50 000 gratis | 852 al sembrar → gratis |
| Operaciones clase B (lectura) | 0,004 USD / 10 000 | 250 000/mes previstas |

**Total verificado en la calculadora: 0,08 USD/mes.** Ocho céntimos, y el desglose
del CSV es elocuente: los 3 GiB de almacenamiento redondean a **cero**, las 1 000
operaciones de clase A caen en la capa gratuita, y los ocho céntimos son
**íntegramente las 250 000 lecturas de clase B**.

Es decir: a nuestra escala, guardar los objetos no cuesta nada; lo poco que cuesta
es leerlos. Que es exactamente la razón por la que el riesgo está en la
transferencia de salida y no en el bucket.

**Conclusión: el almacenamiento de objetos es, a esta escala, insignificante.**
Cuesta céntimos. Lo que domina es el cómputo, y después la base de datos. La
política de encendido y apagado (§5) debe concentrarse ahí, no aquí.

### Un riesgo de costo que no es obvio: la transferencia de salida

El prefijo `hls/` se sirve **sin firma y con lectura pública** (nota 1b de
[`NOTAS_TECNICAS.md`](NOTAS_TECNICAS.md)), porque un reproductor no hereda la
*query string* al resolver las variantes del manifiesto. Eso significa que cada
reproducción completa es transferencia de salida a internet.

Una reproducción íntegra del perfil medio en 720p son ~197 MB. **Mil
reproducciones completas serían ~192 GiB de egreso**, y la calculadora los cifra
en **36,29 USD/mes** (`us-east1` → Sudamérica, *Premium tier*).

Para poner esa cifra en contexto: es **casi tres cuartas partes de un cupón
entero** gastados en descargar videos que a nadie le interesa ver, y supera el
costo mensual de cualquiera de las dos máquinas virtuales. El uso normal de
desarrollo y pruebas funcionales —5 GiB/mes— cuesta 0,76 USD.

La consecuencia es una restricción para el plan de carga de **H2 y H4**: el
escenario 1 es actividad académica (catálogo, inscripción, contenido, progreso,
quizzes) y **no requiere descargar videos completos**; el escenario 2 mide carga
y procesamiento, es decir *subida* y trabajo del worker, no reproducción masiva.
Los guiones deben verificar el manifiesto y, como máximo, los primeros
segmentos. Descargar videos enteros a escala convertiría una prueba de capacidad
en una factura de transferencia.

---

## 4. Estimación de costos

**Supuestos.** Operación **24×7** durante una ventana de un mes = **730 horas**
por recurso, según la aclaración del docente de que el proyecto tiene sentido
para operar de forma continua. Precios de lista, sin descuentos por uso
comprometido, región `us-east1`, fecha 2026-09-25.

| Concepto | Cantidad | USD/mes |
| :--- | :--- | ---: |
| Web Server — `e2-highcpu-2` + disco 30 GiB | 730 h | **39,11** |
| Worker Server — `e2-highcpu-2` + disco 30 GiB | 730 h | **39,11** |
| Cloud SQL PostgreSQL — 1 vCPU / 3,75 GiB, zona única | 730 h | **49,31** |
| Cloud SQL — almacenamiento SSD | 10 GiB | **1,70** |
| Cloud Storage — 3 GiB y sus operaciones | — | **0,08** |
| 2 direcciones IPv4 externas, con las VMs encendidas | 730 h | **3,65** |
| Egreso a internet → Sudamérica, *Premium tier* | 5 GiB | **0,76** |
| **TOTAL, operación 24×7** | | **133,73** |

Cotizado pero **no incluido**, por las razones de cada línea:

| Concepto | USD/mes | Por qué no está |
| :--- | ---: | :--- |
| Cloud NAT (1 VM, 5 GiB procesados) | 4,90 | Más caro que darle una IP externa al worker — ver abajo |
| Egreso de 192 GiB | 36,29 | El escenario que los planes de carga deben evitar (§3) |
| 2 IPv4 reservadas con las VMs **apagadas** | 14,59 | No es una alternativa sino una advertencia — ver §5 |

### Una IP externa para el worker, no Cloud NAT

Si B3 deja el Worker Server en una subred privada, necesitará Cloud NAT para
salir a internet a bajar imágenes y actualizaciones. Los números resuelven la
disyuntiva:

| Opción | USD/mes |
| :--- | ---: |
| **2 IPv4 externas** (una por VM) | **3,65** |
| 1 IPv4 externa + Cloud NAT para el worker | 1,83 + 4,90 = 6,73 |

Cloud NAT cuesta casi el doble. La recomendación para **B3** es darle al worker su
propia IPv4 externa y **cerrar todo el tráfico entrante por reglas de firewall**:
tener dirección pública no es lo mismo que ser alcanzable, y la seguridad la da la
regla de firewall, no la ausencia de IP.

### La base de datos: por qué 1 vCPU dedicada y no un perfil compartido

Los perfiles de núcleo compartido de Cloud SQL (`db-f1-micro`, `db-g1-small`)
están **excluidos del SLA** y la documentación de Google los describe como
instancias de prueba y desarrollo, no de producción. Reaparece el problema del
worker: el informe de capacidad debe registrar «conexiones y carga de la base de
datos administrada», y si esa base tiene CPU compartida con ráfagas, es probable
que se convierta en el cuello de botella y que lo que se mida sean sus créditos
de ráfaga en lugar de la plataforma.

De ahí la elección: **1 vCPU dedicada y 3,75 GiB**, que es el mínimo dedicado del
catálogo, con 10 GiB de SSD. Son 49,31 USD de cómputo más 1,70 de almacenamiento.

Es el componente más caro de la estimación después de las dos VMs juntas, y aun
así es el mínimo defendible: bajar de ahí significa volver al núcleo compartido y
perder la reproducibilidad de la medición.

Hay que vigilar las conexiones: el pool de la API abre hasta `DB_MAX_OPEN_CONNS`
(25 por defecto) y a eso se suma el del worker; las instancias pequeñas de Cloud
SQL permiten bastante menos de 100 conexiones (nota 11 de
[`NOTAS_TECNICAS.md`](NOTAS_TECNICAS.md)). Con 1 vCPU conviene comprobar el límite
efectivo antes del escenario 1, no durante.

**Si la base resulta ser el cuello de botella del escenario 1, eso es un hallazgo
del informe, no un error de dimensionamiento.** Lo que el enunciado pide es medir
la línea base de esta configuración y explicar dónde se degrada, no construir la
infraestructura que aguante más.

### La estimación 24×7 frente al crédito disponible

Son dos números distintos y conviene no confundirlos:

* La **estimación 24×7** es la cifra de planeación que el docente pidió: cuánto
  costaría operar la infraestructura de forma continua durante un mes. Son
  **133,73 USD/mes**.
* El **gasto real** lo gobierna la política de encendido y apagado (§5).

Puesto de otro modo: **un cupón de 50 USD paga unos 11 días de operación
continua.** Los cuatro, poco más de mes y medio. Por eso los recursos solo se
encienden para implementar, probar o sustentar, y por eso la política de apagado
no es una recomendación de buenas prácticas sino la condición para terminar la
entrega.

### El crédito: 50 USD a la vez, 200 en total

Los cuatro cupones se redimen **en secuencia**, no todos de una vez: cuando el
crédito activo se agota, se redime el siguiente. Eso tiene tres consecuencias
prácticas.

**1. El techo operativo es 50 USD, no 200.** El presupuesto y las alertas se
configuran sobre 50 USD, que es el crédito disponible en cualquier momento. Los
200 USD son el total acumulado del semestre, no un saldo.

**2. Las alertas deben medir el gasto bruto, no el gasto después de créditos.**
Este es el ajuste que más importa acertar. Un presupuesto de GCP puede calcular
el costo **incluyendo o excluyendo los créditos promocionales**. Si los incluye,
el cupón absorbe el consumo, el costo «después de créditos» se mantiene cerca de
cero y **las alertas no se disparan hasta que el crédito ya se agotó** — es decir,
justo cuando dejan de servir. Hay que configurar el presupuesto para que **excluya
los créditos** y mida el consumo bruto contra los 50 USD.

De todas formas, el indicador autoritativo de cuánto cupón queda es la página de
**créditos de la cuenta de facturación**, no el presupuesto. El presupuesto avisa;
el saldo de crédito es el que dice cuándo toca redimir el siguiente.

**3. Guardar cupones en reserva podía caducarlos, pero no es el caso.** Los
cuatro cupones son idénticos y dejan margen de sobra:

| | |
| :--- | :--- |
| Activación | 2026-08-19 |
| **Redimir antes de** | **2026-12-19** |
| Crédito válido hasta | 2027-08-19 |
| Tipo de aplicación | *Net pricing* |
| Alcance | Any service on this billing account |

El cupón 1 se redimió el 2026-09-25. Con la fecha límite de redención en
diciembre y el crédito vigente hasta agosto de 2027, **el esquema secuencial no
corre riesgo de expiración dentro de la ventana de la entrega** y no hay que
adelantar ninguna redención.

**Qué pasa al llegar a cero.** La consola de Cloud no expone una sección de
métodos de pago para esta cuenta —el perfil de pagos se administra en el centro
de pagos de Google, no aquí—, pero el resumen de facturación la identifica como
**«Paid account»** y no como cuenta de prueba. De ahí el supuesto de trabajo:

> **Al agotarse el crédito, el consumo pasa a cobrarse; no se suspende.**

Es la hipótesis conservadora y por eso se adopta sin más verificación. Si
resultara equivocada y los servicios se suspendieran, la disciplina que se deriva
de ella sirve igual; al revés no: dar por hecho que se suspende y descubrir que
alguien pagó de su bolsillo es el error caro.

**Procedimiento al agotar un cupón.** Cuando el saldo activo se acerque a cero:
redimir el siguiente cupón, comprobar que el saldo se refleja en la cuenta, y
**solo entonces** seguir operando.

**Línea base de consumo**, tomada el 2026-09-26 antes de crear ningún recurso:

| | |
| :--- | :--- |
| Crédito «Cloud solutions development» | 50,00 de 50,00 USD — 100 % restante |
| Gasto acumulado (1–26 de septiembre de 2026) | 0,00 USD |

Es contra estos dos números que se contrasta después el consumo observado, como
pide el enunciado.

---

## 5. Presupuesto, alertas y política de encendido

### Presupuesto y alertas

La cuenta de facturación es propia y su *usage scope* es «any service on this
billing account», de modo que el equipo tiene permiso para crear presupuestos.
**No aplica la excepción de «limitación del laboratorio»**: el criterio se cumple
con un presupuesto activo.

| | |
| :--- | :--- |
| **Monto** | **50 USD** — el crédito activo en cualquier momento, no los 200 acumulados |
| **Periodo** | **Rango personalizado**, no mensual (ver abajo) |
| **Ahorros y créditos** | **Todas las casillas de la sección *Savings* deseleccionadas**, para que el presupuesto mida el consumo bruto |
| **Umbrales de alerta** | 25 %, 50 %, 80 %, 100 % |
| **Destinatarios** | Los cuatro integrantes |
| **Alcance** | Toda la cuenta de facturación, que contiene un único proyecto: `plataforma-mooc-entrega2` |

**Por qué el periodo es un rango personalizado y no mensual.** Un presupuesto
mensual se reinicia el día 1 de cada mes. El cupón no: son 50 USD que se agotan
acumulándose, sin importar el calendario. Con un presupuesto mensual de 50 USD,
gastar 30 en octubre y 30 en noviembre no cruzaría ningún umbral, y sin embargo el
cupón ya estaría agotado. El rango personalizado acumula desde su fecha de inicio
y es el único que refleja cómo se consume el crédito.

**Sobre el alcance.** La intención era acotar el presupuesto al proyecto, pero la
consola no permitió seleccionarlo en el desplegable de *Scope*. Se dejó con el
alcance de toda la cuenta de facturación, que hoy es equivalente: esa cuenta
contiene un único proyecto. La consecuencia es que `Email alerts to project
owners` queda deshabilitado —irrelevante, porque las alertas llegan por
`Email alerts to billing admins and users`— y que **si alguna vez se crea un
segundo proyecto bajo esta cuenta, el presupuesto lo abarcaría también**. Si eso
ocurre, hay que volver a acotarlo o las cifras dejarán de ser atribuibles a la
entrega.

**Por qué las casillas de *Savings* van deseleccionadas.** En la consola los
créditos aparecen en una sección llamada *Savings*, y **por defecto están todas
seleccionadas**, es decir, el presupuesto descuenta el cupón antes de comparar
contra el monto. Con el crédito absorbiendo el consumo, el costo ronda cero y los
umbrales no se cruzan nunca. Deseleccionarlas es lo que hace que las alertas
lleguen mientras el cupón se gasta, en lugar de después.

Dos advertencias sobre lo que un presupuesto de GCP **no** hace:

* **Notifica, no corta.** Existen *spend cap budgets* que sí pausan el consumo,
  pero están en Preview y solo cubren Gemini API, Cloud Run y Cloud Run
  functions — ninguno de los servicios de esta entrega. Para Compute Engine,
  Cloud SQL y Cloud Storage la alerta al 80 % es un aviso para apagar recursos, no
  una red de seguridad.
* **Si incluye los créditos en el cálculo, no avisa de nada.** Con el cupón
  absorbiendo el consumo, el costo después de créditos ronda cero y los umbrales
  nunca se cruzan. Es el error que dejaría al equipo sin aviso hasta que el
  crédito se agotara.

Y un detalle de la consola: las alertas llegan por omisión a quienes tengan rol
de administrador o de gestor de costos sobre la cuenta de facturación. **Añadir
otros correos no es un campo de texto**: hay que crear canales de notificación de
Cloud Monitoring y asociarlos al presupuesto. Alternativa más simple para un
equipo de cuatro: dar a los otros tres el rol *Billing Account Costs Manager*,
con lo que reciben las alertas sin configurar canales.

### Política de encendido y apagado

1. **Por defecto, todo apagado.** Las VMs se encienden para implementar, probar o
   sustentar, y se detienen al terminar la sesión de trabajo.
2. **Detener una VM no elimina todos sus costos, y una IP ociosa cuesta más que
   una en uso.** Con las dos máquinas y la base detenidas, sin borrar nada, se
   sigue pagando:

   | Concepto | USD/mes |
   | :--- | ---: |
   | 2 discos persistentes de 30 GiB | 6,00 |
   | **2 IPv4 reservadas, sin VM encendida** | **14,59** |
   | Almacenamiento de Cloud SQL (10 GiB SSD) | 1,70 |
   | Cloud Storage | 0,08 |
   | **Total con todo apagado** | **22,37** |

   Casi la mitad de un cupón al mes por no tener nada encendido. Y lo
   contraintuitivo: **las mismas dos direcciones IP cuestan 3,65 USD con las VMs
   encendidas y 14,59 con las VMs apagadas** —cuatro veces más—, porque Google
   penaliza las direcciones reservadas y ociosas para desincentivar que se
   acaparen.

   La consecuencia práctica: para pausas de un día o un fin de semana, se apagan
   las VMs y se conservan las IP. **Para pausas largas conviene liberar las
   direcciones**, lo que baja el costo fijo a 7,78 USD/mes, a cambio de tener que
   reconfigurar DNS y certificados al volver. Esa decisión se toma con el
   calendario de la entrega delante, no sobre la marcha.
3. **Durante una corrida de capacidad, nada se apaga ni se redimensiona.** La
   configuración efectiva permanece fija; cualquier cambio de tamaño obliga a
   reportar los resultados como una configuración diferente.
4. **No iniciar una corrida larga con menos crédito del que va a consumir.** Con
   la estimación en la mano el cálculo es trivial, y evita los dos desenlaces
   malos: que la corrida se corte a mitad —perdiendo la medición— o que el
   consumo se desborde hacia cargos reales. Si el saldo no alcanza, primero se
   redime el siguiente cupón.
5. **Cloud SQL — suspender la instancia.** Resuelto en **C2** (#119); las
   condiciones del proveedor están abajo, en su propio apartado.
6. **Al cerrar la entrega** (issue I6), se elimina la instancia de base de datos
   administrada, conservando antes los respaldos, los datos sintéticos y los
   scripts necesarios para reconstruirla. Se documenta qué recursos se conservan,
   su costo y cómo recrear el entorno para una sustentación.

---

## 6. Generador de carga

Fuera de las dos máquinas virtuales de la aplicación, según el enunciado y
confirmado por el docente: **los usuarios virtuales no se simulan desde la misma
nube**. Se ejecuta desde una **máquina local** apuntando al despliegue.

Implicaciones para B1:

* **No hay una tercera VM que costear.** El generador no aparece en la
  estimación.
* Hay que **registrar la ubicación y los recursos** de esa máquina —CPU, memoria,
  ancho de banda de subida y bajada, y latencia medida hacia `us-east1`— y
  verificar que no sea ella la que limita los resultados. Si el cuello de botella
  es el enlace doméstico, el informe mediría la casa y no la plataforma.
* La herramienta se decide en **H1**; el docente mencionó JMeter como ejemplo.

---

## 7. Correo

**SMTP administrado externo, sin máquina virtual.** El docente autorizó una VM
dedicada a Mailpit, pero también aclaró que no le interesa probar el correo
dentro de las pruebas de carga: 730 horas mensuales de cómputo para un componente
que no se mide no se justifican, y el issue **F1** (#126) ya planeaba sustituir
Mailpit por SMTP en la nube.

Con ello el correo deja de ser infraestructura que mantener, y la verificación de
cuentas queda como en producción. F1 elige el proveedor y gestiona sus
credenciales **fuera del repositorio**.

---

## 8. Seguridad de la evidencia

El docente aclaró el alcance de «las evidencias no deben aparecer en el
repositorio»: se refiere a **no subir credenciales, archivos de cuentas de
servicio ni archivos `.env`**. Los documentos de evidencia sí se versionan.

En la práctica, para esta entrega:

* Las llaves de cuentas de servicio y los `.env` nunca entran al repositorio; la
  gestión de secretos es el issue **B4** (#117).
* Las capturas y salidas que sí se versionan se revisan antes: identificadores de
  proyecto y de recursos son aceptables; tokens, llaves y cadenas de conexión con
  contraseña, no.

---

## Suspender la instancia de base de datos: las condiciones del proveedor

Issue **C2** (#119). La regla 5 de operación pedía consultar y registrar aquí las
condiciones antes de detener la instancia, porque es la palanca de coste más
grande que tenemos: **la base es el componente más caro después de las dos VMs**,
y el crédito por integrante son 50 USD.

### El mecanismo

`infra/terraform/database.tf` expone `db_activation_policy`, con `ALWAYS` y
`NEVER`. Detener la instancia es un cambio de código y un `apply`, no un clic en
la consola.

Que sea variable **de código y no de entorno** es deliberado: si viviera en el
entorno de cada uno, el `apply` de quien no la exportara volvería a encender la
instancia en silencio y el aviso llegaría en la factura.

### Qué se deja de pagar, y qué no

La documentación de Google es explícita:

> «Stopping an instance suspends instance charges. The instance data is
> unaffected, and charges for storage and IP addresses continue to apply.»
>
> — [Start, stop, and restart instances](https://docs.cloud.google.com/sql/docs/postgres/start-stop-restart-instance)

Traducido a nuestra estimación:

| Concepto | 24×7 | Detenida |
| :--- | ---: | ---: |
| Cómputo — 1 vCPU / 3,75 GiB | 49,31 USD | **0,00** |
| Almacenamiento SSD 10 GiB | 1,70 USD | 1,70 USD |
| **Total** | **51,01 USD** | **1,70 USD** |

**Detener la instancia ahorra el 97 % de su coste.** Es la diferencia entre que un
cupón de 50 USD dure un mes o dure el semestre, y por eso la instancia debería
existir para las corridas y estar detenida el resto del tiempo.

La cifra hay que seguir contrastándola con el informe de facturación tras la
primera parada real; lo que ya no es una suposición es **qué** se deja de pagar.

### La consecuencia que no está en la página de precios

**Mientras la instancia está detenida, las copias automáticas no se ejecutan.**
Se registran como omitidas:

> «When a Cloud SQL instance stopped running since the last successful backup, a
> new automated backup isn't created», con estado `STATUS_SKIPPED` en los
> registros de auditoría.
>
> — [View audit logs for automated backups](https://docs.cloud.google.com/sql/docs/postgres/backup-recovery/view-audit-logs-for-automated-backups)

Para esta entrega es aceptable, y conviene decir por qué en lugar de dejarlo
implícito: **los datos son sintéticos y reproducibles**. El esquema sale de
`migrations/` y los datos de `scripts/seeds/`, así que una base perdida se
reconstruye con `scripts/recrear-entorno.sh`. No hay nada que solo exista en la
base.

Eso deja de ser cierto en el momento en que una corrida genere datos que haya que
conservar para el informe. **Antes de detener la instancia después de un
escenario de carga, tomar una copia manual** —`gcloud sql backups create`, 82 s
medidos— o exportar a un bucket. Si se detiene sin hacerlo, la ventana automática
no lo va a cubrir.

### Lo que la documentación no dice

Dos cosas que buscamos y no están documentadas, así que **no se pueden dar por
supuestas**:

* Si Google reactiva por su cuenta una instancia detenida, y en qué
  circunstancias.
* Si hay un límite de tiempo para dejarla detenida.

Lo que sí está documentado es un caso de parada automática distinto: una instancia
a punto de quedarse sin espacio **se detiene sola** para evitar pérdida de datos, y
vuelve a detenerse a las 24 horas si el problema persiste. Contra eso ya hay
defensa desde C1: `disk_autoresize` activo con techo en 20 GiB.

La conclusión práctica: **al reanudar para una corrida, comprobar el estado antes
de dar por hecho que sigue detenida**, y al revés. Un `gcloud sql instances
describe --format='value(state,settings.activationPolicy)'` cuesta un segundo.
