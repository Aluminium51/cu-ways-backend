ALTER TABLE "jobs" DROP CONSTRAINT "chk_job_cancellation";
ALTER TABLE "jobs" DROP COLUMN "cancel_reason";
ALTER TABLE "jobs" DROP COLUMN "cancelled_at";
