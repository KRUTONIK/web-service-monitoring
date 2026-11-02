-- +goose Up
ALTER TABLE monitored_services
    ADD COLUMN IF NOT EXISTS owner_id UUID REFERENCES users(id) ON DELETE CASCADE,
    ADD COLUMN IF NOT EXISTS name TEXT,
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

ALTER TABLE monitored_services
    ALTER COLUMN id SET DEFAULT gen_random_uuid()::TEXT;

UPDATE monitored_services
SET name = url
WHERE name IS NULL;

CREATE INDEX monitored_services_owner_id_idx ON monitored_services (owner_id);

-- +goose Down
DROP INDEX IF EXISTS monitored_services_owner_id_idx;
ALTER TABLE monitored_services
    DROP COLUMN IF EXISTS created_at,
    DROP COLUMN IF EXISTS name,
    DROP COLUMN IF EXISTS owner_id;
