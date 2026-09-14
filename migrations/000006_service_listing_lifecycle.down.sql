DROP TRIGGER IF EXISTS "trg_services_set_updated_at" ON "services";
DROP FUNCTION IF EXISTS "set_service_updated_at"();
DROP INDEX IF EXISTS "idx_services_active_marketer";

ALTER TABLE "services"
    DROP COLUMN IF EXISTS "deleted_at",
    DROP COLUMN IF EXISTS "updated_at";
