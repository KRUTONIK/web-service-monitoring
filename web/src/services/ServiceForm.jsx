import { Button, Group, Stack, Switch, TextInput } from '@mantine/core'
import { useState } from 'react'

const emptyService = { name: '', url: '', enabled: true }

export function ServiceForm({ service, onSubmit, pending, submitLabel = 'Сохранить' }) {
  const [form, setForm] = useState(service ?? emptyService)

  function submit(event) {
    event.preventDefault()
    onSubmit(form)
  }

  return (
    <form onSubmit={submit}>
      <Stack gap="sm">
        <TextInput label="Название" placeholder="Основной сайт" value={form.name} onChange={(event) => setForm({ ...form, name: event.currentTarget.value })} required maxLength={100} />
        <TextInput label="URL" placeholder="https://example.com" value={form.url} onChange={(event) => setForm({ ...form, url: event.currentTarget.value })} required />
        <Switch label="Проверки включены" checked={form.enabled} onChange={(event) => setForm({ ...form, enabled: event.currentTarget.checked })} />
        <Group justify="flex-end">
          <Button type="submit" loading={pending}>{submitLabel}</Button>
        </Group>
      </Stack>
    </form>
  )
}
