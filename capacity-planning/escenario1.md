# Escenario 1: Actividad académica concurrente — Plan de pruebas y capacidad

**Bloque:** H — Análisis de capacidad  
**Rúbrica:** Análisis de capacidad — Escenario 1 (10%)  
**Issue:** #132 (H2) — la ejecución y el análisis son el #133 (H3)  
**Dependencias:** B1 (dimensionamiento), G2 (Postman en la nube), H1 (instrumentación y generador)  
**Desbloquea:** H3 (ejecución, análisis y cuello de botella)  
**Estado del documento:** propuesta lista para revisión. Este plan define el recorrido, la mezcla, los niveles y los criterios **antes de la primera corrida**; queda acordado cuando el equipo lo aprueba en el pull request (ver §13). Después de acordado, H3 no cambia estos números: si hace falta cambiarlos, se cambia aquí primero y se anota por qué.

---

## 1. Qué mide este escenario

La actividad normal de un estudiante en una plataforma de cursos, muchos a la vez: ver el catálogo, abrir un curso, inscribirse, leer el contenido, registrar su avance y contestar un cuestionario.

Es **carga de la API y de la base de datos**. No incluye descargar video: el recorrido no pide manifiestos ni segmentos, porque cada reproducción íntegra sale como transferencia facturada (nota técnica 13; ~197 MB por video de 10 min, 36 USD por mil reproducciones). Por eso el recorrido completo solo intercambia JSON (unos pocos KB por petición) y el egreso de toda la campaña es del orden de decenas de MB. El consumo de video se mide en el escenario 2.

**Qué se quiere descubrir:** con cuántos estudiantes simultáneos deja la plataforma de responder dentro de los criterios de la §8, qué componente se satura primero (API, base de datos, conexiones) y cómo falla.

## 2. El recorrido

Cada usuario virtual es un estudiante con su propia cuenta y ejecuta este recorrido **3 veces seguidas** (3 «sesiones»; el porqué está en la §5). Cada paso lleva una validación que mira el **estado resultante**, no solo el código HTTP.

| # | Paso | Petición | Tipo | Qué se valida además del 200 |
| :-: | :--- | :--- | :---: | :--- |
| 1 | Catálogo | `GET /api/v1/courses?limit=20` | lectura | El curso del escenario aparece publicado en la lista |
| 2 | Detalle del curso | `GET /api/v1/courses/{id}` | lectura | `status = published` |
| 3 | Inscripción | `POST /api/v1/courses/{id}/enrollments` (con `Idempotency-Key`) | **escritura** | `status = active` |
| 4 | Verificar inscripción | `GET /api/v1/courses/{id}/enrollments/me` | lectura | `enrolled = true`, inscripción activa: lo que se escribió en 3 quedó guardado |
| 5 | Módulos | `GET /api/v1/courses/{id}/modules` | lectura | Trae al menos un módulo (y ya no da 403: el contenido exige inscripción activa, nota 6) |
| 6 | Unidades | `GET /api/v1/modules/{id}/units` | lectura | Trae al menos una unidad |
| 7 | Recursos | `GET /api/v1/units/{id}/resources` | lectura | Trae los 2 recursos de la unidad |
| 8 | Cuestionario | `GET /api/v1/resources/{id}/quiz` | lectura | 2 preguntas y **ningún campo `is_correct`** (la clave no viaja al estudiante) |
| 9 | Progreso: 2 latidos | `POST /api/v1/progress/heartbeat` × 2 (recurso de lectura y recurso de video, `completed=true`) | **escritura** ×2 | Responde el avance recalculado |
| 10 | Enviar el quiz | `POST /api/v1/quizzes/{id}/submissions` (con `Idempotency-Key`) | **escritura** | Calificación esperada para ese intento (ver §5), número de intento correcto, intentos restantes correctos |
| 11 | **Envío duplicado** | El mismo POST del paso 10, misma `Idempotency-Key` | **escritura** (repetición) | Devuelve **el mismo `submission_id`** y el mismo número de intento: no calificó dos veces |
| 12 | Avance tras el quiz | `GET /api/v1/progress/courses/{id}` | lectura | `total_count = 3` y `completed_count` esperado según el intento (2, 3, 3; ver §5) |
| 13 | Mis envíos | `GET /api/v1/quizzes/{id}/submissions/me` | lectura | Hay **exactamente un envío por intento** hecho hasta ahora: el duplicado del paso 11 no creó otro |

Esos son los identificadores de la semilla funcional (curso `d0000000-…0001`, quiz `fa000000-…0001` con 2 preguntas y `max_attempts = 3`, 3 recursos obligatorios). El guion los descubre navegando las respuestas (módulo → unidad → recursos) y solo el curso y el recurso del quiz se fijan por parámetro.

