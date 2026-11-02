import { Alert, Anchor, Button, Paper, PasswordInput, Stack, Text, TextInput, Title } from '@mantine/core'
import { useState } from 'react'

export function AuthPage({ onSubmit, pending, error }) {
  const [mode, setMode] = useState('login')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')

  function submit(event) {
    event.preventDefault()
    onSubmit({ mode, email, password })
  }

  const registration = mode === 'register'
  return (
    <Paper component="form" shadow="sm" p="xl" radius="md" withBorder onSubmit={submit}>
      <Stack gap="md">
        <div>
          <Title order={2}>{registration ? 'Создать аккаунт' : 'Войти'}</Title>
          <Text c="dimmed" size="sm">Управляйте проверками своих веб-сервисов.</Text>
        </div>
        {error && <Alert color="red">{error}</Alert>}
        <TextInput label="Электронная почта" type="email" value={email} onChange={(event) => setEmail(event.currentTarget.value)} required />
        <PasswordInput label="Пароль" description={registration ? 'Не менее 10 символов' : undefined} value={password} onChange={(event) => setPassword(event.currentTarget.value)} required />
        <Button type="submit" loading={pending}>{registration ? 'Зарегистрироваться' : 'Войти'}</Button>
        <Text size="sm" ta="center">
          {registration ? 'Уже есть аккаунт? ' : 'Нет аккаунта? '}
          <Anchor component="button" type="button" onClick={() => setMode(registration ? 'login' : 'register')}>
            {registration ? 'Войти' : 'Зарегистрироваться'}
          </Anchor>
        </Text>
      </Stack>
    </Paper>
  )
}
