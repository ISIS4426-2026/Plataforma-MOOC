# Evidencia A2 — Readiness, identificador de instancia y tiempo de arranque ([#194](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/194))

| Criterio de aceptación | |
| :--- | :---: |
| El endpoint devuelve 200 solo cuando la instancia puede atender, y 503 cuando no | ✅ §2 |
| La respuesta no revela secretos, versiones ni topología interna | ✅ §3 |
| Para cada petición se puede saber qué instancia la atendió | ✅ §4 |
| El tiempo de arranque hasta listo queda medido y documentado | ✅ §5 |

---

## 1. Por qué hacen falta dos sondas y no una

`GET /api/v1/health` existía, pero servía de healthcheck de Docker Compose. El
despliegue elástico tiene **dos consumidores con consecuencias distintas**:

| Quién pregunta | Qué hace si falla |
| :--- | :--- |
| El grupo administrado (autohealing) | **destruye y recrea** la máquina |
| El balanceador | deja de enviarle tráfico, sin tocarla |

Darles la misma sonda es caro. Si la comprobación del grupo dependiera de la
base de datos, **un parpadeo de Cloud SQL haría fallar a las tres réplicas a la
vez y el grupo las recrearía todas**. Un problema transitorio de una dependencia
compartida se convertiría en una caída completa, y encima la capacidad nueva
arrancaría contra la misma base que sigue sin responder.

De ahí el reparto:

| Ruta | Quién la mira | Prueba |
| :--- | :--- | :--- |
| `/api/v1/live` | autohealing del grupo (C1, C5) | nada |
| `/api/v1/ready` | health check del balanceador (C3, C4) | la base, más la bandera de arranque |
| `/api/v1/health` | Compose, arnés E2E, Postman | la base, con detalle por dependencia |

`/api/v1/health` **se queda como estaba**. Es la vista interna y la usan el
healthcheck de Compose, el arnés E2E y la colección de Postman, que afirman
sobre `services.database`. Cambiarla habría roto evidencia de la Entrega 2 sin
ganar nada.

## 2. Las sondas responden lo que deben

Corrida real contra el binario, parando el contenedor de PostgreSQL a mitad:

```
=== con la base arriba ===
  live   200
  ready  200

=== con la base caída ===
  live   200  {"status":"pass"}
  ready  503  {"status":"fail"}
  health 503  {"status":"fail","timestamp":"...","services":{"database":"down"}}
```

**Liveness aguanta la caída de la base y readiness no.** Eso es justo lo que
evita que una incidencia de Cloud SQL se lleve por delante el grupo entero.

Hay un tercer caso que no se ve ahí y que cubre
`TestReadinessFallaAntesDeEstarListo`: la ventana en la que el proceso vive pero
todavía se está montando. Durante ella `/ready` responde 503 **aunque la base
conteste**, porque mandarle tráfico sería repartir errores. `Shutdown` hace lo
simétrico: baja la bandera antes de drenar, para que el balanceador retire la
réplica mientras las peticiones en curso terminan.

## 3. Las sondas no revelan nada

El cuerpo es `{"status":"pass"}` o `{"status":"fail"}`. Ni el nombre de la
dependencia que falló, ni versiones, ni direcciones internas. Quien consulta
estas rutas es una sonda automática que solo mira el código de estado; el
detalle vive en `/api/v1/health`, que es la vista interna, y en el log.

`TestLasSondasNoRevelanNada` lo fija: recorre las dos rutas, con la dependencia
sana y caída, y falla si el cuerpo contiene `database`, `postgres`, `redis`,
`sql`, `bucket`, `storage`, `version`, `host`, `addr`, `port` o `instance`. El
caso de fallo usa a propósito un error con texto delator —*«se cayó la base de
datos en el host interno»*— para comprobar que no se filtra al cuerpo.

## 4. Qué réplica atendió cada petición

Sin esto no hay forma de demostrar que el balanceador reparte, que es lo que
piden **C4** y el **escenario 3A**. El enunciado lo admite de forma expresa:
*«para identificar los destinos se podrán utilizar logs correlacionados o un
identificador técnico de instancia en las evidencias»*.

Toda respuesta lleva `X-Instance-ID`:

```
X-Instance-Id: 3c065ef8bf12
```

**Lo que sale fuera es un resumen, no el nombre.** El nombre que GCE da a una
instancia de un grupo administrado delata el nombre del grupo y el sufijo que lo
numera; eso es topología interna. El resumen basta para distinguir réplicas y
agrupar peticiones por destino, y no dice cómo se llama la máquina ni cuántas
hay.

