import { qs, request } from './client'

export const getOverview = () => request('/api/dashboard/overview')

export const getRecentJobs = (filters = {}) => request(`/api/dashboard/recent-jobs${qs(filters)}`)

export const getThroughput = () => request('/api/dashboard/throughput')
