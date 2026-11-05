import { Alert, AppShell, Button, Container, Group, Loader, Paper, Stack, Text, Title } from '@mantine/core'
import { useEffect, useState } from 'react'
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
  const [path, setPath] = useState(window.location.pathname)
  useEffect(() => { const listener = () => setPath(window.location.pathname); window.addEventListener('popstate', listener); return () => window.removeEventListener('popstate', listener) }, [])
  const navigate = (to) => { window.history.pushState({}, '', to); setPath(to) }
  const detailID = path.match(/^\/([^/]+)$/)?.[1]
  const routeFound = path === '/' || Boolean(detailID)

  if (userQuery.isPending) return <Loader />
  if (userQuery.isError) return <Alert color="red">{userQuery.error.message}</Alert>

  return <AppShell header={{ height: 64 }} footer={{ height: 42 }} padding="md">
    <AppShell.Header><Container size="xl" h="100%"><Group justify="space-between" h="100%">
          <div>
            <Title order={2}>Мониторинг веб-сервисов</Title>
          </div>
          {userQuery.data && <Group><Text c="dimmed" size="sm">{userQuery.data.email}</Text><Button variant="default" loading={logoutMutation.isPending} onClick={() => logoutMutation.mutate()}>Выйти</Button></Group>}
        </Group></Container></AppShell.Header>
    <AppShell.Main><Container size="xl" py="xl">
        {userQuery.data && routeFound ? <ServiceDashboard serviceID={detailID} onNavigate={navigate} /> : userQuery.data ? <NotFound onNavigate={navigate} /> : (
          <Paper maw={440} mx="auto" w="100%"><AuthPage onSubmit={(credentials) => authMutation.mutate(credentials)} pending={authMutation.isPending} error={authMutation.error?.message} /></Paper>
        )}
    </Container></AppShell.Main><AppShell.Footer><Container size="xl" h="100%"><Group justify="space-between" wrap="wrap" py="sm"><Text size="xs" c="dimmed">Мониторинг веб-сервисов</Text><Text size="xs" c="dimmed">Покладов Никита</Text></Group></Container></AppShell.Footer></AppShell>
}

function NotFound({ onNavigate }) {
  return <Paper maw={560} mx="auto" p="xl"><Stack align="center"><Title order={2}>Страница не найдена</Title><Text ta="center" c="dimmed">Страница не найдена или у вас нет доступа к этому сервису.</Text><Button onClick={() => onNavigate('/')}>К сервисам</Button></Stack></Paper>
}

export default App
