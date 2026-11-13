-- An earlier US032 preview installed the column at this version.
ALTER TABLE jobs DROP COLUMN IF EXISTS completed_at;
