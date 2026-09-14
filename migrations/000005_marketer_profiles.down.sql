DROP INDEX IF EXISTS "idx_marketer_campuses_option";
DROP INDEX IF EXISTS "idx_marketer_expertise_option";

DROP TABLE IF EXISTS "marketer_campuses";
DROP TABLE IF EXISTS "marketer_expertise";
DROP TABLE IF EXISTS "campus_options";
DROP TABLE IF EXISTS "expertise_options";

ALTER TABLE "marketers"
    DROP CONSTRAINT IF EXISTS "chk_marketer_availability_status",
    DROP CONSTRAINT IF EXISTS "chk_marketer_experience_years",
    ALTER COLUMN "bio" DROP NOT NULL,
    ALTER COLUMN "availability_text" DROP NOT NULL,
    DROP COLUMN IF EXISTS "availability_status",
    DROP COLUMN IF EXISTS "experience_years";

DROP INDEX IF EXISTS "idx_users_phone_unique";
