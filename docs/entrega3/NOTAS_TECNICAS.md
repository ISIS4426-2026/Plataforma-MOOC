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
base retrospectiva. Eso necesita un **prefijo de estado remoto distinto**, o
arrasará con lo que haya de la Entrega 3.

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

`docker-compose.worker.yml` incluye el `redis` que hace de cola de asynq. En la
Entrega 2 eso era aceptable porque la VM worker era fija. En la Entrega 3 esa
máquina pertenece a un grupo elástico que puede destruirla, y el enunciado lo
prohíbe explícitamente: la cola debe estar fuera de ambos grupos.

Ojo con el efecto colateral: la señal de autoscaling de workers (C6) se calcula
sobre la cola. Si el exportador de esa métrica vive en una instancia del grupo,
el autoscaler se queda ciego justo cuando reduce a una instancia. **Tiene que
exportarse desde fuera de los grupos elásticos.**

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

---

Este archivo no contiene credenciales, llaves ni secretos.
