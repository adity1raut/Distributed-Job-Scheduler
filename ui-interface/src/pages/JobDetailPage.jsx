import { Fragment, useState } from 'react'
import { useParams } from 'react-router-dom'
import toast from 'react-hot-toast'
import { executionLogs, getJob, retryJob } from '../api/jobs'
import ErrorBanner from '../components/ErrorBanner'
import StatusBadge from '../components/StatusBadge'
import { usePolling } from '../hooks/usePolling'

export default function JobDetailPage() {
  const { jobId } = useParams()
  const { data: job, error, reload } = usePolling(() => getJob(jobId), [jobId], 4000)
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
      <div className="page-header">
        <h2 className="mono">Job {job.id.slice(0, 8)}</h2>
        <StatusBadge status={job.status} />
      </div>
      <ErrorBanner message={error} />

      <div className="detail-grid">
        <div>
          <span className="muted">Type</span>
          <div>{job.type}</div>
        </div>
        <div>
          <span className="muted">Attempts</span>
          <div>
            {job.attempts}/{job.max_attempts}
          </div>
        </div>
        <div>
          <span className="muted">Run at</span>
          <div>{new Date(job.run_at).toLocaleString()}</div>
        </div>
        <div>
          <span className="muted">Updated</span>
          <div>{new Date(job.updated_at).toLocaleString()}</div>
        </div>
      </div>

      <h4>Payload</h4>
      <pre className="payload-view">{JSON.stringify(job.payload, null, 2)}</pre>

      {job.last_error && (
        <>
          <h4>Last error</h4>
          <pre className="payload-view error-text">{job.last_error}</pre>
        </>
      )}

      {(job.status === 'failed' || job.status === 'dead') && (
        <button className="btn" onClick={handleRetry} disabled={retrying}>
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
                  <td>{ex.attempt_number}</td>
                  <td>
                    <StatusBadge status={ex.status} />
                  </td>
                  <td>{new Date(ex.started_at).toLocaleString()}</td>
                  <td>{ex.duration_ms != null ? `${ex.duration_ms}ms` : '—'}</td>
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
                            <span className="mono">{new Date(log.logged_at).toLocaleTimeString()}</span>
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
