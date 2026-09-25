#!/usr/bin/env bash

set -euo pipefail

COMPOSE=(
    docker compose
    --project-name monitoring-prototype-test
    -f compose.yml
)

cleanup() {
    "${COMPOSE[@]}" down -v
}

trap cleanup EXIT

echo "Starting prototype..."
"${COMPOSE[@]}" up --build -d

echo "Waiting for API Service..."
for i in {1..60}; do
    status=$(docker inspect \
        --format='{{.State.Health.Status}}' \
        monitoring-api 2>/dev/null || true)

    if [ "$status" = "healthy" ]; then
        break
    fi

    if [ "$i" -eq 60 ]; then
        echo "API Service failed to become healthy"
        "${COMPOSE[@]}" logs api
        exit 1
    fi

    sleep 2
done

echo "Waiting for Checker Service..."
for i in {1..60}; do
    status=$(docker inspect \
        --format='{{.State.Status}}' \
        monitoring-checker 2>/dev/null || true)

    if [ "$status" = "running" ]; then
        break
    fi

    if [ "$i" -eq 60 ]; then
        echo "Checker Service did not start"
        "${COMPOSE[@]}" logs checker
        exit 1
    fi

    sleep 2
done

echo "Checking Metrics Service..."
status=$(docker inspect \
    --format='{{.State.Status}}' \
    monitoring-metrics 2>/dev/null || true)
if [ "$status" != "running" ]; then
    echo "Metrics Service is not running"
    "${COMPOSE[@]}" logs metrics
    exit 1
fi

echo "Checking API result..."
for i in {1..60}; do
    if result=$(curl --fail --silent http://localhost:8080/api/checks/latest); then
        break
    fi

    if [ "$i" -eq 60 ]; then
        echo "Monitoring result did not become available"
        "${COMPOSE[@]}" logs api checker metrics
        exit 1
    fi

    sleep 2
done

echo "$result" | jq -e '
    .service_url == "https://example.com" and
    .available == true and
    .status_code == 200 and
    .response_time_ms >= 0
' > /dev/null

echo "Checking Web UI..."
curl --fail --silent http://localhost:5173/ |
    grep --quiet '<div id="root"></div>'

echo "Prototype integration test passed."
