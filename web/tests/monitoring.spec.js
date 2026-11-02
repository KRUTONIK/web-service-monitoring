import { expect, test as base } from '@playwright/test'

const test = base.extend({
  page: async ({ page }, run) => {
    const pageErrors = []
    page.on('pageerror', (error) => pageErrors.push(error.message))
    await run(page)
    expect(pageErrors).toEqual([])
  },
})

const user = { id: 'user-1', email: 'user@example.test' }
const service = {
  id: 'service-1',
  name: 'Основной сайт',
  url: 'https://service.example',
  enabled: true,
  interval_seconds: 60,
  timeout_seconds: 5,
  version: 1,
  created_at: '2026-11-01T09:00:00Z',
  updated_at: '2026-11-01T09:00:00Z',
}

function checkResult(overrides = {}) {
  return {
    service_id: service.id,
    service_url: service.url,
    checked_at: '2026-11-01T10:30:00Z',
    available: true,
    status_code: 200,
    response_time_ms: 42,
    ...overrides,
  }
}

async function mockUnauthenticated(page) {
  await page.route('**/api/auth/me', (route) => route.fulfill({ status: 401, contentType: 'application/json', body: JSON.stringify({ error: 'Требуется аутентификация' }) }))
}

async function mockDashboard(page, { services = [service], latest = checkResult(), latestStatus = 200, latestHandler } = {}) {
  let created = false
  const createdService = { ...service, id: 'service-2', name: 'Тестовый сервис', url: 'https://new.example' }
  await page.route('**/api/auth/me', (route) => route.fulfill({ contentType: 'application/json', body: JSON.stringify(user) }))
  await page.route('**/api/services', (route) => {
    if (route.request().method() === 'GET') return route.fulfill({ contentType: 'application/json', body: JSON.stringify(created ? [...services, createdService] : services) })
    expect(route.request().postDataJSON()).toEqual({ name: 'Тестовый сервис', url: 'https://new.example', enabled: true, interval_seconds: 60, timeout_seconds: 5 })
    created = true
    return route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify(createdService) })
  })
  await page.route('**/api/checks/latest?service_id=service-1', (route) => {
    if (latestHandler) return latestHandler(route)
    return route.fulfill({ status: latestStatus, contentType: 'application/json', body: JSON.stringify(latest) })
  })
}

test('показывает форму входа неавторизованному пользователю', async ({ page }) => {
  await mockUnauthenticated(page)
  await page.goto('/')
  await expect(page.getByRole('heading', { name: 'Войти' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Войти' })).toBeVisible()
})

test('регистрирует пользователя и открывает пустой список сервисов', async ({ page }) => {
  await mockUnauthenticated(page)
  await page.route('**/api/auth/register', (route) => route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify(user) }))
  await page.route('**/api/services', (route) => route.fulfill({ contentType: 'application/json', body: '[]' }))
  await page.goto('/')
  await page.getByRole('button', { name: 'Зарегистрироваться' }).click()
  await page.getByLabel('Электронная почта').fill(user.email)
  await page.getByLabel('Пароль').fill('correct-horse-battery')
  await page.getByRole('button', { name: 'Зарегистрироваться' }).click()
  await expect(page.getByText('Сервисов пока нет. Добавьте первый сервис для мониторинга.')).toBeVisible()
})

test('показывает сервис и последнюю проверку', async ({ page }) => {
  await mockDashboard(page)
  await page.goto('/')
	await expect(page.getByRole('heading', { name: 'Основной сайт' })).toBeVisible()
  await expect(page.getByText('Доступен')).toBeVisible()
  await expect(page.getByText('42 мс')).toBeVisible()
  await expect(page.getByText(/01\.11\.2026/)).toBeVisible()
})

test('сообщает об отсутствии результатов проверки', async ({ page }) => {
  await mockDashboard(page, { latest: { error: 'Результатов нет' }, latestStatus: 404 })
  await page.goto('/')
  await expect(page.getByText('Результатов проверки пока нет.')).toBeVisible()
})

test('создаёт сервис', async ({ page }) => {
  await mockDashboard(page, { services: [] })
  await page.goto('/')
  await page.getByRole('button', { name: 'Добавить сервис' }).click()
  await page.getByLabel('Название').fill('Тестовый сервис')
  await page.getByLabel('URL').fill('https://new.example')
	await expect(page.getByText('Интервал проверки')).toBeVisible()
	await page.getByRole('button', { name: 'Добавить', exact: true }).click()
	await expect(page.getByRole('heading', { name: 'Тестовый сервис' })).toBeVisible()
})

test('завершает сессию', async ({ page }) => {
  await mockDashboard(page)
  await page.route('**/api/auth/logout', (route) => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ status: 'ok' }) }))
  await page.goto('/')
  await page.getByRole('button', { name: 'Выйти' }).click()
  await expect(page.getByRole('heading', { name: 'Войти' })).toBeVisible()
})

