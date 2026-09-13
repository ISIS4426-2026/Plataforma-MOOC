-- Script de Limpieza Determinística de Datos — Plataforma MOOC
--
-- Trunca todas las tablas transaccionales de la base de datos PostgreSQL
-- respetando integridad referencial (CASCADE).
-- La tabla audit_logs es append-only ante DELETE/UPDATE (disparador),
-- pero TRUNCATE permite el vaciado controlado en entornos de desarrollo/pruebas.

TRUNCATE TABLE
    quiz_submissions,
    quiz_options,
    quiz_questions,
    quizzes,
    resources,
    units,
    modules,
    courses,
    student_progress,
    progress_events,
    badges,
    user_sessions,
    email_verification_tokens,
    password_reset_tokens,
    audit_logs,
    users
CASCADE;
