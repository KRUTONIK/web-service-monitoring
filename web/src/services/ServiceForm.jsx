import { Alert, Button, Group, NumberInput, Select, Stack, Switch, TextInput } from '@mantine/core'
import { useState } from 'react'

const emptyService = { name: '', url: '', enabled: true, interval_seconds: 60, timeout_seconds: 5 }
const intervals = [
  { value: '10', label: '10 с' },
  { value: '30', label: '30 с' },
  { value: '60', label: '1 мин' },
  { value: '300', label: '5 мин' },
]

export function ServiceForm({ service, onSubmit, pending, submitLabel = 'Сохранить' }) {
  const [form, setForm] = useState(service ?? emptyService)
  const [error, setError] = useState('')

  function submit(event) {
    event.preventDefault()
    const intervalSeconds = Number(form.interval_seconds)
    const timeoutSeconds = Number(form.timeout_seconds)
    if (timeoutSeconds < 1 || timeoutSeconds > 30 || timeoutSeconds >= intervalSeconds) {
      setError('Тайм-аут должен быть от 1 до 30 секунд и меньше интервала проверки.')
      return
    }
    setError('')
    onSubmit({ name: form.name, url: form.url, enabled: form.enabled, interval_seconds: intervalSeconds, timeout_seconds: timeoutSeconds })
  }

  return (
    <form onSubmit={submit}>
      <Stack gap="sm">
        <TextInput label="Название" placeholder="Основной сайт" value={form.name} onChange={(event) => setForm({ ...form, name: event.currentTarget.value })} required maxLength={100} />
        <TextInput label="URL" placeholder="https://example.com" value={form.url} onChange={(event) => setForm({ ...form, url: event.currentTarget.value })} required />
        <Switch label="Проверки включены" checked={form.enabled} onChange={(event) => setForm({ ...form, enabled: event.currentTarget.checked })} />
        <Select label="Интервал проверки" data={intervals} value={String(form.interval_seconds)} onChange={(value) => setForm({ ...form, interval_seconds: Number(value) })} allowDeselect={false} />
        <NumberInput label="Тайм-аут, с" value={form.timeout_seconds} onChange={(value) => setForm({ ...form, timeout_seconds: Number(value) })} min={1} max={30} required />
        {error && <Alert color="red">{error}</Alert>}
        <Group justify="flex-end">
          <Button type="submit" loading={pending}>{submitLabel}</Button>
        </Group>
      </Stack>
    </form>
  )
}
