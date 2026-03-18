-- +goose Up
-- +goose StatementBegin

-- Drop indexes first (they will be dropped automatically with tables, but explicit is better)
DROP INDEX IF EXISTS idx_transactions_user_id;
DROP INDEX IF EXISTS idx_transactions_trs_hash;
DROP INDEX IF EXISTS idx_blocks_workchain_shard_seqno;

-- Drop tables
DROP TABLE IF EXISTS transactions;
DROP TABLE IF EXISTS blocks;
DROP TABLE IF EXISTS ton_connect_storage;

-- Drop ENUM types that were only used by transactions table
DROP TYPE IF EXISTS transactionaction;
DROP TYPE IF EXISTS transactionstatus;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Recreate ENUM types
CREATE TYPE transactionaction AS ENUM ('create_order', 'cancel_order', 'complete_order', 'fail_order');
CREATE TYPE transactionstatus AS ENUM ('created', 'pending', 'completed', 'failed');

-- Recreate ton_connect_storage table
CREATE TABLE ton_connect_storage (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- Recreate blocks table
CREATE TABLE blocks (
    id BIGSERIAL PRIMARY KEY,
    workchain INTEGER NOT NULL,
    shard BIGINT NOT NULL,
    seq_no BIGINT NOT NULL,
    root_hash CHARACTER(64) NOT NULL,
    file_hash CHARACTER(64) NOT NULL,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    last_lt BIGINT NOT NULL,
    done BOOLEAN NOT NULL DEFAULT FALSE,
    in_progress BOOLEAN NOT NULL DEFAULT FALSE
);

-- Recreate transactions table
CREATE TABLE transactions (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    action transactionaction NOT NULL,
    status transactionstatus NOT NULL DEFAULT 'created',
    raw_body CHARACTER VARYING,
    trs_hash CHARACTER VARYING UNIQUE,
    CONSTRAINT transactions_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Recreate indexes
CREATE INDEX idx_transactions_user_id ON transactions(user_id);
CREATE INDEX idx_transactions_trs_hash ON transactions(trs_hash);
CREATE INDEX idx_blocks_workchain_shard_seqno ON blocks(workchain, shard, seq_no);

-- +goose StatementEnd
