# Monitoring Prototype

## Purpose

The prototype validates the minimum end-to-end monitoring scenario and the
selected technology stack before implementation of the monitoring core.

The scenario is:

1. Checker Service sends an HTTP request to one configured URL.
2. Checker Service measures the response time and determines availability.
3. The result is stored in InfluxDB.
4. API Service reads the latest stored result.
5. Web UI displays the result to the user.

```text
Target web service
        |
        v
Checker Service ---> InfluxDB ---> API Service ---> Web UI
```

PostgreSQL, RabbitMQ and Notification Service remain in the project
infrastructure but are not used by this prototype scenario.

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

Checker Service is a one-shot process at this stage. It performs one check,
writes one result and exits. Periodic scheduling belongs to the monitoring-core
stage.

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
| `MONITOR_URL` | Checker | `https://example.com` |
| `API_LISTEN_ADDRESS` | API | `:8080` |
| `INFLUXDB_URL` | Checker, API | `http://localhost:8086` |
| `INFLUXDB_ORG` | Checker, API | `monitoring` |
| `INFLUXDB_BUCKET` | Checker, API | `monitoring` |
| `INFLUXDB_TOKEN` | Checker, API | `monitoring-test-token` |
| `VITE_API_URL` | Web build | `http://localhost:8080` |

The Compose configuration replaces the InfluxDB URL with the internal service
address `http://influxdb:8086`.

## Running with Docker

Build and start all components:

```bash
docker compose up --build
```

Checker Service should finish with status `Exited (0)`. The infrastructure, API
and Web UI continue running.

Inspect the result:

```bash
curl http://localhost:8080/api/checks/latest
```

Run another check:

```bash
docker compose run --rm checker
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

Unit tests cover the HTTP checker, environment configuration, InfluxDB write and
query clients, CSV response parsing and API handlers.

The full prototype integration test runs through Docker Compose:

```bash
bash scripts/prototype-test.sh
```

The test builds the images, waits for API readiness, verifies the Checker exit
code, validates the returned JSON and checks that the Web UI is available. CI
runs this scenario after the infrastructure integration test.

## Prototype limitations

- only one URL is configured through the environment;
- the check is executed once rather than periodically;
- only the latest result is available through the API;
- there is no service-management interface;
- PostgreSQL and RabbitMQ are not integrated into the scenario;
- incidents, analytics and notifications are not implemented;
- authentication and production secret management are outside the prototype
  scope.
