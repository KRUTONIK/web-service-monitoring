-- +goose Up
ALTER TABLE monitored_services
    ADD COLUMN interval_seconds INTEGER NOT NULL DEFAULT 10,
    ADD COLUMN timeout_seconds INTEGER NOT NULL DEFAULT 5;

-- +goose Down
ALTER TABLE monitored_services
    DROP COLUMN timeout_seconds,
    DROP COLUMN interval_seconds;
