-- +goose Up
-- +goose StatementBegin
ALTER TABLE orders
  ADD COLUMN opposite_vault_id INTEGER;

ALTER TABLE orders
  ADD CONSTRAINT orders_opposite_vault_id_fkey
  FOREIGN KEY (opposite_vault_id) REFERENCES vaults(id)
  ON DELETE SET NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE orders
  DROP CONSTRAINT IF EXISTS orders_opposite_vault_id_fkey;

ALTER TABLE orders
  DROP COLUMN IF EXISTS opposite_vault_id;
-- +goose StatementEnd
