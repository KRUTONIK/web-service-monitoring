# Changelog

## v0.2.0 — Prototype

### Added

- minimal end-to-end web-service monitoring scenario
- service configuration storage in PostgreSQL
- versioned Protocol Buffers contracts for internal gRPC APIs
- startup configuration transfer from API Service to Checker Service over gRPC
- asynchronous configuration updates through RabbitMQ
- HTTP availability checks and result storage in InfluxDB
- Metrics Service for reading monitoring results from InfluxDB
- latest monitoring result REST API
- minimal Web UI for displaying service status
- Docker Compose configuration and integration test for the complete prototype

### Changed

- separated metric reading from Checker Service into Metrics Service
- aligned service communication with the target project architecture

## v0.1.1 — Infrastructure Integration Tests

### Added

- Docker Compose infrastructure for PostgreSQL, RabbitMQ and InfluxDB
- infrastructure health checks
- automated infrastructure integration tests
- integration tests as a required CI stage

## v0.1.0 — Infrastructure

Initial infrastructure release.

### Added

- base microservice repository structure
- API, Checker and Notification Go service skeletons
- React Web UI skeleton
- Git Flow branching strategy
- Conventional Commits validation
- Go formatting and static analysis
- React ESLint checks
- automated unit tests and coverage reports
- automatic application build
- CI build and coverage artifacts
- automated quality metrics
- protected main and develop branches
