import { useState } from 'react'
import toast from 'react-hot-toast'
import { pauseQueue, queueStats, resumeQueue, updateQueueConfig } from '../../api/queues'
import { usePolling } from '../../hooks/usePolling'
import ErrorBanner from '../ErrorBanner'

const STAT_FIELDS = [
  ['scheduled', ''],
  ['queued', ''],
  ['claimed', 'claimed'],
  ['running', 'running'],
  ['completed', 'completed'],
  ['failed', 'failed'],
  ['dead', 'dead'],
]

export default function QueueConfigPanel({ queue, onChanged }) {
  const { data: stats } = usePolling(() => queueStats(queue.id), [queue.id], 5000)
  const [priority, setPriority] = useState(queue.priority)
  const [concurrencyLimit, setConcurrencyLimit] = useState(queue.concurrency_limit)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)

  const handleSave = async (e) => {
    e.preventDefault()
    setError('')
    setSaving(true)
    try {
      await updateQueueConfig(queue.id, {
        priority: Number(priority),
        concurrency_limit: Number(concurrencyLimit),
      })
      onChanged()
      toast.success('Queue configuration saved')
    } catch (err) {
      setError(err.message)
      toast.error(err.message)
    } finally {
      setSaving(false)
    }
  }

  const togglePause = async () => {
    try {
      if (queue.is_paused) await resumeQueue(queue.id)
      else await pauseQueue(queue.id)
      onChanged()
      toast.success(queue.is_paused ? 'Queue resumed' : 'Queue paused')
    } catch (err) {
      toast.error(err.message)
    }
  }

  return (
    <div>
      <div className="stat-grid">
        {STAT_FIELDS.map(([field, tone]) => (
          <div className={`stat-card${tone ? ` stat-${tone}` : ''}`} key={field}>
            <div className="stat-label">{field}</div>
            <div className="stat-value">{stats?.[field] ?? '—'}</div>
          </div>
        ))}
      </div>

      <form className="inline-form" onSubmit={handleSave}>
        <label className="inline-label">
          Priority
          <input
            type="number"
            value={priority}
            onChange={(e) => setPriority(e.target.value)}
            style={{ width: 80 }}
          />
        </label>
        <label className="inline-label">
          Concurrency limit
          <input
            type="number"
            min={1}
            value={concurrencyLimit}
            onChange={(e) => setConcurrencyLimit(e.target.value)}
            style={{ width: 90 }}
          />
        </label>
        <button className="btn" type="submit" disabled={saving}>
          Save
        </button>
        <button type="button" className="btn-ghost" onClick={togglePause}>
          {queue.is_paused ? 'Resume queue' : 'Pause queue'}
        </button>
      </form>
      <ErrorBanner message={error} />
    </div>
  )
}
