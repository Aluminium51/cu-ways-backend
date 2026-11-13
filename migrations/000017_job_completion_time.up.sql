-- Store the exact completion and simulated payout release instant.
-- Local databases from the US032 preview may already have this column.
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS completed_at timestamptz;
