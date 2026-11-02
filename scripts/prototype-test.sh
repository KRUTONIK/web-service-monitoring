#!/usr/bin/env bash

set -euo pipefail

COMPOSE=(
    docker compose
    --project-name monitoring-prototype-test
    -f compose.yml
)

cleanup() {
    "${COMPOSE[@]}" down -v
    rm -f "$cookie_jar" "$response_body"
}

service_id() {
    "${COMPOSE[@]}" ps -q "$1" | head -n 1
}

fail() {
    echo "$1" >&2
    "${COMPOSE[@]}" logs api scheduler checker metrics >&2 || true
    exit 1
}

request() {
    local method="$1"
    local url="$2"
    shift 2
    http_code=$(curl --silent --show-error --output "$response_body" --write-out '%{http_code}' --request "$method" "$@" "$url") || fail "Не удалось выполнить запрос ${method} ${url}"
}

expect_code() {
    local expected="$1"
    local operation="$2"
    if [ "$http_code" != "$expected" ]; then
        echo "${operation}: получен HTTP ${http_code}, ожидался ${expected}" >&2
        cat "$response_body" >&2
        fail "Запрос завершился с неожиданным HTTP-кодом"
    fi
}

cookie_jar=$(mktemp)
response_body=$(mktemp)

trap cleanup EXIT

echo "Запуск прототипа..."
"${COMPOSE[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
"${COMPOSE[@]}" up --build -d

echo "Ожидание API Service..."
for i in {1..60}; do
    api_id=$(service_id api)
    status=$(docker inspect \
        --format='{{.State.Health.Status}}' \
        "$api_id" 2>/dev/null || true)

    if [ "$status" = "healthy" ]; then
        break
    fi

    if [ "$i" -eq 60 ]; then
        fail "API Service не перешёл в работоспособное состояние"
    fi

    sleep 2
done

echo "Ожидание Checker Service..."
for i in {1..60}; do
    checker_id=$(service_id checker)
    status=$(docker inspect \
        --format='{{.State.Status}}' \
        "$checker_id" 2>/dev/null || true)

    if [ "$status" = "running" ]; then
        break
    fi

    if [ "$i" -eq 60 ]; then
        fail "Checker Service не запустился"
    fi

    sleep 2
done

echo "Проверка Metrics Service..."
metrics_id=$(service_id metrics)
status=$(docker inspect \
    --format='{{.State.Status}}' \
    "$metrics_id" 2>/dev/null || true)
if [ "$status" != "running" ]; then
    fail "Metrics Service не запущен"
fi

echo "Проверка ограничения доступа к результатам..."
request GET "http://localhost:8080/api/checks/latest?service_id=unknown"
expect_code 401 "Неавторизованный запрос последней проверки"

echo "Регистрация тестового пользователя..."
request POST "http://localhost:8080/api/auth/register" \
    --cookie-jar "$cookie_jar" \
    --header "Origin: http://localhost:5173" \
    --header "Content-Type: application/json" \
    --data '{"email":"prototype-test@example.test","password":"prototype-test-password"}'
expect_code 201 "Регистрация тестового пользователя"

echo "Создание контролируемого сервиса..."
request POST "http://localhost:8080/api/services" \
    --cookie "$cookie_jar" \
    --header "Origin: http://localhost:5173" \
    --header "Content-Type: application/json" \
    --data '{"name":"Тестовый сервис прототипа","url":"http://test-service","enabled":true}'
expect_code 201 "Создание контролируемого сервиса"
service_id=$(node -e '
const service = JSON.parse(process.argv[1]);
if (!service.id) process.exit(1);
process.stdout.write(service.id);
' "$(<"$response_body")") || fail "Ответ создания сервиса не содержит идентификатор"

echo "Ожидание результата проверки через API..."
result=""
for i in {1..60}; do
    request GET "http://localhost:8080/api/checks/latest?service_id=${service_id}" --cookie "$cookie_jar"
    if [ "$http_code" = "200" ]; then
        result=$(<"$response_body")
        break
    fi

    if [ "$http_code" != "404" ]; then
        echo "Получен HTTP ${http_code} при запросе результата проверки:" >&2
        cat "$response_body" >&2
        fail "Не удалось получить результат мониторинга"
    fi

    if [ "$i" -eq 60 ]; then
        fail "Результат мониторинга не стал доступен за отведённое время"
    fi

    sleep 2
done

node -e '
const result = JSON.parse(process.argv[1]);
if (result.service_id !== process.argv[2] || result.service_url !== "http://test-service" || result.available !== true || result.status_code !== 200 || result.response_time_ms < 0) {
  process.exit(1);
}
' "$result" "$service_id" || fail "Получен некорректный результат мониторинга"

echo "Проверка Web UI..."
curl --fail --silent http://localhost:5173/ |
    grep --quiet '<div id="root"></div>'

echo "Интеграционный тест прототипа успешно пройден."
