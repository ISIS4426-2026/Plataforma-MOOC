# Evidencia A3 — Migraciones controladas ([#195](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/195))

| Criterio de aceptación | |
| :--- | :---: |
| Tres instancias simultáneas aplican el esquema una sola vez y ninguna falla | ✅ §2 |
| El procedimiento vive en el script de arranque, no en pasos manuales | ✅ §4 |
| Documentado qué hacer si una migración falla a mitad de una rotación | ✅ §5 |

---

## 1. El defecto, reproducido

Con una sola VM daba igual quién migrara: había uno. Un grupo administrado puede
levantar tres instancias a la vez, y las tres arrancarían con el mismo esquema
pendiente.

El aplicador anterior leía qué faltaba y después lo aplicaba. Entre esas dos
cosas hay una ventana, y dos instancias caben dentro. **Reproducido contra una
base limpia, tres ejecuciones simultáneas del aplicador tal y como estaba:**

```
  instancia 2 -> codigo de salida 3
  instancia 3 -> codigo de salida 3
  instancia 1 -> codigo de salida 0

psql:/repo/migrations/000001_init_schema.up.sql:13: ERROR:  duplicate key value
  violates unique constraint "pg_type_typname_nsp_index"
DETAIL:  Key (typname, typnamespace)=(users, 2200) already exists.
```

**Dos de tres instancias no habrían arrancado.** Y el grupo administrado las
habría recreado para que fallaran exactamente igual, en bucle.

## 2. El arreglo, y la prueba del criterio

Cada migración toma un **advisory lock de transacción** antes de comprobar si ya
está aplicada, y la comprobación ocurre **dentro** del lock:

```
  1. BEGIN                   -- lo pone --single-transaction
  2. tomar el lock           -- la segunda instancia espera aquí
  3. preguntar si ya está    -- ya con el lock en la mano
  4. aplicar y registrar, o no hacer nada
  5. COMMIT                  -- suelta el lock y libera a la siguiente
```

Preguntar antes del paso 2 sería volver a la carrera que esto evita. La lista
que el script lee al empezar sirve para informar, no para decidir.

Es un lock **de transacción**, no de sesión: PostgreSQL lo suelta en el commit,
en el rollback y también si el proceso muere. Un lock de sesión olvidado
bloquearía los arranques siguientes hasta que alguien lo soltara a mano.

**Misma prueba, mismo escenario, aplicador nuevo:**

```
=== tres instancias simultáneas ===
  instancia 1 -> codigo de salida 0
  instancia 3 -> codigo de salida 0
  instancia 2 -> codigo de salida 0

  instancia 1: 7 aplicada(s), 1 ya aplicada(s)
  instancia 2: 0 aplicada(s), 8 ya aplicada(s)
  instancia 3: 1 aplicada(s), 7 ya aplicada(s)
```

El reparto es casual —el lock se lo da a quien llegue primero— y es justo lo que
demuestra que funciona: la 2 llegó tarde a todas y no ejecutó ni una, la 3 ganó
una carrera. Estado final de la base:

```
 registradas | distintas
-------------+-----------
           8 |         8        19 tablas en el esquema public
```

**Ocho migraciones, ocho filas, ninguna repetida.** Aplicadas una sola vez entre
las tres instancias, y las tres terminaron con éxito.

## 3. Un reinicio no duplica nada

```
==> Nada que aplicar: el esquema ya esta al dia.

 count
-------
     8
```

La tabla de control ya lo garantizaba antes de A3; el lock no la sustituye, la
protege de la concurrencia.

## 4. Dónde se ejecuta

**Decisión: arranque protegido con el lock, no paso explícito de despliegue.**

El enunciado no exige automatizar el despliegue cloud, así que no hay una
canalización donde colgar un paso previo a la rotación del grupo. Montarla solo
para esto sería construir maquinaria que nadie más usa. Con el lock, que cada
instancia aplique al arrancar es seguro y no necesita nada más.

La contrapartida está en §6.

