import { RotateCcw } from 'lucide-react'
import { Fragment, useState } from 'react'
import { useParams } from 'react-router-dom'
import { executionLogs, getJob, retryJob } from '../api/jobs'
import { getProject } from '../api/projects'
import { getQueue } from '../api/queues'
import Breadcrumbs from '../components/Breadcrumbs'
import CopyableId from '../components/CopyableId'
import CopyBlock from '../components/CopyBlock'
import ErrorBanner from '../components/ErrorBanner'
import StatusBadge from '../components/StatusBadge'
import Timestamp from '../components/Timestamp'
import { usePolling } from '../hooks/usePolling'
import { toast } from '../lib/toast'

export default function JobDetailPage() {
  const { jobId } = useParams()
  const { data: job, error, reload } = usePolling(() => getJob(jobId), [jobId], 4000)
  const { data: queue } = usePolling(() => (job ? getQueue(job.queue_id) : Promise.resolve(null)), [job?.queue_id], 0)
  const { data: project } = usePolling(
    () => (queue ? getProject(queue.project_id) : Promise.resolve(null)),
    [queue?.project_id],
    0,
  )
  const [openLogs, setOpenLogs] = useState(null)
  const [logs, setLogs] = useState([])
  const [retrying, setRetrying] = useState(false)

  const toggleLogs = async (executionId) => {
    if (openLogs === executionId) {
      setOpenLogs(null)
      return
    }
    const fetched = await executionLogs(executionId)
    setLogs(fetched)
    setOpenLogs(executionId)
  }

  const handleRetry = async () => {
    setRetrying(true)
    try {
      await retryJob(jobId)
      reload()
      toast.success('Job requeued')
    } catch (err) {
      toast.error(err.message)
    } finally {
      setRetrying(false)
    }
  }

  if (!job) return <p className="muted">Loading…</p>

  return (
    <div>
      <Breadcrumbs
        items={[
          { label: 'Projects', to: '/projects' },
          { label: project?.name || '…', to: project ? `/projects/${project.id}` : undefined },
          { label: queue?.name || '…', to: queue ? `/queues/${queue.id}` : undefined },
          { label: 'Job' },
        ]}
      />
      <div className="page-header">
        <div>
          <h2>Job detail</h2>
          <CopyableId id={job.id} />
        </div>
        <StatusBadge status={job.status} />
      </div>
      <ErrorBanner message={error} />

      <div className="detail-grid">
        <div>
          <span className="muted">Type</span>
          <div className="capitalize">{job.type}</div>
        </div>
        <div>
          <span className="muted">Attempts</span>
          <div className="mono">
            {job.attempts}/{job.max_attempts}
          </div>
        </div>
        <div>
          <span className="muted">Run at</span>
          <div>
            <Timestamp value={job.run_at} />
          </div>
        </div>
        <div>
          <span className="muted">Updated</span>
          <div>
            <Timestamp value={job.updated_at} />
          </div>
        </div>
      </div>

      {job.executions?.length > 0 && (
        <div className="attempt-timeline">
          {job.executions.map((ex, i) => (
            <div className="attempt-node" key={ex.id}>
              <div className={`attempt-dot attempt-${ex.status}`} title={`Attempt ${ex.attempt_number}: ${ex.status}`}>
                {ex.attempt_number}
              </div>
              {i < job.executions.length - 1 && <div className="attempt-line" />}
            </div>
          ))}
        </div>
      )}

      <h4>Payload</h4>
      <CopyBlock text={JSON.stringify(job.payload, null, 2)} />

      {job.last_error && (
        <>
          <h4>Last error</h4>
          <CopyBlock text={job.last_error} tone="error" />
        </>
      )}

      {(job.status === 'failed' || job.status === 'dead') && (
        <button className="btn" onClick={handleRetry} disabled={retrying}>
          <RotateCcw size={14} />
          {retrying ? 'Retrying…' : 'Retry job'}
        </button>
      )}

      <h4>Execution history</h4>
      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Attempt</th>
              <th>Status</th>
              <th>Started</th>
              <th>Duration</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {job.executions?.map((ex) => (
              <Fragment key={ex.id}>
                <tr>
                  <td className="mono num">{ex.attempt_number}</td>
                  <td>
                    <StatusBadge status={ex.status} />
                  </td>
                  <td>
                    <Timestamp value={ex.started_at} />
                  </td>
                  <td className="mono num">{ex.duration_ms != null ? `${ex.duration_ms}ms` : '—'}</td>
                  <td>
                    <button className="btn-ghost" onClick={() => toggleLogs(ex.id)}>
                      {openLogs === ex.id ? 'Hide logs' : 'View logs'}
                    </button>
                  </td>
                </tr>
                {openLogs === ex.id && (
                  <tr>
                    <td colSpan={5}>
                      <div className="log-list">
                        {logs.length === 0 && <p className="muted">No logs.</p>}
                        {logs.map((log) => (
                          <div key={log.id} className={`log-line log-${log.level}`}>
                            <span className="mono">
                              <Timestamp value={log.logged_at} />
                            </span>
                            <span className="log-level">{log.level}</span>
                            <span>{log.message}</span>
                          </div>
                        ))}
                      </div>
                    </td>
                  </tr>
                )}
              </Fragment>
            ))}
          </tbody>
        </table>
      </div>
      {job.executions?.length === 0 && <p className="muted">No execution attempts yet.</p>}
    </div>
  )
}
