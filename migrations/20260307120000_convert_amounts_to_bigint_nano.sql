-- +goose Up
-- +goose StatementBegin

-- Convert order amount fields from float (TON) to bigint (nano)
-- 1 TON = 1,000,000,000 nano

ALTER TABLE orders
  ALTER COLUMN amount TYPE bigint USING ROUND(COALESCE(amount, 0) * 1e9)::bigint,
  ALTER COLUMN initial_amount TYPE bigint USING ROUND(COALESCE(initial_amount, 0) * 1e9)::bigint,
  ALTER COLUMN price_rate TYPE bigint USING ROUND(COALESCE(price_rate, 0) * 1e9)::bigint,
  ALTER COLUMN slippage TYPE bigint USING ROUND(COALESCE(slippage, 0) * 1e9)::bigint;

-- Fix stuck orders: amount near zero but status still deployed (floating-point rounding bug)
UPDATE orders SET amount = 0, status = 'completed'
WHERE status = 'deployed' AND amount > 0 AND amount < 10000;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE orders
  ALTER COLUMN amount TYPE double precision USING amount::double precision / 1e9,
  ALTER COLUMN initial_amount TYPE double precision USING initial_amount::double precision / 1e9,
  ALTER COLUMN price_rate TYPE double precision USING price_rate::double precision / 1e9,
  ALTER COLUMN slippage TYPE double precision USING slippage::double precision / 1e9;

-- +goose StatementEnd
