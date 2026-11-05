-- +goose Up
ALTER TABLE monitored_services ADD COLUMN notifications_enabled BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE notification_outbox (
    event_id TEXT PRIMARY KEY,
    event_type TEXT NOT NULL,
    service_id TEXT NOT NULL REFERENCES monitored_services(id),
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ,
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT
);
CREATE INDEX notification_outbox_unpublished_idx ON notification_outbox (created_at) WHERE published_at IS NULL;

CREATE TABLE notification_deliveries (
    event_id TEXT PRIMARY KEY,
    status TEXT NOT NULL,
    sent_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_error TEXT
);

-- +goose Down
DROP TABLE notification_deliveries;
DROP TABLE notification_outbox;
ALTER TABLE monitored_services DROP COLUMN notifications_enabled;
