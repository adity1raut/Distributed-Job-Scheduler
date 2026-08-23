import { Inbox } from 'lucide-react'
import { useCallback, useState } from 'react'
import { listJobs } from '../../api/jobs'
import { usePolling } from '../../hooks/usePolling'
import CopyableId from '../CopyableId'
import EmptyState from '../EmptyState'
import ErrorBanner from '../ErrorBanner'
import Select from '../Select'
import { SkeletonRows } from '../Skeleton'
import StatusBadge from '../StatusBadge'
import Timestamp from '../Timestamp'

const STATUSES = ['', 'scheduled', 'queued', 'claimed', 'running', 'completed', 'failed', 'dead']
const STATUS_OPTIONS = STATUSES.map((s) => ({ value: s, label: s || 'All statuses' }))

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
        <Select value={status} onChange={handleStatusChange} options={STATUS_OPTIONS} style={{ minWidth: 160 }} />
        <button className="btn-ghost" onClick={reload}>
          Refresh
        </button>
        {data && <span className="table-count">{data.items.length} shown</span>}
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
            {loading && !data && <SkeletonRows rows={5} cols={6} />}
            {data?.items.map((job) => (
              <tr key={job.id}>
                <td>
                  <CopyableId id={job.id} to={`/jobs/${job.id}`} />
                </td>
                <td className="capitalize">{job.type}</td>
                <td>
                  <StatusBadge status={job.status} />
                </td>
                <td className="mono num">
                  {job.attempts}/{job.max_attempts}
                </td>
                <td>
                  <Timestamp value={job.run_at} />
                </td>
                <td>
                  <Timestamp value={job.updated_at} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {data && data.items.length === 0 && (
        <EmptyState
          icon={Inbox}
          title={status ? `No ${status} jobs` : 'No jobs yet'}
          hint={status ? 'Try a different status filter.' : 'Submit one above to see it here.'}
        />
      )}

      <div className="table-toolbar">
        <button
          className="btn-ghost"
          disabled={cursorStack.length === 1}
          onClick={() => setCursorStack((s) => s.slice(0, -1))}
        >
          Previous
        </button>
        <button
          className="btn-ghost"
          disabled={!data?.next_cursor}
          onClick={() => setCursorStack((s) => [...s, data.next_cursor])}
        >
          Next
        </button>
      </div>
    </div>
  )
}
