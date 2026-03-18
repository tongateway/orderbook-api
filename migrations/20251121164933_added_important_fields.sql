-- +goose Up
-- +goose StatementBegin
ALTER TABLE vault_factories ADD COLUMN vault_code CHARACTER VARYING NOT NULL DEFAULT 'temporarily_value';
ALTER TABLE vault_factories ADD COLUMN order_code CHARACTER VARYING NOT NULL DEFAULT 'temporarily_value';
ALTER TABLE vault_factories ADD COLUMN matcher_fee_collector_code CHARACTER VARYING NOT NULL DEFAULT 'temporarily_value';
ALTER TABLE vault_factories ADD COLUMN commision_rate DOUBLE PRECISION NOT NULL DEFAULT 0.0;
ALTER TABLE vault_factories ADD COLUMN matcher_commision_rate DOUBLE PRECISION NOT NULL DEFAULT 0.0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE vault_factories DROP COLUMN vault_code;
ALTER TABLE vault_factories DROP COLUMN order_code;
ALTER TABLE vault_factories DROP COLUMN matcher_fee_collector_code;
ALTER TABLE vault_factories DROP COLUMN commision_rate;
ALTER TABLE vault_factories DROP COLUMN matcher_commision_rate;
-- +goose StatementEnd
