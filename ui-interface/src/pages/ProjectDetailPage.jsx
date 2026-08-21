import { useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import toast from 'react-hot-toast'
import { getProject } from '../api/projects'
import { createQueue, listQueues } from '../api/queues'
import ErrorBanner from '../components/ErrorBanner'
import { usePolling } from '../hooks/usePolling'

export default function ProjectDetailPage() {
  const { projectId } = useParams()
  const { data: project } = usePolling(() => getProject(projectId), [projectId], 0)
  const { data: queues, error, reload } = usePolling(() => listQueues(projectId), [projectId], 5000)

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
      <div className="page-header">
        <h2>{project?.name || 'Project'}</h2>
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
          Create queue
        </button>
      </form>
      <ErrorBanner message={error || formError} />

      {queues && queues.length === 0 && <p className="muted">No queues yet — create one above.</p>}

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
            {queues?.map((queue) => (
              <tr key={queue.id}>
                <td>
                  <Link to={`/queues/${queue.id}`}>{queue.name}</Link>
                </td>
                <td>{queue.priority}</td>
                <td>{queue.concurrency_limit}</td>
                <td>
                  <span className={`badge ${queue.is_paused ? 'badge-failed' : 'badge-completed'}`}>
                    {queue.is_paused ? 'paused' : 'active'}
                  </span>
                </td>
                <td>{new Date(queue.created_at).toLocaleString()}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
