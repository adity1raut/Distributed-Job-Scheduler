// Tiny in-house toast store — no external toast library. A toast is just a
// plain object pushed onto an array; ToastHost is the only subscriber that
// turns it into UI. Call sites just do `toast.success('...')` /
// `toast.error('...')`, same shape as the library call it replaces.
let toasts = []
let listeners = new Set()
let nextId = 1

function notify() {
  listeners.forEach((listener) => listener(toasts))
}

function push(type, message) {
  const id = nextId++
  toasts = [...toasts, { id, type, message }]
  notify()
  return id
}

export function subscribeToasts(listener) {
  listeners.add(listener)
  listener(toasts)
  return () => listeners.delete(listener)
}

export function dismissToast(id) {
  toasts = toasts.filter((t) => t.id !== id)
  notify()
}

export const toast = {
  success: (message) => push('success', message),
  error: (message) => push('error', message),
}
