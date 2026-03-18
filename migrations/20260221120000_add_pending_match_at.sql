-- +goose Up
-- +goose StatementBegin
ALTER TABLE orders
  ADD COLUMN pending_match_at TIMESTAMPTZ;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE orders
  DROP COLUMN IF EXISTS pending_match_at;
-- +goose StatementEnd
