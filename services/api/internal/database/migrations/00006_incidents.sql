-- +goose Up
CREATE TABLE incidents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id TEXT NOT NULL UNIQUE,
    service_id TEXT NOT NULL REFERENCES monitored_services(id),
    started_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ
);

CREATE INDEX incidents_service_started_idx ON incidents (service_id, started_at DESC);
CREATE UNIQUE INDEX incidents_one_open_per_service_idx ON incidents (service_id) WHERE ended_at IS NULL;

-- +goose Down
DROP TABLE incidents;
