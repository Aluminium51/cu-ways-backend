CREATE TABLE "users" (
    "user_id" integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "name" varchar(100) NOT NULL,
    "email" varchar(255) UNIQUE NOT NULL,
    "phone" varchar(20),
    "line_id" varchar(50),
    "created_at" timestamp NOT NULL
);

CREATE TABLE "creators" (
    "user_id" integer PRIMARY KEY
);

CREATE TABLE "marketers" (
    "user_id" integer PRIMARY KEY,
    "bio" text,
    "experience" text,
    "availability_text" text
);

CREATE TABLE "services" (
    "service_id" integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "user_id" integer NOT NULL,
    "service_type" varchar(100) NOT NULL,
    "scope_text" text,
    "price" decimal(10, 2) NOT NULL,
    "created_at" timestamp NOT NULL
);

CREATE TABLE "surveys" (
    "survey_id" integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "user_id" integer NOT NULL,
    "title" varchar(200) NOT NULL,
    "description" text,
    "survey_link" text NOT NULL,
    "target_group" varchar(200),
    "desired_responses" integer,
    "deadline" timestamp,
    "created_at" timestamp NOT NULL
);

CREATE TABLE "jobs" (
    "job_id" integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "user_id" integer NOT NULL,
    "accepted_offer_id" integer UNIQUE,
    "job_status" varchar(20) NOT NULL,
    "created_at" timestamp NOT NULL
);

CREATE TABLE "is_used_in" (
    "job_id" integer NOT NULL,
    "survey_id" integer NOT NULL,
    PRIMARY KEY ("job_id", "survey_id")
);

CREATE TABLE "offers" (
    "offer_id" integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "job_id" integer NOT NULL,
    "user_id" integer NOT NULL,
    "offered_price" decimal(10, 2) NOT NULL,
    "offer_status" varchar(20) NOT NULL,
    "created_at" timestamp NOT NULL
);

CREATE TABLE "attachments" (
    "attachment_id" integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "job_id" integer NOT NULL,
    "attachment_type" varchar(30) NOT NULL,
    "url" text NOT NULL,
    "upload_at" timestamp NOT NULL
);

CREATE TABLE "payments" (
    "payment_id" integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "job_id" integer NOT NULL,
    "amount" decimal(10, 2) NOT NULL,
    "method" varchar(30) NOT NULL,
    "payment_status" varchar(20) NOT NULL,
    "paid_at" timestamp
);

CREATE TABLE "reviews" (
    "review_id" integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "job_id" integer UNIQUE NOT NULL,
    "rating" integer NOT NULL,
    "comment" text,
    "created_at" timestamp NOT NULL
);

CREATE UNIQUE INDEX "idx_offers_job_offer" ON "offers" ("job_id", "offer_id");

COMMENT ON TABLE "users" IS 'Superclass. Add DB constraint/trigger if you want to enforce complete inheritance: every user must be creator or marketer or both.';
COMMENT ON COLUMN "creators"."user_id" IS 'PK and FK to users.user_id';
COMMENT ON COLUMN "marketers"."user_id" IS 'PK and FK to users.user_id';
COMMENT ON COLUMN "services"."user_id" IS 'Refers to marketers.user_id';
COMMENT ON COLUMN "surveys"."user_id" IS 'Refers to creators.user_id';
COMMENT ON TABLE "jobs" IS 'accepted_offer_id is nullable because a job can exist before any offer is accepted.';
COMMENT ON COLUMN "jobs"."user_id" IS 'Refers to creators.user_id';
COMMENT ON COLUMN "jobs"."accepted_offer_id" IS 'Nullable until one offer is accepted';
COMMENT ON COLUMN "jobs"."job_status" IS 'Pending | Accepted | In Progress | Completed | Cancelled';
COMMENT ON TABLE "is_used_in" IS 'Bridge table for M:N relation between JOB and SURVEY';
COMMENT ON TABLE "offers" IS 'Option A: keep offer_status including Accepted.';
COMMENT ON COLUMN "offers"."user_id" IS 'Refers to marketers.user_id';
COMMENT ON COLUMN "offers"."offer_status" IS 'Pending | Accepted | Rejected | Withdrawn';
COMMENT ON COLUMN "attachments"."attachment_type" IS 'Brief | Proof | Other';
COMMENT ON COLUMN "payments"."method" IS 'Bank Transfer | Credit Card | PromptPay | Other';
COMMENT ON COLUMN "payments"."payment_status" IS 'Pending | Paid | Failed | Refunded';
COMMENT ON COLUMN "reviews"."job_id" IS 'At most 1 review per job';
COMMENT ON COLUMN "reviews"."rating" IS '0-5';

