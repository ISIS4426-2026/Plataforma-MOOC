-- ============================================================================
-- Plataforma MOOC — Seed de Datos Sintéticos Determinísticos
-- ============================================================================
-- Este script inserta datos fijos y reproducibles para los tres roles
-- del sistema (administrador, profesor, estudiante) y cursos en distintos
-- estados (published, draft, unpublished).
--
-- Todos los UUIDs son determinísticos y estrictamente hexadecimales [0-9a-f].
-- Contraseña unificada para todos los usuarios de prueba: "Password123!"
-- Hash bcrypt correspondiente: $2a$10$YA4fcEQZPadI.VaSD4eTseXzKCNA2xbNXpM8aEqp7GrfF5VTSIGNC
-- ============================================================================

-- ----------------------------------------------------------------------------
-- 1. USUARIOS (Roles: administrador, profesor, estudiante)
-- ----------------------------------------------------------------------------
INSERT INTO users (id, email, password_hash, full_name, role, status, created_at, updated_at)
VALUES
    -- Administradores
    ('a0000000-0000-0000-0000-000000000001', 'admin@plataforma-mooc.test',
     '$2a$10$YA4fcEQZPadI.VaSD4eTseXzKCNA2xbNXpM8aEqp7GrfF5VTSIGNC',
     'Administrador Principal', 'administrador', 'active',
     '2026-09-01 08:00:00+00', '2026-09-01 08:00:00+00'),

    ('a0000000-0000-0000-0000-000000000002', 'admin.secundario@plataforma-mooc.test',
     '$2a$10$YA4fcEQZPadI.VaSD4eTseXzKCNA2xbNXpM8aEqp7GrfF5VTSIGNC',
     'Administrador Secundario', 'administrador', 'active',
     '2026-09-01 08:05:00+00', '2026-09-01 08:05:00+00'),

    -- Profesores (Creados exclusivamente por administración)
    ('b0000000-0000-0000-0000-000000000001', 'profesor1@plataforma-mooc.test',
     '$2a$10$YA4fcEQZPadI.VaSD4eTseXzKCNA2xbNXpM8aEqp7GrfF5VTSIGNC',
     'Prof. Carlos Mendoza', 'profesor', 'active',
     '2026-09-01 09:00:00+00', '2026-09-01 09:00:00+00'),

    ('b0000000-0000-0000-0000-000000000002', 'profesor2@plataforma-mooc.test',
     '$2a$10$YA4fcEQZPadI.VaSD4eTseXzKCNA2xbNXpM8aEqp7GrfF5VTSIGNC',
     'Prof. Laura Gómez', 'profesor', 'active',
     '2026-09-01 09:30:00+00', '2026-09-01 09:30:00+00'),

    -- Estudiantes (Activo, Activo con avance, Pendiente de verificación, Suspendido)
    ('c0000000-0000-0000-0000-000000000001', 'estudiante1@plataforma-mooc.test',
     '$2a$10$YA4fcEQZPadI.VaSD4eTseXzKCNA2xbNXpM8aEqp7GrfF5VTSIGNC',
     'Estudiante Juan Pérez', 'estudiante', 'active',
     '2026-09-02 10:00:00+00', '2026-09-02 10:00:00+00'),

    ('c0000000-0000-0000-0000-000000000002', 'estudiante2@plataforma-mooc.test',
     '$2a$10$YA4fcEQZPadI.VaSD4eTseXzKCNA2xbNXpM8aEqp7GrfF5VTSIGNC',
     'Estudiante María Rodríguez', 'estudiante', 'active',
     '2026-09-02 10:15:00+00', '2026-09-02 10:15:00+00'),

    ('c0000000-0000-0000-0000-000000000003', 'estudiante.pendiente@plataforma-mooc.test',
     '$2a$10$YA4fcEQZPadI.VaSD4eTseXzKCNA2xbNXpM8aEqp7GrfF5VTSIGNC',
     'Estudiante Sin Verificar', 'estudiante', 'pending_verification',
     '2026-09-02 11:00:00+00', '2026-09-02 11:00:00+00'),

    ('c0000000-0000-0000-0000-000000000004', 'estudiante.suspendido@plataforma-mooc.test',
     '$2a$10$YA4fcEQZPadI.VaSD4eTseXzKCNA2xbNXpM8aEqp7GrfF5VTSIGNC',
     'Estudiante Cuenta Suspendida', 'estudiante', 'suspended',
     '2026-09-02 11:30:00+00', '2026-09-02 11:30:00+00')
