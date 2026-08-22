import { Send } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'react-toastify'
import { submitJob } from '../../api/jobs'

const TYPES = ['immediate', 'delayed', 'scheduled', 'batch']

export default function JobSubmitForm({ queueId, onSubmitted }) {
  const [type, setType] = useState('immediate')
  const [payload, setPayload] = useState('{"task":"echo"}')
  const [delayMs, setDelayMs] = useState(5000)
  const [runAt, setRunAt] = useState('')
  const [batchCount, setBatchCount] = useState(3)
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const handleSubmit = async (e) => {
    e.preventDefault()
    setError('')

    let parsedPayload
    try {
      parsedPayload = JSON.parse(payload)
    } catch {
      setError('Payload must be valid JSON')
      return
    }

    const input = { type, payload: parsedPayload }
    if (type === 'delayed') input.delay_ms = Number(delayMs)
    if (type === 'scheduled') {
      if (!runAt) {
        setError('Pick a run-at time')
        return
      }
      input.run_at = new Date(runAt).toISOString()
    }
    if (type === 'batch') input.batch_count = Number(batchCount)

    setSubmitting(true)
    try {
      const jobs = await submitJob(queueId, input)
      onSubmitted()
      toast.success(jobs.length > 1 ? `${jobs.length} jobs submitted` : 'Job submitted')
    } catch (err) {
      setError(err.message)
      toast.error(err.message)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form className="job-submit-form" onSubmit={handleSubmit}>
      <div className="inline-form">
        <select value={type} onChange={(e) => setType(e.target.value)}>
          {TYPES.map((t) => (
            <option key={t} value={t}>
              {t}
            </option>
          ))}
        </select>
        {type === 'delayed' && (
          <input
            type="number"
            min={1}
            value={delayMs}
            onChange={(e) => setDelayMs(e.target.value)}
            title="Delay (ms)"
            style={{ width: 110 }}
          />
        )}
        {type === 'scheduled' && (
          <input type="datetime-local" value={runAt} onChange={(e) => setRunAt(e.target.value)} />
        )}
        {type === 'batch' && (
          <input
            type="number"
            min={2}
            value={batchCount}
            onChange={(e) => setBatchCount(e.target.value)}
            title="Batch count"
            style={{ width: 100 }}
          />
        )}
        <button className="btn" type="submit" disabled={submitting}>
          <Send size={14} />
          {submitting ? 'Submitting…' : 'Submit job'}
        </button>
      </div>
      <textarea
        className="payload-input mono"
        rows={2}
        value={payload}
        onChange={(e) => setPayload(e.target.value)}
        spellCheck={false}
      />
      {error && <p className="error">{error}</p>}
    </form>
  )
}
