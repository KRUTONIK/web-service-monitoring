# Web Service Monitoring

Microservice-based system for monitoring web-service availability and performance.

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