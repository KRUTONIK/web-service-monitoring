import { Alert, Badge, Button, Card, Group, Loader, Modal, Select, SimpleGrid, Stack, Table, Text, TextInput, Title } from '@mantine/core'
import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import { apiRequest, jsonRequest } from '../api.js'
import { ServiceForm } from './ServiceForm.jsx'

function formatDate(value) {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '—' : date.toLocaleString('ru-RU')
}

function formatChartDate(value) {
  return new Date(value).toLocaleString('ru-RU', { hour: '2-digit', minute: '2-digit', second: '2-digit', day: '2-digit', month: '2-digit' })
}

const statePresentation = {
  available: { label: 'Доступен', color: 'green' },
  unavailable: { label: 'Недоступен', color: 'red' },
  unknown: { label: 'Нет актуальных данных', color: 'yellow' },
  paused: { label: 'Мониторинг отключён', color: 'gray' },
}

export function ServiceDashboard() {
  const queryClient = useQueryClient()
  const [selectedID, setSelectedID] = useState(null)
  const [editing, setEditing] = useState(null)
  const [creating, setCreating] = useState(false)
  const [historyPeriod, setHistoryPeriod] = useState('24h')
  const [historyPage, setHistoryPage] = useState(0)
  const [historyPageSize, setHistoryPageSize] = useState(5)
  const [searchInput, setSearchInput] = useState('')
  const [search, setSearch] = useState('')
  const [statusFilter, setStatusFilter] = useState('')
  const [overviewPage, setOverviewPage] = useState(0)
  const [overviewPageSize, setOverviewPageSize] = useState(20)
  const [incidentsPage, setIncidentsPage] = useState(0)
  useEffect(() => { const timer = setTimeout(() => { setSearch(searchInput); setOverviewPage(0) }, 300); return () => clearTimeout(timer) }, [searchInput])
  const overviewQuery = useQuery({ queryKey: ['monitoring-overview', search, statusFilter, overviewPage, overviewPageSize], queryFn: () => apiRequest(`/api/monitoring/overview?search=${encodeURIComponent(search)}&status=${statusFilter}&limit=${overviewPageSize}&offset=${overviewPage * overviewPageSize}`), retry: false, refetchInterval: 10_000 })
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['monitoring-overview'] })
  const createMutation = useMutation({
    mutationFn: (service) => apiRequest('/api/services', jsonRequest('POST', service)),
    onSuccess: async (service) => {
      await invalidate()
      setSelectedID(service.id)
      setCreating(false)
    },
  })
  const updateMutation = useMutation({
    mutationFn: ({ id, service }) => apiRequest(`/api/services/${id}`, jsonRequest('PUT', service)),
    onSuccess: async (service) => {
      await invalidate()
      setSelectedID(service.id)
      setEditing(null)
    },
  })
  const deleteMutation = useMutation({
    mutationFn: (id) => apiRequest(`/api/services/${id}`, { method: 'DELETE' }),
    onSuccess: async (_, id) => {
      await invalidate()
      if (selectedID === id) setSelectedID(null)
    },
  })

  const services = overviewQuery.data?.services ?? []
  const currentServiceID = services.some((service) => service.id === selectedID) ? selectedID : services[0]?.id ?? null
  const statusQuery = useQuery({
    queryKey: ['service-status', currentServiceID],
    queryFn: () => apiRequest(`/api/services/${currentServiceID}/status`),
    enabled: Boolean(currentServiceID),
    retry: false,
    refetchInterval: 10_000,
  })
  const incidentsQuery = useQuery({
    queryKey: ['incidents', currentServiceID, incidentsPage],
    queryFn: () => apiRequest(`/api/services/${currentServiceID}/incidents?limit=20&offset=${incidentsPage * 20}`),
    enabled: Boolean(currentServiceID),
    retry: false,
	refetchInterval: 10_000,
  })
  const chartQuery = useQuery({
    queryKey: ['check-history-chart', currentServiceID, historyPeriod],
    queryFn: () => apiRequest(`/api/services/${currentServiceID}/checks?period=${historyPeriod}&chart=true&limit=10000&offset=0`),
    enabled: Boolean(currentServiceID),
    retry: false,
    refetchInterval: historyPeriod === '24h' ? 10_000 : 30_000,
  })
  const historyQuery = useQuery({
    queryKey: ['check-history-page', currentServiceID, historyPeriod, historyPage, historyPageSize],
    queryFn: () => apiRequest(`/api/services/${currentServiceID}/checks?period=${historyPeriod}&limit=${historyPageSize + 1}&offset=${historyPage * historyPageSize}`),
    enabled: Boolean(currentServiceID),
    retry: false,
    refetchInterval: 10_000,
  })
  const metricsQuery = useQuery({ queryKey: ['service-metrics', currentServiceID, historyPeriod], queryFn: () => apiRequest(`/api/services/${currentServiceID}/metrics?period=${historyPeriod}`), enabled: Boolean(currentServiceID), retry: false, refetchInterval: 30_000 })

  if (overviewQuery.isPending) return <Loader />
  if (overviewQuery.isError) return <Alert color="red">{overviewQuery.error.message}</Alert>

  const selectedService = services.find((service) => service.id === currentServiceID)
  return (
    <Stack gap="lg">
      <Group justify="space-between">
        <div>
          <Title order={2}>Мониторинг сервисов</Title>
          <Text c="dimmed" size="sm">Текущее состояние всех ваших сервисов.</Text>
        </div>
        <Button onClick={() => setCreating(true)}>Добавить сервис</Button>
      </Group>

      <OverviewCounters summary={overviewQuery.data.summary} />
      <Group grow align="end"><TextInput label="Поиск" value={searchInput} onChange={(event) => setSearchInput(event.currentTarget.value)} placeholder="Название или URL" /><Select label="Состояние" value={statusFilter} onChange={(value) => { setStatusFilter(value ?? ''); setOverviewPage(0) }} data={[{ value: '', label: 'Все состояния' }, { value: 'available', label: 'Доступные' }, { value: 'unavailable', label: 'Недоступные' }, { value: 'unknown', label: 'Нет данных' }, { value: 'paused', label: 'Отключённые' }]} /><Select label="На странице" value={String(overviewPageSize)} onChange={(value) => { setOverviewPageSize(Number(value)); setOverviewPage(0) }} data={['10', '20', '50']} /></Group>
      {services.length === 0 ? (
        <Card withBorder><Text>Сервисов пока нет. Добавьте первый сервис для мониторинга.</Text></Card>
      ) : (
        <OverviewTable services={services} onSelect={(id) => { setSelectedID(id); setHistoryPage(0); setIncidentsPage(0) }} />
      )}
      <Group justify="space-between"><Button aria-label="Предыдущая страница сервисов" variant="default" disabled={overviewPage === 0} onClick={() => setOverviewPage((page) => page - 1)}>Назад</Button><Text size="sm">{overviewQuery.data.pagination.total} сервисов</Text><Button aria-label="Следующая страница сервисов" variant="default" disabled={(overviewPage + 1) * overviewPageSize >= overviewQuery.data.pagination.total} onClick={() => setOverviewPage((page) => page + 1)}>Вперёд</Button></Group>

      {selectedService && (
        <Card withBorder>
          <Stack gap="md">
            <Group justify="space-between">
              <div>
                <Title order={3}>{selectedService.name}</Title>
                <Text size="sm" c="dimmed">{selectedService.url}</Text>
                <Text size="sm" c="dimmed">Интервал: {selectedService.interval_seconds} с · Тайм-аут: {selectedService.timeout_seconds} с</Text>
              </div>
              <Badge color={selectedService.enabled ? 'green' : 'gray'}>{selectedService.enabled ? 'Включён' : 'Отключён'}</Badge>
            </Group>
            <Group>
              <Button variant="default" onClick={() => setEditing(selectedService)}>Изменить</Button>
              <Button color="red" variant="subtle" loading={deleteMutation.isPending} onClick={() => {
                if (window.confirm(`Удалить сервис «${selectedService.name}»?`)) deleteMutation.mutate(selectedService.id)
              }}>Удалить</Button>
            </Group>
          </Stack>
        </Card>
      )}

      {selectedService && <ServiceStatus query={statusQuery} />}
      {selectedService && <MonitoringMetrics query={metricsQuery} />}
      {selectedService && <CheckHistory chartQuery={chartQuery} query={historyQuery} period={historyPeriod} page={historyPage} pageSize={historyPageSize} onPeriodChange={(value) => { setHistoryPeriod(value); setHistoryPage(0) }} onPageChange={setHistoryPage} onPageSizeChange={(value) => { setHistoryPageSize(Number(value)); setHistoryPage(0) }} />}
      {selectedService && <Incidents query={incidentsQuery} page={incidentsPage} onPageChange={setIncidentsPage} />}

      <Modal opened={creating} onClose={() => setCreating(false)} title="Новый сервис">
        {createMutation.error && <Alert mb="sm" color="red">{createMutation.error.message}</Alert>}
        <ServiceForm onSubmit={(service) => createMutation.mutate(service)} pending={createMutation.isPending} submitLabel="Добавить" />
      </Modal>
      <Modal opened={Boolean(editing)} onClose={() => setEditing(null)} title="Изменить сервис">
        {updateMutation.error && <Alert mb="sm" color="red">{updateMutation.error.message}</Alert>}
        <ServiceForm key={editing?.id} service={editing} onSubmit={(service) => updateMutation.mutate({ id: editing.id, service })} pending={updateMutation.isPending} />
      </Modal>
    </Stack>
  )
}

