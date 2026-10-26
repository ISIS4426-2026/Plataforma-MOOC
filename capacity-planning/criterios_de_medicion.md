# Criterios de medición y validación de métricas — Entrega 3

**Fecha:** 2026-10-25. **Estado:** acordado antes de la primera corrida de la
Entrega 3, incluida la línea base retrospectiva de A7.
**Issue:** [#198](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/198) (A6).

Este documento no define *qué* se mide: eso está en
[`escenario1.md`](./escenario1.md) y [`escenario2.md`](./escenario2.md). Define
**cómo se mide y qué puede publicarse**, y se aplica por igual a los dos
escenarios, a la línea base y a todas las corridas de esta entrega.

## Por qué existe

La retroalimentación calificó *Análisis de capacidad — Escenario 2* con
**6.67/10**. Tres de los cuatro motivos son defectos de instrumentación, no de
la plataforma:

| Lo que marcó la revisión | Causa localizada |
| :--- | :--- |
| Se midió contra Docker Compose y se publicó como nube | `-base http://api:8080` era el valor **por omisión** del programa, y la salida cruda no registraba el destino |
| Los perfiles de 2, 10 y 30 min eran clips de 8, 20 y 40 s | `run_capacity_escenario2.sh` **los generaba así**, con las duraciones escritas a mano, sin comparar con lo declarado |
| El tiempo al primer cuadro era latencia HTTP más 45 ms fijos | Constante en el código, presentada como «sonda de reproductor real» |
| Hay p95 por debajo del p50 | **Tres estimadores de percentil distintos** en cuatro archivos; el del escenario 2 usaba una fórmula por percentil y con n=2 el «p95» era el valor mínimo |

Ninguno se detectó durante la entrega porque **nada los comprobaba**. Las reglas
de este documento están implementadas en
[`scripts/capacity_criterios.py`](../scripts/capacity_criterios.py) y se ejecutan
dentro de los scripts que publican las tablas, no en una revisión aparte: una
revisión aparte es exactamente lo que no ocurrió.

```bash
python scripts/capacity_criterios.py --autoprueba
```

Reproduce el defecto de la Entrega 2 y comprueba que las reglas nuevas lo
impiden. Si esto falla, no se mide nada.

---

## 1. Clasificación de resultados

Toda petición cae en **una** de estas clases, y la clase decide si cuenta como
fallo. La separación ya existía para el escenario 1; aquí se cierra para la
infraestructura elástica, que introduce clases nuevas.

| Clase | Qué es | Cuenta como fallo |
| :--- | :--- | :---: |
| **Éxito** | 2xx con el estado funcional esperado | no |
| **Rechazo esperado** | 401, 403, 409 y 429 que el guion provoca a propósito, y los 4xx de validación de entrada | no |
| **Fallo funcional** | 2xx con el estado **equivocado**: un envío duplicado que califica dos veces, un progreso que no avanza | **sí** |
| **Fallo real de aplicación** | 5xx, tiempo agotado, conexión rechazada | **sí** |
| **Fallo de infraestructura** | 502 y 503 del balanceador, conexión reiniciada durante el reemplazo de una instancia | **sí**, y además se cuenta aparte |

Las tres reglas que esto fija:

1. **Un 2xx con el estado equivocado es un fallo**, y es más grave que la
   lentitud: es un defecto de correctitud bajo carga.
2. **Un rechazo esperado no se mezcla con un fallo.** Un 429 del limitador es el
   sistema funcionando; contarlo como error oculta los errores de verdad.
3. **Un fallo de infraestructura se cuenta aparte, pero no se perdona.** En la
   Entrega 3 la API vive en un grupo administrado detrás de un balanceador: un
   502 mientras se reemplaza una instancia es una petición que el usuario perdió.
   Se registra en su propia columna porque su causa es distinta y porque es
   precisamente lo que C4 y G5 tienen que demostrar que **no** pasa, no para
   descontarlo del total.

## 2. Unidades y ventanas de agregación

| Magnitud | Unidad | Ventana |
| :--- | :--- | :--- |
| Latencia de petición | milisegundos enteros, medidos por el generador de extremo a extremo | la muestra |
| Rendimiento | peticiones por segundo, dos decimales | ventana de agregación de la corrida |
| Tasa de fallos | porcentaje con tres decimales | la misma ventana |
| Duración de etapa del flujo multimedia | milisegundos; segundos con tres decimales cuando pasa de 1 s | la muestra |
| Utilización de CPU y memoria | porcentaje, muestras de 60 s de Cloud Monitoring | media y máximo de la ventana |
| Profundidad y antigüedad de cola | tareas y segundos | máximo de la ventana |

**Una sola ventana de agregación por corrida:** desde el inicio de la primera
petición medida hasta el fin de la última, descontado el calentamiento. Se
publica en cada resumen.

> Esto corrige un defecto real: el resumen dividía el rendimiento por la
> distancia entre los *inicios* de las peticiones, que deja fuera la última,
> mientras publicaba como duración de la corrida una ventana que **sí** incluía
> el calentamiento. Dos ventanas distintas en la misma tabla.

**El calentamiento se descuenta siempre y se declara:** por omisión, la rampa
más 15 s. Una corrida no puede elegir su calentamiento después de ver los datos.

## 3. Número mínimo de observaciones

Un percentil es un estadístico de orden: el pX solo existe si hay suficientes
observaciones para que ese orden signifique algo. La cota es **n ≥ 1/(1−X)**.

| Percentil | Observaciones mínimas |
| :--- | ---: |
| p50 | 5 |
| p95 | 20 |
| p99 | 100 |

Por debajo de la cota **no se publica un número**: se publica `n/d (n=k)` y, si
la cifra importa, la mediana, el mínimo, el máximo y las observaciones en crudo.

> **Esto invalida todos los p95 por etapa del escenario 2 de la Entrega 2.** Los
> niveles subían 1, 2, 4, 8 y 12 vídeos. Ninguno llega a 20. No es que esos p95
> estuvieran mal calculados —además lo estaban—: es que no existían.
>
> **Consecuencia para G4:** o un nivel sube **20 vídeos o más**, o sus etapas se
> publican con mediana, mínimo, máximo y observaciones en crudo. Lo que no vale
> es un p95 de ocho subidas.
>
> El consumo HLS es el caso contrario: 25 estudiantes concurrentes por 34
> segmentos son 850 observaciones. Ahí el p95 es sólido.

## 4. Un solo estimador de percentil

**Rango más cercano con techo:** índice = ⌈p/100 × n⌉ − 1 sobre la muestra
ordenada. Implementado una vez en
[`capacity_criterios.py`](../scripts/capacity_criterios.py) y replicado
idénticamente en Go en `scripts/capacity_escenario2/main.go`.

No es el único estimador defendible. Es el único que usamos. Es monótono en p por
construcción, así que **p50 ≤ p95 ≤ p99 no puede fallar por el estimador**.

**El script que genera las tablas comprueba la monotonía y aborta si no se
cumple.** Si llega a dispararse, la causa está en los datos —muestras de ventanas
distintas mezcladas, o un conteo que cambia entre un percentil y otro—, y eso hay
que resolverlo antes de publicar.

Lo que había antes, y por qué salían invertidos:

| Archivo | p50 | p95 |
| :--- | :--- | :--- |
| `capacity_resumen_jtl.py` | ⌊p/100·n + 0,5⌉ − 1 | igual |
| `capacity_metricas_gcp.py` | — | igual que el anterior |
| `capacity_analisis_h3.py` | `v[n//2]` | `v[int(0,95·n)]` |
| `capacity_escenario2/main.go` | `v[n/2]` | `v[int((n−1)·0,95)]` |

La última fila con n=2 da índice 1 para el p50 e índice **0** para el p95: el p95
publicado era el valor mínimo de la muestra.

## 5. Destino y entorno registrados

**Una medición sin destino registrado no se publica.** Cada corrida escribe, junto
a sus datos crudos, un `destino.json` con la URL efectiva, el entorno, el commit
del código, el generador y la ventana UTC. El script que agrega se niega a
producir tablas si falta.

- **No hay destino por omisión.** `run_escenario1.sh`, `h2_nube.sh`, `h3_nube.sh`
  y la campaña del escenario 2 exigen que se declare. Heredar un destino en
  silencio es como se midió Compose y se publicó nube.
- **La clase del destino no se declara a mano:** se deriva del host. `api`,
  `minio`, `localhost` y `host.docker.internal` son `local`; el resto, `remoto`.
- **Las corridas que se publiquen como medición de la nube se ejecutan con
  `EXIGIR_NUBE=1`**, que hace fallar el resumen si el destino es local.
- La base de los derivados HLS **se declara**. Antes se adivinaba comparando la
  URL con la IP de la VM de la Entrega 2; con la IP cambiada —y cambia en cada
  recreación— la campaña habría medido los derivados de MinIO creyendo medir los
  del bucket.

## 6. Tiempo al primer cuadro

El enunciado es explícito: si se reporta tiempo al primer cuadro o
interrupciones, **deben medirse con un reproductor**.

**Regla: esas dos métricas solo se publican si las produjo un reproductor.**
Mientras no haya uno, no aparecen en ninguna tabla. Lo que esta campaña puede
sostener es *tiempo hasta el primer segmento descargado*, que se publica con ese
nombre y marcado como transporte.

Cuando se mida, el tiempo al primer cuadro es el intervalo entre asignar el
`master.m3u8` como fuente y el primer evento `playing`, con Chromium sin
ventana y hls.js, registrando `loadstart`, `loadedmetadata`, `canplay`,
`playing`, `waiting` y `stalled`. Se descompone en descarga del manifiesto
maestro, descarga de la lista de variante, descarga del segmento 0 y
decodificación inicial. **La decodificación inicial es la diferencia entre el
evento `playing` y la llegada del segmento 0, medida; no una constante.**

Lo implementa G4. Hasta entonces, la casilla de la rúbrica queda como pendiente
declarada, que es honesto, en vez de cubierta con una suma.

## 7. Patrón de consumo de segmentos

Se declara en la corrida y queda en la cabecera de cada archivo de evidencia.

| Patrón | Qué modela | Qué se puede publicar |
| :--- | :--- | :--- |
| `cadencia` | Estudiantes reproduciendo a 1x: buffer inicial de 2 segmentos y una pausa por segmento igual a su duración | latencia de segmento, margen de buffer, interrupciones |
| `maxima` | Descarga sin pausa: saturación de red y coste de egreso | latencia de segmento, tasa de transferencia |

Dos reglas:

1. **La duración del segmento se lee del `#EXTINF` de la lista**, no se supone.
   El código asumía 6,0 s; los segmentos duran ~7,06 s porque el GOP se calcula
   con 30 fps sobre un vídeo a 25 (nota técnica 24). El supuesto se equivocaba
   un 18 %.
2. **Con `maxima` no se publican margen de buffer ni interrupciones.** Sin
   cadencia no hay instante de reproducción que incumplir, así que cualquier
   cifra de esas dos sería inventada.

> **Lo que estaba mal:** el simulador recibía la cadencia y el número de
> estudiantes y **no usaba ninguno de los dos**. Un solo lector secuencial, sin
> pausas, leyendo tres segmentos. Los niveles declaraban 2, 10, 25, 50 y 100
> estudiantes con cadencia de 6,0 s, y el informe publicó esas cifras. Nunca hubo
> más de un estudiante ni una sola pausa.

## 8. Ninguna métrica derivada de una constante

**Prohibido publicar una métrica que sea una constante sumada, restada o
multiplicada a otra medición.** El resultado es la medición original desplazada,
y presentarla como otra cosa es afirmar algo que no se midió.

Las tres que había:

| Métrica publicada | Cómo se calculaba | Por qué no mide nada |
| :--- | :--- | :--- |
| Tiempo al primer cuadro, 48 a 52 ms | latencia HTTP **+ 45 ms** | Los 45 ms eran una constante; la cifra es la latencia de red con otro nombre |
| Margen de buffer, +5,99 s | **6,0** − latencia de segmento | Una constante menos una latencia de red; además el 6,0 era erróneo |
| Interrupciones, 0 eventos | un segmento tardando más de **6 s**, descargando a máxima velocidad | Cero estaba garantizado por aritmética, no observado |

Las tres aparecían juntas en la misma sección del informe, y las tres se
presentaron como medidas con un reproductor real.

## 9. Éxito, degradación y parada

Los umbrales siguen siendo los de los planes, que no se cambian aquí:
[escenario 1, §8](./escenario1.md#8-criterios-de-éxito-saturación-y-parada) y
[escenario 2, §7](./escenario2.md#7-criterios-de-éxito-saturación-y-parada).

Lo que añade la Entrega 3, porque la infraestructura es elástica:

- **Degradación durante el escalamiento.** Un escalamiento correcto no pierde
  peticiones. Un 502 o una conexión reiniciada mientras entra o sale una
  instancia es degradación, se reporta en su propia columna y, si pasa del
  0,1 %, el escenario no está sano aunque la latencia aguante.
- **Tiempo hasta que la capacidad nueva sirve.** Desde que se cruza el umbral de
  escalamiento hasta que la instancia nueva contesta en el balanceador. Se mide
  con el `X-Instance-ID` de las respuestas (nota técnica 19), y es la métrica que
  decide si la política de C6 llega a tiempo.
- **Las sondas no cuentan como carga.** Las peticiones de salud del balanceador y
  del autosanado se excluyen de todas las tablas; inflan el rendimiento y tienen
  una latencia que no es la de un usuario.
- **Parada inmediata, además de lo de los planes:** si el autosanado recrea una
  instancia durante una corrida, la corrida se descarta. No se mide una
  plataforma que cambió de forma a mitad de la medición; se anota por qué y se
  repite.

## 10. Lista de comprobación antes de publicar una cifra

Si cualquiera de estas falla, la cifra no sale del directorio de evidencia.

- [ ] La corrida tiene `destino.json`, y su clase es la que el informe afirma.
- [ ] El entorno declarado coincide con el destino registrado.
- [ ] La ventana de agregación está declarada, y es una sola.
- [ ] El calentamiento descontado está declarado, y se fijó antes de la corrida.
- [ ] Cada percentil publicado llega a su mínimo de observaciones.
- [ ] p50 ≤ p95 ≤ p99 en todos los juegos de la tabla.
- [ ] Ninguna métrica es una constante combinada con otra medición.
- [ ] El tiempo al primer cuadro y las interrupciones, si aparecen, los produjo
      un reproductor.
- [ ] El patrón de consumo está declarado y es compatible con lo que se publica.
- [ ] Los perfiles multimedia pasaron `generar_media_prueba.sh --verificar`.
- [ ] Las sondas de salud están excluidas.
- [ ] La evidencia no contiene credenciales, llaves ni secretos.

## 11. Qué de la Entrega 2 queda invalidado

| Cifra publicada | Estado |
| :--- | :--- |
| Escenario 1 completo (H2, H3) | **Vale.** Midió contra la nube, con destino identificable, miles de muestras y percentiles de un estimador monótono |
| Escenario 2: tasas de servicio de 12 a 124 vídeos/min | **No vale.** Son clips de 8 s; con perfiles reales un vCPU rinde 0,43 vídeos/min (nota técnica 24) |
| Escenario 2: p95 por etapa | **No vale.** Entre 1 y 12 observaciones, y con un estimador que se invertía |
| Escenario 2: tiempo al primer cuadro, margen de buffer, interrupciones | **No vale.** Derivadas de constantes |
| Escenario 2: estudiantes concurrentes y cadencia | **No vale.** Nunca se aplicaron |
| Escenario 2: recuento de segmentos y almacenamiento | **Se pasa un 20 %** por el GOP a 30 fps sobre un vídeo a 25 (para H3) |

**A7 vuelve a medir el escenario 2 contra la nube con estos criterios.** Ninguna
cifra de escenario 2 de la Entrega 2 entra en una comparación sin pasar por ahí.

---

## Cómo se verifica que esto se cumple

```bash
python scripts/capacity_criterios.py --autoprueba        # las reglas, en código
python scripts/capacity_criterios.py --destino <corrida> # qué midió una corrida
bash   scripts/generar_media_prueba.sh --verificar       # los perfiles declarados
go run ./scripts/capacity_escenario2                     # se niega a correr sin destino
```

Este archivo no contiene credenciales, llaves ni secretos.
