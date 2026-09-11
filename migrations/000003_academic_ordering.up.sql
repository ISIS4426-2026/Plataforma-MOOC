-- Collision-free ordering for the academic hierarchy (issue #16).
--
-- Reordering rewrites every sibling's position in one transaction (see
-- internal/domain/ordering.go for the algorithm). A plain UNIQUE constraint
-- would reject that rewrite mid-transaction, the moment any UPDATE makes two
-- rows momentarily share a position before the rest of the pass corrects it.
-- DEFERRABLE INITIALLY DEFERRED postpones the check to COMMIT, so the
-- transaction only fails if a collision is still present once the whole
-- sibling set has been rewritten -- which the algorithm guarantees never
-- happens, since it always assigns a dense 0..n-1 sequence in a single pass.

ALTER TABLE modules
    ADD CONSTRAINT uq_modules_course_position UNIQUE (course_id, position) DEFERRABLE INITIALLY DEFERRED;

ALTER TABLE units
    ADD CONSTRAINT uq_units_module_position UNIQUE (module_id, position) DEFERRABLE INITIALLY DEFERRED;

ALTER TABLE resources
    ADD CONSTRAINT uq_resources_unit_position UNIQUE (unit_id, position) DEFERRABLE INITIALLY DEFERRED;