function OverviewCounters({ summary }) {
  const values = [{ label: 'Доступны', value: summary.available, color: 'green' }, { label: 'Недоступны', value: summary.unavailable, color: 'red' }, { label: 'Нет данных', value: summary.unknown, color: 'gray' }, { label: 'Отключены', value: summary.paused, color: 'gray' }]
  return <SimpleGrid cols={{ base: 2, sm: 4 }}>{values.map((item) => <Card key={item.label} withBorder><Text size="sm" c="dimmed">{item.label}</Text><Text fw={700} c={item.color} size="xl">{item.value}</Text></Card>)}</SimpleGrid>
}

function OverviewTable({ services, onSelect }) {
  return <Card withBorder p={0}><Table highlightOnHover><Table.Thead><Table.Tr><Table.Th>Сервис</Table.Th><Table.Th>Состояние</Table.Th><Table.Th>HTTP-код</Table.Th><Table.Th>Время ответа</Table.Th><Table.Th>Последняя проверка</Table.Th></Table.Tr></Table.Thead><Table.Tbody>{services.map((item) => <Table.Tr key={item.id} style={{ cursor: 'pointer' }} onClick={() => onSelect(item.id)}><Table.Td><Text fw={600}>{item.name}</Text><Text size="sm" c="dimmed">{item.url}</Text></Table.Td><Table.Td><Badge color={statePresentation[item.status.state].color}>{statePresentation[item.status.state].label}</Badge></Table.Td><Table.Td>{item.status.last_check?.status_code || '—'}</Table.Td><Table.Td>{item.status.last_check ? `${item.status.last_check.response_time_ms} мс` : '—'}</Table.Td><Table.Td>{item.status.last_check ? formatDate(item.status.last_check.checked_at) : '—'}</Table.Td></Table.Tr>)}</Table.Tbody></Table></Card>
}

