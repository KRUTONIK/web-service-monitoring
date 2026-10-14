import { expect, test } from '@playwright/test'

const checkedAt = '2026-09-25T12:30:00Z'

function checkResult(overrides = {}) {
  return {
    service_id: 'prototype-service',
    service_url: 'https://service.example',
    checked_at: checkedAt,
    available: true,
    status_code: 200,
    response_time_ms: 42,
    ...overrides,
  }
}

async function mockLatestCheck(page, result, status = 200) {
  await page.route('**/api/checks/latest', async (route) => {
    await route.fulfill({
      status,
      contentType: 'application/json',
      body: JSON.stringify(result),
    })
  })
}

test('отображает состояние загрузки', async ({ page }) => {
  let resolveRequest
  const requestPending = new Promise((resolve) => {
    resolveRequest = resolve
  })
  await page.route('**/api/checks/latest', async (route) => {
    await requestPending
    await route.fulfill({ contentType: 'application/json', body: JSON.stringify(checkResult()) })
  })

  const navigation = page.goto('/')
  await expect(page.getByText('Загрузка…')).toBeVisible()
  resolveRequest()
  await navigation
  await expect(page.getByText('Доступен')).toBeVisible()
})

test('показывает доступный сервис и его метрики', async ({ page }) => {
  await mockLatestCheck(page, checkResult())

  await page.goto('/')

  await expect(page.getByText('https://service.example')).toBeVisible()
  await expect(page.getByText('Доступен')).toBeVisible()
  await expect(page.getByText('200', { exact: true })).toBeVisible()
  await expect(page.getByText('42 мс')).toBeVisible()
  await expect(page.getByText(/25\.09\.2026/)).toBeVisible()
})

test('показывает недоступный сервис и ошибку проверки', async ({ page }) => {
  await mockLatestCheck(page, checkResult({
    available: false,
    status_code: 503,
    response_time_ms: 125,
    error: 'Сервис временно недоступен',
  }))

  await page.goto('/')

  await expect(page.getByText('Недоступен', { exact: true })).toBeVisible()
  await expect(page.getByText('503', { exact: true })).toBeVisible()
  await expect(page.getByText('125 мс')).toBeVisible()
  await expect(page.getByText('Сервис временно недоступен')).toBeVisible()
})

test('обновляет результат по кнопке', async ({ page }) => {
  let refreshed = false
  await page.route('**/api/checks/latest', async (route) => {
    await route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify(checkResult(refreshed
        ? { service_url: 'https://updated.example', response_time_ms: 87 }
        : {})),
    })
  })

  await page.goto('/')
  await expect(page.getByText('https://service.example')).toBeVisible()

  refreshed = true
  await page.getByRole('button', { name: 'Обновить' }).click()

  await expect(page.getByText('https://updated.example')).toBeVisible()
  await expect(page.getByText('87 мс')).toBeVisible()
})

test('сообщает об отсутствии результатов мониторинга', async ({ page }) => {
  await mockLatestCheck(page, { error: 'check result not found' }, 404)

  await page.goto('/')

  await expect(page.getByText('Результатов проверки пока нет')).toBeVisible()
})

test('корректно показывает ошибку API без ошибок JavaScript', async ({ page }) => {
  const pageErrors = []
  page.on('pageerror', (error) => pageErrors.push(error.message))
  await mockLatestCheck(page, { error: 'monitoring unavailable' }, 502)

  await page.goto('/')

  await expect(page.getByText('Не удалось получить данные мониторинга')).toBeVisible()
  expect(pageErrors).toEqual([])
})