### Por qué el paso 11 es la comprobación de doble calificación

El criterio del enunciado es que el envío definitivo sea idempotente. La prueba lo verifica dos veces y por caminos distintos: por la **respuesta** (mismo `submission_id`, mismo intento) y por el **estado** (el paso 13 cuenta los envíos guardados; y, tras la corrida, una consulta directa a la base confirma que no hay dos envíos del mismo intento, ver §11).

## 3. La mezcla de lecturas y escrituras

Por sesión son **14 peticiones: 9 lecturas (64 %) y 5 escrituras (36 %)** (inscripción, dos latidos, envío del quiz y su repetición). Es constante por construcción: todos los niveles ejecutan exactamente el mismo recorrido, así que las proporciones no cambian al subir la carga y lo único que varía entre niveles es el número de estudiantes simultáneos. Comparar niveles con mezclas distintas mediría la mezcla, no la capacidad.

## 4. La autenticación: fuera del recorrido medido

**Decisión: el login no entra en el recorrido medido.**

Razón, con el número delante: el inicio de sesión está limitado a **10 intentos por minuto por IP** (`RATE_LIMIT_LOGIN_ATTEMPTS=10`, `RATE_LIMIT_LOGIN_WINDOW=1m`), y todo el generador de carga sale de una sola IP. Con 200 usuarios el login tardaría más de 20 minutos y, al llegar a 10, respondería 429. Si formara parte del recorrido, el informe mediría el limitador, no la plataforma.

Cómo se resuelve:
- **Antes de medir**, `scripts/capacity_login_tokens.sh` inicia sesión con las cuentas a un ritmo de una cada 7 s (≈ 8,5/min, bajo el límite) y guarda el token de cada una en un archivo local. Las sesiones duran 24 h (`SESSION_TTL`), así que se hace una vez al día. El archivo es `capacity-planning/datos/tokens.csv`, está en `.gitignore` y nunca se sube: son credenciales.
- **Durante la corrida**, cada usuario virtual manda su token en `Authorization: Bearer`.
- El costo de la autenticación en sí (bcrypt, sesión en Redis) queda para la variante de abajo.

### Variante separada: ráfaga de login

`escenario1_login_rafaga.jmx` lanza N inicios de sesión casi simultáneos desde una IP. **No mide capacidad: mide el limitador.** Resultado esperado con N = 15: 10 respuestas 200 y 5 respuestas 429 (probado en local, ver evidencia H2). Se ejecuta una vez, aparte de la escalera, y su resultado se reporta en una sección propia. Cuando el limitador rechaza, eso es **comportamiento correcto**, no una falla.

## 5. Cuentas, intentos y datos: sin conflictos artificiales

- **Un hilo = una cuenta.** Hay 200 estudiantes de carga (`carga.estudiante0001…0200`, sembrados por G1). Un nivel de N usuarios usa las cuentas 1…N. Ninguna cuenta la comparten dos hilos a la vez.
- **Intentos distintos.** El quiz admite 3 intentos por estudiante. Por eso el recorrido se repite exactamente 3 veces por usuario, y en cada sesión el intento tiene un resultado previsto distinto:

  | Sesión | Respuestas | Calificación esperada | `completed_count` esperado tras el paso 12 |
  | :-: | :--- | :-: | :-: |
  | 1 | Pregunta 1 bien, 2 mal | 50 (no aprueba) | 2 |
  | 2 | Ambas bien | 100 (aprueba: el quiz cuenta como completado) | 3 |
  | 3 | Ambas mal | 0 | 3 (lo aprobado queda aprobado) |

  El progreso solo llega a 3 de 3 cuando se aprueba el quiz: es la comprobación de que el avance es **válido**, no solo aceptado.
- **Claves de idempotencia únicas** por corrida, usuario, sesión y operación (`<corrida>-u<usuario>-s<intento>-<operación>`), para que una corrida nunca choque con la anterior ni un usuario con otro.
- **Reinicio entre corridas.** Cada corrida gasta el estado de las cuentas (inscripción activa, 3 intentos usados). `scripts/seeds/capacity_reset.sql` las deja como recién sembradas sin tocar las sesiones (el archivo de tokens sigue sirviendo) ni la auditoría (que es de solo añadir). **Sin reiniciar, una corrida repetida da 409 «sin intentos»**: eso se clasifica como rechazo de negocio y no como falla de capacidad (§9; se probó a propósito en local).
- **Los 4 estudiantes funcionales y los cursos de la semilla funcional no se tocan.**

## 6. Línea base, niveles y repetición

Todo nivel usa el mismo recorrido, la misma mezcla, los mismos tiempos de espera y los mismos criterios; solo cambia el número de usuarios.

