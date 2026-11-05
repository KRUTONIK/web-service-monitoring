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

function statusResponseTime(page, value) {
  return page.getByText('Время ответа', { exact: true }).locator('..').getByText(value, { exact: true })
}

function statusCheckTime(page) {
  return page.getByText('Время проверки', { exact: true }).locator('..').getByText(/01\.11\.2026/)
}

async function mockDashboard(page, { services = [service], status = serviceStatus(), statusCode = 200, statusHandler, incidents = [], checks = [checkResult()], checksHandler } = {}) {
  let created = false
  const createdService = { ...service, id: 'service-2', name: 'Тестовый сервис', url: 'https://new.example' }
  await page.route('**/api/auth/me', (route) => route.fulfill({ contentType: 'application/json', body: JSON.stringify(user) }))
  await page.route('**/api/monitoring/overview?**', (route) => {
    const values = created ? [...services, createdService] : services
    const url = new URL(route.request().url())
    const offset = Number(url.searchParams.get('offset') || 0)
    const limit = Number(url.searchParams.get('limit') || 20)
    const search = (url.searchParams.get('search') || '').toLowerCase()
    const status = url.searchParams.get('status') || ''
    const filtered = values.filter((value) => (!search || value.name.toLowerCase().includes(search) || value.url.toLowerCase().includes(search)) && (!status || status === (value.enabled ? 'available' : 'paused')))
    return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ summary: { available: values.filter((value) => value.enabled).length, unavailable: 0, unknown: 0, paused: values.filter((value) => !value.enabled).length }, services: filtered.slice(offset, offset + limit).map((value) => ({ ...value, status: { state: value.enabled ? 'available' : 'paused', current: value.enabled, last_check: value.enabled ? checkResult({ service_id: value.id, service_url: value.url }) : undefined } })), pagination: { limit, offset, total: filtered.length } }) })
  })
  await page.route('**/api/services', (route) => {
    if (route.request().method() === 'GET') return route.fulfill({ contentType: 'application/json', body: JSON.stringify(created ? [...services, createdService] : services) })
    expect(route.request().postDataJSON()).toEqual({ name: 'Тестовый сервис', url: 'https://new.example', enabled: true, interval_seconds: 60, timeout_seconds: 5 })
    created = true
    return route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify(createdService) })
  })
  await page.route('**/api/services/*', (route) => {
    if (route.request().method() !== 'GET') return route.fallback()
    const id = route.request().url().split('/').pop()
    const value = (created ? [...services, createdService] : services).find((item) => item.id === id)
    return value ? route.fulfill({ contentType: 'application/json', body: JSON.stringify(value) }) : route.fulfill({ status: 404, contentType: 'application/json', body: JSON.stringify({ error: 'Сервис не найден' }) })
  })
  await page.route('**/api/services/*/status', (route) => {
    if (statusHandler) return statusHandler(route)
    return route.fulfill({ status: statusCode, contentType: 'application/json', body: JSON.stringify(status) })
  })
  await page.route('**/api/services/*/incidents?**', (route) => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ items: incidents, limit: 20, offset: 0, total: incidents.length }) }))
  await page.route('**/api/services/*/checks?**', (route) => {
    if (checksHandler) return checksHandler(route)
    const url = new URL(route.request().url())
    return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ checks, limit: Number(url.searchParams.get('limit')), offset: Number(url.searchParams.get('offset')) }) })
  })
}

async function openService(page, name = service.name) {
  await page.getByRole('row', { name: new RegExp(name) }).click()
  await expect(page).toHaveURL(new RegExp(`/${name === service.name ? service.id : ''}$`))
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
  await page.route('**/api/monitoring/overview?**', (route) => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ summary: { available: 0, unavailable: 0, unknown: 0, paused: 0 }, services: [], pagination: { limit: 20, offset: 0, total: 0 } }) }))
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
	await openService(page)
	await expect(page.getByRole('heading', { name: 'Основной сайт' })).toBeVisible()
  await expect(page.getByText('Доступен', { exact: true })).toBeVisible()
  await expect(statusResponseTime(page, '42 мс')).toBeVisible()
  await expect(statusCheckTime(page)).toBeVisible()
})

