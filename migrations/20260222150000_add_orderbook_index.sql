-- +goose NO TRANSACTION

-- +goose Up
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_orders_orderbook
ON orders (from_coin_id, to_coin_id, price_rate)
WHERE status = 'deployed';

-- +goose Down
DROP INDEX IF EXISTS idx_orders_orderbook;