| Nivel | Usuarios simultáneos | Para qué |
| :--- | :-: | :--- |
| **Piloto** | 3 | Validar guion y validaciones. Es el criterio de aceptación de H2 |
| **Línea base** | 1 | La latencia de cada paso sin competencia: el mejor caso, contra el que se compara todo |
| **Nivel 1** | 10 | Sub-saturación esperada |
| **Nivel 2** | 25 | |
| **Nivel 3** | 50 | |
| **Nivel 4** | 100 | |
| **Nivel 5** | 200 | Todas las cuentas sembradas |

Son 5 niveles crecientes más la línea base: el enunciado pide al menos tres.

**Repetición cerca del límite.** Cuando H3 identifique el nivel en que aparece la saturación (§8), se repite **3 veces** el último nivel sin saturación y el primero con saturación, con reinicio de cuentas entre repeticiones, y se reporta la dispersión entre repeticiones. Una sola corrida de un nivel no se considera resultado.

**Si el nivel 5 no satura** con los tiempos de espera del plan, se repiten los niveles 4 y 5 con tiempos de espera más cortos (1–2 s en vez de 4–6 s), y se reporta como serie aparte: la mezcla lectura/escritura sigue igual, pero el ritmo por usuario sube.

## 7. Patrón de inyección

| Parámetro | Valor | Razón |
| :--- | :--- | :--- |
| Herramienta | Apache JMeter 5.6.3 (Docker, `justb4/jmeter`), fuera de las dos VMs | H1 |
| Llegada de usuarios | Rampa lineal de **30 s** hasta el nivel completo (60 s para los niveles 4 y 5) | Evita un arranque en frío y simultáneo que mediría el arranque del pool de conexiones |
| Pausa entre pasos (*think time*) | Uniforme **4–6 s** | Un estudiante lee; sin pausa se mediría un ciclo de peticiones y no personas |
| Duración de un usuario | 3 sesiones × 14 pasos × ~5 s ≈ **3,5 min** | Sale de los 3 intentos del quiz; no se estira artificialmente |
| Calentamiento | Antes de la escalera, una corrida corta de nivel 1 que no se reporta; además el resumen **omite los primeros 45 s** de cada corrida | Cachés y pools fríos |
| Ventana medida | Desde que termina la rampa hasta que el primer usuario termina (≈ 3 min con todos activos) | Es el tramo con la concurrencia completa |
| Pausa entre corridas | 2 min como mínimo, más el reinicio de cuentas | Que la base y los pools se estabilicen |
| Carga aproximada | Nivel 5: 200 usuarios / 5 s ≈ **40 peticiones/s** | Referencia para leer los resultados |
| Orden | Ascendente. No se salta de nivel | Cada nivel confirma que el anterior estaba sano |

**Criterio de parada de la escalera:** se sube de nivel solo si el nivel actual no activó ningún criterio de parada de la §8.3.

## 8. Criterios de éxito, saturación y parada

Las latencias son las que mide el generador (de extremo a extremo, incluida la red hasta el servidor).

### 8.1 Éxito (un nivel «está sano»)
- **Fallos reales: 0** (tolerancia 0,1 %). Ver clasificación en la §9.
- **Fallos de validación funcional: 0.** Una respuesta 200 con el estado equivocado es un fallo, no un éxito.
- **Lecturas:** p95 ≤ 500 ms por paso.
- **Escrituras** (inscripción, latidos, envío): p95 ≤ 1 000 ms por paso.
- p99 global ≤ 2 s.

### 8.2 Saturación (el nivel «está saturado»)
Cualquiera de estas, sostenida durante **2 minutos**:
- Fallos reales > 1 %.
- p95 global > 1 s (el doble del umbral de las lecturas).
- El rendimiento (peticiones/s) deja de crecer al subir usuarios: la curva se aplana o baja.
- Fallos de validación funcional > 0 (un estado incorrecto bajo carga es un defecto de correctitud, más grave que la lentitud).
- Instrumentación (H1): CPU de `mooc-web-server` > 85 % sostenida, CPU de Cloud SQL > 80 % sostenida, o conexiones a la base cerca del tope de los pools (`DB_MAX_OPEN_CONNS = 25` por proceso, nota 11).

### 8.3 Parada inmediata
- Fallos reales > 5 % durante 30 s.
- p95 global > 5 s durante 1 min.
- Reinicio o caída de un contenedor de la API (OOM u otra causa).
- Errores de la base por conexiones agotadas.

Al parar se anota el motivo y el nivel; **no** se reintenta ese nivel sin cambiar algo y documentarlo.

## 9. Rechazos de negocio frente a fallos reales