function MonitoringMetrics({ query }) {
  if (query.isPending) return <Card withBorder><Text c="dimmed">Загрузка показателей…</Text></Card>
  if (query.isError) return <Alert color="red">{query.error.message}</Alert>
  const metrics = query.data
  if (metrics.total_checks === 0) return <Card withBorder><Title order={3}>Показатели мониторинга</Title><Text c="dimmed">За выбранный период проверок не было.</Text></Card>
  const duration = metrics.downtime_seconds >= 60 ? `${Math.floor(metrics.downtime_seconds / 60)} мин ${metrics.downtime_seconds % 60} с` : `${metrics.downtime_seconds} с`
  const responseValue = metrics.has_response_metrics ? `${Math.round(metrics.average_response_ms)} мс` : '—'
  const p95Value = metrics.has_response_metrics ? `${metrics.p95_response_ms} мс` : '—'
  return <Card withBorder><Stack gap="sm"><Title order={3}>Показатели мониторинга</Title><SimpleGrid cols={{ base: 1, sm: 3 }}><Metric label="Успешные проверки" value={`${metrics.success_rate.toLocaleString('ru-RU', { maximumFractionDigits: 1 })}%`} /><Metric label="Среднее время ответа" value={responseValue} /><Metric label="P95" value={p95Value} /><Metric label="Инциденты" value={metrics.incident_count} /><Metric label="Время недоступности" value={duration} /></SimpleGrid></Stack></Card>
}