test('показывает загрузку последней проверки и затем результат', async ({ page }) => {
  let releaseLatest
  const latestResponse = new Promise((resolve) => { releaseLatest = resolve })
  await mockDashboard(page, {
    latestHandler: async (route) => {
      await latestResponse
      await route.fulfill({ contentType: 'application/json', body: JSON.stringify(checkResult()) })
    },
  })

  const navigation = page.goto('/', { waitUntil: 'domcontentloaded' })
  await expect(page.getByRole('status', { name: 'Загрузка последней проверки' })).toBeVisible()
  releaseLatest()
  await navigation
  await expect(page.getByText('Доступен')).toBeVisible()
})

test('показывает недоступность сервиса и детали ошибки', async ({ page }) => {
  await mockDashboard(page, {
    latest: checkResult({ available: false, status_code: 503, response_time_ms: 215, error: 'Сервис временно недоступен' }),
  })
  await page.goto('/')
  await expect(page.getByText('Недоступен', { exact: true })).toBeVisible()
  await expect(page.getByText('503')).toBeVisible()
  await expect(page.getByText('215 мс')).toBeVisible()
  await expect(page.getByText('Сервис временно недоступен')).toBeVisible()
})

test('обновляет последнюю проверку вручную без перезагрузки страницы', async ({ page }) => {
  let requests = 0
  await mockDashboard(page, {
    latestHandler: (route) => {
      requests += 1
      return route.fulfill({ contentType: 'application/json', body: JSON.stringify(checkResult({ response_time_ms: requests === 1 ? 42 : 87 })) })
    },
  })
  await page.goto('/')
  await expect(page.getByText('42 мс')).toBeVisible()
  await page.getByRole('button', { name: 'Обновить' }).click()
  await expect(page.getByText('87 мс')).toBeVisible()
  expect(requests).toBe(2)
})

test('показывает ошибку Metrics API', async ({ page }) => {
  await mockDashboard(page, { latestStatus: 502, latest: { error: 'Не удалось получить данные метрик' } })
  await page.goto('/')
  await expect(page.getByText('Не удалось получить данные метрик')).toBeVisible()
})

test('изменяет сервис', async ({ page }) => {
  const services = [{ ...service }]
  const updated = { ...service, name: 'Изменённый сайт', url: 'https://updated.example', interval_seconds: 30, timeout_seconds: 7, version: 2 }
  await mockDashboard(page, { services })
  await page.route('**/api/services/service-1', (route) => {
    expect(route.request().postDataJSON()).toEqual({ name: updated.name, url: updated.url, enabled: true, interval_seconds: 30, timeout_seconds: 7 })
    services[0] = updated
    return route.fulfill({ contentType: 'application/json', body: JSON.stringify(updated) })
  })
  await page.goto('/')
  await page.getByRole('button', { name: 'Изменить' }).click()
  await page.getByLabel('Название').fill(updated.name)
  await page.getByLabel('URL').fill(updated.url)
	await page.getByLabel('Интервал проверки').selectOption('30')
	await page.getByLabel('Тайм-аут, с').fill('7')
  await page.getByRole('button', { name: 'Сохранить' }).click()
  await expect(page.getByRole('heading', { name: updated.name })).toBeVisible()
  await expect(page.getByText(updated.url)).toBeVisible()
})

test('удаляет сервис из списка', async ({ page }) => {
  const services = [{ ...service }]
  await mockDashboard(page, { services })
  await page.route('**/api/services/service-1', (route) => {
    services.splice(0, 1)
    return route.fulfill({ status: 204 })
  })
  await page.goto('/')
  page.once('dialog', (dialog) => dialog.accept())
  await page.getByRole('button', { name: 'Удалить' }).click()
  await expect(page.getByText('Сервисов пока нет. Добавьте первый сервис для мониторинга.')).toBeVisible()
  await expect(page.getByRole('heading', { name: service.name })).not.toBeVisible()
})
