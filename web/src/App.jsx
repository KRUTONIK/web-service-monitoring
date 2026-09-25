import { useCallback, useEffect, useState } from 'react'
import './App.css'

const apiURL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080'

async function getLatestCheck(signal) {
  const response = await fetch(`${apiURL}/api/checks/latest`, { signal })
  if (response.ok) return response.json()
  if (response.status === 404) throw new Error('Результатов проверки пока нет')
  throw new Error('Не удалось получить данные мониторинга')
}

function formatDate(value) {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '—' : date.toLocaleString('ru-RU')
}

function App() {
  const [check, setCheck] = useState(null)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)

  const refresh = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      setCheck(await getLatestCheck())
    } catch (requestError) {
      setError(requestError.message)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    const controller = new AbortController()
    getLatestCheck(controller.signal)
      .then(setCheck)
      .catch((requestError) => {
        if (requestError.name !== 'AbortError') setError(requestError.message)
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false)
      })
    return () => controller.abort()
  }, [])

  return (
    <main className="container">
      <header>
        <div>
          <h1>Мониторинг веб-сервиса</h1>
          <p>Последний результат проверки</p>
        </div>
        <button type="button" onClick={refresh} disabled={loading}>
          {loading ? 'Обновление…' : 'Обновить'}
        </button>
      </header>

      {loading && !check && <p className="message">Загрузка…</p>}
      {error && !check && <p className="message error">{error}</p>}

      {check && (
        <section>
          <div className="service">
            <strong>{check.service_url}</strong>
            <span className={check.available ? 'available' : 'unavailable'}>
              {check.available ? 'Доступен' : 'Недоступен'}
            </span>
          </div>

          <dl>
            <div>
              <dt>HTTP-код</dt>
              <dd>{check.status_code || '—'}</dd>
            </div>
            <div>
              <dt>Время ответа</dt>
              <dd>{check.response_time_ms} мс</dd>
            </div>
            <div>
              <dt>Время проверки</dt>
              <dd>{formatDate(check.checked_at)}</dd>
            </div>
          </dl>

          {check.error && <p className="error">{check.error}</p>}
          {error && <p className="error">{error}</p>}
        </section>
      )}
    </main>
  )
}

export default App