ALTER TABLE "jobs" ADD CONSTRAINT "chk_job_status" CHECK (
    "job_status" IN ('Pending', 'Accepted', 'In Progress', 'Completed', 'Cancelled')
);

ALTER TABLE "offers" ADD CONSTRAINT "chk_offer_status" CHECK (
    "offer_status" IN ('Pending', 'Accepted', 'Rejected', 'Withdrawn')
);

ALTER TABLE "payments" ADD CONSTRAINT "chk_payment_status" CHECK (
    "payment_status" IN ('Pending', 'Paid', 'Failed', 'Refunded')
);

ALTER TABLE "payments" ADD CONSTRAINT "chk_payment_method" CHECK (
    "method" IN ('Bank Transfer', 'Credit Card', 'PromptPay', 'Other')
);

ALTER TABLE "attachments" ADD CONSTRAINT "chk_attachment_type" CHECK (
    "attachment_type" IN ('Brief', 'Proof', 'Other')
);

ALTER TABLE "services" ADD CONSTRAINT "chk_service_price" CHECK ("price" >= 0);
ALTER TABLE "offers" ADD CONSTRAINT "chk_offer_price" CHECK ("offered_price" >= 0);
ALTER TABLE "payments" ADD CONSTRAINT "chk_payment_amount" CHECK ("amount" >= 0);
ALTER TABLE "reviews" ADD CONSTRAINT "chk_rating" CHECK ("rating" BETWEEN 0 AND 5);
ALTER TABLE "surveys" ADD CONSTRAINT "chk_desired_responses" CHECK (
    "desired_responses" IS NULL OR "desired_responses" > 0
);

ALTER TABLE "creators"
    ADD CONSTRAINT "fk_creator_user"
    FOREIGN KEY ("user_id") REFERENCES "users" ("user_id") ON DELETE RESTRICT;

ALTER TABLE "marketers"
    ADD CONSTRAINT "fk_marketer_user"
    FOREIGN KEY ("user_id") REFERENCES "users" ("user_id") ON DELETE RESTRICT;

ALTER TABLE "services"
    ADD CONSTRAINT "fk_service_marketer"
    FOREIGN KEY ("user_id") REFERENCES "marketers" ("user_id") ON DELETE RESTRICT;

ALTER TABLE "surveys"
    ADD CONSTRAINT "fk_survey_creator"
    FOREIGN KEY ("user_id") REFERENCES "creators" ("user_id") ON DELETE RESTRICT;

ALTER TABLE "jobs"
    ADD CONSTRAINT "fk_job_creator"
    FOREIGN KEY ("user_id") REFERENCES "creators" ("user_id") ON DELETE RESTRICT;

ALTER TABLE "is_used_in"
    ADD CONSTRAINT "fk_used_job"
    FOREIGN KEY ("job_id") REFERENCES "jobs" ("job_id") ON DELETE RESTRICT;

ALTER TABLE "is_used_in"
    ADD CONSTRAINT "fk_used_survey"
    FOREIGN KEY ("survey_id") REFERENCES "surveys" ("survey_id") ON DELETE RESTRICT;

ALTER TABLE "offers"
    ADD CONSTRAINT "fk_offer_job"
    FOREIGN KEY ("job_id") REFERENCES "jobs" ("job_id") ON DELETE RESTRICT;

ALTER TABLE "offers"
    ADD CONSTRAINT "fk_offer_marketer"
    FOREIGN KEY ("user_id") REFERENCES "marketers" ("user_id") ON DELETE RESTRICT;

ALTER TABLE "jobs"
    ADD CONSTRAINT "fk_job_accepted_offer"
    FOREIGN KEY ("job_id", "accepted_offer_id")
    REFERENCES "offers" ("job_id", "offer_id") ON DELETE RESTRICT;

ALTER TABLE "attachments"
    ADD CONSTRAINT "fk_attachment_job"
    FOREIGN KEY ("job_id") REFERENCES "jobs" ("job_id") ON DELETE RESTRICT;

ALTER TABLE "payments"
    ADD CONSTRAINT "fk_payment_job"
    FOREIGN KEY ("job_id") REFERENCES "jobs" ("job_id") ON DELETE RESTRICT;

ALTER TABLE "reviews"
    ADD CONSTRAINT "fk_review_job"
    FOREIGN KEY ("job_id") REFERENCES "jobs" ("job_id") ON DELETE RESTRICT;
