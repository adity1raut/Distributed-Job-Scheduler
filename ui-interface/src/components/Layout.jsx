import { NavLink, Outlet } from 'react-router-dom'

export default function Layout() {
  return (
    <div className="app-shell">
      <nav className="sidebar">
        <NavLink to="/" end>
          Dashboard
        </NavLink>
        <NavLink to="/projects">Projects</NavLink>
        <NavLink to="/queues">Queues</NavLink>
        <NavLink to="/jobs">Jobs</NavLink>
      </nav>
      <main className="content">
        <Outlet />
      </main>
    </div>
  )
}
