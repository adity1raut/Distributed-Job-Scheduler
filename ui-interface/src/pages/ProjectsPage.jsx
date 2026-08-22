import { FolderKanban, FolderPlus, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router-dom'
import toast from 'react-hot-toast'
import { createProject, deleteProject, listProjects } from '../api/projects'
import EmptyState from '../components/EmptyState'
import ErrorBanner from '../components/ErrorBanner'
import Timestamp from '../components/Timestamp'
import { usePolling } from '../hooks/usePolling'

export default function ProjectsPage() {
  const { data: projects, error, loading, reload } = usePolling(listProjects, [], 0)
  const [name, setName] = useState('')
  const [formError, setFormError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const handleCreate = async (e) => {
    e.preventDefault()
    setFormError('')
    setSubmitting(true)
    try {
      await createProject(name)
      setName('')
      reload()
      toast.success(`Project "${name}" created`)
    } catch (err) {
      setFormError(err.message)
      toast.error(err.message)
    } finally {
      setSubmitting(false)
    }
  }

  const handleDelete = async (id, name) => {
    if (!confirm(`Delete "${name}" and everything under it — queues, jobs, execution history?`)) return
    try {
      await deleteProject(id)
      reload()
      toast.success(`Project "${name}" deleted`)
    } catch (err) {
      toast.error(err.message)
    }
  }

  return (
    <div>
      <div className="page-header">
        <div>
          <h2>Projects</h2>
          <p className="page-sub">Each project owns its own set of job queues.</p>
        </div>
      </div>

      <form className="inline-form" onSubmit={handleCreate}>
        <input
          placeholder="Project name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          required
        />
        <button className="btn" type="submit" disabled={submitting}>
          <FolderPlus size={15} />
          Create project
        </button>
      </form>
      <ErrorBanner message={error || formError} />

      {!loading && projects && projects.length === 0 && (
        <EmptyState icon={FolderKanban} title="No projects yet" hint="Create one above to start scheduling jobs." />
      )}

      <div className="card-grid">
        {projects?.map((project) => (
          <div className="entity-card" key={project.id}>
            <div className="entity-card-icon">
              <FolderKanban size={16} strokeWidth={2} />
            </div>
            <Link to={`/projects/${project.id}`} className="entity-card-title">
              {project.name}
            </Link>
            <div className="entity-card-meta">
              Created <Timestamp value={project.created_at} />
            </div>
            <button className="btn-ghost btn-danger" onClick={() => handleDelete(project.id, project.name)}>
              <Trash2 size={13} />
              Delete
            </button>
          </div>
        ))}
      </div>
    </div>
  )
}
