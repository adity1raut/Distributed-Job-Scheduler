import { getOverview } from '../api/dashboard'
import ErrorBanner from '../components/ErrorBanner'
import Metric from '../components/Metric'
import { SkeletonCards } from '../components/Skeleton'
import Timestamp from '../components/Timestamp'
import { usePolling } from '../hooks/usePolling'

export default function DashboardPage() {
  const { data, error, loading, updatedAt } = usePolling(getOverview, [], 5000)

  return (
    <div>
      <div className="page-header">
        <div>
          <h2>Overview</h2>
          <p className="page-sub">System health across every project in your organization.</p>
        </div>
        <div className="live-indicator">
          <span className="live-dot" />
          {updatedAt ? <>Updated <Timestamp value={updatedAt} /></> : 'Live'}
        </div>
      </div>
      <ErrorBanner message={error} />

      {loading && !data && <SkeletonCards count={8} />}

      {data && (
        <>
          <div className="section-label">Inventory</div>
          <div className="metric-grid">
            <Metric label="Projects" value={data.total_projects} />
            <Metric label="Queues" value={data.total_queues} />
            <Metric label="Online workers" value={data.online_workers} tone="running" />
          </div>

          <div className="section-label">Throughput</div>
          <div className="metric-grid">
            <Metric label="Queued jobs" value={data.queued_jobs} />
            <Metric label="Running jobs" value={data.running_jobs} tone="running" />
            <Metric label="Completed (24h)" value={data.completed_jobs_24h} tone="completed" />
            <Metric label="Failed (24h)" value={data.failed_jobs_24h} tone="failed" />
            <Metric label="Dead-lettered" value={data.dead_jobs} tone="dead" />
          </div>
        </>
      )}
    </div>
  )
}
