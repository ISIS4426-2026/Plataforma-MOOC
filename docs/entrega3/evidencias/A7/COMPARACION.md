# A7 — Lo medido contra lo publicado ([#199](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/199))

Medición **retrospectiva**: el entorno de dos VMs de la Entrega 2, recreado el
2026-10-26 y medido contra la nube con los perfiles de A5 y los criterios de A6.

Las **dos corridas que G3 usa como linea base** llevan el codigo corregido
`3affa11`, el mismo que correra la arquitectura elastica, para que la diferencia
entre ambas sea puramente arquitectonica.

| | Escenario 1 | Escenario 2 |
| :--- | :--- | :--- |
| Imagen medida | `3affa11394ea` | `3affa11394ea` |
| Esquema | 8 migraciones | 8 migraciones, 19 tablas |
| PITR | activado | activado desde la creacion |
| Destino registrado | `https://34.24.52.111.sslip.io` → `remoto` | igual |
| Base de datos | `mooc-db-1`, `db-custom-1-3840`, IP privada | recreada, IP `10.171.240.5` |
| Generador | JMeter en Docker, en el portatil | campana Go en el portatil, cola y base por tunel IAP |
| Ventana UTC | 2026-10-26 | 2026-10-27T02:08:12Z – 03:59:18Z |

Hubo ademas una corrida del escenario 1 **con el codigo del tag**
(`1827cdc2ee24c9c785529f0443dc76c7f607e0f5`, 7 migraciones, sin PITR) que sirvio
para avalar el metodo y para medir el coste de las correcciones de la Entrega 1
(§2). Sus datos crudos no se conservaron.

> **Lo desplegado en la Entrega 2 no era el código del tag.** El `web.conf` de la
> VM traía `IMAGE_TAG=b393116bf172…`, **59 commits antes del commit del tag**. Los
> números publicados salen de tres estados distintos: `b393116b` en la nube para
> el escenario 1, Docker Compose en local para el escenario 2, y el tag como lo
> que el repositorio dice que fue la entrega. Esta línea base usa **el código del
> tag**, que es lo comparable con la Entrega 3 porque también partirá de un estado
> etiquetado.

---

## 1. Escenario 1: la plataforma reproduce; la red del generador no

### 1.1 Lo medido

| corrida | usr | n | pet./s | p50 | p95 | p99 | CPU web med./máx. | CPU SQL med./máx. | conx | tps |
| :--- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| A-nivel1 | 1 | 33 | 0,2 | 77 | 286 | n/d | 3,97 / 5,77 | 11,07 / 11,59 | 3 | 4,8 |
| A-nivel10 | 10 | 353 | 1,9 | 80 | 295 | 346 | 5,35 / 8,95 | 11,46 / 12,00 | 4 | 17,6 |
| A-nivel25 | 25 | 881 | 4,3 | 80 | 296 | 816 | 5,77 / 8,88 | 11,59 / 11,96 | 5 | 42,3 |
| A-nivel50 | 50 | 1 771 | 9,0 | 80 | 288 | 340 | 6,06 / 6,88 | 13,07 / 15,12 | 5 | 86,2 |
| A-nivel100 | 100 | 3 241 | 16,5 | 81 | 292 | 354 | 9,21 / 9,83 | 15,45 / 17,80 | 6 | 178,1 |
| A-nivel200 | 200 | 6 512 | 32,6 | 77 | 280 | 333 | 13,11 / 18,37 | 18,38 / 25,51 | 9 | 354,5 |
| B-nivel100 | 100 | 3 004 | 52,6 | 74 | 112 | 303 | 14,90 / 18,53 | 13,42 / 15,63 | 12 | 202,1 |
| B-nivel200 | 200 | 5 981 | **103,5** | 73 | 122 | 291 | 22,42 / 37,33 | 24,87 / 35,20 | 16 | 1 039,7 |

Cero fallos reales, cero fallos de validación funcional y cero *timeouts* en las
ocho. La verificación en la base da `OK` en todas: 3 envíos por estudiante,
`min = max = 3`, **0 duplicados** — la idempotencia del envío se sostiene a 200
usuarios concurrentes.

El `n/d` del p99 de la línea base es la regla de A6 funcionando: 33 muestras no
sostienen un p99, que necesita 100. La Entrega 2 publicó ahí `1 711 ms` con un
asterisco al pie; ahora la tabla lo dice sola.

### 1.2 Contra lo publicado

