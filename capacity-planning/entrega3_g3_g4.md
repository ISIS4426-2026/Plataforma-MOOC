# G3 y G4: plan de las corridas comparadas de la Entrega 3

**Issues:** G3 [#223](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/223) (escenario 1) y G4 [#224](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/224) (escenario 2) · **Responsable:** `@tmichelldiaz` · **Desbloquean:** G7
**Estado:** propuesta para aprobar **antes** de la primera corrida. Una vez acordada, no se cambian estos números sin cambiarlos aquí primero y decir por qué (mismo trato que `escenario1.md` §13).
**Escrito el:** 2026-11-01. Los planes de los escenarios (`escenario1.md`, `escenario2.md`) y las reglas de medición (`criterios_de_medicion.md`, A6) **siguen vigentes**; este documento solo fija lo que cambia en la Entrega 3.

---

## 0. Lo que ya está listo y lo que falta

Listo: línea base de A7 (dos escenarios, código corregido `3affa11`), instrumentación y criterios de parada de G1 (recolector `capacity_metricas_gcp.py`, contraste del generador, gráficas), la VM generadora definida en Terraform (apagada), E1, E2 y E3 cerrados, y el entorno elástico desplegado con `97120ca`.

**Falta, medido hoy contra la nube** (la lista es el trabajo previo de §5):

| Hallazgo | Consecuencia |
| :--- | :--- |
| Las 200 cuentas de carga **no existen** en la base nueva (`carga.estudiante0001` responde 401) | cargar `scripts/seeds/capacity_data.sql` antes de nada |
| `cargar_cuentas_carga_nube.sh` apunta a `mooc-web-server` y a la IP `10.171.240.3`, que ya no existen (la base está en `10.171.240.7` y las VM de la API son de un grupo con nombres que cambian) | el reinicio de cuentas y la verificación en base de cada corrida no funcionan hoy |
| El `6379` de la cola solo lo abren las cuentas de la API y del worker (`queue.tf`) | la campaña del escenario 2 no puede leer la cola desde la VM generadora |
| La cuenta de la VM generadora no puede leer el secreto de la base | tampoco puede reiniciar cuentas ni verificar la base de datos |
| La campaña del escenario 2 mide el consumo HLS sobre los **mismos** vídeos que están en cola: con una cola atrasada, los niveles 2 a 4 de A7 no descargaron ni un segmento | hay que separar el consumo de la cola (§4.3) |
| El tiempo al primer cuadro sigue sin medirse (A6 §6 exige Chromium sin ventana + hls.js) | G4 lo implementa; es criterio de aceptación |

---

## 1. Cómo se compara: tres operandos, no dos

La línea base de A7 salió de **un portátil**, con un balanceador inexistente y una VM fija. Si G3 compara esa línea contra la nube elástica desde otro generador, el resultado mezcla arquitectura, generador y código. A7 ya lo advirtió (§1.4 de `COMPARACION.md`): el p95 y p99 globales **no son comparables entre generadores**.

Por eso se añade un operando de control, medido **desde la misma VM generadora y por el mismo balanceador**:

| Operando | Qué es | Para qué sirve |
| :--- | :--- | :--- |
| **A7** | Primera arquitectura, 1 VM fija, portátil como generador, `3affa11` | Lo que se publicó como línea base |
| **R** (referencia) | La arquitectura de hoy con la API **fijada en 1 instancia** (`min = max = 1`) y 1 worker | Aísla el generador, el balanceador y el código nuevo |
| **E** (elástica) | La arquitectura de hoy tal como está desplegada: API y workers de 1 a 3 | Lo que se quiere demostrar |

**Qué diferencia mide cada flecha:**

- **R frente a E** es el efecto de la elasticidad: mismo generador, mismo código, mismo balanceador, misma base. Esta es la comparación que se publica como «efecto de la arquitectura elástica».
- **A7 frente a R** mezcla generador, balanceador y los 8 cambios de código (§2.2). Se publica **declarada como tal**, y solo con las cuatro magnitudes que A7 §1.4 dice que sí se pueden comparar: rendimiento, latencia del servidor sobre conexión caliente, tasa de errores y utilización.

Fijar la API en 1 instancia se hace con `gcloud compute instance-groups managed update-autoscaling … --min-num-replicas=1 --max-num-replicas=1` y se restaura al terminar (lo mismo que ya hicimos para E1 y E2). No es un cambio de Terraform.

---

## 2. G3: escenario 1 sobre la arquitectura elástica

### 2.1 Lo que se mantiene constante (igual que A7)

Recorrido de 14 pasos, 64 % lecturas / 36 % escrituras, 3 sesiones por usuario, cuentas independientes con su token, `Idempotency-Key` única por corrida/usuario/sesión/operación, pausa 4000 ± 2000 ms en la serie A y 1000 ± 1000 ms en la B, rampa 30 s (60 s en los niveles de 100 y 200), calentamiento omitido (rampa + 15 s), JMeter 5.5 por digest, `us-east1`, Cloud SQL `db-custom-1-3840`. Los criterios de éxito, saturación y parada son los de `escenario1.md` §8.

### 2.2 Diferencias declaradas (cada una, posible causa de lo que se vea)

| Diferencia | A7 | R y E |
| :--- | :--- | :--- |
| Generador | portátil (`DESKTOP-…`), red doméstica | VM `e2-standard-4` en `us-east1-c`, misma región |
| Entrada | Nginx en la VM, TLS con sslip.io | balanceador HTTPS gestionado |
| Cómputo de la API | 1 VM fija | **R:** 1 instancia fija · **E:** 1 a 3 |
| Cola y sesiones | Redis en la VM worker | Redis en la VM de la cola, fuera de los grupos |
| Código | `3affa11` | `97120ca` (8 commits sobre `internal/`, `cmd/` y `migrations/`) |

De esos 8 commits solo uno toca el recorrido del escenario 1: la corrección de E2 añade **una sentencia** (el candado de transacción) a cada `POST /enrollments`, 3 por usuario sobre unas 42 peticiones. No se descuenta; se declara. Se comprobó en el diff (`git diff 3affa11 main -- internal cmd migrations`): el catálogo y el detalle del curso conservan la misma consulta (solo se refactorizó para añadir `GET /me/courses`), y el resto es del worker, de la descarga de archivos y de la configuración, que este recorrido no usa.

**Sobre el punto 7 del issue** (descontar el coste que A7 midió de snapshot de quiz y PITR): la línea base de A7 que usa G3 **ya lleva** esas correcciones, y R y E corren el mismo código, así que el coste está en los dos lados y se cancela solo (nota 20, «Corrección sobre cómo se usa ese número»). Lo que sí se informa es la cifra de A7 §2 (+8 ms de p50 y +34 % de CPU de la base) como contexto, no como resta.

### 2.3 Qué se corre

Las corridas son largas de poner en marcha y cortas de ejecutar (3,5 min cada una), así que **lo que importa es la repetición, no el número de niveles**.

| Serie | Qué | Usuarios | Operandos | Repeticiones |
| :--- | :--- | :--- | :--- | :---: |
| **A (escalera)** | Pausa 4–6 s, la de A7 | 1, 10, 25, 50, 100, 200 | E completa · R solo 100 y 200 | 1 en niveles bajos · **3** en 100 y 200 |
| **B (ritmo alto)** | Pausa 1–2 s, la de A7 | 100, 200 | R y E | **3** |
| **C (más allá de A7)** | Pausa 0,25–0,75 s con 200 usuarios | 200 | R y E | **3** |
| **S (sostenida)** | Corridas B-200 seguidas, con reinicio de cuentas entre una y otra, **durante unos 20 min** | 200 | E | 12 seguidas |

Por qué existen C y S:

- **C.** A7 ya mostró que 200 usuarios con 4–6 s de pausa no saturan la plataforma (103,5 pet./s con la base al 35 %), así que con la serie A sola no se encuentra el límite y no se puede decir si «se desplazó» (tarea 6 del issue). C sube el ritmo por usuario hasta unas 350 a 400 pet./s. **No tiene línea base de A7**, y se dice.
- **S.** Una corrida dura unos 3 a 4 minutos y el autoescalador de la API tarda más que eso en reaccionar: mira el promedio de peticiones por instancia, tiene una estabilización de 300 s y la VM nueva necesita unos 3 a 4 min en arrancar. En una sola corrida E **se verá como R**. Doce corridas seguidas dan un tramo de carga sostenida donde se ve el escalamiento y sus efectos, **y además doce repeticiones para informar la variación**. La dinámica fina del ciclo de escalamiento es de G5; aquí solo importa el efecto sobre la latencia y el rendimiento.

Si la nube no escala en S, eso es un hallazgo, no un fallo de la corrida.

### 2.4 Variación

Se informa por nivel y operando: media, desviación estándar y rango de las tres repeticiones de rendimiento, p50/p95/p99 de servidor y CPU de la base, **no un solo número**. En S, la tendencia por posición de la corrida (1.ª a 12.ª). Un nivel con una sola corrida no es resultado (`escenario1.md` §6).

### 2.5 Qué se compara y cómo se prueba la causa

| Medición | Fuente |
| :--- | :--- |
| p50, p95, p99, rendimiento, errores, *timeouts* por paso | JMeter (`.jtl` original) con `capacity_resumen_jtl.py` |
| Latencia del servidor sobre conexión caliente | las filas del `.jtl` sin `Connect` (la regla de A7 §1.4) |
| CPU y memoria de la API y de la base, conexiones, transacciones/s, instancias existentes y operativas | `capacity_metricas_gcp.py` |
| Balanceador: solicitudes, p95 por minuto | Cloud Monitoring, solo como contraste |
| Cola | **no aplica como carga** (el recorrido no encola trabajos); se registra como control y debe quedar en 0 |

**Análisis del cuello de botella:** se publica con el dato que lo sostiene, no con una impresión. La hipótesis a contrastar es que, con la API escalando de 1 a 3 instancias, el límite pasa a la base: PostgreSQL tiene 1 vCPU y `max_connections = 100`, y cada API y cada worker pueden abrir hasta 25 conexiones (`DB_MAX_OPEN_CONNS`), así que **3 instancias de API ya pueden pedir 75 conexiones**. Cualquier otra dependencia (Redis, balanceador) se mira con la misma tabla. Si no se desplaza, se dice.

### 2.6 Integridad (no solo códigos HTTP)

- Tras cada corrida, la verificación en base de siempre: 3 envíos por estudiante, `min = max = 3`, 0 envíos duplicados del mismo intento (`capacity_verificar_estado.sql`).
- El paso 11 del recorrido (envío duplicado con la misma clave) valida `submission_id` e intento idénticos.
- Se añade la comparación con la auditoría, que E2 mostró que sí detecta efectos duplicados: tras la serie, los `quiz.submitted` de las cuentas de carga deben ser exactamente los envíos aceptados, y un solo `enrollment.created` por cuenta.
- Toda corrida pasa el contraste del generador de G1 y las comprobaciones de A6 **antes** de publicarse. Una corrida donde el generador limita se rechaza y se repite.

---

## 3. Qué se guarda de cada corrida (bloquea el cierre)

El entorno se destruye en I6 y las métricas caducan, así que **al terminar cada corrida**, sin esperar al final de la sesión:

`resultados.jtl`, `resumen.txt`, `destino.json` (URL efectiva, commit, generador, ventana UTC, núcleos), `generador.csv`, `verificacion_bd.txt`, `metricas_gcp.json` y `serie_temporal.csv` de la ventana, y la gráfica de cuatro paneles (`capacity_graficas.py`). Una captura del tablero por operando. Sin credenciales, tokens ni IPs privadas en ninguna imagen (la lista de comprobación de G1 aplica).

---

## 4. G4: escenario 2 sobre la arquitectura elástica

### 4.1 Lo que se mantiene constante

Los tres perfiles reales de A5 (120, 600 y 1800 s, 1280×720, H.264 + AAC, ~174 kb/s, **declarados con su `ffprobe` adjunto**), la escalera HLS de 360p y 720p, `WORKER_CONCURRENCY=2` **por instancia**, niveles de subidas 1, 2, 4, 8 y 12 y de estudiantes 2, 10, 25, 50 y 100, patrón de consumo `cadencia` con buffer inicial de 2 segmentos (la descarga a máxima velocidad se corre aparte y se analiza aparte, tarea 5 del issue), `MAX_SEGMENTOS` acotado.

### 4.2 Diferencias declaradas

Las mismas de §2.2 más: **workers de 1 a 3** (concurrencia total efectiva de 2 a 6, registrada minuto a minuto durante toda la corrida: tarea 7), **el generador está dentro de la región**, así que la subida a Cloud Storage y la bajada de segmentos ya no miden un enlace doméstico (A7 §3.5 dijo que la etapa 2 medía el enlace del portátil y que el almacenamiento no quedaba descartado como cuello de botella) y la descarga desde Cloud Storage a una VM de la misma región no genera egreso.

### 4.3 Defectos de la campaña que se corrigen antes de medir

| Defecto (de A7) | Corrección |
| :--- | :--- |
| El consumo HLS toma los vídeos que están en la cola: con la cola atrasada, los niveles 2 a 4 no descargaron un segmento | antes de la corrida se dejan **vídeos ya procesados** de los tres perfiles; el consumo concurrente usa esos, mientras las subidas del nivel alimentan la cola. Las dos cosas ocurren a la vez, que es lo que pide la tarea 1, y el consumo deja de depender de la cola |
| El margen de buffer y las interrupciones salían fijos | ya corregido en código; se mide por primera vez |
| Etapas 4, 5 y 6 reconstruidas desde el registro del worker porque el sondeo no las alcanzaba | en la nube los workers son instancias que desaparecen: el sondeo de la campaña pasa a leer el estado de cada recurso en la base y los tiempos de cola de la propia cola, y el registro del worker queda como contraste |
| La campaña lee la cola con el inspector de Redis | se abre un acceso de **solo la VM generadora** al `6379` (§5), o se leen las métricas personalizadas de la cola; se decide en §6 |
| Tiempo al primer cuadro no medido | **G4 lo implementa** con Chromium sin ventana y hls.js, como dice A6 §6: asignar el `master.m3u8`, registrar `loadstart`, `loadedmetadata`, `canplay`, `playing`, `waiting` y `stalled`, y descomponer en manifiesto, lista de variante, segmento 0 y decodificación inicial (esta última medida como diferencia entre `playing` y la llegada del segmento 0) |

### 4.4 Qué se corre y cuánto dura

| | |
| :--- | :--- |
| Niveles | 0 (línea base) a 4, como en A7, con los mismos perfiles |
| Operandos | **E** (workers 1 a 3) y **R** (workers fijos en 1, para tener la referencia desde el mismo generador) |
| Duración | A7 tardó **111 min** con 2 vCPU y una demanda de unos 13 600 s de vCPU contra 13 330 disponibles. Con hasta 3 workers se espera bastante menos, pero el escalamiento tarda (el worker nuevo necesita unos 3 a 4 min en arrancar y el grupo tiene una estabilización de 900 s), así que se presupuestan **entre 60 y 120 min** por operando |
| Repeticiones | Los trabajos son pocos y largos (con 1800 s de vídeo, un trabajo son ~35 min de una vCPU), así que las etapas **no alcanzan 20 observaciones** por perfil (A6, nota 25). Se publican con mediana, mínimo, máximo y observaciones en crudo, **no con p95**. El consumo HLS sí lo permite (25 estudiantes × 34 segmentos = 850 observaciones) |

### 4.5 Medición por etapa (sin colapsarlas)

Autorización y confirmación de carga (API) → transferencia al almacenamiento → espera en cola → procesamiento → tiempo hasta `available` → manifiestos y segmentos. Más: trabajos completados, reintentos, fallos, profundidad y antigüedad de la cola, drenaje al terminar la generación de carga, y qué atendió la API frente a qué sirvió el almacenamiento.

### 4.6 Qué no se atribuye sin evidencia

Ninguna mejora de descarga o reproducción se atribuye al autoescalado de la API sin evidencia: el tráfico de segmentos **no pasa por la API**, así que lo esperable es que escalar la API no cambie nada ahí, y se compara contra R para decirlo con datos. El efecto de escalar los workers se analiza aparte: sobre la espera en cola, la tasa de procesamiento y el tiempo hasta `available`.

---

## 5. Trabajo previo, antes de la primera corrida

| # | Qué | Dónde | Toca la nube |
| :-: | :--- | :--- | :---: |
| 1 | Cargar las 200 cuentas de carga y el curso/quiz de la semilla funcional en la base | `scripts/seeds/capacity_data.sql` | sí (escribe datos de prueba) |
| 2 | Hacer que `cargar_cuentas_carga_nube.sh` encuentre una instancia de la API que exista y la IP actual de la base | `scripts/cargar_cuentas_carga_nube.sh` | no |
| 3 | Permitir a la VM generadora leer el secreto de la base **solo mientras exista** (o ejecutar el SQL desde una VM de la API) | `generator.tf` | sí (IAM; lo valida F1) |
| 4 | Abrir `6379` a la cuenta de la VM generadora, atada a `generator_enabled` | `generator.tf` | sí (red) |
| 5 | Adaptar `h3_nube.sh` para correr **desde la VM** y dejarlo en `docs/entrega3/` (no sobreescribir la evidencia de la Entrega 2) | `scripts/` | no |
| 6 | Separar el consumo HLS de la cola; sondeo por base; tiempo al primer cuadro con Chromium + hls.js | `scripts/capacity_escenario2/` | no |
| 7 | Regenerar los tres vídeos de perfil con su `ffprobe` (`scratch/` quedó vacío) | `scripts/generar_media_prueba.sh` | no |
| 8 | Ensayo corto de todo el camino desde la VM (una corrida de 3 usuarios, un subida de nivel 0) | VM generadora | sí |

El orden es: 2, 5, 6 y 7 (código y archivos, sin gastar nada) → una revisión → 3 y 4 en un PR de infraestructura con `apply` desde `main` → encender la VM, cargar cuentas (1), generar tokens (los tokens duran 24 h y se generan a ~7 s por cuenta, unos 24 min) y ensayar (8).

---

## 6. Decisiones que hay que tomar

1. **¿Se acepta el operando de control R?** Duplica corridas, pero es lo que permite decir qué parte de la diferencia es la arquitectura. Sin R, la comparación es contra un portátil.
2. **Cómo se verifica la base desde la VM generadora:** (a) secreto de la base legible por la cuenta de la VM mientras exista, o (b) ejecutar el SQL de reinicio y verificación desde una VM de la API por IAP, como en la Entrega 2. (a) es más simple y se puede quitar con la VM; (b) no toca IAM pero es más lento.
3. **Cómo lee G4 la cola:** (a) regla de firewall del `6379` para la cuenta del generador, atada a `generator_enabled`, o (b) solo las métricas personalizadas de Cloud Monitoring (`queue/pending`, `active`, `outstanding`, `oldest_pending_age_seconds`). (b) no abre nada nuevo, pero pierde el tiempo de espera **por trabajo**; (a) lo conserva.
4. **Coordinación del entorno con G5 (`@CrispisCas9`) y G6 (`@DiegoOrtizRuiz`).** Las tres campañas cambian el tamaño de los grupos y generan carga. Ninguna puede correr a la vez que otra, y los mínimos del autoescalador tienen que quedar restaurados al terminar cada una. Hay que acordar una agenda de ventanas.
5. **Presupuesto.** La VM generadora cuesta ~0,13 USD/h; el resto del coste es el entorno encendido y las instancias extra mientras duran las corridas. Hay que fijar un tope antes de empezar, y la cifra real se informa al cerrar (como hizo A7).

---

Este archivo no contiene credenciales, llaves ni secretos.