test('показывает историю и запрашивает выбранный период', async ({ page }) => {
  const checks = [checkResult({ checked_at: '2026-11-01T10:30:00Z', response_time_ms: 42 }), checkResult({ checked_at: '2026-11-01T10:20:00Z', available: false, status_code: 503, error: 'Ошибка соединения' })]
  await mockDashboard(page, { checks })
  await page.goto('/')
	await openService(page)
  await expect(page.getByRole('heading', { name: 'История проверок' })).toBeVisible()
  await expect(page.getByText('Ошибка соединения')).toBeVisible()
  await expect(page.getByTestId('history-chart').locator('svg')).toBeVisible()
  await expect(page.getByTestId('history-chart').locator('.recharts-dot')).toHaveCount(2)
  await page.getByRole('textbox', { name: 'Период истории' }).click()
  await page.getByRole('option', { name: '7 дней', exact: true }).click()
  await expect(page.getByRole('textbox', { name: 'Период истории' })).toHaveValue('7 дней')
})

for (const scenario of [
  ['успех, ошибка, успех', [checkResult({ checked_at: '2026-11-01T12:00:00Z', response_time_ms: 100 }), checkResult({ checked_at: '2026-11-01T12:00:10Z', available: false, status_code: 503, response_time_ms: 500 }), checkResult({ checked_at: '2026-11-01T12:00:20Z', response_time_ms: 110 })], 3],
  ['успех, две ошибки, успех', [checkResult({ checked_at: '2026-11-01T12:00:00Z' }), checkResult({ checked_at: '2026-11-01T12:00:10Z', available: false, status_code: 503 }), checkResult({ checked_at: '2026-11-01T12:00:20Z', available: false, status_code: 504 }), checkResult({ checked_at: '2026-11-01T12:00:30Z' })], 4],
  ['только успехи', [checkResult({ checked_at: '2026-11-01T12:00:00Z', response_time_ms: 0 }), checkResult({ checked_at: '2026-11-01T12:00:10Z', response_time_ms: 5000 })], 2],
  ['только ошибки', [checkResult({ checked_at: '2026-11-01T12:00:00Z', available: false, status_code: 503, response_time_ms: 0 }), checkResult({ checked_at: '2026-11-01T12:00:10Z', available: false, status_code: 503, response_time_ms: 5000 })], 2],
  ['один успех', [checkResult({ checked_at: '2026-11-01T12:00:00Z', response_time_ms: 0 })], 1],
  ['одна ошибка', [checkResult({ checked_at: '2026-11-01T12:00:00Z', available: false, status_code: 503, response_time_ms: 0 })], 1],
  ['длинный пропуск', [checkResult({ checked_at: '2026-11-01T12:00:00Z' }), checkResult({ checked_at: '2026-11-01T14:00:00Z', available: false, status_code: 503 })], 2],
]) {
  test(`отображает ошибки на графике: ${scenario[0]}`, async ({ page }) => {
    await mockDashboard(page, { checks: scenario[1] })
    await page.goto('/')
    await openService(page)
    const chart = page.getByTestId('history-chart')
    await expect(chart.locator('svg')).toBeVisible()
    await expect(chart.locator('.recharts-dot')).toHaveCount(scenario[2])
  })
}

test('распределяет десять проверок с интервалом 10 секунд по временной оси', async ({ page }) => {
  const checks = Array.from({ length: 10 }, (_, index) => checkResult({ checked_at: `2026-11-01T12:00:${String(index * 10).padStart(2, '0')}Z`, response_time_ms: [0, 100, 500, 5000][index % 4], available: index % 2 === 0 }))
  await mockDashboard(page, { checks })
  await page.goto('/')
  await openService(page)
  const dots = page.getByTestId('history-chart').locator('.recharts-dot')
  await expect(dots).toHaveCount(10)
  const x = await dots.evaluateAll((elements) => elements.map((element) => Number(element.getAttribute('cx'))))
  expect(new Set(x.map((value) => value.toFixed(2))).size).toBe(10)
  expect(Math.max(...x) - Math.min(...x)).toBeGreaterThan(200)
})

