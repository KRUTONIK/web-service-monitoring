import { Alert, Button, Container, Group, Loader, Paper, Stack, Text, Title } from '@mantine/core'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiRequest, jsonRequest } from './api.js'
import { AuthPage } from './auth/AuthPage.jsx'
import { ServiceDashboard } from './services/ServiceDashboard.jsx'

function currentUser() {
  return apiRequest('/api/auth/me').catch((error) => {
    if (error.status === 401) return null
    throw error
  })
}

function App() {
  const queryClient = useQueryClient()
  const userQuery = useQuery({ queryKey: ['current-user'], queryFn: currentUser, retry: false })
  const authMutation = useMutation({
    mutationFn: ({ mode, email, password }) => apiRequest(`/api/auth/${mode === 'register' ? 'register' : 'login'}`, jsonRequest('POST', { email, password })),
    onSuccess: (user) => queryClient.setQueryData(['current-user'], user),
  })
  const logoutMutation = useMutation({
    mutationFn: () => apiRequest('/api/auth/logout', { method: 'POST' }),
    onSuccess: () => queryClient.setQueryData(['current-user'], null),
  })

  if (userQuery.isPending) return <Loader />
  if (userQuery.isError) return <Alert color="red">{userQuery.error.message}</Alert>

  return (
    <Container size="md" py="xl">
      <Stack gap="xl">
        <Group justify="space-between">
          <div>
            <Title order={1}>Мониторинг веб-сервисов</Title>
            {userQuery.data && <Text c="dimmed" size="sm">{userQuery.data.email}</Text>}
          </div>
          {userQuery.data && <Button variant="default" loading={logoutMutation.isPending} onClick={() => logoutMutation.mutate()}>Выйти</Button>}
        </Group>
        {userQuery.data ? <ServiceDashboard /> : (
          <Paper maw={440} mx="auto" w="100%"><AuthPage onSubmit={(credentials) => authMutation.mutate(credentials)} pending={authMutation.isPending} error={authMutation.error?.message} /></Paper>
        )}
      </Stack>
    </Container>
  )
}

export default App
