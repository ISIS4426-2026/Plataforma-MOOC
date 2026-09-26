# Evidencia A6 — Progreso y badges verificables (issue #113)

Salidas reales de la ejecución local, capturadas sobre la base sembrada con los
datos sintéticos (`bash ./scripts/seed.sh --reset`).

| Archivo | Qué demuestra |
|---|---|
| [`newman_progreso_insignias.txt`](./newman_progreso_insignias.txt) | Corrida completa de la colección Postman: 31 peticiones definidas, 39 ejecutadas, **64 aserciones, 0 fallos**. |
| [`concurrencia_e_idempotencia.txt`](./concurrencia_e_idempotencia.txt) | Criterios 1 y 2: latidos repetidos no inflan el avance, y 8 latidos concurrentes dejan un estado consistente. |
| [`insignia_verificacion_publica.txt`](./insignia_verificacion_publica.txt) | Criterio 3: el código resuelve sin autenticación. Incluye el GET condicional con ETag y las respuestas de `/badges/{badge_id}`. |

## Los tres criterios de aceptación

**1. Heartbeats repetidos del mismo recurso no inflan el porcentaje.**
El avance es un conjunto que se recalcula en cada latido, no un contador que se
incrementa. El porcentaje sale de la **intersección** de ese conjunto con los
recursos vigentes del curso, así que ni un id repetido suma dos veces ni uno
heredado de una versión anterior arrastra a nadie hacia el 100%.

**2. N heartbeats concurrentes dejan un estado consistente.**
Serializados con `FOR UPDATE` sobre la fila de la proyección, el mismo idioma que
ya usa `lockCourse`. La evidencia incluye la corrida con el candado **quitado a
propósito**: sin él se pierden 5 de 8 actualizaciones y la prueba falla. Eso es lo
que demuestra que la prueba no pasa por casualidad. Ese cambio no está en el
código entregado; el archivo muestra también la restauración.

**3. El código de verificación de un badge resuelve sin autenticación.**
`GET /api/v1/badges/verify/{verification_code}`, sin cabecera `Authorization`. La
respuesta confirma que el código corresponde a una insignia real y vigente de un
curso con nombre, y **no revela nada del estudiante** — ni nombre, ni correo, ni
identificadores. Así lo pedía `api/openapi.yaml`, que describe este endpoint como
*"public privacy-preserving badge verification ... without revealing student
personal identity or email"*.

## Sobre las credenciales

Ninguna aparece en estos archivos. Los tokens de sesión se obtuvieron antes de
capturar la salida y no se escribieron; la salida de newman no imprime cabeceras
ni cuerpos de petición.

Los `verification_code` y los ids que sí aparecen pertenecen a la semilla
sintética y se regeneran en cada `seed.sh --reset`. Un código de verificación es,
por diseño, la mitad **pública** de la credencial: existe para ser compartido con
quien quiera comprobar la insignia.

## Qué queda fuera de A6

La **imagen** de la insignia no se genera. `badges.image_key` guarda una ruta
derivada de `(estudiante, curso)` para que sea estable, pero no hay ningún objeto
en ella, y esa llave nunca sale al cliente: es un detalle de almacenamiento que el
estudiante no tiene por qué direccionar.

La nota de alcance del propio issue #113 recuerda que los badges no aparecen en
ninguna línea de la rúbrica. Se construyeron de todas formas porque el progreso
—que sí es obligatorio para el escenario 1 de capacidad— ya dejaba la emisión a un
`INSERT` de distancia.
