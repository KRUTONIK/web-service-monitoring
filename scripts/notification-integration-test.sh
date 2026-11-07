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
  curl -sS -u monitoring:monitoring "${RABBIT_URL}/api/queues/%2F/notifications.events.retry" >&2 || true
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

mail_subject_count_is() {
  local subject=$1 expected=$2
  local messages
  messages=$(curl -fsS "${MAILPIT_URL}/api/v1/messages") || return 1
  node -e 'const messages = JSON.parse(process.argv[1]).messages || []; process.exit(messages.filter(message => message.Subject === process.argv[2]).length === Number(process.argv[3]) ? 0 : 1)' "$messages" "$subject" "$expected"
}

mail_recipient_count_is() {
  local recipient=$1 expected=$2 messages
  messages=$(curl -fsS "${MAILPIT_URL}/api/v1/messages") || return 1
  node -e 'const messages = JSON.parse(process.argv[1]).messages || []; process.exit(messages.filter(message => (message.To || []).some(item => item.Address === process.argv[2])).length === Number(process.argv[3]) ? 0 : 1)' "$messages" "$recipient" "$expected"
}

recovery_duration_is() {
  local expected=$1 messages message_id body
  messages=$(curl -fsS "${MAILPIT_URL}/api/v1/messages") || return 1
  message_id=$(node -e 'const messages = JSON.parse(process.argv[1]).messages || []; const found = messages.find(message => message.Subject === process.argv[2]); if (!found) process.exit(1); process.stdout.write(found.ID)' "$messages" "$recovery_subject") || return 1
  body=$(curl -fsS "${MAILPIT_URL}/view/${message_id}.txt") || return 1
  if [[ "$body" != *"Продолжительность: ${expected}"* ]]; then
    echo "Recovery email ${message_id} has an unexpected duration (expected ${expected}):" >&2
    echo "$body" >&2
    return 1
  fi
}

delivery_is_sent() {
  local event_id=$1
  "${COMPOSE[@]}" exec -T postgres psql -At -U monitoring -d monitoring \
    -c "SELECT status FROM notification_deliveries WHERE event_id='${event_id}'" | grep -qx sent
}

delivery_is_failed() {
  local event_id=$1
  "${COMPOSE[@]}" exec -T postgres psql -At -U monitoring -d monitoring -c "SELECT status FROM notification_deliveries WHERE event_id='${event_id}'" | grep -qx failed
}

queue_has_message() {
  local name=$1 queue
  queue=$(curl -fsS -u monitoring:monitoring "${RABBIT_URL}/api/queues/%2F/${name}") || return 1
  node -e 'process.exit(Number(JSON.parse(process.argv[1]).messages) > 0 ? 0 : 1)' "$queue"
}

mailpit_smtp_ready() {
  "${COMPOSE[@]}" exec -T notification sh -c 'nc -z mailpit 1025'
}

