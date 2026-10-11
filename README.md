# Web Service Monitoring

Microservice-based system for monitoring web-service availability and performance.

## Current status

The project is at the monitoring-core stage (`v0.3.1` in development). It
periodically checks configured web services, stores idempotent results in
InfluxDB and exposes the latest result through the API and Web UI. Failed jobs
are retried with a delay and then retained in a RabbitMQ dead-letter queue.

## Quick start

Requirements:

- Docker with Docker Compose

Start the complete prototype from the repository root:

```bash
docker compose up --build
```

Open:

- Web UI: http://localhost:5173
- API health check: http://localhost:8080/health
- latest check result: http://localhost:8080/api/checks/latest

Scheduler Service creates checks automatically. Checker Service remains running
as a worker and receives jobs through RabbitMQ. A successful job is
acknowledged only after its result is saved.

Additional Checker workers can be started when needed:

```bash
docker compose up --scale checker=2
```

Stop the prototype while preserving stored data:

```bash
docker compose down
```

See [Prototype documentation](docs/prototype.md) for configuration, verification
and current limitations.

## Architecture

The system consists of:

- Web UI — React
- API Service — Go
- Scheduler Service — Go
- Checker Service — Go
- Metrics Service — Go
- Notification Service — Go
- PostgreSQL — configuration and incidents
- InfluxDB — monitoring results
- RabbitMQ — asynchronous events between microservices
- gRPC — synchronous internal requests

API Service owns configuration in PostgreSQL. Scheduler Service requests its
startup configuration from API over gRPC, receives later configuration updates
through RabbitMQ and sends check jobs to Checker workers. Checker Service writes
monitoring results to InfluxDB, while Metrics Service provides read access to
them. API reads metrics from Metrics Service over gRPC.

Check jobs are processed with at-least-once delivery. A stable `job_id` makes
InfluxDB writes idempotent. Temporary failures travel through a delayed retry
queue; malformed jobs and jobs that exhaust retries are moved to the DLQ. The
Scheduler uses publisher confirms, so an overflow or routing failure is logged
instead of silently losing a job.
API Service has no direct access to InfluxDB. Docker initialization creates a
single InfluxDB token for the prototype. Separate read and write credentials
are deferred until the production-hardening stage.

## Project structure

- `services/api` — API Service
- `services/scheduler` — Scheduler Service
- `services/checker` — Checker Service
- `services/metrics` — Metrics Service
- `services/notification` — Notification Service
- `contracts` — versioned Protocol Buffers contracts and generated Go code
- `web` — React Web UI
- `deploy` — deployment configuration
- `docs` — project documentation
- `scripts` — utility scripts
- `metrics` — quality metrics

## Versions

- `v0.1.0` — infrastructure
- `v0.2.0` — prototype
- `v0.3.0` — monitoring core
- `v0.3.1` — monitoring core reliability fixes
- `v0.4.0` — management and state tracking
- `v0.5.0` — analytics and notifications
- `v1.0.0` — final release
