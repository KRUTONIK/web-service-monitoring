import { Alert, Badge, Button, Card, Group, Select, Stack, Table, Text, Title } from '@mantine/core'
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { apiRequest } from '../api.js'
import { formatDate } from './shared.js'

const periods = [{ value: '1h', label: '1 час' }, { value: '6h', label: '6 часов' }, { value: '24h', label: '24 часа' }, { value: '7d', label: '7 дней' }, { value: '30d', label: '30 дней' }]

export function HistoryTable({ serviceID, period, onPeriodChange }) {
  const [page, setPage] = useState(0)
  const [pageSize, setPageSize] = useState(5)
  const query = useQuery({
    queryKey: ['check-history-page', serviceID, period, page, pageSize],
    queryFn: () => apiRequest(`/api/services/${serviceID}/checks?period=${period}&limit=${pageSize + 1}&offset=${page * pageSize}`),
    enabled: Boolean(serviceID), placeholderData: keepPreviousData, retry: false, refetchInterval: 10_000,
  })
  const checks = query.data?.checks ?? []
  const pageChecks = checks.slice(0, pageSize)
  const changePeriod = (value) => { if (value) { setPage(0); onPeriodChange(value) } }
  const changePageSize = (value) => { if (value) { setPageSize(Number(value)); setPage(0) } }
  return <Card withBorder data-testid="history-table"><Stack gap="md">
    <Group justify="space-between" align="end"><Title order={3}>История проверок</Title><Select aria-label="Период истории" value={period} onChange={changePeriod} allowDeselect={false} data={periods} /></Group>
    {query.isError ? <Alert color="red">{query.error.message}</Alert> : query.isPending ? <Text c="dimmed">Загрузка истории проверок…</Text> : pageChecks.length === 0 ? <Text c="dimmed">За выбранный период проверок не было.</Text> : <>
      <Table striped highlightOnHover withTableBorder><Table.Thead><Table.Tr><Table.Th>Дата и время</Table.Th><Table.Th>Доступность</Table.Th><Table.Th>HTTP-код</Table.Th><Table.Th>Время ответа</Table.Th><Table.Th>Ошибка</Table.Th></Table.Tr></Table.Thead><Table.Tbody>{pageChecks.map((check, index) => <Table.Tr key={`${check.checked_at}-${index}`}><Table.Td>{formatDate(check.checked_at)}</Table.Td><Table.Td><Badge color={check.available ? 'green' : 'red'}>{check.available ? 'Успешно' : 'Ошибка'}</Badge></Table.Td><Table.Td>{check.status_code || '—'}</Table.Td><Table.Td>{check.response_time_ms} мс</Table.Td><Table.Td>{check.error || '—'}</Table.Td></Table.Tr>)}</Table.Tbody></Table>
      <Group justify="space-between"><Select aria-label="Записей на странице" value={String(pageSize)} onChange={changePageSize} data={['5', '10', '25', '50', '100']} /><Group gap="xs"><Button variant="default" size="xs" disabled={page === 0 || query.isFetching} onClick={() => setPage((value) => value - 1)}>Назад</Button><Text size="sm">Страница {page + 1}</Text><Button variant="default" size="xs" disabled={checks.length <= pageSize || query.isFetching} onClick={() => setPage((value) => value + 1)}>Вперёд</Button></Group></Group>
    </>}
  </Stack></Card>
}
