const UNITS = [
  ['year', 31536000],
  ['month', 2592000],
  ['day', 86400],
  ['hour', 3600],
  ['minute', 60],
]

// "2m ago" / "in 3h" — falls back to "just now" inside a 10s window so
// polling ticks don't make every fresh row look off-by-a-few-seconds stale.
export function relativeTime(input) {
  const date = input instanceof Date ? input : new Date(input)
  const diffSec = (date.getTime() - Date.now()) / 1000
  const abs = Math.abs(diffSec)

  if (abs < 10) return 'just now'

  for (const [unit, secs] of UNITS) {
    if (abs >= secs) {
      const value = Math.round(abs / secs)
      return diffSec < 0 ? `${value}${unit[0]} ago` : `in ${value}${unit[0]}`
    }
  }
  const value = Math.round(abs / 60) || 1
  return diffSec < 0 ? `${value}m ago` : `in ${value}m`
}

export function absoluteTime(input) {
  const date = input instanceof Date ? input : new Date(input)
  return date.toLocaleString()
}

export function shortId(id) {
  return id ? id.slice(0, 8) : ''
}
