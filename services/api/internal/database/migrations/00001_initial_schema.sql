-- +goose Up
CREATE TABLE IF NOT EXISTS monitored_services (
    id TEXT PRIMARY KEY,
    url TEXT NOT NULL,
    enabled BOOLEAN NOT NULL,
    version BIGINT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

-- +goose Down
SELECT 1;