El arranque de la instancia (**B3**, [#202](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/202))
ejecuta, antes de levantar los contenedores:

```bash
bash ./scripts/migrate.sh            # aplica lo que falte, protegido por el lock
bash ./scripts/migrate.sh --require  # y se niega a seguir si aun falta algo
```

La segunda línea parece redundante y no lo es: cubre el caso en que la primera
no llegue a ejecutarse —un arranque que la salta, un operador que despliega a
mano— y convierte un esquema desfasado en un arranque que falla en vez de en una
instancia que atiende contra una base que no corresponde.

## 5. Cuando el esquema no trae lo que la imagen necesita

`--require` es el modo nuevo. No aplica nada: responde si esta base trae lo que
esta copia del repositorio necesita.

```
    FALTA        000008_quiz_attempt_drafts

ERROR: el esquema no trae 1 migracion(es) que esta imagen necesita.
       Aplicalas antes de que la instancia empiece a atender:
         bash ./scripts/migrate.sh
```

Código de salida **1**, así que el arranque se detiene y la API no se levanta.
La instancia nunca llega a declararse lista (A2), el balanceador no le envía
tráfico y el autohealing acaba reemplazándola — ruidoso, que es lo correcto
cuando la alternativa es servir contra un esquema equivocado.

Con el esquema al día: `==> El esquema trae todo lo que esta imagen necesita.`,
código **0**.

## 6. Si una migración falla a mitad de una rotación

El DDL de PostgreSQL es transaccional y cada migración se aplica junto con su
propio registro en la misma transacción. Una que falla no deja ni esquema a
medias ni registro: la base queda en la versión anterior y la instancia no
arranca.

Lo que sí hay que tener presente, y es la contrapartida de migrar al arrancar:

> **Durante una rotación conviven la imagen vieja y la nueva.** La instancia
> nueva aplica la migración mientras las viejas siguen sirviendo contra el
> esquema ya cambiado. Por eso **toda migración tiene que ser compatible con la
> imagen anterior**: añadir una columna anulable sí, renombrar o borrar una que
> el código viejo lee, no.

Un borrado o un renombrado se hace en dos despliegues: primero dejar de usarlo,
después quitarlo.

Procedimiento si pasa:

| | |
| :--- | :--- |
| 1 | `bash ./scripts/migrate.sh --status` dice en qué versión quedó la base |
| 2 | La rotación se detiene sola: las instancias nuevas no pasan `--require` y no atienden |
| 3 | Las instancias viejas siguen sirviendo con el esquema anterior, que está intacto |
| 4 | Arreglar la migración, publicar imagen nueva y repetir |

El estado peligroso —esquema aplicado sin registrar, o registrado sin aplicar—
no puede ocurrir, porque ambos viven en la misma transacción.

## 7. Cómo reproducirlo

Necesita Docker. La base de prueba va en el puerto 5433 para no chocar con una
local:

```bash
docker run -d --rm --name a3-pg -p 5433:5432 \
  -e POSTGRES_USER=moocuser -e POSTGRES_PASSWORD=moocpassword -e POSTGRES_DB=moocdb \
  postgres:16-alpine

for i in 1 2 3; do
  ( docker run --rm --network host -v "$PWD:/repo" -w /repo \
      -e DATABASE_URL='postgres://moocuser:moocpassword@127.0.0.1:5433/moocdb?sslmode=disable' \
      postgres:16 bash scripts/migrate.sh > /tmp/instancia$i.log 2>&1
    echo "instancia $i -> $?" ) &
done; wait
```

Dos avisos de Windows, los dos reales:

- El montaje necesita la ruta **de Windows**, no la de Git Bash:
  `-v "$(cygpath -w /tmp/a3):/repo"`. Con `/tmp/...` el contenedor monta un
  directorio vacío y el script «no existe».
- La copia de trabajo puede tener CRLF aunque el repositorio guarde LF. Dentro
  del contenedor eso da `$'\r': command not found`. `sed 's/\r$//'` sobre una
  copia lo resuelve; `.gitattributes` ya fuerza LF en el repositorio.

---

Este archivo no contiene credenciales, llaves ni secretos.
