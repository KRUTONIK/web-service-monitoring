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

echo "Запуск инфраструктуры..."
"${COMPOSE[@]}" up -d

echo "Ожидание запуска контейнеров..."
services=(postgres rabbitmq influxdb)
for service in "${services[@]}"; do
    echo "Ожидание сервиса $service..."
    for i in {1..30}; do
        container_id=$(service_id "$service")
        status=$(docker inspect --format='{{.State.Health.Status}}' "$container_id" 2>/dev/null || true)
        if [ "$status" = "healthy" ]; then
            echo "Сервис $service работоспособен"
            break
        fi
        if [ "$i" -eq 30 ]; then
            echo "Сервис $service не перешёл в работоспособное состояние"
            "${COMPOSE[@]}" logs "$service"
            exit 1
        fi
        sleep 2
    done
done

echo "Проверка PostgreSQL..."
"${COMPOSE[@]}" exec -T postgres psql -U monitoring -d monitoring -c "SELECT 1;"

echo "Проверка RabbitMQ..."
"${COMPOSE[@]}" exec -T rabbitmq rabbitmq-diagnostics -q ping

echo "Проверка InfluxDB..."
curl --fail --silent http://localhost:8086/health > /dev/null

echo "Все интеграционные тесты инфраструктуры успешно пройдены."