ON CONFLICT (id) DO UPDATE SET
    email = EXCLUDED.email,
    password_hash = EXCLUDED.password_hash,
    full_name = EXCLUDED.full_name,
    role = EXCLUDED.role,
    status = EXCLUDED.status,
    updated_at = EXCLUDED.updated_at;

-- ----------------------------------------------------------------------------
-- 2. CURSOS (Estados: published, draft completo, draft incompleto, unpublished)
-- ----------------------------------------------------------------------------
INSERT INTO courses (id, stable_id, title, description, version, status, author_id, created_at, updated_at)
VALUES
    -- Curso 1: Publicado e inmutable (v1)
    ('d0000000-0000-0000-0000-000000000001', 'e0000000-0000-0000-0000-000000000001',
     'Arquitectura Cloud y Sistemas Distribuidos',
     'Fundamentos de diseño de software en la nube, monolitos modulares, colas de trabajo y observabilidad.',
     1, 'published', 'b0000000-0000-0000-0000-000000000001',
     '2026-09-03 08:00:00+00', '2026-09-03 09:00:00+00'),

    -- Curso 2: Borrador Completo (Listo para publicar en pruebas interactivas)
    ('d0000000-0000-0000-0000-000000000002', 'e0000000-0000-0000-0000-000000000002',
     'Desarrollo Backend Concurrente con Go',
     'Aprende concurrencia avanzada en Go: goroutines, canales, patrones worker pool y context.',
     1, 'draft', 'b0000000-0000-0000-0000-000000000001',
     '2026-09-04 10:00:00+00', '2026-09-04 10:00:00+00'),

    -- Curso 3: Borrador Incompleto (Para probar lista exhaustiva de errores de validación)
    ('d0000000-0000-0000-0000-000000000003', 'e0000000-0000-0000-0000-000000000003',
     'Seguridad en la Nube y Criptografía Aplicada',
     'Curso preliminar sin módulos ni unidades asignadas.',
     1, 'draft', 'b0000000-0000-0000-0000-000000000002',
     '2026-09-04 11:00:00+00', '2026-09-04 11:00:00+00'),

    -- Curso 4: Despublicado (Para validar flujo de re-edición y unpublish del MVP)
    ('d0000000-0000-0000-0000-000000000004', 'e0000000-0000-0000-0000-000000000004',
     'Introducción a DevSecOps y Contenedores Docker',
     'Curso temporalmente despublicado por el profesor para revisión de contenidos.',
     1, 'unpublished', 'b0000000-0000-0000-0000-000000000002',
     '2026-09-03 14:00:00+00', '2026-09-04 12:00:00+00')
ON CONFLICT (id) DO UPDATE SET
    title = EXCLUDED.title,
    description = EXCLUDED.description,
    status = EXCLUDED.status,
    author_id = EXCLUDED.author_id,
    updated_at = EXCLUDED.updated_at;

