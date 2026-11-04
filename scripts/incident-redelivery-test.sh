#!/usr/bin/env bash
set -euo pipefail

# Настоящий сценарий: InfluxDB принимает результат, а публикация инцидента
# временно невозможна. compose.test.yml задаёт 15-секундный TTL retry-очереди.
PROJECT_NAME="incident-redelivery-test"
COMPOSE=(docker compose -f compose.yml -f compose.test.yml --project-name "$PROJECT_NAME")
RABBIT_URL=http://localhost:15672
SERVICE_ID=incident-redelivery-service
STARTED_JOB=incident-redelivery-started
RESOLVED_JOB=incident-redelivery-resolved

fail() { echo "ERROR: $*" >&2; exit 1; }
cleanup() { "${COMPOSE[@]}" down -v >/dev/null 2>&1 || true; }
trap cleanup EXIT

wait_for() {
  local description="$1"; shift
  for _ in $(seq 1 60); do
    if "$@"; then return 0; fi
    sleep 1
  done
  fail "timeout: $description"
}

queue_field() {
  curl -fsS -u monitoring:monitoring "$RABBIT_URL/api/queues/%2F/$1" |
    node -e "let b='';process.stdin.on('data',d=>b+=d).on('end',()=>console.log(JSON.parse(b).$2 ?? 0))"
}

