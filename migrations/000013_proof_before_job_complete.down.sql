DROP TRIGGER IF EXISTS "trg_check_proof_before_job_complete" ON "jobs";
DROP FUNCTION IF EXISTS "check_proof_before_job_complete"();
DROP INDEX IF EXISTS "idx_attachments_proof_job";
