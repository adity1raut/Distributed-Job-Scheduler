import { useCallback, useEffect, useState } from 'react'

// Polls `fetcher` immediately and then every `intervalMs`, matching the
// "live updates via polling" approach documented for this dashboard.
// `deps` is the caller-controlled invalidation key — the effect refetches
// whenever any of those values change, independent of the fetcher
// function's own identity (which is why `fetcher` itself isn't a dep).
export function usePolling(fetcher, deps, intervalMs = 5000) {
  const [data, setData] = useState(null)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)
  const [updatedAt, setUpdatedAt] = useState(null)
  const [reloadTick, setReloadTick] = useState(0)

  useEffect(() => {
    let cancelled = false

    const run = async () => {
      setLoading(true)
      try {
        const result = await fetcher()
        if (!cancelled) {
          setData(result)
          setError('')
          setUpdatedAt(new Date())
        }
      } catch (err) {
        if (!cancelled) setError(err.message)
      } finally {
        if (!cancelled) setLoading(false)
      }
    }

    run()
    if (!intervalMs) return () => {
      cancelled = true
    }

    const id = setInterval(run, intervalMs)
    return () => {
      cancelled = true
      clearInterval(id)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, intervalMs, reloadTick])

  const reload = useCallback(() => setReloadTick((t) => t + 1), [])

  return { data, error, loading, updatedAt, reload }
}
