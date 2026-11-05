ALTER TABLE "payments" DROP CONSTRAINT "chk_payment_method";
ALTER TABLE "payments" ADD CONSTRAINT "chk_payment_method"
    CHECK ("method" IN ('Bank Transfer', 'Credit Card', 'PromptPay', 'Other'));
ALTER TABLE "payments" DROP COLUMN "created_at";
DROP INDEX "idx_payments_job_id";
