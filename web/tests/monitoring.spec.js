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

async function mockDashboard(page, { services = [service], latest = checkResult(), latestStatus = 200 } = {}) {
	let created = false
	const createdService = { ...service, id: 'service-2', name: 'Тестовый сервис', url: 'https://new.example' }
  await page.route('**/api/auth/me', (route) => route.fulfill({ contentType: 'application/json', body: JSON.stringify(user) }))
  await page.route('**/api/services', (route) => {
		if (route.request().method() === 'GET') return route.fulfill({ contentType: 'application/json', body: JSON.stringify(created ? [...services, createdService] : services) })
		created = true
		return route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify(createdService) })
  })
  await page.route('**/api/checks/latest?service_id=service-1', (route) => route.fulfill({ status: latestStatus, contentType: 'application/json', body: JSON.stringify(latest) }))
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
