import { ListChecks, Send, Settings2, SkullIcon } from 'lucide-react'
import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { getProject } from '../api/projects'
import { getQueue } from '../api/queues'
import Breadcrumbs from '../components/Breadcrumbs'
import DlqPanel from '../components/queue/DlqPanel'
import JobSubmitForm from '../components/queue/JobSubmitForm'
import JobsTable from '../components/queue/JobsTable'
import QueueConfigPanel from '../components/queue/QueueConfigPanel'
import ScheduledJobsPanel from '../components/queue/ScheduledJobsPanel'
import { usePolling } from '../hooks/usePolling'

const TABS = [
  { key: 'Jobs', icon: Send },
  { key: 'Scheduled', icon: ListChecks },
  { key: 'Dead letters', icon: SkullIcon },
  { key: 'Configuration', icon: Settings2 },
]

export default function QueueDetailPage() {
  const { queueId } = useParams()
  const { data: queue, reload: reloadQueue } = usePolling(() => getQueue(queueId), [queueId], 0)
  const { data: project } = usePolling(() => (queue ? getProject(queue.project_id) : Promise.resolve(null)), [queue?.project_id], 0)
  const [tab, setTab] = useState('Jobs')
  const [refreshKey, setRefreshKey] = useState(0)

  if (!queue) return <p className="muted">Loading…</p>

  return (
    <div>
      <Breadcrumbs
        items={[
          { label: 'Projects', to: '/projects' },
          { label: project?.name || '…', to: project ? `/projects/${project.id}` : undefined },
          { label: queue.name },
        ]}
      />
      <div className="page-header">
        <div>
          <h2>{queue.name}</h2>
          <p className="page-sub">
            Priority {queue.priority} · concurrency {queue.concurrency_limit}
          </p>
        </div>
        <span className={`badge ${queue.is_paused ? 'badge-failed' : 'badge-completed'}`}>
          {queue.is_paused ? 'paused' : 'active'}
        </span>
      </div>

      <div className="tab-row">
        {TABS.map(({ key, icon: Icon }) => (
          <button key={key} className={`tab${tab === key ? ' tab-active' : ''}`} onClick={() => setTab(key)}>
            <Icon size={14} />
            {key}
          </button>
        ))}
      </div>

      {tab === 'Jobs' && (
        <>
          <JobSubmitForm queueId={queueId} onSubmitted={() => setRefreshKey((k) => k + 1)} />
          <JobsTable queueId={queueId} refreshKey={refreshKey} />
        </>
      )}
      {tab === 'Scheduled' && <ScheduledJobsPanel queueId={queueId} />}
      {tab === 'Dead letters' && <DlqPanel queueId={queueId} />}
      {tab === 'Configuration' && <QueueConfigPanel queue={queue} onChanged={reloadQueue} />}
    </div>
  )
}
