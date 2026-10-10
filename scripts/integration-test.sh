#!/usr/bin/env bash

set -euo pipefail

COMPOSE=(
    docker compose
    --project-name monitoring-infrastructure-test
    -f deploy/docker-compose.infrastructure.yml
)

cleanup() {
    "${COMPOSE[@]}" down -v
}

service_id() {
    "${COMPOSE[@]}" ps -q "$1" | head -n 1
}

trap cleanup EXIT

echo "Starting infrastructure..."
"${COMPOSE[@]}" up -d

echo "Waiting for containers..."
services=(postgres rabbitmq influxdb)
for service in "${services[@]}"; do
    echo "Waiting for $service..."
    for i in {1..30}; do
        container_id=$(service_id "$service")
        status=$(docker inspect --format='{{.State.Health.Status}}' "$container_id" 2>/dev/null || true)
        if [ "$status" = "healthy" ]; then
            echo "$service is healthy"
            break
        fi
        if [ "$i" -eq 30 ]; then
            echo "$service failed to become healthy"
            "${COMPOSE[@]}" logs "$service"
            exit 1
        fi
        sleep 2
    done
done

echo "Testing PostgreSQL..."
"${COMPOSE[@]}" exec -T postgres psql -U monitoring -d monitoring -c "SELECT 1;"

echo "Testing RabbitMQ..."
"${COMPOSE[@]}" exec -T rabbitmq rabbitmq-diagnostics -q ping

echo "Testing InfluxDB..."
curl --fail --silent http://localhost:8086/health > /dev/null

echo "All infrastructure integration tests passed."
