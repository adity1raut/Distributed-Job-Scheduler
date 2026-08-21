import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { getQueue } from '../api/queues'
import DlqPanel from '../components/queue/DlqPanel'
import JobSubmitForm from '../components/queue/JobSubmitForm'
import JobsTable from '../components/queue/JobsTable'
import QueueConfigPanel from '../components/queue/QueueConfigPanel'
import ScheduledJobsPanel from '../components/queue/ScheduledJobsPanel'
import { usePolling } from '../hooks/usePolling'

const TABS = ['Jobs', 'Scheduled', 'Dead letters', 'Configuration']

export default function QueueDetailPage() {
  const { queueId } = useParams()
  const { data: queue, reload: reloadQueue } = usePolling(() => getQueue(queueId), [queueId], 0)
  const [tab, setTab] = useState('Jobs')
  const [refreshKey, setRefreshKey] = useState(0)

  if (!queue) return <p className="muted">Loading…</p>

  return (
    <div>
      <div className="page-header">
        <h2>{queue.name}</h2>
        <span className={`badge ${queue.is_paused ? 'badge-failed' : 'badge-completed'}`}>
          {queue.is_paused ? 'paused' : 'active'}
        </span>
      </div>

      <div className="tab-row">
        {TABS.map((t) => (
          <button key={t} className={`tab${tab === t ? ' tab-active' : ''}`} onClick={() => setTab(t)}>
            {t}
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
