import { request } from './client'

export const getOverview = () => request('/api/dashboard/overview')
