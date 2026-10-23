DROP TRIGGER IF EXISTS "trg_sync_offer_status_after_job_accept" ON "jobs";
DROP FUNCTION IF EXISTS "fn_sync_offer_status_after_job_accept"();
DROP TRIGGER IF EXISTS "trg_validate_job_accepted_offer" ON "jobs";
DROP FUNCTION IF EXISTS "fn_validate_job_accepted_offer"();
