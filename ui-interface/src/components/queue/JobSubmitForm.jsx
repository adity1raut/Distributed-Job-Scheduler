import { Info, Send } from 'lucide-react'
import { useState } from 'react'
import { submitJob } from '../../api/jobs'
import { toast } from '../../lib/toast'
import Select from '../Select'

const TYPES = ['immediate', 'delayed', 'scheduled', 'batch']

// What each type actually changes about when/how many job rows get
// created — the part that isn't obvious from the type name alone.
const TYPE_HINTS = {
  immediate: 'Runs as soon as a worker is free. No extra fields needed.',
  delayed: 'Runs delay_ms after you hit submit — the countdown starts now, not when a worker picks it up.',
  scheduled: 'Creates one job that runs once, at the exact date & time you pick.',
  batch: 'Creates batch_count separate job rows sharing one batch_id — each one runs and retries independently.',
}

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
        <Select value={type} onChange={setType} options={TYPES} style={{ minWidth: 140 }} />
        {type === 'delayed' && (
          <label className="inline-label">
            Delay (ms)
            <input
              type="number"
              min={1}
              value={delayMs}
              onChange={(e) => setDelayMs(e.target.value)}
              style={{ width: 110 }}
            />
          </label>
        )}
        {type === 'scheduled' && (
          <label className="inline-label">
            Run at
            <input type="datetime-local" value={runAt} onChange={(e) => setRunAt(e.target.value)} />
          </label>
        )}
        {type === 'batch' && (
          <label className="inline-label">
            Batch count
            <input
              type="number"
              min={2}
              value={batchCount}
              onChange={(e) => setBatchCount(e.target.value)}
              style={{ width: 100 }}
            />
          </label>
        )}
        <button className="btn" type="submit" disabled={submitting}>
          <Send size={14} />
          {submitting ? 'Submitting…' : 'Submit job'}
        </button>
      </div>
      <p className="form-hint">
        <Info size={13} />
        {TYPE_HINTS[type]}
      </p>
      <textarea
        className="payload-input mono"
        rows={2}
        value={payload}
        onChange={(e) => setPayload(e.target.value)}
        spellCheck={false}
      />
      <p className="form-hint">
        <Info size={13} />
        The demo worker only understands three payload fields — <code>task</code> (set to{' '}
        <code>&quot;fail&quot;</code> to force a failure), <code>sleep_ms</code> (how long the job takes to
        run), and <code>fail_rate</code> (0–1 chance of a random failure). Anything else you put in here is
        stored and shown back to you, but has no effect on execution.
      </p>
      {error && <p className="error">{error}</p>}
    </form>
  )
}
