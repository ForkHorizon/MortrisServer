import type { ReactNode } from 'react'
import { NavLink } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'

const NAV_ITEMS = [
  { to: '/', label: 'Overview', end: true, adminOnly: false },
  { to: '/events', label: 'Event Explorer', end: false, adminOnly: false },
  { to: '/funnel', label: 'Funnel', end: false, adminOnly: false },
  { to: '/retention', label: 'Retention', end: false, adminOnly: false },
  { to: '/installations', label: 'Installation Timeline', end: false, adminOnly: true },
  { to: '/catalog', label: 'Catalog', end: false, adminOnly: false },
  { to: '/system', label: 'System Health', end: false, adminOnly: false },
  { to: '/policy', label: 'Policy', end: false, adminOnly: false },
]

export function Layout({ children }: { children: ReactNode }) {
  const { session, currentProject, setCurrentProject, logout } = useAuth()

  return (
    <div className="app-shell">
      <a href="#main-content" className="skip-link">
        Skip to main content
      </a>
      <header className="site-header">
        <span className="brand">Mortris</span>
        {session && session.project_ids.length > 1 && (
          <label className="project-select">
            Project
            <select value={currentProject} onChange={(e) => setCurrentProject(e.target.value)}>
              {session.project_ids.map((id) => (
                <option key={id} value={id}>
                  {id}
                </option>
              ))}
            </select>
          </label>
        )}
        {session && (
          <div className="header-user">
            <span>
              {session.email} ({session.role})
            </span>
            <button type="button" onClick={() => void logout()}>
              Sign out
            </button>
          </div>
        )}
      </header>
      <div className="app-body">
        {session && (
          <nav aria-label="Dashboard sections">
            <ul>
              {NAV_ITEMS.filter((item) => !item.adminOnly || session.role === 'admin').map((item) => (
                <li key={item.to}>
                  <NavLink to={item.to} end={item.end}>
                    {item.label}
                  </NavLink>
                </li>
              ))}
            </ul>
          </nav>
        )}
        <main id="main-content">{children}</main>
      </div>
    </div>
  )
}
