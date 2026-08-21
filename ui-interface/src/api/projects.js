import { request } from './client'

export const listProjects = () => request('/api/projects')

export const createProject = (name) =>
  request('/api/projects', { method: 'POST', body: JSON.stringify({ name }) })

export const getProject = (id) => request(`/api/projects/${id}`)

export const deleteProject = (id) => request(`/api/projects/${id}`, { method: 'DELETE' })
