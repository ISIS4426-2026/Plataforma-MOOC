ALTER TABLE quiz_options
    DROP CONSTRAINT IF EXISTS uq_quiz_options_question_position;

ALTER TABLE quiz_options
    DROP COLUMN IF EXISTS position;

DROP INDEX IF EXISTS idx_quiz_submissions_attempt;

ALTER TABLE quizzes
    DROP CONSTRAINT IF EXISTS uq_quizzes_resource;

ALTER TABLE quiz_questions
    DROP CONSTRAINT IF EXISTS uq_quiz_questions_quiz_position;

ALTER TABLE quiz_submissions
    DROP CONSTRAINT IF EXISTS uq_quiz_submissions_attempt;

ALTER TABLE quiz_submissions
    DROP COLUMN IF EXISTS attempt;

ALTER TABLE quizzes
    DROP COLUMN IF EXISTS max_attempts;
