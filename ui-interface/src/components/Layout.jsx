import { LayoutGrid, LogOut, Server, Timer } from 'lucide-react'
import { NavLink, Outlet } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'

const NAV = [
  { to: '/', label: 'Overview', icon: LayoutGrid, end: true },
  { to: '/projects', label: 'Projects', icon: Timer, end: false },
  { to: '/workers', label: 'Workers', icon: Server, end: false },
]

export default function Layout() {
  const { user, logout } = useAuth()
  const initial = user?.email?.[0]?.toUpperCase() ?? '?'

  return (
    <div className="app-shell">
      <nav className="sidebar">
        <div className="brand">
          <span className="brand-mark">JS</span>
          <span className="brand-name">Job Scheduler</span>
        </div>

        <div className="nav-links">
          {NAV.map(({ to, label, icon: Icon, end }) => (
            <NavLink key={to} to={to} end={end} className={({ isActive }) => (isActive ? 'active' : '')}>
              <Icon size={16} strokeWidth={2} />
              {label}
            </NavLink>
          ))}
        </div>

        <div className="sidebar-spacer" />

        <div className="sidebar-footer">
          <div className="user-chip">
            <span className="user-avatar">{initial}</span>
            <span className="sidebar-user" title={user?.email}>
              {user?.email}
            </span>
          </div>
          <button className="logout" onClick={logout}>
            <LogOut size={15} />
            Log out
          </button>
        </div>
      </nav>
      <main className="content">
        <Outlet />
      </main>
    </div>
  )
}