-- ----------------------------------------------------------------------------
-- 3. MÓDULOS (Jerarquía Nivel 2)
-- ----------------------------------------------------------------------------
INSERT INTO modules (id, stable_id, course_id, title, position, created_at)
VALUES
    -- Módulos de Curso 1 (Publicado)
    ('d1000000-0000-0000-0000-000000000001', 'e1000000-0000-0000-0000-000000000001',
     'd0000000-0000-0000-0000-000000000001',
     'Módulo 1: Fundamentos de Arquitectura en la Nube', 0, '2026-09-03 08:10:00+00'),

    ('d1000000-0000-0000-0000-000000000002', 'e1000000-0000-0000-0000-000000000002',
     'd0000000-0000-0000-0000-000000000001',
     'Módulo 2: Evaluación y Buenas Prácticas', 1, '2026-09-03 08:20:00+00'),

    -- Módulo de Curso 2 (Borrador Completo)
    ('d1000000-0000-0000-0000-000000000003', 'e1000000-0000-0000-0000-000000000003',
     'd0000000-0000-0000-0000-000000000002',
     'Módulo 1: Goroutines y Canales', 0, '2026-09-04 10:10:00+00'),

    -- Módulo de Curso 4 (Despublicado)
    ('d1000000-0000-0000-0000-000000000004', 'e1000000-0000-0000-0000-000000000004',
     'd0000000-0000-0000-0000-000000000004',
     'Módulo 1: Fundamentos de Contenedores', 0, '2026-09-03 14:10:00+00')
ON CONFLICT (id) DO UPDATE SET
    title = EXCLUDED.title,
    position = EXCLUDED.position;

-- ----------------------------------------------------------------------------
-- 4. UNIDADES (Jerarquía Nivel 3)
-- ----------------------------------------------------------------------------
INSERT INTO units (id, stable_id, module_id, title, position, created_at)
VALUES
    -- Unidades de Curso 1
    ('d2000000-0000-0000-0000-000000000001', 'e2000000-0000-0000-0000-000000000001',
     'd1000000-0000-0000-0000-000000000001',
     'Unidad 1.1: Patrones de Desacoplamiento y Statelessness', 0, '2026-09-03 08:15:00+00'),

    ('d2000000-0000-0000-0000-000000000002', 'e2000000-0000-0000-0000-000000000002',
     'd1000000-0000-0000-0000-000000000002',
     'Unidad 2.1: Cuestionario Diagnóstico de Arquitectura', 0, '2026-09-03 08:25:00+00'),

    -- Unidad de Curso 2
    ('d2000000-0000-0000-0000-000000000003', 'e2000000-0000-0000-0000-000000000003',
     'd1000000-0000-0000-0000-000000000003',
     'Unidad 1.1: Sincronización Básica con Canales', 0, '2026-09-04 10:15:00+00'),

    -- Unidad de Curso 4
    ('d2000000-0000-0000-0000-000000000004', 'e2000000-0000-0000-0000-000000000004',
     'd1000000-0000-0000-0000-000000000004',
     'Unidad 1.1: Dockerfile y Capas de Imagen', 0, '2026-09-03 14:15:00+00')
ON CONFLICT (id) DO UPDATE SET
    title = EXCLUDED.title,
    position = EXCLUDED.position;

-- ----------------------------------------------------------------------------
-- 5. RECURSOS (Jerarquía Nivel 4 — Tipos: text, video, quiz, pdf)
-- ----------------------------------------------------------------------------
INSERT INTO resources (id, stable_id, unit_id, title, type, position, is_visible, is_mandatory, allow_download, content_text, object_key, processing_status, created_at)
VALUES
    -- Recursos de Curso 1 (Publicado)
    ('d3000000-0000-0000-0000-000000000001', 'e3000000-0000-0000-0000-000000000001',
     'd2000000-0000-0000-0000-000000000001',
     'Guía Canónica de Monolito Modular', 'text', 0, true, true, true,
     '# Arquitectura Monolito Modular\n\nEl sistema desacopla el dominio de HTTP e infraestructura siguiendo arquitectura limpia...',
     NULL, 'completed', '2026-09-03 08:30:00+00'),

    ('d3000000-0000-0000-0000-000000000002', 'e3000000-0000-0000-0000-000000000002',
     'd2000000-0000-0000-0000-000000000001',
     'Video Streaming: Workers y Asynq', 'video', 1, true, true, false,
     NULL, 'courses/c1/video_workers_hls.m3u8', 'completed', '2026-09-03 08:35:00+00'),

    ('d3000000-0000-0000-0000-000000000003', 'e3000000-0000-0000-0000-000000000003',
     'd2000000-0000-0000-0000-000000000002',
     'Quiz: Evaluación Diagnóstica de Arquitectura', 'quiz', 0, true, true, false,
     NULL, NULL, 'completed', '2026-09-03 08:40:00+00'),

    -- Recurso de Curso 2 (Borrador Completo)
    ('d3000000-0000-0000-0000-000000000004', 'e3000000-0000-0000-0000-000000000004',
     'd2000000-0000-0000-0000-000000000003',
     'Introducción a Canales en Go', 'text', 0, true, true, true,
     '# Canales en Go\n\nLos canales proveen sincronización y comunicación segura entre goroutines sin memoria compartida.',
     NULL, 'completed', '2026-09-04 10:20:00+00'),

    -- Recurso de Curso 4 (Despublicado)
    ('d3000000-0000-0000-0000-000000000005', 'e3000000-0000-0000-0000-000000000005',
     'd2000000-0000-0000-0000-000000000004',
     'Manual de Seguridad en Docker', 'pdf', 0, true, true, true,
     NULL, 'courses/c4/manual_seguridad_docker.pdf', 'completed', '2026-09-03 14:20:00+00')
