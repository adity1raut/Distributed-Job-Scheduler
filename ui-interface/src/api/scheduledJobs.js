import { request } from './client'

export const listScheduledJobs = (queueId) => request(`/api/queues/${queueId}/scheduled-jobs`)

export const createScheduledJob = (queueId, input) =>
  request(`/api/queues/${queueId}/scheduled-jobs`, { method: 'POST', body: JSON.stringify(input) })

export const pauseScheduledJob = (id) => request(`/api/scheduled-jobs/${id}/pause`, { method: 'POST' })

export const resumeScheduledJob = (id) => request(`/api/scheduled-jobs/${id}/resume`, { method: 'POST' })