api_ready() { curl -fsS http://localhost:8080/health >/dev/null; }
checker_ready() { [ "$(queue_field checker.jobs consumers)" -ge 1 ]; }
retry_pending() { [ "$(queue_field checker.jobs.retry messages)" -eq 1 ]; }
retry_count() { [ "$(queue_field checker.jobs.retry messages)" -eq "$1" ]; }
retry_not_delivered() { [ "$(queue_field checker.jobs messages)" -eq 0 ] && [ "$(queue_field checker.jobs.retry messages)" -eq 1 ]; }
retry_drained() { [ "$(queue_field checker.jobs.retry messages)" -eq 0 ] && [ "$(queue_field checker.jobs messages_unacknowledged)" -eq 0 ]; }
incident_dlq_has_message() { [ "$(queue_field incidents.events.dlq messages)" -eq 1 ]; }
checker_logged_mandatory_return() { "${COMPOSE[@]}" logs checker 2>&1 | grep -q 'message was not routed'; }
influx_has() {
  curl -fsS -H 'Authorization: Token monitoring-test-token' -H 'Content-Type: application/vnd.flux' \
    --data "from(bucket: \"monitoring\") |> range(start: 0) |> filter(fn: (r) => r._measurement == \"service_check\" and r.job_id == \"$1\") |> limit(n: 1)" \
    'http://localhost:8086/api/v2/query?org=monitoring' | grep -q "$1"
}
event_count() { "${COMPOSE[@]}" exec -T postgres psql -At -U monitoring -d monitoring -c "SELECT count(*) FROM incident_events WHERE event_id='$1';"; }
incident_counts() { "${COMPOSE[@]}" exec -T postgres psql -At -U monitoring -d monitoring -c "SELECT count(*) FILTER (WHERE ended_at IS NULL), count(*) FILTER (WHERE ended_at IS NOT NULL) FROM incidents WHERE service_id='$SERVICE_ID';"; }
event_processed() { [ "$(event_count "$1")" -eq 1 ]; }

publish_job() {
  publish_job_for "$1" "$SERVICE_ID" "$2" "$3"
}

publish_job_for() {
  local job_id="$1" service_id="$2" service_url="$3" scheduled_at="$4" payload request
  payload=$(node -e 'console.log(JSON.stringify({job_id:process.argv[1],service_id:process.argv[2],service_url:process.argv[3],timeout_seconds:3,scheduled_at:process.argv[4]}))' "$job_id" "$service_id" "$service_url" "$scheduled_at")
  request=$(node -e 'console.log(JSON.stringify({properties:{content_type:"application/json",delivery_mode:2},routing_key:"checker.jobs",payload:process.argv[1],payload_encoding:"string"}))' "$payload")
  curl -fsS -u monitoring:monitoring -H 'Content-Type: application/json' --data "$request" "$RABBIT_URL/api/exchanges/%2F/amq.default/publish" |
    node -e 'let b="";process.stdin.on("data",d=>b+=d).on("end",()=>{if(!JSON.parse(b).routed)process.exit(1)})'
}

publish_invalid_event() {
  local request
  request=$(node -e 'console.log(JSON.stringify({properties:{content_type:"application/json",delivery_mode:2},routing_key:"incidents.events",payload:"{\\\"id\\\":\\\"invalid\\\",\\\"service_id\\\":\\\"invalid\\\",\\\"type\\\":\\\"invalid\\\"}",payload_encoding:"string"}))')
  curl -fsS -u monitoring:monitoring -H 'Content-Type: application/json' --data "$request" "$RABBIT_URL/api/exchanges/%2F/amq.default/publish" >/dev/null
}

echo 'Starting isolated incident redelivery environment...'
"${COMPOSE[@]}" up --build -d >/dev/null
wait_for 'API readiness' api_ready
wait_for 'Checker consumer' checker_ready
publish_invalid_event
wait_for 'invalid event in DLQ on clean queue' incident_dlq_has_message
"${COMPOSE[@]}" exec -T postgres psql -U monitoring -d monitoring -c "INSERT INTO monitored_services (id,url,enabled,version,updated_at) VALUES ('$SERVICE_ID','http://test-service',FALSE,1,NOW());" >/dev/null

# A successful first check creates the prior available result and no incident.
"${COMPOSE[@]}" stop api >/dev/null
curl -fsS -u monitoring:monitoring -X DELETE "$RABBIT_URL/api/queues/%2F/incidents.events" >/dev/null || true
available_at=$(date -u +%Y-%m-%dT%H:%M:%S.000000000Z)
publish_job incident-redelivery-available http://test-service "$available_at"
wait_for 'available result in InfluxDB' influx_has incident-redelivery-available

echo 'Checking incident.started retry...'
started_at=$(date -u -d '+1 second' +%Y-%m-%dT%H:%M:%S.000000000Z)
publish_job "$STARTED_JOB" http://missing-incident-redelivery.invalid "$started_at"
wait_for 'failed result in InfluxDB' influx_has "$STARTED_JOB"
wait_for 'started job in retry queue' retry_pending
wait_for 'mandatory return for absent incident queue' checker_logged_mandatory_return
retry_not_delivered || fail 'started job was delivered before Consumer recovery'
"${COMPOSE[@]}" start api >/dev/null
wait_for 'API recovery' api_ready
wait_for 'started event processing' event_processed "$STARTED_JOB:incident.started"
[ "$(incident_counts)" = '1|0' ] || fail "started incident counts: $(incident_counts)"

echo 'Checking incident.resolved retry...'
"${COMPOSE[@]}" stop api >/dev/null
curl -fsS -u monitoring:monitoring -X DELETE "$RABBIT_URL/api/queues/%2F/incidents.events" >/dev/null || true
resolved_at=$(date -u -d '+2 second' +%Y-%m-%dT%H:%M:%S.000000000Z)
publish_job "$RESOLVED_JOB" http://test-service "$resolved_at"
wait_for 'recovered result in InfluxDB' influx_has "$RESOLVED_JOB"
wait_for 'resolved job in retry queue' retry_pending
retry_not_delivered || fail 'resolved job was delivered before Consumer recovery'
"${COMPOSE[@]}" start api >/dev/null
wait_for 'API recovery after resolved' api_ready
wait_for 'resolved event processing' event_processed "$RESOLVED_JOB:incident.resolved"
[ "$(incident_counts)" = '0|1' ] || fail "resolved incident counts: $(incident_counts)"
wait_for 'retry queue drain' retry_drained

echo 'Checking five sequential retry publications through one Checker...'
for number in $(seq 1 5); do
  service="incident-retry-series-$number"
  job="incident-retry-series-available-$number"
  "${COMPOSE[@]}" exec -T postgres psql -U monitoring -d monitoring -c "INSERT INTO monitored_services (id,url,enabled,version,updated_at) VALUES ('$service','http://test-service',FALSE,1,NOW());" >/dev/null
  publish_job_for "$job" "$service" http://test-service "$(date -u +%Y-%m-%dT%H:%M:%S.000000000Z)"
  wait_for "available result $number" influx_has "$job"
done
"${COMPOSE[@]}" stop api >/dev/null
curl -fsS -u monitoring:monitoring -X DELETE "$RABBIT_URL/api/queues/%2F/incidents.events" >/dev/null || true
for number in $(seq 1 5); do
  job="incident-retry-series-failed-$number"
  publish_job_for "$job" "incident-retry-series-$number" http://test-service:81 "$(date -u -d "+$number second" +%Y-%m-%dT%H:%M:%S.000000000Z)"
  wait_for "failed result $number" influx_has "$job"
done
wait_for 'five jobs in retry queue' retry_count 5
"${COMPOSE[@]}" start api >/dev/null
wait_for 'API recovery after sequence' api_ready
for number in $(seq 1 5); do
  wait_for "processed retry event $number" event_processed "incident-retry-series-failed-$number:incident.started"
done
wait_for 'retry queue drain after sequence' retry_drained

echo 'PostgreSQL:'
"${COMPOSE[@]}" exec -T postgres psql -U monitoring -d monitoring -c "SELECT event_id,event_type,service_id FROM incident_events WHERE service_id='$SERVICE_ID' ORDER BY occurred_at; SELECT service_id,started_at,ended_at FROM incidents WHERE service_id='$SERVICE_ID';"
echo "RabbitMQ: retry=$(queue_field checker.jobs.retry messages) jobs=$(queue_field checker.jobs messages) unacked=$(queue_field checker.jobs messages_unacknowledged)"
echo "Checker redelivery passed: job IDs=$STARTED_JOB,$RESOLVED_JOB; event IDs=$STARTED_JOB:incident.started,$RESOLVED_JOB:incident.resolved"