observe_dlq_is_unchanged() {
  local event_id=$1
  for _ in $(seq 1 8); do
    mail_recipient_count_is dlq@example.test 0 || fail "DLQ event ${event_id} was unexpectedly delivered"
    queue_has_message notifications.events.dlq || fail "DLQ event ${event_id} disappeared from notifications.events.dlq"
    sleep 1
  done
  delivery_is_failed "$event_id" || fail "DLQ delivery ${event_id} no longer has failed status"
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

seed_disabled_notification_service() {
  "${COMPOSE[@]}" exec -T postgres psql -v ON_ERROR_STOP=1 -U monitoring -d monitoring <<'SQL'
INSERT INTO monitored_services (id, url, enabled, version, updated_at, owner_id, name, notifications_enabled)
VALUES ('notifications-disabled-service', 'https://disabled.example.test', TRUE, 1, NOW(), '00000000-0000-0000-0000-000000000001', 'Сервис без уведомлений', FALSE);
SQL
}

incident_is_recorded() {
  "${COMPOSE[@]}" exec -T postgres psql -At -U monitoring -d monitoring \
    -c "SELECT COUNT(*) FROM incidents WHERE event_id='notification-integration-1'" | grep -qx 1
}

count_is() {
  local query=$1 expected=$2
  "${COMPOSE[@]}" exec -T postgres psql -At -U monitoring -d monitoring \
    -c "$query" | grep -qx "$expected"
}

publish_incident() {
  local event_id=$1 type=$2 service_id=$3 occurred_at=$4 status_code=$5
  local event body publish_response
  event=$(node -e 'console.log(JSON.stringify({id:process.argv[1],type:process.argv[2],service_id:process.argv[3],occurred_at:process.argv[4],status_code:Number(process.argv[5]),response_time_ms:125,error:"integration failure"}))' "$event_id" "$type" "$service_id" "$occurred_at" "$status_code")
  body=$(node -e 'console.log(JSON.stringify({properties:{delivery_mode:2},routing_key:"incidents.events",payload:process.argv[1],payload_encoding:"string"}))' "$event")
  publish_response=$(curl -fsS -u monitoring:monitoring -H 'content-type: application/json' --data "$body" "${RABBIT_URL}/api/exchanges/%2F/amq.default/publish") || fail "RabbitMQ rejected ${event_id}"
  node -e 'const response = JSON.parse(process.argv[1]); process.exit(response.routed === true ? 0 : 1)' "$publish_response" || fail "${event_id} was not routed: ${publish_response}"
}

publish_notification() {
  local event_id=$1 recipient=$2 event body response
  event=$(node -e 'console.log(JSON.stringify({id:process.argv[1],type:"incident.started",service_id:"notification-service",service_name:"SMTP retry fixture",recipient:process.argv[2],occurred_at:new Date().toISOString(),status_code:503,response_time_ms:125,error:"SMTP integration failure"}))' "$event_id" "$recipient")
  body=$(node -e 'console.log(JSON.stringify({properties:{content_type:"application/json",delivery_mode:2},routing_key:"notifications.events",payload:process.argv[1],payload_encoding:"string"}))' "$event")
  response=$(curl -sS -u monitoring:monitoring -H 'content-type: application/json' --data "$body" "${RABBIT_URL}/api/exchanges/%2F/amq.default/publish") || fail "RabbitMQ rejected ${event_id}"
  node -e 'process.exit(JSON.parse(process.argv[1]).routed === true ? 0 : 1)' "$response" || fail "${event_id} was not routed: ${response}"
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
seed_disabled_notification_service

failure_subject='[Мониторинг] Недоступен сервис «Интеграционный сервис»'
recovery_subject='[Мониторинг] Восстановлен сервис «Интеграционный сервис»'

# A. First failure: incident, outbox, email and delivery status are created.
publish_incident notification-start incident.started notification-service 2026-11-05T12:00:00Z 503
wait_for 'opened incident' count_is "SELECT COUNT(*) FROM incidents WHERE service_id='notification-service' AND ended_at IS NULL" 1
wait_for 'published failure outbox' count_is "SELECT COUNT(*) FROM notification_outbox WHERE service_id='notification-service' AND published_at IS NOT NULL" 1
wait_for 'one failure email' mail_subject_count_is "$failure_subject" 1
wait_for 'sent failure delivery' delivery_is_sent notification-start

# B and E. A repeated transition and replayed event never make a new email/outbox.
publish_incident notification-repeat incident.started notification-service 2026-11-05T12:01:00Z 503
wait_for 'repeat event recorded' count_is "SELECT COUNT(*) FROM incident_events WHERE service_id='notification-service'" 2
wait_for 'no duplicate failure notification' mail_subject_count_is "$failure_subject" 1
count_is "SELECT COUNT(*) FROM notification_outbox WHERE service_id='notification-service'" 1
publish_incident notification-start incident.started notification-service 2026-11-05T12:00:00Z 503
wait_for 'replayed event remains idempotent' mail_subject_count_is "$failure_subject" 1

# C. Recovery closes the incident and produces exactly one separate email.
publish_incident notification-resolved incident.resolved notification-service 2026-11-05T12:02:00Z 200
wait_for 'closed incident' count_is "SELECT COUNT(*) FROM incidents WHERE service_id='notification-service' AND ended_at IS NOT NULL" 1
wait_for 'published recovery outbox' count_is "SELECT COUNT(*) FROM notification_outbox WHERE service_id='notification-service' AND published_at IS NOT NULL" 2
wait_for 'one recovery email' mail_subject_count_is "$recovery_subject" 1
wait_for 'sent recovery delivery' delivery_is_sent notification-resolved
wait_for 'two-minute recovery duration' recovery_duration_is 2m0s

# D. Incidents remain operational when the service disables notifications.
publish_incident notification-disabled incident.started notifications-disabled-service 2026-11-05T12:03:00Z 503
wait_for 'disabled-service incident' count_is "SELECT COUNT(*) FROM incidents WHERE service_id='notifications-disabled-service'" 1
count_is "SELECT COUNT(*) FROM notification_outbox WHERE service_id='notifications-disabled-service'" 0
mail_subject_count_is "$failure_subject" 1
mail_subject_count_is "$recovery_subject" 1

# F. SMTP outage: the event is retried and is delivered once after Mailpit returns.
retry_subject='[Мониторинг] Недоступен сервис «SMTP retry fixture»'
"${COMPOSE[@]}" stop mailpit >/dev/null
publish_notification notification-retry retry@example.test
wait_for 'failed retry delivery' delivery_is_failed notification-retry
wait_for 'message in retry queue' queue_has_message notifications.events.retry
"${COMPOSE[@]}" start mailpit >/dev/null
wait_for 'Mailpit after retry' curl -fsS "${MAILPIT_URL}/api/v1/messages"
wait_for 'sent retry delivery' delivery_is_sent notification-retry
wait_for 'one recovered retry email' mail_recipient_count_is retry@example.test 1

# G. A persistent SMTP outage exhausts retries and routes the event to DLQ.
"${COMPOSE[@]}" stop mailpit >/dev/null
publish_notification notification-dlq dlq@example.test
wait_for 'failed DLQ delivery' delivery_is_failed notification-dlq
wait_for 'message in notification DLQ' queue_has_message notifications.events.dlq
"${COMPOSE[@]}" start mailpit >/dev/null
wait_for 'Mailpit after DLQ' curl -fsS "${MAILPIT_URL}/api/v1/messages"
wait_for 'Mailpit SMTP after DLQ' mailpit_smtp_ready
observe_dlq_is_unchanged notification-dlq
echo 'Notification integration test passed.'
