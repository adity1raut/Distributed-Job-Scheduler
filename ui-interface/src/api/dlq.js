import { request } from './client'

export const listDeadLetters = (queueId, limit = 50) =>
  request(`/api/queues/${queueId}/dlq?limit=${limit}`)

export const replayDeadLetter = (id) => request(`/api/dlq/${id}/replay`, { method: 'POST' })
