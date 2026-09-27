-- Intentos de quiz (issue #112).
--
-- El esquema de la Entrega 1 dejo las cuatro tablas de quizzes pero sin nada
-- que expresara los intentos, y el enunciado los pide explicitamente: «quizzes
-- de seleccion multiple con intentos ... conforme a la politica configurada».
-- Esta migracion agrega esa politica y la garantia que la hace cumplible bajo
-- concurrencia.

-- Cuantas veces puede presentarse un quiz. Por quiz y no global, porque es una
-- decision del autor, igual que passing_score.
--
-- Tres por defecto: suficiente para que un error de lectura no cueste el curso,
-- y poco para que el quiz siga evaluando.
ALTER TABLE quizzes
    ADD COLUMN IF NOT EXISTS max_attempts INT NOT NULL DEFAULT 3
        CHECK (max_attempts >= 1);

-- El numero de intento que representa cada envio.
--
-- Se guarda en lugar de deducirse al leer porque es lo que permite la
-- restriccion de abajo: sin una columna, el limite de intentos solo podria
-- comprobarse contando filas, y dos envios simultaneos contarian lo mismo antes
-- de que ninguno hubiera insertado.
ALTER TABLE quiz_submissions
    ADD COLUMN IF NOT EXISTS attempt INT NOT NULL DEFAULT 1
        CHECK (attempt >= 1);

-- La garantia de que dos envios concurrentes no se convierten en dos intentos
-- con el mismo numero.
--
-- El servicio calcula el intento como «los que ya hay, mas uno» e inserta. Si
-- dos peticiones lo hacen a la vez, ambas calculan el mismo numero y la base
-- rechaza la segunda. Esa es la diferencia entre comprobar el limite y
-- garantizarlo: una comprobacion en la aplicacion siempre tiene una ventana
-- entre el conteo y la insercion, y esta restriccion la cierra.
--
-- No es DEFERRABLE, a diferencia de las de ordenamiento academico: aqui no hay
-- ninguna reescritura masiva que necesite violarla a mitad de transaccion.
ALTER TABLE quiz_submissions
    ADD CONSTRAINT uq_quiz_submissions_attempt UNIQUE (quiz_id, student_id, attempt);

-- Una pregunta por posicion dentro de un quiz, con la misma forma diferible que
-- el resto de la jerarquia academica: reordenar reescribe todas las posiciones
-- en una pasada y momentaneamente dos filas comparten valor.
ALTER TABLE quiz_questions
    ADD CONSTRAINT uq_quiz_questions_quiz_position UNIQUE (quiz_id, position)
        DEFERRABLE INITIALLY DEFERRED;

-- Un quiz por recurso. La tabla ya llevaba resource_id con clave ajena, pero
-- nada impedia colgar dos quizzes del mismo recurso, y entonces
-- GetByResourceID no tendria una respuesta sino dos.
ALTER TABLE quizzes
    ADD CONSTRAINT uq_quizzes_resource UNIQUE (resource_id);

-- El orden de las opciones dentro de una pregunta.
--
-- Sin esta columna no habia ninguno. quiz_options solo tenia created_at, y
-- PostgreSQL resuelve NOW() como la hora de inicio de la transaccion, asi que
-- todas las opciones insertadas juntas comparten el mismo instante y el
-- desempate cae en un UUID aleatorio. El resultado es que las opciones salian
-- en orden distinto en cada lectura y el autor no podia decidirlo.
ALTER TABLE quiz_options
    ADD COLUMN IF NOT EXISTS position INT NOT NULL DEFAULT 0
        CHECK (position >= 0);

-- Diferible, igual que el resto del ordenamiento academico: reordenar reescribe
-- todas las posiciones en una pasada y a mitad de ella dos filas coinciden.
ALTER TABLE quiz_options
    ADD CONSTRAINT uq_quiz_options_question_position UNIQUE (question_id, position)
        DEFERRABLE INITIALLY DEFERRED;

-- Los envios se leen casi siempre por (estudiante, quiz) y ordenados por
-- intento, que es como se cuenta cuantos quedan y cual fue el ultimo.
CREATE INDEX IF NOT EXISTS idx_quiz_submissions_attempt
    ON quiz_submissions(student_id, quiz_id, attempt DESC);
