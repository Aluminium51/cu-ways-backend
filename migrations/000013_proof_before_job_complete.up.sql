CREATE INDEX "idx_attachments_proof_job" ON "attachments" ("job_id")
WHERE "attachment_type" = 'Proof';

CREATE FUNCTION "check_proof_before_job_complete"()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM "attachments"
        WHERE "job_id" = NEW."job_id" AND "attachment_type" = 'Proof'
    ) THEN
        RAISE EXCEPTION 'Job % cannot be completed: Proof attachment is required', NEW."job_id";
    END IF;

    IF NEW."accepted_offer_id" IS NULL THEN
        RAISE EXCEPTION 'Job % cannot be completed: accepted offer is required', NEW."job_id";
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER "trg_check_proof_before_job_complete"
BEFORE UPDATE OF "job_status" ON "jobs"
FOR EACH ROW
WHEN (NEW."job_status" = 'Completed' AND OLD."job_status" IS DISTINCT FROM NEW."job_status")
EXECUTE FUNCTION "check_proof_before_job_complete"();
