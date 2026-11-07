# Автоматизированное тестирование

## Цель и среда

Проверки подтверждают работу пользовательского мониторинга: сессионную
аутентификацию, управление собственными сервисами, получение и отображение
результатов, историю, инциденты и email-уведомления.

- **Go `testing`** — модульные тесты сервисов и контрактов с тестовыми
  реализациями внешних зависимостей или локальными HTTP-серверами.
- **Playwright** — пользовательские сценарии React Web UI в Chromium с
  перехваченными REST-ответами.
- **Docker Compose** — изолированные интеграционные сценарии с PostgreSQL,
  RabbitMQ, InfluxDB, сервисами приложения и, где требуется, Mailpit.
- **GitHub Actions** — единый запуск проверок, сборки и публикация отчётов.

## Матрица покрытия

| Область | Что проверяется | Тип | Сценарии |
|---|---|---|---|
| Контракты и Scheduler | `job_id`, снимки конфигурации, версии, интервалы, публикация и ошибки gRPC | модульный | Go-тесты `contracts`, `services/scheduler` |
| Checker | HTTP-статусы, URL, тайм-ауты, запись результатов, инциденты и retry | модульный | Go-тесты `services/checker` |
| Metrics | CSV InfluxDB, временные метки, история, агрегаты и ошибки gRPC | модульный | Go-тесты `services/metrics` |
| API и пользователи | сессии, доступ владельца, CRUD сервисов, `notifications_enabled`, метрики и инциденты | модульный | Go-тесты `services/api` |
| Уведомления | SMTP, некорректные события, идемпотентность, retry, DLQ и UTF-8 | модульный | Go-тесты `services/notification` |
| Инфраструктура и мониторинг | готовность компонентов, планирование, несколько Checker, retry и DLQ заданий | интеграционный | `integration-test.sh`, `monitoring-core-test.sh` |
| Пользовательский поток | регистрация, создание сервиса, результат проверки, история и изоляция | интеграционный | `prototype-test.sh` |
| События инцидентов | повторная доставка и совместимость очередей RabbitMQ | интеграционный | `incident-redelivery-test.sh`, `incident-queue-compatibility-test.sh` |
| Почтовая доставка | SMTP retry, успешная доставка, отсутствие дублей и DLQ без самопроизвольной отправки | интеграционный | `notification-integration-test.sh` |
| Web UI | аутентификация, сервисы, история, инциденты, уведомления, маршрутизация и ошибки API | пользовательский | `web/tests/monitoring.spec.js` |

Интеграционные сценарии используют уникальные тестовые данные и удаляют только
свои Compose-проекты и volumes. Сложные взаимодействия PostgreSQL, RabbitMQ и
InfluxDB проверяются в них без замены реальных систем моками.

## Локальный запуск

В корне репозитория:

```bash
go test ./contracts/... ./services/api/... ./services/checker/... ./services/metrics/... ./services/notification/... ./services/scheduler/...
go vet ./contracts/... ./services/api/... ./services/checker/... ./services/metrics/... ./services/notification/... ./services/scheduler/...
bash scripts/integration-test.sh
bash scripts/prototype-test.sh
bash scripts/monitoring-core-test.sh
bash scripts/incident-redelivery-test.sh
bash scripts/incident-queue-compatibility-test.sh
bash scripts/notification-integration-test.sh
```

Пользовательские проверки запускаются из каталога `web`:

```bash
npm ci
npx playwright install chromium
npm run lint
npm run build
npm run test:e2e
```

После Playwright доступны `web/playwright-report/index.html` и
`web/test-results/junit.xml`; эти отчёты создаются заново и не добавляются в
Git.

## GitHub Actions и ограничения

CI проверяет сообщения коммитов, форматирование, `go vet`, Go-тесты с
покрытием, golangci-lint, ESLint, production build, Playwright и все
перечисленные Docker Compose-сценарии. После их успеха CI собирает артефакт
приложения и сводку метрик качества.

Артефакты `go-coverage`, `go-test-reports`, `playwright-report`,
`application-build` и `quality-metrics` доступны также при ошибке последующей
проверки. Текущее покрытие и число тестов рассчитываются в каждом запуске CI;
искусственная цель в 100% не используется.

Интеграционные проверки не заменяют нагрузочное тестирование и не доказывают
гарантированную exactly-once доставку: авария между SMTP-отправкой и записью
статуса теоретически может привести к повторному письму. Известные
предупреждения сторонних зависимостей описаны в
[аудите зависимостей](dependency-audit.md).
