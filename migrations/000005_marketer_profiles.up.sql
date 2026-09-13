-- Contact numbers are canonicalized by the application and unique when present.
CREATE UNIQUE INDEX "idx_users_phone_unique"
    ON "users" ("phone")
    WHERE "phone" IS NOT NULL;

-- Keep the legacy experience text for backward compatibility, while exposing
-- normalized fields for filtering and sorting.
ALTER TABLE "marketers"
    ADD COLUMN "experience_years" integer,
    ADD COLUMN "availability_status" varchar(20) DEFAULT 'available';

UPDATE "marketers"
SET "bio" = COALESCE("bio", ''),
    "availability_text" = COALESCE("availability_text", ''),
    "experience_years" = COALESCE("experience_years", 0),
    "availability_status" = COALESCE("availability_status", 'available');

ALTER TABLE "marketers"
    ALTER COLUMN "bio" SET NOT NULL,
    ALTER COLUMN "availability_text" SET NOT NULL,
    ALTER COLUMN "experience_years" SET NOT NULL,
    ALTER COLUMN "availability_status" SET NOT NULL,
    ALTER COLUMN "availability_status" DROP DEFAULT;

ALTER TABLE "marketers"
    ADD CONSTRAINT "chk_marketer_experience_years"
        CHECK ("experience_years" BETWEEN 0 AND 80),
    ADD CONSTRAINT "chk_marketer_availability_status"
        CHECK ("availability_status" IN ('available', 'limited', 'unavailable'));

CREATE TABLE "expertise_options" (
    "expertise_id" integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "slug" varchar(80) UNIQUE NOT NULL,
    "name" varchar(120) NOT NULL
);

CREATE TABLE "campus_options" (
    "campus_id" integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "slug" varchar(80) UNIQUE NOT NULL,
    "name" varchar(120) NOT NULL
);

CREATE TABLE "marketer_expertise" (
    "user_id" integer NOT NULL,
    "expertise_id" integer NOT NULL,
    PRIMARY KEY ("user_id", "expertise_id"),
    CONSTRAINT "fk_marketer_expertise_marketer"
        FOREIGN KEY ("user_id") REFERENCES "marketers" ("user_id") ON DELETE CASCADE,
    CONSTRAINT "fk_marketer_expertise_option"
        FOREIGN KEY ("expertise_id") REFERENCES "expertise_options" ("expertise_id") ON DELETE RESTRICT
);

CREATE TABLE "marketer_campuses" (
    "user_id" integer NOT NULL,
    "campus_id" integer NOT NULL,
    PRIMARY KEY ("user_id", "campus_id"),
    CONSTRAINT "fk_marketer_campuses_marketer"
        FOREIGN KEY ("user_id") REFERENCES "marketers" ("user_id") ON DELETE CASCADE,
    CONSTRAINT "fk_marketer_campuses_option"
        FOREIGN KEY ("campus_id") REFERENCES "campus_options" ("campus_id") ON DELETE RESTRICT
);

CREATE INDEX "idx_marketer_expertise_option" ON "marketer_expertise" ("expertise_id");
CREATE INDEX "idx_marketer_campuses_option" ON "marketer_campuses" ("campus_id");

INSERT INTO "expertise_options" ("slug", "name") VALUES
    ('survey-distribution', 'Survey Distribution'),
    ('participant-recruitment', 'Participant Recruitment'),
    ('data-collection', 'Data Collection'),
    ('quantitative-analysis', 'Quantitative Analysis'),
    ('qualitative-analysis', 'Qualitative Analysis'),
    ('report-preparation', 'Report Preparation');

INSERT INTO "campus_options" ("slug", "name") VALUES
    ('cu-main-campus', 'CU Main Campus'),
    ('cu-health-sciences-campus', 'CU Health Sciences Campus'),
    ('off-campus', 'Off-campus'),
    ('online-remote', 'Online / Remote');