| | Entrega 2 (publicado) | A7 (retrospectiva) | |
| :--- | ---: | ---: | :--- |
| Peticiones medidas, nivel 200 | 6 473 | 6 512 | **igual** |
| Rendimiento, nivel 200 | 32,8 pet./s | 32,6 pet./s | **igual** |
| Transacciones/s en Cloud SQL | 342–356 | 354,5 | **igual** |
| CPU Cloud SQL máx. | 26,0–27,1 % | 25,5 % | **igual** |
| CPU web máx. | 15,0–33,9 % | 18,4 % | dentro del rango |
| p50 global | 94 ms | 77 ms | −18 % |
| p95 global | 380 ms | 280 ms | −26 % |
| **p99 global** | **1 365 ms** | **333 ms** | **−76 %** |

Los recuentos de muestras coinciden casi exactamente en todos los niveles (33,
352→353, 882→881, 1 770→1 771, 3 234→3 241, 6 473→6 512): el recorrido y la
mezcla son los mismos. El rendimiento y la carga de la base también. Lo único que
se mueve es la cola de la distribución, y se mueve mucho.

### 1.3 De dónde viene esa diferencia, probado

El informe de la Entrega 2 ya sospechaba que su cola era coste de conexión:
decía que *«el 100 % de las peticiones de 1,2–1,7 s»* abrían conexión nueva.
Quitando la variable, queda demostrado:

| Nivel 200 usuarios | Entrega 2 | A7 |
| :--- | ---: | ---: |
| Peticiones con conexión nueva | 800 / 8 400 = **9,5 %** | 800 / 8 400 = **9,5 %** |
| `Connect` p50 (TCP + TLS) | 260 ms | 206 ms |
| **`Connect` p95** | **1 284 ms** | **268 ms** |
| `Connect` máx. | 3 800 ms | 804 ms |
| Peticiones de 1,2–1,7 s | **161** | **0** |
| CPU máx. del generador | 233 % | 213 % |

Y la latencia del servidor, aislada del coste de conectar:

| Peticiones sobre conexión ya abierta, nivel 200 | Entrega 2 | A7 |
| :--- | ---: | ---: |
| n | 7 600 | 7 600 |
| p50 | 92 ms | 76 ms |
| **p95** | **117 ms** | **111 ms** |
| p99 | 415 ms | 153 ms |

**La plataforma se comporta igual: el p95 del servidor coincide dentro del 5 %.**
La fracción de conexiones nuevas es idéntica, así que el recorrido no cambió. Lo
que cambió es cuánto cuesta abrir una conexión desde el generador: el p95 de
`Connect` cae de 1 284 ms a 268 ms, y las 161 peticiones lentas desaparecen.

### 1.4 La regla que esto deja para G3 y G7

> **El p95 y el p99 globales del escenario 1 no son comparables entre
> generadores ni entre redes.** Incluyen el coste de establecer conexión, que es
> una propiedad del cliente y de su enlace, no de la plataforma.
>
> Lo que sí es comparable, y reproduce la Entrega 2: **rendimiento**, **latencia
> del servidor sobre conexión caliente**, **tasa de errores** y **utilización de
> infraestructura**.

Al comparar la Entrega 3 contra esta línea base hay que usar esas cuatro, o
declarar que el generador y su red son los mismos. Si no, se estaría atribuyendo
a la arquitectura elástica una diferencia que es del enlace de quien mide.

### 1.5 Información nueva: la Entrega 2 nunca midió la serie B

En `docs/entrega2/evidencias/H3/resultados/` no hay ninguna corrida `B-*`: la
Entrega 2 corrió el nivel de 200 tres veces en vez de subir el ritmo. Así que su
conclusión —*«máximo probado: 200 usuarios / 32,8 pet./s»*— describía el límite
del **guion**, no el de la plataforma.

Con pausas de 1–2 s en lugar de 4–6 s, los mismos 200 usuarios dan **103,5
pet./s**, 3,2 veces más, con:

- p95 de **122 ms**, mejor que el de la serie A;
- **1 040 transacciones/s** en Cloud SQL, el triple;
- CPU del servidor web al **37 %** y de Cloud SQL al **35 %**;
- 16 conexiones a la base, lejos del tope del pool (≈ 29);
- cero errores y cero fallos de validación.

**Sigue sin saturar.** Para G7: el techo de 32,8 pet./s que la Entrega 2 publicó
era un artefacto del tiempo de espera entre pasos, y el margen real es al menos
tres veces mayor. Para G4: si la Entrega 3 quiere encontrar el punto de
degradación, con pausas de 4–6 s no lo va a encontrar; hay que subir el ritmo o
el número de usuarios muy por encima de 200.

---

