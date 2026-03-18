-- +goose Up
-- +goose StatementBegin
ALTER TABLE orders ADD COLUMN initial_amount DOUBLE PRECISION;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE orders DROP COLUMN IF EXISTS initial_amount;
-- +goose StatementEnd
