ALTER TABLE "offers"
    ADD COLUMN "job_request_id" integer,
    ADD COLUMN "delivery_date" date,
    ADD COLUMN "message" varchar(300);

COMMENT ON COLUMN "offers"."job_request_id" IS
    'Accepted invitation the offer answers. NULL only for offers created before invitations existed.';
COMMENT ON COLUMN "offers"."delivery_date" IS
    'Estimated delivery date proposed by the Marketer.';
COMMENT ON COLUMN "offers"."message" IS
    'Optional message from the Marketer to the Creator.';

ALTER TABLE "offers"
    ADD CONSTRAINT "fk_offer_job_request"
    FOREIGN KEY ("job_request_id") REFERENCES "job_requests" ("job_request_id") ON DELETE RESTRICT;

-- One active (Pending or Accepted) offer per invitation. Withdrawn and
-- rejected offers stay as history and allow a new offer.
CREATE UNIQUE INDEX "idx_offers_active_job_request"
    ON "offers" ("job_request_id")
    WHERE "offer_status" IN ('Pending', 'Accepted');
