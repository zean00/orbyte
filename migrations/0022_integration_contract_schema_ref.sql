ALTER TABLE integration_contracts
ADD COLUMN IF NOT EXISTS schema_ref TEXT;
