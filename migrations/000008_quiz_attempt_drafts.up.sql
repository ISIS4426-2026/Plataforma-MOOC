-- Intentos congelados, guardado parcial y expiracion de quizzes.
--
-- La revision de la Entrega 1 lo marco en el criterio de evaluacion academica:
-- «no hay ningun worker o instancia que permita un guardado temporal, no es
-- posible hacer snapshots, el envio no congela el cuestionario, asi que editar
-- el quiz altera intentos pasados».
--
-- Lo ultimo es lo grave. Hasta ahora un envio se calificaba leyendo las tablas
-- vivas, asi que si el autor cambiaba una opcion despues de que un estudiante
-- presentara, la nota registrada dejaba de corresponder a lo que ese estudiante
-- habia visto. La calificacion no era reproducible.

-- El limite de tiempo del intento, en segundos.
--
-- Nulo significa sin limite, que es como se comportaban todos los quizzes hasta
-- ahora: hacerlo obligatorio cambiaria el significado de los que ya existen.
ALTER TABLE quizzes
    ADD COLUMN IF NOT EXISTS time_limit_seconds INT
        CHECK (time_limit_seconds IS NULL OR time_limit_seconds > 0);

-- El cuestionario tal y como estaba cuando se califico este envio.
--
-- Nulo en las filas anteriores a esta migracion: de aquellas no hay forma de
-- saber contra que se calificaron, y rellenarlo con el quiz actual seria
-- inventar un dato. Un nulo dice la verdad, que es que no se conserva.
ALTER TABLE quiz_submissions
    ADD COLUMN IF NOT EXISTS quiz_snapshot JSONB;

-- El intento en curso: lo que el estudiante ve, lo que lleva respondido y
-- cuando se le acaba el tiempo.
--
-- La clave de respuestas viaja dentro de snapshot y no sale de la base: la
-- calificacion contra el snapshot se hace en SQL, igual que se hacia contra las
-- tablas vivas. Esa es la razon de guardar el quiz entero y no solo los
-- enunciados.
CREATE TABLE IF NOT EXISTS quiz_attempt_drafts (
    quiz_id    UUID NOT NULL REFERENCES quizzes(id) ON DELETE CASCADE,
    student_id UUID NOT NULL REFERENCES users(id)   ON DELETE CASCADE,

    -- Que numero de intento ocupara este borrador cuando se envie. Se fija al
    -- iniciar para que la expiracion y el snapshot pertenezcan a un intento
    -- concreto y no al «siguiente, sea cual sea».
    attempt INT NOT NULL CHECK (attempt >= 1),

    snapshot JSONB NOT NULL,

    -- Lo respondido hasta ahora, con la misma forma que quiz_submissions.answers.
    -- Vacio al iniciar: un intento recien abierto no ha respondido nada.
    answers JSONB NOT NULL DEFAULT '[]'::jsonb,

    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Nulo cuando el quiz no tiene limite de tiempo.
    expires_at TIMESTAMPTZ,

    -- Un borrador por estudiante y quiz. Iniciar dos veces no abre dos
    -- intentos: devuelve el que ya estaba, que es lo que permite recuperar el
    -- trabajo despues de cerrar el navegador.
    PRIMARY KEY (quiz_id, student_id)
);

-- La consulta de limpieza y la de «cuales van a caducar» van por aqui.
CREATE INDEX IF NOT EXISTS idx_quiz_attempt_drafts_expires
    ON quiz_attempt_drafts(expires_at)
    WHERE expires_at IS NOT NULL;
