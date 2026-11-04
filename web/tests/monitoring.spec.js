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

function serviceStatus(overrides = {}) {
  return { state: 'available', current: true, last_check: checkResult(), ...overrides }
}

async function mockDashboard(page, { services = [service], status = serviceStatus(), statusCode = 200, statusHandler, incidents = [], checks = [checkResult()], checksHandler } = {}) {
  let created = false
  const createdService = { ...service, id: 'service-2', name: 'Тестовый сервис', url: 'https://new.example' }
  await page.route('**/api/auth/me', (route) => route.fulfill({ contentType: 'application/json', body: JSON.stringify(user) }))
  await page.route('**/api/services', (route) => {
    if (route.request().method() === 'GET') return route.fulfill({ contentType: 'application/json', body: JSON.stringify(created ? [...services, createdService] : services) })
    expect(route.request().postDataJSON()).toEqual({ name: 'Тестовый сервис', url: 'https://new.example', enabled: true, interval_seconds: 60, timeout_seconds: 5 })
    created = true
    return route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify(createdService) })
  })
  await page.route('**/api/services/*/status', (route) => {
    if (statusHandler) return statusHandler(route)
    return route.fulfill({ status: statusCode, contentType: 'application/json', body: JSON.stringify(status) })
  })
  await page.route('**/api/services/*/incidents', (route) => route.fulfill({ contentType: 'application/json', body: JSON.stringify(incidents) }))
  await page.route('**/api/services/*/checks?**', (route) => {
    if (checksHandler) return checksHandler(route)
    const url = new URL(route.request().url())
    return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ checks, limit: Number(url.searchParams.get('limit')), offset: Number(url.searchParams.get('offset')) }) })
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

test('показывает сервис и актуальное состояние', async ({ page }) => {
  await mockDashboard(page)
  await page.goto('/')
	await expect(page.getByRole('heading', { name: 'Основной сайт' })).toBeVisible()
  await expect(page.getByText('Доступен')).toBeVisible()
  await expect(page.getByText('42 мс')).toBeVisible()
  await expect(page.getByText(/01\.11\.2026/)).toBeVisible()
})

test('показывает историю и запрашивает выбранный период', async ({ page }) => {
  const checks = [checkResult({ checked_at: '2026-11-01T10:30:00Z', response_time_ms: 42 }), checkResult({ checked_at: '2026-11-01T10:20:00Z', available: false, status_code: 503, error: 'Ошибка соединения' })]
  await mockDashboard(page, { checks })
  await page.goto('/')
  await expect(page.getByRole('heading', { name: 'История проверок' })).toBeVisible()
  await expect(page.getByText('Ошибка соединения')).toBeVisible()
  await page.getByRole('textbox', { name: 'Период истории' }).click()
  await page.getByRole('option', { name: '7 дней', exact: true }).click()
  await expect(page.getByRole('textbox', { name: 'Период истории' })).toHaveValue('7 дней')
})

test('пагинирует историю отдельно от графика и меняет размер страницы', async ({ page }) => {
  const requests = []
  await mockDashboard(page, { checksHandler: (route) => {
    const url = new URL(route.request().url())
    requests.push(url.search)
    const limit = Number(url.searchParams.get('limit'))
    const offset = Number(url.searchParams.get('offset'))
    const checks = limit === 1000 ? [checkResult({ response_time_ms: 10 }), checkResult({ response_time_ms: 20 })] : Array.from({ length: limit }, (_, index) => checkResult({ checked_at: `2026-11-01T10:${String(29 - index - offset).padStart(2, '0')}:00Z`, response_time_ms: index + offset }))
    return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ checks, limit, offset }) })
  } })
  await page.goto('/')
  await expect(page.getByRole('textbox', { name: 'Записей на странице' })).toHaveValue('5')
  await page.getByRole('button', { name: 'Вперёд' }).click()
  await expect.poll(() => requests.some((request) => request.includes('limit=5') && request.includes('offset=5'))).toBeTruthy()
  await page.getByRole('textbox', { name: 'Записей на странице' }).click()
  await page.getByRole('option', { name: '10', exact: true }).click()
  await expect.poll(() => requests.some((request) => request.includes('limit=10') && request.includes('offset=0'))).toBeTruthy()
  expect(requests.some((request) => request.includes('limit=1000') && request.includes('chart=true'))).toBeTruthy()
})

test('обновляет все данные текущего экрана', async ({ page }) => {
  const requests = { services: 0, status: 0, incidents: 0, checks: 0 }
  await mockDashboard(page, { statusHandler: (route) => { requests.status += 1; return route.fulfill({ contentType: 'application/json', body: JSON.stringify(serviceStatus()) }) }, checksHandler: (route) => { requests.checks += 1; const url = new URL(route.request().url()); return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ checks: [checkResult()], limit: Number(url.searchParams.get('limit')), offset: 0 }) }) } })
  await page.route('**/api/services', (route) => { requests.services += 1; return route.fulfill({ contentType: 'application/json', body: JSON.stringify([service]) }) })
  await page.route('**/api/services/*/incidents', (route) => { requests.incidents += 1; return route.fulfill({ contentType: 'application/json', body: '[]' }) })
  await page.goto('/')
  await page.getByRole('button', { name: 'Обновить' }).click()
  await expect.poll(() => Object.values(requests).every((count) => count >= 2)).toBeTruthy()
})

