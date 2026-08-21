import { useCallback, useState } from 'react'
import { Link } from 'react-router-dom'
import { listJobs } from '../../api/jobs'
import { usePolling } from '../../hooks/usePolling'
import ErrorBanner from '../ErrorBanner'
import StatusBadge from '../StatusBadge'

const STATUSES = ['', 'scheduled', 'queued', 'claimed', 'running', 'completed', 'failed', 'dead']

export default function JobsTable({ queueId, refreshKey }) {
  const [status, setStatus] = useState('')
  const [cursorStack, setCursorStack] = useState([''])
  const cursor = cursorStack[cursorStack.length - 1]

  const fetcher = useCallback(
    () => listJobs(queueId, { status: status || undefined, cursor: cursor || undefined, limit: 15 }),
    [queueId, status, cursor],
  )
  // Only the first page auto-refreshes; a paged-forward view stays put so
  // rows don't shift under the reader while they're looking at page 2+.
  const { data, error, loading, reload } = usePolling(
    fetcher,
    [queueId, status, cursor, refreshKey],
    cursor === '' ? 5000 : 0,
  )

  const handleStatusChange = (value) => {
    setStatus(value)
    setCursorStack([''])
  }

  return (
    <div>
      <div className="table-toolbar">
        <select value={status} onChange={(e) => handleStatusChange(e.target.value)}>
          {STATUSES.map((s) => (
            <option key={s || 'all'} value={s}>
              {s || 'all statuses'}
            </option>
          ))}
        </select>
        <button className="btn-ghost" onClick={reload}>
          Refresh
        </button>
      </div>
      <ErrorBanner message={error} />

      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>ID</th>
              <th>Type</th>
              <th>Status</th>
              <th>Attempts</th>
              <th>Run at</th>
              <th>Updated</th>
            </tr>
          </thead>
          <tbody>
            {data?.items.map((job) => (
              <tr key={job.id}>
                <td>
                  <Link to={`/jobs/${job.id}`} className="mono">
                    {job.id.slice(0, 8)}
                  </Link>
                </td>
                <td>{job.type}</td>
                <td>
                  <StatusBadge status={job.status} />
                </td>
                <td>
                  {job.attempts}/{job.max_attempts}
                </td>
                <td>{new Date(job.run_at).toLocaleString()}</td>
                <td>{new Date(job.updated_at).toLocaleString()}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {loading && !data && <p className="muted">Loading…</p>}
      {data && data.items.length === 0 && <p className="muted">No jobs match this filter.</p>}

      <div className="table-toolbar">
        <button
          className="btn-ghost"
          disabled={cursorStack.length === 1}
          onClick={() => setCursorStack((s) => s.slice(0, -1))}
        >
          ← Previous
        </button>
        <button
          className="btn-ghost"
          disabled={!data?.next_cursor}
          onClick={() => setCursorStack((s) => [...s, data.next_cursor])}
        >
          Next →
        </button>
      </div>
    </div>
  )
}
