# Evidencia A5 — Quizzes y calificación con envío idempotente (issue #112)

Salidas reales de la ejecución local, sobre la base sembrada con los datos
sintéticos.

| Archivo | Qué demuestra |
| :--- | :--- |
| [`newman_quizzes.txt`](./newman_quizzes.txt) | Corrida completa de la colección Postman: 28 peticiones, **56 aserciones, 0 fallos**. |
| [`garantias_repositorio.txt`](./garantias_repositorio.txt) | Las ocho pruebas de integración, y la corrida con la restricción de unicidad quitada a propósito. |

## Los tres criterios de aceptación

**1. Dos envíos concurrentes con la misma `Idempotency-Key` producen una sola
calificación.**

Se sostiene en dos capas, y ninguna basta sola:

* El **middleware de idempotencia** reserva la clave de forma atómica. El
  segundo envío no llega al handler: recibe la calificación ya registrada, o un
  409 si el primero sigue en curso. Por eso un reintento no consume un intento.
* Debajo, la **restricción `UNIQUE (quiz_id, student_id, attempt)`** cubre el
  caso que la clave no cubre: dos peticiones distintas que llegan a la vez. El
  servicio calcula el intento como «los que ya hay, más uno», y sin esa
  restricción dos peticiones simultáneas calcularían el mismo número. Contar y
  después insertar siempre deja una ventana entre ambas cosas.

La colección comprueba el reenvío con la misma clave. La concurrencia real no se
puede reproducir con newman, que es secuencial, así que la cubre
`TestQuizRepositoryConcurrentSubmissionsRespectTheLimit` con ocho envíos a la
vez.

**2. Superar el límite de intentos devuelve 409, no un intento extra.**

Tras los tres intentos permitidos, el cuarto responde `409 quiz_conflict`, y la
lista de intentos sigue teniendo exactamente tres: el rechazo no deja rastro.

**3. Las opciones correctas no aparecen en ninguna respuesta al estudiante.**

La protección no es una etiqueta de serialización, es una consulta distinta: la
lectura del estudiante **no selecciona `is_correct`**. No oculta el dato, no lo
carga. Un error en la capa HTTP no puede filtrar lo que nunca llegó a memoria.

Además, la vista del estudiante y la del autor son **tipos distintos** en el
handler, no el mismo con un campo opcional, de modo que la del estudiante no
tiene dónde alojar la clave.

La colección lo verifica sobre el **texto crudo** de la respuesta, que es la
única forma de afirmar que la cadena no está en ninguna parte:

```javascript
pm.expect(pm.response.text()).to.not.include("is_correct");
```

## Que las pruebas tienen dientes

`garantias_repositorio.txt` incluye una corrida con la restricción de unicidad
**quitada a propósito**. Sin ella, seis envíos simultáneos tienen éxito frente a
un límite de tres, y los números de intento se repiten:

```
6 envíos tuvieron éxito con un límite de 3 intentos
quedaron 6 envíos registrados con un límite de 3
el intento 2 aparece dos veces
```

Ese cambio no está en el código entregado; el archivo muestra también la
restauración y que la prueba vuelve a pasar.

## Decisiones que conviene conocer

**Aprobar un cuestionario completa su recurso**, y lo registra el servidor al
calificar en lugar de esperar un latido del cliente. Es lo coherente con que el
enunciado rechace los avances enviados por el cliente: aquí el servidor sabe de
primera mano que el estudiante terminó el recurso, porque acaba de calificarlo.

Si ese registro fallara, **la calificación se conserva igual**. Perder el avance
se arregla con un latido; perder un intento por un fallo posterior a la
calificación no se arregla.

**El autor no se evalúa a sí mismo.** Puede leer su cuestionario con la clave,
pero presentar exige inscripción activa, y darle una calificación lo pondría en
las estadísticas de su propio curso.

**Un quiz sin preguntas no se aprueba.** Calificar con 100 lo convertiría en un
aprobado automático, que es lo contrario de evaluar.

## Sobre la colección

Monta **su propio curso y cuestionario** en cada corrida. No es un capricho: los
intentos se agotan, así que reutilizar el quiz sembrado haría que la segunda
ejecución no pudiera presentar nada. Con un curso nuevo los intentos siempre
empiezan en cero, y la colección es re-ejecutable sin reiniciar la base —
comprobado con tres corridas seguidas.

## Nunca

Credenciales, llaves ni secretos. Estos archivos son salidas de pruebas locales
y no contienen ninguno.