## 2. El coste de las correcciones de la Entrega 1, como número

Dos corridas del escenario 1 sobre **la misma infraestructura**, el mismo
generador y la misma red; lo único que cambia es el código y PITR.

> **Esto NO se resta en la comparación de G3.** La Entrega 3 elástica corre el
> mismo código corregido, así que el coste de las correcciones está en los dos
> lados y se cancela solo. La línea base de G3 es **`escenario1-1st-architecture`**, y
> su diferencia con la arquitectura elástica es puramente arquitectónica.
>
> Este número vale por otra cosa: cuantifica lo que costaron las correcciones de
> la Entrega 1, que es material para **H3** y para el informe de **G7**, no un
> término de una resta.

### 2.1 La medida limpia: latencia del servidor sobre conexión caliente

Nivel de 200 usuarios, n = 7 600 peticiones en cada lado, aislada del coste de
abrir conexión —que es propiedad del generador y no de la plataforma (§1.4)—:

| | Tag `1827cdc2` | Corregido `3affa11` | Coste |
| :--- | ---: | ---: | ---: |
| p50 | 76 ms | 84 ms | **+8 ms · +10,5 %** |
| p95 | 111 ms | 128 ms | **+17 ms · +15,3 %** |
| p99 | 153 ms | 180 ms | **+27 ms · +17,6 %** |

### 2.2 Segunda medida, independiente: la CPU de la base

A 200 usuarios y **con el mismo caudal de transacciones** (354,5 → 350,9 tps):

| | Tag | Corregido | |
| :--- | ---: | ---: | :--- |
| CPU Cloud SQL, media / máx | 18,38 / 25,51 % | **24,67 / 30,69 %** | **+34 % de media** |
| CPU servidor web, media / máx | 13,11 / 18,37 % | 12,00 / 15,55 % | igual |
| Conexiones a la base, máx | 9 | 10 | igual |
| Rendimiento | 32,6 pet./s | 32,5 pet./s | igual |

Que la CPU de la base suba un tercio **sin que suba el número de transacciones**
señala el origen: cada escritura cuesta más, no hay más escrituras. Eso es PITR
escribiendo registro de transacciones de forma continua, más las dos operaciones
extra por envío del snapshot de quiz.

Las dos medidas son consistentes y se obtuvieron por caminos distintos —el
generador por un lado, Cloud Monitoring por el otro—, así que el coste de las
correcciones está medido, no estimado.

### 2.3 Qué hay dentro de ese coste

| Corrección | Qué añade por petición |
| :--- | :--- |
| Snapshot de quiz (`000008`) | dos operaciones más por envío |
| PITR | registro de transacciones continuo en todas las escrituras |
| Separación de sesiones a la base lógica 1 | nada apreciable: en las dos corridas el Redis es el mismo y está en la VM worker |
| Middleware nuevo (CORS, `X-Instance-ID`) | coste fijo en **todas** las peticiones, que es lo que explica que suba también la mediana |

### 2.4 Lo que falta de esta corrida

La **serie B** (pausas de 1–2 s) del lado corregido: el proceso fue detenido por
falta de memoria en la máquina generadora justo al empezarla. La serie A, que es
la que se compara, está completa en los dos lados con sus seis niveles.

---

## 3. Escenario 2: medido contra la nube con perfiles reales

Primera vez que esta campaña se ejecuta contra GCP y con vídeos de 2, 10 y 30
minutos. Costó cinco intentos, y los cinco defectos que los invalidaron están en
la §5.

### 3.1 Lo que quedó medido

Medido con el **código corregido** `3affa11` y los perfiles reales de A5, en la
ventana 2026-10-27T02:08:12Z – 03:59:18Z.

| | Entrega 2 (publicado) | A7 |
| :--- | ---: | ---: |
| Pico de cola, nivel 4 | 12 tareas | **14 tareas** |
| Antigüedad máxima en cola | 2,91 s | **3 723 s · 62 min** |
| Procesamiento, perfil corto (120 s) | — | 170,2 s mediana (n=16) |
| Procesamiento, perfil medio (600 s) | — | 855,7 s mediana (n=10) |
| Procesamiento, perfil largo (1800 s) | — | 1 711,3 s (n=1) |
| Espera en cola, agregado (n=27) | — | **p50 974 s · p95 3 724 s** |
| Etapa 6, total hasta disponible | 4,259–6,385 **s** | **p50 1 325 s · p95 4 557 s** |
| Trabajos en estado terminal | 57/57 | **27/27 en `completed`**, 0 fallos, 0 DLQ |
| CPU del worker en la ventana | 6,9 % máx (solo alojaba Redis) | **96,31 % media, 99,97 % máx** |
| CPU del servidor web | — | 2,67 % media, 7,35 % máx |

