ALTER TABLE "job_requests"
    ADD COLUMN "decline_reason" varchar(50),
    ADD COLUMN "decline_note" varchar(300);

COMMENT ON COLUMN "job_requests"."decline_reason" IS
    'Optional reason chosen by the Marketer when declining. NULL unless the request is Declined.';
COMMENT ON COLUMN "job_requests"."decline_note" IS
    'Optional free-text note from the Marketer when declining. NULL unless the request is Declined.';

ALTER TABLE "job_requests" ADD CONSTRAINT "chk_job_request_decline_reason" CHECK (
    "decline_reason" IS NULL OR "decline_reason" IN (
        'Schedule conflict', 'Budget too low', 'Target audience mismatch', 'Fully booked'
    )
);

-- Decline details describe a decline only; other statuses must not carry them.
ALTER TABLE "job_requests" ADD CONSTRAINT "chk_job_request_decline_details" CHECK (
    "request_status" = 'Declined' OR ("decline_reason" IS NULL AND "decline_note" IS NULL)
);
