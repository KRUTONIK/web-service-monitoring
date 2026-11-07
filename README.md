# Мониторинг веб-сервисов

Микросервисная система для пользовательского мониторинга доступности и времени
ответа веб-сервисов. Текущая версия — `v1.0.0`.

## Возможности

- регистрация и аутентификация пользователей по сессионной cookie;
- создание, изменение, включение, отключение и мягкое удаление собственных
  веб-сервисов;
- индивидуальные интервалы проверок и HTTP-тайм-ауты;
- отображение доступности сервиса: «Доступен», «Недоступен», «Нет актуальных
  данных» и «Мониторинг отключён»;
- хранение последних проверок в InfluxDB;
- фиксация начала сбоев, восстановления и истории инцидентов;
- email-уведомления владельцу о сбое и восстановлении (включаются отдельно для каждого сервиса);
- повторная обработка заданий и сообщений RabbitMQ без дублирования
  результатов или инцидентов.

## Архитектура

- **Web UI** предоставляет регистрацию, управление сервисами, их состоянием и
  историей инцидентов; работает с API по REST.
- **API Service** хранит пользователей, конфигурации и инциденты в PostgreSQL,
  предоставляет REST API, получает метрики от Metrics Service по gRPC и
  обрабатывает события инцидентов RabbitMQ.
- **Scheduler Service** получает конфигурацию API по gRPC, применяет быстрые
  обновления из RabbitMQ и создаёт задания только для включённых сервисов.
- **Checker Service** выполняет HTTP-проверки, записывает результаты в
  InfluxDB и публикует события начала сбоя или восстановления.
- **Metrics Service** читает результаты InfluxDB и предоставляет их API по
  gRPC.
- **Notification Service** принимает отдельную очередь `notifications.events`,
  отправляет письма SMTP и сохраняет статус доставки.

PostgreSQL является источником истины для пользователей, настроек и
инцидентов; InfluxDB хранит результаты проверок. RabbitMQ передаёт задания,
изменения конфигурации и события. API Service не обращается к InfluxDB
напрямую.

Событие инцидента фиксируется вместе с записью в `notification_outbox` в одной
транзакции PostgreSQL. Publisher API передаёт outbox в отдельную durable
очередь `notifications.events` с publisher confirms. Notification Service
использует retry с задержкой и DLQ. Обычные повторные доставки идемпотентны;
авария между SMTP-отправкой и записью статуса теоретически может дать дубль.

## Технологии

Go, React, Vite, TanStack Query, PostgreSQL, InfluxDB, RabbitMQ, gRPC, Docker
Compose и GitHub Actions.

## Запуск через Docker Compose

Нужны Docker и Docker Compose. В корне репозитория:

```bash
docker compose up --build -d
```

После запуска откройте Web UI и зарегистрируйте пользователя. Затем создайте
сервис в интерфейсе и задайте его URL, интервал и тайм-аут проверки.

Основные адреса:

- Web UI: <http://localhost:5173>
- REST API: <http://localhost:8080>
- проверка готовности API: <http://localhost:8080/health>
- RabbitMQ Management: <http://localhost:15672> (`monitoring` / `monitoring`)
- InfluxDB: <http://localhost:8086>
- Mailpit: <http://localhost:8025>

Mailpit принимает локальную почту на `mailpit:1025` и не пересылает её во
внешнюю сеть. Для внешнего SMTP задайте в `.env` (см. `.env.example`)
`SMTP_ADDRESS`, `SMTP_FROM`, `SMTP_USERNAME`, `SMTP_PASSWORD` и
`SMTP_TLS=true`; TLS-сертификаты проверяются стандартной библиотекой Go.
`WEB_PUBLIC_URL` добавляет в письмо ссылку на страницу сервиса. Пароль SMTP
не выводится в журналы.

Остановить сервисы, сохранив данные:

```bash
docker compose down
```

Удалить также локальные данные:

```bash
docker compose down -v
```

### Несколько Checker

Checker работают как конкурирующие потребители одной очереди. Для запуска двух
экземпляров:

```bash
docker compose up --scale checker=2
```

## Проверки и тесты

GitHub Actions выполняет проверку сообщений коммитов, Go-тесты, `go vet`,
golangci-lint, ESLint, production build, Playwright и Docker Compose
интеграционные сценарии.

Локальные интеграционные сценарии:

```bash
bash scripts/integration-test.sh
bash scripts/prototype-test.sh
bash scripts/monitoring-core-test.sh
bash scripts/incident-redelivery-test.sh
bash scripts/incident-queue-compatibility-test.sh
bash scripts/notification-integration-test.sh
```

Web-проверки запускаются из каталога `web`:

```bash
npm run lint
npm run build
npm run test:e2e
```

Подробности о тестировании приведены в
[документации по тестированию](docs/testing.md). Известные предупреждения
зависимостей описаны в [техническом долге](docs/dependency-audit.md).

## Ограничения

Для прототипной среды InfluxDB
использует общий токен вместо разделённых прав чтения и записи.