function CheckHistory({ chartQuery, query, period, page, pageSize, onPeriodChange, onPageChange, onPageSizeChange }) {
  if (query.isPending) return <Card withBorder><Text c="dimmed">Загрузка истории проверок…</Text></Card>
  if (query.isError) return <Alert color="red">{query.error.message}</Alert>
  const history = query.data
  const checks = history.checks ?? []
  const pageChecks = checks.slice(0, pageSize)
  const hasNextPage = checks.length > pageSize
  const chartData = (chartQuery.data?.checks ?? []).map((check) => ({ ...check, checkedAt: new Date(check.checked_at).getTime() })).filter((check) => !Number.isNaN(check.checkedAt)).sort((left, right) => left.checkedAt - right.checkedAt).filter((check, index, values) => index === 0 || check.checkedAt !== values[index - 1].checkedAt)
  return <Card withBorder>
    <Stack gap="md">
      <Group justify="space-between" align="end">
        <Title order={3}>История проверок</Title>
        <Select aria-label="Период истории" value={period} onChange={(value) => value && onPeriodChange(value)} data={[{ value: '24h', label: '24 часа' }, { value: '7d', label: '7 дней' }, { value: '30d', label: '30 дней' }]} />
      </Group>
      {chartQuery.isPending ? <Text c="dimmed">Загрузка графика…</Text> : chartData.length > 0 && <>
        <div style={{ width: '100%', height: 260 }}>
          <ResponsiveContainer>
            <LineChart data={chartData} margin={{ top: 10, right: 12, left: 0, bottom: 10 }}>
              <XAxis dataKey="checkedAt" type="number" scale="time" domain={['dataMin', 'dataMax']} minTickGap={60} tickFormatter={formatChartDate} tick={{ fontSize: 11 }} />
              <YAxis unit=" мс" width={70} tick={{ fontSize: 11 }} />
              <Tooltip content={<HistoryTooltip />} />
              <Line type="linear" dataKey="response_time_ms" stroke="#228be6" strokeWidth={2} connectNulls={false} dot={({ cx, cy, payload }) => <circle data-testid="history-point" cx={cx} cy={cy} r="4" fill={payload.available ? '#40c057' : '#fa5252'} />} activeDot={{ r: 6 }} />
            </LineChart>
          </ResponsiveContainer>
        </div>
      </>}
      {pageChecks.length === 0 ? <Text c="dimmed">За выбранный период проверок не было.</Text> : <>
        <Table striped highlightOnHover withTableBorder>
          <Table.Thead><Table.Tr><Table.Th>Дата и время</Table.Th><Table.Th>Доступность</Table.Th><Table.Th>HTTP-код</Table.Th><Table.Th>Время ответа</Table.Th><Table.Th>Ошибка</Table.Th></Table.Tr></Table.Thead>
          <Table.Tbody>{pageChecks.map((check, index) => <Table.Tr key={`${check.checked_at}-${index}`}><Table.Td>{formatDate(check.checked_at)}</Table.Td><Table.Td><Badge color={check.available ? 'green' : 'red'}>{check.available ? 'Успешно' : 'Ошибка'}</Badge></Table.Td><Table.Td>{check.status_code || '—'}</Table.Td><Table.Td>{check.response_time_ms} мс</Table.Td><Table.Td>{check.error || '—'}</Table.Td></Table.Tr>)}</Table.Tbody>
        </Table>
        <Group justify="space-between">
          <Select aria-label="Записей на странице" value={String(pageSize)} onChange={(value) => value && onPageSizeChange(value)} data={['5', '10', '25', '50', '100']} />
          <Group gap="xs"><Button variant="default" size="xs" disabled={page === 0 || query.isFetching} onClick={() => onPageChange(page - 1)}>Назад</Button><Text size="sm">Страница {page + 1}</Text><Button variant="default" size="xs" disabled={!hasNextPage || query.isFetching} onClick={() => onPageChange(page + 1)}>Вперёд</Button></Group>
        </Group>
      </>}
    </Stack>
  </Card>
}

