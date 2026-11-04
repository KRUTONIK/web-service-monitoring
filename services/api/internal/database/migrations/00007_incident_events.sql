-- +goose Up
CREATE TABLE incident_events (
    event_id TEXT PRIMARY KEY,
    service_id TEXT NOT NULL REFERENCES monitored_services(id),
    event_type TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX incident_events_service_time_idx ON incident_events (service_id, occurred_at DESC);

-- +goose Down
DROP TABLE incident_events;
