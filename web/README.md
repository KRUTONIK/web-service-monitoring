# Web UI

Интерфейс показывает последнюю проверку веб-сервиса: URL, доступность,
HTTP-код, время ответа и время проверки. Данные поступают из REST API.

Приложение разработано на React и Vite.

## Локальный запуск

Установите зависимости:

```bash
npm install
```

Запустите сервер разработки:

```bash
npm run dev
```

По умолчанию Vite откроет интерфейс по адресу, указанному в выводе команды.

## Сборка и проверка

Проверить код ESLint:

```bash
npm run lint
```

Собрать приложение:

```bash
npm run build
```

## Адрес API

По умолчанию интерфейс обращается к `http://localhost:8080`. Чтобы указать
другой адрес, задайте переменную `VITE_API_URL` до запуска или сборки:

```bash
VITE_API_URL=http://localhost:8080 npm run dev
```

В PowerShell:

```powershell
$env:VITE_API_URL = "http://localhost:8080"
npm run dev
```