El tiempo hasta disponible pasa de **cinco segundos a más de veinte minutos de
mediana**: tres órdenes de magnitud, y es la consecuencia directa de medir
perfiles reales en vez de clips de ocho segundos.

**El hallazgo no es el transcodificado, es la cola.** La espera en cola es el
**73 %** del p50 del tiempo hasta disponible. La demanda de la campaña fueron
9 720 s de vídeo, unos 13 600 s de vCPU a los factores medidos; 2 vCPU en los
6 666 s que duró la corrida dan 13 330 s. **La demanda excedió la capacidad de
principio a fin**, y de ahí salen a la vez el 96 % de CPU media y una cola que no
se recupera en ningún nivel.

Las etapas 4, 5 y 6 se reconstruyeron desde los registros del propio worker
(`duración_ms` por trabajo) y la hora de creación del objeto en el bucket, no
desde el sondeo de la campaña, que solo alcanzó a cronometrar 8 de los 27. Ver
[`escenario2-1st-architecture/etapas_reconstruidas.txt`](./escenario2-1st-architecture/etapas_reconstruidas.txt).

### 3.2 El factor de transcodificación no es una constante

La nota técnica 24 declaraba **1,2×**, medido en un portátil con un vCPU vía
Docker. En el worker real depende de la contención:

| Situación | Factor |
| :--- | ---: |
| Un trabajo solo en la VM (ffmpeg usa los dos núcleos) | **0,78×** (corto) · **0,89×** (largo) |
| Dos trabajos a la vez (un núcleo cada uno) | **1,38×** (corto) · **1,42×** (medio) |

La corrida con el código corregido lo confirma de forma independiente, con 27
trabajos en vez de una medida aislada: **1,39×** el perfil corto (n=16) y
**1,42×** el medio (n=10). El largo salió en 0,95× con una sola observación,
porque le tocó correr casi sin compañía al final del drenaje; es el mismo efecto
de contención, visto desde el otro lado.

### 3.3 Para C6 y B2: subir la concurrencia compra un 10 %, no el doble

Consecuencia directa de lo anterior, y es un argumento **medido** a favor de
escalar horizontalmente:

| | trabajos/min |
| :--- | ---: |
| `WORKER_CONCURRENCY=1` | 0,639 |
| `WORKER_CONCURRENCY=2` | 0,703 |
| **Ganancia** | **+10,1 %** |

ffmpeg ya paraleliza entre los núcleos disponibles, así que doblar la
concurrencia dentro de una VM de dos vCPU no dobla el rendimiento: reparte los
mismos núcleos. **El único palanca real es más instancias de worker**, que es
exactamente lo que hace el autoescalado de C6.

### 3.4 El drenaje sí se observó esta vez

La corrección de la nota 27 funcionó: **ninguna lectura de la cola falló**. La
cronología completa está en `drenaje_cola_verificacion.txt`, sin huecos, y el
túnel IAP no se reinició ni una vez en 1 h 51 min.

| | |
| :--- | :--- |
| Cola al cortar la inyección | 14 en espera, 2 activas |
| Al cerrar la ventana | **0 en espera**, 2 activas |
| Completadas | 29 → **44** |
| Observación | 1 h 9 min 7,9 s, sin errores de lectura |

El plazo se agotó con dos tareas en vuelo, así que **esa duración no es el tiempo
de drenaje**: es cuánto se observó. Las dos cerraron después y el log del worker
registra 27 de 27 completadas.

### 3.5 Lo que no quedó medido, y por qué

- **Las etapas 4, 5 y 6 de 19 de los 27 trabajos**, desde el sondeo de la
  campaña: no alcanzaron estado terminal dentro del presupuesto de su nivel. No es
  un fallo de la plataforma —procesó los 27—, es que un trabajo encolado detrás
  de otros catorce no cambia de estado durante una hora. Reconstruidas desde los
  registros del worker.
- **El consumo HLS de los niveles 2, 3 y 4.** No descargaron **ni un segmento**:
  sus vídeos seguían en la cola, así que no había manifiesto que pedir. Las
  latencias de segmento solo existen para los niveles 0 y 1 (p95 181 y 203 ms).
- **El margen de buffer y las interrupciones, en todos los niveles.** El
  simulador tomaba el instante de reproducción del inicio de la sesión en vez de
  de cuando se llena el buffer inicial, así que el margen salía fijo en −14,40 s
  y el **100 %** de los segmentos con plazo contaba como interrupción: 16 de 16 en
  el nivel 0, 80 de 80 en el nivel 1. Corregido en el código; no re-medido.
