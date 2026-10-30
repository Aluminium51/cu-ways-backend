ALTER TABLE "reviews" DROP CONSTRAINT "chk_rating";
ALTER TABLE "reviews" ADD CONSTRAINT "chk_rating" CHECK ("rating" BETWEEN 0 AND 5);
COMMENT ON COLUMN "reviews"."rating" IS '0-5';
