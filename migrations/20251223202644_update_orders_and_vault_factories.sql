-- +goose Up
-- +goose StatementBegin

-- Add new fields to orders table
ALTER TABLE orders ADD COLUMN provider_raw_address CHARACTER VARYING(67);
ALTER TABLE orders ADD COLUMN fee_num INTEGER;
ALTER TABLE orders ADD COLUMN fee_denom INTEGER;
ALTER TABLE orders ADD COLUMN matcher_fee_num INTEGER;
ALTER TABLE orders ADD COLUMN matcher_fee_denom INTEGER;

-- Remove fields from vault_factories table
ALTER TABLE vault_factories DROP COLUMN IF EXISTS commision_rate;
ALTER TABLE vault_factories DROP COLUMN IF EXISTS matcher_commision_rate;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Remove fields from orders table
ALTER TABLE orders DROP COLUMN IF EXISTS provider_raw_address;
ALTER TABLE orders DROP COLUMN IF EXISTS fee_num;
ALTER TABLE orders DROP COLUMN IF EXISTS fee_denom;
ALTER TABLE orders DROP COLUMN IF EXISTS matcher_fee_num;
ALTER TABLE orders DROP COLUMN IF EXISTS matcher_fee_denom;

-- Restore fields to vault_factories table
ALTER TABLE vault_factories ADD COLUMN commision_rate DOUBLE PRECISION NOT NULL DEFAULT 0.0;
ALTER TABLE vault_factories ADD COLUMN matcher_commision_rate DOUBLE PRECISION NOT NULL DEFAULT 0.0;

-- +goose StatementEnd

