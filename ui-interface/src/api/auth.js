import { request } from './client'

export const register = (organizationName, email, password) =>
  request('/api/auth/register', {
    method: 'POST',
    body: JSON.stringify({ organization_name: organizationName, email, password }),
  })

export const login = (email, password) =>
  request('/api/auth/login', { method: 'POST', body: JSON.stringify({ email, password }) })
