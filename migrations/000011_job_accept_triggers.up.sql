-- Hiring a Marketer sets jobs.accepted_offer_id. The application performs the
-- whole transition (job and offer statuses); this trigger only guards the
-- result, whichever code path writes the column, and never changes a row.
-- Offers must therefore be synced before the job row is updated.
CREATE FUNCTION "fn_guard_job_accepted_offer"() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    v_offer_status varchar(20);
BEGIN
    -- Clearing the column is how a hired job's rows are deleted, because the
    -- jobs and offers foreign keys reference each other.
    IF NEW."accepted_offer_id" IS NULL THEN
        RETURN NEW;
    END IF;
    IF OLD."job_status" <> 'Pending' THEN
        RAISE EXCEPTION 'job % is % and its accepted offer cannot change', NEW."job_id", OLD."job_status"
            USING ERRCODE = 'check_violation', CONSTRAINT = 'trg_guard_job_accepted_offer';
    END IF;
    IF NEW."job_status" = 'Pending' THEN
        RAISE EXCEPTION 'job % must leave Pending when it accepts offer %', NEW."job_id", NEW."accepted_offer_id"
            USING ERRCODE = 'check_violation', CONSTRAINT = 'trg_guard_job_accepted_offer';
    END IF;

    SELECT "offer_status" INTO v_offer_status
    FROM "offers"
    WHERE "offer_id" = NEW."accepted_offer_id" AND "job_id" = NEW."job_id";
    IF NOT FOUND OR v_offer_status <> 'Accepted' THEN
        RAISE EXCEPTION 'offer % is not an Accepted offer of job %', NEW."accepted_offer_id", NEW."job_id"
            USING ERRCODE = 'check_violation', CONSTRAINT = 'trg_guard_job_accepted_offer';
    END IF;
    IF EXISTS (
        SELECT 1 FROM "offers"
        WHERE "job_id" = NEW."job_id"
          AND "offer_id" <> NEW."accepted_offer_id"
          AND "offer_status" IN ('Pending', 'Accepted')
    ) THEN
        RAISE EXCEPTION 'job % still has other active offers after accepting offer %', NEW."job_id", NEW."accepted_offer_id"
            USING ERRCODE = 'check_violation', CONSTRAINT = 'trg_guard_job_accepted_offer';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER "trg_guard_job_accepted_offer"
    BEFORE UPDATE OF "accepted_offer_id" ON "jobs"
    FOR EACH ROW
    WHEN (NEW."accepted_offer_id" IS DISTINCT FROM OLD."accepted_offer_id")
    EXECUTE FUNCTION "fn_guard_job_accepted_offer"();
