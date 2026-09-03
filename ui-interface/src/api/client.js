// Resolution order: a runtime-injected value (window.__APP_CONFIG__, written
// by the Docker image's entrypoint from the container's VITE_API_URL env var
// — see docker-entrypoint.d/40-render-config.sh) takes priority over the
// build-time Vite env var, so one built image can be deployed against
// different API URLs without a rebuild. The Vite env var still works
// unchanged for platforms that inject it at build time (Vercel/Netlify/CI)
// instead of running our Docker image.
const BASE_URL = window.__APP_CONFIG__?.API_URL || import.meta.env.VITE_API_URL || 'http://localhost:8080'

export async function request(path, options = {}) {
  const token = localStorage.getItem('token')
  const headers = { 'Content-Type': 'application/json', ...options.headers }
  if (token) headers.Authorization = `Bearer ${token}`

  const res = await fetch(`${BASE_URL}${path}`, { ...options, headers })

  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    if (res.status === 401) {
      localStorage.removeItem('token')
      localStorage.removeItem('user')
    }
    throw new Error(body.error?.message || `Request failed with status ${res.status}`)
  }

  if (res.status === 204) return null
  return res.json()
}

export function qs(params = {}) {
  const entries = Object.entries(params).filter(([, v]) => v !== undefined && v !== null && v !== '')
  if (entries.length === 0) return ''
  return '?' + new URLSearchParams(entries).toString()
}