test('пагинирует историю отдельно от графика и меняет размер страницы', async ({ page }) => {
  const requests = []
  await mockDashboard(page, { checksHandler: (route) => {
    const url = new URL(route.request().url())
    requests.push(url.search)
    const limit = Number(url.searchParams.get('limit'))
    const offset = Number(url.searchParams.get('offset'))
    const checks = url.searchParams.get('chart') === 'true' ? [checkResult({ response_time_ms: 10 }), checkResult({ response_time_ms: 20 })] : Array.from({ length: limit }, (_, index) => checkResult({ checked_at: `2026-11-01T10:${String(29 - index - offset).padStart(2, '0')}:00Z`, response_time_ms: index + offset }))
    return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ checks, limit, offset }) })
  } })
  await page.goto('/')
	await openService(page)
  await expect(page.getByRole('textbox', { name: 'Записей на странице' })).toHaveValue('5')
  await page.getByTestId('history-table').getByRole('button', { name: 'Вперёд' }).click()
  await expect.poll(() => requests.some((request) => request.includes('limit=6') && request.includes('offset=5'))).toBeTruthy()
  await page.getByRole('textbox', { name: 'Записей на странице' }).click()
  await page.getByRole('option', { name: '10', exact: true }).click()
  await expect.poll(() => requests.some((request) => request.includes('limit=11') && request.includes('offset=0'))).toBeTruthy()
  expect(requests.some((request) => request.includes('limit=500') && request.includes('chart=true'))).toBeTruthy()
})

test('сохраняет график при задержке ответа таблицы и не запрашивает его повторно', async ({ page }) => {
  let chartRequests = 0
  let releaseTable
  const tableResponse = new Promise((resolve) => { releaseTable = resolve })
  await mockDashboard(page, { checksHandler: async (route) => {
    const url = new URL(route.request().url())
    if (url.searchParams.get('chart') === 'true') { chartRequests += 1; return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ checks: [checkResult()], limit: 500, offset: 0 }) }) }
    if (url.searchParams.get('offset') === '5') await tableResponse
    return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ checks: Array.from({ length: 6 }, (_, index) => checkResult({ checked_at: `2026-11-01T10:${20 - index}:00Z` })), limit: 6, offset: Number(url.searchParams.get('offset')) }) })
  } })
  await page.goto('/')
  await openService(page)
  await expect(page.getByTestId('history-chart').locator('svg')).toBeVisible()
  await page.getByTestId('history-table').getByRole('button', { name: 'Вперёд' }).click()
  await expect(page.getByTestId('history-chart').locator('svg')).toBeVisible()
  expect(chartRequests).toBe(1)
  releaseTable()
  await expect(page.getByText('Страница 2')).toBeVisible()
  expect(chartRequests).toBe(1)
})

test('обновляет оперативные данные текущего экрана автоматически', async ({ page }) => {
  await page.clock.install()
  const requests = { status: 0, checks: 0 }
  await mockDashboard(page, { statusHandler: (route) => { requests.status += 1; return route.fulfill({ contentType: 'application/json', body: JSON.stringify(serviceStatus()) }) }, checksHandler: (route) => { requests.checks += 1; const url = new URL(route.request().url()); return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ checks: [checkResult()], limit: Number(url.searchParams.get('limit')), offset: 0 }) }) } })
  await page.route('**/api/services/*/incidents?**', (route) => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ items: [], limit: 20, offset: 0, total: 0 }) }))
  await page.goto('/')
  await openService(page)
  await expect.poll(() => Object.values(requests).every((count) => count >= 1)).toBeTruthy()
  await page.clock.fastForward(10_000)
  await expect.poll(() => Object.values(requests).every((count) => count >= 2)).toBeTruthy()
})

test('сообщает об отсутствии результатов проверки', async ({ page }) => {
  await mockDashboard(page, { status: serviceStatus({ state: 'unknown', current: false, last_check: undefined }) })
  await page.goto('/')
  await openService(page)
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
  await navigation
  const detailNavigation = page.waitForURL(`**/${service.id}`)
  await page.getByRole('row', { name: /Основной сайт/ }).click()
  await detailNavigation
  await expect(page.getByRole('status', { name: 'Загрузка последней проверки' })).toBeVisible()
  releaseStatus()
  await expect(page.getByText('Доступен', { exact: true })).toBeVisible()
})

test('показывает недоступность сервиса и детали ошибки', async ({ page }) => {
  await mockDashboard(page, {
    status: serviceStatus({ state: 'unavailable', last_check: checkResult({ available: false, status_code: 503, response_time_ms: 215, error: 'Сервис временно недоступен' }) }),
  })
  await page.goto('/')
  await openService(page)
  await expect(page.getByText('Недоступен', { exact: true })).toBeVisible()
  await expect(page.getByText('503')).toBeVisible()
  await expect(statusResponseTime(page, '215 мс')).toBeVisible()
  await expect(page.getByText('Сервис временно недоступен')).toBeVisible()
})

