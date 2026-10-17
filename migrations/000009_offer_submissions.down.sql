DROP INDEX IF EXISTS "idx_offers_active_job_request";
ALTER TABLE "offers" DROP CONSTRAINT IF EXISTS "fk_offer_job_request";
ALTER TABLE "offers"
    DROP COLUMN IF EXISTS "message",
    DROP COLUMN IF EXISTS "delivery_date",
    DROP COLUMN IF EXISTS "job_request_id";
