import { Alert, Badge, Button, Card, Group, Loader, Modal, Select, SimpleGrid, Stack, Table, Text, Title } from '@mantine/core'
import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import { apiRequest, jsonRequest } from '../api.js'
import { ServiceForm } from './ServiceForm.jsx'

function formatDate(value) {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '—' : date.toLocaleString('ru-RU')
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
  const [historyOffset, setHistoryOffset] = useState(0)
  const servicesQuery = useQuery({ queryKey: ['services'], queryFn: () => apiRequest('/api/services') })
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['services'] })
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

  const services = servicesQuery.data ?? []
  const currentServiceID = services.some((service) => service.id === selectedID) ? selectedID : services[0]?.id ?? null
  const statusQuery = useQuery({
    queryKey: ['service-status', currentServiceID],
    queryFn: () => apiRequest(`/api/services/${currentServiceID}/status`),
    enabled: Boolean(currentServiceID),
    retry: false,
    refetchInterval: 10_000,
  })
  const incidentsQuery = useQuery({
    queryKey: ['incidents', currentServiceID],
    queryFn: () => apiRequest(`/api/services/${currentServiceID}/incidents`),
    enabled: Boolean(currentServiceID),
    retry: false,
	refetchInterval: 10_000,
  })
  const historyQuery = useQuery({
    queryKey: ['check-history', currentServiceID, historyPeriod, historyOffset],
    queryFn: () => apiRequest(`/api/services/${currentServiceID}/checks?period=${historyPeriod}&limit=100&offset=${historyOffset}`),
    enabled: Boolean(currentServiceID),
    retry: false,
  })

  if (servicesQuery.isPending) return <Loader />
  if (servicesQuery.isError) return <Alert color="red">{servicesQuery.error.message}</Alert>

  const selectedService = services.find((service) => service.id === currentServiceID)
  return (
    <Stack gap="lg">
      <Group justify="space-between">
        <div>
          <Title order={2}>Сервисы</Title>
          <Text c="dimmed" size="sm">Добавляйте и настраивайте проверки.</Text>
        </div>
        <Button onClick={() => setCreating(true)}>Добавить сервис</Button>
      </Group>

      {services.length === 0 ? (
        <Card withBorder><Text>Сервисов пока нет. Добавьте первый сервис для мониторинга.</Text></Card>
      ) : (
        <Select label="Выбранный сервис" data={services.map((service) => ({ value: service.id, label: service.name }))} value={currentServiceID} onChange={(value) => { setSelectedID(value); setHistoryOffset(0) }} />
      )}

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
      {selectedService && <CheckHistory query={historyQuery} period={historyPeriod} onPeriodChange={(value) => { setHistoryPeriod(value); setHistoryOffset(0) }} onMore={() => setHistoryOffset((offset) => offset + 100)} />}
      {selectedService && <Incidents query={incidentsQuery} />}

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

function CheckHistory({ query, period, onPeriodChange, onMore }) {
  if (query.isPending) return <Card withBorder><Text c="dimmed">Загрузка истории проверок…</Text></Card>
  if (query.isError) return <Alert color="red">{query.error.message}</Alert>
  const history = query.data
  const checks = history.checks ?? []
  const chartData = [...checks].reverse().map((check) => ({ ...check, label: formatDate(check.checked_at) }))
  return <Card withBorder>
    <Stack gap="md">
      <Group justify="space-between" align="end">
        <Title order={3}>История проверок</Title>
        <Select aria-label="Период истории" value={period} onChange={(value) => value && onPeriodChange(value)} data={[{ value: '24h', label: '24 часа' }, { value: '7d', label: '7 дней' }, { value: '30d', label: '30 дней' }]} />
      </Group>
      {checks.length === 0 ? <Text c="dimmed">За выбранный период проверок не было.</Text> : <>
        <div style={{ width: '100%', height: 260 }}>
          <ResponsiveContainer>
            <LineChart data={chartData} margin={{ top: 10, right: 12, left: 0, bottom: 10 }}>
              <XAxis dataKey="label" minTickGap={60} tick={{ fontSize: 11 }} />
              <YAxis unit=" мс" width={70} tick={{ fontSize: 11 }} />
              <Tooltip labelFormatter={(value) => value} formatter={(value, _, item) => [`${value} мс`, item.payload.available ? 'Успешная проверка' : 'Неуспешная проверка']} />
              <Line type="monotone" dataKey="response_time_ms" stroke="#228be6" strokeWidth={2} connectNulls={false} dot={({ cx, cy, payload }) => <circle cx={cx} cy={cy} r="4" fill={payload.available ? '#40c057' : '#fa5252'} />} activeDot={{ r: 6 }} />
            </LineChart>
          </ResponsiveContainer>
        </div>
        <Table striped highlightOnHover withTableBorder>
          <Table.Thead><Table.Tr><Table.Th>Дата и время</Table.Th><Table.Th>Доступность</Table.Th><Table.Th>HTTP-код</Table.Th><Table.Th>Время ответа</Table.Th><Table.Th>Ошибка</Table.Th></Table.Tr></Table.Thead>
          <Table.Tbody>{checks.map((check, index) => <Table.Tr key={`${check.checked_at}-${index}`}><Table.Td>{formatDate(check.checked_at)}</Table.Td><Table.Td><Badge color={check.available ? 'green' : 'red'}>{check.available ? 'Успешно' : 'Ошибка'}</Badge></Table.Td><Table.Td>{check.status_code || '—'}</Table.Td><Table.Td>{check.response_time_ms} мс</Table.Td><Table.Td>{check.error || '—'}</Table.Td></Table.Tr>)}</Table.Tbody>
        </Table>
        {checks.length === history.limit && <Button variant="default" loading={query.isFetching} onClick={onMore}>Показать ещё</Button>}
      </>}
    </Stack>
  </Card>
}

function Incidents({ query }) {
  if (query.isPending) return <Card withBorder><Text c="dimmed">Загрузка инцидентов…</Text></Card>
  if (query.isError) return <Alert color="red">{query.error.message}</Alert>
  const incidents = query.data
  if (incidents.length === 0) return <Card withBorder><Text c="dimmed">Инцидентов пока не было.</Text></Card>
  const duration = (incident) => {
    if (!incident.ended_at) return 'Продолжается'
    const seconds = Math.max(0, Math.round((new Date(incident.ended_at) - new Date(incident.started_at)) / 1000))
    return `${seconds} с`
  }
  return <Card withBorder><Stack gap="sm"><Title order={3}>Инциденты</Title>{incidents.map((incident) => <div key={incident.id}><Group justify="space-between"><Text fw={600}>{incident.ended_at ? 'Завершённый сбой' : 'Текущий сбой'}</Text><Badge color={incident.ended_at ? 'gray' : 'red'}>{duration(incident)}</Badge></Group><Text size="sm" c="dimmed">Начало: {formatDate(incident.started_at)}</Text>{incident.ended_at && <Text size="sm" c="dimmed">Завершение: {formatDate(incident.ended_at)}</Text>}</div>)}</Stack></Card>
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
        <Group justify="space-between"><Title order={3}>Состояние сервиса</Title><Group gap="sm"><Badge color={presentation.color}>{presentation.label}</Badge><Button variant="default" size="xs" loading={query.isFetching} onClick={() => query.refetch()}>Обновить</Button></Group></Group>
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
