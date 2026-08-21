import { useState } from 'react'
import { Link } from 'react-router-dom'
import toast from 'react-hot-toast'
import { createProject, deleteProject, listProjects } from '../api/projects'
import ErrorBanner from '../components/ErrorBanner'
import { usePolling } from '../hooks/usePolling'

export default function ProjectsPage() {
  const { data: projects, error, reload } = usePolling(listProjects, [], 0)
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
    if (!confirm('Delete this project and everything under it?')) return
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
        <h2>Projects</h2>
      </div>

      <form className="inline-form" onSubmit={handleCreate}>
        <input
          placeholder="Project name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          required
        />
        <button className="btn" type="submit" disabled={submitting}>
          Create project
        </button>
      </form>
      <ErrorBanner message={error || formError} />

      {projects && projects.length === 0 && <p className="muted">No projects yet — create one above.</p>}

      <div className="card-grid">
        {projects?.map((project) => (
          <div className="entity-card" key={project.id}>
            <Link to={`/projects/${project.id}`} className="entity-card-title">
              {project.name}
            </Link>
            <div className="entity-card-meta">
              Created {new Date(project.created_at).toLocaleDateString()}
            </div>
            <button className="btn-ghost" onClick={() => handleDelete(project.id, project.name)}>
              Delete
            </button>
          </div>
        ))}
      </div>
    </div>
  )
}
