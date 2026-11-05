import { Card, Text } from '@mantine/core'
import { useQuery } from '@tanstack/react-query'
import { Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import { apiRequest } from '../api.js'

function chartData(checks) {
  return checks.map((check) => ({ ...check, time: new Date(check.checked_at).getTime() })).filter((check) => Number.isFinite(check.time)).sort((a, b) => a.time - b.time)
}

function timeDomain(data) {
  const times = data.map((point) => point.time)
  const start = Math.min(...times)
  const end = Math.max(...times)
  if (start === end) return [start - 5_000, end + 5_000]
  return [start, end]
}

function point({ cx, cy, payload }) {
  return !Number.isFinite(cx) || !Number.isFinite(cy) ? null : <circle className="recharts-dot" cx={cx} cy={cy} r={4} fill={payload.available ? '#40c057' : '#fa5252'} stroke="#fff" strokeWidth={1.5} />
}

function HistoryTooltip({ active, payload }) {
  const point = payload?.[0]?.payload
  if (!active || !point) return null
  return <Card withBorder shadow="sm" p="xs"><Text size="sm">{new Date(point.time).toLocaleString('ru-RU')}</Text><Text size="sm">{point.available ? 'Успешно' : 'Ошибка'} · {point.response_time_ms ?? '—'} мс</Text><Text size="sm">HTTP-код: {point.status_code ?? '—'}</Text>{point.error && <Text size="sm" c="red">{point.error}</Text>}</Card>
}

export function HistoryChart({ serviceID, period }) {
  const query = useQuery({ queryKey: ['service-chart', serviceID, period], queryFn: () => apiRequest(`/api/services/${serviceID}/checks?period=${period}&chart=true&limit=500&offset=0`), enabled: Boolean(serviceID), retry: false, refetchInterval: period === '24h' ? 10_000 : 30_000 })
  if (query.isPending) return <Card withBorder data-testid="history-chart"><Text c="dimmed">Загрузка графика…</Text></Card>
  if (query.isError) return <Card withBorder data-testid="history-chart"><Text c="red">{query.error.message}</Text></Card>
  const data = chartData(query.data.checks ?? [])
  const domain = timeDomain(data)
  return <Card withBorder data-testid="history-chart">{data.length === 0 ? <Text c="dimmed">Нет данных для графика.</Text> : <div style={{ height: 260 }}><ResponsiveContainer><LineChart data={data}><XAxis dataKey="time" type="number" scale="time" domain={domain} tickFormatter={(value) => new Date(value).toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })} /><YAxis unit=" мс" /><Tooltip content={<HistoryTooltip />} /><Line type="linear" dataKey="response_time_ms" name="Время ответа" stroke="#868e96" strokeWidth={2} dot={point} activeDot={{ r: 5 }} isAnimationActive={false} /></LineChart></ResponsiveContainer></div>}</Card>
}
