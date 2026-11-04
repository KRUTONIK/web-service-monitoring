const apiURL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080'

export class APIError extends Error {
  constructor(status, message) {
    super(message)
    this.status = status
  }
}

export async function apiRequest(path, options = {}) {
  const response = await fetch(`${apiURL}${path}`, {
    credentials: 'include',
    ...options,
  })
  const contentType = response.headers.get('content-type') ?? ''
  const body = contentType.includes('application/json') ? await response.json() : null

  if (!response.ok) {
    throw new APIError(response.status, body?.error ?? 'Не удалось выполнить запрос')
  }

  return body
}

export function jsonRequest(method, body) {
  return {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  }
}
