import { CalendarClock, CalendarPlus, Pause, Play } from 'lucide-react'
import { useState } from 'react'
import toast from 'react-hot-toast'
import {
  createScheduledJob,
  listScheduledJobs,
  pauseScheduledJob,
  resumeScheduledJob,
} from '../../api/scheduledJobs'
import { usePolling } from '../../hooks/usePolling'
import EmptyState from '../EmptyState'
import ErrorBanner from '../ErrorBanner'
import { SkeletonRows } from '../Skeleton'
import Timestamp from '../Timestamp'

export default function ScheduledJobsPanel({ queueId }) {
  const { data, error, loading, reload } = usePolling(() => listScheduledJobs(queueId), [queueId], 5000)
  const [cron, setCron] = useState('*/5 * * * *')
  const [payload, setPayload] = useState('{}')
  const [formError, setFormError] = useState('')

  const handleCreate = async (e) => {
    e.preventDefault()
    setFormError('')
    let parsed
    try {
      parsed = JSON.parse(payload)
    } catch {
      setFormError('Payload template must be valid JSON')
      return
    }
    try {
      await createScheduledJob(queueId, { cron_expression: cron, payload_template: parsed })
      reload()
      toast.success('Schedule added')
    } catch (err) {
      setFormError(err.message)
      toast.error(err.message)
    }
  }

  const toggle = async (sj) => {
    try {
      if (sj.is_active) await pauseScheduledJob(sj.id)
      else await resumeScheduledJob(sj.id)
      reload()
      toast.success(sj.is_active ? 'Schedule paused' : 'Schedule resumed')
    } catch (err) {
      toast.error(err.message)
    }
  }

  return (
    <div>
      <form className="inline-form" onSubmit={handleCreate}>
        <input
          value={cron}
          onChange={(e) => setCron(e.target.value)}
          placeholder="Cron expression, e.g. */5 * * * *"
          className="mono"
          style={{ width: 200 }}
        />
        <input
          value={payload}
          onChange={(e) => setPayload(e.target.value)}
          placeholder="Payload template JSON"
          className="mono"
          style={{ width: 200 }}
        />
        <button className="btn" type="submit">
          <CalendarPlus size={15} />
          Add schedule
        </button>
      </form>
      <ErrorBanner message={error || formError} />

      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Cron</th>
              <th>Next run</th>
              <th>State</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {loading && !data && <SkeletonRows rows={2} cols={4} />}
            {data?.map((sj) => (
              <tr key={sj.id}>
                <td className="mono">{sj.cron_expression}</td>
                <td>
                  <Timestamp value={sj.next_run_at} />
                </td>
                <td>
                  <span className={`badge ${sj.is_active ? 'badge-completed' : 'badge-failed'}`}>
                    {sj.is_active ? 'active' : 'paused'}
                  </span>
                </td>
                <td>
                  <button className="btn-ghost" onClick={() => toggle(sj)}>
                    {sj.is_active ? <Pause size={13} /> : <Play size={13} />}
                    {sj.is_active ? 'Pause' : 'Resume'}
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {data && data.length === 0 && (
        <EmptyState icon={CalendarClock} title="No cron schedules" hint="Add one above to run jobs on a recurring basis." />
      )}
    </div>
  )
}