ON CONFLICT (id) DO UPDATE SET
    title = EXCLUDED.title,
    position = EXCLUDED.position,
    is_visible = EXCLUDED.is_visible,
    is_mandatory = EXCLUDED.is_mandatory,
    allow_download = EXCLUDED.allow_download,
    content_text = EXCLUDED.content_text,
    object_key = EXCLUDED.object_key,
    processing_status = EXCLUDED.processing_status;

-- ----------------------------------------------------------------------------
-- 6. QUIZZES, PREGUNTAS Y OPCIONES
-- ----------------------------------------------------------------------------
INSERT INTO quizzes (id, resource_id, title, passing_score, created_at, updated_at)
VALUES
    ('fa000000-0000-0000-0000-000000000001', 'd3000000-0000-0000-0000-000000000003',
     'Evaluación de Conceptos de Arquitectura Cloud', 70,
     '2026-09-03 08:42:00+00', '2026-09-03 08:42:00+00')
ON CONFLICT (id) DO UPDATE SET
    title = EXCLUDED.title,
    passing_score = EXCLUDED.passing_score;

INSERT INTO quiz_questions (id, quiz_id, prompt, position, created_at)
VALUES
    ('fb000000-0000-0000-0000-000000000001', 'fa000000-0000-0000-0000-000000000001',
     '¿Dónde deben almacenarse los binarios multimedia según la Sección 7 del pliego?',
     1, '2026-09-03 08:43:00+00'),

    ('fb000000-0000-0000-0000-000000000002', 'fa000000-0000-0000-0000-000000000001',
     '¿Cómo se valida y computa el progreso de los estudiantes en la plataforma?',
     2, '2026-09-03 08:44:00+00')
ON CONFLICT (id) DO UPDATE SET
    prompt = EXCLUDED.prompt,
    position = EXCLUDED.position;

INSERT INTO quiz_options (id, question_id, text, is_correct, created_at)
VALUES
    -- Opciones Pregunta 1
    ('fc000000-0000-0000-0000-000000000001', 'fb000000-0000-0000-0000-000000000001',
     'Exclusivamente en almacenamiento de objetos (S3/MinIO) vía URLs prefirmadas', true, '2026-09-03 08:45:00+00'),

    ('fc000000-0000-0000-0000-000000000002', 'fb000000-0000-0000-0000-000000000001',
     'En la base de datos relacional PostgreSQL como campos BYTEA', false, '2026-09-03 08:45:00+00'),

    ('fc000000-0000-0000-0000-000000000003', 'fb000000-0000-0000-0000-000000000001',
     'En el sistema de archivos temporal de los workers', false, '2026-09-03 08:45:00+00'),

    -- Opciones Pregunta 2
    ('fc000000-0000-0000-0000-000000000004', 'fb000000-0000-0000-0000-000000000002',
     'Estrictamente en servidor mediante heartbeats verificados, permanencia y eventos', true, '2026-09-03 08:46:00+00'),

    ('fc000000-0000-0000-0000-000000000005', 'fb000000-0000-0000-0000-000000000002',
     'El cliente envía directamente su porcentaje completado vía HTTP', false, '2026-09-03 08:46:00+00')
