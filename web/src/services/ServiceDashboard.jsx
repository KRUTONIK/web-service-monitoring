import { Alert, Badge, Button, Card, Group, Loader, Modal, Select, SimpleGrid, Stack, Text, Title } from '@mantine/core'
import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
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
        <Select label="Выбранный сервис" data={services.map((service) => ({ value: service.id, label: service.name }))} value={currentServiceID} onChange={setSelectedID} />
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
