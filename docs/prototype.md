# Monitoring Core

## Purpose

The monitoring core validates the end-to-end scenario and the selected
technology stack for periodic checks of configured web services.

The scenario is:

1. API Service stores one service configuration in PostgreSQL.
2. On startup, Scheduler Service requests the configuration snapshot from API
   Service through gRPC.
3. Scheduler Service sends check jobs through RabbitMQ at the configured
   interval.
4. Checker Service sends an HTTP request to the configured URL, measures the
   response time and determines availability.
5. Checker Service writes the result to InfluxDB.
6. Web UI requests the latest result from API Service over REST.
7. API Service requests the result from Metrics Service through gRPC.
8. Metrics Service reads the result from InfluxDB.
9. API Service returns the result to Web UI.

```text
Web UI --REST--> API Service --SQL--> PostgreSQL
                    |
                    +--gRPC--> Metrics Service --read--> InfluxDB
                    |
                    +--events--> RabbitMQ --> Scheduler Service --> RabbitMQ --> Checker workers

Checker Service --write-------------> InfluxDB
```

API Service does not access InfluxDB directly. Checker Service does not access
PostgreSQL or read monitoring results. Metrics Service has read-only
responsibility for monitoring data. Notification Service remains outside this
prototype scenario.

## Implemented behavior

Checker Service records:

- service URL;
- UTC check time;
- availability;
- HTTP status code;
- response time in milliseconds;
- transport error, when present.

HTTP responses from `200` through `299` are treated as available. Network
errors, timeouts and other response codes are treated as unavailable.

Scheduler Service receives a configuration snapshot first and then consumes
durable configuration-update messages. It ignores stale versions and schedules
enabled services at `SCHEDULER_INTERVAL`. Jobs are sent to a durable shared
queue; several Checker instances can consume it as competing consumers.
Checker acknowledges a job only after the result has been stored. The job queue
is bounded by length and lifetime to prevent stale tasks from accumulating.

## Reliable job delivery

RabbitMQ provides at-least-once delivery, therefore a Checker can receive the
same job more than once. Scheduler creates a stable `job_id` from the service,
URL and scheduled check time. Checker keeps that identifier and writes it as an
InfluxDB tag with the scheduled timestamp. A repeated delivery overwrites the
same InfluxDB point instead of creating another measurement.

```text
Scheduler --publisher confirm--> checker.jobs --> Checker
                                        |          |
                              reject-publish       +-- success: write InfluxDB, ACK
                              when full            |
                                                   +-- temporary failure
                                                        |
                                                        v
                                             checker.retry exchange
                                                        |
                                                        v
                                         checker.jobs.retry (delay)
                                                        |
                                                        v
                                                   checker.jobs

malformed job or exhausted attempts -----------------> checker.jobs.dlq
```

`checker.jobs` is durable, keeps at most 1000 tasks and uses
`reject-publish`, so queue overflow is returned to Scheduler rather than
silently dropping an older task. Scheduler waits for a RabbitMQ publisher
confirm and logs an unrecoverable publication failure with the service and job
identifiers.

Checker classifies malformed jobs as permanent failures. Other processing
errors are temporary: they are published to the delayed retry queue with the
attempt number and error description in headers. After three attempts, Checker
publishes the unchanged job body to `checker.jobs.dlq`, adds final-error
metadata and acknowledges the source delivery. The retry delay is configured by
`CHECKER_JOB_RETRY_DELAY`; the main queue TTL is configured separately with
`CHECKER_JOB_TTL` and is never shorter than the full retry window.

Each configuration has a monotonically increasing `version`. Scheduler Service
first replaces its local state with the startup snapshot and then consumes
queued updates. An update whose version is not greater than the snapshot or
current local version is acknowledged and ignored. This prevents an older
queued message from rolling back a newer startup snapshot.

Synchronous internal requests use gRPC:

- Scheduler Service calls `ConfigurationService.GetSnapshot` on API Service;
- API Service calls `MetricsService.GetLatestCheck` on Metrics Service.

RabbitMQ is used for asynchronous configuration-update events and check jobs.
It is not used as an RPC transport.

API Service provides:

| Method | Path | Result |
|---|---|---|
| `GET` | `/health` | API process readiness |
| `GET` | `/api/checks/latest` | Latest result stored during the previous 24 hours |

Web UI shows the service URL, current availability, HTTP status code, response
time and check time. The user can request the latest stored result again with
the refresh button.

## Configuration

| Variable | Service | Default |
|---|---|---|
| `API_LISTEN_ADDRESS` | API | `:8080` |
| `API_GRPC_LISTEN_ADDRESS` | API | `:9091` |
| `METRICS_GRPC_ADDRESS` | API | `localhost:9092` |
| `POSTGRES_DSN` | API | local monitoring database |
| `PROTOTYPE_SERVICE_URL` | API | `http://test-service` in Compose |
| `API_GRPC_ADDRESS` | Scheduler | `localhost:9091` |
| `SCHEDULER_INTERVAL` | Scheduler | `1m` |
| `CHECKER_JOB_RETRY_DELAY` | Scheduler | `5s` |
| `CHECKER_JOB_TTL` | Scheduler | `1m` |
| `RABBITMQ_URL` | API, Scheduler, Checker | local monitoring broker |
| `METRICS_GRPC_LISTEN_ADDRESS` | Metrics | `:9092` |
| `INFLUXDB_URL` | Checker, Metrics | `http://localhost:8086` |
| `INFLUXDB_ORG` | Checker, Metrics | `monitoring` |
| `INFLUXDB_BUCKET` | Checker, Metrics | `monitoring` |
| `INFLUXDB_TOKEN` | Checker, Metrics | `monitoring-test-token` |
| `VITE_API_URL` | Web build | `http://localhost:8080` |

Compose replaces database, broker and gRPC addresses with their internal
service names. Checker and Metrics Service use the common development token
created by InfluxDB initialization. Separate write-only and read-only tokens
are intentionally left for a later security-hardening stage; they are not
required to demonstrate the prototype's vertical scenario.

## Running with Docker

Build and start all components:

```bash
docker compose up --build
```

Scheduler and Checker Service should remain in the `running` state so checks
can be published and consumed automatically.

Several Checker workers can share the same job queue:

```bash
docker compose up --scale checker=2
```

Inspect the result:

```bash
curl http://localhost:8080/api/checks/latest
```

Stop all services and preserve InfluxDB data:

```bash
docker compose down
```

Remove containers and stored prototype data:

```bash
docker compose down -v
```

## Automated verification

Unit tests cover the HTTP checker, versioned configuration state, gRPC service
adapters, environment configuration, InfluxDB write and query clients, CSV
response parsing and API handlers.

The basic prototype integration test runs through Docker Compose:

```bash
bash scripts/prototype-test.sh
```

The test builds the images, waits for API, Checker and Metrics Service, polls
until a monitoring result is available, validates the returned JSON and checks
that the Web UI is available.

The monitoring-core integration test additionally verifies periodic scheduling,
two competing Checker workers, redelivery after a worker failure, retry and
DLQ processing while InfluxDB is unavailable, and versioned configuration
updates:

```bash
bash scripts/monitoring-core-test.sh
```

CI runs both integration scenarios after the infrastructure checks.

## Prototype limitations

- only one URL is seeded through the environment;
- one global monitoring interval is used; per-service intervals are deferred;
- only the latest result is available through the API;
- there is no service-management interface;
- incidents, analytics and notifications are not implemented;
- authentication and production secret management are outside the current
  scope.
