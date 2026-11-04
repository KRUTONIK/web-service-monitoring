#!/usr/bin/env bash
set -euo pipefail

# Verifies upgrade safety for a durable incidents.events queue created by an
# earlier version without queue arguments.
PROJECT_NAME="incident-queue-compatibility-test"
COMPOSE=(docker compose -f compose.yml -f compose.test.yml --project-name "$PROJECT_NAME")
RABBIT_URL=http://localhost:15672

cleanup() { "${COMPOSE[@]}" down -v >/dev/null 2>&1 || true; }
trap cleanup EXIT

wait_for() {
  local description="$1"; shift
  for _ in $(seq 1 60); do
    if "$@"; then return 0; fi
    sleep 1
  done
  echo "timeout: $description" >&2
  exit 1
}

api_ready() { curl -fsS http://localhost:8080/health >/dev/null; }
queue_messages() {
  curl -fsS -u monitoring:monitoring "$RABBIT_URL/api/queues/%2F/$1" |
    node -e "let b='';process.stdin.on('data',d=>b+=d).on('end',()=>console.log(JSON.parse(b).messages ?? 0))"
}
dlq_has_message() { [ "$(queue_messages incidents.events.dlq)" -eq 1 ]; }

echo 'Starting RabbitMQ with legacy incidents queue...'
"${COMPOSE[@]}" up --build -d postgres rabbitmq influxdb metrics test-service >/dev/null
wait_for 'RabbitMQ management API' curl -fsS -u monitoring:monitoring "$RABBIT_URL/api/overview" >/dev/null
curl -fsS -u monitoring:monitoring -H 'Content-Type: application/json' -X PUT \
  --data '{"durable":true,"auto_delete":false,"arguments":{}}' \
  "$RABBIT_URL/api/queues/%2F/incidents.events" >/dev/null

echo 'Starting API against legacy queue...'
"${COMPOSE[@]}" up -d api >/dev/null
wait_for 'API startup with legacy queue' api_ready

request=$(node -e 'console.log(JSON.stringify({properties:{content_type:"application/json",delivery_mode:2},routing_key:"incidents.events",payload:"{\\\"id\\\":\\\"invalid\\\",\\\"service_id\\\":\\\"invalid\\\",\\\"type\\\":\\\"invalid\\\"}",payload_encoding:"string"}))')
curl -fsS -u monitoring:monitoring -H 'Content-Type: application/json' --data "$request" "$RABBIT_URL/api/exchanges/%2F/amq.default/publish" >/dev/null
wait_for 'invalid legacy-queue message in DLQ' dlq_has_message
echo 'Legacy incidents.events queue compatibility and DLQ passed.'
