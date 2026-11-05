import { Card, Text } from '@mantine/core'
import { useQuery } from '@tanstack/react-query'
import { Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import { apiRequest } from '../api.js'

export function HistoryChart({ serviceID, period }) {
  const query = useQuery({ queryKey: ['service-chart', serviceID, period], queryFn: () => apiRequest(`/api/services/${serviceID}/checks?period=${period}&chart=true&limit=500&offset=0`), enabled: Boolean(serviceID), retry: false, refetchInterval: period === '24h' ? 10_000 : 30_000 })
  if (query.isPending) return <Card withBorder data-testid="history-chart"><Text c="dimmed">Загрузка графика…</Text></Card>
  if (query.isError) return <Card withBorder data-testid="history-chart"><Text c="red">{query.error.message}</Text></Card>
  const data = (query.data.checks ?? []).map((check) => ({ ...check, time: new Date(check.checked_at).getTime(), success: check.available ? check.response_time_ms : null, failure: check.available ? null : check.response_time_ms })).filter((check) => Number.isFinite(check.time)).sort((a, b) => a.time - b.time)
  return <Card withBorder data-testid="history-chart">{data.length === 0 ? <Text c="dimmed">Нет данных для графика.</Text> : <div style={{ height: 260 }}><ResponsiveContainer><LineChart data={data}><XAxis dataKey="time" type="number" scale="time" /><YAxis unit=" мс" /><Tooltip /><Line dataKey="success" stroke="#40c057" dot={false} connectNulls={false} /><Line dataKey="failure" stroke="#fa5252" dot={false} connectNulls={false} /></LineChart></ResponsiveContainer></div>}</Card>
}
