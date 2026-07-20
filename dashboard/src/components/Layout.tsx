import type { ReactNode } from 'react'
import { NavLink } from 'react-router-dom'
import { useAuth } from '../auth/useAuth'

const NAV_ITEMS = [
  { to: '/', label: 'Overview', end: true, managerOnly: false, ownerOnly: false },
  { to: '/events', label: 'Event Explorer', end: false, managerOnly: false, ownerOnly: false },
  { to: '/funnel', label: 'Funnel', end: false, managerOnly: false, ownerOnly: false },
  { to: '/retention', label: 'Retention', end: false, managerOnly: false, ownerOnly: false },
  { to: '/installations', label: 'Installation Timeline', end: false, managerOnly: true, ownerOnly: false },
  { to: '/catalog', label: 'Catalog', end: false, managerOnly: false, ownerOnly: false },
  { to: '/system', label: 'System Health', end: false, managerOnly: false, ownerOnly: false },
  { to: '/policy', label: 'Policy', end: false, managerOnly: false, ownerOnly: false },
  { to: '/project', label: 'Project settings', end: false, managerOnly: true, ownerOnly: false },
  { to: '/projects', label: 'Projects', end: false, managerOnly: false, ownerOnly: true },
  { to: '/accounts', label: 'Accounts', end: false, managerOnly: false, ownerOnly: true },
]

export function Layout({ children }: { children: ReactNode }) {
  const { session, currentProject, setCurrentProject, logout } = useAuth()
  const projectRole = session?.projects.find((project) => project.id === currentProject)?.role
  const canManageCurrent = session?.role === 'owner' || projectRole === 'project_admin'

  return (
    <div className="app-shell">
      <a href="#main-content" className="skip-link">
        Skip to main content
      </a>
      <header className="site-header">
        <span className="brand">Mortris</span>
        {session && session.projects.length > 0 && (
          <label className="project-select">
            Project
            <select value={currentProject} onChange={(e) => setCurrentProject(e.target.value)}>
              {session.projects.map((project) => (
                <option key={project.id} value={project.id}>
                  {project.display_name} ({project.environment})
                </option>
              ))}
            </select>
          </label>
        )}
        {session && (
          <div className="header-user">
            <span>
              {session.username} ({session.role})
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
              {NAV_ITEMS.filter((item) => !item.ownerOnly || session.role === 'owner').filter((item) => !item.managerOnly || canManageCurrent).map((item) => (
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
