# Notas técnicas de la Entrega 3

Hallazgos que afectan a más de un issue. Cada uno dice dónde falla, para que no
se redescubra a mitad de una corrida.

**Léelo antes de empezar tu issue y anota antes de cerrarlo.** Si tu issue no
dejó nada aquí, dilo explícitamente al cerrarlo: así se distingue «no había nada
que anotar» de «no lo revisé». Es criterio de aceptación de los 44 issues.

El equivalente de la entrega anterior, [`docs/entrega2/NOTAS_TECNICAS.md`](../entrega2/NOTAS_TECNICAS.md),
acumuló 41 hallazgos y dice de sí mismo que la mitad salieron de errores que
costaron una tarde y se habrían evitado leyéndolos. Sigue vigente: lo de aquí no
lo reemplaza, lo continúa.

| | Hallazgo | Afecta a |
| :--- | :--- | :--- |
| 1 | [Un `apply` desde una rama borra el trabajo de otro, y no falla al hacerlo](#1) | **todos los de infra**: A7, B1, B2, B3, C1, C3, C5, F2, I6 |
| 2 | [La línea base del escenario 2 de la Entrega 2 no sirve](#2) | A5, A6, A7, G4, G7 |
| 3 | [La caché de sesiones es local a cada VM, y el logout no se propaga](#3) | A4, D1, E1, E2 |
| 4 | [La cola vive dentro de la máquina que el autoscaler puede retirar](#4) | B1, C5, C6, C7, G1, G6 |
| 5 | [Con un MIG, tres réplicas pueden migrar la base a la vez](#5) | A3, B3, C1, C5 |
| 6 | [Las capturas no se pueden rehacer después de I6](#6) | los 18 issues con criterio de captura, I4 |
| 7 | [El contrato de la API ya existe: el frontend no tiene que esperar a la infraestructura](#7) | D1, D2, D3, H1 |
| 8 | [El estado `available` del enunciado es `completed` en el código](#8) | D2, D3, G4, I1 |
| 9 | [Reproducir vídeos completos en las pruebas cuesta más que las propias VMs](#9) | A5, F3, G4, G6 |
| 10 | [Las métricas de Cloud Monitoring caducan](#10) | C6, G1, G5, G6, G7 |
| 11 | [Reproducir la CI en local pide dos contenedores y dos linters](#11) | **todos** |
| 12 | [`make lint` se rompe en Windows si la ruta lleva espacios](#12) | **todos los que linten en Windows** |
| 13 | [El contrato de la API arrastra dos avisos de Spectral](#13) | D1, H1, H2, H3 |
| 14 | [Una raya o una barra en un encabezado generan un ancla con guion doble](#14) | G7, H3, I1, I2, I3 |
| 15 | [La CI avisa de dos obsolescencias con fecha](#15) | H1, H2, H3 |
| 16 | [Solo el vídeo estaba cubierto, y por eso un PDF nunca llegaba al estudiante](#16) | D2, D3, E3, G4, H3 |
| 17 | [El quiz se congela por intento, y el envío directo sigue funcionando](#17) | D2, E2, G3 |
| 18 | [PITR cambia el coste con la escritura, no con el tamaño de la base](#18) | A7, F3, I2, I6 |
| 19 | [Tres sondas de salud, y cada una contesta a quien le pregunta](#19) | B3, C1, C3, C4, C5, E1, G5 |
| 20 | [La línea base no lleva las correcciones, y eso mete una variable en G3](#20) | A7, G3, G4, G7 |
| 21 | [Toda migración tiene que ser compatible con la imagen anterior](#21) | **cualquiera que añada una migración**: B3, C1, C5, D1 |
| 22 | [Dónde se aloje el frontend decide tres configuraciones a la vez](#22) | **D1**, A4, C3, E1 |
| 23 | [La cola y las sesiones comparten política de expulsión](#23) | B1, C6, F3 |
| 24 | [Transcodificar un vídeo real cuesta 1,2× su duración, y eso cambia la campaña](#24) | **G4, G6**, A7, C5, C6, F3 |
| 25 | [Antes de publicar una cifra hay once reglas, y están en código](#25) | **todos los que midan**: A7, G1, G3, G4, G5, G6, G7, H2, H3 |
| 26 | [El entorno de la Entrega 2 está a medias, y ninguna imagen del tag está publicada](#26) | **A7, B1, B2**, C1, C3, C5, G3, I6 |
| 27 | [Un `FLUSHDB` en el Redis de asynq mata la observabilidad y no se nota](#27) | **B1, C6, C7**, A7, G4, G6 |

---

<a id="1"></a>
## 1. Un `apply` desde una rama borra el trabajo de otro, y no falla al hacerlo

Heredado de la Entrega 2 (nota 16 de aquel documento) y **más peligroso ahora**:
en la Entrega 2 había dos personas tocando Terraform; en ésta hay dos franjas
completas sobre la misma configuración —@fredyxander con B1, B2 y C1–C4, y
@DiegoOrtizRuiz con B3, B4 y C5–C7— y el estado remoto es uno solo.

El estado remoto no sabe de ramas. Si aplicas desde una rama que no tiene los
recursos de otra, Terraform los considera sobrantes y los destruye, sin avisar
de que estaba trabajando alguien más.

**La regla sigue siendo: `terraform apply` solo desde `main`, nunca desde una
rama de trabajo.** En rama se valida con `fmt`, `init -backend=false` y
`validate`, que es justo lo que hace la CI.

A7 añade un caso nuevo: recrea el entorno **de la Entrega 2** para medir la línea
base retrospectiva.

> **Corrección (A7, 2026-10-25).** Esta nota decía que A7 necesitaba un prefijo de
> estado distinto. **Es al revés, y el prefijo nuevo no es viable.** El entorno de
> la Entrega 2 sigue existiendo a medias —VPC, NAT, las dos VMs apagadas, los
> buckets, Artifact Registry, 50 recursos en total— y está registrado en el
> prefijo `entrega2`. Un prefijo nuevo intentaría crear de cero recursos que ya
> existen, y los buckets tienen nombre único global: fallaría.
>
> **A7 usa `entrega2`** —el plan desde el tag lo confirma: 3 a crear, 0 a cambiar,
> **0 a destruir**— y **la Entrega 3 estrena `entrega3`** cuando B1 y B2 empiecen.
> Lo que hay que respetar es el orden: A7 corre y destruye **antes** de que
> ninguna rama de la Entrega 3 aplique Terraform. Con una sola arquitectura viva
> a la vez, un prefijo no mezcla nada.
>
> **Para B1, B2, B3, C1, C3, C5, F2 e I6:** vuestro primer `terraform apply` va
> con el prefijo `entrega3`. Antes de aplicar, `bash ./scripts/a7_preparar.sh`
> dice si A7 ya terminó: si el plan desde el tag propone destrucciones, es que
> alguien se adelantó.

<a id="2"></a>
## 2. La línea base del escenario 2 de la Entrega 2 no sirve

La retroalimentación del curso calificó el criterio *Análisis de capacidad —
Escenario 2* con **6.67/10** y lo invalidó por cuatro motivos concretos:

- Se midió **contra Docker Compose, no contra la nube**: el destino registrado es
  `http://api:8080`.
- Los perfiles declarados de **2, 10 y 30 minutos** eran clips de **8, 20 y 40
  segundos**.
- El **tiempo al primer cuadro** se reportó como latencia HTTP más **45 ms fijos**
  — una constante, no una medición.
- Hay **p95 por debajo del p50** en las tablas publicadas.

Consecuencia práctica: la Entrega 3 exige comparar contra la Entrega 2, y esa
comparación no se puede hacer con los números publicados. **A5** regenera los
archivos reales, **A6** fija las reglas de medición y **A7** vuelve a medir
contra la nube como medición retrospectiva.

No reutilices ninguna cifra de `capacity-planning/pruebas_de_carga_entrega2.md`
para el escenario 2 sin pasar por A7.

<a id="3"></a>
## 3. La caché de sesiones es local a cada VM, y el logout no se propaga

`docker-compose.prod.yml` levanta un contenedor `redis` **dentro de la propia VM
web**, y `internal/cache/session_cache.go` lo usa como caché de sesiones.

Con una sola VM no se nota. Con 1–3 réplicas sí:

- `Authenticate` (`internal/auth/service.go`) cae a PostgreSQL cuando la caché
  falla, así que **la sesión sobrevive** al cambiar de réplica. Eso funciona.
- `Logout` borra la entrada **solo de la caché local**. Otra réplica seguirá
  sirviendo esa sesión desde su propia caché hasta que expire el TTL. **Eso es un
  defecto de seguridad, no una molestia.**

Lo mismo aplica a `DeleteAllForUser` y a la suspensión de usuarios. A4 lo
resuelve apuntando la caché al servicio compartido de B1.

<a id="4"></a>
## 4. La cola vive dentro de la máquina que el autoscaler puede retirar

> **Resuelta en B1 (#200)** en cuanto a la cola: vive en `mooc-queue-server`,
> fuera de los dos grupos. **La advertencia del exportador sigue viva** y es de
> C6; ver más abajo.

`docker-compose.worker.yml` incluía el `redis` que hace de cola de asynq. En la
Entrega 2 eso era aceptable porque la VM worker era fija. En la Entrega 3 esa
máquina pertenece a un grupo elástico que puede destruirla, y el enunciado lo
prohíbe explícitamente: la cola debe estar fuera de ambos grupos. B1 la saca, y
el `REDIS_URL` del worker pasa a ser obligatorio en vez de tener `redis:6379`
por omisión.

Ojo con el efecto colateral, que **B1 no resuelve**: la señal de autoscaling de
workers (C6) se calcula sobre la cola. Si el exportador de esa métrica vive en
una instancia del grupo, el autoscaler se queda ciego justo cuando reduce a una
instancia. **Tiene que exportarse desde fuera de los grupos elásticos.**
`sa-queue-server` ya lleva `monitoring.metricWriter` para que la VM de la cola
pueda ser ese sitio, pero el exportador hay que escribirlo en C6.

<a id="5"></a>
## 5. Con un MIG, tres réplicas pueden migrar la base a la vez

En la Entrega 2 daba igual quién aplicara el esquema: había una sola VM. Un grupo
administrado que levanta hasta tres instancias a la vez puede lanzar tres
`migrate` en paralelo sobre la misma base.

A3 decide entre un paso explícito del despliegue antes de rotar el grupo o un
advisory lock de PostgreSQL. Hasta que A3 cierre, **no asumas que arrancar una
instancia es seguro**: coordínalo con quien tenga A3 abierto.

<a id="6"></a>
## 6. Las capturas no se pueden rehacer después de I6

I6 destruye el entorno. Después de eso no hay dónde volver a filmar un ciclo de
escalamiento, un health check expulsando una instancia o un recorrido de
estudiante.

Por eso 18 issues llevan un criterio que **bloquea el cierre** si falta la
captura. No es burocracia: relanzar una corrida de autoscaling solo para grabarla
cuesta dinero y tiempo, y el enunciado permite mostrar procesos largos de
ejecuciones previas **solo si sus registros completos están en el repositorio**.

**Antes de cerrar uno de esos issues, corre
`bash ./scripts/check_evidencias.sh`.** Lee la columna *Captura* del README de
evidencias y falla si algún directorio está vacío o si un archivo pasa de
10 MiB. Es criterio de aceptación de I3 y de I5, así que el tag `entrega-3` no
se crea con capturas faltantes — pero esperar a I5 para descubrirlo es tarde:
para entonces puede que el entorno ya no exista.

Convención: `docs/entrega3/evidencias/<clave>/capturas/`, PNG para pantallas,
consola y gráficas, vídeo corto solo cuando el movimiento es la prueba, nombre
`AAAA-MM-DD_descripcion.png`. Sin credenciales, tokens ni IPs privadas a la
vista. Lo que pase de ~10 MB va a la plataforma del vídeo y se enlaza, no al
repositorio.

<a id="7"></a>
## 7. El contrato de la API ya existe: el frontend no tiene que esperar a la infraestructura

`api/openapi.yaml` fija el contrato completo. D1 puede desarrollarse contra
Docker Compose local o contra la URL de la Entrega 2 y cambiar **solo la URL
base** cuando C3 publique el balanceador. Lo mismo vale para H1: el análisis
estático arranca con alcance Go y añade el frontend cuando D1 publique su primer
build.

Lo único que sí hay que coordinar pronto es el **origen** del frontend, porque
`CSRF_ALLOWED_ORIGINS` y la configuración de CORS dependen de dónde se aloje
(bucket detrás del mismo balanceador, o servido por las instancias de API). Esa
decisión es de D1 y la consume A4.

<a id="8"></a>
## 8. El estado `available` del enunciado es `completed` en el código

Heredado de la Entrega 2 (nota 4). El enunciado habla de que un recurso llega a
`available`; el código lo llama `completed`. Afecta a cualquier aserción, vista
de frontend o métrica que compare contra ese nombre.

<a id="9"></a>
## 9. Reproducir vídeos completos en las pruebas cuesta más que las propias VMs

Heredado de la Entrega 2 (nota 13). Con los perfiles **reales** que exige A5 —de
verdad 2, 10 y 30 minutos— el coste de transferencia y de transcodificación sube
respecto de lo que se midió con clips de segundos.

Estímalo **antes** de lanzar la corrida grande, no después: es tarea explícita de
A5 y entra en la estimación de F3.

<a id="10"></a>
## 10. Las métricas de Cloud Monitoring caducan

La línea de tiempo de instancias, la métrica personalizada de cola y los paneles
no estarán disponibles semanas después de la corrida, y desaparecen del todo
cuando el proyecto pierde los recursos en I6.

Exporta los datos crudos de cada corrida al repositorio en el momento, no confíes
en volver a la consola. G1 define el formato; G5 y G6 dependen de que esos datos
existan para reconstruir la línea de tiempo del ciclo.

<a id="11"></a>
## 11. Reproducir la CI en local pide dos contenedores y dos linters

Sin ellos, una corrida local parece rota cuando no lo está:

- **Sin Redis**, los 8 tests de `internal/worker` fallan con
  `dial tcp [::1]:6379: connection refused`. No es un fallo del código.
- **Sin `DATABASE_URL`**, `TestMigrationsRunCleanlyOnPostgres` **se salta** en
  vez de ejecutarse: el paquete sale en verde sin haber probado las migraciones
  contra una base real.
- **Sin `golangci-lint` ni `spectral`**, `make lint` no corre.

```bash
docker run -d --rm -p 6379:6379 redis:7-alpine
docker run -d --rm -p 5432:5432 \
  -e POSTGRES_USER=moocuser -e POSTGRES_PASSWORD=moocpassword -e POSTGRES_DB=moocdb \
  postgres:16-alpine
export DATABASE_URL='postgres://moocuser:moocpassword@127.0.0.1:5432/moocdb?sslmode=disable'
export REDIS_URL='127.0.0.1:6379'
go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8
npm install -g @stoplight/spectral-cli
```

Las versiones están fijadas en `.github/workflows/ci.yml`: golangci-lint
**v1.64.8**, Terraform **1.16.4**, Go **1.24**. Si alguien sube una en el código
y se olvida del workflow, el trabajo de Terraform lo detecta.

<a id="12"></a>
## 12. `make lint` se rompe en Windows si la ruta lleva espacios

```
bash: D:\Fredy\Uniandes\Materias\Cloud: No such file or directory
make: *** [lint] Error 127
```

El objetivo invoca `./scripts/lint.sh` y `make` no cita la ruta del proyecto.
No es un problema del script: `bash ./scripts/lint.sh` funciona. Afecta a
cualquiera cuyo checkout esté bajo una ruta con espacios, que en este equipo es
`…/Cloud architecture/…`.

Lo mismo vale para la nota de la Entrega 2 sobre `core.longpaths`: Windows sigue
siendo el entorno frágil de este repositorio.

<a id="13"></a>
## 13. El contrato de la API arrastra dos avisos de Spectral

```
api/openapi.yaml
 1060:9  warning  operation-description  paths./modules/{module_id}.put
 1196:9  warning  operation-description  paths./units/{unit_id}.put
```

Son **warnings, no errores**, así que la CI pasa. Pero son deuda real del
contrato: dos operaciones de edición sin `description`, justo las que D1 va a
consumir para la autoría de estructura. Entra en el inventario de H3 como deuda
de documentación, y se arregla en cualquier momento con dos líneas.

<a id="14"></a>
## 14. Una raya o una barra en un encabezado generan un ancla con guion doble

GitHub construye el ancla quitando la puntuación y cambiando **cada espacio** por
un guion, no cada racha de espacios. Así que `— ` y ` / ` dejan dos espacios y,
por tanto, **dos guiones**:

| Encabezado | Ancla real |
| :--- | :--- |
| `# Informe … y Capacidad — Entrega 2` | `#informe--y-capacidad--entrega-2` |
| `#### Sonda … (HTML5 / Media Source Extensions)` | `#sonda--html5--media-source-extensions` |

Tres índices de `capacity-planning/pruebas_de_carga_entrega2.md` estaban rotos
por esto y se corrigieron al abrir la Entrega 3. **Un cuarto apuntaba a un
encabezado que vive dentro de un bloque de código**, que no genera ancla ninguna.

Afecta a cualquiera que escriba un documento largo con índice: G7, H3, I1, I2 e
I3. Comprueba las anclas antes de dar el documento por cerrado; un índice roto no
lo detecta la CI.

<a id="15"></a>
## 15. La CI avisa de dos obsolescencias con fecha

En toda ejecución de `.github/workflows/ci.yml`:

- `actions/checkout@v4` y `hashicorp/setup-terraform@v3` declaran **Node.js 20**,
  ya obsoleto; el runner los fuerza a Node.js 24. Se arregla subiendo a
  `actions/checkout@v5` y `hashicorp/setup-terraform@v4`.
- La etiqueta `ubuntu-latest` **migra a Ubuntu 26 el 19 de octubre de 2026**. El
  salto de imagen puede cambiar versiones de herramientas del sistema sin aviso;
  fijar `ubuntu-24.04` lo hace predecible mientras dura la entrega.

Hoy no rompen nada, pero H1 toca este workflow para añadir el análisis estático,
así que es el momento barato de atenderlo. Si no se atiende, entra al inventario
de H3 con su fecha.

<a id="16"></a>
## 16. Solo el vídeo estaba cubierto, y por eso un PDF nunca llegaba al estudiante

El worker enrutaba **cualquier** recurso a la canalización HLS, así que a
`ffprobe` se le pedía encontrar un flujo de vídeo dentro de un PDF. Respondía
«file carries no video or audio stream», que es `ErrUnprocessable`, o sea un
fallo permanente: el recurso quedaba en `failed` y nunca estaba disponible.

Sobrevivió a la Entrega 1 y a la Entrega 2 porque **nada lo ejercitaba**: ni el
E2E, ni las colecciones de Postman, ni la semilla subían algo que no fuera mp4.
Lo encontró la revisión de la Entrega 1 y se corrigió en la rama
`fix/entrega1-review-findings`: el worker enruta ahora los documentos por una
rama propia, y el E2E y Postman cubren el caso.

Tres cosas que conviene llevarse de esto:

- **D2 y D3** pueden ya contar con que un PDF o una imagen llegan a `completed`.
  Antes del arreglo, la vista de «estado de procesamiento» habría mostrado
  siempre un fallo para los documentos.
- **Ninguna petición de Postman espera a que el worker termine.** La colección no
  podía detectar este tipo de defecto y sigue sin poder: el guardián es el tramo
  del E2E.
- **Una rama nueva en una canalización compartida necesita su caso de
  compatibilidad hacia atrás.** Un trabajo ya encolado no lleva `task_type`, y si
  el `switch` no trata la cadena vacía como audiovisual, el redespliegue cambia
  el significado de los mensajes que ya están en la cola.

El tramo nuevo del E2E escribe en `docs/entrega3/evidencias/correcciones-entrega1/`.
`docs/entrega2/evidencias/G3/` es el registro congelado de lo que se calificó y
no se sobrescribe.

<a id="17"></a>
## 17. El quiz se congela por intento, y el envío directo sigue funcionando

Hay dos caminos para presentar, y los dos valen:

- **Con intento abierto** (`POST /quizzes/{id}/attempts`): el cuestionario se
  congela al abrirlo y la nota sale por lo que el estudiante vio. Admite
  guardado parcial y, si el quiz tiene `time_limit_seconds`, vence.
- **Envío directo** a `/submissions`, sin abrir nada: es como funcionaba todo
  antes, es lo que hace el arnés E2E, y **sigue funcionando igual**. También
  congela el cuestionario, pero en el instante del envío.

En los dos casos, editar el quiz después deja de alterar una nota ya
registrada. Antes no era así, y eso era lo que el revisor marcó.

Para **D2**: la vista del estudiante debería abrir el intento al entrar al quiz
y guardar al cambiar de pregunta. Recuperar el avance con
`GET /attempts/current` es lo que cubre el caso de cerrar el navegador. Un
intento vencido se devuelve igualmente con `expired: true`, para poder decir
«se te acabó el tiempo» en vez de «no habías empezado».

Dos detalles que ahorran sorpresas: abrir el intento **dos veces devuelve el
mismo**, y un envío fuera de plazo **no consume intento**.

<a id="18"></a>
## 18. PITR cambia el coste con la escritura, no con el tamaño de la base

`point_in_time_recovery_enabled` está ahora en `true`, que es lo que baja el RPO
de 24 horas a prácticamente cero. El RTO ya estaba en unos 9 minutos, medido en
C2.

Lo que conviene vigilar: el registro de escritura continuo se factura por
almacenamiento y **crece con la actividad de escritura**, no con el tamaño de la
base. Sobre un volumen académico son céntimos, pero una campaña de carga escribe
mucho más que el uso normal. Entra en la estimación de **F3** y conviene mirar
el consumo real después de las corridas de G3 y G4.

Dos avisos operativos: activar PITR **reinicia la instancia**, así que no se
toca con una campaña en marcha; y **A7 recrea desde el tag `entrega-2`**, que
lleva `false`, de modo que la línea base retrospectiva no queda contaminada por
este cambio.

<a id="19"></a>
## 19. Tres sondas de salud, y cada una contesta a quien le pregunta

| Ruta | Quién la mira | Prueba |
| :--- | :--- | :--- |
| `/api/v1/live` | autohealing del grupo (C1, C5) | nada |
| `/api/v1/ready` | health check del balanceador (C3, C4) | la base, más la bandera de arranque |
| `/api/v1/health` | Compose, arnés E2E, Postman | la base, con detalle por dependencia |

**No las intercambies.** El grupo administrado *destruye y recrea* la máquina
que falla su comprobación; el balanceador solo deja de enviarle tráfico. Si el
autohealing mirara `/ready`, un parpadeo de Cloud SQL haría fallar a las tres
réplicas a la vez y el grupo las recrearía todas: un problema transitorio de una
dependencia compartida convertido en una caída completa.

`/api/v1/health` se queda como estaba porque el E2E y Postman afirman sobre
`services.database`. Es la vista interna; las otras dos no revelan dependencias
ni topología, y hay una prueba que lo fija.

**Para C4 y G5:** toda respuesta lleva `X-Instance-ID`, un resumen corto que
distingue réplicas sin delatar el nombre de la máquina. El nombre completo va en
cada línea de log bajo la clave `instance`. Con eso se demuestra el reparto y se
busca en Cloud Logging por réplica.

**Para B3, C2 y C6:** el log `api server ready` trae `startup` y el criterio con
el que se decide que la instancia está lista. En local son **93 ms**, que es
solo la parte de aplicación. El arranque que calibra el autoscaler —VM, imagen,
secretos, contenedores— lo mide B3. Si el periodo de inicialización se fija por
debajo del arranque real, el autoscaler cuenta como sobrecargada una réplica que
todavía se está montando y pide otra.

<a id="20"></a>
## 20. La línea base no lleva las correcciones, y eso mete una variable en G3

**A7 despliega el código del tag `entrega-2` (`1827cdc`)**, no el de `main`. Es
deliberado: desplegar el código corregido convertiría la línea base en otra
cosa y la comparación de G3 y G4 dejaría de significar nada. Las imágenes del
tag siguen en Artifact Registry, que I6 conservó justo para esto.

Lo que sí entra en A7, y debe entrar, son **A5** (los archivos de 2, 10 y 30
minutos reales) y **A6** (destino registrado, percentiles comprobados, TTFF
medido). Sin ellos reproducirías la misma medición inválida que costó un tercio
del criterio del escenario 2. A7 no es «repetir la Entrega 2 tal cual»: es el
sistema de la Entrega 2 bajo la disciplina de medición nueva.

### La variable suelta

Tres cambios ya mergeados meten coste **por petición** en el camino que miden G3
y G4, y la línea base no los tendrá:

| Cambio | Dónde pesa |
| :--- | :--- |
| Snapshot de quiz | `Grade` arma el snapshot en SQL y borra el borrador: dos operaciones extra por envío. El escenario 1 envía dos veces por usuario (pasos 10 y 11) |
| PITR | registro de escritura continuo en Cloud SQL, y el escenario 1 es mitad escrituras |
| Enrutado de documentos | sin efecto: el escenario 2 es solo vídeo |

Si en G3 sube la latencia de escritura, **sin medir esto no se puede decir si
fue la arquitectura elástica o el snapshot de quizzes**. El enunciado lo prohíbe
expresamente: *«mantener constantes los parámetros ajenos al cambio estudiado;
cualquier diferencia deberá declararse y discutirse como posible causa de los
resultados»*.

### Qué se decidió

**Medirlo, no solo declararlo.** En el mismo encendido de A7, después de la
corrida de línea base, se despliega la imagen de `main` sobre la **misma
infraestructura de dos VMs** y se repite el escenario 1. Eso aísla el coste de
las correcciones sin mezclarlo con el cambio de arquitectura: **+8 ms de p50 y
+34 % de CPU en Cloud SQL al mismo caudal de transacciones** (medido, ver
[`evidencias/A7/COMPARACION.md`](evidencias/A7/COMPARACION.md) §2).

> **Corrección sobre cómo se usa ese número.** No se descuenta en la comparación
> de G3. La línea base de G3 es la corrida **corregida**, y la Entrega 3 elástica
> corre ese mismo código, así que el coste de las correcciones está en los dos
> lados y se cancela por construcción. Esa diferencia es un hallazgo para **H3**
> y para explicar el 6.67 en **G7**, no un término de una resta.

Esa segunda corrida es además la primera oportunidad de ejecutar los pasos del
E2E que no se han podido probar: documentos, imágenes y el intento de quiz
congelado.

<a id="21"></a>
## 21. Toda migración tiene que ser compatible con la imagen anterior

Las migraciones se aplican **al arrancar la instancia**, protegidas por un
advisory lock de transacción (`scripts/migrate.sh`). Tres instancias simultáneas
aplican el esquema una sola vez y ninguna falla; está probado en
[`evidencias/A3`](./evidencias/A3/README.md), junto con la demostración de que
el aplicador anterior hacía fallar a dos de las tres.

**La contrapartida, que es lo que hay que recordar al escribir una migración:**

> Durante una rotación del grupo conviven la imagen vieja y la nueva. La
> instancia nueva aplica la migración mientras las viejas siguen sirviendo
> contra el esquema ya cambiado.

Por tanto:

| Se puede | No se puede |
| :--- | :--- |
| Añadir una tabla | Borrar una tabla o columna que el código viejo lea |
| Añadir una columna anulable o con `DEFAULT` | Renombrar una columna |
| Añadir un índice | Añadir un `NOT NULL` sin valor por defecto |
| Relajar una restricción | Endurecer una restricción que el código viejo viole |

Un borrado o un renombrado se hace **en dos despliegues**: primero dejar de
usarlo en el código, después quitarlo del esquema. La migración `000008` cumple
la regla: todo lo que añade es anulable o una tabla nueva.

**Para B3:** el arranque ejecuta `migrate.sh` y después `migrate.sh --require`.
La segunda parece redundante y no lo es: si el esquema no trae lo que la imagen
necesita, sale con código 1 y la API no se levanta. Mejor una instancia que no
arranca que una que atiende contra una base que no corresponde.

Dos trampas al reproducirlo en Windows: el montaje de Docker necesita la ruta de
Windows (`cygpath -w`), y la copia de trabajo puede tener CRLF aunque el
repositorio guarde LF, lo que dentro del contenedor da `$'
': command not
found`.

<a id="22"></a>
## 22. Dónde se aloje el frontend decide tres configuraciones a la vez

La cookie de sesión es `SameSite=Lax` por omisión, y **`Lax` solo viaja en
navegaciones de primer nivel**. Un frontend servido desde *otro origen* que la
API no podría autenticarse: su `fetch` saldría sin cookie. El login respondería
200, dejaría su cookie, y la siguiente petición llegaría sin sesión.

| Dónde se aloje | Qué hace falta |
| :--- | :--- |
| **Detrás del mismo balanceador** ← recomendado | nada. Mismo sitio, `lax` basta, sin CORS |
| En otro origen | las **tres** juntas: CORS con credenciales, `SESSION_COOKIE_SAMESITE=none`, y el origen en `CSRF_ALLOWED_ORIGINS` |

**Faltando cualquiera de las tres, el frontend no se autentica.** Es la clase de
fallo que se diagnostica mal, porque cada pieza por separado parece correcta.

**Para D1:** elegir el mismo balanceador no es solo comodidad. Con `none` el
navegador deja de aportar nada contra CSRF y la protección descansa entera en la
comprobación de origen. Un fallo de configuración en esa lista pasa de ser una
molestia a ser la única barrera que faltaba.

El middleware de CORS ya está (`internal/http/middleware/cors.go`): comparte
lista con CSRF, nunca devuelve comodín —incompatible con `Allow-Credentials`— y
expone `X-Request-ID` y `X-Instance-ID`, que son las que C4 y G5 necesitan para
demostrar el reparto entre réplicas.

<a id="23"></a>
## 23. La cola y las sesiones comparten política de expulsión

Las sesiones viven en la base lógica **1** de Redis y la cola de asynq en la
**0** (`REDIS_SESSION_DB`). Eso separa lo que se puede inspeccionar, medir y
vaciar sin tocar lo otro — un `FLUSHDB` equivocado se lleva una sola cosa.

**Lo que no separa es la memoria.** La política de expulsión es de la instancia
entera, no de la base. Y asynq **exige `maxmemory-policy noeviction`**: un
trabajo expulsado es un trabajo perdido.

Consecuencia para **B1**: con las dos cosas en el mismo servicio, el control de
memoria de las sesiones es **su TTL**, no la expulsión. Si las sesiones llenaran
la memoria, lo que rompe es la cola — y rompe en silencio, porque un `LPUSH`
rechazado se ve como un trabajo que nunca llegó.

Dos salidas, a decidir en B1: dimensionar con holgura y vigilar el uso de
memoria, o darles instancias separadas. Lo segundo cuesta más y lo primero
exige una alerta; lo que no vale es no elegir.

<a id="24"></a>
## 24. Transcodificar un vídeo real cuesta 1,2× su duración, y eso cambia la campaña

Medido, no estimado: la escalera real (360p + 720p) sobre el perfil corto, en
**1 vCPU**, tarda **141 s para 120 s de vídeo**.

| Perfil | Transcodificación en 1 vCPU |
| :--- | ---: |
| Corto (2 min) | ~2,4 min |
| Medio (10 min) | ~12 min |
| Largo (30 min) | **~35 min** |

### Las tasas de la Entrega 2 no sirven

El resumen del escenario 2 declara **12,29 a 123,87 vídeos/min**. Con perfiles
reales, un worker de 1 vCPU rinde **0,43 vídeos/min** en el perfil corto. La
diferencia es de 30 a 290 veces, y es aritméticamente imposible: no se
transcodifican 124 vídeos de dos minutos en un minuto con dos vCPU.

Esos números salen de los clips de 8 segundos. **No se comparan con nada medido
con perfiles reales.** A7 los rehace.

### Para G4 y G6: una corrida de Nivel 3 se mide en horas

El plan declara Nivel 3 con **3,0 vídeos/min** de inyección y dos workers.

| | |
| :--- | :--- |
| Tasa de servicio real, 2 workers, perfil corto | ~0,85 vídeos/min |
| Tasa de llegada declarada | 3,0 vídeos/min |
| Acumulación | ~2,2 vídeos/min |

Diez minutos de inyección dejan ~22 vídeos en cola, **~26 minutos más** de
drenaje si todos fueran cortos. Cada perfil largo en la mezcla añade 35 minutos
de una vCPU.

Hay que decidirlo **al planificar**, no al descubrirlo: o se baja la tasa de
inyección, o se acota cuántos largos entran, o se asume la duración y el coste.
Con los clips de 8 segundos esto no se veía.

**Para C6**, el lado bueno: con el procesamiento tan lento respecto de la
llegada, la profundidad de cola es una señal de escalamiento clarísima y con
mucho margen de reacción. El autoscaler tiene minutos, no segundos, para
levantar un worker.

### Un defecto menor, de paso

El recuento declarado de segmentos, `2 × ⌈D/6⌉`, se pasa un 20 %: el worker fija
el GOP en `segmentSeconds()*30 = 180` fotogramas asumiendo 30 fps, pero el vídeo
va a 25, así que los segmentos duran ~7,06 s y no 6. Salen 34 en vez de 40 para
el perfil corto. No rompe nada; la cifra declarada no es la real. Para **H3**.

<a id="25"></a>
## 25. Antes de publicar una cifra hay once reglas, y están en código

Las reglas de medición de toda la entrega están acordadas y fechadas en
[`capacity-planning/criterios_de_medicion.md`](../../capacity-planning/criterios_de_medicion.md)
(A6, 2026-10-25) y, lo que importa más, **implementadas**:

```bash
python scripts/capacity_criterios.py --autoprueba   # corre esto antes de medir
```

Un criterio que vive solo en un documento se olvida a las dos corridas. La
Entrega 2 tenía los criterios escritos en los dos planes y publicó percentiles
invertidos igualmente, porque nada los miraba.

### Lo que te va a frenar si no lo sabes

- **No hay destino por omisión en ningún guion.** `run_escenario1.sh`,
  `h2_nube.sh`, `h3_nube.sh` y la campaña del escenario 2 exigen que declares
  `BASE_URL` o `-base`, y además `ENTORNO`. La IP de la VM de la Entrega 2 ya no
  es el valor por omisión de nada.
- **Cada corrida escribe `destino.json`** y el resumen se niega a hacer tablas si
  falta. Una corrida sin él queda marcada `"publicable": false`.
- **Con `EXIGIR_NUBE=1` el resumen falla si el destino es local.** Úsalo en toda
  corrida que vayas a publicar como medición de la nube.
- **`-hls-base` es obligatoria contra la nube.** Antes se adivinaba comparando la
  URL con una IP literal.
- **Las tablas dicen `n/d (n=k)`** donde no hay observaciones para sostener el
  percentil. No es un error: es la respuesta.

### Dos cosas que cambian decisiones de otros issues

**Un p95 necesita 20 observaciones, y un p99 cien.** Eso invalida *todos* los p95
por etapa del escenario 2 de la Entrega 2 —los niveles subían 1, 2, 4, 8 y 12
vídeos— y obliga a **G4** a elegir: o un nivel sube 20 vídeos o más, o sus etapas
se publican con mediana, mínimo, máximo y observaciones en crudo. Con la nota 24
encima —cada vídeo largo son 35 minutos de una vCPU—, esa elección tiene coste y
hay que hacerla al planificar.

El consumo HLS es el caso contrario: 25 estudiantes por 34 segmentos son 850
observaciones. Ahí sí hay p95.

**Dos métricas más de la Entrega 2 salían de una constante**, además del tiempo al
primer cuadro que marcó la revisión: el margen de buffer era `6,0 − latencia` y
una interrupción se contaba si un segmento tardaba más de 6 s **sobre un patrón
que descargaba sin pausa**, así que el «0 interrupciones» estaba garantizado por
aritmética. Y el simulador recibía la cadencia y el número de estudiantes y no
usaba ninguno de los dos: nunca hubo más de un estudiante ni una sola pausa, pese
a que se publicaron 100 concurrentes.

Para **G4 y G7**: ninguna de esas cifras se puede comparar. La §11 del documento
de criterios dice, cifra por cifra, qué sobrevive de la Entrega 2. El escenario 1
sobrevive entero; del escenario 2 solo el planteamiento.

**Para H3**, de paso: el 6,0 supuesto se equivocaba un 18 %. La duración de
segmento ahora se lee del `#EXTINF`, que es ~7,06 s por lo de la nota 24.

<a id="26"></a>
## 26. El entorno de la Entrega 2 está a medias, y ninguna imagen del tag está publicada

Estado real, comprobado el 2026-10-25 con `bash ./scripts/a7_preparar.sh`:

| | |
| :--- | :--- |
| Las dos VMs | **existen, apagadas** (`TERMINATED`). Conservan disco e IP |
| Cloud SQL | **borrada el 2026-09-28**, y también fuera del estado de Terraform |
| VPC, NAT, buckets, Artifact Registry, cuentas de servicio | existen: 50 recursos en el estado |
| Plan de Terraform desde el tag | **3 a crear, 0 a cambiar, 0 a destruir** |

Consecuencias que ahorran tiempo:

- **Encender no basta.** Falta la base, así que hace falta `terraform apply`. Y
  **Terraform no gestiona el encendido**: `desired_status` no está declarado, por
  eso el plan dice «0 a cambiar» con las dos máquinas apagadas. Se encienden con
  `gcloud compute instances start`.
- **La IP privada de la base será distinta** de la que las VMs tienen escrita.
  Hay que editar `/etc/mooc/web.conf` y `/etc/mooc/worker.conf` y reiniciar los
  servicios, como en el paso 4 de I6.
- El nombre `mooc-db-1` ya está libre (Google reserva una semana; se borró hace
  27 días). Si volviera a estarlo, se aplica con `-var db_instance_generation=2`,
  **y eso cambia el nombre de conexión y la IP otra vez**.

### Ninguna de las imágenes publicadas es el código del tag

En Artifact Registry hay cinco imágenes, todas del 2026-09-27. **Ninguna
corresponde al commit del tag `entrega-2` ni a `main`.** La más reciente está
**59 commits antes del tag**, y la diferencia incluye `internal/worker/` con
`queue_metrics.go`, que es justo lo que el escenario 2 instrumenta.

Así que medir la línea base exige **construir y publicar** la imagen del tag, y
la corrida corregida la de `main`. Dos cosas que conviene saber antes:

- `publish_images.sh` usa `git rev-parse HEAD` y `.` como contexto, **ambos
  relativos al directorio actual**. Hay que hacer `cd` al árbol del tag; llamarlo
  por ruta absoluta desde el repositorio etiquetaría el código de `main` con el
  SHA del tag.
- `entrega-2` es un **tag anotado**: `git rev-parse entrega-2` devuelve el objeto
  de tag (`e81fa4ef…`), no el commit. El SHA que vale como `IMAGE_TAG` es
  `git rev-parse entrega-2^{commit}` → `1827cdc2…`. `prepare_web_env.sh` solo
  valida que sean 40 caracteres hexadecimales, así que el objeto de tag pasaría la
  validación y luego no existiría como imagen.

### Por qué la Entrega 2 midió el escenario 2 contra Docker Compose

No fue descuido. La campaña necesita el inspector de la cola (Redis), y el
cortafuegos solo abre el 6379 dentro de `10.0.1.0/24`; Cloud SQL no tiene IP
pública. Desde un portátil no hay a dónde conectarse, y `-base http://api:8080`
estaba como valor por omisión.

La salida no es medir Compose, es llevar los dos puertos por el **túnel SSH de
IAP**, que usa el 22 y no abre nada nuevo —**F1** sigue pudiendo demostrar que no
hay puertos de aplicación ni de datos expuestos—:

```bash
gcloud compute ssh mooc-worker-server --zone=us-east1-b --tunnel-through-iap \
    --project=plataforma-mooc-entrega2 \
    -- -N -L 6379:localhost:6379 -L 5432:<ip privada de la base>:5432
```

### Lo que cuesta, para que nadie se lo encuentre a mitad

| | |
| :--- | :--- |
| Entorno completo encendido | 0,169 USD/h · **4,05 USD/día** |
| Campaña de A7 completa | ~7,5 h → **1,69 USD** acotando el consumo HLS |
| Consumo HLS **sin acotar**, desde el portátil | 22,1 GB de egreso → **2,65 USD**, más que todo el cómputo |
| Escenario 2 con perfiles reales | 162 min de vídeo a 1,2× en 2 vCPU → **97 min de pared** irreducibles |

Para **G4 y G6**: el egreso del consumo HLS es la partida que se dispara, no el
cómputo. `-max-segmentos` lo acota, y con 10 segmentos por estudiante quedan 8
con instante de reproducción que cumplir, que es lo mínimo para que el margen de
buffer y las interrupciones signifiquen algo.

### Lo desplegado en la Entrega 2 no era el código del tag

El `/etc/mooc/web.conf` de la VM traía `IMAGE_TAG=b393116bf172…`, **59 commits
antes del commit del tag**. Así que los números publicados de la Entrega 2 salen
de **tres estados de código distintos**: `b393116b` en la nube para el
escenario 1, Docker Compose en local para el escenario 2, y el tag como lo que el
repositorio dice que fue la entrega.

La línea base de A7 se mide con **el código del tag**, que es lo comparable con la
Entrega 3 porque también partirá de un estado etiquetado. Pero se declara como
«el código del tag», no como «lo que estuvo vivo».

### Lo que corre en las VMs son copias congeladas, no el repositorio

La unidad de systemd apunta a `/home/dfortizr1/mooc/`:

```
ExecStartPre=/bin/bash /home/dfortizr1/mooc/prepare_env.sh
ExecStart=/usr/bin/docker compose -f /home/dfortizr1/mooc/docker-compose.prod.yml up -d
```

`prepare_env.sh` y el compose son copias de la época del tag. **Cambiar
`IMAGE_TAG` cambia la imagen y nada más.** Para desplegar `main` de verdad hay
que copiar también esos dos archivos, porque `main` añade `REDIS_SESSION_DB=1`,
`INSTANCE_ID` y `SESSION_COOKIE_SAMESITE`.

Sin ellos, **las sesiones se quedan en la base lógica 0 junto a la cola de
asynq**, que es el riesgo de la nota 23. Comprobado en vivo: con el código del
tag, `redis-cli info keyspace` en el `mooc-queue` del worker muestra las sesiones
en `db0`.

Afecta a **cualquiera que despliegue sobre estas VMs**, y es un argumento más
para que la Entrega 3 construya las imágenes y la configuración desde el
repositorio en el arranque de la instancia, no a mano en un home.

### `cmd/seed-media` solo puede correr en la VM worker

Transcodifica **en proceso** (`mediaProcessor.Handle`, no encola), así que escribe
los derivados con las credenciales de la máquina donde corre. Y el reparto de
permisos le prohíbe al SA web escribir derivados a propósito:
`api_creator_non_derivatives` frente a `hls_worker_writer`. En la VM web da un
403 que es la política funcionando.

Además escribe hacia `media/hls/` en vez del bucket público, porque el bucket de
derivados solo está declarado en el `.env` del worker.

**A7 no lo usa:** la campaña del escenario 2 sube los tres perfiles reales 27
veces por el camino de producción y deja el bucket sembrado con evidencia de la
propia medición. Si alguien lo ejecuta y falla a mitad, deja un recurso en
`processing` y un original huérfano: hay que limpiarlos antes de medir, porque la
verificación terminal cuenta estados bajo `originals/`.

### Dos trampas de operar las VMs desde Windows

- **El `.env` es de root.** `source` sin `sudo` falla con `Permission denied` y,
  al quedarse sin `DATABASE_URL`, el binario busca Postgres en `localhost`. Se
  resuelve con `sudo bash -c 'set -a; source …; set +a; exec …'`, que no deja
  copia del secreto en disco.
- **Las comillas no sobreviven a `plink`.** `gcloud compute ssh --command` con
  comillas anidadas, `$(...)` o `$$…$$` llega destrozado y da un error de
  sintaxis con un `^` que no señala nada. Escribe el script, cópialo y ejecútalo.
  Y **`--quiet` siempre**: sin él `pscp`/`plink` pueden esperar una confirmación
  que nadie teclea, sin emitir una sola línea.
- El túnel IAP hacia el worker dio **14 kB/s con cortes** mientras hacia la web
  daba 3,4 MB/s, con las dos máquinas ociosas. Para mover algo grande, pásalo por
  el bucket: es dentro de la región, no cuesta egreso y no depende del túnel.

### `h3_nube.sh escalera` metía el limitador de login en la medición

Dos cosas que cuestan una corrida entera:

- **`escalera` es un subcomando obligatorio.** Sin él, el script imprime su uso y
  sale con 1.
- En modo nube, `escalera` borraba `tokens.csv` y volvía a iniciar sesión **en
  paralelo a las corridas**: ~8,5 logins/min dentro de la ventana medida, que es
  justo lo que el paso separado de tokens existe para evitar (`escenario1.md` §4).
  Ahora hay `REUSAR_TOKENS=1`, que usa el camino que `repetir` ya tenía. El valor
  por omisión sigue siendo rehacerlos: reutilizar es una afirmación explícita de
  que son de este entorno y de hace menos de 24 h.
- `CAPACITY_PASSWORD` tiene que ir exportada, o el script la pide por teclado y
  se cuelga sin terminal interactiva.

### `REDIS_URL` en la disposición de dos VMs

`docker-compose.prod.yml` de `main` ya no levanta Redis y exige `REDIS_URL`
(A4). **Funciona sin cambios sobre las dos VMs**, porque `prepare_web_env.sh` ya
escribe `REDIS_URL='${QUEUE_PRIVATE_IP}:6379'`: la cola de la VM worker por la
VPC, con las sesiones en la base lógica 1. No hace falta esperar a B1 para
desplegar `main` sobre la Entrega 2.

<a id="27"></a>
## 27. Un `FLUSHDB` en el Redis de asynq mata la observabilidad y no se nota

Costó **tres corridas del escenario 2**, y es la clase de fallo más peligrosa que
ha salido en esta entrega: la plataforma sigue funcionando perfectamente y lo
único que muere es la capacidad de verla.

### Qué pasa exactamente

Asynq mantiene un conjunto `asynq:queues` con los nombres de las colas que
existen. `FLUSHDB` lo borra. A partir de ahí, el inspector responde:

```
NOT_FOUND: queue "default" does not exist
```

Y **no se recupera solo**, porque el cliente de asynq registra la cola *una vez
por proceso*: un contenedor de la API que llevaba arriba desde antes de la purga
cree que ya la registró y nunca vuelve a añadirla.

Mientras tanto todo lo demás funciona: el worker consume porque tiene el nombre
de la cola configurado, los trabajos se transcodifican, los recursos llegan a
`completed`. Lo único roto es la vista.

### Lo que produjo

La campaña registró **«pico de cola: 0 tareas»** habiendo confirmado 12 subidas,
y el archivo de drenaje escribió **«la cola se ha vaciado totalmente»** con 13
trabajos pendientes en la base. Un informe indistinguible de una corrida buena.

```
$ redis-cli -n 0 smembers asynq:queues
(vacío)
$ redis-cli -n 0 --scan --pattern 'asynq*'
asynq:{default}:active
asynq:{default}:t:media-originals/...
asynq:{default}:processed:2026-10-26
```

La cola estaba ahí; el registro no.

### Cómo se purga bien

[`scripts/a7_purgar.sh`](../../scripts/a7_purgar.sh) borra las estructuras una
por una y **conserva el registro**. Si te toca hacerlo a mano:

```bash
redis-cli -n 0 del 'asynq:{default}:pending' 'asynq:{default}:active' ...
redis-cli -n 0 --scan --pattern 'asynq:{default}:t:*' | xargs -r redis-cli -n 0 del
redis-cli -n 0 sadd asynq:queues default     # restaura el registro
```

Y si alguien ya hizo el `FLUSHDB`, el arreglo es ese `sadd`, o reiniciar el
contenedor de la API para que su cliente vuelva a registrarse.

### Para B1, C6 y C7

**La cola de la Entrega 3 es un servicio compartido**, así que un `FLUSHDB`
—para limpiar sesiones, para liberar memoria, por costumbre— no afecta solo a
quien lo ejecuta:

- **C6** escala los workers por profundidad de cola. Con el registro borrado, la
  métrica vale **0** para siempre y el autoescalador **nunca sube un worker**
  mientras la cola crece. Eso no es una medición mala: es una política de
  escalado que no se dispara, y en producción se vería como lentitud sin causa.
- **C7** demuestra el drenaje de un worker que se retira. Con el inspector ciego,
  la demostración sale vacía y parece correcta.
- **B1** debería dejar escrito en el runbook de la cola que `FLUSHDB` y
  `FLUSHALL` están prohibidos ahí, y por qué. La nota 23 ya avisa de que las
  sesiones y la cola comparten instancia; esto es la otra cara: la herramienta
  obvia para limpiar una rompe la otra en silencio.

### Lo que salvó el diagnóstico

Que la campaña dejara de publicar ceros. Tras el segundo intento cambié
`if err == nil` por contar los fallos y declarar el drenaje **no observable**,
y el cuarto intento dijo «3 667 lecturas de la cola fallaron en este nivel» y
salió con código 1. Sin eso, la quinta corrida habría dado un informe
aparentemente bueno.

**Regla general, no solo para asynq:** una lectura que falla no es un cero. Si
el código no distingue «no pude ver» de «vi que era cero», la evidencia acabará
afirmando lo segundo.

---

Este archivo no contiene credenciales, llaves ni secretos.

## 28. Una conclusión escrita a mano es una constante que sobrevive a todo

La campaña del escenario 2 genera seis archivos de evidencia. Las tablas por
nivel siempre se derivaron de lo medido, pero **las conclusiones estaban
escritas fijas**, redactadas durante la Entrega 2. Sobrevivieron al cambio de
entorno (Compose a GCP), de código (tag a corregido) y de perfiles (clips de 8 s
a vídeos de 30 min), y acabaron contradiciendo a la tabla que tenían encima en la
misma página:

| Afirmaba | Medido en la línea base del código corregido |
| :--- | :--- |
| «p95 inferior a 20 ms» en el plano de control | `n/d`: ningún nivel llega a 20 observaciones |
| PUT «entre 25 y 45 MB/s» · «> 30 MB/s» | 0,71–2,14 MB/s |
| Descarga HLS «> 120 MB/s» | 3,51 MB/s |
| Antigüedad de cola «superó los 35 s» | 3 723 s |
| Cómputo «~2,8 s por tarea» | 95–168 s |
| «hasta 10 tareas en espera» | pico de 14 |
| «pending = 0, processing = 0» | 2 recursos en `processing` |
| «sin estados indeterminados» | dos líneas después de «huérfanos: 2» |
| «0 trabajos huérfanos ✅» | 2 |

Y un caso aparte, porque no era una constante sino un reloj mal puesto: **el
margen de buffer del simulador HLS era negativo por construcción**. El instante
de reproducción se tomaba del inicio de la sesión en vez de de cuando se llena el
buffer inicial, y encima se le restaba el buffer. Con la cadencia a 1× el reloj
de descarga y el de reproducción avanzan igual, así que el déficit inicial nunca
se recuperaba: el margen salía fijo en −14,40 s y **el 100 % de los segmentos con
plazo contaba como interrupción** —16 de 16 en el nivel 0, 80 de 80 en el nivel
1—. Con el reloj correcto el margen es +14,30 s constante, que es exactamente la
profundidad del buffer, y una interrupción vuelve a significar algo: que una
descarga tardó más que la duración de un segmento.

**Qué se cambió.** En `scripts/capacity_escenario2/main.go`, los tres documentos
de conclusiones (`resumen_ejecutivo`, `analisis_cuello_de_botella`,
`reporte_carga`) y el README se derivan ahora de un `resumirHechos()` común; el
veredicto del drenaje y las palomas del README son `switch` sobre los datos; y
`margenTexto`/`stallsTexto` dicen «no aplica» cuando no se descargó ningún
segmento con instante de reproducción, en vez de publicar un cero.

**La regla, para B, C, F y G.** Un veredicto que no puede fallar no es un
veredicto. Si un documento de evidencia contiene una frase que seguiría siendo
literalmente la misma con otros datos, esa frase no es evidencia: es decoración,
y tarde o temprano contradice a la tabla de al lado. Es el mismo defecto que A6
prohíbe en §7 para las métricas derivadas de constantes, aplicado a la prosa.

**Lo que no se pudo arreglar.** El proceso que escribió esa evidencia ya no
existía cuando se encontró el defecto, así que los cuatro documentos de la
corrida del 2026-10-27 se rederivaron **a mano** de los artefactos medidos, y
cada uno lo declara en su cabecera. A partir de la siguiente corrida se derivan
solos. Para no repetirlo: conviene que la campaña vuelque `levels` y `drain` en
JSON junto a la evidencia, para poder regenerar los documentos sin volver a medir.

## 29. Destruir Cloud SQL con `-target` falla por el usuario, no por la instancia

El runbook de A7 y el de I6 destruyen la base con `-target`, para no tocar el
resto del entorno. Con los tres recursos apuntados a la vez, falla:

```
Error: failed to delete user moocuser in instance mooc-db-1:
  role "moocuser" cannot be dropped because some objects depend on it
  Details: 20 objects in database moocdb
```

Terraform intenta borrar `google_sql_user.app` **antes** que la instancia, y
PostgreSQL se niega porque ese rol es dueño de las 19 tablas del esquema. No es
un problema de permisos ni de protección de borrado: es que el orden de
destrucción de un `-target` múltiple no respeta que el usuario vive dentro de la
instancia. Y el fallo deja el destroy a medias, con el recurso de la base lógica
ya fuera del estado.

**Lo que funciona:** sacar el usuario del estado —no toca la nube, el usuario
desaparece con la instancia— y destruir solo la instancia.

```bash
terraform state rm google_sql_user.app
terraform plan -destroy -out=d.tfplan -target=google_sql_database_instance.main
terraform apply d.tfplan
```

Después, `terraform plan` debe decir **3 to add, 0 to change, 0 to destroy**: los
tres recursos vuelven a estar declarados y ausentes, que es la línea de partida.
Si dice otra cosa, el estado y la nube no coinciden.

**Dos trampas de alrededor, para no repetirlas:**

- **Un plan guardado caduca enseguida.** Cualquier `terraform plan` refresca el
  estado remoto y le cambia el serial, así que un `-out` de hace cinco minutos da
  «Saved plan is stale». Hay que generar y aplicar el plan seguidos, sin ningún
  `plan` en medio.
- **Un plan guardado lleva las variables en claro**, la contraseña de la base
  entre ellas, igual que el estado. `*.tfplan` está en `.gitignore` por eso;
  antes no lo estaba.

<a id="30"></a>
## 30. La cola es una VM propia, y eso trae un punto único de fallo que hay que mirar

B1 (#200) comparó Memorystore con una VM dedicada y eligió la VM. Lo que decidió
no fue el coste, aunque también: **Memorystore no soporta AOF**. Su única
persistencia son instantáneas RDB con intervalo mínimo de una hora, y el nivel
Basic se recupera precisamente de la última instantánea, así que una cola puede
perder una hora de trabajos en un reinicio. El criterio de aceptación «los
trabajos encolados sobreviven al reinicio del servicio de cola» no se cumple con
la opción gestionada.

| | Persistencia | Pérdida al reiniciar | USD/mes |
| :--- | :--- | :--- | ---: |
| Memorystore Basic M1 | solo RDB, mínimo 1 h | hasta 1 hora de cola | 35,77 |
| Memorystore Standard M1 | RDB + réplica | reinicio completo | 46,72 |
| VM con Redis (elegida) | AOF `appendfsync everysec` | ≤ 1 segundo | **13,23** |

### Lo que esto obliga a los demás

| Issue | Qué tiene que saber |
| :--- | :--- |
| **B3, B4, C5** | `REDIS_URL` sale de `terraform output queue_private_ip`, que es una **IP interna estática**. Está reservada a propósito: si la asignara el DHCP, recrear la cola obligaría a rotar los dos grupos |
| **C6** | La VM de la cola es el sitio del exportador de la señal de autoscaling, y su cuenta ya tiene `monitoring.metricWriter`. El exportador hay que escribirlo |
| **C7** | El drenaje de workers se apoya en que la cola sobreviva al retiro de la instancia. Ya es cierto por construcción |
| **F1** | `mooc-allow-queue` acota el 6379 a la subred, pero **hoy no restringe nada**: `mooc-allow-internal` abre todos los puertos entre todo lo que esté en la subred. La regla empieza a servir cuando F1 acote aquella |
| **F3** | 13,23 USD/mes de lista. El coste real, con el consumo observado, lo cierra F3 |
| **G1** | **Falta la alerta de memoria.** Con `noeviction`, quedarse sin memoria no corrompe nada pero **detiene el encolado**. La nota 23 exigía elegir entre dimensionar con holgura o separar instancias: se eligió holgura, y esa elección está a medias sin la alerta |
| **I6** | Al destruir, la cola es una VM más. El AOF vive en su disco de arranque: destruirla **sí** pierde los trabajos encolados, que es lo correcto al cerrar |

### El punto único de fallo, dicho sin adornos

`mooc-queue-server` es un punto único de fallo para los dos grupos elásticos: si
cae, ni la API encola ni los workers consumen. Se mitiga con `automatic_restart`
y `on_host_maintenance = MIGRATE`, y el AOF hace que un reinicio no sea pérdida
de datos, pero **no es alta disponibilidad**. El enunciado pide la cola *fuera*
de los grupos, no redundante; si la Entrega 3 quisiera redundancia hay que
elegir entre Memorystore Standard --con su ventana de pérdida-- o un segundo
nodo con replicación que habría que operar.

### Un agujero que existía en el repositorio

`scripts/seed.sh` hacía `FLUSHDB` contra `${REDIS_URL}`. Mientras cada máquina
tenía su propio Redis era un problema de laboratorio; con una cola compartida se
lleva la cola de producción y, peor, el registro `asynq:queues` de la nota 27.

Corregido en dos sitios, porque uno no basta:

- el guion solo vacía el Redis **local de Compose**, y avisa cuando `REDIS_URL`
  no es local en vez de hacerlo en silencio;
- el servidor desactiva `FLUSHDB` y `FLUSHALL` **por nombre**
  (`rename-command ... ""`), para que la defensa no dependa de la buena memoria
  de quien ejecuta el guion.

### Lo que bloquea el primer `apply` de la Entrega 3

El backend sigue en `prefix = "entrega2"` y el README de A7 §1 dejó anotado que
la Entrega 3 estrena `entrega3`. Un prefijo nuevo arranca con **estado vacío** e
intentaría crear de cero los recursos que A7 dejó vivos a propósito --los
buckets, que tienen nombre único global, el Artifact Registry, la VPC, los
secretos y las cuentas de servicio--, y fallaría en los buckets.

Hay que decidirlo en **B2** o **I6**, y las salidas son dos: seguir en `entrega2`
--que respeta el invariante de A7, una sola arquitectura viva a la vez, y deja el
nombre del prefijo como algo cosmético-- o **migrar el estado** copiando el
objeto de `entrega2/` a `entrega3/` en el bucket antes de cambiar el prefijo. Lo
que no vale es cambiar el prefijo y aplicar.

<a id="31"></a>
## 31. La regla que abría todos los puertos ya no existe, y eso cambia cuatro issues

Hasta B2 (#201), `mooc-allow-internal` permitía **todos** los puertos TCP y UDP
entre cualquier par de instancias de la subred. Con dos VMs fijas era una
comodidad; con grupos administrados que levantan y retiran instancias es una
superficie que crece sola. Y hacía **decorativa** cualquier regla más
específica: la que B1 escribió para el 6379 de la cola no restringía nada
mientras aquella existiera.

En su lugar hay tres reglas con propósito, dos etiquetas de grupo y una
ausencia deliberada.

| Regla | Origen | Destino | Puerto |
| :--- | :--- | :--- | :--- |
| `mooc-allow-lb-health` | `130.211.0.0/22`, `35.191.0.0/16` | etiqueta `mooc-api-node` | 80/tcp |
| `mooc-allow-queue` | cuentas de la API y de los workers | cuenta de la cola | 6379/tcp |
| `mooc-allow-internal-icmp` | rango de la subred | la subred | ICMP |

**No hay ninguna regla con destino `mooc-worker-node`**, y es a propósito: nadie
le habla a un worker. Si alguien añade una, que explique qué flujo la necesita.

### Lo que esto obliga a los demás

| Issue | Qué tiene que hacer o saber |
| :--- | :--- |
| **C1** | La plantilla de la API lleva la etiqueta **`mooc-api-node`** y **no declara `access_config`**. Sin la etiqueta, las comprobaciones de salud no llegan y el grupo entero sale como no sano; con `access_config`, se rompe el criterio del barrido desde internet |
| **C3** | `mooc-allow-lb-health` presupone un balanceador de aplicación **global**, que entrega desde esos dos rangos. Un balanceador **regional** se basa en Envoy y entrega desde una subred de solo proxy (`REGIONAL_MANAGED_PROXY`) que habría que crear: entonces el origen cambia y hay que volver a `network.tf`. El puerto del backend es el **80**, porque el balanceador termina TLS |
| **C5** | La plantilla de los workers lleva **`mooc-worker-node`**, sin `access_config`, y su **cuenta de servicio tiene que ser `sa-worker-server`**: la regla de la cola va por identidad, así que una cuenta distinta deja al worker sin poder encolar ni leer |
| **C6** | `sa-queue-server` ya tiene `monitoring.metricWriter` para alojar el exportador de la señal. **Si el exportador va a otro sitio, hay que retirar ese permiso**: hoy es el único permiso sin uso de las tres cuentas, y está declarado como tal |
| **F1** | Su verificación ya tiene sentido. Antes de B2, comprobar que el 6379 solo se alcanza desde quien debe habría dado un falso positivo por `allow_internal` |

### Por qué la regla de la cola va por cuenta y no por etiqueta

Una etiqueta de red la puede poner cualquiera que sepa crear una instancia, y
entonces la regla protege un nombre y no un componente. La cuenta de servicio la
fija la plantilla del grupo y cambiarla es una operación de IAM.

Trampa de la API de Compute: **`source_service_accounts` no se puede combinar con
etiquetas en ninguno de los dos lados**, así que el destino también va por
cuenta. Si alguien intenta añadir `target_tags` a esa regla, el proveedor la
rechaza.

### El barrido desde internet se cumple por estructura, no por cortafuegos

Las instancias de los grupos **no llevan IP externa**, así que no hay dirección
a la que llamar. El cortafuegos gobierna lo que llega por dentro de la VPC y lo
que entrega Google --balanceador, salud, IAP--, que no son el camino público
hacia la instancia. Es mejor garantía que una regla, porque una regla alguien la
puede ampliar.

En la misma línea, B2 retiró la **IP externa de `mooc-worker-server`**, que
estaba descrita como «cerrada por firewall perimetral»: superficie sin función.
Con ella se retiró la reserva `google_compute_address.worker_server_ip`, porque
una IP estática reservada y sin adjuntar **se factura**, y la salida
`worker_server_public_ip`. La administración entra por IAP y la salida va por el
Cloud NAT, así que no se perdió ningún camino.

La única dirección pública que queda es la de `mooc-web-server`, con su regla
80/443 acotada a la etiqueta `web-server`, que solo lleva ella. **No alcanza a
los grupos.** Se retira cuando el balanceador de C3 sea el único camino de
entrada y C1 sustituya esa VM; quitarla antes dejaría la arquitectura de la
Entrega 2 inalcanzable sin nada que la reemplace.

### Lo que ya estaba y no hizo falta tocar

`private_ip_google_access` en la subred y el Cloud NAT existían desde la Entrega
2. Importan juntos: con acceso privado a Google **y** NAT, una instancia sin IP
externa puede bajar su imagen del registro, leer su secreto y enviar registros y
métricas. Es lo que hace viable que ninguna instancia de los grupos tenga
dirección pública.

<a id="32"></a>
## 32. Una instancia nueva se levanta sola, y un solo valor pin el código entero

B3 (#202) sustituyó el arranque que dejaba la máquina a medias --instalaba
Docker y nada más-- por `infra/bootstrap/mooc-bootstrap.sh`, que la deja
atendiendo peticiones o consumiendo trabajos sin que nadie entre por SSH. Las
unidades de systemd también están ahora **en el repositorio**
(`infra/bootstrap/mooc-{web,worker}.service`).

Lo que se fue: `sudo tee /etc/mooc/<rol>.conf`, el clonado en
`/home/dfortizr1/mooc` y la unidad escrita a mano con esa ruta dentro. Dos de
esas cosas no sobreviven a un grupo administrado --el directorio personal no
existe en una instancia nueva, y la configuración en un disco la borra una
recreación (nota 19)--.

### El guion no se parametriza con `templatefile`

Lo lee todo del **servidor de metadatos en tiempo de ejecución**. Así sus bytes
son idénticos para los dos roles y para todas las instancias, y lo único que
distingue una de otra es su metadata. El criterio «todas las instancias de un
grupo y una corrida ejecutan la misma versión de código y configuración» pasa a
ser estructural en vez de algo que haya que vigilar en cada plantilla.

**`mooc-image-tag` pin las dos mitades del despliegue**: la imagen del contenedor
y el `git checkout --detach <SHA>` de `/opt/mooc`, de donde salen los Compose,
los guiones de entorno y las unidades. Clonar `main` y correr la imagen de otro
commit es cómo se acaba depurando una diferencia que no está en el código que se
está leyendo.

### Lo que esto obliga a los demás

| Issue | Qué tiene que hacer o saber |
| :--- | :--- |
| **C1, C5** | La plantilla se escribe `metadata = merge(local.mooc_metadata_comun, local.mooc_metadata_web)` --o `_worker`--, de `infra/terraform/bootstrap.tf`. El contrato completo de claves está en la evidencia de B3 §2 |
| **C1, C5** | El guion va en `metadata["startup-script"]`, **no** en `metadata_startup_script` |
| **C3, C4** | La sonda de listo es **`/api/v1/ready`** (A2): prueba la base y la bandera de arranque. Es la que mide B3 y la que debe usar el health check, para que lo medido y lo decidido sean lo mismo |
| **C6** | El periodo de inicialización del autoescalado se dimensiona con el número que escribe `/var/lib/mooc/arranque.json`, **más** el arranque del sistema operativo: B3 mide desde el inicio del guion, no desde la creación de la instancia |
| **C7** | `--timeout 120` en la unidad del worker es un compromiso, **no una política de drenaje**. C7 tiene que apagar el consumo de tareas nuevas antes de parar el contenedor |
| **A3** | El arranque **no aplica migraciones**, a propósito (nota 5). Cuándo migrar sigue siendo de A3, y es un paso del despliegue |
| **cualquier `apply`** | Hacen falta tres variables nuevas: `TF_VAR_image_tag` (SHA de 40 hex, validado), `TF_VAR_smtp_from` y `TF_VAR_smtp_username`. Las dos de SMTP no son secretos pero identifican la cuenta del equipo y el repositorio es público, así que no tienen valor por omisión |

### La trampa: `metadata_startup_script` es ForceNew en los dos sentidos

Cambiar su contenido **recrea la VM**. Y quitarlo para mover el guion a
`metadata["startup-script"]` --que sí se actualiza en sitio-- **también**. El
plan lo dice con `# forces replacement`.

Por eso B3 **no tocó las dos VMs fijas**: migrarlas al arranque nuevo las
destruye, y B3 no lo necesita porque su comprobación bloqueante es una captura de
la consola **del grupo**, que exige C1 y C5 de todos modos. Para una plantilla es
irrelevante: una plantilla es inmutable y cambiarla ya es crear otra.

Si alguien quiere ensayar el arranque antes de C1, se puede, y la consecuencia es
recrear esas dos VMs.

### Dos cosas pequeñas que cuestan una tarde

- **`-p mooc` en todos los `docker compose`.** Sin nombre de proyecto explícito,
  Compose lo deriva del directorio: si la ruta cambia, los contenedores de la
  anterior quedan como otro proyecto en vez de reutilizarse, que es la
  duplicación que el criterio prohíbe.
- **`docker compose down` mata a los 10 segundos.** El worker atiende SIGTERM con
  parada ordenada de asynq, pero una tarea real tarda minutos --el perfil largo
  midió 1 711 s en A7--. No es pérdida de datos, porque asynq la reencola y el
  worker es idempotente, pero es trabajo tirado. La unidad usa `--timeout 120`.

### Y una que parecía trabajo y no lo era

**FFmpeg no se instala en el anfitrión.** Va dentro de la imagen del worker
(`Dockerfile.worker`, `apk add ffmpeg`), así que la tarea «dependencias
multimedia disponibles en el arranque de los workers» se cumple por la imagen.
Instalarlo también en la VM habría sido una dependencia duplicada que nadie usa.