test('показывает ошибку Metrics API', async ({ page }) => {
  await mockDashboard(page, { statusCode: 502, status: { error: 'Не удалось получить данные метрик' } })
  await page.goto('/')
  await openService(page)
  await expect(page.getByText('Не удалось получить данные метрик')).toBeVisible()
})

test('показывает открытый и завершённый инциденты', async ({ page }) => {
  await mockDashboard(page, { incidents: [
    { id: 'incident-open', service_id: service.id, started_at: '2026-11-01T10:00:00Z', ended_at: null },
    { id: 'incident-closed', service_id: service.id, started_at: '2026-11-01T08:00:00Z', ended_at: '2026-11-01T08:01:30Z' },
  ] })
  await page.goto('/')
	await openService(page)
  await expect(page.getByText('Текущий сбой')).toBeVisible()
  await expect(page.getByText('Завершённый сбой')).toBeVisible()
  await expect(page.getByText('90 с')).toBeVisible()
})

test('сохраняет карточку инцидентов при задержке следующей страницы', async ({ page }) => {
  let release
  const delayed = new Promise((resolve) => { release = resolve })
  const incidents = Array.from({ length: 21 }, (_, index) => ({ id: `incident-${index}`, service_id: service.id, started_at: `2026-11-01T${String(index % 10).padStart(2, '0')}:00:00Z`, ended_at: null }))
  await mockDashboard(page)
  await page.route('**/api/services/*/incidents?**', async (route) => {
    const offset = Number(new URL(route.request().url()).searchParams.get('offset'))
    if (offset === 20) await delayed
    await route.fulfill({ contentType: 'application/json', body: JSON.stringify({ items: incidents.slice(offset, offset + 20), limit: 20, offset, total: incidents.length }) })
  })
  await page.goto('/')
  await openService(page)
  const card = page.getByTestId('incidents')
  await expect(card.getByRole('heading', { name: 'Инциденты' })).toBeVisible()
  await card.getByRole('button', { name: 'Вперёд' }).click()
  await expect(card.getByRole('heading', { name: 'Инциденты' })).toBeVisible()
  await expect(card.getByText('Обновление инцидентов…')).toBeVisible()
  release()
  await expect(card.getByText('Начало: 01.11.2026')).toBeVisible()
})

test('показывает устаревший результат как неактуальные данные', async ({ page }) => {
  await mockDashboard(page, { status: serviceStatus({ state: 'unknown', current: false }) })
  await page.goto('/')
  await openService(page)
  await expect(page.getByText('Нет актуальных данных')).toBeVisible()
  await expect(page.getByText('Показан последний известный результат.')).toBeVisible()
  await expect(statusResponseTime(page, '42 мс')).toBeVisible()
})

test('показывает отключённый мониторинг', async ({ page }) => {
  await mockDashboard(page, { status: serviceStatus({ state: 'paused', current: false, last_check: undefined }) })
  await page.goto('/')
  await openService(page)
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
  await openService(page)
  await expect(statusResponseTime(page, '42 мс')).toBeVisible()
  await page.clock.fastForward(10_000)
  await expect(statusResponseTime(page, '64 мс')).toBeVisible()
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
  await page.getByRole('row', { name: /Второй сайт.*https:\/\/second\.example/ }).click()
  await expect(page).toHaveURL(/\/service-2$/)
  await expect(statusResponseTime(page, '99 мс')).toBeVisible()
  await expect(page.getByText('Недоступен', { exact: true })).toBeVisible()
})

test('сохраняет выбранный сервис при пагинации, поиске и фильтре', async ({ page }) => {
  const selected = { ...service, id: 'service-1', name: 'Выбранный сайт' }
  const otherServices = Array.from({ length: 20 }, (_, index) => ({ ...service, id: `service-${index + 2}`, name: `Сервис ${index + 2}`, url: `https://service-${index + 2}.example` }))
  await mockDashboard(page, { services: [selected, ...otherServices] })
  await page.goto('/')
  await page.getByRole('row', { name: /Выбранный сайт.*https:\/\/service\.example/ }).click()
  await expect(page.getByRole('heading', { name: 'Выбранный сайт' })).toBeVisible()
  await page.goBack()
  await expect(page).toHaveURL(/\/$/)
  await page.getByRole('button', { name: 'Следующая страница сервисов' }).click()
  await page.getByLabel('Поиск').fill('Сервис 11')
  await expect(page.getByRole('row', { name: /Сервис 11/ })).toBeVisible()
  await page.getByRole('textbox', { name: 'Состояние' }).click()
  await page.getByRole('option', { name: 'Доступные', exact: true }).click()
  await page.getByRole('row', { name: /Сервис 11.*https:\/\/service-11\.example/ }).click()
  await expect(page.getByRole('heading', { name: 'Сервис 11' })).toBeVisible()
})

