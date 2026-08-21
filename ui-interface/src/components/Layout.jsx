import { NavLink, Outlet } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'

export default function Layout() {
  const { user, logout } = useAuth()

  return (
    <div className="app-shell">
      <nav className="sidebar">
        <div className="brand">Job Scheduler</div>
        <NavLink to="/" end>
          Overview
        </NavLink>
        <NavLink to="/projects">Projects</NavLink>
        <NavLink to="/workers">Workers</NavLink>
        <div className="sidebar-spacer" />
        <div className="sidebar-user">{user?.email}</div>
        <button className="logout" onClick={logout}>
          Log out
        </button>
      </nav>
      <main className="content">
        <Outlet />
      </main>
    </div>
  )
}
