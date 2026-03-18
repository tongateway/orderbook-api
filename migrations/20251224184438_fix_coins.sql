-- +goose Up
-- +goose StatementBegin

-- Make id_coingecko, name, and symbol nullable in coins table
ALTER TABLE coins ALTER COLUMN id_coingecko DROP NOT NULL;
ALTER TABLE coins ALTER COLUMN name DROP NOT NULL;
ALTER TABLE coins ALTER COLUMN symbol DROP NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Restore NOT NULL constraints
ALTER TABLE coins ALTER COLUMN id_coingecko SET NOT NULL;
ALTER TABLE coins ALTER COLUMN name SET NOT NULL;
ALTER TABLE coins ALTER COLUMN symbol SET NOT NULL;

-- +goose StatementEnd
