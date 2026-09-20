#!/usr/bin/env bash

set -euo pipefail

COMPOSE_FILE="deploy/docker-compose.infrastructure.yml"

cleanup() {
    docker compose -f "$COMPOSE_FILE" down -v
}

trap cleanup EXIT

echo "Starting infrastructure..."
docker compose -f "$COMPOSE_FILE" up -d

echo "Waiting for containers..."

services=(
    "monitoring-postgres"
    "monitoring-rabbitmq"
    "monitoring-influxdb"
)

for service in "${services[@]}"; do
    echo "Waiting for $service..."

    for i in {1..30}; do
        status=$(docker inspect \
            --format='{{.State.Health.Status}}' \
            "$service" 2>/dev/null || true)

        if [ "$status" = "healthy" ]; then
            echo "$service is healthy"
            break
        fi

        if [ "$i" -eq 30 ]; then
            echo "$service failed to become healthy"
            docker logs "$service"
            exit 1
        fi

        sleep 2
    done
done

echo "Testing PostgreSQL..."
docker compose -f "$COMPOSE_FILE" exec -T postgres \
    psql \
    -U monitoring \
    -d monitoring \
    -c "SELECT 1;"

echo "Testing RabbitMQ..."
docker compose -f "$COMPOSE_FILE" exec -T rabbitmq \
    rabbitmq-diagnostics -q ping

echo "Testing InfluxDB..."
curl --fail --silent \
    http://localhost:8086/health \
    > /dev/null

echo "All infrastructure integration tests passed."