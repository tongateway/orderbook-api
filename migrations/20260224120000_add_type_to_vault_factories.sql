-- +goose Up
ALTER TABLE vault_factories ADD COLUMN IF NOT EXISTS type character varying;

-- +goose Down
ALTER TABLE vault_factories DROP COLUMN IF EXISTS type;
