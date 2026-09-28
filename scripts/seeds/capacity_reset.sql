-- ============================================================================
-- Plataforma MOOC — Reinicio de las cuentas de carga (issue #132 / H2)
-- ============================================================================
-- Deja a los 200 estudiantes de carga (capacity_data.sql) como recien
-- sembrados: sin inscripcion, sin progreso y sin intentos de quiz. Se corre
-- ENTRE corridas del escenario 1, porque el recorrido gasta el estado de cada
-- cuenta: la inscripcion queda activa y el quiz solo admite max_attempts
-- intentos por estudiante.
--
-- Lo que NO toca:
--   * user_sessions: las sesiones (24 h) siguen validas, asi que el archivo de
--     tokens que dejo el paso de login sigue sirviendo y no hay que volver a
--     iniciar sesion (el login esta limitado a 10 por minuto por IP).
--   * audit_logs: es de solo anadir por diseno (disparador); la auditoria de
--     corridas anteriores permanece, que es lo correcto.
--   * Los 4 estudiantes funcionales ni ningun dato de la semilla funcional.
-- ============================================================================

BEGIN;

DELETE FROM quiz_submissions
 WHERE student_id IN (SELECT id FROM users WHERE email LIKE 'carga.estudiante%@plataforma-mooc.test');

DELETE FROM badges
 WHERE student_id IN (SELECT id FROM users WHERE email LIKE 'carga.estudiante%@plataforma-mooc.test');

DELETE FROM progress_events
 WHERE student_id IN (SELECT id FROM users WHERE email LIKE 'carga.estudiante%@plataforma-mooc.test');

DELETE FROM student_progress
 WHERE student_id IN (SELECT id FROM users WHERE email LIKE 'carga.estudiante%@plataforma-mooc.test');

DELETE FROM enrollments
 WHERE student_id IN (SELECT id FROM users WHERE email LIKE 'carga.estudiante%@plataforma-mooc.test');

COMMIT;
