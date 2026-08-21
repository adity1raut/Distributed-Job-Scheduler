import { qs, request } from './client'

export const listJobs = (queueId, filters = {}) =>
  request(`/api/queues/${queueId}/jobs${qs(filters)}`)

export const submitJob = (queueId, input) =>
  request(`/api/queues/${queueId}/jobs`, { method: 'POST', body: JSON.stringify(input) })

export const getJob = (id) => request(`/api/jobs/${id}`)

export const retryJob = (id) => request(`/api/jobs/${id}/retry`, { method: 'POST' })

export const executionLogs = (executionId) => request(`/api/executions/${executionId}/logs`)
