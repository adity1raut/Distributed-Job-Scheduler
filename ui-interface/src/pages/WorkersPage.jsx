import { Server } from 'lucide-react'
import { Fragment, useState } from 'react'
import { listWorkers, workerHeartbeats } from '../api/workers'
import EmptyState from '../components/EmptyState'
import ErrorBanner from '../components/ErrorBanner'
import { SkeletonRows } from '../components/Skeleton'
import Timestamp from '../components/Timestamp'
import { usePolling } from '../hooks/usePolling'

export default function WorkersPage() {
  const { data: workers, error, loading } = usePolling(listWorkers, [], 5000)
  const [expanded, setExpanded] = useState(null)
  const [heartbeats, setHeartbeats] = useState([])

  const toggle = async (workerId) => {
    if (expanded === workerId) {
      setExpanded(null)
      return
    }
    const beats = await workerHeartbeats(workerId, 20)
    setHeartbeats(beats)
    setExpanded(workerId)
  }

  return (
    <div>
      <div className="page-header">
        <div>
          <h2>Workers</h2>
          <p className="page-sub">Every worker that has registered, with its live heartbeat status.</p>
        </div>
      </div>
      <ErrorBanner message={error} />
      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Hostname</th>
              <th>Status</th>
              <th>Started</th>
              <th>Last heartbeat</th>
              <th>Active jobs</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {loading && !workers && <SkeletonRows rows={2} cols={6} />}
            {workers?.map((w) => (
              <Fragment key={w.id}>
                <tr>
                  <td>{w.hostname}</td>
                  <td>
                    <span className={`badge ${w.status === 'online' && !w.is_stale ? 'badge-completed' : 'badge-failed'}`}>
                      <span className={`status-dot${w.status === 'online' && !w.is_stale ? ' pulse' : ''}`} />
                      {w.status === 'online' && w.is_stale ? 'stale' : w.status}
                    </span>
                  </td>
                  <td>
                    <Timestamp value={w.started_at} />
                  </td>
                  <td>
                    <Timestamp value={w.last_heartbeat_at} />
                  </td>
                  <td className="mono num">{w.active_job_count ?? 0}</td>
                  <td>
                    <button className="btn-ghost" onClick={() => toggle(w.id)}>
                      History
                    </button>
                  </td>
                </tr>
                {expanded === w.id && (
                  <tr>
                    <td colSpan={6}>
                      <div className="log-list">
                        {heartbeats.length === 0 && <p className="muted">No heartbeat history.</p>}
                        {heartbeats.map((hb) => (
                          <div key={hb.id} className="log-line">
                            <span className="mono">
                              <Timestamp value={hb.reported_at} />
                            </span>
                            <span>{hb.active_job_count} active</span>
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
      {workers && workers.length === 0 && (
        <EmptyState icon={Server} title="No workers registered" hint="Start `cmd/worker` to see it appear here." />
      )}
    </div>
  )
}
