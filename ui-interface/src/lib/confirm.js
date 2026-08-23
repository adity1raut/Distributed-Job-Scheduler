// Promise-based replacement for window.confirm() — ConfirmHost renders the
// actual dialog; this just hands it a request and resolves when the user
// picks an option. Falls back to window.confirm if the host somehow isn't
// mounted yet, so a call never hangs.
let listener = null

export function subscribeConfirm(fn) {
  listener = fn
  return () => {
    if (listener === fn) listener = null
  }
}

export function confirmDialog(message, options = {}) {
  return new Promise((resolve) => {
    if (!listener) {
      resolve(window.confirm(message))
      return
    }
    listener({ message, ...options, resolve })
  })
}
