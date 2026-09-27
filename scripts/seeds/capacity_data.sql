-- ============================================================================
-- Plataforma MOOC — Semilla de Capacidad (issue #127 / G1)
-- ============================================================================
-- Cuentas adicionales para el escenario 1 de análisis de capacidad (H2/H3):
-- suficientes estudiantes distintos como para que cada usuario virtual del
-- guion de carga tenga su propia cuenta y sus propios intentos, sin
-- conflictos artificiales entre hilos concurrentes.
--
-- Deliberadamente NO están inscritos en ningún curso. El recorrido declarado
-- en H2 incluye la inscripción como uno de los pasos medidos (catálogo, curso,
-- inscripción, contenido, progreso, quiz — ver NOTAS_TECNICAS.md nota 6), así
-- que pre-inscribirlos aquí le quitaría al guion de carga el paso de escritura
-- que se supone debe medir.
--
-- Cantidad declarada: 200 estudiantes. Es un número deliberado, no arbitrario:
-- suficiente para dar a "al menos tres niveles crecientes" (H2) cuentas
-- propias sin reutilización incluso en el nivel más alto, y lo bastante
-- pequeño para no complicar la limpieza determinística ni el costo de
-- operaciones de lectura del bucket. Se puede ampliar cambiando el límite
-- superior de generate_series si un nivel de carga futuro lo requiere.
--
-- Mismos IDs deterministas y misma contraseña de prueba que
-- synthetic_data.sql: "Password123!" -> $2a$10$YA4fcEQZPadI.VaSD4eTseXzKCNA2xbNXpM8aEqp7GrfF5VTSIGNC
--
-- Bloque de UUID propio ("c9") para no colisionar con los cuatro estudiantes
-- funcionales ("c0...0001" a "...0004") que ya usa synthetic_data.sql.
-- ============================================================================

INSERT INTO users (id, email, password_hash, full_name, role, status, created_at, updated_at)
SELECT
    ('c9000000-0000-0000-0000-' || lpad(to_hex(i), 12, '0'))::uuid,
    'carga.estudiante' || lpad(i::text, 4, '0') || '@plataforma-mooc.test',
    '$2a$10$YA4fcEQZPadI.VaSD4eTseXzKCNA2xbNXpM8aEqp7GrfF5VTSIGNC',
    'Estudiante de Carga ' || lpad(i::text, 4, '0'),
    'estudiante',
    'active',
    TIMESTAMPTZ '2026-09-10 08:00:00+00' + (i || ' seconds')::interval,
    TIMESTAMPTZ '2026-09-10 08:00:00+00' + (i || ' seconds')::interval
FROM generate_series(1, 200) AS i
ON CONFLICT (id) DO UPDATE SET
    email = EXCLUDED.email,
    password_hash = EXCLUDED.password_hash,
    full_name = EXCLUDED.full_name,
    role = EXCLUDED.role,
    status = EXCLUDED.status,
    updated_at = EXCLUDED.updated_at;
