-- Record when and why a Creator cancelled a job, so incomplete contracts are
-- logged. Jobs cancelled before this migration have no known time or reason
-- and keep NULL for both.
ALTER TABLE "jobs" ADD COLUMN "cancelled_at" timestamp;
ALTER TABLE "jobs" ADD COLUMN "cancel_reason" varchar(300);

-- Cancellation details only belong to Cancelled jobs.
ALTER TABLE "jobs" ADD CONSTRAINT "chk_job_cancellation" CHECK (
    "job_status" = 'Cancelled' OR ("cancelled_at" IS NULL AND "cancel_reason" IS NULL)
);

COMMENT ON COLUMN "jobs"."cancelled_at" IS 'Set by the server when the Creator cancels; NULL for jobs cancelled before it was recorded';
COMMENT ON COLUMN "jobs"."cancel_reason" IS 'Optional Creator note given on cancellation';