- **El tiempo al primer cuadro.** No se publica: solo vale si lo produce un
  reproductor (A6 §6). La campaña mide transporte y lo llama por su nombre.
- **La etapa 2 mide el enlace de subida del generador**, 0,71–2,14 MB/s, no la
  capacidad del bucket; y la descarga masiva, 3,51 MB/s, tampoco. En la Entrega 2
  la campaña corría dentro de Docker Compose y esa etapa medía un *loopback*
  local: de ahí los 36–48 MB/s. **No son comparables**, y el almacenamiento
  **no queda descartado** como cuello de botella: hay que repetirlo desde
  `us-east1`.

---

## 4. Parámetros constantes y diferencias declaradas

La tabla está en [`RUNBOOK.md`](./RUNBOOK.md) §12. Diferencias que esta campaña
añadió a la lista y conviene no perder:

| Diferencia | Valor en la corrida 1 | Nota |
| :--- | :--- | :--- |
| `transaction_log_retention_days` | 7 | `main` lo baja a 3 al activar PITR |
| Base lógica de las sesiones en Redis | **0**, junto a la cola de asynq | el código del tag no tiene `REDIS_SESSION_DB`; comprobado con `info keyspace` |
| Verificación del certificado TLS | desactivada en el cliente de la campaña (`InsecureSkipVerify`) | el coste del *handshake* sí se mide; la verificación no |
| `Origin` en las peticiones de la campaña | el propio destino medido | sin esta cabecera la API responde 403 `csrf_origin_rejected` |

---

## 5. Los seis intentos del escenario 2, y qué defecto destapó cada uno

Esto no es una bitácora de incidencias: es la explicación de por qué el criterio
*Análisis de capacidad — Escenario 2* se calificó **6.67/10**. Ninguno de estos
defectos puede manifestarse contra Docker Compose con clips de ocho segundos, así
que la Entrega 2 corrió esta misma campaña «con éxito» en minutos.

| # | Qué falló | Defecto encontrado |
| :--- | :--- | :--- |
| 1 | 403 en la primera petición que crea algo | La campaña **nunca enviaba `Origin`**, y la protección contra CSRF lo exige. Contra Compose el origen por omisión ya estaba permitido |
| 2 | 23 trabajos como «fallidos / DLQ» | Esperaba **6 minutos fijos** por trabajo, calibrados para clips. Un vídeo de 10 min tarda 12. Además las etapas 4-6 de esos trabajos salían **negativas**, y la campaña imprimía «Finalizado Exitosamente» |
| 3 y 4 | Pico de cola 0 con 12 subidas confirmadas | El inspector **se comía el error** y publicaba ceros; el drenaje escribía «la cola se ha vaciado totalmente» con 13 pendientes. Causa raíz: un `FLUSHDB` nuestro borró el registro `asynq:queues` (nota 27) |
| 5 | 21 trabajos sin medir pese a completarse | El detector de atasco miraba si avanzaba **un recurso**, no el sistema. En una cola saturada todo lo encolado parece colgado: la antigüedad máxima fue de 65 minutos |
| 6 | La evidencia se contradecía a sí misma en la misma página | **Las conclusiones estaban escritas fijas**, heredadas de la Entrega 2: afirmaban PUT «> 30 MB/s» con 0,71 medidos, «sin estados indeterminados» dos líneas después de «huérfanos: 2» y «0 interrupciones» en niveles que no descargaron ni un segmento. Además el **margen de buffer era negativo por construcción**: el reloj del instante de reproducción partía del inicio de la sesión |

Los seis están corregidos en `scripts/capacity_escenario2/`. El más importante
no es ninguno de los plazos:

> **Una lectura que falla no es un cero.** Si el código no distingue «no pude
> ver» de «vi que era cero», la evidencia acabará afirmando lo segundo. Es el
> mismo error que un p95 sobre dos muestras y que un tiempo al primer cuadro con
> 45 ms fijos: tres formas de publicar como hecho algo que nadie midió.

Y el sexto añade el caso límite: **una conclusión escrita a mano es una constante
que sobrevive a todo**. Sobrevivió al cambio de entorno, de código y de perfiles,
y acabó contradiciendo a la tabla que tenía encima. De ahi la regla que A6 ya
exige y que ahora el código cumple: **un veredicto que no puede fallar no es un
veredicto**.

---

Este archivo no contiene credenciales, llaves ni secretos.
