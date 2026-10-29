ALTER TABLE "reviews" DROP CONSTRAINT "chk_rating";
-- Enforce 1-5 for new rows without rewriting any historical 0-star reviews.
ALTER TABLE "reviews" ADD CONSTRAINT "chk_rating" CHECK ("rating" BETWEEN 1 AND 5) NOT VALID;
COMMENT ON COLUMN "reviews"."rating" IS '1-5 stars';
