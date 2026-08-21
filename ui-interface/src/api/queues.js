import { request } from './client'

export const listQueues = (projectId) => request(`/api/projects/${projectId}/queues`)

export const createQueue = (projectId, input) =>
  request(`/api/projects/${projectId}/queues`, { method: 'POST', body: JSON.stringify(input) })

export const getQueue = (id) => request(`/api/queues/${id}`)

export const updateQueueConfig = (id, input) =>
  request(`/api/queues/${id}`, { method: 'PATCH', body: JSON.stringify(input) })

export const pauseQueue = (id) => request(`/api/queues/${id}/pause`, { method: 'POST' })

export const resumeQueue = (id) => request(`/api/queues/${id}/resume`, { method: 'POST' })

export const queueStats = (id) => request(`/api/queues/${id}/stats`)

export const deleteQueue = (id) => request(`/api/queues/${id}`, { method: 'DELETE' })
