-- +goose Up
-- +goose StatementBegin
ALTER TABLE api_keys 
ADD COLUMN rate_limit INTEGER NOT NULL DEFAULT 1;

COMMENT ON COLUMN api_keys.rate_limit IS 'Rate limit in requests per second for this API key (default: 1)';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE api_keys DROP COLUMN IF EXISTS rate_limit;
-- +goose StatementEnd