El nombre completo sí va al log, donde solo lo lee el equipo:

```json
{"msg":"http request","instance":"mooc-api-grupo-7h3k","request_id":"d55ca782-…",
 "method":"GET","path":"/api/v1/ready","status":200,"trace_id":"6dffc9eb…"}
```

La asimetría es deliberada: correlacionar en Cloud Logging necesita el nombre
real; la evidencia que se publica, no. Se etiqueta el logger al construir el
servidor, así que **toda** línea lo lleva, no solo las del log de acceso.

El valor lo fija `INSTANCE_ID`. En la nube lo resuelve
`scripts/prepare_web_env.sh` desde el servidor de metadatos:

```bash
curl -fsS --max-time 2 -H 'Metadata-Flavor: Google' \
  'http://metadata.google.internal/computeMetadata/v1/instance/id'
```

El **id numérico, no el nombre**: es único en el proyecto y estable entre
reinicios del contenedor, mientras que un grupo administrado reutiliza nombres.
Si la consulta falla no se aborta el arranque —la API cae en el nombre de la
máquina, que ya distingue réplicas—, porque negarse a levantar un servidor web
por una consulta de metadatos sería el intercambio equivocado.

## 5. Tiempo de arranque

```json
{"msg":"api server ready","instance":"mooc-api-grupo-7h3k","addr":":18081",
 "instance_public":"3c065ef8bf12","startup":"93ms",
 "ready_criterion":"el puerto acepta conexiones y las dependencias estan construidas"}
```

| | |
| :--- | :--- |
| Criterio de «listo» | el puerto acepta conexiones **y** las dependencias están construidas |
| Medido desde | el arranque del proceso, no desde `NewServer` |
| Valor en local | **93 ms** · binario compilado, PostgreSQL y Redis en contenedores locales |

El reloj arranca en una variable de paquete, que Go inicializa antes de `main()`.
Medirlo desde la construcción del servidor daba **2 ms** y dejaba fuera lo que de
verdad tarda: abrir el pool de PostgreSQL, conectar con Redis y construir el
cliente de almacenamiento.

**Este número no es el que calibra el autoscaler.** El de la nube incluye
arrancar la VM, bajar la imagen de Artifact Registry, leer los secretos y
levantar los contenedores; lo mide **B3** y es el que alimenta el periodo de
inicialización de **C2** y **C6**. Los 93 ms son solo la parte de aplicación, y
sirven para saber que no es ahí donde está el coste.

Si el periodo de inicialización se fija por debajo del arranque real, el
autoscaler cuenta como sobrecargada una réplica que todavía se está montando y
pide otra.

## 6. El arranque no depende de ficheros de una ejecución anterior

Comprobado al levantar el binario con el directorio de trabajo limpio: la API no
lee ningún estado local previo. Todo lo que necesita son variables de entorno,
PostgreSQL, Redis y el almacenamiento de objetos. Es lo que permite que una
instancia recién creada por el grupo quede lista sin intervención.

## 7. Pruebas

Diez casos nuevos, ninguno necesita nube:

| `internal/http/handler/readiness_test.go` | |
| :--- | :--- |
| `LivenessNoDependeDeLaBase` | 200 aunque la base no responda |
| `ReadinessPasaConLaBaseArriba` | 200 y `status: pass` |
| `ReadinessFallaConLaBaseCaida` | 503, que es lo que saca la réplica de rotación |
| `ReadinessFallaAntesDeEstarListo` | 503 mientras la instancia se monta |
| `LasSondasNoRevelanNada` | ni una palabra prohibida en el cuerpo |

| `internal/http/middleware/instance_test.go` | |
| :--- | :--- |
| `InstanceIDSaleEnLaCabecera` | la cabecera lleva el resumen |
| `DosInstanciasSeDistinguen` | sin esto el reparto no sería demostrable |
| `LaIdentidadEsEstable` | el mismo proceso no cambia de identidad entre peticiones |
| `LaCabeceraNoDelataElNombreDeLaMaquina` | resumen fuera, nombre real en el log |
| `SiempreHayIdentidad` | una réplica sin identidad es la que no se puede rastrear |

Más cuatro pasos en el tramo 0 del arnés E2E: las dos sondas, que el cuerpo
público no filtra, y que cada respuesta dice qué réplica la atendió. **Esos
cuatro no se han ejecutado contra la nube**: el entorno sigue apagado.

---

Este archivo no contiene credenciales, llaves ni secretos.