function HistoryTooltip({ active, payload }) {
  if (!active || !payload?.[0]) return null
  const check = payload[0].payload
  return <div style={{ background: 'white', border: '1px solid #dee2e6', borderRadius: 4, padding: 8 }}>
    <Text size="sm">{formatDate(check.checked_at)}</Text>
    <Text size="sm">{check.available ? 'Успешная проверка' : 'Неуспешная проверка'}</Text>
    <Text size="sm">HTTP-код: {check.status_code || '—'}</Text>
    <Text size="sm">Время ответа: {check.response_time_ms} мс</Text>
    {check.error && <Text size="sm" c="red">{check.error}</Text>}
  </div>
}

function Incidents({ query, page, onPageChange }) {
  if (query.isPending) return <Card withBorder><Text c="dimmed">Загрузка инцидентов…</Text></Card>
  if (query.isError) return <Alert color="red">{query.error.message}</Alert>
  const incidents = query.data.items ?? []
  if (incidents.length === 0) return <Card withBorder><Text c="dimmed">Инцидентов пока не было.</Text></Card>
  const duration = (incident) => {
    if (!incident.ended_at) return 'Продолжается'
    const seconds = Math.max(0, Math.round((new Date(incident.ended_at) - new Date(incident.started_at)) / 1000))
    return `${seconds} с`
  }
  return <Card withBorder><Stack gap="sm"><Title order={3}>Инциденты</Title>{incidents.map((incident) => <div key={incident.id}><Group justify="space-between"><Text fw={600}>{incident.ended_at ? 'Завершённый сбой' : 'Текущий сбой'}</Text><Badge color={incident.ended_at ? 'gray' : 'red'}>{duration(incident)}</Badge></Group><Text size="sm" c="dimmed">Начало: {formatDate(incident.started_at)}</Text>{incident.ended_at && <Text size="sm" c="dimmed">Завершение: {formatDate(incident.ended_at)}</Text>}</div>)}<Group justify="space-between"><Button size="xs" variant="default" disabled={page === 0} onClick={() => onPageChange(page - 1)}>Назад</Button><Button size="xs" variant="default" disabled={(page + 1) * 20 >= query.data.total} onClick={() => onPageChange(page + 1)}>Вперёд</Button></Group></Stack></Card>
}

function ServiceStatus({ query }) {
  if (query.isPending) return <div role="status" aria-label="Загрузка последней проверки"><Loader size="sm" /></div>
  if (query.isError) {
    return <Alert color="red">{query.error.message}</Alert>
  }
  const status = query.data
  const presentation = statePresentation[status.state] ?? statePresentation.unknown
  const check = status.last_check
  return (
    <Card withBorder>
      <Stack gap="sm">
        <Group justify="space-between"><Title order={3}>Состояние сервиса</Title><Badge color={presentation.color}>{presentation.label}</Badge></Group>
        {!check && status.state === 'unknown' && <Text c="dimmed">Проверок для сервиса пока нет.</Text>}
        {check && <>
          {!status.current && <Text c="dimmed">Показан последний известный результат.</Text>}
          <SimpleGrid cols={{ base: 1, sm: 3 }}>
            <Metric label="HTTP-код" value={check.status_code || '—'} />
            <Metric label="Время ответа" value={`${check.response_time_ms} мс`} />
            <Metric label="Время проверки" value={formatDate(check.checked_at)} />
          </SimpleGrid>
          {check.error && <Alert color="red">{check.error}</Alert>}
        </>}
      </Stack>
    </Card>
  )
}

function Metric({ label, value }) {
  return <div><Text size="sm" c="dimmed">{label}</Text><Text fw={600}>{value}</Text></div>
}
