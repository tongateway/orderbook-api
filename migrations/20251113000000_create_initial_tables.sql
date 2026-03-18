-- +goose Up
-- +goose StatementBegin

-- Create ENUM types
CREATE TYPE orderstatus AS ENUM ('created', 'deployed', 'cancelled', 'completed', 'failed', 'pending_match');
CREATE TYPE transactionaction AS ENUM ('create_order', 'cancel_order', 'complete_order', 'fail_order');
CREATE TYPE transactionstatus AS ENUM ('created', 'pending', 'completed', 'failed');
CREATE TYPE vaulttype AS ENUM ('jetton', 'ton');
CREATE TYPE ordertype AS ENUM ('jetton_to_jetton', 'jetton_to_ton', 'ton_to_jetton');

-- Create users table
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    telegram_id INTEGER NOT NULL UNIQUE,
    username CHARACTER VARYING NOT NULL,
    first_name CHARACTER VARYING,
    last_name CHARACTER VARYING,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Create wallets table
CREATE TABLE wallets (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    raw_address CHARACTER VARYING(67) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT wallets_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Create vault_factories table
CREATE TABLE vault_factories (
    id SERIAL PRIMARY KEY,
    version DOUBLE PRECISION NOT NULL,
    raw_address CHARACTER VARYING(67) NOT NULL UNIQUE,
    owner_address CHARACTER VARYING(67) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Create vaults table
CREATE TABLE vaults (
    id SERIAL PRIMARY KEY,
    factory_id INTEGER NOT NULL,
    raw_address CHARACTER VARYING(67) UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    jetton_minter_address CHARACTER VARYING(67),
    _type vaulttype NOT NULL,
    jetton_wallet_code CHARACTER VARYING,
    CONSTRAINT vaults_factory_id_fkey FOREIGN KEY (factory_id) REFERENCES vault_factories(id) ON DELETE CASCADE
);

-- Create coins table
CREATE TABLE coins (
    id SERIAL PRIMARY KEY,
    id_coingecko CHARACTER VARYING NOT NULL UNIQUE,
    name CHARACTER VARYING NOT NULL,
    symbol CHARACTER VARYING NOT NULL,
    created_at_db TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ton_raw_address CHARACTER VARYING NOT NULL UNIQUE,
    hex_jetton_wallet_code CHARACTER VARYING,
    jetton_content CHARACTER VARYING,
    decimals INTEGER
);

-- Create orders table
CREATE TABLE orders (
    id SERIAL PRIMARY KEY,
    title CHARACTER VARYING(200) NOT NULL,
    user_id INTEGER,
    raw_address CHARACTER VARYING(67) UNIQUE,
    wallet_id INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    vault_id INTEGER NOT NULL,
    status orderstatus NOT NULL DEFAULT 'created',
    amount DOUBLE PRECISION,
    price_rate DOUBLE PRECISION,
    slippage DOUBLE PRECISION,
    _type ordertype,
    to_jetton_minter_address CHARACTER VARYING(67),
    deployed BOOLEAN DEFAULT FALSE,
    deployed_at TIMESTAMPTZ,
    CONSTRAINT orders_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT orders_wallet_id_fkey FOREIGN KEY (wallet_id) REFERENCES wallets(id) ON DELETE CASCADE,
    CONSTRAINT orders_vault_id_fkey FOREIGN KEY (vault_id) REFERENCES vaults(id) ON DELETE CASCADE
);

-- Create transactions table
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

-- Create blocks table
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

-- Create ton_connect_storage table
CREATE TABLE ton_connect_storage (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- Create indexes
CREATE INDEX idx_transactions_user_id ON transactions(user_id);
CREATE INDEX idx_transactions_trs_hash ON transactions(trs_hash);
CREATE INDEX idx_blocks_workchain_shard_seqno ON blocks(workchain, shard, seq_no);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Drop tables in reverse order (respecting foreign key dependencies)
DROP TABLE IF EXISTS ton_connect_storage;
DROP TABLE IF EXISTS blocks;
DROP TABLE IF EXISTS transactions;
DROP TABLE IF EXISTS orders;
DROP TABLE IF EXISTS coins;
DROP TABLE IF EXISTS vaults;
DROP TABLE IF EXISTS vault_factories;
DROP TABLE IF EXISTS wallets;
DROP TABLE IF EXISTS users;

-- Drop ENUM types
DROP TYPE IF EXISTS ordertype;
DROP TYPE IF EXISTS vaulttype;
DROP TYPE IF EXISTS transactionstatus;
DROP TYPE IF EXISTS transactionaction;
DROP TYPE IF EXISTS orderstatus;

-- +goose StatementEnd

