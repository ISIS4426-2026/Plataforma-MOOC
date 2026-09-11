-- Database-level immutability for the audit trail (issue #18).
--
-- A trigger is used instead of (or as well as) revoking UPDATE/DELETE grants
-- from the application role: a grant revocation only protects the trail from
-- whichever role the API happens to connect as in a given environment, and
-- that role's name is not something this migration can know. A trigger
-- rejects the operation on the table itself, so the guarantee holds
-- regardless of which role attempts it -- the application role, a future
-- migration, or a developer connected directly with psql.

CREATE OR REPLACE FUNCTION reject_audit_log_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'audit_logs is append-only: % is not permitted on an existing row', TG_OP;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER audit_logs_no_update
    BEFORE UPDATE ON audit_logs
    FOR EACH ROW EXECUTE FUNCTION reject_audit_log_mutation();

CREATE TRIGGER audit_logs_no_delete
    BEFORE DELETE ON audit_logs
    FOR EACH ROW EXECUTE FUNCTION reject_audit_log_mutation();
