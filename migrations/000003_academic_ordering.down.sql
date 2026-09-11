ALTER TABLE resources DROP CONSTRAINT IF EXISTS uq_resources_unit_position;
ALTER TABLE units DROP CONSTRAINT IF EXISTS uq_units_module_position;
ALTER TABLE modules DROP CONSTRAINT IF EXISTS uq_modules_course_position;
