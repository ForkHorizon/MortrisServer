import { Suspense, lazy } from 'react'
import type { ComponentType } from 'react'
import { BrowserRouter, Route, Routes } from 'react-router-dom'
import { AuthProvider } from './auth/AuthContext'
import { RequireAdmin, RequireAuth } from './auth/RequireAuth'
import { Layout } from './components/Layout'
import { LoginPage } from './pages/LoginPage'

const OverviewPage = lazy(() => import('./pages/OverviewPage').then((m) => ({ default: m.OverviewPage })))
const EventExplorerPage = lazy(() => import('./pages/EventExplorerPage').then((m) => ({ default: m.EventExplorerPage })))
const FunnelPage = lazy(() => import('./pages/FunnelPage').then((m) => ({ default: m.FunnelPage })))
const RetentionPage = lazy(() => import('./pages/RetentionPage').then((m) => ({ default: m.RetentionPage })))
const InstallationTimelinePage = lazy(() => import('./pages/InstallationTimelinePage').then((m) => ({ default: m.InstallationTimelinePage })))
const CatalogPage = lazy(() => import('./pages/CatalogPage').then((m) => ({ default: m.CatalogPage })))
const SystemHealthPage = lazy(() => import('./pages/SystemHealthPage').then((m) => ({ default: m.SystemHealthPage })))
const PolicyAdminPage = lazy(() => import('./pages/PolicyAdminPage').then((m) => ({ default: m.PolicyAdminPage })))
const ProjectsPage = lazy(() => import('./pages/ProjectsPage').then((m) => ({ default: m.ProjectsPage })))
const ProjectAdminPage = lazy(() => import('./pages/ProjectAdminPage').then((m) => ({ default: m.ProjectAdminPage })))
const AccountsPage = lazy(() => import('./pages/AccountsPage').then((m) => ({ default: m.AccountsPage })))
const GameplayDiagnosticsPage = lazy(() => import('./pages/GameplayDiagnosticsPage').then((m) => ({ default: m.GameplayDiagnosticsPage })))

const dashboardRoutes: Array<{ path: string; Page: ComponentType; adminOnly?: boolean }> = [
  { path: '/', Page: OverviewPage },
  { path: '/events', Page: EventExplorerPage },
  { path: '/funnel', Page: FunnelPage },
  { path: '/retention', Page: RetentionPage },
  { path: '/installations', Page: InstallationTimelinePage, adminOnly: true },
  { path: '/catalog', Page: CatalogPage },
  { path: '/gameplay', Page: GameplayDiagnosticsPage, adminOnly: true },
  { path: '/system', Page: SystemHealthPage },
  { path: '/policy', Page: PolicyAdminPage },
  { path: '/project', Page: ProjectAdminPage },
  { path: '/projects', Page: ProjectsPage, adminOnly: true },
  { path: '/accounts', Page: AccountsPage, adminOnly: true },
]

function RouteFallback() {
  return <p role="status">Loading…</p>
}

function ProtectedPage({ Page, adminOnly = false }: { Page: ComponentType; adminOnly?: boolean }) {
  const content = <Suspense fallback={<RouteFallback />}><Page /></Suspense>
  return adminOnly ? <RequireAdmin>{content}</RequireAdmin> : <RequireAuth>{content}</RequireAuth>
}

function DashboardRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      {dashboardRoutes.map(({ path, Page, adminOnly }) => (
        <Route key={path} path={path} element={<ProtectedPage Page={Page} adminOnly={adminOnly} />} />
      ))}
    </Routes>
  )
}

export default function App() {
  return <BrowserRouter><AuthProvider><Layout><DashboardRoutes /></Layout></AuthProvider></BrowserRouter>
}
