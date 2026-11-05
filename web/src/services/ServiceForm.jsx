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
  const parsed = service ? new URL(service.url) : null
  const [form, setForm] = useState({ ...(service ?? emptyService), url: parsed ? `${parsed.host}${parsed.pathname}${parsed.search}` : '', protocol: parsed ? parsed.protocol.slice(0, -1) : 'https' })
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
    const value = form.url.trim()
    const fullURL = /^https?:\/\//i.test(value) ? value : `${form.protocol}://${value}`
    onSubmit({ name: form.name, url: fullURL, enabled: form.enabled, interval_seconds: intervalSeconds, timeout_seconds: timeoutSeconds })
  }

  return (
    <form onSubmit={submit}>
      <Stack gap="sm">
        <TextInput label="Название" placeholder="Основной сайт" value={form.name} onChange={(event) => setForm({ ...form, name: event.currentTarget.value })} required maxLength={100} />
        <Group align="end" wrap="nowrap"><Select label="Протокол" w={120} value={form.protocol} onChange={(value) => value && setForm({ ...form, protocol: value })} allowDeselect={false} data={[{ value: 'https', label: 'HTTPS' }, { value: 'http', label: 'HTTP' }]} /><TextInput flex={1} label="URL" placeholder="example.com/health" value={form.url} onChange={(event) => { const value = event.currentTarget.value; const match = value.match(/^(https?):\/\/(.*)$/i); setForm({ ...form, protocol: match ? match[1].toLowerCase() : form.protocol, url: match ? match[2] : value }) }} required /></Group>
        <Switch label="Проверки включены" checked={form.enabled} onChange={(event) => setForm({ ...form, enabled: event.currentTarget.checked })} />
        <Select label="Интервал проверки" data={intervals} value={String(form.interval_seconds)} onChange={(value) => value && setForm({ ...form, interval_seconds: Number(value) })} allowDeselect={false} />
        <NumberInput label="Тайм-аут, с" value={form.timeout_seconds} onChange={(value) => setForm({ ...form, timeout_seconds: Number(value) })} min={1} max={30} required />
        {error && <Alert color="red">{error}</Alert>}
        <Group justify="flex-end">
          <Button type="submit" loading={pending}>{submitLabel}</Button>
        </Group>
      </Stack>
    </form>
  )
}
