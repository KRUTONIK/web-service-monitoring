import { Alert, Badge, Button, Card, Group, Loader, Modal, SimpleGrid, Stack, Text, Title } from '@mantine/core'
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { apiRequest, jsonRequest } from '../api.js'
import { ServiceForm } from './ServiceForm.jsx'
import { HistoryChart } from './HistoryChart.jsx'
import { HistoryTable } from './HistoryTable.jsx'
import { formatDate, statePresentation } from './shared.js'

const Metric = ({ label, value }) => <div><Text size="sm" c="dimmed">{label}</Text><Text fw={600}>{value}</Text></div>

function Status({ serviceID }) {
  const query = useQuery({ queryKey: ['service-status', serviceID], queryFn: () => apiRequest(`/api/services/${serviceID}/status`), retry: false, refetchInterval: 10_000 })
  if (query.isPending) return <div role="status" aria-label="Загрузка последней проверки"><Loader size="sm" /></div>
  if (query.isError) return <Alert color="red">{query.error.message}</Alert>
  const value = query.data; const presentation = statePresentation[value.state] ?? statePresentation.unknown; const check = value.last_check
  return <Card withBorder><Stack gap="sm"><Group justify="space-between"><Title order={3}>Состояние сервиса</Title><Badge color={presentation.color}>{presentation.label}</Badge></Group>{!check && value.state === 'unknown' && <Text c="dimmed">Проверок для сервиса пока нет.</Text>}{check && <>{!value.current && <Text c="dimmed">Показан последний известный результат.</Text>}<SimpleGrid cols={{ base: 1, sm: 3 }}><Metric label="HTTP-код" value={check.status_code || '—'} /><Metric label="Время ответа" value={`${check.response_time_ms} мс`} /><Metric label="Время проверки" value={formatDate(check.checked_at)} /></SimpleGrid>{check.error && <Alert color="red">{check.error}</Alert>}</>}</Stack></Card>
}

function Metrics({ serviceID, period }) {
  const query = useQuery({ queryKey: ['service-metrics', serviceID, period], queryFn: () => apiRequest(`/api/services/${serviceID}/metrics?period=${period}`), retry: false, refetchInterval: 30_000 })
  if (query.isPending) return <Card withBorder><Text c="dimmed">Загрузка показателей…</Text></Card>
  if (query.isError) return <Alert color="red">{query.error.message}</Alert>
  const data = query.data; if (data.total_checks === 0) return <Card withBorder><Title order={3}>Показатели мониторинга</Title><Text c="dimmed">За выбранный период проверок не было.</Text></Card>
  const duration = data.downtime_seconds >= 60 ? `${Math.floor(data.downtime_seconds / 60)} мин ${data.downtime_seconds % 60} с` : `${data.downtime_seconds} с`
  return <Card withBorder><Stack gap="sm"><Title order={3}>Показатели мониторинга</Title><SimpleGrid cols={{ base: 1, sm: 3 }}><Metric label="Успешные проверки" value={`${data.success_rate.toLocaleString('ru-RU', { maximumFractionDigits: 1 })}%`} /><Metric label="Среднее время ответа" value={data.has_response_metrics && Number.isFinite(data.average_response_ms) ? `${Math.round(data.average_response_ms)} мс` : '—'} /><Metric label="P95" value={data.has_response_metrics && Number.isFinite(data.p95_response_ms) ? `${data.p95_response_ms} мс` : '—'} /><Metric label="Инциденты" value={data.incident_count} /><Metric label="Время недоступности" value={duration} /></SimpleGrid></Stack></Card>
}

