# Monitoring Prototype

## Purpose

The prototype validates the minimum end-to-end monitoring scenario and the
selected technology stack before implementation of the monitoring core.

The scenario is:

1. API Service stores one service configuration in PostgreSQL.
2. On startup, Checker Service requests the configuration snapshot from API
   Service through gRPC.
3. Checker Service sends an HTTP request to the configured URL, measures the
   response time and determines availability.
4. Checker Service writes the result to InfluxDB.
5. Web UI requests the latest result from API Service over REST.
6. API Service requests the result from Metrics Service through gRPC.
7. Metrics Service reads the result from InfluxDB.
8. API Service returns the result to Web UI.

```text
Web UI --REST--> API Service --SQL--> PostgreSQL
                    |
                    +--gRPC--> Metrics Service --read--> InfluxDB
                    |
                    +--events--> RabbitMQ --> Checker Service

Checker Service --gRPC GetSnapshot--> API Service
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

Checker Service remains running after the initial check. It consumes durable
configuration-update messages and performs another check when it receives a
newer enabled configuration. Periodic scheduling belongs to the
monitoring-core stage.

Each configuration has a monotonically increasing `version`. Checker Service
first replaces its local state with the startup snapshot and then consumes
queued updates. An update whose version is not greater than the snapshot or
current local version is acknowledged and ignored. This prevents an older
queued message from rolling back a newer startup snapshot.

Synchronous internal requests use gRPC:

- Checker Service calls `ConfigurationService.GetSnapshot` on API Service;
- API Service calls `MetricsService.GetLatestCheck` on Metrics Service.

RabbitMQ is used only for asynchronous configuration-update events. It is not
used as an RPC transport.

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
| `PROTOTYPE_SERVICE_URL` | API | `https://example.com` |
| `API_GRPC_ADDRESS` | Checker | `localhost:9091` |
| `RABBITMQ_URL` | API, Checker | local monitoring broker |
| `METRICS_GRPC_LISTEN_ADDRESS` | Metrics | `:9092` |
| `INFLUXDB_URL` | Checker, Metrics | `http://localhost:8086` |
| `INFLUXDB_ORG` | Checker, Metrics | `monitoring` |
| `INFLUXDB_BUCKET` | Checker, Metrics | `monitoring` |
| `INFLUXDB_TOKEN` | Checker, Metrics | `monitoring-test-token` |
| `VITE_API_URL` | Web build | `http://localhost:8080` |

Compose replaces database, broker and gRPC addresses with their internal
service names. The prototype uses one InfluxDB token; separate write-only and
read-only tokens are deferred to deployment hardening.

## Running with Docker

Build and start all components:

```bash
docker compose up --build
```

Checker Service should remain in the `running` state so it can consume RabbitMQ
messages.

Inspect the result:

```bash
curl http://localhost:8080/api/checks/latest
```

Repeat the startup check:

```bash
docker compose restart checker
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

The full prototype integration test runs through Docker Compose:

```bash
bash scripts/prototype-test.sh
```

The test builds the images, waits for API, Checker and Metrics Service, polls
until a monitoring result is available, validates the returned JSON and checks
that the Web UI is available. CI runs this scenario after the infrastructure
integration test.

## Prototype limitations

- only one URL is seeded through the environment;
- the check is executed on startup or configuration update, not periodically;
- only the latest result is available through the API;
- there is no service-management interface;
- incidents, analytics and notifications are not implemented;
- authentication and production secret management are outside the prototype
  scope.
