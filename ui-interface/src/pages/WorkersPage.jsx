import { Fragment, useState } from 'react'
import { listWorkers, workerHeartbeats } from '../api/workers'
import ErrorBanner from '../components/ErrorBanner'
import { usePolling } from '../hooks/usePolling'

export default function WorkersPage() {
  const { data: workers, error } = usePolling(listWorkers, [], 5000)
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
        <h2>Workers</h2>
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
            {workers?.map((w) => (
              <Fragment key={w.id}>
                <tr>
                  <td>{w.hostname}</td>
                  <td>
                    <span className={`badge ${w.status === 'online' && !w.is_stale ? 'badge-completed' : 'badge-failed'}`}>
                      {w.status === 'online' && w.is_stale ? 'stale' : w.status}
                    </span>
                  </td>
                  <td>{new Date(w.started_at).toLocaleString()}</td>
                  <td>{w.last_heartbeat_at ? new Date(w.last_heartbeat_at).toLocaleTimeString() : '—'}</td>
                  <td>{w.active_job_count ?? 0}</td>
                  <td>
                    <button className="btn-ghost" onClick={() => toggle(w.id)}>
                      {expanded === w.id ? 'Hide' : 'History'}
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
                            <span className="mono">{new Date(hb.reported_at).toLocaleString()}</span>
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
      {workers && workers.length === 0 && <p className="muted">No workers have registered yet.</p>}
    </div>
  )
}
