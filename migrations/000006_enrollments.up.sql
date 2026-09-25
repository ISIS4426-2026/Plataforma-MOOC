-- Enrollments (issue #111). Section 5.1 of the spec asks for "inscripcion,
-- retiro y reinscripcion conservando progreso y resultados", which is what
-- shapes this table.
--
-- Withdrawing is a status change, never a delete: the row carries the student's
-- history with the course, and student_progress is keyed on the same pair. If a
-- withdrawal removed the row, re-enrolling would either orphan the progress or
-- look like a first enrollment, and the spec asks for the opposite.
--
-- The key is (student_id, course_stable_id), matching student_progress and
-- badges. course_stable_id has no foreign key for the same reason those two
-- don't: courses is UNIQUE (stable_id, version), so stable_id alone is not a
-- unique target. That is deliberate -- it is the identifier meant to survive a
-- new published version, which is exactly what has to happen for progress to be
-- preserved across one.
CREATE TABLE IF NOT EXISTS enrollments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    student_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    course_stable_id UUID NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'withdrawn')),
    enrolled_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    withdrawn_at TIMESTAMPTZ,
    UNIQUE (student_id, course_stable_id)
);

-- "My courses" reads by student; a roster and the access check read by course.
CREATE INDEX IF NOT EXISTS idx_enrollments_student ON enrollments(student_id);
CREATE INDEX IF NOT EXISTS idx_enrollments_course ON enrollments(course_stable_id);

-- The access check asks "is this student actively enrolled in this course" on
-- every read of course content, so it gets its own partial index rather than
-- filtering the pair index by status.
CREATE INDEX IF NOT EXISTS idx_enrollments_active
    ON enrollments(student_id, course_stable_id)
    WHERE status = 'active';