Lo que hace útil al informe es no confundir «la plataforma dijo que no» con «la plataforma falló». El resumen de cada corrida (`scripts/capacity_resumen_jtl.py`) clasifica cada respuesta:

| Respuesta | Clase | Cuenta como fallo de capacidad |
| :--- | :--- | :---: |
| 2xx y la validación pasa | éxito | no |
| 2xx pero la validación falla (estado incorrecto) | **fallo de validación** | **sí**, y grave |
| 429 | rechazo de negocio: límite de tasa | no (solo aparece en la variante de login) |
| 401 / 403 / 404 / 409 / otros 4xx | rechazo de negocio o de configuración (cuenta sin inscribir, sin intentos, token vencido) | no: es un problema del entorno de la prueba y se corrige antes de repetir |
| 5xx | **fallo real** | **sí** |
| Timeout, conexión rechazada, sin respuesta | **fallo real** | **sí** |

En un recorrido bien armado no debería haber 4xx: cada usuario tiene su cuenta, sus intentos y su token. Si aparecen, primero se revisa el entorno (reinicio de cuentas, vigencia de tokens), no la plataforma.

## 10. Resultados por corrida

Cada corrida deja en `docs/entrega2/evidencias/H2/resultados/<etiqueta>_<fecha>/`:
- `resultados.jtl`: las muestras **originales**, una fila por petición (marca de tiempo, paso, código, latencia, hilo, mensaje de fallo). No contiene cabeceras, cuerpos ni tokens.
- `resumen.txt`: el resumen clasificado.
- `jmeter.log`: el registro de JMeter.

El `.jtl` es el dato primario: cualquier tabla o gráfica del informe de H3 debe poder rehacerse desde él.

Comando (una corrida):

```bash
BASE_URL=https://34.24.52.111.sslip.io bash scripts/run_escenario1.sh <usuarios> <etiqueta>
```

## 11. Procedimiento de una corrida

1. **Cuentas y tokens (una vez al día):** `BASE_URL=… CAPACITY_PASSWORD=… bash scripts/capacity_login_tokens.sh 200` (≈ 24 min a 7 s por cuenta; para niveles bajos basta pedir menos).
2. **Estado limpio:** ejecutar `scripts/seeds/capacity_reset.sql` en la base (mismo acceso que se usó para cargar `capacity_data.sql`).
3. **Correr** el nivel con `scripts/run_escenario1.sh` (para los niveles 4 y 5, con `RAMP_SECONDS=60`).
4. **Verificar el estado desde la base**, sin pasar por la API: `scripts/seeds/capacity_verificar_estado.sql`. Con N usuarios debe dar N estudiantes con exactamente 3 envíos cada uno (3 × N en total), N inscripciones activas y **0** envíos duplicados del mismo intento.
5. **Mirar la instrumentación de H1** (CPU de las VMs, conexiones y CPU de Cloud SQL) durante la ventana medida.
6. **Reiniciar cuentas** y pasar al siguiente nivel.

## 12. Riesgos y límites conocidos

- **Un solo generador, una sola IP.** Toda la carga sale de una máquina. Es lo que permiten las condiciones (H1), y es la razón por la que el login se saca del recorrido.
- **El generador puede ser su propio límite.** H1 midió ≈ 5,8 Mbps de bajada y poca memoria libre (≈ 600 MiB) en la máquina que ejecuta la prueba. El recorrido mueve ~100 KB/s en el nivel 5, muy por debajo de esa banda, pero la memoria sí es un riesgo con 200 hilos: cerrar otras aplicaciones y vigilar el uso del contenedor (`docker stats`) durante los niveles 4 y 5. Si el propio generador se satura, el resultado no vale.
- **La API todavía no reporta sus métricas de aplicación** hasta que se redespliegue el `docker-compose` de H1; mientras tanto la instrumentación de la API se apoya en las métricas de la VM y de Cloud SQL.
- **Datos de prueba en la nube:** las cuentas de carga usan la contraseña de prueba de la semilla. Al terminar las pruebas de capacidad conviene borrarlas o cambiarles la contraseña.
- **Sin descarga de video** a propósito (§1).

## 13. Acuerdo del equipo

Antes de la primera corrida de H3, el equipo revisa y aprueba en el pull request de H2 los puntos que condicionan todo lo demás:

- [ ] Recorrido y validaciones (§2)
- [ ] Mezcla 64 % lecturas / 36 % escrituras (§3)
- [ ] Login fuera del recorrido medido, y la ráfaga como variante (§4)
- [ ] Niveles 1 / 10 / 25 / 50 / 100 / 200 y repetición ×3 cerca del límite (§6)
- [ ] Patrón de inyección y ventana medida (§7)
- [ ] Umbrales de éxito, saturación y parada (§8)
