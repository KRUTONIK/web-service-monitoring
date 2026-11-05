#!/usr/bin/env bash

set -euo pipefail

PROJECT_NAME=notification-integration-test
COMPOSE=(docker compose --project-name "$PROJECT_NAME")
RABBIT_URL='http://localhost:15672'
MAILPIT_URL='http://localhost:8025'

cleanup() { "${COMPOSE[@]}" down -v >/dev/null 2>&1 || true; }

diagnose() {
  local status=$1
  echo "Notification integration test failed (exit ${status}). Diagnostics:" >&2
  "${COMPOSE[@]}" ps >&2 || true
  echo 'RabbitMQ notification queue:' >&2
  curl -sS -u monitoring:monitoring "${RABBIT_URL}/api/queues/%2F/notifications.events" >&2 || true
  echo >&2
  echo 'RabbitMQ retry queue:' >&2
  curl -sS -u monitoring:monitoring "${RABBIT_URL}/api/queues/%2F/notifications.retry" >&2 || true
  echo >&2
  echo 'Mailpit messages:' >&2
  curl -sS "${MAILPIT_URL}/api/v1/messages" >&2 || true
  echo >&2
  "${COMPOSE[@]}" logs --tail 100 rabbitmq notification api mailpit postgres >&2 || true
}

finish() {
  local status=$?
  if [ "$status" -ne 0 ]; then
    diagnose "$status"
  fi
  cleanup
  exit "$status"
}
trap finish EXIT

fail() {
  echo "$1" >&2
  return 1
}

wait_for() {
  local description=$1
  shift
  for _ in $(seq 1 60); do
    if "$@" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  fail "timeout waiting for ${description}"
}

queue_ready() {
  local name=$1
  local queue
  queue=$(curl -fsS -u monitoring:monitoring "${RABBIT_URL}/api/queues/%2F/${name}") || return 1
  node -e 'const queue = JSON.parse(process.argv[1]); process.exit(queue.durable && Number(queue.consumers) >= 1 ? 0 : 1)' "$queue"
}

mailpit_has_notification() {
  curl -fsS "${MAILPIT_URL}/api/v1/messages" | grep -q 'Недоступен сервис'
}

delivery_is_sent() {
  "${COMPOSE[@]}" exec -T postgres psql -At -U monitoring -d monitoring \
    -c "SELECT status FROM notification_deliveries WHERE event_id='notification-integration-1'" | grep -qx sent
}

seed_notification_service() {
  "${COMPOSE[@]}" exec -T postgres psql -v ON_ERROR_STOP=1 -U monitoring -d monitoring <<'SQL'
INSERT INTO users (id, email, password_hash)
VALUES ('00000000-0000-0000-0000-000000000001', 'owner@example.test', 'integration-test')
ON CONFLICT (id) DO UPDATE SET email = EXCLUDED.email;

INSERT INTO monitored_services (id, url, enabled, version, updated_at, owner_id, name, notifications_enabled)
VALUES ('notification-service', 'https://example.test', TRUE, 1, NOW(), '00000000-0000-0000-0000-000000000001', 'Интеграционный сервис', TRUE)
ON CONFLICT (id) DO UPDATE SET
  url = EXCLUDED.url,
  enabled = EXCLUDED.enabled,
  owner_id = EXCLUDED.owner_id,
  name = EXCLUDED.name,
  notifications_enabled = EXCLUDED.notifications_enabled,
  deleted_at = NULL;
SQL
}

incident_is_recorded() {
  "${COMPOSE[@]}" exec -T postgres psql -At -U monitoring -d monitoring \
    -c "SELECT COUNT(*) FROM incidents WHERE event_id='notification-integration-1'" | grep -qx 1
}

outbox_is_published() {
  "${COMPOSE[@]}" exec -T postgres psql -At -U monitoring -d monitoring \
    -c "SELECT COUNT(*) FROM notification_outbox WHERE event_id='notification-integration-1' AND published_at IS NOT NULL" | grep -qx 1
}

"${COMPOSE[@]}" up --build -d postgres rabbitmq influxdb metrics api notification mailpit
wait_for 'API health endpoint' curl -fsS http://localhost:8080/health
wait_for 'Mailpit HTTP API' curl -fsS "${MAILPIT_URL}/api/v1/messages"
# Queue declaration happens in asynchronous consumer startup. Waiting for
# durable queues and consumers avoids publishing to an unroutable default
# exchange while RabbitMQ, API, or Notification Service is still starting.
wait_for 'incident queue and API consumer' queue_ready incidents.events
wait_for 'notification queue and consumer' queue_ready notifications.events
seed_notification_service

event='{"id":"notification-integration-1","type":"incident.started","service_id":"notification-service","occurred_at":"2026-11-05T12:00:00Z","status_code":503,"error":"integration failure"}'
body=$(node -e 'console.log(JSON.stringify({properties:{delivery_mode:2},routing_key:"incidents.events",payload:process.argv[1],payload_encoding:"string"}))' "$event")
publish_response=$(curl -fsS -u monitoring:monitoring -H 'content-type: application/json' --data "$body" "${RABBIT_URL}/api/exchanges/%2F/amq.default/publish") || fail 'RabbitMQ Management API rejected incident publication'
node -e 'const response = JSON.parse(process.argv[1]); if (response.routed !== true) { console.error(JSON.stringify(response)); process.exit(1) }' "$publish_response" || fail "incident was not routed; RabbitMQ response: ${publish_response}"

wait_for 'incident persisted by API' incident_is_recorded
wait_for 'outbox publication by API' outbox_is_published
wait_for 'notification email in Mailpit' mailpit_has_notification
wait_for 'sent notification delivery in PostgreSQL' delivery_is_sent
echo 'Notification integration test passed.'
