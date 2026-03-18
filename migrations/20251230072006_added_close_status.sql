-- +goose Up
-- +goose StatementBegin

-- Add 'closed' status to orderstatus enum
ALTER TYPE orderstatus ADD VALUE IF NOT EXISTS 'closed';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Note: PostgreSQL does not support removing enum values directly
-- If rollback is needed, you would need to:
-- 1. Create a new enum without 'closed'
-- 2. Update all rows to use the new enum
-- 3. Drop the old enum and rename the new one
-- This is a complex operation and should be done manually if needed

-- +goose StatementEnd
