# Web Service Monitoring

Microservice-based system for monitoring web-service availability and performance.

## Current status

The project is at the prototype stage (`v0.2.0` in development). The implemented
vertical scenario checks one configured web service, stores the result in
InfluxDB, exposes the latest result through the API and displays it in the Web
UI.

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

The Checker Service performs one check and exits with code `0`. This is expected
prototype behavior. Run it again without rebuilding the other services:

```bash
docker compose run --rm checker
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
- Checker Service — Go
- Notification Service — Go
- PostgreSQL — configuration and incidents
- InfluxDB — monitoring results
- RabbitMQ — communication between microservices

## Project structure

- `services/api` — API Service
- `services/checker` — Checker Service
- `services/notification` — Notification Service
- `web` — React Web UI
- `deploy` — deployment configuration
- `docs` — project documentation
- `scripts` — utility scripts
- `metrics` — quality metrics

## Versions

- `v0.1.0` — infrastructure
- `v0.2.0` — prototype
- `v0.3.0` — monitoring core
- `v0.4.0` — management and state tracking
- `v0.5.0` — analytics and notifications
- `v1.0.0` — final release
