-- Hiring a Marketer is recorded by setting jobs.accepted_offer_id. These
-- triggers keep the job and its offers consistent with that column, whichever
-- code path performs the update.

-- Reject an accepted offer that does not belong to the job or is no longer
-- active, before the row is written. A Pending job whose caller did not choose
-- another status becomes Accepted; the Marketer later starts it.
CREATE FUNCTION "fn_validate_job_accepted_offer"() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    v_offer_status varchar(20);
BEGIN
    SELECT "offer_status" INTO v_offer_status
    FROM "offers"
    WHERE "offer_id" = NEW."accepted_offer_id" AND "job_id" = NEW."job_id";

    IF NOT FOUND THEN
        RAISE EXCEPTION 'accepted offer % does not belong to job %', NEW."accepted_offer_id", NEW."job_id"
            USING ERRCODE = 'CW001', CONSTRAINT = 'trg_validate_job_accepted_offer';
    END IF;
    IF v_offer_status NOT IN ('Pending', 'Accepted') THEN
        RAISE EXCEPTION 'offer % is % and cannot be accepted for job %', NEW."accepted_offer_id", v_offer_status, NEW."job_id"
            USING ERRCODE = 'CW001', CONSTRAINT = 'trg_validate_job_accepted_offer';
    END IF;

    IF OLD."job_status" = 'Pending' AND NEW."job_status" = 'Pending' THEN
        NEW."job_status" := 'Accepted';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER "trg_validate_job_accepted_offer"
    BEFORE UPDATE OF "accepted_offer_id" ON "jobs"
    FOR EACH ROW
    WHEN (NEW."accepted_offer_id" IS NOT NULL AND NEW."accepted_offer_id" IS DISTINCT FROM OLD."accepted_offer_id")
    EXECUTE FUNCTION "fn_validate_job_accepted_offer"();

-- Once a Pending job hires, its accepted offer becomes Accepted and every other
-- Pending offer on the job becomes Rejected. Withdrawn and Rejected history is
-- left unchanged.
CREATE FUNCTION "fn_sync_offer_status_after_job_accept"() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    UPDATE "offers" SET "offer_status" = 'Accepted'
    WHERE "offer_id" = NEW."accepted_offer_id" AND "offer_status" <> 'Accepted';

    UPDATE "offers" SET "offer_status" = 'Rejected'
    WHERE "job_id" = NEW."job_id"
      AND "offer_id" <> NEW."accepted_offer_id"
      AND "offer_status" = 'Pending';
    RETURN NULL;
END;
$$;

CREATE TRIGGER "trg_sync_offer_status_after_job_accept"
    AFTER UPDATE OF "accepted_offer_id" ON "jobs"
    FOR EACH ROW
    WHEN (OLD."job_status" = 'Pending' AND NEW."accepted_offer_id" IS NOT NULL
          AND NEW."accepted_offer_id" IS DISTINCT FROM OLD."accepted_offer_id")
    EXECUTE FUNCTION "fn_sync_offer_status_after_job_accept"();
