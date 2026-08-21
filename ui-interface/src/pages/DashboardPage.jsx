import { getOverview } from '../api/dashboard'
import ErrorBanner from '../components/ErrorBanner'
import StatCard from '../components/StatCard'
import { usePolling } from '../hooks/usePolling'

export default function DashboardPage() {
  const { data, error, loading } = usePolling(getOverview, [], 5000)

  return (
    <div>
      <div className="page-header">
        <h2>Overview</h2>
        <span className="live-dot" title="Updates every 5s" />
      </div>
      <ErrorBanner message={error} />
      {loading && !data ? (
        <p className="muted">Loading…</p>
      ) : (
        data && (
          <div className="stat-grid">
            <StatCard label="Projects" value={data.total_projects} />
            <StatCard label="Queues" value={data.total_queues} />
            <StatCard label="Queued jobs" value={data.queued_jobs} />
            <StatCard label="Running jobs" value={data.running_jobs} tone="running" />
            <StatCard label="Completed (24h)" value={data.completed_jobs_24h} tone="completed" />
            <StatCard label="Failed (24h)" value={data.failed_jobs_24h} tone="failed" />
            <StatCard label="Dead-lettered" value={data.dead_jobs} tone="dead" />
            <StatCard label="Online workers" value={data.online_workers} tone="running" />
          </div>
        )
      )}
    </div>
  )
}