ON CONFLICT (id) DO UPDATE SET
    text = EXCLUDED.text,
    is_correct = EXCLUDED.is_correct;

-- ----------------------------------------------------------------------------
-- 7. INSCRIPCIONES (Issue #111)
-- ----------------------------------------------------------------------------
-- Van antes del progreso porque lo sostienen: desde que el contenido del curso
-- exige inscripcion activa, un estudiante con avance y sin inscripcion es un
-- estado que se contradice -- no podria leer el contenido que supuestamente
-- completo, ni reportar un latido mas.
--
-- Son exactamente dos, una por cada estudiante con progreso sembrado. Los otros
-- dos estudiantes (pendiente de verificacion y suspendido) no tienen avance, y
-- darles una inscripcion seria inventar datos en lugar de respaldarlos.
--
-- El conflicto se resuelve sobre (student_id, course_stable_id) y no sobre el id:
-- es la llave que de verdad colisiona cuando la semilla se carga sobre una base
-- donde alguien ya se inscribio por la API con otro id.
INSERT INTO enrollments (id, student_id, course_stable_id, status, enrolled_at, withdrawn_at)
VALUES
    -- Estudiante 1: inscrito y avanzando
    ('fc100000-0000-0000-0000-000000000001', 'c0000000-0000-0000-0000-000000000001',
     'e0000000-0000-0000-0000-000000000001', 'active', '2026-09-04 09:00:00+00', NULL),

    -- Estudiante 2: inscrito, y por eso pudo completar el curso y recibir la insignia
    ('fc100000-0000-0000-0000-000000000002', 'c0000000-0000-0000-0000-000000000002',
     'e0000000-0000-0000-0000-000000000001', 'active', '2026-09-04 09:30:00+00', NULL)
ON CONFLICT (student_id, course_stable_id) DO UPDATE SET
    status = 'active',
    enrolled_at = EXCLUDED.enrolled_at,
    withdrawn_at = NULL;

-- ----------------------------------------------------------------------------
-- 8. PROGRESO DE ESTUDIANTES (Progreso Validado e Insignias)
-- ----------------------------------------------------------------------------
-- Los latidos son el registro de hechos y student_progress la proyeccion que se
-- deriva de ellos, asi que se siembran los dos: una proyeccion sin los
-- latidos que la produjeron contradice el diseno que la API implementa.
INSERT INTO progress_events (id, student_id, course_stable_id, resource_stable_id, dwell_time_seconds, completed, created_at)
VALUES
    -- Estudiante 1: leyo la guia y la termino
    ('fc200000-0000-0000-0000-000000000001', 'c0000000-0000-0000-0000-000000000001',
     'e0000000-0000-0000-0000-000000000001', 'e3000000-0000-0000-0000-000000000001',
     180, false, '2026-09-05 14:55:00+00'),
    ('fc200000-0000-0000-0000-000000000002', 'c0000000-0000-0000-0000-000000000001',
     'e0000000-0000-0000-0000-000000000001', 'e3000000-0000-0000-0000-000000000001',
     120, true, '2026-09-05 15:00:00+00'),

    -- Estudiante 2: recorrio los tres recursos obligatorios
    ('fc200000-0000-0000-0000-000000000003', 'c0000000-0000-0000-0000-000000000002',
     'e0000000-0000-0000-0000-000000000001', 'e3000000-0000-0000-0000-000000000001',
     240, true, '2026-09-05 15:30:00+00'),
    ('fc200000-0000-0000-0000-000000000004', 'c0000000-0000-0000-0000-000000000002',
     'e0000000-0000-0000-0000-000000000001', 'e3000000-0000-0000-0000-000000000002',
     600, true, '2026-09-05 15:45:00+00'),
    ('fc200000-0000-0000-0000-000000000005', 'c0000000-0000-0000-0000-000000000002',
     'e0000000-0000-0000-0000-000000000001', 'e3000000-0000-0000-0000-000000000003',
     300, true, '2026-09-05 16:00:00+00')
ON CONFLICT (id) DO NOTHING;

INSERT INTO student_progress (id, student_id, course_stable_id, completed_resources, percent_completed, is_approved, updated_at)
VALUES
    -- Estudiante 1: 1 de los 3 recursos obligatorios del curso.
    --
    -- 33.33 y no 50.00: la API recalcula el porcentaje en cada lectura contra los
    -- recursos obligatorios y visibles que el curso tiene, y el curso sembrado
    -- tiene tres. Un numero guardado que no coincide con el que la API reporta no
    -- se sostiene como evidencia.
    ('fd000000-0000-0000-0000-000000000001', 'c0000000-0000-0000-0000-000000000001',
     'e0000000-0000-0000-0000-000000000001',
     '["e3000000-0000-0000-0000-000000000001"]'::jsonb,
     33.33, false, '2026-09-05 15:00:00+00'),

    -- Estudiante 2: 100% completado y aprobado
    ('fd000000-0000-0000-0000-000000000002', 'c0000000-0000-0000-0000-000000000002',
     'e0000000-0000-0000-0000-000000000001',
     '["e3000000-0000-0000-0000-000000000001", "e3000000-0000-0000-0000-000000000002", "e3000000-0000-0000-0000-000000000003"]'::jsonb,
     100.00, true, '2026-09-05 16:00:00+00')
ON CONFLICT (id) DO UPDATE SET
    completed_resources = EXCLUDED.completed_resources,
    percent_completed = EXCLUDED.percent_completed,
    is_approved = EXCLUDED.is_approved,
    updated_at = EXCLUDED.updated_at;

-- Insignia digital para Estudiante 2 (Aprobado)
INSERT INTO badges (id, student_id, course_stable_id, verification_code, image_key, is_revoked, issued_at)
VALUES
    ('fe000000-0000-0000-0000-000000000001', 'c0000000-0000-0000-0000-000000000002',
     'e0000000-0000-0000-0000-000000000001',
     'fe100000-0000-0000-0000-000000000001',
     -- La llave se deriva de (curso, estudiante), como hace domain.BadgeImageKey.
     -- Escribirla a mano con otra forma es justamente la deriva que esa funcion
     -- existe para evitar.
     'badges/e0000000-0000-0000-0000-000000000001/c0000000-0000-0000-0000-000000000002.png',
     false, '2026-09-05 16:05:00+00')
ON CONFLICT (id) DO UPDATE SET
    verification_code = EXCLUDED.verification_code,
    image_key = EXCLUDED.image_key,
    is_revoked = EXCLUDED.is_revoked;

-- ----------------------------------------------------------------------------
-- 9. REGISTROS DE AUDITORÍA INMUTABLE INICIALES
-- ----------------------------------------------------------------------------
INSERT INTO audit_logs (id, actor_id, action, target_resource, details, ip_address, user_agent, created_at)
VALUES
    ('ff000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001',
     'system.seed_initialized', 'system:database',
     '{"description": "Initial synthetic dataset loaded deterministically"}'::jsonb,
     '127.0.0.1', 'synthetic-data-seed/1.0', '2026-09-01 08:00:00+00'),

    ('ff000000-0000-0000-0000-000000000002', 'b0000000-0000-0000-0000-000000000001',
     'course.published', 'course:d0000000-0000-0000-0000-000000000001',
     '{"version": 1, "title": "Arquitectura Cloud y Sistemas Distribuidos"}'::jsonb,
     '127.0.0.1', 'synthetic-data-seed/1.0', '2026-09-03 09:00:00+00')
ON CONFLICT (id) DO NOTHING;
