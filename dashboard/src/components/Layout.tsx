import type { ReactNode } from 'react'
import { NavLink } from 'react-router-dom'
import type { SessionInfo } from '../api/types'
import { useAuth } from '../auth/useAuth'

const NAV_ITEMS = [
  { to: '/', label: 'Overview', end: true },
  { to: '/events', label: 'Event Explorer', end: false },
  { to: '/funnel', label: 'Funnel', end: false },
  { to: '/retention', label: 'Retention', end: false },
  { to: '/installations', label: 'Installation Timeline', end: false, managerOnly: true },
  { to: '/catalog', label: 'Catalog', end: false },
  { to: '/system', label: 'System Health', end: false },
  { to: '/policy', label: 'Policy', end: false },
  { to: '/project', label: 'Project settings', end: false, managerOnly: true },
  { to: '/projects', label: 'Projects', end: false, ownerOnly: true },
  { to: '/accounts', label: 'Accounts', end: false, ownerOnly: true },
]

export function Layout({ children }: { children: ReactNode }) {
  const { session, currentProject, setCurrentProject, logout } = useAuth()
  return (
    <div className="app-shell">
      <a href="#main-content" className="skip-link">Skip to main content</a>
      <SiteHeader session={session} currentProject={currentProject} setCurrentProject={setCurrentProject} logout={logout} />
      <div className="app-body">
        {session && <Navigation session={session} currentProject={currentProject} />}
        <main id="main-content">{children}</main>
      </div>
    </div>
  )
}

function SiteHeader({ session, currentProject, setCurrentProject, logout }: {
  session: SessionInfo | null
  currentProject: string
  setCurrentProject: (projectID: string) => void
  logout: () => Promise<void>
}) {
  return (
    <header className="site-header">
      <span className="brand">Mortris</span>
      {session && session.projects.length > 0 && <label className="project-select">Project<select value={currentProject} onChange={(e) => setCurrentProject(e.target.value)}>{session.projects.map((project) => <option key={project.id} value={project.id}>{project.display_name} ({project.environment})</option>)}</select></label>}
      {session && <div className="header-user"><span>{session.username} ({session.role})</span><button type="button" onClick={() => void logout()}>Sign out</button></div>}
    </header>
  )
}

function Navigation({ session, currentProject }: { session: SessionInfo; currentProject: string }) {
  const projectRole = session.projects.find((project) => project.id === currentProject)?.role
  const canManageCurrent = session.role === 'owner' || projectRole === 'project_admin'
  const visibleItems = NAV_ITEMS.filter((item) => !item.ownerOnly || session.role === 'owner').filter((item) => !item.managerOnly || canManageCurrent)
  return <nav aria-label="Dashboard sections"><ul>{visibleItems.map((item) => <li key={item.to}><NavLink to={item.to} end={item.end}>{item.label}</NavLink></li>)}</ul></nav>
}
