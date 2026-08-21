import { request } from './client'

export const listWorkers = () => request('/api/workers')

export const workerHeartbeats = (workerId, limit = 50) =>
  request(`/api/workers/${workerId}/heartbeats?limit=${limit}`)
