import { FolderKanban, Inbox, Server } from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router-dom'
import { getOverview, getRecentJobs, getThroughput } from '../api/dashboard'
import { listProjects } from '../api/projects'
import { listQueues } from '../api/queues'
import { listWorkers } from '../api/workers'
import CopyableId from '../components/CopyableId'
import EmptyState from '../components/EmptyState'
import ErrorBanner from '../components/ErrorBanner'
import Metric from '../components/Metric'
import Select from '../components/Select'
import { SkeletonCards, SkeletonRows } from '../components/Skeleton'
import StatusBadge from '../components/StatusBadge'
import ThroughputChart from '../components/ThroughputChart'
import Timestamp from '../components/Timestamp'
import { usePolling } from '../hooks/usePolling'

const STATUS_OPTIONS = [
  { value: '', label: 'All statuses' },
  { value: 'scheduled', label: 'Scheduled' },
  { value: 'queued', label: 'Queued' },
  { value: 'claimed', label: 'Claimed' },
  { value: 'running', label: 'Running' },
  { value: 'completed', label: 'Completed' },
  { value: 'failed', label: 'Failed' },
  { value: 'dead', label: 'Dead' },
]

async function fetchProjectsWithQueues() {
  const projects = await listProjects()
  return Promise.all(projects.map(async (p) => ({ ...p, queues: await listQueues(p.id) })))
}

export default function DashboardPage() {
  const { data, error, loading, updatedAt } = usePolling(getOverview, [], 5000)
  const [jobStatus, setJobStatus] = useState('')
  const { data: recentJobs, loading: jobsLoading } = usePolling(
    () => getRecentJobs({ limit: 20, status: jobStatus || undefined }),
    [jobStatus],
    5000,
  )
  // Project/queue structure and the worker roster change far less often than
  // job status — polling them on the same 5s cadence as the rest of the page
  // was most of what pushed a busy org over RATE_LIMIT_PER_MIN, especially
  // since the project list fans out into one extra request per project.
  const { data: projectQueues, loading: projectsLoading } = usePolling(fetchProjectsWithQueues, [], 20000)
  const { data: workers, loading: workersLoading } = usePolling(listWorkers, [], 15000)
  const { data: throughput, loading: throughputLoading } = usePolling(getThroughput, [], 30000)

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

          {throughputLoading && !throughput ? (
            <span className="skeleton skeleton-bar" style={{ width: '100%', height: 200 }} />
          ) : (
            throughput && <ThroughputChart buckets={throughput} />
          )}
        </>
      )}

      <div className="dashboard-columns">
        <div className="dashboard-col">
          <div className="section-header">
            <span className="section-label">Worker pool</span>
            <Link to="/workers" className="section-link">
              View all →
            </Link>
          </div>
          <div className="table-wrap dashboard-scroll">
            <table>
              <thead>
                <tr>
                  <th>Hostname</th>
                  <th>Status</th>
                  <th>Active jobs</th>
                </tr>
              </thead>
              <tbody>
                {workersLoading && !workers && <SkeletonRows rows={7} cols={3} />}
                {workers?.map((w) => (
                  <tr key={w.id}>
                    <td
                      title={w.hostname}
                      style={{ maxWidth: 220, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}
                    >
                      {w.hostname}
                    </td>
                    <td>
                      <span className={`badge ${w.status === 'online' && !w.is_stale ? 'badge-completed' : 'badge-failed'}`}>
                        <span className={`status-dot${w.status === 'online' && !w.is_stale ? ' pulse' : ''}`} />
                        {w.status === 'online' && w.is_stale ? 'stale' : w.status}
                      </span>
                    </td>
                    <td className="mono num">{w.active_job_count ?? 0}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {workers && workers.length === 0 && (
            <EmptyState icon={Server} title="No workers registered" hint="Start `cmd/worker` to see it appear here." />
          )}
        </div>

        <div className="dashboard-col">
          <div className="section-header">
            <span className="section-label">Projects &amp; queues</span>
            <Link to="/projects" className="section-link">
              View all →
            </Link>
          </div>
          <div className="dashboard-scroll">
            {projectsLoading && !projectQueues && (
              <div className="project-group">
                <span className="skeleton skeleton-bar" style={{ width: 160, height: 14 }} />
              </div>
            )}
            {projectQueues?.map((project) => (
              <div className="project-group" key={project.id}>
                <div className="project-group-header">
                  <Link to={`/projects/${project.id}`} className="row-title">
                    {project.name}
                  </Link>
                  <span className="muted">
                    {project.queues.length} {project.queues.length === 1 ? 'queue' : 'queues'}
                  </span>
                </div>
                {project.queues.length > 0 ? (
                  <div className="queue-mini-list">
                    {project.queues.map((q) => (
                      <div className="queue-mini-row" key={q.id}>
                        <Link to={`/queues/${q.id}`} className="row-title">
                          {q.name}
                        </Link>
                        <span className="queue-mini-meta">
                          <span className="mono">p{q.priority} · c{q.concurrency_limit}</span>
                          <span className={`badge ${q.is_paused ? 'badge-failed' : 'badge-completed'}`}>
                            {q.is_paused ? 'paused' : 'active'}
                          </span>
                        </span>
                      </div>
                    ))}
                  </div>
                ) : (
                  <p className="muted">No queues yet.</p>
                )}
              </div>
            ))}
          </div>
          {projectQueues && projectQueues.length === 0 && (
            <EmptyState icon={FolderKanban} title="No projects yet" hint="Create one to start scheduling jobs." />
          )}
        </div>
      </div>

      <div className="section-header">
        <span className="section-label">Recent jobs</span>
        <Select value={jobStatus} onChange={setJobStatus} options={STATUS_OPTIONS} style={{ minWidth: 150 }} />
      </div>
      <div className="table-wrap dashboard-scroll">
        <table>
          <thead>
            <tr>
              <th>ID</th>
              <th>Project / Queue</th>
              <th>Type</th>
              <th>Status</th>
              <th>Attempts</th>
              <th>Updated</th>
            </tr>
          </thead>
          <tbody>
            {jobsLoading && !recentJobs && <SkeletonRows rows={7} cols={6} />}
            {recentJobs?.map((job) => (
              <tr key={job.id}>
                <td>
                  <CopyableId id={job.id} to={`/jobs/${job.id}`} />
                </td>
                <td>
                  <Link to={`/queues/${job.queue_id}`} className="row-title">
                    {job.queue_name}
                  </Link>
                  <div className="muted">{job.project_name}</div>
                </td>
                <td className="capitalize">{job.type}</td>
                <td>
                  <StatusBadge status={job.status} />
                </td>
                <td className="mono num">
                  {job.attempts}/{job.max_attempts}
                </td>
                <td>
                  <Timestamp value={job.updated_at} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {recentJobs && recentJobs.length === 0 && (
        <EmptyState
          icon={Inbox}
          title={jobStatus ? `No ${jobStatus} jobs` : 'No jobs yet'}
          hint={jobStatus ? 'Try a different status filter.' : 'Submit one from a queue to see it here.'}
        />
      )}
    </div>
  )
}
