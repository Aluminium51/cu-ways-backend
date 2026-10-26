-- The trigger only guards writes the application already performs, so it can
-- be dropped without changing application behavior.
DROP TRIGGER IF EXISTS "trg_guard_job_accepted_offer" ON "jobs";
DROP FUNCTION IF EXISTS "fn_guard_job_accepted_offer"();
