#!/usr/bin/env bash

set -euo pipefail

COMPOSE=(
    docker compose
    --project-name monitoring-core-test
    -f compose.yml
    -f compose.test.yml
)

cleanup() {
    "${COMPOSE[@]}" down -v
}

fail() {
    echo "$1" >&2
    "${COMPOSE[@]}" logs >&2 || true
    exit 1
}

service_id() {
    "${COMPOSE[@]}" ps -q "$1" | head -n 1
}

rabbit_queue_value() {
    local queue="$1"
    local field="$2"
    local response
    response=$(curl --fail --silent --user monitoring:monitoring "http://localhost:15672/api/queues/%2F/${queue}")
    node -e 'console.log(JSON.parse(process.argv[1])[process.argv[2]] ?? 0)' "$response" "$field"
}

influx_job_count() {
    local service_id="$1"
    local service_url="${2:-}"
    local url_filter=""
    if [ -n "$service_url" ]; then
        url_filter=" and r.service_url == \"${service_url}\""
    fi
    local query
    query="from(bucket: \"monitoring\")
  |> range(start: -10m)
  |> filter(fn: (r) => r._measurement == \"service_check\" and r.service_id == \"${service_id}\"${url_filter})
  |> keep(columns: [\"job_id\"])
  |> distinct(column: \"job_id\")"
    local response
    response=$(curl --fail --silent \
        --header "Authorization: Token monitoring-test-token" \
        --header "Accept: application/csv" \
        --header "Content-Type: application/vnd.flux" \
        --data-binary "$query" \
        "http://localhost:8086/api/v2/query?org=monitoring")
    node -e 'const ids = process.argv[1].match(/\b[a-f0-9]{64}\b/g) ?? []; console.log(new Set(ids).size)' "$response"
}

wait_for_api() {
    for _ in {1..60}; do
        local api_id
        api_id=$(service_id api)
        local status
        status=$(docker inspect --format='{{.State.Health.Status}}' "$api_id" 2>/dev/null || true)
        if [ "$status" = "healthy" ]; then
            return
        fi
        sleep 1
    done
    fail "API Service failed to become healthy"
}

wait_for_influxdb() {
    for _ in {1..60}; do
        if curl --fail --silent http://localhost:8086/health >/dev/null; then
            return
        fi
        sleep 1
    done
    fail "InfluxDB failed to become healthy"
}

wait_for_job_count() {
    local service_id="$1"
    local expected="$2"
    local service_url="${3:-}"
    for _ in {1..40}; do
        local count
        if ! count=$(influx_job_count "$service_id" "$service_url"); then
            sleep 1
            continue
        fi
        if [ "$count" -ge "$expected" ]; then
            return
        fi
        sleep 1
    done
    fail "Expected ${expected} jobs for ${service_id}"
}

wait_for_queue_value() {
    local queue="$1"
    local field="$2"
    local expected="$3"
    local attempts="${4:-40}"
    for ((attempt = 1; attempt <= attempts; attempt++)); do
        local value
        if ! value=$(rabbit_queue_value "$queue" "$field"); then
            sleep 1
            continue
        fi
        if [ "$value" -ge "$expected" ]; then
            return
        fi
        sleep 1
    done
    fail "Expected ${queue}.${field} to reach ${expected}"
}

wait_for_checker_log_count() {
    local pattern="$1"
    local expected="$2"
    for _ in {1..40}; do
        local count
        count=$("${COMPOSE[@]}" logs --no-log-prefix checker 2>&1 | grep --fixed-strings --count "$pattern" || true)
        if [ "$count" -ge "$expected" ]; then
            return
        fi
        sleep 1
    done
    fail "Expected ${expected} Checker log entries matching ${pattern}"
}

trap cleanup EXIT

echo "Starting monitoring-core integration environment..."
"${COMPOSE[@]}" up --build -d --scale checker=2 >/dev/null 2>&1
wait_for_api

echo "Checking periodic scheduling..."
wait_for_job_count prototype-service 2 http://test-service

echo "Checking multiple Checker workers..."
if [ "$("${COMPOSE[@]}" ps -q checker | wc -l | tr -d ' ')" != "2" ]; then
    fail "Expected two Checker containers"
fi
wait_for_queue_value checker.jobs consumers 2
"${COMPOSE[@]}" exec -T postgres psql -U monitoring -d monitoring -c "
    INSERT INTO monitored_services (id, url, enabled, version, updated_at)
    VALUES ('slow-service', 'http://slow-test-service:8080', TRUE, 1, NOW())
    ON CONFLICT (id) DO UPDATE SET url = EXCLUDED.url, enabled = EXCLUDED.enabled, version = EXCLUDED.version, updated_at = EXCLUDED.updated_at;
" >/dev/null
"${COMPOSE[@]}" restart api >/dev/null
wait_for_api
wait_for_job_count slow-service 1 http://slow-test-service:8080
for checker_id in $("${COMPOSE[@]}" ps -q checker); do
    if ! docker logs "$checker_id" 2>&1 | grep --quiet "checked service="; then
        fail "Checker ${checker_id} did not process a job"
    fi
done

echo "Checking redelivery after Checker failure..."
"${COMPOSE[@]}" stop scheduler >/dev/null
wait_for_queue_value checker.jobs messages_unacknowledged 1
"${COMPOSE[@]}" kill checker >/dev/null
"${COMPOSE[@]}" up -d --no-deps --scale checker=2 checker >/dev/null
wait_for_checker_log_count "checked service=slow-service" 1

echo "Checking DLQ after InfluxDB failure..."
"${COMPOSE[@]}" stop checker >/dev/null
for queue in checker.jobs checker.jobs.retry; do
    curl --fail --silent --request DELETE --user monitoring:monitoring \
        "http://localhost:15672/api/queues/%2F/${queue}/contents" >/dev/null
done
"${COMPOSE[@]}" up -d --no-deps --scale checker=2 checker >/dev/null
wait_for_queue_value checker.jobs consumers 2
"${COMPOSE[@]}" stop influxdb >/dev/null
dlq_payload=$(node -e '
process.stdout.write(JSON.stringify({
  job_id: "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
  service_id: "prototype-service",
  service_url: "http://test-service",
  scheduled_at: new Date().toISOString()
}));
')
dlq_request=$(node -e '
const payload = process.argv[1];
process.stdout.write(JSON.stringify({
  properties: {content_type: "application/json", delivery_mode: 2},
  routing_key: "checker.jobs",
  payload,
  payload_encoding: "string"
}));
' "$dlq_payload")
routed=$(curl --fail --silent --user monitoring:monitoring \
    --header "Content-Type: application/json" \
    --data "$dlq_request" \
    "http://localhost:15672/api/exchanges/%2F/amq.default/publish")
node -e 'if (!JSON.parse(process.argv[1]).routed) process.exit(1)' "$routed" || fail "DLQ test job was not routed"
wait_for_queue_value checker.jobs.dlq messages 1 80
"${COMPOSE[@]}" start influxdb >/dev/null
wait_for_influxdb
"${COMPOSE[@]}" start scheduler >/dev/null

echo "Checking versioned configuration updates..."
"${COMPOSE[@]}" exec -T postgres psql -U monitoring -d monitoring -c "
    UPDATE monitored_services
    SET url = 'http://updated-test-service', version = 2, updated_at = NOW()
    WHERE id = 'prototype-service';
" >/dev/null
"${COMPOSE[@]}" restart api >/dev/null
wait_for_api
wait_for_job_count prototype-service 1 http://updated-test-service
updated_before=$(influx_job_count prototype-service http://updated-test-service)
stale_payload='{"event":"service.updated","service":{"id":"prototype-service","url":"http://test-service","enabled":true,"version":1,"updated_at":"2026-10-11T00:00:00Z"}}'
rabbit_request=$(node -e '
const payload = process.argv[1];
process.stdout.write(JSON.stringify({
  properties: {content_type: "application/json", delivery_mode: 2},
  routing_key: "scheduler.config.updates",
  payload,
  payload_encoding: "string"
}));
' "$stale_payload")
routed=$(curl --fail --silent --user monitoring:monitoring \
    --header "Content-Type: application/json" \
    --data "$rabbit_request" \
    "http://localhost:15672/api/exchanges/%2F/amq.default/publish")
node -e 'if (!JSON.parse(process.argv[1]).routed) process.exit(1)' "$routed" || fail "Stale configuration message was not routed"
wait_for_job_count prototype-service "$((updated_before + 1))" http://updated-test-service

echo "Monitoring-core integration tests passed."
