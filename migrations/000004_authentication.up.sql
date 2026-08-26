ALTER TABLE "users"
    ADD COLUMN "password_hash" varchar(255),
    ADD COLUMN "role" varchar(20) NOT NULL DEFAULT 'user';

ALTER TABLE "users"
    ADD CONSTRAINT "chk_user_role" CHECK ("role" IN ('user', 'admin'));
