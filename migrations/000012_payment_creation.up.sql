-- Historical payments have no known creation time; preserve NULL for them.
ALTER TABLE "payments" ADD COLUMN "created_at" timestamp;
CREATE INDEX "idx_payments_job_id" ON "payments" ("job_id");
ALTER TABLE "payments" DROP CONSTRAINT "chk_payment_method";
-- Enforce supported methods on new rows while preserving legacy Other rows.
ALTER TABLE "payments" ADD CONSTRAINT "chk_payment_method"
    CHECK ("method" IN ('PromptPay', 'Bank Transfer', 'Credit Card')) NOT VALID;
