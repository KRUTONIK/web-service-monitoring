import { Card, Text } from '@mantine/core'
import { useQuery } from '@tanstack/react-query'
import { Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import { apiRequest } from '../api.js'

function chartData(checks) {
  const points = checks.map((check) => ({ ...check, time: new Date(check.checked_at).getTime(), success: check.available ? check.response_time_ms : null, failure: check.available ? null : check.response_time_ms })).filter((check) => Number.isFinite(check.time)).sort((a, b) => a.time - b.time)
  const intervals = points.slice(1).map((point, index) => point.time - points[index].time).filter((value) => value > 0).sort((a, b) => a - b)
  const median = intervals[Math.floor(intervals.length / 2)] ?? 0
  const gap = Math.max(5 * 60_000, median * 3)
  return points.flatMap((point, index) => index > 0 && point.time - points[index - 1].time > gap ? [{ time: points[index - 1].time + (point.time - points[index - 1].time) / 2, success: null, failure: null, gap: true }, point] : [point])
}

function timeDomain(data) {
  const times = data.filter((point) => !point.gap).map((point) => point.time)
  const start = Math.min(...times)
  const end = Math.max(...times)
  if (start === end) return [start - 5_000, end + 5_000]
  return [start, end]
}

function point(color) {
  return ({ cx, cy, payload }) => payload.gap || !Number.isFinite(cx) || !Number.isFinite(cy) ? null : <circle className="recharts-dot" cx={cx} cy={cy} r={4} fill={color} stroke="#fff" strokeWidth={1.5} />
}

function HistoryTooltip({ active, payload }) {
  const point = payload?.find((item) => !item.payload.gap)?.payload
  if (!active || !point) return null
  return <Card withBorder shadow="sm" p="xs"><Text size="sm">{new Date(point.time).toLocaleString('ru-RU')}</Text><Text size="sm">{point.available ? 'Успешно' : 'Ошибка'} · {point.response_time_ms ?? '—'} мс</Text><Text size="sm">HTTP-код: {point.status_code ?? '—'}</Text>{point.error && <Text size="sm" c="red">{point.error}</Text>}</Card>
}

export function HistoryChart({ serviceID, period }) {
  const query = useQuery({ queryKey: ['service-chart', serviceID, period], queryFn: () => apiRequest(`/api/services/${serviceID}/checks?period=${period}&chart=true&limit=500&offset=0`), enabled: Boolean(serviceID), retry: false, refetchInterval: period === '24h' ? 10_000 : 30_000 })
  if (query.isPending) return <Card withBorder data-testid="history-chart"><Text c="dimmed">Загрузка графика…</Text></Card>
  if (query.isError) return <Card withBorder data-testid="history-chart"><Text c="red">{query.error.message}</Text></Card>
  const data = chartData(query.data.checks ?? [])
  const domain = timeDomain(data)
  return <Card withBorder data-testid="history-chart">{data.length === 0 ? <Text c="dimmed">Нет данных для графика.</Text> : <div style={{ height: 260 }}><ResponsiveContainer><LineChart data={data}><XAxis dataKey="time" type="number" scale="time" domain={domain} tickFormatter={(value) => new Date(value).toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })} /><YAxis unit=" мс" /><Tooltip content={<HistoryTooltip />} /><Line dataKey="success" name="Успешно" stroke="#40c057" strokeWidth={2} dot={point('#40c057')} activeDot={{ r: 5 }} connectNulls={false} isAnimationActive={false} /><Line dataKey="failure" name="Ошибка" stroke="#fa5252" strokeWidth={2} dot={point('#fa5252')} activeDot={{ r: 5 }} connectNulls={false} isAnimationActive={false} /></LineChart></ResponsiveContainer></div>}</Card>
}
