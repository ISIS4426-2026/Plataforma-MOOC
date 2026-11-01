# Notas técnicas de la Entrega 3

Hallazgos que afectan a más de un issue. Cada uno dice dónde falla, para que no
se redescubra a mitad de una corrida.

**Léelo antes de empezar tu issue y anota antes de cerrarlo.** Si tu issue no
dejó nada aquí, dilo explícitamente al cerrarlo: así se distingue «no había nada
que anotar» de «no lo revisé». Es criterio de aceptación de los 44 issues.

La numeración tiene huecos a propósito: las notas **7** y **26** se retiraron el
29 de octubre de 2026 porque describían un estado del mundo que ya no existe —el
frontend esperando a la infraestructura, y el entorno de la Entrega 2 a medias—.
Los números no se reutilizan ni se corren: se citan por todo el repositorio y en
los READMEs de evidencia, y renumerar dejaría cada cita apuntando a otra cosa.

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
| 27 | [Un `FLUSHDB` en el Redis de asynq mata la observabilidad y no se nota](#27) | **B1, C6, C7**, A7, G4, G6 |
| 46 | [El frontend se sirve en el mismo origen, y la API no tiene «mis cursos» ni «quién soy»](#46) | **D2, D3**, A4, B3, C1, C3, E1 |
| 47 | [Interfaces del estudiante y del profesor: trampas que ya costaron tiempo](#47) | **D2, D3**, A1, A5, B1, C3, E1 |
| 48 | [Un estudiante inscrito no puede abrir un PDF, una imagen ni un archivo descargable](#48) | **D2, D5**, G1, A5 |
| 49 | [El generador de carga sale de una VM propia, y lo que mide el balanceador contrasta lo que mide JMeter](#49) | **G1**, G3, G4, G5, G6, G7 |

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
> fuera de los dos grupos. **Y en C6 (#209) en cuanto al exportador**, que es la
> otra mitad: `cmd/queue-exporter` corre en esa misma VM y escribe la señal
> directamente en Cloud Monitoring. El aviso de abajo se conserva porque explica
> por qué el exportador no podía vivir en el grupo.

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

> **Resuelta en A3 (#195).** El aviso de abajo ya no aplica: arrancar una
> instancia es seguro. Se conserva porque explica por qué hizo falta el arreglo.

En la Entrega 2 daba igual quién aplicara el esquema: había una sola VM. Un grupo
administrado que levanta hasta tres instancias a la vez puede lanzar tres
`migrate` en paralelo sobre la misma base. A3 lo reprodujo: **dos de tres
instancias no arrancaban**, y el grupo las recreaba para que fallaran igual.

A3 eligió el **advisory lock de transacción** por migración, con la comprobación
de «ya aplicada» **dentro** del lock, en lugar de un paso explícito de
despliegue. Y decidió que el esquema se aplique **en el arranque**: B3 lo ejecuta
en un `ExecStartPre` de las dos unidades, antes de levantar los contenedores
(nota 32).

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

> **Corregido en C5 (#208).** Sí hay una, y el flujo que la necesita es el
> autohealing: `mooc-allow-worker-health` deja entrar al **9090** desde
> `130.211.0.0/22` y `35.191.0.0/16` para que la comprobación de salud alcance
> la sonda `/readyz` del worker. Sigue sin entrar nadie más: ni el balanceador
> ni el resto de la subred. Y lo que B2 decía de las **métricas** sigue siendo
> cierto --el agente las raspa en `localhost`--; lo que se abre es la sonda, que
> comparte puerto porque el worker no tiene otro servidor HTTP.

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
| **A3** | El arranque **sí aplica el esquema**, que es lo que A3 §4 decidió: `migrate.sh` y `migrate.sh --require` en un `ExecStartPre`, antes de levantar los contenedores, y en los dos roles. Seguro con tres instancias a la vez por el advisory lock de transacción |
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

### El arranque migra, y eso hay que leerlo junto con la nota 5

La nota 5 avisaba de que tres réplicas pueden migrar la base a la vez y decía
«hasta que A3 cierre, no asumas que arrancar una instancia es seguro». **A3
cerró**, con un advisory lock de transacción por migración, y eligió
explícitamente que el esquema se aplique en el arranque y no como paso de
despliegue. Así que la nota 5 está resuelta y el aviso ya no aplica.

La primera versión de B3 no migraba, «dejándoselo a A3». Era un error: un entorno
nuevo habría arrancado sin esquema. Dos lecciones pequeñas de ahi:

- **Un issue cerrado puede haber decidido lo contrario de lo que su nota de
  advertencia decía**, porque la nota se escribio antes. Hay que leer la decision,
  no solo la advertencia.
- `migrate.sh` necesita **`psql`**, y el arranque anterior solo instalaba
  `postgresql-client` en la VM web. Ahora lo instalan los dos roles, porque los
  dos aplican el esquema.

### Y una que parecía trabajo y no lo era

**FFmpeg no se instala en el anfitrión.** Va dentro de la imagen del worker
(`Dockerfile.worker`, `apk add ffmpeg`), así que la tarea «dependencias
multimedia disponibles en el arranque de los workers» se cumple por la imagen.
Instalarlo también en la VM habría sido una dependencia duplicada que nadie usa.

<a id="33"></a>
## 33. Los secretos tienen tres niveles, y `cloudsql.client` no se usa

B4 (#203) inventarió la configuración. Dos cajones —público y secreto— no
alcanzan: forzarlos es lo que lleva a tratar como secreto algo que no lo es, o a
publicar algo que no se debe.

| Nivel | Dónde vive | Qué hay |
| :--- | :--- | :--- |
| **Secreto** | Secret Manager, leído por la identidad de la instancia | `db-password`, `smtp-password` |
| **No secreto, no publicable** | metadata de la instancia y `/etc/mooc/web.conf` | `smtp_from`, `smtp_username` |
| **Público** | el repositorio | todo lo demás |

El nivel 2 existe porque `mail.tf` ya lo había razonado: el usuario del proveedor
**no es un secreto** —en Brevo es el correo de la cuenta, en SendGrid la cadena
literal `apikey`— y tratarlo como tal sería teatro. Lo que protege la cuenta es
la contraseña, que sí está en el nivel 1.

**Y no existe ninguna llave de cuenta de servicio.** Ni para Terraform ni para
las VMs: identidades adjuntas y credenciales del servidor de metadatos. Un
secreto que no existe no se filtra, no caduca y no hay que rotarlo.

### El hallazgo: `roles/cloudsql.client` hoy no se usa

Ese rol concede `cloudsql.instances.connect`, que es lo que consumen **el proxy
de autenticación y los conectores** de Cloud SQL. Esta plataforma **no usa
ninguno de los dos**: se conecta por TCP directo a la IP privada, y en ese camino
**IAM no se consulta**. Lo que permite la conexión es la ruta de la VPC más la
contraseña de PostgreSQL.

El comentario de `service_accounts.tf` decía «permite abrir la conexión». La
segunda mitad —«no administra la instancia»— es cierta; la primera no, con este
método de conexión. Corregido.

**Esto corrige la auditoría de B2**, que declaró un solo permiso sin uso. Son
dos.

| Issue | Qué le toca |
| :--- | :--- |
| **F2** | Retirar `cloudsql.client` de las dos cuentas, **comprobándolo contra un despliegue vivo**: si alguna ruta lo necesitara, el fallo sería «permiso denegado» y no «conexión rechazada», y a ciegas se distinguen mal |
| **F2** | Decidir si la metadata debe ser publicable literalmente. Hoy lleva los dos valores del nivel 2; el arreglo es un secreto más y unas diez líneas, contra una decisión documentada |
| **I5, I6** | `bash ./scripts/b4_barrido_secretos.sh` antes de etiquetar y al cerrar. Sale con 1 si encuentra algo |

### Las dos capas de identidad no son la misma pregunta

- **IAM** responde «¿puede esta máquina hablar con este servicio de Google?».
  Quien es, es la VM; se prueba con un token del servidor de metadatos; se revoca
  quitando un enlace de IAM.
- **PostgreSQL** responde «¿qué rol de base es esta sesión?». Quien es, es
  `moocuser`; se prueba con contraseña sobre `sslmode=require`; se revoca
  cambiando la contraseña, **no desde IAM**.

Son ortogonales. Quitar `cloudsql.client` no cerraría la base, y tener la
contraseña sin la ruta privada no la abriría.

### Un barrido que lista coincidencias no sirve de nada

La primera versión de `b4_barrido_secretos.sh` imprimía 22 URIs con contraseña,
todas de desarrollo local. Eso es **peor que no comprobar**: un informe con ruido
no distingue «no hay secretos» de «nadie lo lee».

Ahora **afirma**: toda contraseña en una URI de conexión tiene que ser una de las
cuatro declaradas como de desarrollo (`moocpassword`, `postgres`,
`UnaContrasena*`, `a?b&c=d`), y cualquier otra es un hallazgo. Los marcadores en
mayúsculas y las referencias a variable se excluyen explícitamente.

**Y la garantía real no es el barrido.** `moocpassword` y `minioadmin` están en el
repositorio a propósito, porque son credenciales de contenedores que solo existen
en la máquina de quien los levanta. Lo que las hace inofensivas es que
`internal/config/validate.go` **rechaza el arranque** con `APP_ENV=production` si
sobreviven a un despliegue — y el barrido comprueba que esa comprobación siga
existiendo y pasando. Es la diferencia entre una convención, que se olvida, y una
comprobación, que no.

### `terraform.tfvars.example` estaba desfasado

No mencionaba las cuatro variables que añadió B3, y **tres son obligatorias**:
`TF_VAR_image_tag`, `TF_VAR_smtp_from` y `TF_VAR_smtp_username`, además de
`TF_VAR_db_password`. Quien siguiera ese archivo veía el `apply` pararse a
preguntar sin entender por qué.

<a id="34"></a>
## 34. Una instancia de grupo no puede servir con el nginx de la Entrega 2

Lo encontró C1 (#204) al escribir el autohealing, y no estaba en la lista de
tareas del issue. Son dos problemas independientes y cada uno basta para que la
instancia no arranque.

1. **El certificado no existe.** `nginx.conf` termina TLS con
   `ssl_certificate /etc/letsencrypt/live/${APP_DOMAIN}/fullchain.pem`. Los
   certificados viven en `/etc/letsencrypt` del anfitrión y **no hay contenedor
   de certbot**: se obtuvieron a mano. En un disco nuevo ese directorio está
   vacío y **nginx no arranca**.
2. **Certbot nunca podría obtenerlo.** Una instancia de grupo no tiene IP pública
   (B2), así que el desafío HTTP-01 no tiene por dónde llegar.

Y el 80 de `nginx.conf` hace `return 301` para todo lo que no sea ACME. Una
comprobación de salud de GCP **solo acepta 200**, así que: 301 → instancia no
sana → el autohealer la recrea → la nueva hace lo mismo. **Un bucle de
recreación que se lee como «la imagen está rota».**

### El modelo que lo resuelve

TLS en el **balanceador** (C3), con certificado administrado por Google; los
backends sirven **HTTP plano en el 80** dentro de la VPC. Sin IP pública en las
instancias, ese HTTP no viaja por internet en ningún tramo.

El perfil nuevo es `nginx.mig.conf`, y se elige por metadata
(`mooc-nginx-config`). No hubo que inventar el mecanismo:
`docker-compose.prod.yml` ya tenía `${NGINX_CONFIG_FILE:-./nginx.conf}`, con
`nginx.bootstrap.conf` como precedente. `prepare_web_env.sh` valida que el valor
sea uno de los tres perfiles conocidos.

**La VM fija no cambia**: su valor por omisión sigue siendo `./nginx.conf`.

### Lo que esto obliga a los demás

| Issue | Qué tiene que saber |
| :--- | :--- |
| **C3** | El balanceador **tiene** que terminar TLS: los backends no lo hacen y no pueden. Backend por el named port `http` → 80. Y tiene que ser **global**, por lo que B2 dejó dicho en la nota 31 |
| **C3** | Lo que `nginx.mig.conf` conserva de `nginx.conf` es la denegación de `/api/v1/metrics` y las cabeceras de proxy. Si alguna vez se quita nginx del Compose, esas dos cosas hay que reponerlas en otro sitio |
| **C4** | La sonda es `/api/v1/ready` en el 80, la de A2, la misma del autohealing de C1. Que la del backend sea otra sería decidir con un criterio distinto del medido |
| **C2** | El grupo ignora `target_size` a propósito (`ignore_changes`), para que un `apply` no deshaga una decisión del autoescalador a mitad de una corrida |
| **C2, C6** | `initial_delay_sec = 600` está puesto **alto a propósito** y hay que calibrarlo con `/var/lib/mooc/arranque.json` de B3. Por debajo del arranque real, el autohealer mata instancias que se están montando |
| **G3** | La plantilla usa `e2-highcpu-2`, el mismo tipo que la VM fija de A7, para que la diferencia contra la línea base sea solo arquitectónica |

### Por qué el grupo es zonal y no regional

Un grupo regional sobrevive a la caída de una zona. **Aquí no sobreviviría:**
Cloud SQL es `availability_type = "ZONAL"` en `us-east1-b` y la cola compartida
es una VM en la misma zona. Si esa zona cae, la base se va con ella y las
réplicas de las otras zonas fallarían su comprobación de salud, porque
`/api/v1/ready` prueba la base. Lo único que sí cambiaría es el coste: el tráfico
entre zonas se factura.

**La alta disponibilidad empieza por Cloud SQL**, no por el grupo de la API:
`REGIONAL` casi dobla el coste de la instancia de base. Mientras esa decisión no
se tome, un grupo regional es teatro caro. La conversión sería cambiar a
`google_compute_region_instance_group_manager` y añadir `distribution_policy`.

### Y una trampa de operación

Si `mooc-image-tag` nombra un commit cuya imagen **no está publicada** en
Artifact Registry, el arranque falla al bajarla y el autohealer entra en bucle.
Desde C3 eso es un error del **plan** y no del arranque: dos `data` de Artifact
Registry en `bootstrap.tf` lo comprueban antes de aplicar.

### Cuándo hay que publicar, con precisión

«Publicar antes de aplicar no es opcional» se generalizó de más en la primera
versión de esta nota. **`image_tag` es una variable que se elige: no tiene que
ser el `main` del momento**, y el plan solo exige que la imagen de *ese* tag
exista. Son dos cosas distintas.

| Qué cambió | ¿Hay que publicar? |
| :--- | :--- |
| `cmd/`, `internal/`, `Dockerfile*`, `go.mod` | **Sí**: cambia la imagen |
| `docker-compose.*.yml`, `nginx.mig.conf`, `scripts/prepare_*_env.sh`, `infra/bootstrap/` | **Sí**, aunque la imagen salga idéntica |
| Documentación, evidencias, capturas, guiones de prueba | **No** |

Lo que hace que la segunda fila también exija publicar no es obvio: **el arranque
clona el repositorio en el commit que nombra `IMAGE_TAG`** (B3), así que ese tag
selecciona dos cosas a la vez —la imagen del contenedor **y** el árbol de ficheros
de despliegue—. Cambiar solo un compose sin publicar deja a la instancia clonando
un commit viejo con una imagen nueva, o al revés.

Para decidirlo sin suponer, basta comparar contra lo que corre:

```bash
{ git diff --name-only <sha-desplegado>; git ls-files --others --exclude-standard; } |
  grep -E '^(cmd/|internal/|Dockerfile|go[.](mod|sum)|docker-compose[.](prod|worker)[.]yml|nginx[.]mig[.]conf|scripts/prepare_|infra/bootstrap/)'
```

**Las dos mitades de esa orden existen por un motivo**, y la primera version de
esta receta no las tenía:

- `git diff --name-only <sha>` **sin `..HEAD`**. Con el sufijo compara commit
  contra commit y **no ve el trabajo sin commitear**, así que responde «no hace
  falta publicar» con el cambio delante. Sin él compara contra el árbol de
  trabajo, que es lo que se va a construir.
- `git ls-files --others --exclude-standard`, porque **`git diff` no muestra
  ficheros nuevos sin seguir**. Un fichero recien creado en `internal/` cambia la
  imagen y la receta anterior lo ignoraba.

Las dos fallaron de verdad al preparar C5: el cambio en `cmd/worker/main.go`
quedó invisible por lo primero y `internal/worker/readiness.go` por lo segundo,
y la comprobación dio «0 coincidencias» con dos ficheros desplegables tocados.

Sin coincidencias, el `apply` puede reutilizar el `image_tag` que ya está
desplegado. Es lo que se hizo para C4: entre el commit desplegado y `main` solo
había evidencias y notas.

<a id="35"></a>
## 35. Las instancias del grupo heredan el dominio de la VM fija, y eso rompe CSRF

`local.mooc_app_domain` vacío se calcula como
`<ip-publica-de-la-vm-fija>.sslip.io`, y `prepare_web_env.sh` deriva de ese valor
**dos** cosas:

```
APP_BASE_URL='https://${APP_DOMAIN}'
CSRF_ALLOWED_ORIGINS='https://${APP_DOMAIN}'
```

En cuanto el balanceador de C3 sirva en otra dirección, **toda petición mutante
que entre por él se rechaza como origen ajeno**. Es el mismo
`csrf_origin_rejected` 403 que costó el primer intento del escenario 2 en A7, y
se parece a un fallo de la aplicación, no a un valor de configuración.

**No bloquea los criterios de C1**, y el motivo importa: la comprobación de salud
es `/api/v1/ready`, que prueba la base y la bandera de arranque y **no mira el
origen**. Una instancia con el dominio equivocado sale sana y el grupo la recrea
igual, así que las capturas de B1, B3 y C1 se pueden tomar sin balanceador.

| Issue | Qué le toca |
| :--- | :--- |
| **C3** | Fijar `var.app_domain` a la dirección del balanceador, calculada de su dirección reservada igual que hoy se calcula la de la VM fija. **Sin eso, la plataforma responde 403 a todo lo que mute estado** |
| **D1** | Con el frontend detrás del **mismo** balanceador no hace falta CORS ni `SESSION_COOKIE_SAMESITE=none` (A4). En otro origen hacen falta las tres cosas a la vez |
| **G3, G5** | Una corrida de carga que entre por el balanceador con el dominio sin fijar mediría 403, no latencia. Comprobar el origen antes de medir |

<a id="36"></a>
## 36. Un comprobador que imprime no comprueba, y van cuatro

No es casualidad: cuatro veces en la Entrega 3 un verificador dio por bueno algo
que estaba mal, siempre por la misma razón.

| Dónde | Qué hacía | Qué se le escapó |
| :--- | :--- | :--- |
| Escritores del escenario 2 (nota 28) | publicaban conclusiones fijas | contradecían a su propia tabla en la misma página |
| Barrido de secretos (nota 33) | listaba 22 coincidencias | nadie lee un informe con ruido |
| Arranque de la cola (B1) | `CONFIG GET appendonly` **impreso** | redis corría otra configuración y el guion salía con 0 |
| Prueba de autohealing (C1) | esperaba un **nombre** de instancia distinto | un MIG reutiliza el nombre: sondeó 38 min sobre algo ya sano |

**La regla.** Un comprobador tiene que **afirmar** y **salir con código distinto
de cero** cuando la afirmación falla.

**Y hay un quinto caso que conviene distinguir de los cuatro de arriba.** Al
convertir la comprobación de la cola en asercion, el parseo quedó mal:
`redis-cli --no-raw CONFIG GET` numera la salida, así que `sed -n 2p` devolvía
`2) "yes"` y la asercion fallaba **sobre una cola correctamente configurada**.

Eso es un **falso negativo**, y no es lo mismo que los otros cuatro:

| | Falso positivo | Falso negativo |
| :--- | :--- | :--- |
| Qué dice | «esta bien» cuando no lo esta | «esta mal» cuando esta bien |
| Cuando se descubre | cuando ya fallo algo en produccion | de inmediato, el propio arranque se para |
| Coste | el que tenga el fallo que tapo | unos minutos de diagnostico |

Los dos son errores y los dos hay que arreglarlos, pero **si un comprobador va a
equivocarse, que sea por este lado**. Es la misma asimetria que
`initial_delay_sec`: errar por arriba retrasa una sustitucion, errar por abajo
mata instancias sanas en bucle. Si imprime y sigue, no es un comprobador: es
un registro, y nadie lo lee hasta que ya es tarde.

Dos corolarios que los cuatro casos ilustran:

- **Comprobar la clave que delata, no la que da la razón.** En la cola,
  `maxmemory-policy` coincidía **por casualidad** —`noeviction` es también el
  valor por omisión de Debian—, así que mirar solo esa clave no habría detectado
  nada. La que delataba era `appendonly`, cuyo valor por omisión es `no`.
- **Comprobar la identidad correcta.** Un MIG reutiliza el nombre de la
  instancia; lo que distingue una instancia nueva es el **ID** y el
  `creationTimestamp`. Afirmar sobre el atributo equivocado da un veredicto
  equivocado con datos correctos.

### Y la trampa de `systemctl` que lo destapó

`apt-get install redis-server` **deja el servicio corriendo** con la
configuración de Debian. `systemctl enable --now` **no hace nada sobre un
servicio ya activo**: no es `restart`. Resultado: el fichero en disco decía
`bind 10.0.1.8` y el proceso escuchaba en loopback, y la API moría en bucle con
`connection refused`.

| Issue | Qué tiene que saber |
| :--- | :--- |
| **C5, C6, C7** | Cualquier guion de arranque que instale un paquete con servicio y luego escriba su configuración necesita `restart`, no `enable --now`. Las unidades de B3 ya lo hacen bien |
| **G1** | Las comprobaciones que publiquen métricas tienen que afirmar y salir con código, no imprimir. Es la misma regla que A6 §7 aplicada a los verificadores |
| **I5** | Antes de etiquetar, los verificadores deben salir con 0 **por afirmación**, no por no haber mirado |

<a id="37"></a>
## 37. La CPU de la API no sirve como señal de autoscaling, y está medido

C2 (#205) descartó `cpu_utilization` con los datos de A7, no por preferencia.

| Corrida | pet/s | CPU web media/máx | CPU SQL máx | tps |
| :--- | ---: | ---: | ---: | ---: |
| A-nivel1 | 0,2 | 3,74 / 8,37 % | 12,7 % | 4,7 |
| A-nivel200 | 32,5 | 12,00 / 15,55 % | 30,7 % | 350,9 |
| B-nivel200 | **103,5** | **22,42 / 37,33 %** | 35,2 % | **1 039,7** |

A **500 veces** el caudal, la CPU pasa del 3,7 % al 22,4 %: una pendiente de unos
**0,18 puntos por petición/s**. Un objetivo del 60 % se alcanzaría sobre **310
pet/s por instancia**, y a ese caudal tres instancias pedirían del orden de
**9 000 tps a una base de 1 vCPU**.

**Con un objetivo de CPU, el grupo se queda en una instancia mientras la base se
funde.** Esta API no computa: traduce HTTP a SQL y espera.

Y la memoria es peor señal todavía: `webMem%` se quedó entre **30,1 y 30,8 %** en
las ocho corridas, de 0,2 a 103,5 pet/s. Es una constante, no una señal.

### La señal elegida y lo que acopla

`load_balancing_utilization.target = 0.8`, es decir peticiones por segundo y por
instancia. Pero **es un cociente y su denominador no está en C2**.

| Issue | Qué tiene que saber |
| :--- | :--- |
| **C3** | Tiene que declarar `balancing_mode = "RATE"` y `max_rate_per_instance = 100` en el backend service. **Sin eso el autoescalador no escala**, y con `balancing_mode = "UTILIZATION"` volvería a medir CPU por la puerta de atrás |
| **C6** | La señal de los workers es otra —profundidad y antigüedad de cola— y por el mismo razonamiento: lo que hay que medir es lo que se agota, y allí sí es el cómputo |
| **G5** | Tiene que medir si la base cede **antes** de las 3 instancias. Si lo hace, la conclusión no es «el autoescalado no funciona» sino «el siguiente cuello es la base» |
| **C1** | `cooldown_period = 300` es el mismo valor que `initial_delay_sec`, y por la misma medida de B3: 62 s de arranque, 160 s hasta sana |

### El techo de 3 instancias cae donde cae el techo de la base

Con 80 pet/s por instancia, tres cubren 240 pet/s ≈ **2 400 tps**, que
extrapolando la curva de A7 pondrían Cloud SQL cerca del **80 %** de CPU.

**Escalar la API no escala PostgreSQL**: las tres réplicas comparten una
instancia de 1 vCPU. El síntoma de ese techo es latencia creciente en todas las
réplicas a la vez **con CPU de API baja** — el patrón que delata que el cuello no
está donde se está mirando.

Es una **extrapolación lineal, no una medición**: la curva de CPU de la base por
encima de 1 040 tps no se ha medido. Subir `max_replicas` por encima de 3 no
ayudaría: añade réplicas que esperan a la misma base.

<a id="38"></a>
## 38. El balanceador es la única entrada, y encender el entorno pasa a ser explícito

C3 (#206) puso el balanceador de aplicación **global** con certificado
administrado por Google sobre `<IP-del-balanceador>.sslip.io`, y con él se fue
todo el TLS de las instancias.

### El certificado: tres cosas comprobadas antes de elegirlo

| Comprobación | Resultado |
| :--- | :--- |
| El nombre resuelve a la IP que lleva dentro | **A = la IP** ✅ |
| Registro AAAA parásito, causa habitual de `FAILED_NOT_VISIBLE` | **no hay** ✅ |
| CAA que limite quién emite | **no hay**, así que Google Trust Services puede ✅ |

El tercero importa más de lo que parece: un CAA que solo autorizara a Let's
Encrypt habría dejado el certificado en provisión **sin un error que lo
explicara**.

**Riesgos aceptados:** aprovisionar tarda hasta una hora y el 443 no sirve hasta
que está `ACTIVE`; `sslip.io` es un tercero y su caída impediría la **renovación**,
que revalida el DNS; y la IP no puede cambiar, porque el dominio la lleva dentro.

### Lo que se retiró, y por qué no era opcional

El criterio es que la API solo sea alcanzable por el balanceador. La VM web fija
tenía IP pública con 80 y 443 sirviendo la misma aplicación: un segundo camino.

Con ella se fueron su dirección, la regla `mooc-allow-web-ingress`, `nginx.conf`,
`nginx.bootstrap.conf`, `certbot/`, los montajes de `/etc/letsencrypt` y el
puerto 443 del Compose. **Ya no queda ninguna regla de cortafuegos con origen
`0.0.0.0/0` en la VPC.**

La VM del worker sigue: sin superficie pública desde B2, la sustituye C5.

> Su disco era la única copia legible de `SMTP_FROM` y `SMTP_USERNAME` sin
> conocerlos de memoria. Siguen en la metadata de la plantilla, que persiste.

### La nota 35 queda resuelta

`mooc_app_domain` pasa a ser el dominio del balanceador. Antes salía de la IP de
la VM fija, y de ese valor sale `CSRF_ALLOWED_ORIGINS`: **toda petición que
mutara estado entrando por el balanceador habría dado 403**.

### Encender el entorno es ahora un acto explícito

| Variable | Omisión | Qué hace |
| :--- | :--- | :--- |
| `lb_enabled` | `false` | crea las dos reglas de reenvío, que son la entrada pública |
| `api_autoscaling_enabled` | `false` | crea el autoescalador, cuyo `min_replicas = 1` levanta una instancia |
| `db_activation_policy` | **`ALWAYS`** | **no se pudo invertir**: la instancia tiene que existir antes de poder detenerse |

El valor por omisión de los dos primeros se invirtió a propósito: con `true`,
**cualquier `apply` desaparcaba el entorno**, y en este proyecto eso ya costó
dinero — A7 gastó 15 h de entorno encendido sin medir, la mayor parte de sus
4,88 USD.

**La tercera es la que cuesta si se olvida:** un `apply` sin
`-var db_activation_policy=NEVER` vuelve a encender la base a 0,0855 USD/h. El
plan lo dice con `will be updated in-place`.

### Por qué las reglas de reenvío se apagan pero el certificado se conserva

Una regla de reenvío se factura **por hora, sirva o no tráfico**: 0,025 USD/h, y
hay dos. Son 1,20 USD/día aparcado, veinte veces la base detenida. Pero destruir
el balanceador entero obligaría a reaprovisionar el certificado —hasta una hora—
en cada sesión.

Así que se apagan solo las reglas y se conservan dirección, certificado, backend,
mapas y proxies, que **no tienen coste por hora**. Aparcado: ~0,30 USD/día.

| Issue | Qué tiene que saber |
| :--- | :--- |
| **C4** | La sonda del backend es `/api/v1/ready`, la misma del autohealing de C1. Dos sondas distintas serían dos opiniones sobre si una réplica sirve |
| **C5** | La VM fija del worker sigue en pie; retirarla es de C5, como C3 hizo con la web |
| **C7** | `connection_draining_timeout_sec = 60` es para **peticiones**. Un worker no recibe peticiones, así que su drenaje es otro problema |
| **D1** | Con el frontend detrás del **mismo** balanceador no hace falta CORS ni `SESSION_COOKIE_SAMESITE=none` (A4); en otro origen, las tres cosas |
| **G3, G5** | La URL de medición es la del balanceador, y `lb_enabled=true` es obligatorio para que exista |
| **I3** | La URL HTTPS va enlazada desde el README, y sale de `terraform output lb_url` |
| **I6** | Al destruir, el certificado y la dirección se van con el resto. Si se recrean, el dominio cambia porque lleva la IP dentro |

<a id="39"></a>
## 39. La metadata es una copia derivada, y recuperar valores de ella los corrompe

Durante el despliegue de C3, la instancia del grupo no arrancaba:

```
/etc/mooc/web.conf: line 9: unexpected EOF while looking for matching `"'
```

La línea 9 es `SMTP_USERNAME`, y su valor tenía esta forma, con el valor real
sustituido aquí por `VALOR`:

```
SMTP_USERNAME=['["[\'VALOR\']"]']
```

**Triple envoltura.** La causa, en dos pasos:

1. `gcloud ... --format="value[](...extract(value))"` **no devuelve el valor:
   devuelve una lista literal**, `['valor']`. El `[]` del `value[]` lo provoca.
2. Esa salida se usó como **fuente de recuperación** para el siguiente `apply`,
   dos veces. Cada vuelta añadió una capa, y la comilla sin cerrar de la tercera
   rompió el `source` del fichero de configuración.

El síntoma estuvo a la vista antes de romper nada: una consulta de
`mooc-app-domain` devolvió `['136.68.55.254.sslip.io']`, con corchetes, y no se
leyó.

### El error de fondo no es la fórmula

Es **tratar la metadata de una instancia como fuente de verdad cuando es una
copia derivada**. El valor viaja `tfvars → Terraform → metadata → /etc/mooc`, y
leer del final de esa cadena para alimentar el principio es un bucle que acumula
transformaciones.

**Arreglado donde corresponde:** `smtp_from` y `smtp_username` viven en
`infra/terraform/terraform.tfvars`, que está en `.gitignore` desde la Entrega 2 y
es exactamente para lo que `terraform.tfvars.example` dice que sirve. Terraform lo
lee solo, así que el `apply` ya no necesita variables de entorno para ellos y no
hay nada que recuperar.

| Issue | Qué tiene que saber |
| :--- | :--- |
| **C5, C6, C7** | Los valores del nivel 2 de B4 salen de `terraform.tfvars`, **no** de la metadata de una instancia |
| **I5, I6** | Antes de destruir, comprobar que `terraform.tfvars` existe en la máquina de quien vaya a recrear: es la única copia fuera del estado |
| **cualquiera** | Si hace falta leer metadata, `--format="json(metadata.items)"` y parsear. `value[]` con `extract()` miente |

### Dos comprobaciones que llegaron tarde

- `prepare_web_env.sh` valida que `SMTP_FROM` y `SMTP_USERNAME` no lleven saltos
  de línea ni comillas, **pero esa validación corre después del `source`**, y el
  `source` es lo que fallaba. Una comprobación que se ejecuta después del fallo
  que debía detectar no sirve: es la nota 36 con otro disfraz.
- Al diagnosticarlo, un `terraform plan` imprimió los dos valores en claro en el
  diff de la metadata. Son del nivel 2 de B4 —no secretos, pero no publicables—
  así que **la salida de un `plan` que toque esas claves no se pega en una
  evidencia ni en un informe sin redactar**.

<a id="40"></a>
## 40. Una sola comprobación de salud para el balanceador y el autohealing tiene precio

C4 (#207) lo midió. `google_compute_health_check.api` alimenta a **dos**
consumidores: el backend service del balanceador (C3) y el autohealing del grupo
(C1). Es más simple y garantiza que lo que decide el tráfico y lo que decide la
reparación sean lo mismo. Pero:

> **Un parpadeo de 30 segundos de la aplicación no solo saca la réplica de la
> rotación: dispara una reparación de la máquina.**

Medido degradando una réplica a propósito —parando su contenedor `api`, de modo
que nginx devuelva 502 a `/api/v1/ready`:

| T+ | Salud | Acción del grupo | Tráfico |
| ---: | :--- | :--- | :--- |
| 45 s | `HEALTHY` | `NONE` | 25 de 40 peticiones |
| **98 s** | `TIMEOUT` | **`RECREATING`** | **0** |
| 292 s | `TIMEOUT` | `VERIFYING` | 0 |
| **387 s** | `HEALTHY` | `NONE` | vuelve al reparto |

- **Exclusión del balanceador: 43 s.** Cuadra con los parámetros —tres sondeos
  fallidos a diez segundos— más la propagación.
- **Reincorporación: 278 s**, cuando con `healthy_threshold = 2` deberían ser
  ~20 s.

La diferencia no es del balanceador: al restaurar el contenedor, el grupo **ya
había empezado su reparación**, y una instancia con una acción en curso no vuelve
a la rotación aunque su salud se recupere. Hay que esperar a que la acción se
despeje.

**La instancia no llegó a sustituirse**: conservó su id. La reparación se inició y
se resolvió sin crear una máquina nueva.

### Los dos mecanismos, que no son el mismo

| | Exclusión por el balanceador | Reparación por el grupo |
| :--- | :--- | :--- |
| Qué hace | deja de enviarle peticiones nuevas | repara o reemplaza la instancia |
| Escala | **decenas de segundos** | **minutos** (160–273 s en C1) |
| Reversible | sí, en ~20 s | no: una vez recreada, es otra máquina |

| Issue | Qué tiene que saber |
| :--- | :--- |
| **C6, C7** | Tienen la misma decisión del lado de los workers. Si se quiere que el autohealing **no** reaccione a fallos de aplicación transitorios, hacen falta **dos** comprobaciones: una ágil para el reparto y otra tolerante —intervalo mayor, umbral más alto— para la reparación |
| **G5** | Una réplica con `currentAction` distinto de `NONE` **no recibe tráfico aunque esté sana**. Medir reparto sin mirar esa columna da números que no se explican |
| **G7** | Los dos tiempos se publican por separado: 43 s de exclusión y 160–273 s de reparación. Promediarlos no significa nada |

### Y un detalle del reparto que conviene no malinterpretar

Con `balancing_mode = "RATE"`, el balanceador reparte **hasta la capacidad
declarada por instancia**, no a turnos. En reposo el reparto no es mitad y mitad
—se midió 25/15 de 40— y eso no es un defecto: las dos réplicas están muy por
debajo de `max_rate_per_instance = 100`, así que lo que decide es la proximidad
del proxy de Google que atiende cada conexión. Un reparto uniforme solo aparece
sosteniendo carga por encima de la capacidad de una réplica, que es G5.

<a id="41"></a>
## 41. Un worker no se comprueba como una réplica de la API

C5 (#208) tuvo que añadir una sonda al worker, porque el criterio del issue es
«proceso vivo **y conectado a la cola compartida**» y un worker no atiende
peticiones.

| Sonda | Por qué no basta |
| :--- | :--- |
| TCP al 9090 | el puerto acepta conexiones con el proceso vivo y la cola caída |
| HTTP a `/metrics` | responde 200 sin saber nada de la cola |

Las dos dan por **sana una instancia que vive y no consume**: ocupa una máquina
sin hacer trabajo, que es justo lo que el grupo debería reparar.

`internal/worker/readiness.go` añade las dos sondas que la API tenía desde A2:
`/healthz` dice que el proceso vive, y **`/readyz` hace PING sobre la misma
conexión con la que consume**. Comprobar una conexión distinta de la que trabaja
puede dar sano con el consumo roto.

**El autohealing mira `/readyz`.**

### La contrapartida, que hay que saber antes de que pase

Si la cola cae, **todas las instancias del grupo se declaran no sanas a la vez** y
el grupo intentará recrearlas. No arregla nada --el problema está fuera-- pero
tampoco empeora el servicio, porque sin cola no hay trabajo.

Es una consecuencia más del punto único de fallo de la nota 30. Quien quiera
evitarlo tiene que hacer la cola redundante, **no** cambiar la sonda: una sonda
que ignore la cola devuelve el problema del principio.

### Lo que esto obliga a los demás

| Issue | Qué tiene que saber |
| :--- | :--- |
| **C6** | La señal de escalado es la cola, pero la de **salud** es `/readyz`. Son dos cosas distintas sobre el mismo Redis, y confundirlas haría que una cola vacía pareciera un worker enfermo |
| **C7** | `max_unavailable_fixed = 0` levanta la nueva antes de retirar la vieja, pero **no espera a que termine su tarea** --A7 midió 1 711 s en el perfil largo--. El drenaje ordenado es de C7; hasta entonces una rotación tira como mucho una tarea en vuelo por instancia |
| **G4, G6** | La VM fija del worker se retiró: un consumidor fuera del grupo también saca tareas, y entonces no se puede atribuir el trabajo al grupo |
| **C6** | `initial_delay_sec = 420`, mayor que los 300 de la API porque la imagen del worker lleva FFmpeg. **Hay que calibrarlo** con `/var/lib/mooc/arranque.json` del rol worker |

### Y cómo se prueba un worker sin inventarse el trabajo

No vale encolar una tarea a mano con `redis-cli`: asynq las codifica, y una hecha
a mano fallaría al decodificar --probaría el decodificador, no el consumo--.

El camino corto ya existe: `scripts/a7_verificar_e2e.sh` hace el flujo normal de
subida --login, URL prefirmada, PUT, `complete`-- y espera a `completed`. Sube un
PDF y una imagen, que **también pasan por el worker** como trabajos de cola
(nota 16) y son mucho más baratos que un vídeo. Necesita cuentas, así que antes
hay que sembrar la base.

<a id="42"></a>
## 42. La señal que gobierna el autoscaling no se puede publicar desde el grupo

C6 (#209) tenía la métrica desde A2: `internal/worker/queue_metrics.go` publica
`worker.queue.pending`, `active` y `oldest_pending_age_seconds`. Lo que no tenía
es un sitio desde donde publicarla, y la razón es **aritmética del
autoescalador**, no una preferencia de diseño.

### Tres impedimentos, cada uno suficiente

| | Por qué el worker no puede publicar la señal |
| :--- | :--- |
| **El autoescalador SUMA** las series que devuelve su consulta | La profundidad de la cola es una magnitud **global**. Con tres workers publicándola, el autoescalador ve el triple del trabajo que hay y escala por su propio reflejo |
| La serie desaparece en una rotación | Justo cuando la situación está cambiando |
| Con `gce_instance` **no se puede filtrar** | Ver abajo: es lo que descarta el camino del agente de operaciones |

El tercero es el que obligó a cambiar de plan a mitad de C6. La documentación de
GCE sobre métricas personalizadas dice dos cosas que juntas cierran la puerta:

> Si el tipo de recurso es `gce_instance` (el predeterminado), la métrica debe
> incluir una serie temporal separada para cada instancia, y **no se puede
> filtrar por etiquetas de recurso**. Para cualquier otro tipo de recurso, la
> métrica se trata como del grupo, y sí se usa el filtrado por etiquetas.

Un exportador de Prometheus raspado por el agente de operaciones publica bajo
`gce_instance` **de la máquina donde corre**. En la VM de la cola, que no es
miembro del grupo, eso produce una serie que el autoescalador no puede
seleccionar: no es de una instancia del grupo y no admite filtro.

**Por eso el exportador escribe directamente en Cloud Monitoring** con
`CreateTimeSeries` sobre el recurso `generic_node`, que sí es filtrable. No es
una preferencia por la API sobre el agente: es la única forma de que el
autoescalador vea el dato.

### `outstanding`, y por qué no `pending`

La métrica que gobierna el escalado es
`custom.googleapis.com/mooc/queue/outstanding` = `pending` + `active`. La suma se
calcula **en el exportador** porque el autoescalador solo sabe sumar series del
**mismo** tipo de métrica: dos tipos distintos no se pueden sumar en su consulta.

Con `pending` a secas el grupo oscila, y es aritmética:

```
pending=4 -> pide 2 réplicas -> la nueva toma 2 -> pending=2 -> pide 1
          -> retira -> el trabajo en vuelo se reencola -> pending=4 -> ...
```

Un trabajo sale de `pending` en el instante en que **alguien lo empieza**, así que
`pending` baja porque hay más capacidad trabajando, y el autoescalador lee esa
bajada como que sobra. `outstanding` solo baja cuando un trabajo **termina**, así
que durante un vaciado decrece de forma monótona.

### `single_instance_assignment` es «pendientes por consumidor»

El enunciado admite «profundidad o antigüedad de la cola, **trabajos pendientes
por consumidor** u otra señal que represente la demanda». La tercera traduce
directo:

| `outstanding` | réplicas |
| ---: | ---: |
| 2 | 1 |
| 4 | 2 |
| ≥ 6 | 3 |

con `single_instance_assignment = 2`, que es `mooc-worker-concurrency`. El valor
no es arbitrario: es el número de instancias con el que todo el trabajo está
siendo atendido y nada espera. **Si alguien cambia la concurrencia, este número
cambia con ella.**

### Dos trampas que costarían un `apply` o una tarde

1. **`single_instance_assignment` y `target`/`type` son mutuamente excluyentes.**
   `type = "GAUGE"` describe bien la métrica —un nivel instantáneo— y rompe el
   `apply`: la API de Compute acepta un modo o el otro, no los dos.
2. **Un filtro que no devuelve nada no es un error.** El autoescalador sin datos
   **mantiene el tamaño actual**: no baja al mínimo ni se desborda. El grupo se
   queda congelado. Por eso el espacio de nombres sale de
   `local.queue_metric_namespace` y lo leen los dos sitios —el filtro del
   autoescalador y el entorno del exportador— y por eso hay un `output` con el
   filtro literal para pegarlo en el Explorador de Métricas.

### La CPU no sirve aquí, y por la razón contraria a la de C2

A7 midió el worker al **96,31 % de CPU media durante casi dos horas**. Un worker
que transcodifica está saturado mientras tenga **un** trabajo, así que la CPU no
distingue «uno en curso» de «cien esperando»: con un objetivo de CPU el grupo
pediría el máximo por un solo vídeo y lo mantendría hasta vaciar la cola.

En C2 la CPU no servía porque **no se movía** (0,18 puntos por petición/s). Aquí
no sirve porque **está siempre arriba**. Misma conclusión, causas opuestas.

### Lo que esto cambia para los demás

| Issue | Qué tiene que saber |
| :--- | :--- |
| **B4** | Su inventario decía que `sa-queue-server` «no lee secretos y no baja imágenes». La primera mitad sigue en pie; la segunda no: C6 le da `artifactregistry.reader` (solo lectura) porque el exportador es un binario de la imagen `worker`. Corregido en `artifact_registry.tf` |
| **B1** | La VM de la cola pasó de `metadata_startup_script` a `metadata["startup-script"]`. El motivo es que **guarda estado** —el AOF— y el primero es ForceNew. El cambio cuesta **una** recreación, la última; desde entonces tocar el arranque se aplica reiniciando la VM, que conserva el disco |
| **C7** | `scale_in_control` de 1 réplica / 900 s **no es un drenaje**. Acota la frecuencia del daño a un trabajo en vuelo por ventana, y 900 s no cubren los 1 711 s del perfil largo de A7 ni pretenden |
| **G6** | `cooldown_period = 420` es una **estimación**, el mismo número que `initial_delay_sec` de C5 porque contestan la misma pregunta. Hay que calibrarlo con `/var/lib/mooc/arranque.json` del rol worker |
| **G1, G5, G6** | La nota 10 aplica entera: esta métrica personalizada **caduca**. Hay que exportar los puntos crudos de cada corrida al repositorio en el momento |
| **F2** | `monitoring.metricWriter` de `sa-queue-server` **deja de ser un permiso sin uso**: lo consume el exportador. La nota 31 lo tenía marcado como el único sin uso de las tres cuentas |

---

## 43. El estado del autoescalador miente dos veces al encenderlo

**Contexto:** sesión de despliegue de C5 y C6 (#208, #209), 29 de octubre de
2026. `terraform apply` salió limpio —11 añadidos, 2 cambiados, 3 destruidos— y
las tres cosas que siguen habrían pasado por fallos del diseño cuando no lo son.

### `gcloud compute autoscalers` no existe

```
$ gcloud compute autoscalers describe mooc-worker-autoscaler --zone=us-east1-b
ERROR: (gcloud.compute) Invalid choice: 'autoscalers'.
```

No es una ruta mal escrita ni un permiso: el SDK instalado —**586.0.0**, core
2026.09.20— no trae ese grupo de comandos. El único sitio donde este gcloud
expone el estado es el campo `autoscaler` del grupo gestionado, y trae lo mismo:

```bash
gcloud compute instance-groups managed describe mooc-worker-group \
  --zone=us-east1-b --project=plataforma-mooc-entrega2 \
  --format="value(autoscaler.status,autoscaler.recommendedSize,autoscaler.statusDetails[0].message)"
```

Importa porque `scripts/c6_prueba_autoescalado.sh` leía el estado por el camino
que no existe, y su comprobación «¿existe el autoescalador?» se cumplía siempre:
el guion abortaba **antes** de generar carga, diciendo que faltaba un recurso que
estaba creado. Corregido.

Y un detalle del formato, que es el que hacía falsa la comprobación: si el grupo
existe y **no tiene** autoescalador, `--format=value(...)` no imprime una línea
vacía sino **una línea de tabuladores**, que para `[ -z ]` no está vacía. De ahí
el `sed 's/^[[:space:]]*$//'` del guion.

### `CUSTOM_METRIC_INVALID` recién creado es transitorio

Durante los primeros minutos tras el `apply`:

```
status        : ERROR
statusDetails : The monitoring metric that was specified does not exist
                or does not have the required labels. Check the metric.
```

Y a la vez la serie **sí existía**: 18 puntos, el filtro del autoescalador
devolviéndola, el descriptor como `GAUGE`/`INT64` con `generic_node` entre sus
`monitoredResourceTypes`. GCE valida la métrica al crear el recurso y deja ese
veredicto pegado en `statusDetails` hasta el primer sondeo que la lee bien.
**Medido: `ACTIVE` y `statusDetails: null` unos 4 minutos después**, sin tocar
nada.

Es la cara opuesta de la trampa de la nota 42: allí el peligro es un filtro vacío
que **no** da error; aquí es un error que no significa nada. Mientras dura, el
grupo se queda en `min_replicas`, que es su comportamiento normal con la cola
vacía, así que no hay forma de distinguirlo mirando el tamaño. Lo que lo decide
es leer la serie, y para eso está la fase 1 del guion.

### El `apply` que enciende C5 retira la VM suelta del worker

Entre los 3 destruidos hay uno que la tabla del runbook no listaba:
`google_compute_instance.worker_server` (`mooc-worker-server`), la VM suelta que el grupo gestionado de C5
sustituye. Es intencionado y no se lleva nada —un worker es apátrida: consume de
la cola y escribe en GCS— pero el runbook decía «si propone destruir algo más,
**para**», así que su propia regla disparaba un falso positivo. Fila añadida.

Los otros dos sí estaban previstos: `mooc-queue-server` (recreación por el paso a
`metadata["startup-script"]`, nota 42) y la plantilla de la API del 27-oct, que se
rehace porque `var.image_tag` forma parte de ella —`name_prefix` más
`create_before_destroy`, así que la sustitución es segura—.

### Lo que esto cambia para los demás

| Issue | Qué tiene que saber |
| :--- | :--- |
| **G1, G5, G6** | Cualquier guion que lea el estado de un autoescalador tiene que ir por el grupo, no por `gcloud compute autoscalers` |
| **C7** | Al crear o recrear el autoescalador, los primeros minutos de `statusDetails` no son evidencia de nada |
| **D1** | El `apply` que enciende el entorno recrea la plantilla de la API cada vez que cambia `image_tag`. Es lo esperado y la sustitución es segura |

---

## 44. El plazo de drenaje vivia en dos capas que no se hablaban

**Contexto:** C7 (#210). El apagado ordenado del worker no habia que
construirlo: existia entero en asynq y estaba mal configurado.

### 8 segundos contra 120

`mooc-worker.service` le concede al contenedor 120 s para parar
(`down --timeout 120`) y a la unidad 180 s (`TimeoutStopSec`). B3 los puso a
proposito y dejo escrito alli que «la politica de verdad la decide C7».

Lo que no se veia desde la unidad: `asynq` v0.26.0 tiene
`defaultShutdownTimeout = 8 * time.Second` y `NewWorkerEngine` **nunca fijaba
`ShutdownTimeout`**. La peor combinacion posible — la capa de sistema esperando
dos minutos a un proceso que se rindio a los ocho segundos.

Es la clase de fallo que no aparece en ninguna prueba y que nadie nota operando:
el servicio para «bien», solo que tirando trabajo que iba a terminar.

### `requeue` no es `handleFailedMessage`

Leyendo el codigo de asynq para configurarlo bien aparece la distincion que
sostiene todo el criterio del enunciado:

| Camino | Cuando | Consume reintento |
| :--- | :--- | :---: |
| `requeue()` | vence el plazo de drenaje (`close(p.abort)`) | **no** |
| `handleFailedMessage()` | el manejador devuelve error, o vence el *lease* | si |

Importa porque una instancia que el autoescalador retira **no ha fallado en
nada**. Si cada retirada gastara uno de los tres intentos, tres reducciones
seguidas mandarian a la cola de muertos un trabajo perfectamente sano. Con
`scale_in_control` a una replica cada 900 s (C6), tres reducciones son 45
minutos: perfectamente alcanzable en una tarde.

### La idempotencia de produccion era un mapa en memoria

`cmd/worker/main.go` no pasaba `WithIdempotencyStore`, asi que el motor caia al
`MemoryIdempotencyStore` por omision. El `RedisIdempotencyStore` existia
completo y sin usar.

Con un worker suelto casi no se notaba. Desde C5 el worker es un grupo de 1 a 3,
y **el trabajo que un drenaje devuelve a la cola lo recoge otra instancia**, que
no ve ninguna marca.

Sin exagerar el hallazgo: los efectos duplicados si estaban cubiertos, por otro
sitio. `mediaproc.go` comprueba contra la base que el recurso no este ya
`completed` sobre el mismo objeto, y el prefijo de los derivados sale de
`storage.HLSPrefix(resource.StableID)`, que es determinista — un reintento
**sobrescribe** los mismos objetos. El middleware en memoria era una red de
seguridad que no estaba atada a nada, no la unica defensa.

### Una tarea interrumpida decia `failed`

Al cancelarse el contexto el manejador devuelve `context.Canceled`, y
`LoggingMiddleware` registraba `estado=failed` sobre un trabajo sano que iba a
volver a la cola. Como la evidencia que bloquea C7 es **precisamente el registro
del drenaje**, esa linea convertia la prueba en su contrario. Ahora sale como
`estado=interrumpido` con el motivo; lo que devuelve el manejador no cambia.

### `graceful_shutdown` existe y el provider fijado no lo expone

Compute Engine permite un apagado ordenado **de 1 segundo a 6 horas**, y la
restriccion documentada para grupos administrados es que no puedes *saltartelo*
cuando el autoescalador borra una instancia — no que no se pueda usar. Con eso,
un worker retirado terminaria incluso el perfil largo de 1 711 s.

El provider esta fijado en `google 8.4.0` y su esquema **no lo tiene**, ni en
`google_compute_instance_template` ni en `google_compute_instance` (comprobado
con `terraform providers schema -json`). Subirlo a mitad de entrega cambia el
plan de los 68 recursos del estado compartido. Queda anotado: es la mejora de
mayor efecto por menos codigo de todo el bloque C.

### Lo que esto cambia para los demas

| Issue | Que tiene que saber |
| :--- | :--- |
| **E1** | La concurrencia sigue en 2 por instancia y el techo real son **6** transcodificados simultaneos (3 instancias x 2). Los temporales siguen en el volumen `mooc_worker_scratch`, con limpieza por tarea, por `defer` de tarea interrumpida y por `ExecStartPre` |
| **G6** | El ciclo de escalado ahora incluye un drenaje de hasta 90 s por replica retirada. Una medida de «cuanto tarda en reducir» que lo ignore se queda corta |
| **C6** | `single_instance_assignment = 2` y `WORKER_CONCURRENCY=2` son el mismo numero. Si alguien cambia uno, cambia el otro |
| **F2** | El worker usa ahora el Redis de la cola tambien para las claves `idempotency:worker:`. No es una cuenta ni un permiso nuevo: es la misma conexion |
| **G1, G5** | Redis en una sola VM sigue siendo el punto unico de falla del procesamiento: sin el no hay cola, no hay consumo, y el autoescalador de C6 se queda sin señal y **congela** el tamaño del grupo |

---

## 45. El punto de acceso de la arquitectura desplegada

**Punto de acceso:** https://136.68.55.254.sslip.io

**Documentacion navegable:** https://136.68.55.254.sslip.io/api/docs

Es la unica entrada publica (nota 38): todo pasa por el balanceador. La
direccion global esta reservada, asi que el dominio **no cambia** aunque el
entorno se apague y se vuelva a encender; lo que se va al aparcar son las reglas
de reenvio, y mientras no esten la URL no responde.

Las 57 rutas estan verificadas contra este entorno. El detalle, con lo que el
frontend necesita saber antes de escribir el cliente, esta en
[`VERIFICACION_ENDPOINTS.md`](VERIFICACION_ENDPOINTS.md).

| Issue | Que tiene que saber |
| :--- | :--- |
| **D1-D5** | Este es el origen contra el que se construye. El frontend solo puede llamar desde un origen que este en `CSRF_ALLOWED_ORIGINS`, que hoy es unicamente el del balanceador |
| **I3** | Es la URL que va al README de la entrega |
| **G1, G3-G7** | Es el destino de las corridas de carga |

<a id="46"></a>
## 46. El frontend se sirve en el mismo origen, y la API no tiene «mis cursos» ni «quién soy»

**Afecta a:** D2, D3 (consumen el cliente y la sesión), A4 y E1 (origen y cookie), B3, C1 y C3
(la imagen del frontend entra en la plantilla y el balanceador la expone).

**Decisión de D1 (#211): React + Vite + TypeScript, servido por nginx en el mismo origen que la API.**
Aplica la recomendación de la nota 22: con el mismo origen la cookie `SameSite=Lax` viaja sola,
no hay CORS, y la lista de `CSRF_ALLOWED_ORIGINS` solo necesita el origen público. El frontend
se compila a archivos estáticos y los sirve una imagen propia (`Dockerfile.web`, plantilla
`nginx.web.conf`) que reenvía `/api/` a la API. Cada instancia del grupo elástico puede llevar la
misma imagen: **la versión del frontend viaja con la de la plantilla**. Un bucket detrás del
balanceador sigue siendo posible, pero obligaría a coordinar la regla de ruta con C3.

**Lo que B3, C1 y C3 deben saber** (no está resuelto en D1, porque depende de la infraestructura nueva):

- La imagen escucha en **HTTP, puerto 80**; el HTTPS lo termina el balanceador. Variables:
  `API_UPSTREAM` (por defecto `api:8080`) y `STORAGE_ORIGINS` (por defecto
  `https://storage.googleapis.com`, para la política de contenido).
- `docker-compose.prod.yml` **no se tocó**: sigue con el nginx de la Entrega 2. Para que una
  instancia sirva el frontend hay que sustituir ese servicio por esta imagen (o montar el build) y
  decidir quién termina el TLS. Esa integración es de B3 y C1.
- Detrás de un balanceador, `X-Forwarded-For` ya no trae la IP del cliente sino la del balanceador.
  `nginx.web.conf` conserva el criterio de `nginx.conf` (la IP del par, sin fiarse del cliente);
  si C3 necesita la IP real para el límite de login, hay que decidir qué cabecera se acepta y de
  quién. Es el mismo problema que la nota 11 del límite por IP.
- **Origen que debe aceptar la API:** `CSRF_ALLOWED_ORIGINS` = la URL HTTPS pública del
  balanceador (exacta, con esquema y sin barra final). Con otro valor, todo POST con cookie da
  `403 csrf_origin_rejected` y la interfaz lo muestra como «origen rechazado».

**Dos huecos del contrato que D2 y D3 van a tocar** (verificado en `api/openapi.yaml` y en
`internal/http/server.go` de `main`, 2026-10-26):

1. **No hay forma de listar los cursos propios de un profesor.** `GET /courses` es solo el catálogo
   de publicados y no admite filtro por autor. «Consultar sus cursos» es flujo obligatorio del
   profesor y los borradores no salen en el catálogo: **D3 no se puede completar sin un endpoint
   nuevo** (p. ej. `GET /courses?mine=true` o `GET /me/courses`).
2. **No hay `GET /me`.** El login es lo único que entrega el usuario. Mientras no exista, el
   frontend guarda ese usuario (sin token) en `sessionStorage` para sobrevivir a una recarga, y
   trata cualquier `401` como sesión vencida. Es una ayuda de interfaz: la autoridad es la cookie
   y el servidor.

**El cliente nunca lee el token del login.** La respuesta lo trae en el cuerpo, pero el frontend se
autentica solo con la cookie `HttpOnly`: un XSS no tiene un token que robar. La CSRF la cubre la
comprobación de origen de la API (el navegador manda `Origin` en toda escritura del mismo origen).

**Trampas:**

- `vite build` pone en el bundle la dirección de relleno `http://localhost` de `react-router`, y un
  campo `internal`. No son nuestras. `frontend/scripts/check-bundle.mjs` no busca `localhost` a
  secas sino con puerto o loopback, ni `.internal` suelto; la construcción de la imagen **falla**
  si el bundle lleva claves, cadenas de conexión o direcciones privadas.
- El contexto de Docker no incluye `.git`: el commit llega por `--build-arg APP_COMMIT`. Un build
  local con cambios sin commitear se marca `-sin-commit` para que la versión visible no afirme
  más de lo que es.
- `Dockerfile.api` copia todo el repositorio: `frontend/node_modules` y `frontend/dist` están en
  `.dockerignore` para no meterlos en su contexto.

<a id="47"></a>
## 47. Interfaces del estudiante y del profesor: trampas que ya costaron tiempo

Qué hace cada interfaz está en las evidencias de [D2](./evidencias/D2/README.md), [D3](./evidencias/D3/README.md),
[D4](./evidencias/D4/README.md) y [D5](./evidencias/D5/README.md). Aquí, lo que otro del equipo debe saber.

| Trampa | Qué pasa | Qué hace la interfaz | A quién afecta |
| :--- | :--- | :--- | :--- |
| `processing_status` por defecto es `completed` | Un recurso recién creado, **sin archivo**, ya figura como «disponible» (nota 8: `completed` del código = «available» del enunciado) y pasa la validación de publicación | Para tipos con archivo exige `object_key`; sin él muestra «sin archivo», no «procesando» ni «disponible» | A1, A5, G1, quien valide la publicación |
| `GET /courses/{id}` es público para cursos publicados | La interfaz del profesor podía abrir el editor de un curso ajeno | Verifica que el curso esté en `GET /me/courses`; el servidor igual responde 403 a toda escritura | D3, D4 |
| `GET /me/courses` es un endpoint nuevo | Sin él el profesor no puede ver sus borradores | Rama `feat/213-api-mis-cursos-profesor`, cambio aditivo en `api/openapi.yaml`; aprobado por Fredy (2026-10-26) | A2, D3 |
| No existe `GET /me` | El usuario solo llega en el login | Se guarda sin token en `sessionStorage`; cualquier 401 = sesión vencida | D1 |
| Las inscripciones no traen el id del curso | No se puede ir de «mis inscripciones» al curso | `courseIndex()` recorre el catálogo para mapear el id estable al curso | D2 |
| Envío duplicado concurrente del cuestionario | El segundo envío recibe 409 | Guardia `useRef` contra doble clic en enviar, inscribirse, crear curso/módulo/recurso | D2, D3 |
| URL firmada con host interno | En Compose la URL de subida apuntaba a `minio:9000`, inalcanzable desde el navegador | Local: `S3_ENDPOINT=http://host.docker.internal:9000` y `STORAGE_ORIGINS` acorde. En la nube la URL firmada debe ser pública (B1/B2) | A5, B1, E1 |
| La CSP del frontend lista los orígenes de almacenamiento | Subir o reproducir desde un origen no listado falla en silencio | `STORAGE_ORIGINS` y `HLS_BASE_URL` de `nginx.web.conf` | C3, E1 |
| La BD local sin migraciones 7 y 8 | El cuestionario responde 500 al abrir un intento | Aplicar todas las migraciones antes de probar | A1 |
| TypeScript 7 y OneDrive | Los archivos con punto de reanálisis de OneDrive se saltan del globbing | `tsconfig.json` lista `files` explícitos | quien compile en OneDrive |
| Transcodificación lenta en local | 90 s de video a 720p tardaron 2 m 49 s | No se capturó «Procesando → Disponible» en vivo; hacerlo en la nube | D3 |

<a id="48"></a>
## 48. Un estudiante inscrito no puede abrir un PDF, una imagen ni un archivo descargable

Encontrado el 2026-10-31 al recorrer el aula contra el balanceador (D2). **Afecta a:** D2, D5, G1, A5.

`GET /api/v1/media/resources/{id}/download-url` está montada con `requireAuthor` y `authorizeOnResource` solo
deja pasar al **autor del curso o a un administrador** (`internal/media/service.go`). Un estudiante inscrito
recibe `403 forbidden`, siempre. El video no se ve afectado: el HLS sale del bucket público de derivados. Pero
los tipos con archivo que no son video (`pdf`, `presentation`, `downloadable`, `image`, `audio`) **no tienen
ninguna ruta de lectura para el estudiante**, y el enunciado pide «consumir contenido multimedia y documentos».

| Qué | Estado |
| :--- | :--- |
| La interfaz | Lo dice: «Este archivo no se puede descargar», sin pantalla en blanco. Y el aviso global de 403 ya no sigue al estudiante a la pantalla siguiente |
| Arreglo | Rama `fix/media-descarga-estudiante`: la ruta pasa a `requireAuth` y `DownloadURL` autoriza al estudiante con inscripción activa (`EnrollmentRepository.IsActive`) en un curso **publicado**; el resto, 403, también si el recurso no tiene archivo. Pruebas en `internal/media`. **Cambio de la API: hay que republicar la imagen y aplicar** |
| Quién | Es del módulo de media (Fredy) |

<a id="49"></a>
## 49. El generador de carga sale de una VM propia, y lo que mide el balanceador contrasta lo que mide JMeter

**Afecta a:** G1 (#222), G3, G4, G5, G6, G7. Escrito el 2026-11-01.

| Decisión o trampa | Qué hay que saber |
| :--- | :--- |
| **La línea base de A7 se midió desde un portátil** | `destino.json` dice `DESKTOP-…` y A7 documentó que esa red no reproduce lo publicado. G3 y G4 salen de una **VM en la misma región** (`infra/terraform/generator.tf`); esa diferencia hay que **declararla** en G7, no esconderla |
| **El generador está apagado por omisión** | `generator_enabled = false`: no existe ni su cuenta de servicio. Encenderlo es un `apply` con los `-var` del entorno encendido (nota 38) más `-var generator_enabled=true`. Cuesta ~0,13 USD/h; se apaga al terminar |
| **Sin IP pública** | sale por la NAT de B2 y se entra por IAP. No abre ningún puerto: F1 no encuentra nada nuevo. Su cuenta solo escribe registros y métricas |
| **JMeter se fija por digest** | la línea base usó `justb4/jmeter:latest`, que resolvía a **JMeter 5.5**, `sha256:088ac52b…`. `run_escenario1.sh` acepta `JMETER_IMAGE`; en la VM va el digest. Un `latest` que cambie de versión entre campañas rompe «misma herramienta y versión» |
| **Qué métricas existen** | `custom.googleapis.com/mooc/queue/{pending,active,outstanding,oldest_pending_age_seconds}` (C6), `prometheus…/worker_jobs_{processed,failed}_total`, `loadbalancing…/https/{request_count,total_latencies,backend_latencies}`, `compute…/instance_group/size`, `autoscaler…/{current_utilization,recommended_size}` y las del agente de las VM |
| **Las instancias que el grupo destruyó** | Monitoring solo guarda su id numérico. `capacity_metricas_gcp.py` recupera su nombre del **registro de auditoría** de su creación (`instances.insert`); sin ese permiso de lectura quedan como «otro» |
| **«Existentes» no es «operativas»** | las primeras son el tamaño del grupo; las segundas, las instancias que mandan métricas ese minuto. Durante una rotación o un arranque difieren, y es justo lo que G5 y G6 quieren ver |
| **Un percentil del balanceador es por minuto** | Cloud Monitoring no da el percentil de toda la ventana. Contrasta; **no sustituye** al de JMeter. Con menos de 20 puntos el p95 se publica como `null` (nota 25) |
| **El conteo del balanceador se lee con margen** | un punto de Monitoring por minuto, y los que cruzan el borde de la ventana se descartan: sin margen se pierde el primer y el último minuto. `capacity_metricas_gcp.py` lee 2 min antes y 4 después. El conteo es de **todos** los clientes: es una cota inferior de lo que envió el generador |
| **Se compara contra lo ENVIADO, no contra la tabla** | el balanceador cuenta también el calentamiento que el resumen omite. `capacity_contraste_generador.py` cuenta las filas del `.jtl` |
| **Un portátil perdió el 20 % de las solicitudes y la VM no** | la misma calibración (20 usuarios, 3 min): 80 % contadas desde el portátil, 102 % desde la VM. Una razón más para medir G3 y G4 desde la VM |
| **El generador se rechaza solo** | `capacity_contraste_generador.py` cruza CPU/memoria, brecha de latencia contra el balanceador y solicitudes contadas. Sin datos **no** da la corrida por buena. Umbrales fechados en `capacity-planning/criterios_de_medicion.md` |
