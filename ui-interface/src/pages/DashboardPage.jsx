import {
  Activity,
  CheckCircle2,
  FolderKanban,
  Inbox,
  Layers,
  Server,
  Skull,
  XCircle,
} from 'lucide-react'
import { getOverview } from '../api/dashboard'
import ErrorBanner from '../components/ErrorBanner'
import { SkeletonCards } from '../components/Skeleton'
import StatCard from '../components/StatCard'
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
          <div className="stat-grid">
            <StatCard label="Projects" value={data.total_projects} icon={FolderKanban} />
            <StatCard label="Queues" value={data.total_queues} icon={Layers} />
            <StatCard label="Online workers" value={data.online_workers} tone="running" icon={Server} />
          </div>

          <div className="section-label">Throughput</div>
          <div className="stat-grid">
            <StatCard label="Queued jobs" value={data.queued_jobs} icon={Inbox} />
            <StatCard label="Running jobs" value={data.running_jobs} tone="running" icon={Activity} />
            <StatCard label="Completed (24h)" value={data.completed_jobs_24h} tone="completed" icon={CheckCircle2} />
            <StatCard label="Failed (24h)" value={data.failed_jobs_24h} tone="failed" icon={XCircle} />
            <StatCard label="Dead-lettered" value={data.dead_jobs} tone="dead" icon={Skull} />
          </div>
        </>
      )}
    </div>
  )
}
