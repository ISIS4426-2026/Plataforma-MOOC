DROP INDEX IF EXISTS idx_quiz_attempt_drafts_expires;

DROP TABLE IF EXISTS quiz_attempt_drafts;

ALTER TABLE quiz_submissions
    DROP COLUMN IF EXISTS quiz_snapshot;

ALTER TABLE quizzes
    DROP COLUMN IF EXISTS time_limit_seconds;
