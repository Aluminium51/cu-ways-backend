ALTER TABLE "services"
    ADD COLUMN "updated_at" timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ADD COLUMN "deleted_at" timestamp;

COMMENT ON COLUMN "services"."updated_at" IS
    'Time when the service listing was last edited, including soft deletion.';
COMMENT ON COLUMN "services"."deleted_at" IS
    'Soft-delete marker. NULL means the service listing is currently published.';

CREATE INDEX "idx_services_active_marketer"
    ON "services" ("user_id")
    WHERE "deleted_at" IS NULL;

CREATE FUNCTION "set_service_updated_at"()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    NEW."updated_at" = clock_timestamp();
    RETURN NEW;
END;
$$;

CREATE TRIGGER "trg_services_set_updated_at"
BEFORE UPDATE ON "services"
FOR EACH ROW
EXECUTE FUNCTION "set_service_updated_at"();