test('сообщает об отсутствии результатов проверки', async ({ page }) => {
  await mockDashboard(page, { status: serviceStatus({ state: 'unknown', current: false, last_check: undefined }) })
  await page.goto('/')
  await expect(page.getByText('Проверок для сервиса пока нет.')).toBeVisible()
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

test('показывает загрузку состояния и затем результат', async ({ page }) => {
  let releaseStatus
  const statusResponse = new Promise((resolve) => { releaseStatus = resolve })
  await mockDashboard(page, {
    statusHandler: async (route) => {
      await statusResponse
      await route.fulfill({ contentType: 'application/json', body: JSON.stringify(serviceStatus()) })
    },
  })

  const navigation = page.goto('/', { waitUntil: 'domcontentloaded' })
  await expect(page.getByRole('status', { name: 'Загрузка последней проверки' })).toBeVisible()
  releaseStatus()
  await navigation
  await expect(page.getByText('Доступен')).toBeVisible()
})

test('показывает недоступность сервиса и детали ошибки', async ({ page }) => {
  await mockDashboard(page, {
    status: serviceStatus({ state: 'unavailable', last_check: checkResult({ available: false, status_code: 503, response_time_ms: 215, error: 'Сервис временно недоступен' }) }),
  })
  await page.goto('/')
  await expect(page.getByText('Недоступен', { exact: true })).toBeVisible()
  await expect(page.getByText('503')).toBeVisible()
  await expect(page.getByText('215 мс')).toBeVisible()
  await expect(page.getByText('Сервис временно недоступен')).toBeVisible()
})

test('обновляет состояние вручную без перезагрузки страницы', async ({ page }) => {
  let requests = 0
  await mockDashboard(page, {
    statusHandler: (route) => {
      requests += 1
      return route.fulfill({ contentType: 'application/json', body: JSON.stringify(serviceStatus({ last_check: checkResult({ response_time_ms: requests === 1 ? 42 : 87 }) })) })
    },
  })
  await page.goto('/')
  await expect(page.getByText('42 мс')).toBeVisible()
  await page.getByRole('button', { name: 'Обновить' }).click()
  await expect(page.getByText('87 мс')).toBeVisible()
  expect(requests).toBe(2)
})

test('показывает ошибку Metrics API', async ({ page }) => {
  await mockDashboard(page, { statusCode: 502, status: { error: 'Не удалось получить данные метрик' } })
  await page.goto('/')
  await expect(page.getByText('Не удалось получить данные метрик')).toBeVisible()
})

test('показывает открытый и завершённый инциденты', async ({ page }) => {
  await mockDashboard(page, { incidents: [
    { id: 'incident-open', service_id: service.id, started_at: '2026-11-01T10:00:00Z', ended_at: null },
    { id: 'incident-closed', service_id: service.id, started_at: '2026-11-01T08:00:00Z', ended_at: '2026-11-01T08:01:30Z' },
  ] })
  await page.goto('/')
  await expect(page.getByText('Текущий сбой')).toBeVisible()
  await expect(page.getByText('Завершённый сбой')).toBeVisible()
  await expect(page.getByText('90 с')).toBeVisible()
})

test('показывает устаревший результат как неактуальные данные', async ({ page }) => {
  await mockDashboard(page, { status: serviceStatus({ state: 'unknown', current: false }) })
  await page.goto('/')
  await expect(page.getByText('Нет актуальных данных')).toBeVisible()
  await expect(page.getByText('Показан последний известный результат.')).toBeVisible()
  await expect(page.getByText('42 мс')).toBeVisible()
})

test('показывает отключённый мониторинг', async ({ page }) => {
  await mockDashboard(page, { status: serviceStatus({ state: 'paused', current: false, last_check: undefined }) })
  await page.goto('/')
  await expect(page.getByText('Мониторинг отключён')).toBeVisible()
})

test('обновляет состояние автоматически через 10 секунд', async ({ page }) => {
  await page.clock.install()
  let requests = 0
  await mockDashboard(page, {
    statusHandler: (route) => {
      requests += 1
      return route.fulfill({ contentType: 'application/json', body: JSON.stringify(serviceStatus({ last_check: checkResult({ response_time_ms: requests === 1 ? 42 : 64 }) })) })
    },
  })
  await page.goto('/')
  await expect(page.getByText('42 мс')).toBeVisible()
  await page.clock.fastForward(10_000)
  await expect(page.getByText('64 мс')).toBeVisible()
  expect(requests).toBe(2)
})

test('при переключении сервиса показывает только его состояние', async ({ page }) => {
  const second = { ...service, id: 'service-2', name: 'Второй сайт', url: 'https://second.example' }
  await mockDashboard(page, {
    services: [service, second],
    statusHandler: (route) => {
      const isSecond = route.request().url().includes('/service-2/status')
      return route.fulfill({ contentType: 'application/json', body: JSON.stringify(serviceStatus({ state: isSecond ? 'unavailable' : 'available', last_check: checkResult({ service_id: isSecond ? second.id : service.id, response_time_ms: isSecond ? 99 : 42 }) })) })
    },
  })
  await page.goto('/')
  await expect(page.getByText('42 мс')).toBeVisible()
  await page.getByRole('textbox', { name: 'Выбранный сервис' }).click()
  await page.getByRole('option', { name: 'Второй сайт', exact: true }).click()
  await expect(page.getByText('99 мс')).toBeVisible()
  await expect(page.getByText('Недоступен', { exact: true })).toBeVisible()
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
  await page.getByRole('textbox', { name: 'Интервал проверки' }).click()
  await page.getByRole('option', { name: '30 с', exact: true }).click()
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
