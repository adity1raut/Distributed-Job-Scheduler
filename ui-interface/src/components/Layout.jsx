import { LogOut, Menu, X } from 'lucide-react'
import { useState } from 'react'
import { NavLink, Outlet } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import Wordmark from './Wordmark'

const NAV = [
  { to: '/', label: 'Overview', end: true },
  { to: '/projects', label: 'Projects', end: false },
  { to: '/workers', label: 'Workers', end: false },
]

export default function Layout() {
  const { user, logout } = useAuth()
  const [drawerOpen, setDrawerOpen] = useState(false)
  const initial = user?.email?.[0]?.toUpperCase() ?? '?'

  return (
    <div className="app-shell">
      <header className="mobile-topbar">
        <button className="icon-btn" onClick={() => setDrawerOpen(true)} aria-label="Open menu">
          <Menu size={20} />
        </button>
        <div className="brand">
          <Wordmark />
        </div>
      </header>

      {drawerOpen && <div className="sidebar-backdrop" onClick={() => setDrawerOpen(false)} />}

      <nav className={`sidebar${drawerOpen ? ' sidebar-open' : ''}`}>
        <div className="brand sidebar-brand">
          <Wordmark />
          <button className="icon-btn sidebar-close" onClick={() => setDrawerOpen(false)} aria-label="Close menu">
            <X size={18} />
          </button>
        </div>

        <div className="nav-links">
          {NAV.map(({ to, label, end }) => (
            <NavLink
              key={to}
              to={to}
              end={end}
              onClick={() => setDrawerOpen(false)}
              className={({ isActive }) => (isActive ? 'active' : '')}
            >
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
