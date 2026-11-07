#!/usr/bin/env bash

set -euo pipefail

COMPOSE=(
    docker compose
    --project-name monitoring-prototype-test
    -f compose.yml
)

cleanup() {
    "${COMPOSE[@]}" down -v
    rm -f "$cookie_jar" "$cookie_jar_b" "$response_body"
}

service_id() {
    local ids id
    ids=$("${COMPOSE[@]}" ps -q "$1")
    IFS=$'\n' read -r id _ <<< "$ids"
    printf '%s\n' "$id"
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

wait_for_metrics() {
    local service="$1"
    local period="$2"
    for i in {1..30}; do
        request GET "http://localhost:8080/api/services/${service}/metrics?period=${period}" --cookie "$cookie_jar"
        if [ "$http_code" = "200" ]; then
            return
        fi
        sleep 1
    done
    fail "Показатели ${service} за ${period} не стали доступны"
}

write_controlled_checks() {
    local service="$1"
    local now
    now=$(node -e 'process.stdout.write(String(Date.now() * 1000000))')
    local old=$((now - 2 * 60 * 60 * 1000000000))
    curl --fail --silent --show-error \
        --header 'Authorization: Token monitoring-test-token' \
        --header 'Content-Type: text/plain; charset=utf-8' \
        --data-binary "service_check,job_id=metrics-0,service_id=${service},service_url=http://metrics-fixture available=true,status_code=200i,response_time_ms=0i ${now}
service_check,job_id=metrics-100,service_id=${service},service_url=http://metrics-fixture available=true,status_code=200i,response_time_ms=100i $((now + 1))
service_check,job_id=metrics-failure,service_id=${service},service_url=http://metrics-fixture available=false,status_code=503i,response_time_ms=5000i,error=\"upstream failed\" $((now + 2))
service_check,job_id=metrics-200,service_id=${service},service_url=http://metrics-fixture available=true,status_code=200i,response_time_ms=200i $((now + 3))
service_check,job_id=metrics-old,service_id=${service},service_url=http://metrics-fixture available=true,status_code=200i,response_time_ms=999i ${old}" \
        'http://localhost:8086/api/v2/write?org=monitoring&bucket=monitoring&precision=ns'
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
cookie_jar_b=$(mktemp)
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

    # Metrics can briefly return 502 while the newly started InfluxDB query
    # path becomes available. The assertion below still requires the actual
    # check result, so retry both the expected absence and this startup state.
    if [ "$http_code" != "404" ] && [ "$http_code" != "502" ]; then
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

echo "Проверка изоляции пользователей..."
request POST "http://localhost:8080/api/auth/register" \
    --cookie-jar "$cookie_jar_b" \
    --header "Origin: http://localhost:5173" \
    --header "Content-Type: application/json" \
    --data '{"email":"prototype-test-b@example.test","password":"prototype-test-password"}'
expect_code 201 "Регистрация второго тестового пользователя"

for endpoint in \
    "/api/services/${service_id}" \
    "/api/services/${service_id}/checks" \
    "/api/services/${service_id}/metrics" \
    "/api/services/${service_id}/incidents"; do
    request GET "http://localhost:8080${endpoint}" --cookie "$cookie_jar_b"
    expect_code 404 "Доступ второго пользователя к чужому сервису ${endpoint}"
done

request PUT "http://localhost:8080/api/services/${service_id}" \
    --cookie "$cookie_jar_b" \
    --header "Origin: http://localhost:5173" \
    --header "Content-Type: application/json" \
    --data '{"name":"Чужой сервис","url":"http://test-service","enabled":true,"interval_seconds":60,"timeout_seconds":5}'
expect_code 404 "Изменение чужого сервиса"
request DELETE "http://localhost:8080/api/services/${service_id}" \
    --cookie "$cookie_jar_b" \
    --header "Origin: http://localhost:5173"
expect_code 404 "Удаление чужого сервиса"

for endpoint in "/api/services" "/api/monitoring/overview"; do
    request GET "http://localhost:8080${endpoint}" --cookie "$cookie_jar_b"
    expect_code 200 "Список второго пользователя ${endpoint}"
    node -e 'const value = JSON.parse(process.argv[1]); const text = JSON.stringify(value); process.exit(text.includes(process.argv[2]) ? 1 : 0)' "$(<"$response_body")" "$service_id" || fail "Чужой сервис появился в ${endpoint}"
done

request GET "http://localhost:8080/api/services/${service_id}"
expect_code 401 "Неавторизованный доступ к сервису"

echo "Проверка агрегатов и истории InfluxDB..."
request POST "http://localhost:8080/api/services" \
    --cookie "$cookie_jar" \
    --header "Origin: http://localhost:5173" \
    --header "Content-Type: application/json" \
    --data '{"name":"Контролируемые метрики","url":"http://metrics-fixture","enabled":false}'
expect_code 201 "Создание сервиса контролируемых метрик"
metrics_service_id=$(node -e 'const service = JSON.parse(process.argv[1]); if (!service.id) process.exit(1); process.stdout.write(service.id)' "$(<"$response_body")") || fail "Ответ создания сервиса метрик не содержит идентификатор"
write_controlled_checks "$metrics_service_id"
wait_for_metrics "$metrics_service_id" 1h
node -e '
const value = JSON.parse(process.argv[1]);
if (value.total_checks !== 4 || value.successful_checks !== 3 || value.success_rate !== 75 || value.average_response_ms !== 100 || value.p95_response_ms !== 200 || value.has_response_metrics !== true) process.exit(1);
' "$(<"$response_body")" || fail "Агрегаты InfluxDB за 1h некорректны"
request GET "http://localhost:8080/api/services/${metrics_service_id}/checks?period=1h&limit=10" --cookie "$cookie_jar"
expect_code 200 "История контролируемых метрик за 1h"
node -e 'const value = JSON.parse(process.argv[1]); const times = value.checks.map(check => check.response_time_ms).sort((a,b) => a-b); if (value.checks.length !== 4 || JSON.stringify(times) !== JSON.stringify([0,100,200,5000])) process.exit(1)' "$(<"$response_body")" || fail "История InfluxDB за 1h некорректна"
wait_for_metrics "$metrics_service_id" 24h
node -e 'const value = JSON.parse(process.argv[1]); if (value.total_checks !== 5) process.exit(1)' "$(<"$response_body")" || fail "Период 24h не включает историческую проверку"

for fixture in empty failed; do
    request POST "http://localhost:8080/api/services" --cookie "$cookie_jar" --header "Origin: http://localhost:5173" --header "Content-Type: application/json" --data "{\"name\":\"Метрики ${fixture}\",\"url\":\"http://${fixture}-metrics\",\"enabled\":false}"
    expect_code 201 "Создание сервиса ${fixture}"
    fixture_id=$(node -e 'const service = JSON.parse(process.argv[1]); if (!service.id) process.exit(1); process.stdout.write(service.id)' "$(<"$response_body")") || fail "Ответ создания ${fixture} не содержит идентификатор"
    if [ "$fixture" = "failed" ]; then
        curl --fail --silent --show-error --header 'Authorization: Token monitoring-test-token' --header 'Content-Type: text/plain; charset=utf-8' --data-binary "service_check,job_id=metrics-failed,service_id=${fixture_id},service_url=http://failed-metrics available=false,status_code=503i,response_time_ms=5000i $(node -e 'process.stdout.write(String(Date.now() * 1000000))')" 'http://localhost:8086/api/v2/write?org=monitoring&bucket=monitoring&precision=ns'
    fi
    wait_for_metrics "$fixture_id" 1h
    node -e 'const value = JSON.parse(process.argv[1]); const expected = Number(process.argv[2]); if (value.total_checks !== expected || value.successful_checks !== 0 || value.has_response_metrics !== false) process.exit(1)' "$(<"$response_body")" "$([ "$fixture" = failed ] && echo 1 || echo 0)" || fail "Пустые/неуспешные показатели ${fixture} некорректны"
done

echo "Проверка Web UI..."
curl --fail --silent http://localhost:5173/ |
    grep --quiet '<div id="root"></div>'

echo "Интеграционный тест прототипа успешно пройден."
