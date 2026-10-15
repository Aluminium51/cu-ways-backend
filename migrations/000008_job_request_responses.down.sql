ALTER TABLE "job_requests" DROP CONSTRAINT IF EXISTS "chk_job_request_decline_details";
ALTER TABLE "job_requests" DROP CONSTRAINT IF EXISTS "chk_job_request_decline_reason";
ALTER TABLE "job_requests"
    DROP COLUMN IF EXISTS "decline_note",
    DROP COLUMN IF EXISTS "decline_reason";
