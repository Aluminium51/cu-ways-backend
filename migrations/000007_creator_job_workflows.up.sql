CREATE TABLE "job_requests" (
    "job_request_id" integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "job_id" integer NOT NULL,
    "marketer_id" integer NOT NULL,
    "request_status" varchar(20) NOT NULL DEFAULT 'Pending',
    "created_at" timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "responded_at" timestamp,
    CONSTRAINT "chk_job_request_status" CHECK (
        "request_status" IN ('Pending', 'Accepted', 'Declined', 'Closed')
    ),
    CONSTRAINT "fk_job_request_job" FOREIGN KEY ("job_id")
        REFERENCES "jobs" ("job_id") ON DELETE RESTRICT,
    CONSTRAINT "fk_job_request_marketer" FOREIGN KEY ("marketer_id")
        REFERENCES "marketers" ("user_id") ON DELETE RESTRICT
);

-- A pending or accepted invitation is active. Declined and closed rows remain
-- as history and permit a later invitation to the same marketer.
CREATE UNIQUE INDEX "idx_job_request_active_unique"
    ON "job_requests" ("job_id", "marketer_id")
    WHERE "request_status" IN ('Pending', 'Accepted');

CREATE INDEX "idx_job_requests_marketer_status"
    ON "job_requests" ("marketer_id", "request_status", "created_at" DESC);

CREATE INDEX "idx_job_requests_job"
    ON "job_requests" ("job_id", "created_at" DESC);