test('передаёт сортировку обзора на сервер и возвращается к первой странице', async ({ page }) => {
  const requests = []
  await mockDashboard(page, { services: Array.from({ length: 21 }, (_, index) => ({ ...service, id: `service-${index + 1}`, name: `Сервис ${index + 1}` })) })
  await page.route('**/api/monitoring/overview?**', (route) => {
    requests.push(route.request().url())
    return route.fallback()
  })
  await page.goto('/')
  await page.getByRole('button', { name: 'Следующая страница сервисов' }).click()
  await page.getByRole('textbox', { name: 'Сортировка' }).click()
  await page.getByRole('option', { name: 'По названию', exact: true }).click()
  await expect.poll(() => requests.some((url) => url.includes('sort=name') && url.includes('offset=0'))).toBeTruthy()
})

test('изменяет сервис', async ({ page }) => {
  const services = [{ ...service }]
  const updated = { ...service, name: 'Изменённый сайт', url: 'https://updated.example', interval_seconds: 30, timeout_seconds: 7, version: 2 }
  await mockDashboard(page, { services })
  await page.route('**/api/services/service-1', (route) => {
    if (route.request().method() === 'GET') return route.fallback()
    expect(route.request().postDataJSON()).toEqual({ name: updated.name, url: updated.url, enabled: true, interval_seconds: 30, timeout_seconds: 7 })
    services[0] = updated
    return route.fulfill({ contentType: 'application/json', body: JSON.stringify(updated) })
  })
  await page.goto('/')
  await openService(page)
  await page.getByRole('button', { name: 'Изменить' }).click()
  await page.getByLabel('Название').fill(updated.name)
  await page.getByLabel('URL').fill(updated.url)
  await page.getByRole('textbox', { name: 'Интервал проверки' }).click()
  await page.getByRole('option', { name: '30 с', exact: true }).click()
  await page.getByLabel('Тайм-аут, с').fill('7')
  await page.getByRole('button', { name: 'Сохранить' }).click()
  await expect(page.getByRole('heading', { name: updated.name })).toBeVisible()
  await expect(page.getByRole('heading', { name: updated.name }).locator('..').getByText(updated.url, { exact: true })).toBeVisible()
})

test('удаляет сервис из списка', async ({ page }) => {
  const services = [{ ...service }]
  await mockDashboard(page, { services })
  await page.route('**/api/services/service-1', (route) => {
    if (route.request().method() === 'GET') return route.fallback()
    services.splice(0, 1)
    return route.fulfill({ status: 204 })
  })
  await page.goto('/')
  await openService(page)
  page.once('dialog', (dialog) => dialog.accept())
  await page.getByRole('button', { name: 'Удалить' }).click()
  await expect(page.getByText('Сервисов пока нет. Добавьте первый сервис для мониторинга.')).toBeVisible()
  await expect(page.getByRole('heading', { name: service.name })).not.toBeVisible()
})

test('показывает 404 для неизвестного маршрута и сохраняет футер', async ({ page }) => {
  await mockDashboard(page)
  await page.goto('/unknown/path')
  await expect(page.getByRole('heading', { name: 'Страница не найдена' })).toBeVisible()
  await expect(page.getByText('Покладов Никита')).toBeVisible()
  await page.getByRole('button', { name: 'К сервисам' }).click()
  await expect(page).toHaveURL(/\/$/)
})

test('показывает 404 для недоступного сервиса', async ({ page }) => {
  await mockDashboard(page)
  await page.route('**/api/services/missing', (route) => route.fulfill({ status: 404, contentType: 'application/json', body: JSON.stringify({ error: 'Сервис не найден' }) }))
  await page.goto('/missing')
  await expect(page.getByText('Страница не найдена или у вас нет доступа к этому сервису.')).toBeVisible()
})
