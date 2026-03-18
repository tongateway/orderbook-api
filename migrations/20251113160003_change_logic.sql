-- +goose Up
-- +goose StatementBegin
ALTER TABLE orders DROP COLUMN _type;

DROP TYPE ordertype;

ALTER TABLE orders DROP COLUMN to_jetton_minter_address;
ALTER TABLE orders DROP COLUMN deployed;
ALTER TABLE orders DROP COLUMN deployed_at;
ALTER TABLE orders ADD COLUMN from_coin_id INTEGER;
ALTER TABLE orders ADD COLUMN to_coin_id INTEGER;
ALTER TABLE orders ADD FOREIGN KEY (from_coin_id) REFERENCES coins(id);
ALTER TABLE orders ADD FOREIGN KEY (to_coin_id) REFERENCES coins(id);

ALTER TABLE coins RENAME COLUMN created_at_db TO created_at;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE TYPE ordertype AS ENUM ('jetton_to_jetton', 'jetton_to_ton', 'ton_to_jetton');

ALTER TABLE orders ADD COLUMN _type ordertype;
ALTER TABLE orders ADD COLUMN to_jetton_minter_address VARCHAR(67);
ALTER TABLE orders ADD COLUMN deployed BOOLEAN DEFAULT FALSE;
ALTER TABLE orders ADD COLUMN deployed_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE orders DROP COLUMN from_coin_id;
ALTER TABLE orders DROP COLUMN to_coin_id;
ALTER TABLE orders DROP FOREIGN KEY orders_from_coin_id_fkey;
ALTER TABLE orders DROP FOREIGN KEY orders_to_coin_id_fkey;

ALTER TABLE coins RENAME COLUMN created_at TO created_at_db;
-- +goose StatementEnd