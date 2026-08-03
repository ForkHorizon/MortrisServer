import type { ReactNode } from 'react'
import { NavLink, useLocation } from 'react-router-dom'
import type { SessionInfo } from '../api/types'
import { useAuth } from '../auth/useAuth'
import { isHouseArtHost } from '../houseArtHost'
import { useNewEventBadge } from '../hooks/useNewEventBadge'

// '/' is deliberately absent: on this host it renders the house wall, so
// a nav link still labelled "Overview" would point at Houses and lie.
const HOUSE_ART_NAV = new Set(['/houses', '/gameplay'])

const NAV_ITEMS = [
  { to: '/', label: 'Overview', end: true },
  { to: '/ask', label: 'Ask', end: false },
  { to: '/feed', label: 'Live Feed', end: false },
  { to: '/events', label: 'Event Explorer', end: false },
  { to: '/funnel', label: 'Funnel', end: false },
  { to: '/retention', label: 'Retention', end: false },
  { to: '/installations', label: 'Installation Timeline', end: false, managerOnly: true },
  { to: '/catalog', label: 'Catalog', end: false },
  { to: '/houses', label: 'Houses', end: false, managerOnly: true },
  { to: '/gameplay', label: 'Gameplay Diagnostics', end: false, managerOnly: true },
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
      {session && session.projects.length > 0 && !isHouseArtHost() && <label className="project-select">Project<select value={currentProject} onChange={(e) => setCurrentProject(e.target.value)}>{session.projects.map((project) => <option key={project.id} value={project.id}>{project.display_name} ({project.environment})</option>)}</select></label>}
      {session && <div className="header-user"><span>{session.username} ({session.role})</span><button type="button" onClick={() => void logout()}>Sign out</button></div>}
    </header>
  )
}

function Navigation({ session, currentProject }: { session: SessionInfo; currentProject: string }) {
  const projectRole = session.projects.find((project) => project.id === currentProject)?.role
  const canManageCurrent = session.role === 'owner' || projectRole === 'project_admin'
  // The houseart host exists for one thing, so it shows one thing. This
  // hides links, not capability — every route stays reachable by URL and
  // is still guarded server-side.
  const visibleItems = NAV_ITEMS.filter((item) => !item.ownerOnly || session.role === 'owner')
    .filter((item) => !item.managerOnly || canManageCurrent)
    .filter((item) => !isHouseArtHost() || HOUSE_ART_NAV.has(item.to))
  const newEventCount = useNewEventBadge(currentProject)
  const { search } = useLocation()
  return (
    <nav aria-label="Dashboard sections">
      <ul>
        {visibleItems.map((item) => (
          <li key={item.to}>
            <NavLink to={item.to + search} end={item.end}>
              {item.label}
              {item.to === '/catalog' && newEventCount > 0 && (
                <span className="nav-badge" title={`${newEventCount} new undeclared event${newEventCount === 1 ? '' : 's'} this week`}>
                  {newEventCount}
                </span>
              )}
            </NavLink>
          </li>
        ))}
      </ul>
    </nav>
  )
}