function Incidents({ serviceID }) {
  const [page, setPage] = useState(0)
  const query = useQuery({ queryKey: ['incidents', serviceID, page], queryFn: () => apiRequest(`/api/services/${serviceID}/incidents?limit=20&offset=${page * 20}`), placeholderData: keepPreviousData, retry: false, refetchInterval: 10_000 })
  const items = query.data?.items ?? []
  return <Card withBorder data-testid="incidents"><Stack gap="sm"><Title order={3}>Инциденты</Title>{query.isPending ? <Text c="dimmed">Загрузка инцидентов…</Text> : query.isError ? <Alert color="red">{query.error.message}</Alert> : !items.length ? <Text c="dimmed">Инцидентов пока не было.</Text> : items.map((item) => { const seconds = item.ended_at ? Math.max(0, Math.round((new Date(item.ended_at) - new Date(item.started_at)) / 1000)) : null; return <div key={item.id}><Group justify="space-between"><Text fw={600}>{item.ended_at ? 'Завершённый сбой' : 'Текущий сбой'}</Text><Badge color={item.ended_at ? 'gray' : 'red'}>{seconds === null ? 'Продолжается' : `${seconds} с`}</Badge></Group><Text size="sm" c="dimmed">Начало: {formatDate(item.started_at)}</Text>{item.ended_at && <Text size="sm" c="dimmed">Завершение: {formatDate(item.ended_at)}</Text>}</div> })}{query.isFetching && !query.isPending && <Text size="sm" c="dimmed">Обновление инцидентов…</Text>}<Group justify="space-between"><Button size="xs" variant="default" disabled={page === 0 || query.isFetching} onClick={() => setPage((value) => value - 1)}>Назад</Button><Button size="xs" variant="default" disabled={query.isPending || (page + 1) * 20 >= (query.data?.total ?? 0) || query.isFetching} onClick={() => setPage((value) => value + 1)}>Вперёд</Button></Group></Stack></Card>
}

export function ServiceDetailsPage({ serviceID, onNavigate }) {
  const client = useQueryClient(); const [editing, setEditing] = useState(null); const [period, setPeriod] = useState('24h')
  const service = useQuery({ queryKey: ['service', serviceID], queryFn: () => apiRequest(`/api/services/${serviceID}`), retry: false })
  const update = useMutation({ mutationFn: (payload) => apiRequest(`/api/services/${serviceID}`, jsonRequest('PUT', payload)), onSuccess: (value) => { client.setQueryData(['service', serviceID], value); client.invalidateQueries({ queryKey: ['monitoring-overview'] }); setEditing(null) } })
  const remove = useMutation({ mutationFn: () => apiRequest(`/api/services/${serviceID}`, { method: 'DELETE' }), onSuccess: () => { client.invalidateQueries({ queryKey: ['monitoring-overview'] }); onNavigate('/') } })
  if (service.isPending) return <Card withBorder><Text c="dimmed">Загрузка выбранного сервиса…</Text></Card>
  if (service.isError) return service.error.status === 404 ? <Card withBorder><Stack align="center"><Title order={2}>Страница не найдена</Title><Text c="dimmed">Страница не найдена или у вас нет доступа к этому сервису.</Text><Button onClick={() => onNavigate('/')}>К сервисам</Button></Stack></Card> : <Alert color="red">Не удалось загрузить сервис: {service.error.message}</Alert>
  const value = service.data
  return <Stack gap="lg"><Card withBorder><Stack gap="md"><Button variant="subtle" px={0} w="fit-content" onClick={() => onNavigate('/')}>← Назад к сервисам</Button><Group justify="space-between"><div><Title order={3}>{value.name}</Title><Text size="sm" c="dimmed">{value.url}</Text><Text size="sm" c="dimmed">Интервал: {value.interval_seconds} с · Тайм-аут: {value.timeout_seconds} с</Text></div><Badge color={value.enabled ? 'green' : 'gray'}>{value.enabled ? 'Включён' : 'Отключён'}</Badge></Group><Group><Button variant="default" onClick={() => setEditing(value)}>Изменить</Button><Button color="red" variant="subtle" loading={remove.isPending} onClick={() => { if (window.confirm(`Удалить сервис «${value.name}»?`)) remove.mutate() }}>Удалить</Button></Group></Stack></Card><Status serviceID={serviceID} /><Metrics serviceID={serviceID} period={period} /><HistoryChart serviceID={serviceID} period={period} /><HistoryTable serviceID={serviceID} period={period} onPeriodChange={setPeriod} /><Incidents serviceID={serviceID} /><Modal opened={Boolean(editing)} onClose={() => setEditing(null)} title="Изменить сервис">{update.error && <Alert mb="sm" color="red">{update.error.message}</Alert>}<ServiceForm key={editing?.id} service={editing} onSubmit={(payload) => update.mutate(payload)} pending={update.isPending} /></Modal></Stack>
}
