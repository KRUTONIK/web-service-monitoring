import { Alert, Modal } from '@mantine/core'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { apiRequest, jsonRequest } from '../api.js'
import { ServiceDetailsPage } from './ServiceDetailsPage.jsx'
import { ServiceForm } from './ServiceForm.jsx'
import { ServicesPage } from './ServicesPage.jsx'

// The route coordinator owns only service creation; data queries remain local to
// their page or dashboard block so a refresh never blanks unrelated content.
export function ServiceDashboard({ serviceID, onNavigate }) {
  const client = useQueryClient()
  const [creating, setCreating] = useState(false)
  const create = useMutation({
    mutationFn: (payload) => apiRequest('/api/services', jsonRequest('POST', payload)),
    onSuccess: (service) => { client.invalidateQueries({ queryKey: ['monitoring-overview'] }); setCreating(false); onNavigate(`/${service.id}`) },
  })
  if (serviceID) return <ServiceDetailsPage serviceID={serviceID} onNavigate={onNavigate} />
  return <><ServicesPage onNavigate={onNavigate} onCreate={() => setCreating(true)} /><Modal opened={creating} onClose={() => setCreating(false)} title="Новый сервис">{create.error && <Alert mb="sm" color="red">{create.error.message}</Alert>}<ServiceForm onSubmit={(payload) => create.mutate(payload)} pending={create.isPending} submitLabel="Добавить" /></Modal></>
}
