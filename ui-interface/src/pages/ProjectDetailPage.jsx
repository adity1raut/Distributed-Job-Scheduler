import { Layers, ListPlus } from 'lucide-react'
import { useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import toast from 'react-hot-toast'
import { getProject } from '../api/projects'
import { createQueue, listQueues } from '../api/queues'
import Breadcrumbs from '../components/Breadcrumbs'
import EmptyState from '../components/EmptyState'
import ErrorBanner from '../components/ErrorBanner'
import { SkeletonRows } from '../components/Skeleton'
import Timestamp from '../components/Timestamp'
import { usePolling } from '../hooks/usePolling'

export default function ProjectDetailPage() {
  const { projectId } = useParams()
  const { data: project } = usePolling(() => getProject(projectId), [projectId], 0)
  const { data: queues, error, loading, reload } = usePolling(() => listQueues(projectId), [projectId], 5000)

  const [name, setName] = useState('')
  const [priority, setPriority] = useState(0)
  const [concurrencyLimit, setConcurrencyLimit] = useState(5)
  const [formError, setFormError] = useState('')

  const handleCreate = async (e) => {
    e.preventDefault()
    setFormError('')
    try {
      await createQueue(projectId, {
        name,
        priority: Number(priority),
        concurrency_limit: Number(concurrencyLimit),
      })
      setName('')
      reload()
      toast.success(`Queue "${name}" created`)
    } catch (err) {
      setFormError(err.message)
      toast.error(err.message)
    }
  }

  return (
    <div>
      <Breadcrumbs items={[{ label: 'Projects', to: '/projects' }, { label: project?.name || '…' }]} />
      <div className="page-header">
        <div>
          <h2>{project?.name || 'Project'}</h2>
          <p className="page-sub">Queues configure priority, concurrency, and retry behavior independently.</p>
        </div>
      </div>

      <form className="inline-form" onSubmit={handleCreate}>
        <input placeholder="Queue name" value={name} onChange={(e) => setName(e.target.value)} required />
        <input
          type="number"
          title="Priority"
          placeholder="Priority"
          value={priority}
          onChange={(e) => setPriority(e.target.value)}
          style={{ width: 90 }}
        />
        <input
          type="number"
          min={1}
          title="Concurrency limit"
          placeholder="Concurrency"
          value={concurrencyLimit}
          onChange={(e) => setConcurrencyLimit(e.target.value)}
          style={{ width: 100 }}
        />
        <button className="btn" type="submit">
          <ListPlus size={15} />
          Create queue
        </button>
      </form>
      <ErrorBanner message={error || formError} />

      {!loading && queues && queues.length === 0 && (
        <EmptyState icon={Layers} title="No queues yet" hint="Create one above to start submitting jobs." />
      )}

      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Name</th>
              <th>Priority</th>
              <th>Concurrency</th>
              <th>State</th>
              <th>Created</th>
            </tr>
          </thead>
          <tbody>
            {loading && !queues && <SkeletonRows rows={3} cols={5} />}
            {queues?.map((queue) => (
              <tr key={queue.id}>
                <td>
                  <Link to={`/queues/${queue.id}`} className="row-title">
                    {queue.name}
                  </Link>
                </td>
                <td className="mono num">{queue.priority}</td>
                <td className="mono num">{queue.concurrency_limit}</td>
                <td>
                  <span className={`badge ${queue.is_paused ? 'badge-failed' : 'badge-completed'}`}>
                    {queue.is_paused ? 'paused' : 'active'}
                  </span>
                </td>
                <td>
                  <Timestamp value={queue.created_at} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
