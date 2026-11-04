-- +goose Up
ALTER TABLE monitored_services
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

CREATE INDEX monitored_services_active_owner_idx
    ON monitored_services (owner_id)
    WHERE deleted_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS monitored_services_active_owner_idx;
ALTER TABLE monitored_services
    DROP COLUMN IF EXISTS deleted_at;
