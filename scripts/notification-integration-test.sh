#!/usr/bin/env bash
set -euo pipefail
PROJECT_NAME=notification-integration-test
COMPOSE=(docker compose --project-name "$PROJECT_NAME")
cleanup() { "${COMPOSE[@]}" down -v >/dev/null 2>&1 || true; }
trap cleanup EXIT
wait_for() { for _ in $(seq 1 60); do "$@" && return; sleep 1; done; echo "timeout: $*" >&2; exit 1; }
"${COMPOSE[@]}" up --build -d postgres rabbitmq influxdb metrics api notification mailpit
wait_for curl -fsS http://localhost:8080/health
event='{"id":"notification-integration-1","type":"incident.started","service_id":"notification-service","service_name":"Интеграционный сервис","service_url":"https://example.test","recipient":"owner@example.test","occurred_at":"2026-11-05T12:00:00Z","status_code":503,"error":"integration failure"}'
body=$(node -e 'console.log(JSON.stringify({properties:{delivery_mode:2},routing_key:"notifications.events",payload:process.argv[1],payload_encoding:"string"}))' "$event")
curl -fsS -u monitoring:monitoring -H 'content-type: application/json' --data "$body" http://localhost:15672/api/exchanges/%2F/amq.default/publish | grep -q '"routed":true'
wait_for bash -c "curl -fsS http://localhost:8025/api/v1/messages | grep -q 'Недоступен сервис'"
"${COMPOSE[@]}" exec -T postgres psql -At -U monitoring -d monitoring -c "SELECT status FROM notification_deliveries WHERE event_id='notification-integration-1'" | grep -qx sent
echo 'Notification integration test passed.'
