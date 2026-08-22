import { Save } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'react-toastify'
import { pauseQueue, queueStats, resumeQueue, updateQueueConfig } from '../../api/queues'
import { usePolling } from '../../hooks/usePolling'
import ErrorBanner from '../ErrorBanner'
import Metric from '../Metric'
import { SkeletonCards } from '../Skeleton'

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
  const { data: stats, loading } = usePolling(() => queueStats(queue.id), [queue.id], 5000)
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
      <div className="section-label">Live counts</div>
      {loading && !stats ? (
        <SkeletonCards count={7} />
      ) : (
        <div className="metric-grid">
          {STAT_FIELDS.map(([field, tone]) => (
            <Metric key={field} label={field} value={stats?.[field]} tone={tone} />
          ))}
        </div>
      )}

      <div className="section-label">Configuration</div>
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
          <Save size={14} />
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
